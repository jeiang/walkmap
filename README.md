# walkmap

Category search by walking time for Barbados. This repo currently holds
piece 1 of the plan: the PostGIS schema and the OSM/Overture import
pipeline. See `docs/WALKMAP.md` in the `cornn-flaek` flake repo for the
full design.

## Requirements

`nix develop` provides everything: Go, PostgreSQL 17 + PostGIS, DuckDB,
osmium-tool, curl, jq.

## Running the import locally

```
just db-start      # local postgres in .pg/, port 5433, db `walkmap`
just migrate        # extensions + places table
just fetch-osm       # import/osm.sh -> data/places.json, data/barbados.osm.pbf
just fetch-overture  # import/overture.sh -> data/overture.csv
just import           # osm + overture + dedupe
just db-stop
```

`import/osm.sh` pulls from the public Overpass instance by default;
override with `OVERPASS_URL` if it rejects the full-island road export.

## walkmap CLI

```
walkmap migrate                       # idempotent: extensions + places table
walkmap import osm <overpass.json>    # upserts OSM places
walkmap import overture <overture.csv> # upserts Overture places
walkmap import dedupe                 # hides Overture rows that duplicate an OSM row
```

Config is one env var: `DATABASE_URL` (a standard `postgresql://` URL).
