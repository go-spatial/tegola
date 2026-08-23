#!/usr/bin/env bash
set -euo pipefail

# Builds Egypt, Tanzania, Philippines, Peru, and Türkiye locally with locked inputs, quality gates, performance reports, and no object-store publication or manifest activation now, only.

if [ "$#" -ne 1 ]; then echo "usage: $0 RELEASE_DIR" >&2; exit 64; fi
release_dir="$1"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
countries=(EG TZ PH PE TR)
for iso2 in "${countries[@]}"; do
  bash "${script_dir}/build-country-pack.sh" "${release_dir}" "${iso2}"
done
bash "${script_dir}/quality-report.sh" "${release_dir}" "${release_dir}/quality-report.md"
bash "${script_dir}/performance-report.sh" "${release_dir}" "${release_dir}/performance-report.md"
echo "five-country local pilot complete: ${release_dir}"
