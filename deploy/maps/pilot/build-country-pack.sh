#!/usr/bin/env bash
set -euo pipefail

# Builds one independent country tourism PMTiles archive from OSM, using an ephemeral PostGIS schema and optional Natural Earth clipping for combined or shared source extracts.

if [ "$#" -ne 2 ]; then
  echo "usage: $0 RELEASE_DIR ISO2" >&2
  exit 64
fi

release_dir="$1"
iso2="$(printf '%s' "$2" | tr '[:lower:]' '[:upper:]')"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -W 2>/dev/null || pwd)"
lock_file="${MAP_RELEASE_LOCK:-${script_dir}/release-inputs.lock.json}"
country_file="${MAP_COUNTRIES_FILE:-${script_dir}/countries.json}"
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

country_pack_maxzoom="$(lock '.coverage.countryPackMaxZoom // 12')"

source_key="$(jq -er --arg iso2 "${iso2}" '.[$iso2].source // $iso2' "${country_file}")"
source_key="${source_key//$'\r'/}"
local_pbf="$(jq -r --arg iso2 "${iso2}" '.[$iso2].localPbf // empty' "${country_file}")"
if [ -z "${local_pbf}" ]; then
  slug="$(lock --arg source "${source_key}" '.geofabrik.inputs[$source].slug')"
  expected_md5="$(lock --arg source "${source_key}" '.geofabrik.inputs[$source].md5')"
  pbf_url="$(lock --arg source "${source_key}" '.geofabrik.inputs[$source].url')"
  slug="${slug//$'\r'/}"
  expected_md5="${expected_md5//$'\r'/}"
  pbf_url="${pbf_url//$'\r'/}"
fi
osm2pgsql_image="$(lock '.tools.osm2pgsqlImage')"
postgis_image="$(lock '.tools.postgisImage')"
gdal_image="$(lock '.countryBoundaries.gdalImage')"
pmtiles_image="$(lock '.protomaps.pmtilesCliImage')"
park_pattern="$(jq -r --arg iso2 "${iso2}" '.[$iso2].parkPattern // empty' "${country_file}")"
one_of_park_pattern="$(jq -r --arg iso2 "${iso2}" '.[$iso2].oneOfParkPattern // empty' "${country_file}")"
clip_iso2="$(jq -r --arg iso2 "${iso2}" '.[$iso2].clipIso2 // empty' "${country_file}")"
import_bbox="$(jq -r --arg iso2 "${iso2}" '.[$iso2].importBbox // empty' "${country_file}")"

inputs_dir="${release_dir}/inputs"
output_dir="${release_dir}/protected-areas"
mkdir -p "${inputs_dir}" "${output_dir}"
if [ -n "${local_pbf}" ]; then
  pbf_file="${inputs_dir}/${local_pbf}"
  if [ ! -f "${pbf_file}" ]; then echo "local PBF is missing for ${iso2}: ${pbf_file}" >&2; exit 65; fi
else
  pbf_file="${inputs_dir}/${slug}-latest.osm.pbf"
  if [ ! -f "${pbf_file}" ]; then
  curl --fail --location --retry 3 --retry-all-errors --silent --show-error "${pbf_url}" --output "${pbf_file}"
  fi
fi
actual_md5="$(md5sum "${pbf_file}" | awk '{print $1}')"
if [ -z "${local_pbf}" ] && [ "${actual_md5}" != "${expected_md5}" ]; then
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
  if docker exec "${database}" pg_isready -h 127.0.0.1 -U postgres -d gis >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if ! docker exec "${database}" pg_isready -h 127.0.0.1 -U postgres -d gis >/dev/null 2>&1; then
  echo "temporary PostGIS did not become ready for ${iso2}" >&2
  exit 70
fi

psql() {
  docker exec -e PGPASSWORD="${postgres_password}" "${database}" \
    psql -U postgres -d gis -v ON_ERROR_STOP=1 "$@"
}

psql -c 'CREATE EXTENSION IF NOT EXISTS postgis;'
psql -c "CREATE SCHEMA \"${schema}\";"

