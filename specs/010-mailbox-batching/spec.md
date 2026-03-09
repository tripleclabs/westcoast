# Feature Specification: Smart Mailboxes and Message Batching

**Feature Branch**: `010-mailbox-batching`  
**Created**: 2026-03-09  
**Status**: Draft  
**Input**: User description: "Smart Mailboxes & Message Batching Description: Advanced mailbox capabilities that allow actors to process high-throughput data streams efficiently, preventing I/O bottlenecks. Requirements: Bulk Retrieval: An opt-in BatchReceive([]any) interface that allows an actor to consume multiple queued messages from its mailbox in a single execution cycle (e.g., pulling 50 buffered messages at once). I/O Optimization: Batching must facilitate efficient downstream operations, such as allowing a database-writer actor to execute a single bulk SQL INSERT rather than 50 individual queries. This is feature number 10"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Process Batched Workloads (Priority: P1)

As a runtime user building high-throughput actors, I can opt into batched mailbox retrieval so one actor execution cycle can process multiple queued messages at once.

**Why this priority**: This is the core value of the feature and directly addresses throughput and mailbox pressure.

**Independent Test**: Configure a batching-enabled actor, enqueue many messages, and verify each execution cycle receives batches larger than one while preserving message handling correctness.

**Acceptance Scenarios**:

1. **Given** an actor with batching enabled and at least 50 queued messages, **When** the actor runs one cycle, **Then** it can receive and process a batch of multiple messages in that cycle.
2. **Given** an actor with batching enabled and fewer messages than batch limit, **When** the actor runs a cycle, **Then** it receives exactly the available queued messages without waiting for more.

---

### User Story 2 - Optimize I/O with Bulk Operations (Priority: P2)

As a runtime user implementing a sink actor (for example, database writer), I can consume a message batch and issue one downstream bulk operation instead of many one-by-one operations.

**Why this priority**: This provides direct operational benefit by reducing external I/O calls and bottlenecks.

**Independent Test**: Use a batching-enabled sink actor with a downstream operation counter; send N messages and verify fewer downstream operations than messages, while all messages are represented in outcomes.

**Acceptance Scenarios**:

1. **Given** a batching-enabled sink actor, **When** 100 messages are submitted, **Then** downstream processing can occur in grouped operations rather than one operation per message.
2. **Given** batching is disabled for an actor, **When** messages are submitted, **Then** behavior remains compatible with single-message processing semantics.

---

### User Story 3 - Preserve Predictable Failure and Recovery (Priority: P3)

As an operator, I can rely on explicit and deterministic outcomes when batched processing fails or an actor restarts, so I can reason about recovery without hidden data loss.

**Why this priority**: High-throughput features are only usable in production if failure behavior stays clear and observable.

**Independent Test**: Force handler failure during batch processing and verify supervision outcomes, emitted telemetry, and continued processing behavior after restart policy is applied.

**Acceptance Scenarios**:

1. **Given** a batching-enabled actor and a failing batch handler, **When** the actor processes a batch, **Then** supervision policy outcomes are emitted and follow configured restart/stop/escalate behavior.
2. **Given** actor restart after batch failure, **When** new messages are sent, **Then** the actor continues processing with the configured batching mode.

### Failure & Recovery Scenarios *(mandatory)*

- If a batch handler fails, runtime MUST emit explicit failure outcome and apply existing supervision policy decision.
- If part of a batch cannot be processed by actor logic, outcome telemetry MUST identify failure class and preserve deterministic actor lifecycle behavior.
- If actor restarts while mailbox still contains queued work, restart behavior MUST be explicit and consistent with configured supervision/mailbox semantics.
- If a batching-enabled actor is stopped gracefully, stop behavior MUST remain bounded and observable.

### Location Transparency Impact *(mandatory)*

- Callers continue to send messages to logical actor identity/PID; callers do not depend on mailbox internals.
- Batching behavior is internal to actor runtime execution and does not change caller addressing model.
- Public contracts remain compatible with future multi-node transport because batching does not expose in-process reference identity.

### Edge Cases

- Batch size configuration set to zero or negative must fall back to deterministic default behavior.
- Batch-enabled actor with empty mailbox must not spin or emit false success outcomes.
- Mixed message types within one batch must follow existing routing/validation semantics.
- Very large queue depth must still cap retrieval per cycle to configured maximum batch size.
- Timeout/cancellation interactions must remain deterministic for request-response patterns that coexist with batching.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide an opt-in batching mode for actors so a single execution cycle can consume multiple queued messages.
- **FR-002**: System MUST support bulk retrieval semantics equivalent to a `BatchReceive([]any)` capability.
- **FR-003**: System MUST allow configuration of maximum messages retrieved per batch cycle.
- **FR-004**: System MUST preserve deterministic message ordering guarantees within each actor mailbox according to existing runtime ordering policy.
- **FR-005**: System MUST allow batching-enabled actors to process grouped messages in a way that enables downstream bulk operations.
- **FR-006**: System MUST preserve compatibility for actors that do not opt into batching.
- **FR-007**: System MUST emit explicit outcomes for batch success and failure paths.
- **FR-008**: System MUST define expected batch-failure supervision outcomes (restart/stop/escalate) using existing supervision model.
- **FR-009**: System MUST keep public messaging/addressing contracts location-transparent.
- **FR-010**: System MUST bound per-cycle batch retrieval so mailbox draining does not starve runtime fairness.

### Key Entities *(include if feature involves data)*

- **BatchMailboxConfig**: Runtime configuration that determines whether batching is enabled and the maximum batch size per actor cycle.
- **BatchEnvelope**: The set of messages retrieved from mailbox for one execution cycle.
- **BatchProcessingOutcome**: Observable record of a batch execution, including actor identity, batch size, result, and failure reason when applicable.
- **DownstreamBulkOperation**: Logical grouped action emitted by a batching-enabled actor to reduce external I/O calls.

## Assumptions

- Batching is opt-in per actor and not enabled globally by default.
- Existing supervision semantics remain authoritative; this feature extends behavior without introducing a second supervision model.
- Existing request-response and lifecycle semantics continue to work when batching is enabled.
- This feature is scoped to single-node runtime behavior for now.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For batching-enabled actors under sustained queue load, at least 90% of execution cycles process more than one message.
- **SC-002**: For a sink actor workload of 1,000 input messages, downstream operation count is reduced by at least 70% compared with one-by-one processing mode.
- **SC-003**: 100% of forced batch-handler failures produce explicit, test-verifiable supervision outcomes.
- **SC-004**: Non-batching actors retain existing behavior with no regressions in current contract and integration test suites.
