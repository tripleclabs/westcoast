# Contract: Smart Mailboxes and Message Batching

## Purpose
Define caller-visible and operator-visible guarantees for opt-in batch mailbox retrieval and downstream bulk-processing optimization.

## Batch Retrieval Contract
- Runtime MUST allow eligible actors to opt into batch retrieval mode.
- Batch retrieval MUST return up to configured maximum messages per execution cycle.
- Batch retrieval MUST preserve mailbox ordering semantics.
- Batching MUST NOT be required for actors that do not opt in.

## Batch Handler Contract
- Actor logic MUST be able to consume a batch payload in one cycle.
- Batch handling failure MUST produce explicit failure outcome and supervision decision visibility.
- Runtime MUST keep existing lifecycle and supervision contracts intact when batching is enabled.

## I/O Optimization Contract
- Batching MUST make it possible for actor logic to aggregate multiple messages into one downstream operation.
- Runtime MUST not force one downstream call per input message when batching is enabled.
- Bulk operation behavior remains actor-defined while runtime provides retrieval semantics.

## Failure and Recovery Contract
- Empty mailbox in batching mode MUST not emit false success outcomes.
- Invalid batch configuration (e.g., non-positive batch size) MUST be handled deterministically.
- Batch processing failures MUST follow configured supervisor policy decisions.
- Post-restart behavior MUST preserve configured batching mode unless explicitly reconfigured.

## Observability Contract
Required batch outcomes:
- `batch_success`
- `batch_failed_handler`
- `batch_failed_supervision`

Required fields:
- `event_id`
- `actor_id`
- `batch_size`
- `result`
- `timestamp`
- `reason_code` (for failures)

## Location Transparency Contract
- Callers continue to target logical actor identity/PID rather than mailbox internals.
- Batch semantics MUST not require caller knowledge of in-process references.
- Contracts MUST remain transport-agnostic for future distributed routing.

## Invariants
- Batch mode is explicit and actor-scoped.
- Each batch execution emits one deterministic batch outcome.
- Non-batching actors keep existing behavior unchanged.
