#!/usr/bin/env bash
set -euo pipefail

tag="${1:-}"
dist_dir="${TC_WORKER_DIST_DIR:-dist}"
expected_assets=(
  "tc-worker_Darwin_x86_64.tar.gz"
  "tc-worker_Darwin_arm64.tar.gz"
  "tc-worker_Linux_x86_64.tar.gz"
  "tc-worker_Linux_arm64.tar.gz"
)

if [[ -z "${tag}" ]]; then
  echo "usage: scripts/verify-worker-release.sh worker-vX.Y.Z[-alpha.N]" >&2
  exit 2
fi

for asset in "${expected_assets[@]}"; do
  test -s "${dist_dir}/${asset}"
done
test -s "${dist_dir}/checksums.txt"
test -s "${dist_dir}/worker-release-manifest.json"

(
  cd "${dist_dir}"
  shasum -a 256 -c checksums.txt
)

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT

for asset in "${expected_assets[@]}"; do
  asset_dir="${tmp_dir}/${asset%.tar.gz}"
  mkdir -p "${asset_dir}"
  tar -xzf "${dist_dir}/${asset}" -C "${asset_dir}"
  test -x "${asset_dir}/tc-worker"
done

native_suffix=""
case "$(uname -s)/$(uname -m)" in
  Darwin/x86_64) native_suffix="Darwin_x86_64" ;;
  Darwin/arm64) native_suffix="Darwin_arm64" ;;
  Linux/x86_64|Linux/amd64) native_suffix="Linux_x86_64" ;;
  Linux/aarch64|Linux/arm64) native_suffix="Linux_arm64" ;;
esac

if [[ -n "${native_suffix}" ]]; then
  native_bin="${tmp_dir}/tc-worker_${native_suffix}/tc-worker"
  version_output="$("${native_bin}" version)"
  if [[ "${version_output}" != *"${tag}"* ]]; then
    echo "version output does not include ${tag}: ${version_output}" >&2
    exit 1
  fi
  "${native_bin}" --help >/dev/null
  "${native_bin}" setup --help >/dev/null
  "${native_bin}" join --help >/dev/null
  "${native_bin}" doctor --help >/dev/null
fi

echo "verified tc-worker release assets for ${tag}"
