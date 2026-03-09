# Feature Specification: Type-Agnostic Local Messaging

**Feature Branch**: `003-local-struct-messaging`  
**Created**: 2026-03-08  
**Status**: Draft  
**Input**: User description: "Feature 3: Type-Agnostic Local Messaging Description: A high-performance local messaging protocol that avoids serialization overhead. Requirements: Native Struct Passing: For intra-node communication, the system must allow passing native language data structures directly through the mailboxes. Zero-Serialization: The system must strictly avoid encoding/decoding overhead (like JSON or Protobuf) when the sender and receiver reside on the same node. Pattern Matching Emulation: The receiving actor must be able to easily inspect the message type and route its internal logic accordingly."

## Clarifications

### Session 2026-03-08

- Q: How should unsupported message types be handled? → A: Explicitly reject with `rejected_unsupported_type` and emit an event.
- Q: What concrete performance gate should local messaging enforce? → A: p95 local send latency <= 25 µs and throughput >= 1,000,000 msgs/sec on agreed baseline profile.
- Q: How should type-routing precedence work? → A: Exact type match first, optional explicit fallback, otherwise reject unsupported.
- Q: How should nil payloads be handled? → A: Explicitly reject with `rejected_nil_payload`.
- Q: How should message schema-version mismatch be handled? → A: Require exact type+version match and reject mismatch.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Send Native Local Messages (Priority: P1)

As a runtime developer, I can send native structured messages between local actors without
transforming payloads, so local communication remains fast and direct.

**Why this priority**: Native local messaging is the core value of this feature.

**Independent Test**: Send structured messages between actors on one node and verify the receiver
gets the original message shape without serialization side effects.

**Acceptance Scenarios**:

1. **Given** two local actors, **When** one actor sends a native structured message,
   **Then** the receiver processes that message without requiring encode/decode steps.
2. **Given** multiple message shapes sent to one actor, **When** messages are delivered,
   **Then** each arrives with its original type information intact.

---

### User Story 2 - Enforce Zero-Serialization in Intra-Node Paths (Priority: P2)

As a platform owner, I can ensure local messaging paths never incur serialization overhead,
so performance remains predictable under load.

**Why this priority**: Performance goals depend on strict zero-serialization behavior.

**Independent Test**: Execute local actor messaging workloads and verify delivery path behavior
contains no local encode/decode phase while maintaining correctness.

**Acceptance Scenarios**:

1. **Given** local sender and receiver actors, **When** messages are exchanged,
   **Then** delivery outcomes show no local serialization step is required.
2. **Given** high-frequency local traffic, **When** message processing is observed,
   **Then** throughput remains stable and unaffected by serialization overhead.

---

### User Story 3 - Route Logic by Message Type (Priority: P3)

As an actor developer, I can match incoming messages by type and route behavior branches,
so actor code can emulate pattern matching clearly.

**Why this priority**: Type-directed logic is required for ergonomic actor message handling.

**Independent Test**: Deliver mixed message types to one actor and verify each type triggers the
correct handling branch with deterministic outcomes.

**Acceptance Scenarios**:

1. **Given** an actor receiving heterogeneous message types, **When** each message arrives,
   **Then** the actor dispatches to the correct type-specific behavior path.

---

### Failure & Recovery Scenarios *(mandatory)*

- If an unsupported message type is received, the actor MUST return
  `rejected_unsupported_type` and emit a corresponding failure event.
- If a handler fails while matching or processing a type branch, failure handling MUST stay isolated
  to the impacted actor and follow supervision policy.
- Local messaging failures and type-routing mismatches MUST emit observable outcomes for diagnosis.

### Location Transparency Impact *(mandatory)*

- This feature applies to local delivery behavior only and MUST preserve existing location-transparent
  caller contracts.
- Message contracts MUST remain transport-agnostic so distributed routing can add serialization later
  without changing local caller-facing semantics.
- No caller API may require direct process-bound references for type-routing logic.

### Edge Cases

- What happens when a local receiver gets an unsupported message type?
- Nil payloads MUST be rejected with `rejected_nil_payload`.
- Version mismatch MUST be rejected with `rejected_version_mismatch`.
- How does the system avoid ambiguous routing when two handlers target overlapping type categories?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST support local actor-to-actor messaging with native structured payloads.
- **FR-002**: The system MUST preserve message type information across local mailbox delivery.
- **FR-003**: The system MUST avoid local encoding/decoding steps for intra-node message delivery.
- **FR-004**: The system MUST provide deterministic outcomes for unsupported or invalid message types.
- **FR-005**: The system MUST allow actor handlers to route behavior by incoming message type.
- **FR-006**: The system MUST define expected failure handling and supervision outcomes for type-routed processing.
- **FR-007**: The system MUST keep public contracts location-transparent.
- **FR-008**: The system MUST emit observable outcomes for local message handling success/failure.
- **FR-009**: Unsupported message types MUST return `rejected_unsupported_type` and MUST emit an
  explicit rejection event.
- **FR-010**: Local messaging MUST meet both performance gates on the agreed baseline profile:
  p95 local send latency <= 25 µs and throughput >= 1,000,000 msgs/sec.
- **FR-011**: Type routing MUST prioritize exact message-type matches first; if no exact match exists,
  routing MAY use one explicit fallback handler, otherwise the message MUST be rejected as unsupported.
- **FR-012**: Nil message payloads MUST be rejected with outcome `rejected_nil_payload` and MUST emit
  an explicit rejection event.
- **FR-013**: Type-routed local messages MUST require exact type and schema-version match; mismatches
  MUST return `rejected_version_mismatch` and MUST emit an explicit rejection event.

### Assumptions

- This feature scope is limited to single-node actor communication paths.
- Transport-level serialization for cross-node messaging is out of scope and will be handled by
  distributed extensions later.
- Message type routing rules are defined per actor and are deterministic within one actor instance.

### Key Entities *(include if feature involves data)*

- **Local Message**: Native structured payload passed between actors on the same node.
- **Message Type Descriptor**: Metadata used by the receiver to identify the message type branch.
- **Type Routing Rule**: Actor-defined mapping from message type to handling branch.
- **Message Handling Outcome**: Result emitted after processing or rejecting a message.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In conformance tests, 100% of local actor message deliveries preserve original message type information.
- **SC-002**: In local-path instrumentation tests, 100% of intra-node deliveries complete without local encode/decode stages.
- **SC-003**: In mixed-message tests, at least 99.9% of messages route to the correct type-handling branch.
- **SC-004**: In load tests, local messaging throughput meets target profile without serialization-related degradation.
- **SC-005**: On the agreed baseline profile, p95 local send latency is <= 25 µs and throughput is
  >= 1,000,000 msgs/sec.
