# Contract: Lifecycle Management Hooks

## Purpose
Define caller-visible and operator-visible guarantees for actor Start/Stop lifecycle hooks.

## Start Hook Contract
- Actors MAY provide a Start hook.
- Start hook MUST execute before processing the first user message.
- Start hook MUST execute at most once per actor runtime instance.
- If Start hook fails (error or panic), actor MUST not process user messages and MUST expose startup failure deterministically.

## Stop Hook Contract
- Actors MAY provide a Stop hook.
- Stop hook MUST execute during graceful shutdown before shutdown completion is reported.
- Stop hook failure MUST NOT prevent actor from reaching terminal stopped state.
- Shutdown requests for actors without hooks MUST remain valid and deterministic.

## Supervision Compatibility Contract
- Lifecycle hooks MUST NOT change supervision decision semantics for regular message-processing failures.
- When an actor is restarted under supervision, Start hook MUST execute again for that new runtime instance.

## Observability Contract
Required lifecycle outcomes:
- `start_success`
- `start_failed`
- `stop_success`
- `stop_failed`

Required outcome fields:
- `event_id`
- `actor_id`
- `phase`
- `result`
- `timestamp`
- `error_code` (for failures)

## Location Transparency Contract
- Lifecycle behavior MUST be expressed through logical actor identity, not pointer/reference identity.
- Public hook outcomes MUST remain compatible with future multi-node addressing and routing.

## Invariants
- No user messages processed before successful startup completion when Start hook is configured.
- Graceful shutdown always reaches terminal stopped state.
- Every hook execution produces exactly one observable outcome.
