#!/usr/bin/env bash
set -euo pipefail

# Finalizes existing tourism map releases by verifying artifacts, recording immutable manifests, resolving local and remote source checksums, and running validation before object-store publication for development.

if [ "$#" -ne 2 ]; then
  echo "usage: $0 RELEASE_DIR RELEASE_ID" >&2
  exit 64
fi

release_dir="$1"
release_id="$2"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
lock_file="${MAP_RELEASE_LOCK:-${script_dir}/release-inputs.lock.json}"
country_file="${script_dir}/countries.json"
base_url="${MAP_PUBLIC_BASE_URL:-https://maps-dev.meistercrm.com}"
base_url="${base_url%/}"

for command in jq sha256sum; do
  command -v "${command}" >/dev/null 2>&1 || { echo "required command is unavailable: ${command}" >&2; exit 69; }
done

basemap_metadata="$(cat "${release_dir}/basemap/metadata.json")"
protected_packs="$(jq -s --arg base_url "${base_url}" --arg release_id "${release_id}" '
  reduce .[] as $pack ({}; .[$pack.iso2] = {
    tiles: [($base_url + "/releases/" + $release_id + "/protected-areas/" + $pack.iso2 + "/{z}/{x}/{y}.mvt")],
    bounds: $pack.bounds, minZoom: $pack.minZoom, maxZoom: $pack.maxZoom,
    sha256: $pack.sha256, featureCount: $pack.featureCount,
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
  })' "${release_dir}"/protected-areas/*.metadata.json)"

jq -n --arg release_id "${release_id}" --arg base_url "${base_url}" \
  --arg basemap_path "$(jq -er '.path' "${release_dir}/basemap/metadata.json")" \
  --argjson basemap "${basemap_metadata}" --argjson protected_packs "${protected_packs}" \
  '{version: 1, releaseId: $release_id,
    glyphs: ($base_url + "/releases/" + $release_id + "/assets/protomaps-assets/fonts/{fontstack}/{range}.pbf"),
    sprite: ($base_url + "/releases/" + $release_id + "/assets/protomaps-assets/sprites/v4/light"),
    basemap: {tiles: [($base_url + "/releases/" + $release_id + "/" + ($basemap_path | sub("\\.pmtiles$"; "")) + "/{z}/{x}/{y}.mvt")], bounds: $basemap.bounds, minZoom: $basemap.minZoom, maxZoom: $basemap.maxZoom, sha256: $basemap.sha256},
    protectedAreas: $protected_packs}' > "${release_dir}/manifest.json"

source_inputs='[]'
mapfile -t countries < <(jq -er '.coverage.protectedAreaCountries[]' "${lock_file}")
for iso2 in "${countries[@]}"; do
  iso2="${iso2//$'\r'/}"
  source_key="$(jq -er --arg iso2 "${iso2}" '.[$iso2].source // $iso2' "${country_file}")"
  source_key="${source_key//$'\r'/}"
  if jq -e --arg source "${source_key}" 'any(.[]; .source == $source)' <<<"${source_inputs}" >/dev/null; then continue; fi
  local_pbf="$(jq -r --arg iso2 "${iso2}" '.[$iso2].localPbf // empty' "${country_file}")"
  if [ -n "${local_pbf}" ]; then
    pbf_file="${release_dir}/inputs/${local_pbf}"
    [ -f "${pbf_file}" ] || { echo "missing local input: ${pbf_file}" >&2; exit 65; }
    slug="${local_pbf%.osm.pbf}"
    url="local:${local_pbf}"
    md5=""
  else
    slug="$(jq -er --arg source "${source_key}" '.geofabrik.inputs[$source].slug' "${lock_file}")"
    pbf_file="${release_dir}/inputs/${slug}-latest.osm.pbf"
    url="$(jq -er --arg source "${source_key}" '.geofabrik.inputs[$source].url' "${lock_file}")"
    md5="$(jq -er --arg source "${source_key}" '.geofabrik.inputs[$source].md5' "${lock_file}")"
  fi
  pbf_sha256="$(sha256sum "${pbf_file}" | awk '{print $1}')"
  source_inputs="$(jq -c --arg source "${source_key}" --arg slug "${slug}" --arg md5 "${md5}" --arg sha256 "${pbf_sha256}" --arg url "${url}" '. + [{source: $source, slug: $slug, url: $url, md5: (if $md5 == "" then null else $md5 end), sha256: $sha256}]' <<<"${source_inputs}")"
done

protected_metadata="$(jq -s '.' "${release_dir}"/protected-areas/*.metadata.json)"
jq --arg release_id "${release_id}" --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson geofabrik_inputs "${source_inputs}" --argjson basemap "${basemap_metadata}" --argjson protected_archives "${protected_metadata}" \
  '. + {releaseId: $release_id, generatedAt: $generated_at, resolvedInputs: {geofabrik: $geofabrik_inputs}, artifacts: {basemap: $basemap, protectedAreas: $protected_archives}}' \
  "${lock_file}" > "${release_dir}/release.lock.json"

bash "${script_dir}/quality-report.sh" "${release_dir}" "${release_dir}/quality-report.md"
bash "${script_dir}/performance-report.sh" "${release_dir}" "${release_dir}/performance-report.md"
bash "${script_dir}/validate-release.sh" "${release_dir}"
echo "release ready: ${release_dir}"
