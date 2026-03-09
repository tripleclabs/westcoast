# Feature Specification: Supervisor Trees & Fault Tolerance

**Feature Branch**: `004-supervisor-fault-tolerance`  
**Created**: 2026-03-09  
**Status**: Draft  
**Input**: User description: "Supervisor Trees & Fault Tolerance Description: Implementation of the Let it Crash philosophy. The system must assume that individual actors will encounter fatal errors and provide a resilient way to recover. Requirements: Crash Interception: The runtime must catch and isolate fatal panics within an actor, preventing the entire application from crashing. Clean State Restarts: When an actor crashes, the system must automatically respawn it with a fresh, uncorrupted initial state. Mailbox Preservation: During a restart, the"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Isolate Actor Crashes (Priority: P1)

As a runtime consumer, I need actor-level panics to be intercepted and isolated so a single actor crash
never terminates the runtime or other healthy actors.

**Why this priority**: Crash isolation is the minimum requirement for "let it crash" behavior and is a
hard prerequisite for all recovery flows.

**Independent Test**: Trigger a panic in one actor while another actor continues processing messages; verify
the runtime remains available and the healthy actor remains responsive.

**Acceptance Scenarios**:

1. **Given** two running actors, **When** one actor panics while handling a message, **Then** the runtime
   remains alive and the other actor continues handling new messages.
2. **Given** an actor panic, **When** failure is recorded, **Then** a structured failure event is emitted with
   actor identity, message identity, and failure classification.

---

### User Story 2 - Restart with Clean State (Priority: P2)

As a runtime consumer, I need crashed actors to restart from a fresh initial state so corrupted state is
not carried forward after failure.

**Why this priority**: Clean restart semantics are the core resilience behavior once crashes are isolated.

**Independent Test**: Send state-mutating messages, force a panic, then send a state query and verify state
matches initial values and not pre-crash mutated values.

**Acceptance Scenarios**:

1. **Given** an actor with mutated state, **When** the actor panics and restart policy is `restart`, **Then**
   the actor resumes in running status with initial state restored.
2. **Given** repeated actor crashes, **When** the restart limit is exceeded, **Then** the configured supervisor
   outcome (`stop` or `escalate`) is applied deterministically.

---

### User Story 3 - Preserve Mailbox During Restart (Priority: P3)

As a runtime consumer, I need pending messages to survive actor restarts so in-flight work is not silently
lost during failure recovery.

**Why this priority**: Mailbox preservation prevents accidental message loss and enables predictable recovery
under transient actor failures.

**Independent Test**: Queue multiple messages, force a panic on one message, verify remaining queued messages
are still delivered after actor restart in FIFO order.

**Acceptance Scenarios**:

1. **Given** a queued mailbox with unprocessed messages, **When** actor restart occurs, **Then** queued
   unprocessed messages remain available for post-restart processing.
2. **Given** a panic while processing message N, **When** actor restarts, **Then** message N outcome is recorded
   as failed and messages N+1 onward are still processed according to mailbox order.

### Failure & Recovery Scenarios *(mandatory)*

- Actor panic during message handling MUST be isolated to that actor instance.
- Supervisor decision MUST be explicit per failure: `restart`, `stop`, or `escalate`.
- Restarted actors MUST begin from initial state snapshot and running status.
- Mailbox items not yet started at crash time MUST be preserved across restart.
- Runtime MUST emit observable failure and recovery events for each crash lifecycle stage.

### Location Transparency Impact *(mandatory)*

- Callers continue to address actors via logical actor identifiers and PIDs, not in-process pointers.
- Crash/restart behavior MUST not require caller-side direct references to actor memory.
- Failure/restart outcomes MUST remain compatible with future remote routing where actor location may change.

### Edge Cases

- Actor panics repeatedly in a tight loop (restart storm).
- Panic occurs while mailbox is near capacity.
- Supervisor policy is `stop` and mailbox still contains pending messages.
- Actor receives new messages concurrently while restarting.
- Panic is caused by malformed payload and repeats on retried message classes.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST intercept actor panics during message handling and prevent process-wide runtime crash.
- **FR-002**: System MUST emit a failure event for each intercepted actor panic.
- **FR-003**: System MUST apply supervisor policy deterministically for each actor failure.
- **FR-004**: System MUST support at least `restart`, `stop`, and `escalate` supervision outcomes.
- **FR-005**: When policy outcome is `restart`, system MUST respawn the actor with its configured initial state.
- **FR-006**: When policy outcome is `restart`, system MUST transition actor status back to running after restart.
- **FR-007**: System MUST preserve mailbox messages that were not yet started at crash time during restart.
- **FR-008**: System MUST record a terminal failure outcome for the crashed message that triggered panic.
- **FR-009**: System MUST preserve FIFO ordering for remaining queued messages after restart.
- **FR-010**: System MUST keep caller-facing addressing contracts location-transparent across failures and restarts.
- **FR-011**: System MUST expose deterministic behavior when restart limits are exceeded.
- **FR-012**: System MUST expose observable telemetry for crash, restart, and final supervision decisions.

### Key Entities *(include if feature involves data)*

- **SupervisorPolicy**: Defines failure decision rules (`restart`, `stop`, `escalate`) and restart limits.
- **FailureRecord**: Captures actor identifier, message identifier, failure class, timestamp, and decision.
- **RestartLifecycle**: Represents status transitions from running to failed to restarting to running/stopped.
- **MailboxSnapshot**: Logical view of queued unprocessed messages preserved across actor restart.

### Assumptions & Dependencies

- Mailbox preservation applies to queued messages that have not started processing at crash time.
- The message causing the crash is not retried automatically in this feature unless future policy adds retry.
- Supervisor decisions are local to single-node runtime scope for this feature.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In panic-injection tests, 100% of actor panics are isolated without terminating runtime process.
- **SC-002**: In restart-policy tests, 100% of restarted actors return to initial state values after crash.
- **SC-003**: In mailbox-preservation tests, 100% of queued unprocessed messages are retained across restart.
- **SC-004**: In mailbox-order tests, 100% of retained messages preserve FIFO delivery order after restart.
- **SC-005**: In supervision-limit tests, configured post-limit outcomes are applied deterministically in 100% of runs.
- **SC-006**: In failure telemetry tests, crash and recovery lifecycle events are emitted for 100% of crash episodes.
