# tc-worker worker-v0.1.0-alpha.2

This is the first installable `tc-worker` alpha intended for local AI-worker onboarding tests across macOS and Linux.

## What is included

- `tc-worker` binaries for:
  - macOS arm64
  - macOS x86_64
  - Linux arm64
  - Linux x86_64
- `checksums.txt` for archive verification.
- `worker-release-manifest.json` with the release tag, commit, and expected asset list.

## Install

Prereleases are not served through GitHub's `latest` release pointer. Install this alpha with an explicit version:

```sh
curl -fsSL https://raw.githubusercontent.com/nangman-infra/touch-connect/main/scripts/install-worker.sh \
  | VERSION=worker-v0.1.0-alpha.2 sh
```

Then run:

```sh
tc-worker setup
tc-worker join
```

## Release verification

- Worker tests run with coverage before archive creation.
- `go vet ./tc-worker/...` must pass.
- Local docs validation runs when the untracked docs workspace is present.
- Installer shell syntax is checked.
- All four release archives are built from the same tag and commit.
- `checksums.txt` is verified before publishing.
- The native release binary must print the release version and expose `--help`, `setup --help`, `join --help`, and `doctor --help`.

## Notes

This is an alpha release. It is meant to validate the installation and local worker join surface before the broader manager CLI is released.
