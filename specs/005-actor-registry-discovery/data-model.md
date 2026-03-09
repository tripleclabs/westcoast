# Data Model: Local Actor Registry & Discovery

## Entity: RegistryEntry
- Fields:
  - `name` (string, unique among active entries)
  - `pid` (PID)
  - `actor_id` (string)
  - `state` (enum: `active`, `removed`)
  - `updated_at` (timestamp)
- Relationships:
  - Maps one name to one active actor/PID.
- Validation Rules:
  - `name` MUST be non-empty and valid.
  - Active names MUST be unique.

## Entity: RegistryOperation
- Fields:
  - `operation` (enum: `register`, `lookup`, `unregister`)
  - `name` (string)
  - `actor_id` (string, optional for lookup miss)
  - `result` (enum: `success`, `duplicate`, `not_found`, `removed_by_lifecycle`)
  - `at` (timestamp)
- Relationships:
  - Produced by each registry interaction.
- Validation Rules:
  - Every operation MUST produce exactly one deterministic result.

## Entity: LifecycleBinding
- Fields:
  - `actor_id` (string)
  - `name` (string)
  - `terminal_reason` (enum: `graceful_stop`, `supervision_stop`, `supervision_escalate`)
  - `removed_at` (timestamp)
- Relationships:
  - Connects terminal actor lifecycle events to registry cleanup.
- Validation Rules:
  - Terminal lifecycle transitions MUST remove associated active registry entries.

## Entity: NameReservation
- Fields:
  - `name` (string)
  - `owner_actor_id` (string)
  - `owner_pid_generation` (uint64)
- Relationships:
  - Ownership guard for active name mapping.
- Validation Rules:
  - One active owner per name.
  - Ownership changes MUST be explicit via unregister + register sequence.

## State Transitions

### Name Registration Lifecycle
1. `unregistered -> active` on successful register.
2. `active -> active` on lookup.
3. `active -> removed` on explicit unregister or terminal lifecycle cleanup.
4. `removed -> active` on a new successful registration for the same name.

### Lifecycle-Synced Cleanup
1. `actor_running -> actor_stopped_terminal` on graceful/permanent termination.
2. `registry_entry_active -> registry_entry_removed` when terminal transition is observed.
3. `lookup_after_remove -> not_found` for removed names.
