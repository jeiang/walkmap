package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jeiang/walkmap/internal/polyline"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleHealthz checks the database and Valhalla and returns 200 or 503.
func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := map[string]string{}
	ok := true

	if _, err := s.db.Exec(ctx, "select 1"); err != nil {
		checks["database"] = err.Error()
		ok = false
	} else {
		checks["database"] = "ok"
	}

	if err := s.valhalla.status(ctx); err != nil {
		checks["valhalla"] = err.Error()
		ok = false
	} else {
		checks["valhalla"] = "ok"
	}

	status := http.StatusOK
	if !ok {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{"ok": ok, "checks": checks})
}

// handleCategories returns the app taxonomy present in the visible rows,
// with a count per source.
func (s *server) handleCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `
		select category, source, count(*)
		from places
		where not hidden
		group by category, source
		order by category, source
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query categories: "+err.Error())
		return
	}
	defer rows.Close()

	type counts struct {
		Category string         `json:"category"`
		Counts   map[string]int `json:"counts"`
	}
	byCategory := map[string]*counts{}
	var order []string
	for rows.Next() {
		var category, source string
		var count int
		if err := rows.Scan(&category, &source, &count); err != nil {
			writeError(w, http.StatusInternalServerError, "scan categories: "+err.Error())
			return
		}
		c, ok := byCategory[category]
		if !ok {
			c = &counts{Category: category, Counts: map[string]int{}}
			byCategory[category] = c
			order = append(order, category)
		}
		c.Counts[source] = count
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "query categories: "+err.Error())
		return
	}

	out := make([]*counts, 0, len(order))
	for _, category := range order {
		out = append(out, byCategory[category])
	}
	writeJSON(w, http.StatusOK, out)
}

type placeResult struct {
	ID          string   `json:"id"`
	Source      string   `json:"source"`
	Name        *string  `json:"name"` // null for the ~12% of OSM rows with no name tag
	Category    string   `json:"category"`
	Confidence  *float64 `json:"confidence"`
	Lat         float64  `json:"lat"`
	Lon         float64  `json:"lon"`
	WalkMinutes *float64 `json:"walk_minutes,omitempty"`
	WalkMeters  *float64 `json:"walk_meters,omitempty"`
	DistanceM   *float64 `json:"distance_m,omitempty"`
}

// handleNearby implements docs/WALKMAP.md's nearby query: isochrone from
// Valhalla, then PostGIS filter, then either walk-time or straight-line
// sort depending on result-set size.
func (s *server) handleNearby(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	lat, lon, err := parseLatLon(q.Get("lat"), q.Get("lon"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	minutes, err := parseMinutes(q.Get("minutes"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	minConfidence, err := parseMinConfidence(q.Get("min_confidence"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	categories := q["category"]
	if len(categories) == 0 {
		writeError(w, http.StatusBadRequest, "category is required (repeat the param for multiple)")
		return
	}

	ctx := r.Context()

	geometry, err := s.valhalla.isochrone(ctx, lat, lon, minutes)
	if err != nil {
		writeError(w, http.StatusBadGateway, "valhalla isochrone failed: "+err.Error())
		return
	}

	rows, err := s.db.Query(ctx, `
		select id, source, coalesce(name, ''), category, confidence, st_y(geom), st_x(geom),
		       st_distance(geom::geography, st_setsrid(st_makepoint($4, $5), 4326)::geography)
		from places
		where st_within(geom, st_setsrid(st_geomfromgeojson($1), 4326))
		  and category = any($2)
		  and not hidden
		  and (confidence is null or confidence >= $3)
	`, string(geometry), categories, minConfidence, lon, lat)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query places: "+err.Error())
		return
	}
	places, err := scanPlaces(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "scan places: "+err.Error())
		return
	}

	sortedBy := "walk_time"
	const walkTimeLimit = 200
	if len(places) > 0 && len(places) <= walkTimeLimit {
		targets := make([]valhallaLocation, len(places))
		for i, p := range places {
			targets[i] = valhallaLocation{Lat: p.Lat, Lon: p.Lon}
		}
		results, err := s.valhalla.sourcesToTargets(ctx, valhallaLocation{Lat: lat, Lon: lon}, targets)
		if err != nil {
			writeError(w, http.StatusBadGateway, "valhalla sources_to_targets failed: "+err.Error())
			return
		}
		for i := range places {
			minutes, meters := results[i].Minutes, results[i].Meters
			places[i].WalkMinutes = &minutes
			places[i].WalkMeters = &meters
		}
		sort.Slice(places, func(i, j int) bool {
			return orInf(places[i].WalkMinutes) < orInf(places[j].WalkMinutes)
		})
	} else {
		sortedBy = "distance"
		sort.Slice(places, func(i, j int) bool {
			return orInf(places[i].DistanceM) < orInf(places[j].DistanceM)
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"isochrone": json.RawMessage(geometry),
		"sorted_by": sortedBy,
		"places":    places,
	})
}

// orInf treats a negative (unreachable, see sourcesToTargets) or missing
// value as +Inf so it sorts last.
func orInf(f *float64) float64 {
	if f == nil || *f < 0 {
		return 1e18
	}
	return *f
}

// scanPlaces reads places+distance_m rows shared by handleNearby.
func scanPlaces(rows pgx.Rows) ([]placeResult, error) {
	defer rows.Close()
	var places []placeResult
	for rows.Next() {
		var p placeResult
		var distanceM float64
		if err := rows.Scan(&p.ID, &p.Source, &p.Name, &p.Category, &p.Confidence, &p.Lat, &p.Lon, &distanceM); err != nil {
			return nil, err
		}
		p.DistanceM = &distanceM
		places = append(places, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return places, nil
}

// handleSearch implements the pg_trgm name search.
func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := q.Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}
	limit, err := parseLimit(q.Get("limit"), 20, 100)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var hasLatLon bool
	var lat, lon float64
	if q.Get("lat") != "" || q.Get("lon") != "" {
		lat, lon, err = parseLatLon(q.Get("lat"), q.Get("lon"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		hasLatLon = true
	}

	sql := `
		select id, source, name, category, confidence, st_y(geom), st_x(geom),
		       st_distance(geom::geography, st_setsrid(st_makepoint($2, $3), 4326)::geography)
		from places
		where not hidden and (name % $1 or name ilike '%' || $1 || '%')
		order by similarity(name, $1) desc`
	if hasLatLon {
		sql += `, st_distance(geom::geography, st_setsrid(st_makepoint($2, $3), 4326)::geography) asc`
	}
	sql += ` limit $4`

	rows, err := s.db.Query(r.Context(), sql, query, lon, lat, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search: "+err.Error())
		return
	}
	places, err := scanPlaces(rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "scan search results: "+err.Error())
		return
	}
	if !hasLatLon {
		for i := range places {
			places[i].DistanceM = nil
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"places": places})
}

// handleRoute looks up a place by id and asks Valhalla for a pedestrian
// route from from_lat/from_lon to it.
func (s *server) handleRoute(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fromLat, fromLon, err := parseLatLon(q.Get("from_lat"), q.Get("from_lon"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	toID := q.Get("to_id")
	if toID == "" {
		writeError(w, http.StatusBadRequest, "to_id is required")
		return
	}

	var toLat, toLon float64
	err = s.db.QueryRow(r.Context(), `select st_y(geom), st_x(geom) from places where id = $1 and not hidden`, toID).Scan(&toLat, &toLon)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "no such place: "+toID)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "look up place: "+err.Error())
		return
	}

	result, err := s.valhalla.route(r.Context(), valhallaLocation{Lat: fromLat, Lon: fromLon}, valhallaLocation{Lat: toLat, Lon: toLon})
	if err != nil {
		writeError(w, http.StatusBadGateway, "valhalla route failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"shape":   polyline.Decode6(result.Shape),
		"minutes": result.Minutes,
		"meters":  result.Meters,
	})
}
