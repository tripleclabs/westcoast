# Phase 0 Research: Lifecycle Management Hooks

## Decision 1
- Decision: Introduce two optional actor lifecycle hooks: `Start` (pre-first-message) and `Stop` (graceful shutdown).
- Rationale: Matches feature goals with minimal API surface and clear author intent for setup/teardown behavior.
- Alternatives considered:
  - Single generic lifecycle callback with phase parameter: rejected because it is less explicit and easier to misuse.
  - Multiple fine-grained hooks (pre-start/post-start/pre-stop/post-stop): rejected as unnecessary scope expansion for current requirements.

## Decision 2
- Decision: Block user message processing until Start hook completes successfully.
- Rationale: Prevents resource races where message handlers execute before required dependencies are initialized.
- Alternatives considered:
  - Allow concurrent message buffering and delayed processing: rejected due to additional queue semantics complexity and less predictable startup behavior.
  - Allow handlers during startup and rely on user checks: rejected due to inconsistent behavior and higher defect risk.

## Decision 3
- Decision: Treat Start hook failure (error or panic) as startup failure that prevents actor from entering running state.
- Rationale: Keeps failure domains explicit and aligned with let-it-crash semantics while avoiding partially initialized actors.
- Alternatives considered:
  - Ignore Start hook failure and continue running: rejected as unsafe and non-deterministic.
  - Automatic retry loop in startup path: rejected because retry policy belongs to supervision, not hook internals.

## Decision 4
- Decision: Execute Stop hook during graceful shutdown; even on failure, actor must still reach stopped terminal state.
- Rationale: Guarantees cleanup attempt while preserving deterministic shutdown completion and avoiding hung actors.
- Alternatives considered:
  - Abort shutdown if Stop hook fails: rejected because it risks leaked actor lifecycles and deadlocks in stop flows.
  - Skip Stop hook on failures elsewhere: rejected because cleanup must be attempted consistently.

## Decision 5
- Decision: Emit explicit lifecycle outcomes for Start/Stop success and failure for operational diagnostics.
- Rationale: Enables reliable incident triage and satisfies observability requirements without changing caller contracts.
- Alternatives considered:
  - Logs-only observability: rejected due to weak deterministic testability.
  - Metrics-only observability: rejected because per-actor causality and failure reason visibility are limited.

## Decision 6
- Decision: Preserve existing supervision decision policy for message-processing failures; hooks integrate without changing restart/stop/escalate rules.
- Rationale: Reduces regression risk and maintains constitutional requirement for explicit fault boundaries.
- Alternatives considered:
  - Add special supervision rules for hook failures now: rejected as out-of-scope for this feature and likely to complicate policy behavior.

## Decision 7
- Decision: Keep lifecycle hook state and execution records in memory only.
- Rationale: Current runtime scope is single-node and in-memory; persistence is unnecessary for this feature.
- Alternatives considered:
  - Persist lifecycle records externally: rejected as additional complexity with no stated feature value.
