# westcoast Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-08

## Active Technologies
- Go 1.24+ + Go standard library (`context`, `sync`, `sync/atomic`, `time`) (002-location-transparent-pids)
- In-memory resolver index and PID state only (no persistence in this feature) (002-location-transparent-pids)
- Go 1.24+ + Go standard library (`context`, `sync`, `sync/atomic`, `reflect`, `time`) (003-local-struct-messaging)
- In-memory mailbox/message structures only (no persistence in this feature) (003-local-struct-messaging)
- In-memory runtime state and mailbox queues only (no persistence in this feature) (004-supervisor-fault-tolerance)
- Go 1.24+ + Go standard library (`sync`, `sync/atomic`, `time`, `context`) (005-actor-registry-discovery)
- In-memory registry map structures in runtime scope only (005-actor-registry-discovery)
- In-memory runtime lifecycle state and hook execution outcomes only (no persistence) (006-lifecycle-hooks)
- In-memory policy validation state and observability outcomes only (no persistence) (007-pid-gateway-guardrails)
- In-memory Ask wait tracking and response-correlation state only (no persistence) (008-ask-reply-semantics)

- Go 1.24+ + Go standard library (`context`, `sync`, `sync/atomic`, `runtime`, `time`) (001-actor-execution-engine)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for Go 1.24+

## Code Style

Go 1.24+: Follow standard conventions

## Recent Changes
- 008-ask-reply-semantics: Added Go 1.24+ + Go standard library (`context`, `sync`, `sync/atomic`, `time`)
- 007-pid-gateway-guardrails: Added Go 1.24+ + Go standard library (`context`, `sync`, `sync/atomic`, `time`)
- 006-lifecycle-hooks: Added Go 1.24+ + Go standard library (`context`, `sync`, `sync/atomic`, `time`)


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
