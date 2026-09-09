#!/usr/bin/env bash
# Fetches Barbados place data and the full-island road network from
# Overpass. Usage: import/osm.sh <output-dir>
#
# Writes two files into the output dir:
#   places.json       Overpass JSON (out center tags) for `walkmap import osm`
#   barbados.osm.pbf  full-island ways, for Valhalla (piece 2)
#
# OVERPASS_URL overrides the default public instance, e.g. if it rejects
# the full-island XML export as too large:
#   OVERPASS_URL=https://overpass.kumi.systems/api/interpreter import/osm.sh data
set -euo pipefail

out_dir=$1
overpass_url=${OVERPASS_URL:-https://overpass-api.de/api/interpreter}

mkdir -p "$out_dir"
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

places_query='[out:json][timeout:180];area["ISO3166-1"="BB"]->.a;(nwr["shop"](area.a);nwr["amenity"~"^(restaurant|cafe|fast_food|bar|pub|fuel|pharmacy|bank|atm)$"](area.a););out center tags;'
echo "fetching places..." >&2
curl -f -sS --data-urlencode "data=$places_query" "$overpass_url" -o "$tmp_dir/places.json"
mv "$tmp_dir/places.json" "$out_dir/places.json"

# `out meta` (not `out body`/`out skel`) is required here: osmium refuses
# to load Overpass XML without the metadata block it writes.
roads_query='[out:xml][timeout:900];area["ISO3166-1"="BB"]->.a;(way["highway"](area.a);>;);out meta;'
echo "fetching road network..." >&2
curl -f -sS --data-urlencode "data=$roads_query" "$overpass_url" -o "$tmp_dir/barbados.osm.xml"
osmium cat -o "$tmp_dir/barbados.osm.pbf" "$tmp_dir/barbados.osm.xml"
mv "$tmp_dir/barbados.osm.pbf" "$out_dir/barbados.osm.pbf"

echo "wrote $out_dir/places.json and $out_dir/barbados.osm.pbf" >&2
