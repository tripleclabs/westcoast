# westcoast Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-08

## Active Technologies
- Go 1.24+ + Go standard library (`context`, `sync`, `sync/atomic`, `time`) (002-location-transparent-pids)
- In-memory resolver index and PID state only (no persistence in this feature) (002-location-transparent-pids)
- Go 1.24+ + Go standard library (`context`, `sync`, `sync/atomic`, `reflect`, `time`) (003-local-struct-messaging)
- In-memory mailbox/message structures only (no persistence in this feature) (003-local-struct-messaging)
- In-memory runtime state and mailbox queues only (no persistence in this feature) (004-supervisor-fault-tolerance)

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
- 004-supervisor-fault-tolerance: Added Go 1.24+ + Go standard library (`context`, `sync`, `sync/atomic`, `time`)
- 003-local-struct-messaging: Added Go 1.24+ + Go standard library (`context`, `sync`, `sync/atomic`, `reflect`, `time`)
- 002-location-transparent-pids: Added Go 1.24+ + Go standard library (`context`, `sync`, `sync/atomic`, `time`)


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
