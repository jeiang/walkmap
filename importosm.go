package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jeiang/walkmap/internal/taxonomy"
)

type overpassResult struct {
	Elements []overpassElement `json:"elements"`
}

type overpassElement struct {
	Type   string            `json:"type"`
	ID     int64             `json:"id"`
	Lat    *float64          `json:"lat"`
	Lon    *float64          `json:"lon"`
	Center *overpassLatLon   `json:"center"`
	Tags   map[string]string `json:"tags"`
}

type overpassLatLon struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// typeInitial maps an Overpass element type to the OSM id-prefix letter
// used throughout OSM tooling (n/w/r).
func typeInitial(t string) (string, bool) {
	switch t {
	case "node":
		return "n", true
	case "way":
		return "w", true
	case "relation":
		return "r", true
	default:
		return "", false
	}
}

func cmdImportOSM(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var result overpassResult
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse overpass json: %w", err)
	}

	conn, err := connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Whole-source delete+insert in one transaction: the plan mentions a
	// places_new swap table, but for a single `source` this is equivalent
	// and needs no extra table.
	if _, err := tx.Exec(ctx, `delete from places where source = 'osm'`); err != nil {
		return fmt.Errorf("clear osm rows: %w", err)
	}

	var imported, skipped int
	for _, el := range result.Elements {
		prefix, ok := typeInitial(el.Type)
		if !ok {
			skipped++
			continue
		}
		category, ok := taxonomy.OSMCategory(el.Tags)
		if !ok {
			skipped++
			continue
		}
		lat, lon, ok := el.latLon()
		if !ok {
			skipped++
			continue
		}
		rawTags, err := json.Marshal(el.Tags)
		if err != nil {
			return fmt.Errorf("marshal tags for %s%d: %w", prefix, el.ID, err)
		}
		id := fmt.Sprintf("osm:%s%d", prefix, el.ID)
		name := el.Tags["name"]
		_, err = tx.Exec(ctx, `
			insert into places (id, source, name, category, raw_tags, confidence, hidden, geom)
			values ($1, 'osm', nullif($2, ''), $3, $4, null, false, st_setsrid(st_makepoint($5, $6), 4326))
		`, id, name, category, rawTags, lon, lat)
		if err != nil {
			return fmt.Errorf("insert %s: %w", id, err)
		}
		imported++
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Printf("import osm: %d imported, %d skipped\n", imported, skipped)
	return nil
}

// latLon resolves a point from either the node's own lat/lon or the
// way/relation's `out center` centroid.
func (e overpassElement) latLon() (lat, lon float64, ok bool) {
	if e.Lat != nil && e.Lon != nil {
		return *e.Lat, *e.Lon, true
	}
	if e.Center != nil {
		return e.Center.Lat, e.Center.Lon, true
	}
	return 0, 0, false
}
