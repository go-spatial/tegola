#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../.." && pwd)"
data_dir="${OSM_DATA_DIR:-${repo_root}/deploy/maps/data}"
pg_host="${PGHOST:-127.0.0.1}"
pg_port="${PGPORT:-55432}"
pg_database="${PGDATABASE:-gis}"
pg_user="${PGUSER:-postgres}"
pg_password="${PGPASSWORD:-postgres}"
pg_sslmode="${PGSSLMODE:-disable}"
osm2pgsql_image="${OSM2PGSQL_IMAGE:-iboates/osm2pgsql:latest}"
postgres_image="${POSTGRES_IMAGE:-postgres:16-alpine}"
import_mode="${OSM_IMPORT_MODE:-auto}"
docker_network="${OSM_IMPORT_DOCKER_NETWORK:-safari-local-dev_default}"
gis_schema="${GIS_SCHEMA:-gis}"

if ! [[ "${gis_schema}" =~ ^[a-z][a-z0-9_]*$ ]]; then
  echo "GIS_SCHEMA must be a lowercase PostgreSQL identifier" >&2
  exit 64
fi

default_countries="tanzania kenya uganda rwanda burundi"
read -r -a countries <<< "${OSM_COUNTRIES:-${default_countries}}"
drop_middle_tables="${OSM2PGSQL_DROP_MIDDLE_TABLES:-true}"

mkdir -p "${data_dir}"

use_docker=false
if [ "${import_mode}" = "docker" ]; then
  use_docker=true
elif [ "${import_mode}" = "auto" ]; then
  if ! command -v osm2pgsql >/dev/null 2>&1 || ! command -v psql >/dev/null 2>&1; then
    use_docker=true
  fi
fi

if [ "${use_docker}" = true ]; then
  pg_host="${PGHOST:-maps-postgis}"
  pg_port="${PGPORT:-5432}"
fi

run_psql() {
  if [ "${use_docker}" = true ]; then
    docker run --rm --network="${docker_network}" \
      -e PGPASSWORD="${pg_password}" \
      -e PGSSLMODE="${pg_sslmode}" \
      "${postgres_image}" \
      psql \
        -h "${pg_host}" \
        -p "${pg_port}" \
        -U "${pg_user}" \
        -d "${pg_database}" \
        -v ON_ERROR_STOP=1 \
        "$@"
    return
  fi

  PGPASSWORD="${pg_password}" PGSSLMODE="${pg_sslmode}" \
    psql \
      -h "${pg_host}" \
      -p "${pg_port}" \
      -U "${pg_user}" \
      -d "${pg_database}" \
      -v ON_ERROR_STOP=1 \
      "$@"
}

run_osm2pgsql() {
  local mode="$1"
  local pbf_path="$2"
  shift 2

  if [ "${use_docker}" = true ]; then
    docker run --rm --network="${docker_network}" \
      -e PGPASSWORD="${pg_password}" \
      -e PGSSLMODE="${pg_sslmode}" \
      -v "${data_dir}:/data:ro" \
      -v "${script_dir}:/maps:ro" \
      "${osm2pgsql_image}" \
      osm2pgsql \
        --output=flex \
        --style=/maps/protected_areas.lua \
        --schema="${gis_schema}" \
        --slim \
        "$@" \
        "${mode}" \
        --host="${pg_host}" \
        --port="${pg_port}" \
        --database="${pg_database}" \
        --username="${pg_user}" \
        "/data/$(basename "${pbf_path}")"
    return
  fi

  PGPASSWORD="${pg_password}" PGSSLMODE="${pg_sslmode}" \
    osm2pgsql \
      --output=flex \
      --style="${script_dir}/protected_areas.lua" \
      --schema="${gis_schema}" \
      --slim \
      "$@" \
      "${mode}" \
      --host="${pg_host}" \
      --port="${pg_port}" \
      --database="${pg_database}" \
      --username="${pg_user}" \
      "${pbf_path}"
}

run_psql <<SQL
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE SCHEMA IF NOT EXISTS "${gis_schema}";
SQL

first_import=true
last_country_index=$((${#countries[@]} - 1))
for country_index in "${!countries[@]}"; do
  country="${countries[${country_index}]}"
  pbf_path="${data_dir}/${country}-latest.osm.pbf"
  if [ ! -f "${pbf_path}" ]; then
    url="https://download.geofabrik.de/africa/${country}-latest.osm.pbf"
    echo "Downloading ${url}"
    tmp_pbf_path="${pbf_path}.download"
    rm -f "${tmp_pbf_path}"
    curl -fL "${url}" -o "${tmp_pbf_path}"
    mv "${tmp_pbf_path}" "${pbf_path}"
  fi

  if [ "${first_import}" = true ]; then
    mode="--create"
    first_import=false
  else
    mode="--append"
  fi

  drop_args=()
  if [ "${drop_middle_tables}" = "true" ] && [ "${country_index}" -eq "${last_country_index}" ]; then
    drop_args=(--drop)
  fi

  echo "Importing protected areas from ${country}"
  run_osm2pgsql "${mode}" "${pbf_path}" "${drop_args[@]}"
done

run_psql <<SQL
ALTER TABLE "${gis_schema}".protected_areas
  ALTER COLUMN geom TYPE geometry(MultiPolygon, 4326)
  USING ST_Multi(ST_CollectionExtract(ST_MakeValid(geom), 3));

DELETE FROM "${gis_schema}".protected_areas
WHERE geom IS NULL OR ST_IsEmpty(geom);

CREATE INDEX IF NOT EXISTS protected_areas_geom_idx
  ON "${gis_schema}".protected_areas
  USING GIST (geom);

CREATE INDEX IF NOT EXISTS protected_areas_name_idx
  ON "${gis_schema}".protected_areas (name);

ANALYZE "${gis_schema}".protected_areas;

SELECT count(*) AS protected_area_count FROM "${gis_schema}".protected_areas;
SQL
