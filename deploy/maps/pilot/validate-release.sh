#!/usr/bin/env bash
set -euo pipefail

# Validates immutable tourism map releases by checking manifest structure, coordinate bounds, archive checksums, expected country packs, and required runtime assets before publication to Cloudflare safely.

if [ "$#" -ne 1 ]; then
  echo "usage: $0 RELEASE_DIR" >&2
  exit 64
fi

release_dir="$1"
manifest_file="${release_dir}/manifest.json"
lock_file="${release_dir}/release.lock.json"
country_pack_maxzoom="$(jq -er '.coverage.countryPackMaxZoom // 12' "${lock_file}")"
for command in jq sha256sum; do
  command -v "${command}" >/dev/null 2>&1 || {
    echo "required command is unavailable: ${command}" >&2
    exit 69
  }
done

jq -e --argjson expected_packs "$(jq '.coverage.protectedAreaCountries | length' "${lock_file}")" --argjson expected_country_maxzoom "${country_pack_maxzoom}" '
  .version == 1 and
  (.releaseId | type == "string" and length > 0) and
  (.glyphs | test("\\{fontstack\\}")) and
  (.sprite | type == "string" and length > 0) and
  (.basemap.tiles | length == 1) and
  (.basemap.tiles[0] | test("/basemap/[^/]+/\\{z\\}/\\{x\\}/\\{y\\}\\.mvt$")) and
  (.basemap.bounds | length == 4 and all(.[]; type == "number") and .[0] >= -180 and .[1] >= -90 and .[2] <= 180 and .[3] <= 90 and .[0] < .[2] and .[1] < .[3] and . != [-180, -90, 180, 90]) and
  .basemap.minZoom == 0 and .basemap.maxZoom == 12 and
  (.protectedAreas | type == "object" and length == $expected_packs) and
  (all(.protectedAreas[]; (.tiles | length == 1) and (.tiles[0] | test("/protected-areas/[A-Z]{2}/\\{z\\}/\\{x\\}/\\{y\\}\\.mvt$")) and (.bounds | length == 4 and all(.[]; type == "number") and .[0] >= -180 and .[1] >= -90 and .[2] <= 180 and .[3] <= 90 and .[0] < .[2] and .[1] < .[3] and . != [-180, -90, 180, 90]) and .minZoom == 4 and .maxZoom == $expected_country_maxzoom) and (.qualityVersion | type == "number" and . >= 2) and (.tourismPointCount | type == "number" and . >= 0) and (.tourismPointDroppedCount | type == "number" and . >= 0) and (.tourismAreaCount | type == "number" and . >= 0) and (.heritagePointCount | type == "number" and . >= 0) and (.heritagePointDroppedCount | type == "number" and . >= 0) and (.heritageAreaCount | type == "number" and . >= 0) and (.routeCount | type == "number" and . >= 0) and (.routeDroppedCount | type == "number" and . >= 0) and (.waterSportPointCount | type == "number" and . >= 0) and (.waterSportPointDroppedCount | type == "number" and . >= 0) and (.dedupPointDroppedCount | type == "number" and . >= 0)))
' "${manifest_file}" >/dev/null

jq -e --slurpfile manifest "${manifest_file}" '
  all(.artifacts.protectedAreas[] as $pack;
    ($pack.tourismPointCount | type == "number" and . >= 0) and
    ($pack.tourismPointDroppedCount | type == "number" and . >= 0) and
    ($pack.tourismAreaCount | type == "number" and . >= 0) and
    ($pack.qualityVersion | type == "number" and . >= 2) and
    ($pack.heritagePointCount | type == "number" and . >= 0) and
    ($pack.heritagePointDroppedCount | type == "number" and . >= 0) and
    ($pack.heritageAreaCount | type == "number" and . >= 0) and
    ($pack.routeCount | type == "number" and . >= 0) and
    ($pack.routeDroppedCount | type == "number" and . >= 0) and
    ($pack.waterSportPointCount | type == "number" and . >= 0) and
    ($pack.waterSportPointDroppedCount | type == "number" and . >= 0) and
    ($pack.dedupPointDroppedCount | type == "number" and . >= 0) and
    ($manifest[0].protectedAreas[$pack.iso2].tourismPointCount == $pack.tourismPointCount) and
    ($manifest[0].protectedAreas[$pack.iso2].tourismPointDroppedCount == $pack.tourismPointDroppedCount) and
    ($manifest[0].protectedAreas[$pack.iso2].tourismAreaCount == $pack.tourismAreaCount) and
    ($manifest[0].protectedAreas[$pack.iso2].heritagePointCount == $pack.heritagePointCount) and
    ($manifest[0].protectedAreas[$pack.iso2].routeCount == $pack.routeCount)
  )
' "${lock_file}" >/dev/null

basemap_path="$(jq -er '.artifacts.basemap.path' "${lock_file}")"
for archive in "${release_dir}/${basemap_path}" "${release_dir}"/protected-areas/*.pmtiles; do
  actual_sha256="$(sha256sum "${archive}" | awk '{print $1}')"
  relative="${archive#${release_dir}/}"
  expected_sha256="$(jq -er --arg path "${relative}" '
    if .artifacts.basemap.path == $path then .artifacts.basemap.sha256
    else (.artifacts.protectedAreas[] | select(.path == $path) | .sha256)
    end
  ' "${lock_file}")"
  if [ "${actual_sha256}" != "${expected_sha256}" ]; then
    echo "artifact checksum mismatch: ${relative}" >&2
    exit 65
  fi
done

echo "validated map release $(jq -r '.releaseId' "${manifest_file}")"
