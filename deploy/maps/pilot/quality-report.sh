#!/usr/bin/env bash
set -euo pipefail

# Produces deterministic country-pack quality reports with naming, bounds, checksum, layer-count, and de-duplication evidence before any immutable release is considered publishable.

if [ "$#" -ne 2 ]; then echo "usage: $0 RELEASE_DIR OUTPUT_MD" >&2; exit 64; fi
release_dir="$1"
output_md="$2"
output_json="${output_md%.md}.json"
for command in jq sha256sum; do command -v "${command}" >/dev/null 2>&1 || { echo "required command is unavailable: ${command}" >&2; exit 69; }; done

reports='[]'
overall='pass'
for metadata in "${release_dir}"/protected-areas/*.metadata.json; do
  [ -f "${metadata}" ] || continue
  iso2="$(jq -r '.iso2' "${metadata}")"
  archive="${release_dir}/protected-areas/${iso2}.pmtiles"
  if [ ! -f "${archive}" ]; then overall='blocked'; continue; fi
  actual_sha="$(sha256sum "${archive}" | awk '{print $1}')"
  expected_sha="$(jq -r '.sha256' "${metadata}")"
  bounds_ok="$(jq -r '(.bounds|length==4 and all(.[]; type=="number") and .[0]>=-180 and .[1]>=-90 and .[2]<=180 and .[3]<=90 and .[0]<.[2] and .[1]<.[3] and . != [-180,-90,180,90])' "${metadata}")"
  metadata_ok="$(jq -r '(.qualityVersion // 0)>=2 and (.tourismPointDroppedCount|type=="number") and (.heritagePointDroppedCount|type=="number") and (.routeDroppedCount|type=="number") and (.waterSportPointDroppedCount|type=="number") and (.dedupPointDroppedCount|type=="number")' "${metadata}")"
  status='pass'
  if [ "${actual_sha}" != "${expected_sha}" ] || [ "${bounds_ok}" != true ] || [ "${metadata_ok}" != true ]; then status='blocked'; overall='blocked'; fi
  reports="$(jq -c --arg iso2 "${iso2}" --arg status "${status}" --arg actualSha "${actual_sha}" --arg expectedSha "${expected_sha}" --argjson boundsOk "${bounds_ok}" --argjson metadataOk "${metadata_ok}" --argjson published "$(jq -r '(.featureCount // 0) + (.tourismPointCount // 0) + (.tourismAreaCount // 0) + (.heritagePointCount // 0) + (.heritageAreaCount // 0) + (.routeCount // 0) + (.waterSportPointCount // 0)' "${metadata}")" '. + [{iso2:$iso2,status:$status,actualSha:$actualSha,expectedSha:$expectedSha,boundsOk:$boundsOk,requiredMetadataOk:$metadataOk,publishedFeatureCount:$published}]' <<<"${reports}")"
done

jq -n --arg qualityGate "${overall}" --argjson countries "${reports}" '{qualityGate:$qualityGate,countries:$countries}' >"${output_json}"
{
  echo "# Country-pack quality report"
  echo
  echo "Overall gate: **${overall}**"
  echo
  echo "| ISO2 | Status | Bounds | Required metadata | SHA-256 |"
  echo "|---|---|---|---|---|"
  jq -r '.countries[] | "| \(.iso2) | \(.status) | \(.boundsOk) | \(.requiredMetadataOk) | \(.actualSha == .expectedSha) |"' "${output_json}"
} >"${output_md}"
echo "wrote ${output_md} and ${output_json}"
