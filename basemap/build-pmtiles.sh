#!/usr/bin/env bash
# Builds basemap.pmtiles from an OSM PBF with tilemaker's bundled
# OpenMapTiles-compatible config/process (docs/WALKMAP.md, piece 3).
#
# Usage: basemap/build-pmtiles.sh <pbf> <out-dir> [bbox]
#
# bbox (minlon,minlat,maxlon,maxlat) keeps only that region, e.g. Barbados:
#   -59.70,13.02,-59.38,13.36
# Omit it to tile the pbf's full extent.
#
# The OMT "ocean" layer wants coastline/water_polygons.shp -- the OSM
# water-polygons shapefile (osmdata.openstreetmap.de, ~900MB zipped). It's
# fetched once into $TILEMAKER_CACHE (default <out-dir>/cache) and reused
# on rebuilds. Set TILEMAKER_SKIP_COASTLINE=1 to skip the fetch outright;
# a failed or skipped fetch doesn't fail the build -- tilemaker just warns
# and emits no ocean polygons.
set -euo pipefail

pbf=$1
out_dir=$2
bbox=${3:-}

# Resolve to absolute paths up front: the tilemaker invocation below runs
# from inside TILEMAKER_CACHE so its config's relative shapefile "source"
# paths (coastline/water_polygons.shp) resolve correctly.
pbf=$(cd "$(dirname "$pbf")" && pwd)/$(basename "$pbf")
mkdir -p "$out_dir"
out_dir=$(cd "$out_dir" && pwd)

: "${TILEMAKER_CACHE:=$out_dir/cache}"
mkdir -p "$TILEMAKER_CACHE"

tilemaker_bin=$(command -v tilemaker)
share_dir=$(dirname "$(dirname "$tilemaker_bin")")/share/tilemaker
config="$share_dir/config-openmaptiles.json"
process="$share_dir/process-openmaptiles.lua"

coastline_shp="$TILEMAKER_CACHE/coastline/water_polygons.shp"
if [ ! -f "$coastline_shp" ] && [ -z "${TILEMAKER_SKIP_COASTLINE:-}" ]; then
	echo "fetching OSM water-polygons shapefile into $TILEMAKER_CACHE (~900MB, once)..." >&2
	zip="$TILEMAKER_CACHE/water-polygons-split-4326.zip"
	if curl -fSL --connect-timeout 10 -o "$zip" \
		https://osmdata.openstreetmap.de/download/water-polygons-split-4326.zip; then
		rm -rf "$TILEMAKER_CACHE/coastline"
		(cd "$TILEMAKER_CACHE" && unzip -q -o "$(basename "$zip")" && mv water-polygons-split-4326 coastline)
		rm -f "$zip"
	else
		echo "warning: coastline download failed; building without ocean fill" >&2
	fi
fi

# tilemaker picks its output format from the file extension, so the temp
# name must still end in .pmtiles (otherwise it silently writes a z/x/y.pbf
# directory tree instead of a single archive).
out_tmp="$out_dir/.basemap.pmtiles.building.pmtiles"
args=(--input "$pbf" --config "$config" --process "$process" --output "$out_tmp")
[ -n "$bbox" ] && args+=(--bbox "$bbox")

rm -f "$out_tmp"
(cd "$TILEMAKER_CACHE" && "$tilemaker_bin" "${args[@]}")
mv "$out_tmp" "$out_dir/basemap.pmtiles"

size=$(du -h "$out_dir/basemap.pmtiles" | cut -f1)
if [ -f "$coastline_shp" ]; then
	echo "built $out_dir/basemap.pmtiles ($size, with ocean fill)" >&2
else
	echo "built $out_dir/basemap.pmtiles ($size, NO ocean fill -- coastline shapefile not present)" >&2
fi
