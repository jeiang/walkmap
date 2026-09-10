# walkmap

Category search by walking time for Barbados. This repo currently holds
pieces 1-4 of the plan: the PostGIS schema and OSM/Overture import
pipeline, the Valhalla tiles + Go API service, the basemap build +
MapLibre SPA, and the flake's `packages.default`/`packages.frontend` plus
a `nixosModules.default` for `services.walkmap` (see `nix/README.md`).
See `docs/WALKMAP.md` in the `cornn-flaek` flake repo for the full
design.

## Requirements

`nix develop` provides everything: Go, PostgreSQL 17 + PostGIS, DuckDB,
osmium-tool, curl, jq, tilemaker, pmtiles, Node 24 (with npm).

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
| `-static-dir` | `WALKMAP_STATIC_DIR` | unset — `/` returns a placeholder if not set |
| `-basemap` | `WALKMAP_BASEMAP` | unset — `/basemap.pmtiles` 404s if not set |

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
- `GET /basemap.pmtiles` — serves the file at `WALKMAP_BASEMAP` with Range
  support (`http.ServeFile`), needed by the PMTiles JS client. Lives
  outside `WALKMAP_STATIC_DIR`: it's data, rebuilt nightly, not part of
  the SPA build.

## Basemap

```
just build-basemap   # -> data/basemap/basemap.pmtiles, from data/barbados.osm.pbf
```

`basemap/build-pmtiles.sh <pbf> <out-dir> [bbox]` runs tilemaker with its
bundled OpenMapTiles-compatible config/process over the pbf. The OMT
"ocean" layer wants the OSM water-polygons shapefile
(osmdata.openstreetmap.de, ~900MB zipped); it's fetched once into
`$TILEMAKER_CACHE` (default `<out-dir>/cache`) and reused on rebuilds. Set
`TILEMAKER_SKIP_COASTLINE=1` to skip the fetch outright — a failed or
skipped fetch doesn't fail the build, it just omits ocean fill.
`bbox` (`minlon,minlat,maxlon,maxlat`) restricts the tileset to a region;
tilemaker also requires it whenever a shapefile source is in play (i.e.
whenever the coastline cache is present), so pass Barbados's bbox
(`-59.70,13.02,-59.38,13.36`) for a real build.

## Frontend

`frontend/` is a vite + vanilla TypeScript SPA (MapLibre GL JS + the
PMTiles protocol). No runtime requests to third-party hosts: the
OpenMapTiles-schema style, its sprite, and Latin-range glyph PBFs are
vendored under `frontend/public/style/` (see `LICENSE` there) and the
basemap comes from this server's own `/basemap.pmtiles`.

```
just build-frontend   # npm install && npm run build -> frontend/dist
just dev               # vite dev server on :5173, proxying /api and
                        # /basemap.pmtiles to a `just serve` on 127.0.0.1:8867
```

`frontend/dist` is a flat static directory, servable via
`WALKMAP_STATIC_DIR` and meant to be `embed`-ded into the Go binary in
piece 4.
