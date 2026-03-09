# Feature Specification: Native Pub/Sub Event Bus

**Feature Branch**: `011-native-pubsub-broker`  
**Created**: 2026-03-09  
**Status**: Draft  
**Input**: User description: "Native Pub/Sub (Event Bus) Description: A localized broadcast mechanism allowing multiple actors to collaboratively react to system events without tight coupling. Requirements: Zero-Dependency Broker: A built-in Broker actor that manages subscriptions internally using a Trie (prefix tree) data structure, avoiding the need for heavy external message queues (like NATS or Redis) for intra-node communication. Wildcard & Segment Routing: The Trie-based subscription model must support hierarchical, segment-based routing with wildcards (e.g., using + for single-segment matches like user.+.updated, or # for multi-segment tail matches like audit.#). Dynamic Subscriptions: Actors must be able to send an Ask to the Broker to seamlessly subscribe or unsubscribe from specific exact topics or wildcard patterns at runtime. Fan-out: Publishing a message to a topic via the Broker must asynchronously duplicate and route the message to the mailboxes of all PIDs with matching subscriptions in the Trie. This is feature 11"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Publish and Fan-out Events (Priority: P1)

As a runtime user, I can publish an event to a topic through a built-in broker and have all matching subscribers receive it asynchronously.

**Why this priority**: This is the core event-bus value and enables decoupled collaboration between actors.

**Independent Test**: Register multiple subscribers for an exact topic, publish one event, and verify all matching subscriber mailboxes receive duplicated deliveries without blocking the publisher.

**Acceptance Scenarios**:

1. **Given** multiple actors subscribed to the same topic, **When** one event is published to that topic, **Then** the broker fans out asynchronous copies to all matching subscribers.
2. **Given** no matching subscribers for a topic, **When** an event is published, **Then** publish completes deterministically without downstream deliveries.

---

### User Story 2 - Route by Wildcard Topic Patterns (Priority: P2)

As a runtime user, I can subscribe using hierarchical wildcard patterns so one subscription can react to families of related topics.

**Why this priority**: Wildcard routing is required for practical event categorization and reduces subscription duplication.

**Independent Test**: Create subscriptions for exact, single-segment wildcard, and multi-segment tail wildcard patterns; publish events across multiple topic shapes; verify only matching subscribers receive each event.

**Acceptance Scenarios**:

1. **Given** a subscription pattern `user.+.updated`, **When** events are published to `user.profile.updated` and `user.billing.updated`, **Then** both events are delivered to that subscriber.
2. **Given** a subscription pattern `audit.#`, **When** events are published to `audit.security.login` and `audit.config`, **Then** both events are delivered to that subscriber.
3. **Given** a non-matching topic, **When** an event is published, **Then** wildcard subscribers that do not match receive no delivery.

---

### User Story 3 - Manage Subscriptions Dynamically at Runtime (Priority: P3)

As a runtime user, I can subscribe and unsubscribe via Ask-based broker commands so routing behavior can change safely without restarting actors.

**Why this priority**: Dynamic subscription management is necessary for operational control and evolving workflows.

**Independent Test**: Issue Ask subscribe command, publish and verify receipt, issue Ask unsubscribe command, publish again and verify no further deliveries.

**Acceptance Scenarios**:

1. **Given** an actor not subscribed to a topic, **When** it sends an Ask subscribe request to the broker, **Then** the broker confirms the subscription and future matching publishes are delivered.
2. **Given** a subscribed actor, **When** it sends an Ask unsubscribe request, **Then** the broker confirms removal and future matching publishes are not delivered.

### Failure & Recovery Scenarios *(mandatory)*

- If subscribe/unsubscribe Ask payload is invalid, the broker MUST return a deterministic error response and leave existing subscriptions unchanged.
- If publish fan-out includes an unreachable or stopped subscriber PID, the broker MUST report deterministic delivery outcome for that target and continue fan-out for other matching subscribers.
- If broker actor fails during processing, existing supervision policy MUST determine restart/stop/escalate behavior with explicit outcomes.
- If duplicate subscribe requests are received for the same PID and pattern, behavior MUST be deterministic and idempotent.

### Location Transparency Impact *(mandatory)*

- Callers interact with a logical broker actor identity and PID abstractions, not process-local references.
- Subscriber targeting remains PID-based and transport-agnostic, preserving compatibility with future multi-node routing.
- Topic contracts are string-pattern based and independent of node topology.

### Edge Cases

- Exact topic and wildcard pattern overlap for one subscriber must not cause duplicate deliveries for one publish event.
- Single-segment wildcard `+` must match exactly one segment and reject empty/multi-segment substitutions.
- Multi-segment tail wildcard `#` must only apply at the tail position and match zero or more remaining segments.
- Unsubscribe of a non-existent subscription must return deterministic no-op acknowledgement.
- High fan-out publishes must remain non-blocking for publisher-facing broker API behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a built-in broker actor for local Pub/Sub without requiring external message queue dependencies.
- **FR-002**: Broker MUST store and evaluate subscriptions using a Trie (prefix-tree) model over hierarchical topic segments.
- **FR-003**: Broker MUST support exact-topic subscriptions.
- **FR-004**: Broker MUST support single-segment wildcard subscriptions using `+`.
- **FR-005**: Broker MUST support multi-segment tail wildcard subscriptions using `#`.
- **FR-006**: Broker MUST allow actors to subscribe and unsubscribe at runtime via Ask requests.
- **FR-007**: Broker MUST fan out each publish asynchronously to all matching subscriber PIDs.
- **FR-008**: Broker MUST ensure one publish event results in at most one delivery per matching subscriber PID per matching subscription intent.
- **FR-009**: Broker MUST emit deterministic publish and subscription-management outcomes for success and failure cases.
- **FR-010**: Broker MUST preserve location-transparent caller and subscriber contracts.
- **FR-011**: Broker MUST define supervision-compatible failure behavior for broker and delivery-target failures.

### Key Entities *(include if feature involves data)*

- **BrokerActor**: Logical actor responsible for managing subscription lifecycle and publish fan-out behavior.
- **TopicPattern**: Hierarchical segment-based exact or wildcard subscription expression.
- **SubscriptionRecord**: Mapping between subscriber PID and topic pattern, including active state and timestamps.
- **PublishEnvelope**: Event payload plus topic metadata supplied to broker for fan-out.
- **BrokerDeliveryOutcome**: Observable record of publish/subscribe/unsubscribe operation result and reason codes.

## Assumptions

- Feature scope is intra-node communication only; no cross-node transport is introduced in this phase.
- Topic separator follows existing project conventions for segment hierarchies.
- Ask-based broker management commands reuse current Ask semantics and timeout behavior.
- Broker keeps in-memory subscription state for this phase.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For exact-topic subscriptions, 100% of published events are delivered to all matching subscribers and 0% to non-matching subscribers in integration tests.
- **SC-002**: For wildcard routing scenarios (`+` and `#`), 100% of test cases route only to intended subscribers across mixed topic sets.
- **SC-003**: Subscribe/unsubscribe Ask operations return deterministic acknowledgements in under 1 second for 99% of test operations under normal load.
- **SC-004**: Publishing to a topic with 100 matching subscribers completes without blocking publisher-facing call flow and produces deterministic delivery outcomes for all targets.
