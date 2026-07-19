#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 RELEASE_DIR" >&2
  exit 64
fi

release_dir="$1"
manifest_file="${release_dir}/manifest.json"
lock_file="${release_dir}/release.lock.json"
for command in jq sha256sum; do
  command -v "${command}" >/dev/null 2>&1 || {
    echo "required command is unavailable: ${command}" >&2
    exit 69
  }
done

jq -e '
  .version == 1 and
  (.releaseId | type == "string" and length > 0) and
  (.glyphs | test("\\{fontstack\\}")) and
  (.sprite | type == "string" and length > 0) and
  (.basemap.tiles | length == 1) and
  (.basemap.bounds | length == 4) and
  .basemap.minZoom == 0 and .basemap.maxZoom == 12 and
  (.protectedAreas | type == "object" and length == 13) and
  (all(.protectedAreas[]; (.tiles | length == 1) and (.bounds | length == 4) and .minZoom == 4 and .maxZoom == 14)) and
  (.protectedAreas.ZA != null)
' "${manifest_file}" >/dev/null

for archive in "${release_dir}/basemap/safari-mainland-pilot.pmtiles" "${release_dir}"/protected-areas/*.pmtiles; do
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
