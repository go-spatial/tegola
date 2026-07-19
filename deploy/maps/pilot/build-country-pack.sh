#!/usr/bin/env bash
set -euo pipefail

# Builds one independent protected-area PMTiles archive. Every pack uses a
# fresh PostGIS schema in an ephemeral database so an import can never leak
# geometries from another country into the archive.

if [ "$#" -ne 2 ]; then
  echo "usage: $0 RELEASE_DIR ISO2" >&2
  exit 64
fi

release_dir="$1"
iso2="$(printf '%s' "$2" | tr '[:lower:]' '[:upper:]')"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
lock_file="${MAP_RELEASE_LOCK:-${script_dir}/release-inputs.lock.json}"
country_file="${script_dir}/countries.json"
style_file="${script_dir}/../protected_areas.lua"

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "required command is unavailable: $1" >&2
    exit 69
  }
}

for command in curl docker jq md5sum sha256sum; do
  require_command "${command}"
done

if ! jq -e --arg iso2 "${iso2}" '.[$iso2]' "${country_file}" >/dev/null; then
  echo "${iso2} is not an active protected-area country" >&2
  exit 64
fi

lock() {
  jq -er "$@" "${lock_file}"
}

slug="$(lock --arg iso2 "${iso2}" '.geofabrik.inputs[$iso2].slug')"
expected_md5="$(lock --arg iso2 "${iso2}" '.geofabrik.inputs[$iso2].md5')"
geofabrik_base_url="$(lock '.geofabrik.baseUrl')"
osm2pgsql_image="$(lock '.tools.osm2pgsqlImage')"
postgis_image="$(lock '.tools.postgisImage')"
gdal_image="$(lock '.countryBoundaries.gdalImage')"
pmtiles_image="$(lock '.protomaps.pmtilesCliImage')"
park_pattern="$(jq -er --arg iso2 "${iso2}" '.[$iso2].parkPattern' "${country_file}")"
one_of_park_pattern="$(jq -r --arg iso2 "${iso2}" '.[$iso2].oneOfParkPattern // empty' "${country_file}")"

inputs_dir="${release_dir}/inputs"
output_dir="${release_dir}/protected-areas"
mkdir -p "${inputs_dir}" "${output_dir}"
pbf_file="${inputs_dir}/${slug}-latest.osm.pbf"
pbf_url="${geofabrik_base_url}/${slug}-latest.osm.pbf"

curl --fail --location --retry 3 --retry-all-errors --silent --show-error "${pbf_url}" --output "${pbf_file}"
actual_md5="$(md5sum "${pbf_file}" | awk '{print $1}')"
if [ "${actual_md5}" != "${expected_md5}" ]; then
  echo "Geofabrik input changed for ${iso2}; refresh the release input lock deliberately" >&2
  exit 65
fi

run_id="${iso2,,}-${RANDOM}-${RANDOM}"
network="maps-pilot-${run_id}"
database="maps-pilot-${run_id}"
schema="pilot_${iso2,,}"
postgres_password="maps-pilot"
started_database=false
started_network=false

cleanup() {
  if [ "${started_database}" = true ]; then
    docker rm --force "${database}" >/dev/null 2>&1 || true
  fi
  if [ "${started_network}" = true ]; then
    docker network rm "${network}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

docker network create "${network}" >/dev/null
started_network=true
docker run --detach --rm --name "${database}" --network "${network}" \
  -e POSTGRES_DB=gis \
  -e POSTGRES_PASSWORD="${postgres_password}" \
  -e POSTGRES_USER=postgres \
  "${postgis_image}" >/dev/null
started_database=true

for _ in $(seq 1 60); do
  if docker exec "${database}" pg_isready -U postgres -d gis >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if ! docker exec "${database}" pg_isready -U postgres -d gis >/dev/null 2>&1; then
  echo "temporary PostGIS did not become ready for ${iso2}" >&2
  exit 70
fi

psql() {
  docker exec -e PGPASSWORD="${postgres_password}" "${database}" \
    psql -U postgres -d gis -v ON_ERROR_STOP=1 "$@"
}

psql -c 'CREATE EXTENSION IF NOT EXISTS postgis;'
psql -c "CREATE SCHEMA \"${schema}\";"

docker run --rm --network "${network}" \
  -e PGPASSWORD="${postgres_password}" \
  -v "${release_dir}:/work:ro" \
  -v "${style_file}:/maps/protected_areas.lua:ro" \
  "${osm2pgsql_image}" \
  osm2pgsql \
    --output=flex \
    --style=/maps/protected_areas.lua \
    --schema="${schema}" \
    --slim \
    --drop \
    --create \
    --host="${database}" \
    --port=5432 \
    --database=gis \
    --username=postgres \
    /work/inputs/"$(basename "${pbf_file}")"

psql <<SQL
ALTER TABLE "${schema}".protected_areas
  ADD COLUMN country_code text NOT NULL DEFAULT '${iso2}';
ALTER TABLE "${schema}".protected_areas
  ALTER COLUMN geom TYPE geometry(MultiPolygon, 4326)
  USING ST_Multi(ST_CollectionExtract(ST_MakeValid(geom), 3));
DELETE FROM "${schema}".protected_areas
WHERE geom IS NULL OR ST_IsEmpty(geom);
CREATE INDEX protected_areas_geom_idx ON "${schema}".protected_areas USING GIST (geom);
ANALYZE "${schema}".protected_areas;
SQL

area_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".protected_areas;")"
if [ "${area_count}" -eq 0 ]; then
  echo "${iso2} protected-area import is empty" >&2
  exit 65
