#!/usr/bin/env bash
# Builds Valhalla routing tiles from a PBF into a versioned directory under
# <out-dir>/builds/<timestamp>, then swaps <out-dir>/current to point at it
# on success. A failed build leaves the previous `current` (and the tiles
# it points to) untouched, so a bad rebuild never takes down routing.
#
# Usage: valhalla/build-tiles.sh <pbf> <out-dir>
#
# Run this on Linux (nixpkgs' valhalla has no darwin build) e.g.:
#   nix shell nixpkgs#valhalla -c bash valhalla/build-tiles.sh data/barbados.osm.pbf data/valhalla
set -euo pipefail

pbf=$1
out_dir=$2

mkdir -p "$out_dir/builds"
ts=$(date -u +%Y%m%dT%H%M%SZ)
build_dir="$out_dir/builds/$ts"
mkdir -p "$build_dir"

config="$build_dir/valhalla.json"
valhalla_build_config \
	--mjolnir-tile-dir "$build_dir/tiles" \
	--mjolnir-tile-extract "$build_dir/tiles.tar" \
	--mjolnir-admin "$build_dir/admins.sqlite" \
	--mjolnir-timezone "$build_dir/tz.sqlite" \
	>"$config"

# Isochrones out to 90 minutes of walking (docs/WALKMAP.md, piece 2); the
# distance ceiling is a generous ~9 km at brisk walking pace so it never
# clips a legitimate 90-minute contour.
jq '.service_limits.isochrone.max_time_contour = 90
  | .service_limits.isochrone.max_distance_contour = 9000' \
	"$config" >"$config.tmp"
mv "$config.tmp" "$config"

# Admin polygons are used for timezone/country lookups but aren't required
# for pedestrian routing or isochrones; don't fail the whole build if the
# admin data source (Geofabrik/openstreetmap-data) is unreachable.
echo "building admins (optional)..." >&2
valhalla_build_admins -c "$config" "$pbf" || echo "warning: valhalla_build_admins failed, continuing without admin data" >&2

echo "building tiles..." >&2
valhalla_build_tiles -c "$config" "$pbf"

echo "building extract..." >&2
valhalla_build_extract -c "$config" -v

ln -sfn "$build_dir" "$out_dir/current.tmp"
mv -Tf "$out_dir/current.tmp" "$out_dir/current"

echo "built $build_dir, current -> $build_dir" >&2
