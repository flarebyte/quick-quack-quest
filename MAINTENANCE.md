# Maintenance Guide

## Purpose
This document is the default maintenance reference for contributors and AI agents working on `quick-quack-quest`.

## Build Targets Policy
- Current `gh flarebyte` build target is intentionally limited to:
  - `darwin-arm64`
- Rationale:
  - This project uses DuckDB Go bindings and CGO.
  - Cross-compiling CGO targets from macOS to Linux requires a Linux C cross-compiler toolchain.
  - Without that toolchain, `gh flarebyte build` fails for Linux targets even when Go code is correct.

## CGO Notes
- `gh flarebyte build` now supports CGO and reports effective settings.
- For non-native targets (for example `linux-amd64` on macOS), CGO requires target-specific C toolchains.
- Typical local-safe approach:
  - Build only native `darwin-arm64` locally.
  - Add Linux targets later in CI on Linux runners or after configuring cross toolchains.

## Common Commands
- Validate config:
  - `gh flarebyte config validate --config .gh-flarebyte.cue`
- Build release artifacts (configured targets only):
  - `make build`
- Build local binary into `build/` (never repo root):
  - `make build-local`
- Lint:
  - `make lint`
- Test:
  - `make test`
- Coverage gate:
  - `make coverage`

## Coverage Policy
- Coverage threshold is configured in `.gh-flarebyte.cue`.
- `make coverage` delegates to `gh flarebyte cov` and enforces configured minimums.

## If Re-enabling Linux Target
Before adding `linux-amd64` back to `.gh-flarebyte.cue`:
1. Ensure a Linux-compatible C toolchain is available for CGO builds.
2. Verify `gh flarebyte build --target linux-amd64` succeeds locally or in CI.
3. Keep diagnostics for `GOOS`, `GOARCH`, `CGO_ENABLED`, `CC`, and `CXX` visible in build logs.
