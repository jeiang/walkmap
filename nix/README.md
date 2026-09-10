# walkmap NixOS module

`nixosModules.default` (`nix/module.nix`) declares `services.walkmap`.

## Options

| Option | Default | Notes |
|---|---|---|
| `enable` | `false` | |
| `port` | `8867` | |
| `listenAddress` | `"127.0.0.1"` | See "Exposure" below. |
| `dataDir` | `/var/lib/walkmap` | Import output, Valhalla tiles, basemap. Persist this. |
| `user` / `group` | `"walkmap"` / `"walkmap"` | |
| `package` | `packages.default` | The Go binary. |
| `frontendPackage` | `packages.frontend` | Built SPA, served at `/`. |
| `valhallaPackage` | `pkgs.valhalla` | Provides `valhalla_service`. |
| `valhallaPort` | `8002` | Loopback only. |
| `overtureRelease` | `"2026-08-19.0"` | Overture Maps release partition. |
| `overpassUrl` | `"https://overpass-api.de/api/interpreter"` | Matches `import/osm.sh`'s own default. |
| `osmRefresh` | `"*-*-* 03:30:00"` | `walkmap-import-osm.timer`'s `OnCalendar`. |
| `overtureRefresh` | `"monthly"` | `walkmap-import-overture.timer`'s `OnCalendar`. |

## Exposure

No `openFirewall` option is provided. `listenAddress` must be set by the
consumer to the mesh interface or `0.0.0.0`, with the consumer's own
firewall scoping who can reach `port` -- the same pattern as wger.

## First run

1. `walkmap-db-init.service` runs once at boot (superuser `CREATE EXTENSION
   postgis`/`pg_trgm`); `walkmap.service`'s `ExecStartPre` then runs
   `walkmap migrate` as the unprivileged `walkmap` role.
2. `walkmap.service` starts and serves `/healthz` as 503 -- there's no
   Valhalla config yet, so `walkmap-valhalla.service`
   (`ConditionPathExists`) doesn't start.
3. Run `walkmap-import-osm` and `walkmap-import-overture` by hand once
   (`systemctl start walkmap-import-osm`, then
   `systemctl start walkmap-import-overture`) rather than waiting for
   their timers, to see how long each takes on this host and watch for
   failures before trusting the schedule:
   - The first Overture pull is ~2 GB of parquet, pulled through DuckDB's
     bbox pushdown.
   - The basemap build does a one-time ~900 MB water-polygons shapefile
     fetch, cached under `dataDir/basemap/cache` (`TILEMAKER_CACHE`) and
     reused on every rebuild after.
4. `walkmap-import-osm` builds Valhalla tiles and restarts
   `walkmap-valhalla.service` on success (`try-restart`, so it's a no-op
   until tiles exist the first time). Once tiles exist and Valhalla is up,
   `/healthz` reports 200.
