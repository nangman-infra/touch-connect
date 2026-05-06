#!/usr/bin/env bash
set -euo pipefail

tag="${1:-}"
min_coverage="${TC_WORKER_MIN_COVERAGE:-50}"

if [[ -z "${tag}" ]]; then
  echo "usage: scripts/worker-release-readiness.sh worker-vX.Y.Z[-alpha.N]" >&2
  exit 2
fi

case "${tag}" in
  worker-v*) ;;
  *)
    echo "tc-worker release tags must start with worker-v: ${tag}" >&2
    exit 2
    ;;
esac

go test ./tc-worker/... -coverpkg=./tc-worker/... -coverprofile=coverage.worker.out
go vet ./tc-worker/...
if [[ -f "docs/README.md" ]]; then
  python3 scripts/validate_docs.py
else
  echo "docs/README.md not found; skipping local-only docs validation"
fi
sh -n scripts/install-worker.sh

coverage="$(go tool cover -func=coverage.worker.out | awk '/^total:/ {gsub(/%/, "", $3); print $3}')"
awk -v coverage="${coverage}" -v min="${min_coverage}" 'BEGIN {
  if (coverage + 0 < min + 0) {
    printf("tc-worker coverage %.1f%% is below required %.1f%%\n", coverage, min) > "/dev/stderr"
    exit 1
  }
}'

scripts/build-worker-release.sh "${tag}"
scripts/verify-worker-release.sh "${tag}"

echo "tc-worker release readiness passed for ${tag} with coverage ${coverage}%"
