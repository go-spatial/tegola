#!/usr/bin/env bash
set -euo pipefail

# Builds immutable tourism map releases from pinned basemap, protected-area, and local overseas-territory inputs with reproducible manifests and validation.

usage() {
  echo "usage: $0 --release-id YYYYMMDD-<protomaps-key>-<gitsha7> [--output-dir DIR]" >&2
}

release_id=""
output_root="${MAP_RELEASE_OUTPUT_DIR:-$(pwd)/deploy/maps/releases}"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --release-id)
      release_id="${2:-}"
      shift 2
      ;;
    --output-dir)
      output_root="${2:-}"
      shift 2
      ;;
    *)
      usage
      exit 64
      ;;
  esac
done

if ! [[ "${release_id}" =~ ^[0-9]{8}-[a-zA-Z0-9._-]+-[0-9a-f]{7}$ ]]; then
  usage
  exit 64
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
lock_file="${MAP_RELEASE_LOCK:-${script_dir}/release-inputs.lock.json}"
for command in jq sha256sum md5sum curl docker; do
  command -v "${command}" >/dev/null 2>&1 || {
    echo "required command is unavailable: ${command}" >&2
    exit 69
  }
done

release_dir="${output_root}/${release_id}"
if [ -e "${release_dir}" ]; then
  echo "release directory already exists; release IDs are immutable: ${release_dir}" >&2
  exit 73
fi
mkdir -p "${release_dir}"
if [ -n "${MAP_RELEASE_INPUTS_DIR:-}" ]; then
  if [ ! -d "${MAP_RELEASE_INPUTS_DIR}" ]; then
    echo "MAP_RELEASE_INPUTS_DIR does not exist: ${MAP_RELEASE_INPUTS_DIR}" >&2
    exit 64
  fi
  mkdir -p "${release_dir}/inputs"
  cp -R "${MAP_RELEASE_INPUTS_DIR}/." "${release_dir}/inputs/"
fi

export MAP_RELEASE_LOCK="${lock_file}"
bash "${script_dir}/build-basemap.sh" "${release_dir}" "${release_id}"

countries_file="${script_dir}/countries.json"
mapfile -t countries < <(jq -er '.coverage.protectedAreaCountries[]' "${lock_file}")
mkdir -p "${release_dir}/logs"

# Download each distinct Geofabrik source once before parallel imports so shared
# sources such as France cannot be observed half-written by another pack.
declare -A downloaded_sources=()
for iso2 in "${countries[@]}"; do
  iso2="${iso2//$'\r'/}"
  source_key="$(jq -er --arg iso2 "${iso2}" '.[$iso2].source // $iso2' "${countries_file}")"
  source_key="${source_key//$'\r'/}"
  if [ -n "${downloaded_sources["${source_key}"]+x}" ]; then continue; fi
  local_pbf="$(jq -r --arg iso2 "${iso2}" '.[$iso2].localPbf // empty' "${countries_file}")"
  if [ -n "${local_pbf}" ]; then
    pbf_file="${release_dir}/inputs/${local_pbf}"
    if [ ! -f "${pbf_file}" ]; then
      echo "missing local map input for ${iso2}: ${pbf_file}" >&2
      exit 65
    fi
    downloaded_sources["${source_key}"]=1
    continue
  fi
  slug="$(jq -er --arg source "${source_key}" '.geofabrik.inputs[$source].slug' "${lock_file}")"
  url="$(jq -er --arg source "${source_key}" '.geofabrik.inputs[$source].url' "${lock_file}")"
  expected_md5="$(jq -er --arg source "${source_key}" '.geofabrik.inputs[$source].md5' "${lock_file}")"
  slug="${slug//$'\r'/}"; url="${url//$'\r'/}"; expected_md5="${expected_md5//$'\r'/}"
  pbf_file="${release_dir}/inputs/${slug}-latest.osm.pbf"
  if [ ! -f "${pbf_file}" ]; then curl --fail --location --retry 3 --retry-all-errors --silent --show-error "${url}" --output "${pbf_file}"; fi
  actual_md5="$(md5sum "${pbf_file}" | awk '{print $1}')"
  if [ "${actual_md5}" != "${expected_md5}" ]; then echo "Geofabrik input changed for ${source_key}" >&2; exit 65; fi
  downloaded_sources["${source_key}"]=1
done

country_jobs="${MAP_COUNTRY_JOBS:-3}"
case "${country_jobs}" in ''|*[!0-9]*|0) echo "MAP_COUNTRY_JOBS must be a positive integer" >&2; exit 64;; esac
pids=()
failed=false
for iso2 in "${countries[@]}"; do
  iso2="${iso2//$'\r'/}"
  bash "${script_dir}/build-country-pack.sh" "${release_dir}" "${iso2}" >"${release_dir}/logs/${iso2}.log" 2>&1 &
  pids+=("$!")
  if [ "${#pids[@]}" -ge "${country_jobs}" ]; then
    if ! wait "${pids[0]}"; then failed=true; fi
    pids=("${pids[@]:1}")
  fi
done
for pid in "${pids[@]}"; do if ! wait "${pid}"; then failed=true; fi; done
if [ "${failed}" = true ]; then echo "one or more country-pack builds failed; inspect ${release_dir}/logs" >&2; exit 1; fi

pmtiles_image="$(jq -er '.protomaps.pmtilesCliImage' "${lock_file}")"
while IFS= read -r -d '' archive; do
  relative_archive="${archive#${release_dir}/}"
  docker run --rm -v "${release_dir}:/work" "${pmtiles_image}" \
    verify "/work/${relative_archive}"
done < <(find "${release_dir}" -type f -name '*.pmtiles' -print0)

