package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// valhallaClient talks to a Valhalla service (valhalla_service /
// valhalla.json). All requests use a 10s timeout: Valhalla runs on the
// same host/mesh, so anything slower than that means Valhalla itself is
// stuck.
type valhallaClient struct {
	baseURL string
	hc      *http.Client
}

func newValhallaClient(baseURL string) *valhallaClient {
	return &valhallaClient{
		baseURL: baseURL,
		hc:      &http.Client{Timeout: 10 * time.Second},
	}
}

type valhallaLocation struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// status calls GET /status; a non-200 or network error means Valhalla is
// unavailable.
func (c *valhallaClient) status(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/status", nil)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// post issues a POST with a JSON body and decodes a JSON response,
// surfacing Valhalla's error body (it returns {"error": "...", ...} with a
// non-200 status) rather than a bare status code.
func (c *valhallaClient) post(ctx context.Context, path string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("valhalla %s: %w", path, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("valhalla %s: read response: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("valhalla %s: status %d: %s", path, resp.StatusCode, bytes.TrimSpace(respBody))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("valhalla %s: decode response: %w", path, err)
		}
	}
	return nil
}

// isochrone requests one pedestrian contour at minutes and returns the raw
// GeoJSON geometry of the first (only) contour polygon, suitable for
// ST_GeomFromGeoJSON.
func (c *valhallaClient) isochrone(ctx context.Context, lat, lon float64, minutes int) (json.RawMessage, error) {
	body := map[string]any{
		"locations": []valhallaLocation{{Lat: lat, Lon: lon}},
		"costing":   "pedestrian",
		"contours":  []map[string]int{{"time": minutes}},
		"polygons":  true,
	}
	var out struct {
		Features []struct {
			Geometry json.RawMessage `json:"geometry"`
		} `json:"features"`
	}
	if err := c.post(ctx, "/isochrone", body, &out); err != nil {
		return nil, err
	}
	if len(out.Features) == 0 {
		return nil, fmt.Errorf("valhalla /isochrone: no contour in response")
	}
	return out.Features[0].Geometry, nil
}

type walkResult struct {
	Minutes float64
	Meters  float64
}

// sourcesToTargets returns pedestrian walk time/distance from one source to
// each target, in the same order as targets.
func (c *valhallaClient) sourcesToTargets(ctx context.Context, from valhallaLocation, targets []valhallaLocation) ([]walkResult, error) {
	body := map[string]any{
		"sources": []valhallaLocation{from},
		"targets": targets,
		"costing": "pedestrian",
		"units":   "kilometers",
	}
	var out struct {
		SourcesToTargets [][]struct {
			Time     *float64 `json:"time"`
			Distance *float64 `json:"distance"`
		} `json:"sources_to_targets"`
	}
	if err := c.post(ctx, "/sources_to_targets", body, &out); err != nil {
		return nil, err
	}
	if len(out.SourcesToTargets) != 1 {
		return nil, fmt.Errorf("valhalla /sources_to_targets: expected 1 source row, got %d", len(out.SourcesToTargets))
	}
	row := out.SourcesToTargets[0]
	if len(row) != len(targets) {
		return nil, fmt.Errorf("valhalla /sources_to_targets: expected %d targets, got %d", len(targets), len(row))
	}
	results := make([]walkResult, len(row))
	for i, cell := range row {
		if cell.Time == nil || cell.Distance == nil {
			// Valhalla returns null time/distance for an unreachable
			// target; treat it as infinitely far so it sorts last.
			results[i] = walkResult{Minutes: -1, Meters: -1}
			continue
		}
		results[i] = walkResult{Minutes: *cell.Time / 60, Meters: *cell.Distance * 1000}
	}
	return results, nil
}

type routeResult struct {
	Shape   string
	Minutes float64
	Meters  float64
}

// route requests a pedestrian route between two points and returns the
// polyline6-encoded shape plus total time/length.
func (c *valhallaClient) route(ctx context.Context, from, to valhallaLocation) (routeResult, error) {
	body := map[string]any{
		"locations": []valhallaLocation{from, to},
		"costing":   "pedestrian",
		"units":     "kilometers",
	}
	var out struct {
		Trip struct {
			Legs []struct {
				Shape string `json:"shape"`
			} `json:"legs"`
			Summary struct {
				Time   float64 `json:"time"`
				Length float64 `json:"length"`
			} `json:"summary"`
		} `json:"trip"`
	}
	if err := c.post(ctx, "/route", body, &out); err != nil {
		return routeResult{}, err
	}
	if len(out.Trip.Legs) == 0 {
		return routeResult{}, fmt.Errorf("valhalla /route: no legs in response")
	}
	return routeResult{
		Shape:   out.Trip.Legs[0].Shape,
		Minutes: out.Trip.Summary.Time / 60,
		Meters:  out.Trip.Summary.Length * 1000,
	}, nil
}
