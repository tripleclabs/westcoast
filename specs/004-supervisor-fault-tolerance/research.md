# Phase 0 Research: Supervisor Trees & Fault Tolerance

## Decision 1
- Decision: Intercept panics at actor message-processing boundary and convert panic to supervised actor-local failure.
- Rationale: Preserves let-it-crash semantics while preventing process-wide runtime termination.
- Alternatives considered:
  - Global panic recovery wrapper: rejected due to weak actor-level failure attribution.
  - Catch-and-continue in handler: rejected due to hidden corruption risk.

## Decision 2
- Decision: Preserve queued-but-unprocessed mailbox messages across actor restart.
- Rationale: Prevents silent loss of in-flight work and satisfies mailbox-preservation requirement.
- Alternatives considered:
  - Drop mailbox on restart: rejected due to data loss and nondeterministic caller outcomes.
  - Replay entire crash-triggering message automatically: rejected for this scope due to repeat-crash risk.

## Decision 3
- Decision: Reset actor state to initial snapshot on restart, without carrying mutated in-memory state.
- Rationale: Ensures clean state restarts and avoids corrupted state propagation.
- Alternatives considered:
  - Best-effort partial state rollback: rejected due to complex and fragile semantics.
  - Keep existing state through restart: rejected as violation of clean restart requirement.

## Decision 4
- Decision: Maintain deterministic supervision outcomes with explicit limit behavior (`restart` until limit, then configured terminal decision).
- Rationale: Enables predictable operational behavior and testability under repeated failures.
- Alternatives considered:
  - Unlimited restarts: rejected due to restart-storm risk.
  - Implicit fallback behavior after limit: rejected due to ambiguity.

## Decision 5
- Decision: Emit structured failure/recovery telemetry for crash, decision, restart, stop/escalate outcomes.
- Rationale: Required for diagnosis, conformance testing, and runtime health visibility.
- Alternatives considered:
  - Log-only diagnostics: rejected due to weak contract testability.
  - Metrics-only diagnostics: rejected due to missing per-failure causality context.

## Decision 6
- Decision: Keep caller addressing contracts stable across restart by preserving logical actor identity and PID generation semantics.
- Rationale: Supports location transparency and future distributed transport evolution.
- Alternatives considered:
  - Expose restart-instance references to callers: rejected due to coupling to process-local runtime details.
