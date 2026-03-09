# Release Policy

## Versioning

This project follows Semantic Versioning (`MAJOR.MINOR.PATCH`).

- `MAJOR`: incompatible API/behavior changes.
- `MINOR`: backward-compatible features.
- `PATCH`: backward-compatible fixes only.

## Compatibility Contract

- Public API includes exported symbols under `src/actor` and documented behavior in `README.md`.
- Changes to event/result string values are considered breaking unless explicitly deprecated first.

## Release Cadence

- Normal releases are cut from `main` after CI is green.
- Emergency patch releases may be cut from a hotfix branch and merged back to `main`.

## Release Checklist

1. Confirm CI (`.github/workflows/ci.yml`) is green on target commit.
2. Run locally: `go test ./...`, `go test -race ./tests/unit/... ./tests/integration/... ./tests/contract/...`, `go vet ./...`.
3. Update `CHANGELOG.md` under `Unreleased` and move entries into the new version section.
4. Tag release commit as `vMAJOR.MINOR.PATCH`.
5. Publish release notes from changelog entries.

## Deprecation Policy

- Mark deprecated APIs in docs one minor release before removal.
- Actual removal only in the next major release.
