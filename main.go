// Command walkmap runs the walkmap import pipeline (schema migration,
// OSM/Overture ingestion, docs/WALKMAP.md piece 1) and the API service
// (docs/WALKMAP.md piece 2).
package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "walkmap:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageErr()
	}
	switch args[0] {
	case "migrate":
		return cmdMigrate(ctx)
	case "serve":
		return cmdServe(ctx, args[1:])
	case "import":
		if len(args) < 2 {
			return usageErr()
		}
		switch args[1] {
		case "osm":
			if len(args) != 3 {
				return fmt.Errorf("usage: walkmap import osm <overpass.json>")
			}
			return cmdImportOSM(ctx, args[2])
		case "overture":
			if len(args) != 3 {
				return fmt.Errorf("usage: walkmap import overture <overture.csv>")
			}
			return cmdImportOverture(ctx, args[2])
		case "dedupe":
			return cmdDedupe(ctx)
		default:
			return usageErr()
		}
	default:
		return usageErr()
	}
}

func usageErr() error {
	return fmt.Errorf(`usage:
  walkmap migrate
  walkmap serve
  walkmap import osm <overpass.json>
  walkmap import overture <overture.csv>
  walkmap import dedupe`)
}
