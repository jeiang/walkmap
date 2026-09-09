# walkmap

Category search by walking time for Barbados. This repo currently holds
pieces 1-2 of the plan: the PostGIS schema and OSM/Overture import
pipeline, and the Valhalla tiles + Go API service. See `docs/WALKMAP.md`
in the `cornn-flaek` flake repo for the full design.

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

## Valhalla tiles and routing

Valhalla is Linux-only in nixpkgs (`nixpkgs#valhalla`'s `meta.platforms`
lists Linux systems only — verified with `nix eval nixpkgs#valhalla.meta.platforms`,
which is empty on `aarch64-darwin`), so tiles are built and the routing
service run on a Linux host, not this Mac.

On a Linux host, with `nixpkgs#valhalla` available:

```
valhalla/build-tiles.sh data/barbados.osm.pbf data/valhalla   # or: just build-tiles
valhalla_service data/valhalla/current/valhalla.json 1
```

`build-tiles.sh` builds into a fresh `<out-dir>/builds/<timestamp>`
directory and only swaps the `<out-dir>/current` symlink on success, so a
failed rebuild keeps serving the last-good tiles. See `valhalla/config.md`
for the service's listen port (8002 by default) and `valhalla/build-tiles.sh`
for the isochrone limits it configures.

## walkmap serve

```
just serve   # DATABASE_URL against the local postgres; VALHALLA_URL from env, default http://127.0.0.1:8002
```

Flags/env:

| Flag | Env | Default |
|---|---|---|
| `-listen` | `WALKMAP_LISTEN` | `127.0.0.1:8867` |
| | `DATABASE_URL` | (required) |
| `-valhalla-url` | `VALHALLA_URL` | `http://127.0.0.1:8002` |
| `-static-dir` | `WALKMAP_STATIC_DIR` | unset — `/` returns a placeholder until piece 3 adds the SPA |

The service does its own authentication for nothing: the edge sets
`X-Remote-User` and the listener must stay on loopback/the mesh only,
never exposed directly.

Endpoints:

- `GET /healthz` — checks the database (`select 1`) and Valhalla
  (`GET /status`); 200 if both are up, 503 otherwise.
- `GET /api/categories` — taxonomy categories present among visible rows,
  with a count per source.
- `GET /api/nearby?lat=&lon=&category=&minutes=&min_confidence=` —
  Valhalla isochrone, then a PostGIS `ST_Within` filter. `category` may
  repeat. Sorted by Valhalla walk time when the result set is small
  enough to ask for (`sorted_by: "walk_time"`), otherwise by straight-line
  distance (`sorted_by: "distance"`).
- `GET /api/search?q=&lat=&lon=&limit=` — `pg_trgm` name search.
- `GET /api/route?from_lat=&from_lon=&to_id=` — Valhalla pedestrian route
  to a place by id; decodes Valhalla's polyline6 shape into `[[lat,lon],...]`.