fi

has_named_area() {
  local pattern="$1"
  psql -Atc "SELECT EXISTS (SELECT 1 FROM \"${schema}\".protected_areas WHERE coalesce(name, '') ~* '${pattern}');"
}

if [ "$(has_named_area "${park_pattern}")" != "t" ]; then
  echo "${iso2} is missing expected park pattern: ${park_pattern}" >&2
  exit 65
fi
if [ -n "${one_of_park_pattern}" ] && [ "$(has_named_area "${one_of_park_pattern}")" != "t" ]; then
  echo "${iso2} is missing all required Uganda fallback parks: ${one_of_park_pattern}" >&2
  exit 65
fi

bounds="$(psql -Atc "WITH extent AS (SELECT ST_Extent(geom)::box2d AS box FROM \"${schema}\".protected_areas) SELECT json_build_array(ST_XMin(box), ST_YMin(box), ST_XMax(box), ST_YMax(box)) FROM extent;")"
if [ -z "${bounds}" ] || [ "${bounds}" = "null" ]; then
  echo "${iso2} protected-area bounds could not be determined" >&2
  exit 65
fi

output_file="${output_dir}/${iso2}.pmtiles"
pg_connection="PG:host=${database} port=5432 dbname=gis user=postgres password=${postgres_password}"
polygon_sql="SELECT abs(area_id)::bigint AS id, osm_id, name, boundary, leisure, protect_class, protection_title, operator, designation, country_code, geom FROM \"${schema}\".protected_areas"
label_sql="SELECT min(area_id)::bigint AS id, name, country_code, ST_PointOnSurface(ST_UnaryUnion(ST_Collect(geom))) AS geom FROM \"${schema}\".protected_areas WHERE name IS NOT NULL AND btrim(name) <> '' GROUP BY name, country_code"

docker run --rm --network "${network}" -v "${release_dir}:/work" "${gdal_image}" \
  ogr2ogr -f PMTiles \
    -dsco NAME="${iso2} protected areas" \
    -dsco DESCRIPTION="${iso2} protected areas from OpenStreetMap" \
    -dsco TYPE=overlay \
    -dsco MINZOOM=4 \
    -dsco MAXZOOM=14 \
    -nln protected_areas \
    -lco MINZOOM=4 \
    -lco MAXZOOM=14 \
    /work/protected-areas/"$(basename "${output_file}")" \
    "${pg_connection}" \
    -sql "${polygon_sql}"

docker run --rm --network "${network}" -v "${release_dir}:/work" "${gdal_image}" \
  ogr2ogr -f PMTiles -update -append \
    -nln protected_area_labels \
    -lco MINZOOM=4 \
    -lco MAXZOOM=14 \
    /work/protected-areas/"$(basename "${output_file}")" \
    "${pg_connection}" \
    -sql "${label_sql}"

docker run --rm -v "${release_dir}:/work" "${pmtiles_image}" \
  verify /work/protected-areas/"$(basename "${output_file}")"

sha256="$(sha256sum "${output_file}" | awk '{print $1}')"
jq -n \
  --arg iso2 "${iso2}" \
  --arg path "protected-areas/${iso2}.pmtiles" \
  --arg sha256 "${sha256}" \
  --argjson bounds "${bounds}" \
  --argjson featureCount "${area_count}" \
  '{iso2: $iso2, path: $path, sha256: $sha256, bounds: $bounds, minZoom: 4, maxZoom: 14, featureCount: $featureCount}' \
  >"${output_dir}/${iso2}.metadata.json"

echo "built ${iso2}.pmtiles with ${area_count} protected areas"