import_args=()
if [ -n "${import_bbox}" ]; then import_args+=(--bbox "${import_bbox}"); fi
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
    "${import_args[@]}" \
    /work/inputs/"$(basename "${pbf_file}")"

for table in protected_areas tourism_pois tourism_areas heritage_pois heritage_areas outdoor_routes water_sport_pois; do
  psql -c "ALTER TABLE \"${schema}\".${table} ADD COLUMN country_code text NOT NULL DEFAULT '${iso2}';"
  psql -c "CREATE INDEX IF NOT EXISTS ${table}_geom_idx ON \"${schema}\".${table} USING GIST (geom);"
  psql -c "ANALYZE \"${schema}\".${table};"
done
psql -c "ALTER TABLE \"${schema}\".protected_areas ALTER COLUMN geom TYPE geometry(MultiPolygon, 4326) USING ST_Multi(ST_CollectionExtract(ST_MakeValid(geom), 3));"
psql -c "ALTER TABLE \"${schema}\".tourism_areas ALTER COLUMN geom TYPE geometry(MultiPolygon, 4326) USING ST_Multi(ST_CollectionExtract(ST_MakeValid(geom), 3));"
psql -c "ALTER TABLE \"${schema}\".heritage_areas ALTER COLUMN geom TYPE geometry(MultiPolygon, 4326) USING ST_Multi(ST_CollectionExtract(ST_MakeValid(geom), 3));"
psql -c "DELETE FROM \"${schema}\".protected_areas WHERE geom IS NULL OR ST_IsEmpty(geom);"
psql -c "DELETE FROM \"${schema}\".tourism_areas WHERE geom IS NULL OR ST_IsEmpty(geom);"
psql -c "DELETE FROM \"${schema}\".heritage_areas WHERE geom IS NULL OR ST_IsEmpty(geom);"
for table in protected_areas tourism_pois tourism_areas heritage_pois heritage_areas outdoor_routes water_sport_pois; do
  invalid_geometry_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".${table} WHERE geom IS NULL OR ST_IsEmpty(geom) OR ST_SRID(geom) <> 4326 OR NOT ST_IsValid(geom);")"
  if [ "${invalid_geometry_count}" -ne 0 ]; then
    echo "${iso2} ${table} failed geometry quality gate: ${invalid_geometry_count} invalid features" >&2
    exit 65
  fi
done

clip_join=""
area_geom_expr="a.geom"
if [ -n "${clip_iso2}" ]; then
  country_archive="${inputs_dir}/ne_10m_admin_0_countries.zip"
  country_url="$(lock '.countryBoundaries.url')"
  country_sha256="$(lock '.countryBoundaries.sha256')"
  if [ ! -f "${country_archive}" ]; then
    curl --fail --location --retry 3 --retry-all-errors --silent --show-error "${country_url}" --output "${country_archive}"
  fi
  actual_country_sha256="$(sha256sum "${country_archive}" | awk '{print $1}')"
  if [ "${actual_country_sha256}" != "${country_sha256}" ]; then
    echo "Natural Earth boundary checksum mismatch" >&2
    exit 65
  fi
  clip_dir="${release_dir}/clips"
  clip_file="${clip_dir}/${iso2}.geojson"
  mkdir -p "${clip_dir}"
  docker run --rm -v "${release_dir}:/work" "${gdal_image}" \
    ogr2ogr -f GeoJSON /work/clips/"$(basename "${clip_file}")" \
    /vsizip/work/inputs/"$(basename "${country_archive}")"/ne_10m_admin_0_countries.shp \
    -dialect sqlite -sql "SELECT * FROM ne_10m_admin_0_countries WHERE ISO_A2 = '${clip_iso2}' OR ISO_A2_EH = '${clip_iso2}'"
  clip_count="$(jq '.features | length' "${clip_file}")"
  if [ "${clip_count}" -ne 1 ]; then
    echo "expected one Natural Earth clipping feature for ${iso2}/${clip_iso2}, got ${clip_count}" >&2
    exit 65
  fi
  docker run --rm --network "${network}" -v "${release_dir}:/work:ro" "${gdal_image}" \
    ogr2ogr -f PostgreSQL "PG:host=${database} port=5432 dbname=gis user=postgres password=${postgres_password}" \
    /work/clips/"$(basename "${clip_file}")" -nln "${schema}.clip_boundary" -nlt PROMOTE_TO_MULTI
  clip_join="JOIN \"${schema}\".clip_boundary c ON ST_Intersects(a.geom, c.wkb_geometry)"
  area_geom_expr="ST_CollectionExtract(ST_Intersection(a.geom, c.wkb_geometry), 3)"
