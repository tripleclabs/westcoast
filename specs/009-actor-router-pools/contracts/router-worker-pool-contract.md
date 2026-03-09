# Contract: Router and Worker Pool Routing

## Purpose
Define caller-visible and operator-visible guarantees for stateless pool routing and consistent-hash sharding behavior.

## Router Dispatch Contract
- Runtime MUST allow callers to send work to a logical router identity.
- Router MUST select exactly one worker target per dispatch attempt.
- Router dispatch MUST preserve non-blocking sender behavior under normal operation.

## Stateless Strategy Contract
- `round_robin` MUST distribute messages deterministically across configured workers.
- `random` MUST select workers from the configured pool without requiring sender worker selection.
- Stateless routing MUST fail deterministically when no workers are configured.

## Consistent Hash Strategy Contract
- Consistent-hash routing requires a valid message hash key contract.
- Messages with the same hash key MUST route to the same worker while pool membership remains unchanged.
- Missing or invalid hash keys MUST produce deterministic failure outcome.

## Failure and Supervision Contract
- Worker unavailability during routing MUST return explicit routing failure outcome.
- Router actor failure MUST follow existing supervision policy semantics (restart/stop/escalate).
- Routing strategy logic MUST not suppress worker failure visibility.

## Observability Contract
Required outcomes:
- `route_success`
- `route_failed_no_workers`
- `route_failed_invalid_key`
- `route_failed_worker_unavailable`

Required fields:
- `event_id`
- `router_id`
- `message_id`
- `strategy`
- `selected_worker` (if available)
- `outcome`
- `timestamp`
- `reason_code` (for failures)

## Location Transparency Contract
- Callers MUST not require direct worker references to use routing.
- Public routing contracts MUST remain based on logical actor identity/addressing abstractions.
- Routing semantics MUST remain transport-agnostic for future multi-node evolution.

## Invariants
- Every routed message attempt yields one deterministic routing outcome.
- Stable pool membership + same hash key always yields same worker mapping.
- Router strategy changes are explicit configuration choices, not implicit behavior shifts.
