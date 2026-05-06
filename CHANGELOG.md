# Changelog

All notable touch-connect release changes are documented here.

## [worker-v0.1.0-alpha.2] - 2026-05-07

Release: https://github.com/nangman-infra/touch-connect/releases/tag/worker-v0.1.0-alpha.2

Compare: https://github.com/nangman-infra/touch-connect/compare/worker-v0.1.0-alpha.1...worker-v0.1.0-alpha.2

### Added

- Added a release-readiness gate for `tc-worker` that runs worker tests with coverage, `go vet`, local docs validation when the untracked docs workspace is present, installer syntax validation, cross-platform archive builds, checksum verification, and native binary help/version smoke checks.
- Added reusable release build and verification scripts for macOS and Linux on amd64/x86_64 and arm64.
- Added curated GitHub release notes for the worker alpha channel.

### Changed

- Updated the `tc-worker release` GitHub Actions workflow to use the shared release-readiness scripts instead of an inline-only minimal build path.
- Marked alpha, beta, and release-candidate worker tags as GitHub prereleases and kept prereleases out of the repository `latest` release pointer.
- Made `scripts/install-worker.sh` accept both `TC_WORKER_VERSION` and the shorter `VERSION` alias for explicit prerelease installation.

### Release Verification

- Local `scripts/worker-release-readiness.sh worker-v0.1.0-alpha.2` passed before tagging, including local docs validation.
- GitHub Actions `tc-worker sonar` passed on `main` before tagging.
- GitHub Actions `tc-worker release` must pass before this release is considered installable.

## [worker-v0.1.0-alpha.1] - 2026-05-07

Release: https://github.com/nangman-infra/touch-connect/releases/tag/worker-v0.1.0-alpha.1

### Added

- Published the first `tc-worker` prerelease archives for macOS and Linux on amd64/x86_64 and arm64.
- Published checksums for the worker archives.

### Release Verification

- GitHub Actions `tc-worker release` passed.
- This release was intentionally superseded by `worker-v0.1.0-alpha.2`, which adds the full release-readiness surface.