fi

area_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".protected_areas a ${clip_join};")"
# Keep this policy identical for source counts, bounds, SQL export, and the
# staged-artifact check. NAME is replaced with the relevant SQL column.
tourism_name_predicate_template="btrim(coalesce(NAME, '')) <> '' AND lower(btrim(NAME)) NOT IN ('unnamed', 'unknown', 'n/a', 'na', 'null', 'none', 'untitled', 'no name', 'no_name') AND ((btrim(NAME) !~ '^[+-]?[0-9]+([.][0-9]+)?$' AND btrim(NAME) !~ '^[+-]?[0-9]+[A-Za-z]?$' AND btrim(NAME) !~ '^[+][0-9]' AND btrim(NAME) !~ '^-[0-9]') OR (coalesce(FEATURE_CLASS, '') = 'visitor_information' AND coalesce(TOURISM, '') = 'information' AND btrim(NAME) ~ '^[0-9]+[A-Za-z]?$'))"
tourism_name_predicate="${tourism_name_predicate_template//NAME/a.name}"
tourism_name_predicate="${tourism_name_predicate//FEATURE_CLASS/a.feature_class}"
tourism_name_predicate="${tourism_name_predicate//TOURISM/a.tourism}"
generic_name_predicate="btrim(coalesce(NAME, '')) <> '' AND lower(btrim(NAME)) NOT IN ('unnamed', 'unknown', 'n/a', 'na', 'null', 'none', 'untitled', 'no name', 'no_name') AND btrim(NAME) !~ '^[+-]?[0-9]+([.][0-9]+)?$' AND btrim(NAME) !~ '^[+-]?[0-9]+[A-Za-z]?$' AND btrim(NAME) !~ '^[+][0-9]' AND btrim(NAME) !~ '^-[0-9]'"
tourism_point_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".tourism_pois a ${clip_join} WHERE ${tourism_name_predicate};" )"
tourism_point_dropped_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".tourism_pois a ${clip_join} WHERE NOT (${tourism_name_predicate});" )"
tourism_area_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".tourism_areas a ${clip_join} WHERE ${generic_name_predicate//NAME/a.name};")"
heritage_name_predicate="${generic_name_predicate//NAME/a.name}"
route_name_predicate="${generic_name_predicate//NAME/a.name}"
water_name_predicate="${generic_name_predicate//NAME/a.name}"
heritage_point_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".heritage_pois a ${clip_join} WHERE ${heritage_name_predicate};")"
heritage_point_dropped_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".heritage_pois a ${clip_join} WHERE NOT (${heritage_name_predicate});")"
heritage_area_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".heritage_areas a ${clip_join} WHERE ${generic_name_predicate//NAME/a.name};")"
route_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".outdoor_routes a WHERE ${route_name_predicate};")"
route_dropped_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".outdoor_routes a WHERE NOT (${route_name_predicate});")"
water_point_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".water_sport_pois a ${clip_join} WHERE ${water_name_predicate};")"
water_point_dropped_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".water_sport_pois a ${clip_join} WHERE NOT (${water_name_predicate});")"

