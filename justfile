db_url := "postgresql://127.0.0.1:5433/walkmap"

# Start a local postgres in .pg/ (port 5433, trust auth) and create db `walkmap`.
db-start:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p .pg
    if [ ! -d .pg/data ]; then
        initdb -D .pg/data -U "$(whoami)" --auth=trust >/dev/null
    fi
    pg_ctl -D .pg/data -l .pg/log -o "-p 5433 -h 127.0.0.1" start
    for _ in $(seq 1 30); do
        pg_isready -h 127.0.0.1 -p 5433 >/dev/null 2>&1 && break
        sleep 0.5
    done
    createdb -h 127.0.0.1 -p 5433 walkmap 2>/dev/null || true

db-stop:
    pg_ctl -D .pg/data stop -m fast

migrate:
    DATABASE_URL={{db_url}} go run . migrate

fetch-osm:
    import/osm.sh data

fetch-overture:
    import/overture.sh data

# Import osm + overture + dedupe from data/ (run fetch-osm/fetch-overture first).
import:
    DATABASE_URL={{db_url}} go run . import osm data/places.json
    DATABASE_URL={{db_url}} go run . import overture data/overture.csv
    DATABASE_URL={{db_url}} go run . import dedupe

# Run the API service against the local postgres (db-start first).
# Override VALHALLA_URL in the environment to point at a running Valhalla.
serve:
    DATABASE_URL={{db_url}} VALHALLA_URL={{env_var_or_default("VALHALLA_URL", "http://127.0.0.1:8002")}} go run . serve

# Build Valhalla tiles from data/barbados.osm.pbf. Linux only: nixpkgs'
# valhalla has no darwin build (see valhalla/build-tiles.sh).
build-tiles:
    valhalla/build-tiles.sh data/barbados.osm.pbf data/valhalla

# Build data/basemap/basemap.pmtiles from data/barbados.osm.pbf (tilemaker;
# see basemap/build-pmtiles.sh for the coastline cache and bbox knobs).
build-basemap:
    basemap/build-pmtiles.sh data/barbados.osm.pbf data/basemap
