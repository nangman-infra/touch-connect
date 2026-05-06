#!/usr/bin/env sh
set -eu

repo="${TC_WORKER_REPO:-nangman-infra/touch-connect}"
version="${TC_WORKER_VERSION:-${VERSION:-latest}}"
install_dir="${TC_INSTALL_DIR:-${INSTALL_DIR:-$HOME/.local/bin}}"

case "$(uname -s)" in
  Darwin) os="Darwin" ;;
  Linux) os="Linux" ;;
  *) echo "unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="x86_64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

asset="tc-worker_${os}_${arch}.tar.gz"
if [ "$version" = "latest" ]; then
  base_url="https://github.com/${repo}/releases/latest/download"
else
  base_url="https://github.com/${repo}/releases/download/${version}"
fi

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

fetch() {
  url="$1"
  out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$out"
    return
  fi
  echo "curl or wget is required" >&2
  exit 1
}

echo "downloading ${asset} from ${repo} (${version})"
fetch "${base_url}/${asset}" "${tmp_dir}/${asset}"

if fetch "${base_url}/checksums.txt" "${tmp_dir}/checksums.txt" 2>/dev/null; then
  expected="$(grep "  ${asset}$" "${tmp_dir}/checksums.txt" | awk '{print $1}' || true)"
  if [ -n "$expected" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      actual="$(sha256sum "${tmp_dir}/${asset}" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
      actual="$(shasum -a 256 "${tmp_dir}/${asset}" | awk '{print $1}')"
    else
      actual=""
      echo "sha256 tool not found; skipping checksum verification"
    fi
    if [ -n "$actual" ] && [ "$actual" != "$expected" ]; then
      echo "checksum mismatch for ${asset}" >&2
      exit 1
    fi
  fi
fi

tar -xzf "${tmp_dir}/${asset}" -C "$tmp_dir"
if [ ! -f "${tmp_dir}/tc-worker" ]; then
  echo "release archive does not contain tc-worker" >&2
  exit 1
fi

mkdir -p "$install_dir"
install -m 0755 "${tmp_dir}/tc-worker" "${install_dir}/tc-worker"

echo "installed ${install_dir}/tc-worker"
case ":$PATH:" in
  *":${install_dir}:"*) ;;
  *) echo "warning: ${install_dir} is not on PATH" ;;
esac
echo "next: tc-worker setup && tc-worker join"