# Canonicalize duplicate named point features before export. Matching requires
# a normalized name and a 100 m proximity; nearby facilities with different
# names remain distinct. Heritage wins over generic tourism, and tourism wins
# over water-sport duplicates. The dropped total is recorded in release metadata.
pre_dedup_point_count=$((heritage_point_count + tourism_point_count + water_point_count))
psql -c "DELETE FROM \"${schema}\".tourism_pois t USING \"${schema}\".heritage_pois h WHERE lower(regexp_replace(btrim(t.name), '[^[:alnum:]]+', '', 'g')) = lower(regexp_replace(btrim(h.name), '[^[:alnum:]]+', '', 'g')) AND ST_DWithin(t.geom::geography, h.geom::geography, 100) AND ${tourism_name_predicate//a./t.} AND ${heritage_name_predicate//a./h.};" >/dev/null
psql -c "DELETE FROM \"${schema}\".water_sport_pois w USING \"${schema}\".heritage_pois h WHERE lower(regexp_replace(btrim(w.name), '[^[:alnum:]]+', '', 'g')) = lower(regexp_replace(btrim(h.name), '[^[:alnum:]]+', '', 'g')) AND ST_DWithin(w.geom::geography, h.geom::geography, 100) AND ${water_name_predicate//a./w.} AND ${heritage_name_predicate//a./h.};" >/dev/null
psql -c "DELETE FROM \"${schema}\".water_sport_pois w USING \"${schema}\".tourism_pois t WHERE lower(regexp_replace(btrim(w.name), '[^[:alnum:]]+', '', 'g')) = lower(regexp_replace(btrim(t.name), '[^[:alnum:]]+', '', 'g')) AND ST_DWithin(w.geom::geography, t.geom::geography, 100) AND ${water_name_predicate//a./w.} AND ${tourism_name_predicate//a./t.};" >/dev/null
tourism_point_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".tourism_pois a ${clip_join} WHERE ${tourism_name_predicate};" )"
tourism_point_dropped_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".tourism_pois a ${clip_join} WHERE NOT (${tourism_name_predicate});" )"
heritage_point_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".heritage_pois a ${clip_join} WHERE ${heritage_name_predicate};")"
water_point_count="$(psql -Atc "SELECT count(*) FROM \"${schema}\".water_sport_pois a ${clip_join} WHERE ${water_name_predicate};")"
post_dedup_point_count=$((heritage_point_count + tourism_point_count + water_point_count))
dedup_point_dropped_count=$((pre_dedup_point_count - post_dedup_point_count))
if [ "${area_count}" -eq 0 ] && [ "${tourism_point_count}" -eq 0 ] && [ "${tourism_area_count}" -eq 0 ] && [ "${heritage_point_count}" -eq 0 ] && [ "${heritage_area_count}" -eq 0 ] && [ "${route_count}" -eq 0 ] && [ "${water_point_count}" -eq 0 ]; then
  echo "${iso2} country pack contains neither protected-area nor tourism features" >&2
  exit 65
fi

has_named_area() {
  local pattern="$1"
  psql -Atc "SELECT EXISTS (SELECT 1 FROM \"${schema}\".protected_areas WHERE coalesce(name, '') ~* '${pattern}');"
}

if [ "${source_key}" = "${iso2}" ] && [ -n "${park_pattern}" ] && [ "$(has_named_area "${park_pattern}")" != "t" ]; then
  echo "${iso2} is missing expected park pattern: ${park_pattern}" >&2
  exit 65
fi
if [ "${source_key}" = "${iso2}" ] && [ -n "${one_of_park_pattern}" ] && [ "$(has_named_area "${one_of_park_pattern}")" != "t" ]; then
  echo "${iso2} is missing all required Uganda fallback parks: ${one_of_park_pattern}" >&2
  exit 65
fi

bounds="$(psql -Atc "WITH geometries AS (SELECT a.geom FROM \"${schema}\".protected_areas a ${clip_join} UNION ALL SELECT a.geom FROM \"${schema}\".tourism_areas a ${clip_join} UNION ALL SELECT a.geom FROM \"${schema}\".tourism_pois a ${clip_join} WHERE ${tourism_name_predicate} UNION ALL SELECT a.geom FROM \"${schema}\".heritage_areas a ${clip_join} UNION ALL SELECT a.geom FROM \"${schema}\".heritage_pois a ${clip_join} WHERE ${heritage_name_predicate} UNION ALL SELECT a.geom FROM \"${schema}\".outdoor_routes a WHERE ${route_name_predicate} UNION ALL SELECT a.geom FROM \"${schema}\".water_sport_pois a ${clip_join} WHERE ${water_name_predicate}), extent AS (SELECT ST_Extent(geom)::box2d AS box FROM geometries) SELECT json_build_array(ST_XMin(box), ST_YMin(box), ST_XMax(box), ST_YMax(box)) FROM extent;")"
if [ -z "${bounds}" ] || [ "${bounds}" = "null" ]; then
  echo "${iso2} country pack bounds could not be determined" >&2
  exit 65
