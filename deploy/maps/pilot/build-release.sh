#!/usr/bin/env bash
set -euo pipefail

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
for command in jq sha256sum docker; do
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

export MAP_RELEASE_LOCK="${lock_file}"
bash "${script_dir}/build-basemap.sh" "${release_dir}" "${release_id}"

mapfile -t countries < <(jq -er '.coverage.protectedAreaCountries[]' "${lock_file}")
for iso2 in "${countries[@]}"; do
  bash "${script_dir}/build-country-pack.sh" "${release_dir}" "${iso2}"
done

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
      featureCount: $pack.featureCount
    }
  )' "${release_dir}"/protected-areas/*.metadata.json)"

jq -n \
  --arg release_id "${release_id}" \
  --arg base_url "${base_url}" \
  --argjson basemap "${basemap_metadata}" \
  --argjson protected_packs "${protected_packs}" \
  '{
    version: 1,
    releaseId: $release_id,
    glyphs: ($base_url + "/releases/" + $release_id + "/assets/protomaps-assets/fonts/{fontstack}/{range}.pbf"),
    sprite: ($base_url + "/releases/" + $release_id + "/assets/protomaps-assets/sprites/v4/light"),
    basemap: {
      tiles: [($base_url + "/releases/" + $release_id + "/basemap/safari-mainland-pilot/{z}/{x}/{y}.mvt")],
      bounds: $basemap.bounds,
      minZoom: $basemap.minZoom,
      maxZoom: $basemap.maxZoom,
      sha256: $basemap.sha256
    },
    protectedAreas: $protected_packs
  }' >"${release_dir}/manifest.json"

pbf_inputs='[]'
for iso2 in "${countries[@]}"; do
  slug="$(jq -er --arg iso2 "${iso2}" '.geofabrik.inputs[$iso2].slug' "${lock_file}")"
  pbf_file="${release_dir}/inputs/${slug}-latest.osm.pbf"
  pbf_sha256="$(sha256sum "${pbf_file}" | awk '{print $1}')"
  pbf_inputs="$(jq -c \
    --arg iso2 "${iso2}" \
    --arg slug "${slug}" \
    --arg md5 "$(jq -er --arg iso2 "${iso2}" '.geofabrik.inputs[$iso2].md5' "${lock_file}")" \
    --arg sha256 "${pbf_sha256}" \
    --arg url "$(jq -er '.geofabrik.baseUrl' "${lock_file}")/${slug}-latest.osm.pbf" \
    '. + [{iso2: $iso2, slug: $slug, url: $url, md5: $md5, sha256: $sha256}]' <<<"${pbf_inputs}")"
done

protected_metadata="$(jq -s '.' "${release_dir}"/protected-areas/*.metadata.json)"
jq \
  --arg release_id "${release_id}" \
  --arg generated_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --argjson geofabrik_inputs "${pbf_inputs}" \
  --argjson basemap "${basemap_metadata}" \
  --argjson protected_archives "${protected_metadata}" \
  '. + {
    releaseId: $release_id,
    generatedAt: $generated_at,
    resolvedInputs: {geofabrik: $geofabrik_inputs},
    artifacts: {basemap: $basemap, protectedAreas: $protected_archives}
  }' "${lock_file}" >"${release_dir}/release.lock.json"

bash "${script_dir}/validate-release.sh" "${release_dir}"
echo "release ready: ${release_dir}"
