# Feature Specification: Local Actor Registry & Discovery

**Feature Branch**: `005-actor-registry-discovery`  
**Created**: 2026-03-09  
**Status**: Draft  
**Input**: User description: "Local Actor Registry & Discovery Description: A localized directory service that allows actors to find each other dynamically without hardcoded references. Requirements: Named Registration: Actors can optionally register themselves with a human-readable string ID (e.g., user-service-123). Dynamic Lookup: Any part of the system can query the registry with a string ID to retrieve the corresponding PID. Lifecycle Syncing: The registry must automatically remove entries when an actor is gracefully shut down or permanently terminated."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Register Actors by Name (Priority: P1)

As a runtime consumer, I need actors to register with human-readable names so services can be addressed
without hardcoded actor references.

**Why this priority**: Named registration is the foundational capability for discovery and all dependent
lookups.

**Independent Test**: Register an actor under a unique string name and confirm the registration is persisted
and queryable by that name.

**Acceptance Scenarios**:

1. **Given** a running actor and an unused name, **When** the actor is registered with that name,
   **Then** the registry stores the mapping and reports success.
2. **Given** a name already assigned to an active actor, **When** a second registration attempts to reuse
   the same name, **Then** the request is rejected deterministically.

---

### User Story 2 - Resolve Name to PID Dynamically (Priority: P2)

As any component in the runtime, I need to look up an actor by name and retrieve its PID so messaging can
be done through discovery instead of static wiring.

**Why this priority**: Dynamic lookup is the core consumer-facing behavior once names are registered.

**Independent Test**: Register multiple actors with distinct names and verify lookups return the correct PID
for each name.

**Acceptance Scenarios**:

1. **Given** multiple named actor registrations, **When** a lookup is performed with a valid name,
   **Then** the correct PID is returned.
2. **Given** a name that is not registered, **When** lookup is requested, **Then** a not-found result is
   returned without side effects.

---

### User Story 3 - Keep Registry in Sync with Actor Lifecycle (Priority: P3)

As a runtime consumer, I need registry entries to be automatically removed when actors stop permanently so
lookups never return stale destinations.

**Why this priority**: Lifecycle synchronization protects correctness and prevents routing to terminated actors.

**Independent Test**: Register an actor, stop it permanently, and verify the name no longer resolves to a PID.

**Acceptance Scenarios**:

1. **Given** a registered actor, **When** the actor shuts down gracefully, **Then** its registry entry is
   removed automatically.
2. **Given** a registered actor that is permanently terminated by supervision policy, **When** termination
   completes, **Then** the registry entry is removed and future lookups return not-found.

### Failure & Recovery Scenarios *(mandatory)*

- Duplicate name registration attempts MUST be rejected with deterministic outcome.
- Lookup requests for missing names MUST return an explicit not-found result.
- Registry cleanup MUST occur for actors that are permanently stopped or terminated.
- Temporary restarts MUST not lose registration unless the actor becomes permanently unavailable.
- Registry operations MUST remain isolated from actor handler panics in unrelated actors.

### Location Transparency Impact *(mandatory)*

- Discovery clients use logical names and PIDs, not process-local references.
- Caller-facing contracts MUST remain transport-agnostic and compatible with future multi-node routing.
- Registry semantics MUST preserve actor identity abstraction regardless of local process placement.

### Edge Cases

- Name registration contains invalid or empty identifier strings.
- Concurrent registration requests race for the same name.
- Actor is stopped between successful lookup and message send.
- Actor restarts under supervision while name is still registered.
- High-volume lookups occur while lifecycle cleanup is running.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST allow a running actor to register under a human-readable string name.
- **FR-002**: System MUST reject registration for empty or invalid names.
- **FR-003**: System MUST enforce one active actor mapping per registered name at any point in time.
- **FR-004**: System MUST provide dynamic lookup by name that returns the corresponding PID when present.
- **FR-005**: System MUST return deterministic not-found results for unknown names.
- **FR-006**: System MUST remove registry entries when actors are gracefully stopped.
- **FR-007**: System MUST remove registry entries when actors are permanently terminated by supervision policy.
- **FR-008**: System MUST keep registry behavior deterministic under concurrent register/lookup/remove operations.
- **FR-009**: System MUST preserve location-transparent caller contracts for named discovery and PID usage.
- **FR-010**: System MUST expose observable events or outcomes for register success, register reject,
  lookup result, and lifecycle-triggered unregister.
- **FR-011**: System MUST prevent stale lookup results after permanent actor termination.
- **FR-012**: System MUST keep named discovery available while unrelated actor failures are handled.

### Key Entities *(include if feature involves data)*

- **RegistryEntry**: Mapping of `name` to `pid` plus lifecycle state and update timestamp.
- **RegistryOperation**: Register, lookup, and unregister actions with actor identity and operation outcome.
- **LifecycleBinding**: Association between actor lifecycle transitions and automatic registry cleanup behavior.
- **NameReservation**: Uniqueness guard ensuring a name maps to at most one active actor.

### Assumptions & Dependencies

- Discovery scope for this feature is local single-node runtime only.
- Name uniqueness is case-sensitive and exact-match by default.
- Permanent termination includes graceful stop and supervision-driven stop/escalate terminal outcomes.
- Temporary restarts do not remove name mapping unless actor becomes permanently unavailable.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In registration tests, 100% of valid unique names register successfully.
- **SC-002**: In duplicate registration tests, 100% of collisions are rejected deterministically.
- **SC-003**: In lookup tests, 100% of registered names resolve to the correct PID.
- **SC-004**: In not-found tests, 100% of unknown names return explicit not-found outcomes.
- **SC-005**: In lifecycle-sync tests, 100% of permanently terminated actors are removed from registry.
- **SC-006**: In concurrency tests, registry operations maintain deterministic outcomes with no stale active mappings.
