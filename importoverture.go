package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/jeiang/walkmap/internal/taxonomy"
)

// overtureColumns is the header written by import/overture.sh's DuckDB
// query: id,name,category,alt,confidence,src,lon,lat. alt (categories.alternate)
// is read but never used — the plan only maps categories.primary.
var overtureColumns = []string{"id", "name", "category", "alt", "confidence", "src", "lon", "lat"}

func cmdImportOverture(ctx context.Context, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}
	col := make(map[string]int, len(header))
	for i, name := range header {
		col[name] = i
	}
	for _, want := range overtureColumns {
		if _, ok := col[want]; !ok {
			return fmt.Errorf("overture csv missing column %q", want)
		}
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

	// Same delete+insert-in-one-transaction approach as import osm; see
	// the comment there.
	if _, err := tx.Exec(ctx, `delete from places where source = 'overture'`); err != nil {
		return fmt.Errorf("clear overture rows: %w", err)
	}

	var imported, skipped int
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read overture csv: %w", err)
		}

		rawCategory := record[col["category"]]
		category, ok := taxonomy.OvertureCategory(rawCategory)
		if !ok {
			skipped++
			continue
		}
		lon, err := strconv.ParseFloat(record[col["lon"]], 64)
		if err != nil {
			return fmt.Errorf("parse lon for %s: %w", record[col["id"]], err)
		}
		lat, err := strconv.ParseFloat(record[col["lat"]], 64)
		if err != nil {
			return fmt.Errorf("parse lat for %s: %w", record[col["id"]], err)
		}

		var confidence *float64
		if s := record[col["confidence"]]; s != "" {
			c, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return fmt.Errorf("parse confidence for %s: %w", record[col["id"]], err)
			}
			confidence = &c
		}

		rawTags, err := recordJSON(header, record)
		if err != nil {
			return err
		}

		id := "ovt:" + record[col["id"]]
		name := record[col["name"]]
		_, err = tx.Exec(ctx, `
			insert into places (id, source, name, category, raw_tags, confidence, hidden, geom)
			values ($1, 'overture', nullif($2, ''), $3, $4, $5, false, st_setsrid(st_makepoint($6, $7), 4326))
		`, id, name, category, rawTags, confidence, lon, lat)
		if err != nil {
			return fmt.Errorf("insert %s: %w", id, err)
		}
		imported++
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Printf("import overture: %d imported, %d skipped\n", imported, skipped)
	return nil
}

// recordJSON stores the raw CSV row as the places.raw_tags jsonb column,
// keyed by header, so the Overture source record isn't lost even though
// only category/name/confidence/geom are used elsewhere.
func recordJSON(header, record []string) ([]byte, error) {
	m := make(map[string]string, len(header))
	for i, name := range header {
		if i < len(record) {
			m[name] = record[i]
		}
	}
	return json.Marshal(m)
}
