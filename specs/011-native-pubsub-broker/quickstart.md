# Quickstart: Validate Native Pub/Sub Event Bus

## Prerequisites
- Go 1.24+
- Repository root: `/Volumes/Store1/src/3clabs/westcoast`

## 1. Write Failing Tests First (Red)

```bash
go test ./tests/integration/... -run TestBrokerPublishFanOutToExactSubscribers
go test ./tests/integration/... -run TestBrokerWildcardSingleSegmentRouting
go test ./tests/integration/... -run TestBrokerWildcardTailRouting
go test ./tests/integration/... -run TestBrokerDynamicSubscribeViaAsk
go test ./tests/integration/... -run TestBrokerDynamicUnsubscribeViaAsk
go test ./tests/contract/... -run TestNativePubSubBrokerContract
```

Expected before implementation: tests fail, proving missing broker and routing behavior.

## 2. Implement Minimum Feature Slice (Green)

Implement:
- Built-in broker actor with in-memory Trie subscription index
- Ask commands for subscribe/unsubscribe
- Topic publish with asynchronous fan-out to matching PIDs
- Deterministic wildcard routing for `+` and `#` patterns
- Deterministic outcome emission for success/failure/partial delivery paths

## 3. Re-run Verification

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/contract/...
go test ./tests/benchmark/... -run '^$' -bench BenchmarkPubSubBrokerFanout -benchmem
```

## 4. Validate Critical Behavior

```bash
go test ./tests/integration/... -run 'TestBrokerPublishFanOutToExactSubscribers|TestBrokerWildcardSingleSegmentRouting|TestBrokerWildcardTailRouting|TestBrokerDynamicSubscribeViaAsk|TestBrokerDynamicUnsubscribeViaAsk'
```

Pass criteria:
- Exact topic fan-out reaches all intended subscribers.
- Wildcard routing matches only intended topic families.
- Subscribe/unsubscribe Ask commands are acknowledged deterministically.
- Target delivery failures produce explicit outcomes without halting overall fan-out.

## 7. Implemented Test Files

- `tests/unit/pubsub_topic_pattern_validation_test.go`
- `tests/unit/pubsub_trie_match_test.go`
- `tests/unit/pubsub_outcomes_unit_test.go`
- `tests/contract/native_pubsub_broker_contract_test.go`
- `tests/integration/broker_publish_fanout_exact_topic_test.go`
- `tests/integration/broker_publish_no_matching_subscribers_test.go`
- `tests/integration/broker_wildcard_single_segment_routing_test.go`
- `tests/integration/broker_wildcard_tail_routing_test.go`
- `tests/integration/broker_wildcard_nonmatch_test.go`
- `tests/integration/broker_dynamic_subscribe_via_ask_test.go`
- `tests/integration/broker_dynamic_unsubscribe_via_ask_test.go`
- `tests/integration/broker_subscription_idempotency_test.go`
- `tests/integration/broker_invalid_command_outcome_test.go`
- `tests/integration/broker_partial_delivery_resilience_test.go`
- `tests/benchmark/pubsub_broker_fanout_benchmark_test.go`

## 8. Validation Results (2026-03-09)

Executed:

```bash
GOCACHE=$(pwd)/.tmp/gocache go test ./...
```

Observed:

- `westcoast/tests/unit`: pass
- `westcoast/tests/integration`: pass
- `westcoast/tests/contract`: pass
- `westcoast/tests/benchmark`: pass

## 5. Constitution Alignment Checks
- Runtime implementation remains Go-native and lightweight.
- Broker/delivery failures remain explicit and supervision-compatible.
- Failing-then-passing tests prove behavior changes.
- Broker/subscriber contracts remain location-transparent.
- Trie routing/fan-out path remains bounded and measurable.

## 6. Expected Outcomes
- Actors can collaborate through decoupled event publication/subscription.
- Wildcard patterns reduce subscription duplication for hierarchical event taxonomies.
- Built-in broker eliminates intra-node dependency on external MQ infrastructure.
