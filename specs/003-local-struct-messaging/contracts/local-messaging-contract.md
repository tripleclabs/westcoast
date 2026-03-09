# Contract: Local Type-Agnostic Messaging

## Purpose
Define caller-visible behavior and runtime guarantees for local native-struct message delivery
without local serialization overhead.

## Local Message Contract
- Payload is passed as native structured value in local actor-to-actor delivery path.
- Runtime records and validates `type_name` and `schema_version` for routing decisions.
- Local path does not require encode/decode transformation.

## Routing Contract

### Rule Precedence
1. Exact `type_name + schema_version` match.
2. Optional explicit fallback handler.
3. Otherwise reject.

### Terminal Outcomes
- `delivered`
- `rejected_unsupported_type`
- `rejected_nil_payload`
- `rejected_version_mismatch`

Guarantees:
- Exactly one terminal outcome per message.
- Rejection outcomes emit explicit observable event.

## Non-Functional Contract
- Local send path p95 latency <= 25 µs on baseline profile.
- Local throughput >= 1,000,000 msgs/sec on baseline profile.
- Bench harness can enforce thresholds via `WC_ENFORCE_LOCAL_PERF_GATE=1`.

## Event Contract
- `message_processed`
- `message_rejected`
- `message_routed_exact`
- `message_routed_fallback`

Event fields:
- `event_id`
- `message_id`
- `actor_id`
- `type_name`
- `schema_version`
- `outcome`
- `timestamp`

## Invariants
- No local serialization stage is introduced in intra-node path.
- Caller contracts remain location-transparent and transport-agnostic.
- Nil payloads and version mismatches are rejected deterministically.
