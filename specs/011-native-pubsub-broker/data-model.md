# Data Model: Native Pub/Sub Event Bus

## Entity: BrokerActor
- Fields:
  - `broker_id` (string)
  - `active` (boolean)
  - `updated_at` (timestamp)
- Relationships:
  - Owns one `SubscriptionTrie` index and one subscriber reverse index.
  - Produces `BrokerDeliveryOutcome` records for publish and subscription commands.
- Validation Rules:
  - `broker_id` MUST be unique in runtime scope.
  - Broker must remain addressable by logical actor identity/PID.

## Entity: TopicPattern
- Fields:
  - `raw` (string)
  - `segments` (ordered list of strings)
  - `kind` (enum: `exact`, `single_wildcard`, `tail_wildcard`)
- Relationships:
  - Indexed in `SubscriptionTrie` and referenced by `SubscriptionRecord`.
- Validation Rules:
  - `+` MUST represent exactly one segment.
  - `#` MUST only appear as final segment.
  - Empty topics/patterns are invalid.

## Entity: SubscriptionRecord
- Fields:
  - `subscription_id` (string)
  - `subscriber_pid` (PID)
  - `topic_pattern` (TopicPattern)
  - `active` (boolean)
  - `created_at` (timestamp)
  - `updated_at` (timestamp)
- Relationships:
  - Stored in `SubscriptionTrie` and subscriber reverse index.
- Validation Rules:
  - Duplicate active subscription for same PID + pattern is idempotent.
  - Unsubscribe for non-existent record returns deterministic no-op outcome.

## Entity: PublishEnvelope
- Fields:
  - `topic` (string)
  - `payload` (any)
  - `published_at` (timestamp)
  - `publisher_actor_id` (string)
- Relationships:
  - Evaluated against `SubscriptionTrie` to produce target PID set.
- Validation Rules:
  - Topic MUST be valid non-empty segment path.
  - One publish event fan-outs to zero or more matched targets.

## Entity: BrokerDeliveryOutcome
- Fields:
  - `event_id` (uint64)
  - `operation` (enum: `subscribe`, `unsubscribe`, `publish`)
  - `topic_or_pattern` (string)
  - `subscriber_pid` (PID, optional)
  - `matched_count` (integer, optional)
  - `result` (enum: `success`, `invalid_command`, `invalid_pattern`, `target_unreachable`, `not_found_noop`, `partial_delivery`)
  - `reason_code` (string, optional)
  - `timestamp` (timestamp)
- Relationships:
  - Emitted for each broker command and publish operation.
- Validation Rules:
  - Every command/publish emits one deterministic outcome.
  - Failure/partial outcomes include machine-readable reason code.

## State Transitions

### Subscription Lifecycle
1. `absent -> active` on successful subscribe command.
2. `active -> active` on duplicate subscribe command (idempotent no-op success).
3. `active -> absent` on successful unsubscribe command.
4. `absent -> absent` on unsubscribe for non-existent subscription (deterministic no-op).

### Publish Lifecycle
1. `published -> matched` when Trie finds one or more subscribers.
2. `published -> unmatched` when no subscriber patterns match topic.
3. `matched -> delivered` when all matched PID sends succeed.
4. `matched -> partial_delivery` when one or more target deliveries fail while others succeed.

### Broker Fault Lifecycle
1. `running -> failed` on broker processing failure.
2. `failed -> restarted|stopped|escalated` per supervisor decision.
