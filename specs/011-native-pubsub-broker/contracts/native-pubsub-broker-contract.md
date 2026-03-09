# Contract: Native Pub/Sub Event Bus

## Purpose
Define caller-visible and operator-visible guarantees for built-in Trie-backed Pub/Sub, wildcard routing, dynamic subscription management, and publish fan-out.

## Broker Availability Contract
- Runtime MUST provide a built-in broker actor for intra-node Pub/Sub.
- Broker operations MUST not require external message queue infrastructure.

## Topic and Pattern Contract
- Topics are hierarchical segment paths.
- Exact-topic subscriptions MUST match only identical topic paths.
- `+` wildcard MUST match exactly one segment.
- `#` wildcard MUST match zero or more trailing segments and only when placed as the last pattern segment.

## Dynamic Subscription Contract
- Actors MUST be able to Ask broker to subscribe and unsubscribe at runtime.
- Subscribe/unsubscribe commands MUST return deterministic acknowledgements.
- Duplicate subscribe for same PID+pattern MUST be idempotent.
- Unsubscribe of missing subscription MUST be deterministic no-op acknowledgement.

## Publish Fan-out Contract
- Publish MUST evaluate matching subscriptions through Trie lookup.
- Publish MUST asynchronously fan out message copies to all matching subscriber PIDs.
- Publish MUST continue fan-out even when one target delivery fails.
- One publish MUST produce at most one delivery attempt per unique subscriber intent.

## Failure and Supervision Contract
- Invalid subscribe/unsubscribe command payloads MUST return deterministic validation errors.
- Target PID delivery failures MUST emit deterministic target-level outcome classification.
- Broker actor failures MUST follow existing supervisor policy semantics.

## Observability Contract
Required outcome categories:
- `subscribe_success`
- `unsubscribe_success`
- `publish_success`
- `publish_partial_delivery`
- `invalid_command`
- `invalid_pattern`
- `target_unreachable`
- `not_found_noop`

Required fields:
- `event_id`
- `operation`
- `topic_or_pattern`
- `subscriber_pid` (where applicable)
- `matched_count` (for publish)
- `result`
- `timestamp`
- `reason_code` (for non-success paths)

## Location Transparency Contract
- Callers and subscribers use logical actor identity/PID abstractions only.
- Contracts MUST not depend on process-local object references.
- Semantics MUST remain transport-agnostic for future distributed evolution.

## Invariants
- Each broker command yields one deterministic acknowledgement outcome.
- Each publish yields one deterministic publish outcome plus target delivery evidence.
- Wildcard matching semantics are stable and deterministic.
