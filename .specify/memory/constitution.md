<!--
Sync Impact Report
- Version change: 0.0.0 (template) -> 1.0.0
- Modified principles:
  - Template Principle 1 -> I. Go-Native Actor Core
  - Template Principle 2 -> II. Let-It-Crash Fault Tolerance
  - Template Principle 3 -> III. Test-First Engineering (NON-NEGOTIABLE)
  - Template Principle 4 -> IV. Location Transparency by Design
  - Template Principle 5 -> V. Performance Through Simplicity
- Added sections:
  - Technical Guardrails
  - Delivery Workflow & Quality Gates
- Removed sections:
  - None
- Templates requiring updates:
  - ✅ updated: .specify/templates/plan-template.md
  - ✅ updated: .specify/templates/spec-template.md
  - ✅ updated: .specify/templates/tasks-template.md
  - ⚠ pending (not present): .specify/templates/commands/*.md
  - ✅ reviewed (no changes required): .specify/templates/agent-file-template.md
- Follow-up TODOs:
  - None
-->
# Westcoast Actor System Constitution

## Core Principles

### I. Go-Native Actor Core
The runtime MUST be implemented natively in Go and prioritize idiomatic Go APIs, toolchain,
and package layout. External schema-first serialization frameworks are prohibited in the
single-node runtime path; message handling MUST rely on lightweight Go-native representations.
This keeps latency and operational complexity low while preserving developer ergonomics.

### II. Let-It-Crash Fault Tolerance
Actor failures MUST be isolated, observable, and recoverable through supervisor strategies
rather than defensive error swallowing. Actors MUST fail fast on invalid state and delegate
recovery to explicit supervision policies (restart, stop, escalate). This enforces predictable
resilience behavior and mirrors Erlang/Elixir fault domains.

### III. Test-First Engineering (NON-NEGOTIABLE)
All behavior changes MUST follow Red-Green-Refactor: write failing tests first, implement the
minimal passing change, then refactor safely. Tests MUST cover actor lifecycle, mailbox
semantics, supervision outcomes, and regressions for defects. Code without failing-then-passing
test evidence is non-compliant.

### IV. Location Transparency by Design
Public actor interactions MUST use logical actor identities and abstract addressing interfaces,
not transport- or process-bound references. Single-node features MUST define clear seams for
future multi-node transport, discovery, and routing without changing caller-facing contracts.
This prevents rewrites during distributed expansion.

### V. Performance Through Simplicity
Designs MUST optimize for low overhead and predictable performance before adding abstraction.
Every added dependency, synchronization primitive, or allocation-heavy path MUST be justified
with measurable benefit. Prefer straightforward, composable components over framework-heavy
patterns.

## Technical Guardrails

- Language baseline MUST target stable Go with modules and standard tooling (`go test`, `go vet`,
  `gofmt`).
- APIs MUST be context-aware where cancellation and deadlines are relevant.
- Observability MUST include structured logs and metrics around actor restarts, mailbox depth,
  processing latency, and supervisor events.
- Backward-incompatible public API changes MUST include migration notes in the corresponding spec
  and plan artifacts.

## Delivery Workflow & Quality Gates

- Specs MUST define failure scenarios, supervision expectations, and location-transparency impact.
- Plans MUST pass Constitution Check gates before implementation starts and after design updates.
- Tasks MUST include test creation before implementation tasks for each user story.
- Pull requests MUST include: failing test proof (or commit history), passing test run, and
  performance impact notes for hot-path changes.

## Governance

This constitution overrides conflicting local practices for this repository.

Amendment process:
1. Propose changes via pull request updating `.specify/memory/constitution.md` and impacted
   templates/docs.
2. Document rationale, migration impact, and semantic version bump type.
3. Obtain approval from maintainers before merge.

Versioning policy:
- MAJOR: Removes or redefines a principle or governance requirement incompatibly.
- MINOR: Adds a new principle/section or materially expands required guidance.
- PATCH: Clarifies wording without changing required behavior.

Compliance review expectations:
- Every feature plan and task list MUST demonstrate alignment with all five principles.
- Reviewers MUST block merges that violate MUST-level requirements unless a constitution
  amendment is merged first.
- A constitution compliance check MUST be performed during planning and before release tagging.

**Version**: 1.0.0 | **Ratified**: 2026-03-08 | **Last Amended**: 2026-03-08
