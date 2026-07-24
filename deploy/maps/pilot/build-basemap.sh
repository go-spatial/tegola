#!/usr/bin/env bash
set -euo pipefail

# Builds the grouped mainland basemap from a GeoJSON union of buffered country
# polygons. The PMTiles CLI's --region option is intentional: a bounding box
# would pull substantial off-pilot data and create a needlessly large archive.

if [ "$#" -ne 2 ]; then
  echo "usage: $0 RELEASE_DIR RELEASE_ID" >&2
  exit 64
fi

release_dir="$1"
release_id="$2"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
lock_file="${MAP_RELEASE_LOCK:-${script_dir}/release-inputs.lock.json}"

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "required command is unavailable: $1" >&2
    exit 69
  }
}

for command in curl docker jq sha256sum tar; do
  require_command "${command}"
done

lock() {
  jq -er "$1" "${lock_file}"
}

download_and_verify_sha256() {
  local url="$1"
  local expected_sha256="$2"
  local destination="$3"
  curl --fail --location --retry 3 --retry-all-errors --silent --show-error "${url}" --output "${destination}"
  local actual_sha256
  actual_sha256="$(sha256sum "${destination}" | awk '{print $1}')"
  if [ "${actual_sha256}" != "${expected_sha256}" ]; then
    echo "checksum mismatch for ${url}: expected ${expected_sha256}, got ${actual_sha256}" >&2
    exit 65
  fi
}

protomaps_url="$(lock '.protomaps.url')"
protomaps_etag="$(lock '.protomaps.etag')"
protomaps_length="$(lock '.protomaps.contentLength | tostring')"
pmtiles_image="$(lock '.protomaps.pmtilesCliImage')"
gdal_image="$(lock '.countryBoundaries.gdalImage')"
country_url="$(lock '.countryBoundaries.url')"
country_sha256="$(lock '.countryBoundaries.sha256')"
buffer_degrees="$(lock '.countryBoundaries.bufferDegrees | tostring')"
assets_url="$(lock '.protomapsAssets.url')"
assets_sha256="$(lock '.protomapsAssets.sha256')"

header_file="${release_dir}/protomaps.headers"
curl --fail --silent --show-error --head "${protomaps_url}" >"${header_file}"
actual_etag="$(awk -F': *' 'tolower($1) == "etag" {gsub(/"/, "", $2); gsub(/\r/, "", $2); print $2}' "${header_file}" | tail -n 1)"
actual_length="$(awk -F': *' 'tolower($1) == "content-length" {gsub(/\r/, "", $2); print $2}' "${header_file}" | tail -n 1)"
if [ "${actual_etag}" != "${protomaps_etag}" ] || [ "${actual_length}" != "${protomaps_length}" ]; then
  echo "locked Protomaps source changed; refresh the lock deliberately before building" >&2
  exit 65
fi

inputs_dir="${release_dir}/inputs"
mkdir -p "${inputs_dir}" "${release_dir}/basemap" "${release_dir}/assets"
country_archive="${inputs_dir}/ne_10m_admin_0_countries.zip"
assets_archive="${inputs_dir}/protomaps-basemaps-assets.tar.gz"

download_and_verify_sha256 "${country_url}" "${country_sha256}" "${country_archive}"
download_and_verify_sha256 "${assets_url}" "${assets_sha256}" "${assets_archive}"

region_file="${release_dir}/safari-mainland-pilot-region.geojson"
# Unicode escapes keep the SQL single quotes inside jq's single-quoted program,
# avoiding shell-dependent quote construction.
codes_sql="$(jq -er '.coverage.basemapCountries | "\u0027" + join("\u0027,\u0027") + "\u0027"' "${lock_file}")"
region_sql="SELECT ISO_A2, ST_Buffer(geometry, ${buffer_degrees}) AS geometry FROM ne_10m_admin_0_countries WHERE ISO_A2 IN (${codes_sql})"

docker run --rm \
  -v "${release_dir}:/work" \
  "${gdal_image}" \
  ogr2ogr -f GeoJSON /work/"$(basename "${region_file}")" \
  /vsizip/work/inputs/"$(basename "${country_archive}")"/ne_10m_admin_0_countries.shp \
  -dialect sqlite -sql "${region_sql}"

region_count="$(jq '.features | length' "${region_file}")"
expected_region_count="$(jq '.coverage.basemapCountries | length' "${lock_file}")"
if [ "${region_count}" -ne "${expected_region_count}" ]; then
  echo "country-boundary extraction returned ${region_count}; expected ${expected_region_count}" >&2
  exit 65
fi

basemap_file="${release_dir}/basemap/safari-mainland-pilot.pmtiles"
docker run --rm \
  -v "${release_dir}:/work" \
  "${pmtiles_image}" \
  extract "${protomaps_url}" /work/basemap/safari-mainland-pilot.pmtiles \
  --region=/work/"$(basename "${region_file}")" \
  --maxzoom=12 \
  --download-threads="${PMTILES_DOWNLOAD_THREADS:-8}"

docker run --rm -v "${release_dir}:/work" "${pmtiles_image}" \
  verify /work/basemap/safari-mainland-pilot.pmtiles

mkdir -p "${release_dir}/assets/extracted"
tar -xzf "${assets_archive}" -C "${release_dir}/assets/extracted"
assets_root="$(find "${release_dir}/assets/extracted" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
if [ -z "${assets_root}" ] || [ ! -d "${assets_root}/fonts" ] || [ ! -d "${assets_root}/sprites" ]; then
  echo "pinned Protomaps assets archive does not contain fonts and sprites" >&2
  exit 65
fi
mkdir -p "${release_dir}/assets/protomaps-assets"
cp -R "${assets_root}/fonts" "${release_dir}/assets/protomaps-assets/fonts"
cp -R "${assets_root}/sprites" "${release_dir}/assets/protomaps-assets/sprites"

basemap_header="$(docker run --rm -v "${release_dir}:/work" "${pmtiles_image}" \
  show /work/basemap/safari-mainland-pilot.pmtiles --header-json)"
bounds="$(printf '%s' "${basemap_header}" | jq -ce '
  .bounds
  | if type == "array" and length == 4 and all(.[]; type == "number") then .
    else error("PMTiles header did not provide numeric bounds")
    end
')"
sha256="$(sha256sum "${basemap_file}" | awk '{print $1}')"

jq -n \
  --arg path "basemap/safari-mainland-pilot.pmtiles" \
  --arg sha256 "${sha256}" \
  --argjson bounds "${bounds}" \
  '{path: $path, sha256: $sha256, bounds: $bounds, minZoom: 0, maxZoom: 12}' \
  >"${release_dir}/basemap/metadata.json"

echo "built grouped basemap for ${release_id}: ${basemap_file}"