fi
if ! jq -e 'length == 4 and all(.[]; type == "number") and .[0] >= -180 and .[1] >= -90 and .[2] <= 180 and .[3] <= 90 and .[0] < .[2] and .[1] < .[3] and . != [-180, -90, 180, 90]' <<<"${bounds}" >/dev/null; then
  echo "${iso2} country pack bounds failed the coordinate quality gate: ${bounds}" >&2
  exit 65
fi

output_file="${output_dir}/${iso2}.pmtiles"
staging_file="${output_dir}/${iso2}.gpkg"
pg_connection="PG:host=${database} port=5432 dbname=gis user=postgres password=${postgres_password}"
# Always stage from a clean GeoPackage so a failed quality gate cannot leave
# stale or partial layers that contaminate a subsequent retry.
rm -f "${staging_file}"
protected_sql="SELECT abs(a.area_id)::bigint AS id, a.name, a.boundary, a.leisure, a.protect_class, a.protection_title, a.operator, a.designation, '${iso2}' AS country_code, ${area_geom_expr} AS geom FROM \"${schema}\".protected_areas a ${clip_join}"
tourism_point_sql="SELECT abs(a.node_id)::bigint AS id, a.name, a.feature_class, a.tourism, a.amenity, a.natural, a.leisure, '${iso2}' AS country_code, a.geom FROM \"${schema}\".tourism_pois a ${clip_join} WHERE ${tourism_name_predicate}"
tourism_area_sql="SELECT abs(a.area_id)::bigint AS id, a.name, a.feature_class, a.tourism, a.amenity, a.natural, a.leisure, '${iso2}' AS country_code, ${area_geom_expr} AS geom FROM \"${schema}\".tourism_areas a ${clip_join} WHERE ${generic_name_predicate//NAME/a.name}"
protected_label_sql="SELECT min(a.area_id)::bigint AS id, a.name, '${iso2}' AS country_code, ST_PointOnSurface(ST_UnaryUnion(ST_Collect(${area_geom_expr}))) AS geom FROM \"${schema}\".protected_areas a ${clip_join} WHERE a.name IS NOT NULL AND btrim(a.name) <> '' GROUP BY a.name"
heritage_point_sql="SELECT abs(a.node_id)::bigint AS id, a.name, a.feature_class, a.heritage_status, a.heritage_type, a.historical_period, a.civilization, a.designation, '${iso2}' AS country_code, a.geom FROM \"${schema}\".heritage_pois a ${clip_join} WHERE ${heritage_name_predicate}"
heritage_area_sql="SELECT abs(a.area_id)::bigint AS id, a.name, a.feature_class, a.heritage_status, a.heritage_type, a.historical_period, a.civilization, a.designation, '${iso2}' AS country_code, ${area_geom_expr} AS geom FROM \"${schema}\".heritage_areas a ${clip_join} WHERE ${generic_name_predicate//NAME/a.name}"
route_sql="SELECT abs(a.relation_id)::bigint AS id, a.name, a.feature_class, a.route_ref, a.operator, a.network, a.difficulty, a.surface, '${iso2}' AS country_code, a.geom FROM \"${schema}\".outdoor_routes a WHERE ${route_name_predicate}"
water_point_sql="SELECT abs(a.node_id)::bigint AS id, a.name, a.feature_class, a.sport, a.dive_type, a.depth_m, '${iso2}' AS country_code, a.geom FROM \"${schema}\".water_sport_pois a ${clip_join} WHERE ${water_name_predicate}"

create_layer="tourism_pois"
create_sql="${tourism_point_sql}"
create_nlt="POINT"
if [ "${area_count}" -gt 0 ]; then
  create_layer="protected_areas"
  create_sql="${protected_sql}"
  create_nlt="PROMOTE_TO_MULTI"
