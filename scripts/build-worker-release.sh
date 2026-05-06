#!/usr/bin/env bash
set -euo pipefail

tag="${1:-}"
if [[ -z "${tag}" ]]; then
  echo "usage: scripts/build-worker-release.sh worker-vX.Y.Z[-alpha.N]" >&2
  exit 2
fi

case "${tag}" in
  worker-v*) ;;
  *)
    echo "tc-worker release tags must start with worker-v: ${tag}" >&2
    exit 2
    ;;
esac

commit="${GITHUB_SHA:-$(git rev-parse HEAD)}"
dist_dir="${TC_WORKER_DIST_DIR:-dist}"

rm -rf "${dist_dir}"
mkdir -p "${dist_dir}"

build_one() {
  local goos="$1"
  local goarch="$2"
  local suffix="$3"
  local work="${dist_dir}/build-${suffix}"

  mkdir -p "${work}"
  CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${tag} -X main.commit=${commit}" \
    -o "${work}/tc-worker" \
    ./tc-worker/cmd/tc-worker
  tar -C "${work}" -czf "${dist_dir}/tc-worker_${suffix}.tar.gz" tc-worker
}

build_one darwin amd64 Darwin_x86_64
build_one darwin arm64 Darwin_arm64
build_one linux amd64 Linux_x86_64
build_one linux arm64 Linux_arm64

(
  cd "${dist_dir}"
  shasum -a 256 tc-worker_*.tar.gz > checksums.txt
)

cat > "${dist_dir}/worker-release-manifest.json" <<EOF
{
  "name": "tc-worker",
  "tag": "${tag}",
  "commit": "${commit}",
  "assets": [
    "tc-worker_Darwin_x86_64.tar.gz",
    "tc-worker_Darwin_arm64.tar.gz",
    "tc-worker_Linux_x86_64.tar.gz",
    "tc-worker_Linux_arm64.tar.gz",
    "checksums.txt"
  ]
}
EOF

echo "built tc-worker release assets in ${dist_dir}"
