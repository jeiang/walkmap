#!/usr/bin/env bash
# Fetches Overture Places within the Barbados bbox via DuckDB's parquet
# pushdown. Usage: import/overture.sh <output-dir> [release]
set -euo pipefail

out_dir=$1
release=${2:-2026-08-19.0}

mkdir -p "$out_dir"
tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT

duckdb -c "
INSTALL httpfs; LOAD httpfs;
INSTALL spatial; LOAD spatial;
SET s3_region='us-west-2';
COPY (
  SELECT
    id,
    names.primary AS name,
    categories.primary AS category,
    categories.alternate AS alt,
    confidence,
    sources[1].dataset AS src,
    bbox.xmin AS lon,
    bbox.ymin AS lat
  FROM read_parquet('s3://overturemaps-us-west-2/release/${release}/theme=places/type=place/*', filename=true, hive_partitioning=1)
  WHERE bbox.xmin BETWEEN -59.70 AND -59.38
    AND bbox.ymin BETWEEN 13.02 AND 13.36
) TO '${tmp_dir}/overture.csv' (HEADER);
"
mv "$tmp_dir/overture.csv" "$out_dir/overture.csv"
echo "wrote $out_dir/overture.csv" >&2
