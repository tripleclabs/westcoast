# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog, and this project adheres to Semantic Versioning.

## [Unreleased]

### Added

- CI workflow with formatting, vet, tests, race tests, and benchmark smoke checks.
- Runtime stop-hook timeout controls to prevent indefinite shutdown blocking.
- Distributed readiness history retention cap via ring-buffer behavior.

### Changed

- Cross-actor actor-ID send path now preserves caller context.
- PID interaction policy reads/writes are synchronized.

## [0.1.0] - 2026-03-09

### Added

- Initial actor runtime, lifecycle hooks, PID resolver, registry, and supervision behavior.
