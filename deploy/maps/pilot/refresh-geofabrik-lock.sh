#!/usr/bin/env bash
set -euo pipefail

# Run this only in a reviewed change before building a new release. It records
# the checksum sidecars Geofabrik publishes for the exact -latest inputs.

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
lock_file="${MAP_RELEASE_LOCK:-${script_dir}/release-inputs.lock.json}"
temporary_lock="$(mktemp "${lock_file}.XXXXXX")"
trap 'rm -f "${temporary_lock}"' EXIT

for command in curl jq; do
  command -v "${command}" >/dev/null 2>&1 || {
    echo "required command is unavailable: ${command}" >&2
    exit 69
  }
done

cp "${lock_file}" "${temporary_lock}"
while IFS=$'\t' read -r iso2 slug; do
  checksum="$(curl --fail --location --silent --show-error \
    "$(jq -r '.geofabrik.baseUrl' "${temporary_lock}")/${slug}-latest.osm.pbf.md5" | awk '{print $1}')"
  jq --arg iso2 "${iso2}" --arg checksum "${checksum}" \
    '.geofabrik.inputs[$iso2].md5 = $checksum' "${temporary_lock}" >"${temporary_lock}.next"
  mv "${temporary_lock}.next" "${temporary_lock}"
done < <(jq -r '.geofabrik.inputs | to_entries[] | "\(.key)\t\(.value.slug)"' "${temporary_lock}")

mv "${temporary_lock}" "${lock_file}"
trap - EXIT
echo "refreshed Geofabrik checksums in ${lock_file}; commit this lock change before releasing"
