# Data Model: Type-Agnostic Local Messaging

## Entity: LocalMessage
- Fields:
  - `message_id` (uint64, unique)
  - `sender_actor_id` (string)
  - `receiver_actor_id` (string)
  - `payload` (native structured value)
  - `type_name` (string)
  - `schema_version` (string)
  - `accepted_at` (timestamp)
- Relationships:
  - Belongs to one mailbox queue while pending.
  - Produces one MessageHandlingOutcome.
- Validation Rules:
  - Local path MUST not transform payload via encode/decode.
  - `type_name` and `schema_version` MUST be available for routing validation.

## Entity: TypeRoutingRule
- Fields:
  - `actor_id` (string)
  - `type_name` (string)
  - `schema_version` (string)
  - `handler_key` (string)
  - `is_fallback` (bool)
- Relationships:
  - One actor owns many routing rules.
- Validation Rules:
  - Exact type+version rules MUST be evaluated before fallback.
  - At most one fallback rule per actor.

## Entity: MessageHandlingOutcome
- Fields:
  - `message_id` (uint64)
  - `actor_id` (string)
  - `outcome` (enum: `delivered`, `rejected_unsupported_type`, `rejected_nil_payload`, `rejected_version_mismatch`)
  - `completed_at` (timestamp)
  - `error_code` (optional string)
- Relationships:
  - Produced by one LocalMessage handling attempt.
- Validation Rules:
  - Exactly one terminal outcome per message.
  - Rejected outcomes MUST emit observable events.

## State Transitions

### Local Message Lifecycle
1. `submitted -> accepted` when queued in receiver mailbox.
2. `accepted -> routed_exact` when exact type+version rule exists.
3. `accepted -> routed_fallback` when exact match missing and fallback rule exists.
4. `accepted -> rejected_unsupported_type` when no matching route exists.
5. `accepted -> rejected_nil_payload` when payload is nil.
6. `accepted -> rejected_version_mismatch` when type matches but version mismatches rule.
7. `routed_* -> delivered` after handler completion.
