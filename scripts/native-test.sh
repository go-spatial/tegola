#!/usr/bin/env bash
# Native write acceptance. All external databases must be disposable test DBs.
# TEGOLA_REVIEW_MYSQL_DSN / TEGOLA_REVIEW_POSTGIS_DSN enable live SQL tests.
# TEGOLA_REVIEW_MYSQL_ADMIN_DSN supplies the isolated fixture's database/user
# provisioning role; when unset, follow-up tests use TEGOLA_REVIEW_MYSQL_DSN.
# TEGOLA_MOS_MYSQL_DSN optionally selects a separate MOS test database;
# otherwise MOS tests reuse TEGOLA_REVIEW_MYSQL_DSN with their own table names.
# TEGOLA_REVIEW_GPKG overrides the bundled GDAL editing-fixture.gpkg;
# its HTTP test always mutates a temporary copy, never the supplied source.
set -euo pipefail
repo_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_dir"
if [[ "$(go env CGO_ENABLED)" != 1 ]]; then
  echo 'ERROR: CGO_ENABLED=1 and a C compiler are required for GeoPackage acceptance.' >&2
  exit 1
fi
go version
export TEGOLA_MOS_MYSQL_DSN="${TEGOLA_MOS_MYSQL_DSN:-${TEGOLA_REVIEW_MYSQL_DSN:-}}"
printf '%s\n' 'Running provider and HTTP contracts (GeoPackage uses native SQLite).'
go test ./provider/gpkg ./provider/mysql ./provider/postgis ./ogc/wfs ./server -count=1
printf '%s\n' 'ENABLED: native GeoPackage fixture (bundled, unless TEGOLA_REVIEW_GPKG overrides it).'
for profile in MYSQL POSTGIS; do
  variable="TEGOLA_REVIEW_${profile}_DSN"
  if [[ -z "${!variable:-}" ]]; then
    printf 'SKIPPED: %s external fixture (%s is unset).\n' "$profile" "$variable"
  else
    printf 'ENABLED: %s external fixture; its tests ran in the package suite above.\n' "$profile"
  fi
done
if [[ -n "${TEGOLA_MOS_MYSQL_DSN:-}" ]]; then
  printf '%s\n' 'ENABLED: MySQL native MOS geometry and bounds matrix.'
else
  printf '%s\n' 'SKIPPED: MySQL native MOS matrix (both MySQL DSNs are unset).'
fi
