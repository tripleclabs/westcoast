# Feature Specification: Location-Transparent PIDs

**Feature Branch**: `002-location-transparent-pids`  
**Created**: 2026-03-08  
**Status**: Draft  
**Input**: User description: "2. Location-Transparent Process Identifiers (PIDs) Description: Actors must never interact with each"

## Clarifications

### Session 2026-03-08

- Q: How should PID identity behave across actor restarts? → A: Stable logical PID plus internal generation validated by resolver.
- Q: Which delivery outcome contract should PID messaging use? → A: Strict closed enum outcomes.
- Q: What canonical PID shape should be used? → A: `namespace + actor_id + generation`.
- Q: What resolver latency target should this feature enforce? → A: Single-node resolver lookup p95 <= 25 µs.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Address Actors by PID (Priority: P1)

As a runtime developer, I can send messages using process identifiers (PIDs) instead of direct
runtime references, so actor interactions remain decoupled from in-memory placement.

**Why this priority**: PID-based addressing is the core boundary for location transparency.

**Independent Test**: Create actors, resolve by PID, send messages through PID only, and verify
messages are delivered without exposing implementation references.

**Acceptance Scenarios**:

1. **Given** an active actor with a PID, **When** a caller sends a message using only that PID,
   **Then** the target actor receives and processes the message.
2. **Given** a caller with no direct actor reference, **When** it interacts through PID APIs,
   **Then** behavior is equivalent to local direct interaction from a consumer perspective.

---

### User Story 2 - Preserve Stable PID Semantics Through Lifecycle Changes (Priority: P2)

As a platform owner, I can rely on consistent PID behavior when actors restart, stop, or are
replaced, so callers receive deterministic outcomes.

**Why this priority**: Lifecycle correctness prevents stale routing and hidden delivery failures.

**Independent Test**: Trigger stop/restart flows and verify PID resolution returns consistent
state-aware outcomes from the closed contract set (`delivered`, `unresolved`,
`rejected_stopped`, `rejected_stale_generation`, `rejected_not_found`).

**Acceptance Scenarios**:

1. **Given** an actor PID whose actor has stopped, **When** a message is sent, **Then** the
   runtime returns a deterministic unreachable/rejected outcome.
2. **Given** an actor restarted under supervision, **When** callers continue using the same logical
   PID with resolver-validated generation semantics, **Then** stale generation deliveries are
   rejected and current generation routing remains valid without caller-side reference changes.

---

### User Story 3 - Prepare PID Model for Multi-Node Routing (Priority: P3)

As an architecture owner, I can adopt PID metadata and resolution rules that remain valid when
routing extends beyond one node, so distributed evolution does not require caller contract changes.

**Why this priority**: Future multi-node support depends on preserving caller-facing identity rules now.

**Independent Test**: Validate PID format and resolver contract with local-only routing while
confirming no API assumptions require same-process memory access.

**Acceptance Scenarios**:

1. **Given** a PID created in the single-node runtime, **When** resolver metadata is inspected,
   **Then** it contains logical identity information sufficient for future remote routing seams.

---

### Failure & Recovery Scenarios *(mandatory)*

- If PID resolution fails, the runtime MUST return an explicit unresolved/unreachable outcome without
  blocking unrelated actor traffic.
- If a supervised actor restarts, PID mapping behavior MUST remain deterministic per documented
  lifecycle rules.
- Resolution and delivery failures MUST emit observable events for debugging and operational alerts.

### Location Transparency Impact *(mandatory)*

- Callers MUST use PID contracts rather than runtime object/pointer references.
- PID operations MUST not expose node-local memory identity details.
- Resolver interfaces MUST preserve compatibility with future transport/discovery layers.

### Edge Cases

- What happens when a message is sent to a PID that never existed?
- What happens when a PID references an actor that just terminated?
- How are stale PID caches handled after actor restart or replacement?
- How does the runtime prevent accidental dependence on local reference identity?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST issue a unique PID for each actor identity exposed to callers.
- **FR-002**: The system MUST allow message delivery by PID without requiring direct actor references.
- **FR-003**: The system MUST resolve PID-to-target mapping through a dedicated resolver boundary.
- **FR-004**: The system MUST return explicit outcomes from this closed set only:
  `delivered`, `unresolved`, `rejected_stopped`, `rejected_stale_generation`,
  `rejected_not_found`.
- **FR-005**: The system MUST preserve deterministic PID behavior across actor restart and stop events.
- **FR-006**: The system MUST define expected failure handling and supervision outcomes for PID routing.
- **FR-007**: The system MUST keep public contracts location-transparent.
- **FR-008**: The system MUST emit observable events for PID resolution success/failure and message delivery outcome.
- **FR-009**: The system MUST prevent caller APIs from exposing process-bound actor memory references.
- **FR-010**: The system MUST model PID identity as stable logical actor identity plus an internal
  generation/incarnation, and resolver validation MUST reject stale-generation deliveries.
- **FR-011**: The system MUST model canonical PID structure as `namespace + actor_id + generation`.
- **FR-012**: The system MUST meet resolver lookup latency of p95 <= 25 µs on the single-node
  baseline benchmark profile for this feature.

### Assumptions

- Scope for this feature is single-node runtime with forward-compatible PID semantics.
- Global uniqueness is required within one node; cross-node uniqueness strategy is deferred to
  distributed design work.
- Security/authentication layers for PID ownership are out of scope for this feature.

### Key Entities *(include if feature involves data)*

- **PID**: Opaque logical process identifier with `namespace`, `actor_id`, and `generation` fields.
- **PID Resolver**: Runtime component that maps PID to current actor routing target/state.
- **PID Route State**: Reachability status for a PID (reachable, unresolved, stopped, restarting).
- **Delivery Outcome**: Closed enum result returned for PID-based message attempts
  (`delivered`, `unresolved`, `rejected_stopped`, `rejected_stale_generation`,
  `rejected_not_found`).
- **PID Generation**: Internal incarnation value used by resolver to prevent stale message routing.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of external actor interactions in conformance tests use PID-based APIs only.
- **SC-002**: 100% of PID resolution failures return explicit, classified outcomes (no silent drops).
- **SC-003**: In lifecycle tests, PID behavior remains deterministic across restart/stop scenarios in at least 99.9% of cases.
- **SC-004**: Migration simulation confirms no caller contract changes are required when swapping from
  local-only resolver implementation to a transport-abstracted resolver interface.
- **SC-005**: Resolver lookup latency measured on the agreed single-node baseline profile is
  p95 <= 25 µs.