base_url="${MAP_PUBLIC_BASE_URL:-https://maps-dev.meistercrm.com}"
base_url="${base_url%/}"
basemap_metadata="$(cat "${release_dir}/basemap/metadata.json")"
protected_packs="$(jq -s \
  --arg base_url "${base_url}" \
  --arg release_id "${release_id}" \
  'reduce .[] as $pack ({};
    .[$pack.iso2] = {
      tiles: [($base_url + "/releases/" + $release_id + "/protected-areas/" + $pack.iso2 + "/{z}/{x}/{y}.mvt")],
      bounds: $pack.bounds,
      minZoom: $pack.minZoom,
      maxZoom: $pack.maxZoom,
      sha256: $pack.sha256,
      featureCount: $pack.featureCount,
      tourismPointCount: $pack.tourismPointCount,
      tourismPointDroppedCount: $pack.tourismPointDroppedCount,
      tourismAreaCount: ($pack.tourismAreaCount // 0),
      heritagePointCount: ($pack.heritagePointCount // 0),
      heritagePointDroppedCount: ($pack.heritagePointDroppedCount // 0),
      heritageAreaCount: ($pack.heritageAreaCount // 0),
      routeCount: ($pack.routeCount // 0),
      routeDroppedCount: ($pack.routeDroppedCount // 0),
      waterSportPointCount: ($pack.waterSportPointCount // 0),
      waterSportPointDroppedCount: ($pack.waterSportPointDroppedCount // 0),
      dedupPointDroppedCount: ($pack.dedupPointDroppedCount // 0),
      qualityVersion: ($pack.qualityVersion // 2)
    }
  )' "${release_dir}"/protected-areas/*.metadata.json)"

jq -n \
  --arg release_id "${release_id}" \
  --arg base_url "${base_url}" \
  --arg basemap_path "$(jq -er '.path' "${release_dir}/basemap/metadata.json")" \
  --argjson basemap "${basemap_metadata}" \
  --argjson protected_packs "${protected_packs}" \
  '{
    version: 1,
    releaseId: $release_id,
    glyphs: ($base_url + "/releases/" + $release_id + "/assets/protomaps-assets/fonts/{fontstack}/{range}.pbf"),
    sprite: ($base_url + "/releases/" + $release_id + "/assets/protomaps-assets/sprites/v4/light"),
    basemap: {
      tiles: [($base_url + "/releases/" + $release_id + "/" + ($basemap_path | sub("\\.pmtiles$"; "")) + "/{z}/{x}/{y}.mvt")],
      bounds: $basemap.bounds,
      minZoom: $basemap.minZoom,
      maxZoom: $basemap.maxZoom,
      sha256: $basemap.sha256
    },
    protectedAreas: $protected_packs
  }' >"${release_dir}/manifest.json"

source_inputs='[]'
for iso2 in "${countries[@]}"; do
  iso2="${iso2//$'\r'/}"
  source_key="$(jq -er --arg iso2 "${iso2}" '.[$iso2].source // $iso2' "${countries_file}")"
  source_key="${source_key//$'\r'/}"
  if jq -e --arg source "${source_key}" 'any(.[]; .source == $source)' <<<"${source_inputs}" >/dev/null; then
    continue
  fi
  local_pbf="$(jq -r --arg iso2 "${iso2}" '.[$iso2].localPbf // empty' "${countries_file}")"
  if [ -n "${local_pbf}" ]; then
    pbf_file="${release_dir}/inputs/${local_pbf}"
    slug="${local_pbf%.osm.pbf}"
    pbf_sha256="$(sha256sum "${pbf_file}" | awk '{print $1}')"
    source_inputs="$(jq -c --arg source "${source_key}" --arg slug "${slug}" --arg sha256 "${pbf_sha256}" --arg url "local:${local_pbf}" '. + [{source: $source, slug: $slug, url: $url, md5: null, sha256: $sha256}]' <<<"${source_inputs}")"
    continue
  fi
  slug="$(jq -er --arg source "${source_key}" '.geofabrik.inputs[$source].slug' "${lock_file}")"
  pbf_file="${release_dir}/inputs/${slug}-latest.osm.pbf"
  pbf_sha256="$(sha256sum "${pbf_file}" | awk '{print $1}')"
  source_inputs="$(jq -c \
    --arg source "${source_key}" \
    --arg slug "${slug}" \
    --arg md5 "$(jq -er --arg source "${source_key}" '.geofabrik.inputs[$source].md5' "${lock_file}")" \
    --arg sha256 "${pbf_sha256}" \
    --arg url "$(jq -er --arg source "${source_key}" '.geofabrik.inputs[$source].url' "${lock_file}")" \
    '. + [{source: $source, slug: $slug, url: $url, md5: $md5, sha256: $sha256}]' <<<"${source_inputs}")"
done

protected_metadata="$(jq -s '.' "${release_dir}"/protected-areas/*.metadata.json)"
jq \
  --arg release_id "${release_id}" \
  --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson geofabrik_inputs "${source_inputs}" \
  --argjson basemap "${basemap_metadata}" \
  --argjson protected_archives "${protected_metadata}" \
  '. + {
    releaseId: $release_id,
    generatedAt: $generated_at,
    resolvedInputs: {geofabrik: $geofabrik_inputs},
    artifacts: {basemap: $basemap, protectedAreas: $protected_archives}
  }' "${lock_file}" >"${release_dir}/release.lock.json"

bash "${script_dir}/quality-report.sh" "${release_dir}" "${release_dir}/quality-report.md"
bash "${script_dir}/performance-report.sh" "${release_dir}" "${release_dir}/performance-report.md"
bash "${script_dir}/validate-release.sh" "${release_dir}"
echo "release ready: ${release_dir}"
