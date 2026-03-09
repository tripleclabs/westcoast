# Feature Specification: Actor Routers and Worker Pools

**Feature Branch**: `009-actor-router-pools`  
**Created**: 2026-03-09  
**Status**: Draft  
**Input**: User description: "Actor Routers & Worker Pools with stateless worker-pool routing (round-robin/random) and consistent-hash sharding via HashKey() so messages for the same key always route to the same worker."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Stateless Pool Routing (Priority: P1)

As a runtime user, I can send messages to a router actor that distributes work across a pool of identical workers using a stateless strategy so concurrent workloads are parallelized without manually picking workers.

**Why this priority**: This delivers immediate throughput and load-balancing value for the most common pool use case.

**Independent Test**: Configure a router with multiple workers and round-robin/random strategy, submit a burst of messages, and verify work is distributed across more than one worker while all messages are processed.

**Acceptance Scenarios**:

1. **Given** a router configured with round-robin and N workers, **When** M messages are sent to the router, **Then** messages are dispatched across workers in deterministic round-robin order.
2. **Given** a router configured with random and N workers, **When** M messages are sent to the router, **Then** messages are distributed across the pool without sender blocking.

---

### User Story 2 - Consistent Hash Sharding (Priority: P2)

As a runtime user handling key-scoped state (for example, per user), I can route messages through a consistent-hash strategy so all messages with the same key always reach the same worker.

**Why this priority**: Sharded routing prevents state-locking conflicts and is required for safe stateful workloads.

**Independent Test**: Send repeated messages with identical keys and different keys through a hash router, then verify identical keys always hit the same worker while different keys can map to different workers.

**Acceptance Scenarios**:

1. **Given** a consistent-hash router and messages implementing `HashKey()`, **When** multiple messages with the same key are sent, **Then** they are always dispatched to the exact same worker.
2. **Given** a consistent-hash router and messages with different keys, **When** messages are sent, **Then** keys are deterministically mapped across the available worker set.

---

### User Story 3 - Router Fault and Pool Recovery Behavior (Priority: P3)

As an operator, I can rely on predictable routing behavior when workers fail or pool membership changes so routing remains safe and observable.

**Why this priority**: Production pool routing requires deterministic behavior under worker failure and topology changes.

**Independent Test**: Induce worker failure and restart in a routed workload, then verify routing outcomes remain explicit, messages are not silently lost, and post-recovery routing continues per configured strategy.

**Acceptance Scenarios**:

1. **Given** a routed worker fails, **When** the router attempts dispatch, **Then** failure outcomes are observable and supervision behavior remains explicit.
2. **Given** a worker pool recovers, **When** new messages are routed, **Then** router dispatch resumes according to the configured strategy guarantees.

### Failure & Recovery Scenarios *(mandatory)*

- If a target worker in the pool is unavailable, router dispatch outcome must be explicit and deterministic.
- If the router actor fails, supervision policy (restart/stop/escalate) governs recovery and routing availability.
- If consistent-hash receives a message without a valid hash key, routing must fail deterministically with clear error outcome.
- If pool membership changes, deterministic mapping behavior must be preserved according to defined rebalance rules and observable outcomes.

### Location Transparency Impact *(mandatory)*

- Senders target a logical router actor identity; they do not depend on direct worker references.
- Router-to-worker dispatch uses existing actor identity/PID abstractions, preserving transport-agnostic contracts.
- Hash-shard routing contract remains key-based, not node-location-based, so future multi-node routing can preserve caller semantics.

### Edge Cases

- Worker pool size of one behaves as pass-through routing.
- Worker pool size zero must reject dispatch with deterministic failure outcome.
- Random strategy must remain valid under high concurrency and must not panic on contention.
- Round-robin counters must remain correct under concurrent sends.
- Hash collisions across distinct keys are handled deterministically by the chosen hash strategy.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a router actor capability that dispatches incoming messages to a worker pool.
- **FR-002**: System MUST support stateless round-robin routing across configured workers.
- **FR-003**: System MUST support stateless random routing across configured workers.
- **FR-004**: System MUST support consistent-hash routing for messages that expose a hash key.
- **FR-005**: System MUST guarantee that messages with the same hash key route to the same worker while pool membership remains unchanged.
- **FR-006**: System MUST reject consistent-hash dispatch when required hash key data is missing or invalid.
- **FR-007**: System MUST expose deterministic dispatch outcomes for success and failure cases.
- **FR-008**: System MUST preserve non-blocking sender behavior for routed dispatch in normal operation.
- **FR-009**: System MUST define failure handling and supervision expectations for router and worker failures.
- **FR-010**: System MUST keep router-facing contracts location-transparent and independent of in-process references.

### Key Entities *(include if feature involves data)*

- **Router Definition**: Configuration describing routing strategy, worker pool membership, and active routing state.
- **Worker Pool**: Logical collection of worker actors eligible for routed dispatch.
- **Shard Key Contract**: Message-level hash key contract used by consistent routing to assign worker ownership.
- **Routing Outcome Record**: Observable record of dispatch decision/result for diagnostics and test verification.

## Assumptions

- Router behavior is implemented within the existing single-node runtime for this phase.
- Worker pool membership is explicitly configured by runtime users and not auto-discovered.
- Consistent-hash guarantees apply while pool membership is stable; membership changes may remap keys according to documented behavior.
- Existing supervision and observability patterns are reused for router/worker lifecycle events.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In round-robin mode with at least 4 workers and 1,000 routed messages, each worker receives at least 20% of messages and at most 30%.
- **SC-002**: In consistent-hash mode with stable pool membership, 100% of repeated messages for the same key route to the same worker.
- **SC-003**: Under concurrent routed load, 99% of router dispatch operations complete without sender timeout attributable to router coordination.
- **SC-004**: 100% of invalid hash-key or zero-worker routing attempts return explicit, test-verifiable failure outcomes.
