#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../.." && pwd)"
map_assets_dir="${MAP_ASSETS_DIR:-${repo_root}/deploy/maps/map-assets}"
output="${map_assets_dir}/east-africa.pmtiles"
bbox="${SAFARI_EAST_AFRICA_BBOX:-28.0,-12.5,42.5,5.5}"
maxzoom="${PMTILES_MAXZOOM:-14}"
threads="${PMTILES_DOWNLOAD_THREADS:-8}"
protomaps_build_key="${PROTOMAPS_BUILD_KEY:-}"
docker_run_mode="${SAFARI_DOCKER_RUN_MODE:-run}"

mkdir -p "${map_assets_dir}/protomaps-assets"

docker_run() {
  if [ "${docker_run_mode}" != "create-start" ]; then
    docker run --rm "$@"
    return
  fi

  local container_id
  container_id="$(docker create "$@")"
  local exit_code
  if docker start -a "${container_id}"; then
    exit_code="$(docker inspect "${container_id}" --format '{{.State.ExitCode}}' 2>/dev/null || echo 1)"
  else
    exit_code="$(docker inspect "${container_id}" --format '{{.State.ExitCode}}' 2>/dev/null || echo 1)"
  fi
  docker rm "${container_id}" >/dev/null || true
  if [ -z "${exit_code}" ]; then
    exit_code=1
  fi
  return "${exit_code}"
}

if [ -z "${protomaps_build_key}" ]; then
  protomaps_build_key="$(
    docker_run alpine:3.20 sh -eu -c \
      "apk add --no-cache curl jq >/dev/null && curl -fsSL https://build-metadata.protomaps.dev/builds.json | jq -r 'sort_by(.key) | last.key'"
  )"
fi

if [ -z "${protomaps_build_key}" ] || [ "${protomaps_build_key}" = "null" ]; then
  echo "Unable to resolve latest Protomaps build key. Set PROTOMAPS_BUILD_KEY." >&2
  exit 1
fi

source_url="${PROTOMAPS_SOURCE_URL:-https://build.protomaps.com/${protomaps_build_key}}"
echo "Extracting ${source_url} -> ${output}"
docker_run \
  -v "${map_assets_dir}:/data" \
  protomaps/go-pmtiles:latest \
  extract "${source_url}" /data/east-africa.pmtiles \
  --bbox="${bbox}" \
  --maxzoom="${maxzoom}" \
  --download-threads="${threads}"

docker_run \
  -v "${map_assets_dir}:/data" \
  protomaps/go-pmtiles:latest \
  verify /data/east-africa.pmtiles

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT

echo "Downloading Protomaps font and sprite assets"
curl -fsSL "https://github.com/protomaps/basemaps-assets/archive/refs/heads/main.tar.gz" \
  | tar -xz -C "${tmp_dir}"

rm -rf "${map_assets_dir}/protomaps-assets/fonts" "${map_assets_dir}/protomaps-assets/sprites"
mkdir -p "${map_assets_dir}/protomaps-assets"
cp -R "${tmp_dir}/basemaps-assets-main/fonts" "${map_assets_dir}/protomaps-assets/fonts"
cp -R "${tmp_dir}/basemaps-assets-main/sprites" "${map_assets_dir}/protomaps-assets/sprites"

echo "Map assets ready in ${map_assets_dir}"
