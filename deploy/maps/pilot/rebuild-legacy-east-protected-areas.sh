#!/usr/bin/env bash
set -euo pipefail

# Rebuilds the existing dev Tegola table without exposing a half-imported data
# set. The public table is replaced inside one transaction only after Uganda's
# park gate has passed in a staging schema.

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
maps_dir="${script_dir}/.."
release_id="${LEGACY_EAST_RELEASE_ID:-$(date -u +%Y%m%d%H%M%S)}"
staging_schema="gis_legacy_east_${release_id//[^a-zA-Z0-9_]/_}"
staging_schema="${staging_schema,,}"

if ! [[ "${staging_schema}" =~ ^[a-z][a-z0-9_]*$ ]]; then
  echo "generated legacy staging schema is invalid" >&2
  exit 64
fi

run_psql() {
  if [ "${OSM_IMPORT_MODE:-auto}" = "docker" ] || { [ "${OSM_IMPORT_MODE:-auto}" = "auto" ] && { ! command -v psql >/dev/null 2>&1 || ! command -v osm2pgsql >/dev/null 2>&1; }; }; then
    docker run --rm --network="${OSM_IMPORT_DOCKER_NETWORK:-safari-local-dev_default}" \
      -e PGPASSWORD="${PGPASSWORD:-postgres}" \
      -e PGSSLMODE="${PGSSLMODE:-disable}" \
      "${POSTGRES_IMAGE:-postgres:16-alpine}" \
      psql \
        -h "${PGHOST:-maps-postgis}" \
        -p "${PGPORT:-5432}" \
        -U "${PGUSER:-postgres}" \
        -d "${PGDATABASE:-gis}" \
        -v ON_ERROR_STOP=1 \
        "$@"
    return
  fi

  PGPASSWORD="${PGPASSWORD:-postgres}" PGSSLMODE="${PGSSLMODE:-disable}" \
    psql \
      -h "${PGHOST:-127.0.0.1}" \
      -p "${PGPORT:-55432}" \
      -U "${PGUSER:-postgres}" \
      -d "${PGDATABASE:-gis}" \
      -v ON_ERROR_STOP=1 \
      "$@"
}

run_psql -c "DROP SCHEMA IF EXISTS \"${staging_schema}\" CASCADE;"
export GIS_SCHEMA="${staging_schema}"
export OSM_COUNTRIES="tanzania kenya uganda rwanda burundi"
export OSM2PGSQL_DROP_MIDDLE_TABLES=true
bash "${maps_dir}/import-protected-areas.sh"

uganda_count="$(run_psql -Atc "SELECT count(*) FROM \"${staging_schema}\".protected_areas WHERE coalesce(name, '') ~* 'Bwindi';")"
uganda_fallback_count="$(run_psql -Atc "SELECT count(*) FROM \"${staging_schema}\".protected_areas WHERE coalesce(name, '') ~* 'Murchison Falls|Queen Elizabeth|Kidepo';")"
if [ "${uganda_count}" -eq 0 ] || [ "${uganda_fallback_count}" -eq 0 ]; then
  echo "legacy East staging data failed Uganda park gate (Bwindi plus Murchison Falls, Queen Elizabeth, or Kidepo)" >&2
  exit 65
fi

run_psql <<SQL
BEGIN;
LOCK TABLE gis.protected_areas IN ACCESS EXCLUSIVE MODE;
DROP TABLE gis.protected_areas;
ALTER TABLE "${staging_schema}".protected_areas SET SCHEMA gis;
DROP SCHEMA "${staging_schema}" CASCADE;
ANALYZE gis.protected_areas;
COMMIT;
SQL

run_psql -c "SELECT count(*) AS protected_area_count FROM gis.protected_areas;"
echo "legacy East protected areas atomically rebuilt (${release_id})"
