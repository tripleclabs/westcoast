# Feature Specification: Request-Response Ask Semantics

**Feature Branch**: `008-ask-reply-semantics`  
**Created**: 2026-03-09  
**Status**: Draft  
**Input**: User description: "Request-Response (Ask) Semantics & Asynchronous Replies Description: A core messaging pattern that complements fire-and-forget by enabling two-way request-response interactions, functionally equivalent to Erlangs gen_server:call. Ask Interface with timeout, implicit ReplyTo PID, and asynchronous delegation support."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Ask With Timeout (Priority: P1)

As an actor caller, I can send a request using `Ask` and wait for a single response up to a declared timeout so that synchronous request-response workflows are possible without manual polling.

**Why this priority**: This is the core user-facing capability. Without `Ask`, the feature does not exist.

**Independent Test**: Can be fully tested by issuing an `Ask` to a responder actor and verifying success on timely reply and failure on timeout.

**Acceptance Scenarios**:

1. **Given** a running caller and responder actor, **When** the caller sends `Ask` and the responder replies before timeout, **Then** the caller receives exactly one response and no timeout error.
2. **Given** a running caller and responder actor, **When** the caller sends `Ask` and no reply arrives before timeout, **Then** the call returns a timeout error and does not block indefinitely.

---

### User Story 2 - Implicit Reply Routing Context (Priority: P2)

As a receiving actor, I can discover a `ReplyTo` PID in Ask-originated messages so I know exactly where to route my response without requiring out-of-band routing data.

**Why this priority**: Reliable response routing is required for `Ask` correctness and future location transparency.

**Independent Test**: Can be fully tested by handling an Ask-originated message, extracting `ReplyTo`, sending a response through it, and verifying the waiting caller receives the response.

**Acceptance Scenarios**:

1. **Given** a caller uses `Ask`, **When** the target actor reads the incoming message context, **Then** a valid `ReplyTo` PID is present and unique to that request.
2. **Given** a message sent with fire-and-forget semantics, **When** the receiver inspects message context, **Then** no Ask-specific `ReplyTo` requirement is imposed.

---

### User Story 3 - Asynchronous Delegation via ReplyTo (Priority: P3)

As a receiving actor, I can delegate long-running work outside the main actor loop and send the response later via the provided `ReplyTo` PID so that throughput is preserved and head-of-line blocking is avoided.

**Why this priority**: This protects actor responsiveness and enables realistic long-running workflows.

**Independent Test**: Can be tested by sending multiple requests where the responder delegates one long-running task; the actor must continue processing additional messages while eventually replying to the delegated request.

**Acceptance Scenarios**:

1. **Given** an Ask message requiring long work, **When** the receiver delegates work asynchronously, **Then** the actor loop continues processing other messages while the final response is sent later through `ReplyTo`.
2. **Given** delegated work completes after the caller timeout, **When** the late reply is sent, **Then** the original Ask remains timed out and the late reply is safely discarded or ignored by the waiting side.

### Failure & Recovery Scenarios *(mandatory)*

- If a responder actor fails before sending a reply, supervisor policy determines restart/stop/escalate behavior, and the waiting Ask caller receives a failure outcome (timeout or explicit failure) within bounded time.
- If reply delivery fails because `ReplyTo` is invalid, stale, or unreachable, the failure is observable through runtime outcomes/events and does not crash unrelated actors.
- If delegated background work panics or errors, the failure is isolated to that request path; the responder actor main loop remains available for subsequent messages.
- If timeout occurs, the caller-side wait is released deterministically, and any later reply is treated as non-correlating to an active wait.

### Location Transparency Impact *(mandatory)*

- Callers and responders interact through actor identity and PID contracts only; no direct in-process pointers are part of the public contract.
- `ReplyTo` is defined as PID-based addressing so the same request-response contract can be preserved when routing moves from single-node to multi-node transport.
- Ask semantics must remain agnostic to transport topology, with routing seams that allow a future gateway actor to mediate remote replies without changing business logic.

### Edge Cases

- Multiple concurrent Ask calls to the same target actor must not cross-wire replies between callers.
- Duplicate replies for the same Ask request must result in at most one delivered response to the waiter.
- Timeout value of zero or negative must fail fast with a clear error.
- Ask to non-existent or stopped target must return a deterministic failure outcome.
- Caller cancellation before reply arrival must release the waiter and prevent indefinite resource retention.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide an Ask interface that sends a request and waits for one response up to a caller-defined timeout.
- **FR-002**: System MUST return a timeout error when no response is received before the Ask timeout expires.
- **FR-003**: System MUST automatically attach Ask reply-routing context to each Ask-originated message.
- **FR-004**: System MUST expose a `ReplyTo` PID in Ask-originated message context for responders.
- **FR-005**: System MUST allow responders to send replies using only the provided `ReplyTo` PID, without requiring direct access to caller internals.
- **FR-006**: System MUST support deferred/asynchronous replies so responders can complete long-running work after the original message handler returns.
- **FR-007**: System MUST ensure each Ask request is correlated to at most one delivered response.
- **FR-008**: System MUST isolate timed-out or canceled Ask requests so late replies do not re-open or corrupt completed waits.
- **FR-009**: System MUST emit observable outcomes for Ask success, timeout, and reply-routing failure cases.
- **FR-010**: System MUST preserve location-transparent contracts by defining Ask and reply routing exclusively through PID-compatible identifiers.
- **FR-011**: System MUST define failure handling expectations when responder failures occur before reply completion.

### Key Entities *(include if feature involves data)*

- **Ask Request Context**: Request-scoped metadata attached to Ask-originated messages, including a correlation identifier and `ReplyTo` PID.
- **ReplyTo PID**: One-time-use or request-scoped PID that identifies where the response for a specific Ask must be routed.
- **Ask Wait Slot**: Runtime-tracked pending wait state for an Ask call, including timeout deadline and completion status.
- **Ask Response Outcome**: Caller-visible completion result for an Ask request (success, timeout, canceled, or failed delivery).

## Assumptions

- Ask is a caller-facing capability usable by actors and runtime clients in the same process for this phase.
- Default behavior for late replies is non-delivery to completed waits, while still allowing observability of dropped-late-reply outcomes.
- Timeout handling uses existing runtime time semantics and does not require persistent storage.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of Ask calls complete with either a response or timeout outcome within their configured timeout window.
- **SC-002**: In a test with at least 1,000 concurrent Ask calls, 0 reply misroutes occur (no caller receives another caller's response).
- **SC-003**: For delegated long-running requests, responder actors continue processing unrelated messages while the delegated reply is pending in at least 95% of observed runs.
- **SC-004**: 100% of late replies arriving after Ask timeout are rejected from active delivery and recorded as non-success outcomes.