elif [ "${tourism_point_count}" -eq 0 ]; then
  if [ "${tourism_area_count}" -gt 0 ]; then
    create_layer="tourism_areas"
    create_sql="${tourism_area_sql}"
    create_nlt="PROMOTE_TO_MULTI"
  elif [ "${heritage_point_count}" -gt 0 ]; then
    create_layer="heritage_pois"
    create_sql="${heritage_point_sql}"
  elif [ "${heritage_area_count}" -gt 0 ]; then
    create_layer="heritage_areas"
    create_sql="${heritage_area_sql}"
    create_nlt="PROMOTE_TO_MULTI"
  elif [ "${route_count}" -gt 0 ]; then
    create_layer="outdoor_routes"
    create_sql="${route_sql}"
    create_nlt="MULTILINESTRING"
  else
    create_layer="water_sport_pois"
    create_sql="${water_point_sql}"
  fi
fi

docker run --rm --network "${network}" -v "${release_dir}:/work" "${gdal_image}" \
  ogr2ogr -f GPKG -nln "${create_layer}" -nlt "${create_nlt}" \
  /work/protected-areas/"$(basename "${staging_file}")" "${pg_connection}" -sql "${create_sql}"

append_layer() {
  local layer="$1"
  local sql="$2"
  local nlt="$3"
  docker run --rm --network "${network}" -v "${release_dir}:/work" "${gdal_image}" \
    ogr2ogr -f GPKG -update -append -nln "${layer}" -nlt "${nlt}" \
    /work/protected-areas/"$(basename "${staging_file}")" "${pg_connection}" -sql "${sql}"
}

if [ "${create_layer}" != "tourism_pois" ] && [ "${tourism_point_count}" -gt 0 ]; then append_layer tourism_pois "${tourism_point_sql}" POINT; fi
if [ "${create_layer}" != "tourism_areas" ] && [ "${tourism_area_count}" -gt 0 ]; then append_layer tourism_areas "${tourism_area_sql}" PROMOTE_TO_MULTI; fi
if [ "${area_count}" -gt 0 ]; then append_layer protected_area_labels "${protected_label_sql}" POINT; fi
if [ "${create_layer}" != "heritage_pois" ] && [ "${heritage_point_count}" -gt 0 ]; then append_layer heritage_pois "${heritage_point_sql}" POINT; fi
if [ "${create_layer}" != "heritage_areas" ] && [ "${heritage_area_count}" -gt 0 ]; then append_layer heritage_areas "${heritage_area_sql}" PROMOTE_TO_MULTI; fi
if [ "${create_layer}" != "outdoor_routes" ] && [ "${route_count}" -gt 0 ]; then append_layer outdoor_routes "${route_sql}" MULTILINESTRING; fi
if [ "${create_layer}" != "water_sport_pois" ] && [ "${water_point_count}" -gt 0 ]; then append_layer water_sport_pois "${water_point_sql}" POINT; fi

# Re-check the complete exported staging artifact itself. The source SQL
# filter is not sufficient protection against a changed importer, driver, or
# alternate path. GDAL reports SQLite aggregate values as `field = value`.
validate_staged_names() {
  local layer="$1"
  local predicate="$2"
  local invalid_output
  local invalid_count
  local sqlite_invalid_predicate
  if [ "${layer}" = "tourism_pois" ]; then
    sqlite_invalid_predicate="name IS NULL OR trim(name) = '' OR lower(trim(name)) IN ('unnamed','unknown','n/a','na','null','none','untitled','no name','no_name') OR (CAST(trim(name) AS REAL) = trim(name) AND NOT (feature_class = 'visitor_information' AND tourism = 'information'))"
  else
    sqlite_invalid_predicate="name IS NULL OR trim(name) = '' OR lower(trim(name)) IN ('unnamed','unknown','n/a','na','null','none','untitled','no name','no_name') OR CAST(trim(name) AS REAL) = trim(name)"
  fi
  invalid_output="$(docker run --rm -v "${release_dir}:/work:ro" "${gdal_image}" \
    ogrinfo -ro -dialect SQLite \
    -sql "SELECT COUNT(*) AS invalid_count FROM ${layer} WHERE ${sqlite_invalid_predicate}" \
    "/work/protected-areas/$(basename "${staging_file}")" 2>/dev/null || true)"
  invalid_count="$(printf '%s\n' "${invalid_output}" | grep -Eo '[0-9]+$' | tail -1 || true)"
  if [ -z "${invalid_count}" ] || [ "${invalid_count}" -ne 0 ]; then
    echo "${iso2} exported ${layer} failed the display-name quality gate: ${invalid_count:-unknown} invalid features" >&2
    exit 65
  fi
}
if [ "${tourism_point_count}" -gt 0 ]; then validate_staged_names tourism_pois "${tourism_name_predicate_template}"; fi
if [ "${heritage_point_count}" -gt 0 ]; then validate_staged_names heritage_pois "${generic_name_predicate}"; fi
if [ "${route_count}" -gt 0 ]; then validate_staged_names outdoor_routes "${generic_name_predicate}"; fi
if [ "${water_point_count}" -gt 0 ]; then validate_staged_names water_sport_pois "${generic_name_predicate}"; fi

