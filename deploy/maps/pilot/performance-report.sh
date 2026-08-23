#!/usr/bin/env bash
set -euo pipefail

# Produces local PMTiles performance metrics without publishing data, including archive size, feature counts, optional tile metadata, and explicit pass/fail thresholds for local release review only.

if [ "$#" -ne 2 ]; then
  echo "usage: $0 RELEASE_DIR OUTPUT_MD" >&2
  exit 64
fi

release_dir="$1"
output_md="$2"
output_json="${output_md%.md}.json"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

for command in jq sha256sum wc; do
  command -v "${command}" >/dev/null 2>&1 || { echo "required command is unavailable: ${command}" >&2; exit 69; }
done

max_archive_bytes="${MAP_PERF_MAX_ARCHIVE_BYTES:-52428800}"
generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
packs='[]'
for metadata in "${release_dir}"/protected-areas/*.metadata.json; do
  [ -f "${metadata}" ] || continue
  iso2="$(jq -r '.iso2' "${metadata}")"
  archive="${release_dir}/protected-areas/${iso2}.pmtiles"
  [ -f "${archive}" ] || { echo "missing archive for ${iso2}" >&2; exit 65; }
  bytes="$(wc -c <"${archive}" | tr -d '[:space:]')"
  sha256="$(sha256sum "${archive}" | awk '{print $1}')"
  feature_count="$(jq -r '(.featureCount // 0) + (.tourismPointCount // 0) + (.tourismAreaCount // 0) + (.heritagePointCount // 0) + (.heritageAreaCount // 0) + (.routeCount // 0) + (.waterSportPointCount // 0)' "${metadata}")"
  packs="$(jq -c --arg iso2 "${iso2}" --arg path "protected-areas/${iso2}.pmtiles" --arg sha256 "${sha256}" --argjson bytes "${bytes}" --argjson featureCount "${feature_count}" --argjson maxArchiveBytes "${max_archive_bytes}" '. + [{iso2:$iso2,path:$path,archiveBytes:$bytes,featureCount:$featureCount,sha256:$sha256,tileSampling:"not_available_without_worker_or_pmtiles_tile_sampler",archiveStatus:(if $bytes <= $maxArchiveBytes then "pass" else "review" end)}]' <<<"${packs}")"
done

jq -n --arg generatedAt "${generated_at}" --argjson maxArchiveBytes "${max_archive_bytes}" --argjson packs "${packs}" '{generatedAt:$generatedAt,scope:"local immutable pilot artifacts; no R2 or Worker requests",maxArchiveBytes:$maxArchiveBytes,packs:$packs}' >"${output_json}"
{
  echo "# Local PMTiles performance report"
  echo
  echo "Generated: ${generated_at}"
  echo
  echo "This report measures archive size and feature density only. Worker tile latency and visible-tile payloads require a deployed target and are deliberately not requested during the local-only pilot."
  echo
  echo "| ISO2 | Archive bytes | Features | Archive threshold | Tile sampling |"
  echo "|---|---:|---:|---|---|"
  jq -r '.packs[] | "| \(.iso2) | \(.archiveBytes) | \(.featureCount) | \(.archiveStatus) | \(.tileSampling) |"' "${output_json}"
} >"${output_md}"

echo "wrote ${output_md} and ${output_json}"
