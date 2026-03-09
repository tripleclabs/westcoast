# Data Model: Actor Routers and Worker Pools

## Entity: RouterDefinition
- Fields:
  - `router_id` (string)
  - `strategy` (enum: `round_robin`, `random`, `consistent_hash`)
  - `worker_ids` (ordered list of actor IDs)
  - `active` (boolean)
  - `updated_at` (timestamp)
- Relationships:
  - Owns one `WorkerPool` configuration.
  - Produces `RoutingOutcomeRecord` entries on each dispatch.
- Validation Rules:
  - `worker_ids` MUST be non-empty for active routing.
  - `strategy` MUST be one of supported routing modes.

## Entity: WorkerPool
- Fields:
  - `pool_id` (string)
  - `router_id` (string)
  - `workers` (list of worker actor IDs)
  - `pool_size` (integer)
  - `version` (integer)
- Relationships:
  - Referenced by `RouterDefinition` for dispatch decisions.
- Validation Rules:
  - `pool_size` MUST equal number of configured workers.
  - Duplicate worker IDs are disallowed.

## Entity: ShardKeyMessageContract
- Fields:
  - `message_type` (string)
  - `has_hash_key` (boolean)
  - `hash_key_value` (string, optional)
- Relationships:
  - Evaluated only when `RouterDefinition.strategy = consistent_hash`.
- Validation Rules:
  - `has_hash_key` MUST be true for consistent-hash dispatch.
  - `hash_key_value` MUST be non-empty when hash routing is required.

## Entity: RoutingDispatchState
- Fields:
  - `router_id` (string)
  - `round_robin_index` (uint64)
  - `last_selected_worker` (string)
  - `last_key` (string, optional)
  - `dispatched_at` (timestamp)
- Relationships:
  - Mutates on each dispatch attempt.
- Validation Rules:
  - Round-robin index increments monotonically.
  - Selected worker must exist in current pool membership.

## Entity: RoutingOutcomeRecord
- Fields:
  - `event_id` (uint64)
  - `router_id` (string)
  - `message_id` (uint64)
  - `strategy` (string)
  - `selected_worker` (string, optional)
  - `outcome` (enum: `route_success`, `route_failed_no_workers`, `route_failed_invalid_key`, `route_failed_worker_unavailable`)
  - `reason_code` (string, optional)
  - `timestamp` (timestamp)
- Relationships:
  - Emitted per dispatch attempt.
- Validation Rules:
  - Every dispatch attempt emits exactly one outcome record.
  - Failure outcomes include machine-readable `reason_code`.

## State Transitions

### Router Activation
1. `inactive -> active` when router is configured with valid strategy and non-empty pool.
2. `active -> inactive` when router configuration is removed or invalidated.

### Stateless Dispatch
1. `ready -> route_success` when selected worker accepts message.
2. `ready -> route_failed_no_workers` when pool is empty.
3. `ready -> route_failed_worker_unavailable` when selected worker cannot accept dispatch.

### Consistent Hash Dispatch
1. `ready -> route_success` when hash key resolves to valid worker and dispatch is accepted.
2. `ready -> route_failed_invalid_key` when required hash key contract is missing or invalid.
3. `ready -> route_failed_worker_unavailable` when mapped worker is unavailable.
