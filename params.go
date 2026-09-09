package main

import (
	"fmt"
	"strconv"
)

// parseMinutes parses the nearby endpoint's minutes= param: default 20,
// range 5-90 (matching the isochrone max_time_contour set in
// valhalla/build-tiles.sh).
func parseMinutes(s string) (int, error) {
	if s == "" {
		return 20, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("minutes: not an integer")
	}
	if n < 5 || n > 90 {
		return 0, fmt.Errorf("minutes: must be between 5 and 90")
	}
	return n, nil
}

// parseMinConfidence parses min_confidence=: default 0.5, range 0-1.
func parseMinConfidence(s string) (float64, error) {
	if s == "" {
		return 0.5, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("min_confidence: not a number")
	}
	if f < 0 || f > 1 {
		return 0, fmt.Errorf("min_confidence: must be between 0 and 1")
	}
	return f, nil
}

// parseLatLon parses required lat/lon query params.
func parseLatLon(latStr, lonStr string) (lat, lon float64, err error) {
	if latStr == "" || lonStr == "" {
		return 0, 0, fmt.Errorf("lat and lon are required")
	}
	lat, err = strconv.ParseFloat(latStr, 64)
	if err != nil || lat < -90 || lat > 90 {
		return 0, 0, fmt.Errorf("lat: must be a number between -90 and 90")
	}
	lon, err = strconv.ParseFloat(lonStr, 64)
	if err != nil || lon < -180 || lon > 180 {
		return 0, 0, fmt.Errorf("lon: must be a number between -180 and 180")
	}
	return lat, lon, nil
}

// parseLimit parses limit= with a default and a hard max.
func parseLimit(s string, def, max int) (int, error) {
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("limit: must be a positive integer")
	}
	if n > max {
		n = max
	}
	return n, nil
}
