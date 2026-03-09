# Production Readiness Audit

Date: 2026-03-09  
Repository: `/Volumes/Store1/src/3clabs/westcoast`

## Scope

Audit covered runtime correctness, concurrency safety, operability, observability, and deployment readiness for the current single-node actor runtime.

## Evidence Collected

- `go test ./...` passed
- `go test -race ./tests/unit/... ./tests/integration/... ./tests/contract/...` passed
- `go vet ./...` passed
- Bench sanity:
  - `BenchmarkPIDResolverLatency`: p95 ~1125ns at `WC_BENCH_TARGET=50000`
  - `BenchmarkLocalMessagingPerformance`: ~1.7M msg/s, p95 ~958ns at `WC_BENCH_TARGET=50000`

## Readiness Verdict

- Functional readiness: **Good**
- Reliability readiness: **Moderate**
- Operability readiness: **Moderate**
- Overall for production deployment: **Conditional Go** (address High findings first)

## Findings

### High

1. Request context is dropped in cross-actor ID send path.
- Evidence: [`src/actor/runtime.go:669`](/Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go:669) and [`src/actor/runtime.go:683`](/Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go:683)
- Risk: Cancellation/deadline from caller is ignored, which can cause stuck operations and weak shutdown behavior under load.
- Recommendation: Thread the provided `ctx` into downstream send path instead of replacing with `context.Background()`.

2. Actor handlers and lifecycle hooks always run with `context.Background()`.
- Evidence: [`src/actor/runtime.go:284`](/Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go:284), [`src/actor/runtime.go:365`](/Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go:365)
- Risk: No deadline/cancellation propagation into business logic; graceful stop and fail-fast behavior can be delayed or blocked by long-running handlers/hooks.
- Recommendation: Carry actor runtime context to hooks/handlers, and enforce bounded shutdown behavior.

### Medium

3. `Stop()` can block indefinitely on a long or hung stop hook.
- Evidence: [`src/actor/runtime.go:531`](/Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go:531) and [`src/actor/runtime.go:532`](/Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go:532)
- Risk: Control-plane operations can hang, impacting service shutdown/redeploy reliability.
- Recommendation: Add hook timeout policy and explicit stop deadline outcome.

4. Readiness validation records accumulate unbounded in memory.
- Evidence: [`src/actor/outcome.go:68`](/Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go:68), [`src/actor/runtime.go:713`](/Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go:713)
- Risk: Long-lived processes may grow memory usage with repeated readiness checks.
- Recommendation: Add bounded ring buffer or retention limit; expose snapshot API separately from append-only history.

5. Policy mutation methods are not synchronized.
- Evidence: [`src/actor/runtime.go:693`](/Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go:693)
- Risk: Runtime writes/reads on `pidPolicy` can race if changed concurrently in real workloads.
- Recommendation: Protect policy mode with atomic or mutex.

### Low

6. No explicit CI/release pipeline configuration in repository.
- Evidence: repository root has `Makefile` but no CI workflow files.
- Risk: Inconsistent quality gates across environments.
- Recommendation: Add CI with `fmt` check, `vet`, full tests, race suite, and benchmark smoke.

7. Module versioning/release guidance is undocumented.
- Evidence: minimal `go.mod`, no release policy document.
- Risk: Consumer upgrade risk and compatibility uncertainty.
- Recommendation: Publish semver + changelog policy.

## Strengths

- Broad automated test coverage across unit/integration/contract suites.
- Race detector clean on core suites.
- Deterministic outcome/event model for key runtime decisions.
- Guardrails for PID-only and gateway seam exist with contract tests.

## Recommended Action Plan

1. Fix context propagation gaps (High).
2. Add bounded stop-hook execution policy with timeout outcome (Medium).
3. Add synchronization for mutable runtime guardrail policy (Medium).
4. Add retention policy for readiness/guardrail history (Medium).
5. Introduce CI pipeline and release policy docs (Low).

## Suggested Gate

- Promote to **production-ready** only after High findings are resolved and Medium findings have approved mitigation plans.