staging_basename="$(basename "${staging_file}")"
output_basename="$(basename "${output_file}")"
docker run --rm -v "${release_dir}:/work" "${gdal_image}" \
  sh -c "cp '/work/protected-areas/${staging_basename}' '/tmp/${staging_basename}' && ogr2ogr --config GDAL_CACHEMAX 4096 --config GDAL_NUM_THREADS ALL_CPUS -f PMTiles -dsco NAME='${iso2} tourism pack' -dsco DESCRIPTION='${iso2} protected areas and tourism data from OpenStreetMap' -dsco TYPE=overlay -dsco MINZOOM=4 -dsco MAXZOOM=${country_pack_maxzoom} -lco MINZOOM=4 -lco MAXZOOM=${country_pack_maxzoom} '/tmp/${output_basename}' '/tmp/${staging_basename}' && cp '/tmp/${output_basename}' '/work/protected-areas/${output_basename}'"
rm -f "${staging_file}"

docker run --rm -v "${release_dir}:/work" "${pmtiles_image}" \
  verify /work/protected-areas/"$(basename "${output_file}")"

sha256="$(sha256sum "${output_file}" | awk '{print $1}')"
jq -n \
  --argjson qualityVersion 2 \
  --arg iso2 "${iso2}" \
  --arg path "protected-areas/${iso2}.pmtiles" \
  --arg sha256 "${sha256}" \
  --argjson bounds "${bounds}" \
  --argjson featureCount "${area_count}" \
  --argjson tourismPointCount "${tourism_point_count}" \
  --argjson tourismPointDroppedCount "${tourism_point_dropped_count}" \
  --argjson tourismAreaCount "${tourism_area_count}" \
  --argjson heritagePointCount "${heritage_point_count}" \
  --argjson heritagePointDroppedCount "${heritage_point_dropped_count}" \
  --argjson heritageAreaCount "${heritage_area_count}" \
  --argjson routeCount "${route_count}" \
  --argjson routeDroppedCount "${route_dropped_count}" \
  --argjson waterSportPointCount "${water_point_count}" \
  --argjson waterSportPointDroppedCount "${water_point_dropped_count}" \
  --argjson dedupPointDroppedCount "${dedup_point_dropped_count}" \
  --argjson maxZoom "${country_pack_maxzoom}" \
  '{qualityVersion: $qualityVersion, iso2: $iso2, path: $path, sha256: $sha256, bounds: $bounds, minZoom: 4, maxZoom: $maxZoom, featureCount: $featureCount, tourismPointCount: $tourismPointCount, tourismPointDroppedCount: $tourismPointDroppedCount, tourismAreaCount: $tourismAreaCount, heritagePointCount: $heritagePointCount, heritagePointDroppedCount: $heritagePointDroppedCount, heritageAreaCount: $heritageAreaCount, routeCount: $routeCount, routeDroppedCount: $routeDroppedCount, waterSportPointCount: $waterSportPointCount, waterSportPointDroppedCount: $waterSportPointDroppedCount, dedupPointDroppedCount: $dedupPointDroppedCount}' \
  >"${output_dir}/${iso2}.metadata.json"

echo "built ${iso2}.pmtiles with ${area_count} protected areas, ${tourism_point_count} named tourism points, dropped ${tourism_point_dropped_count} invalid tourism points"
