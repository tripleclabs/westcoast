# Contract: Ask Request-Response and Asynchronous Replies

## Purpose
Define caller-visible and responder-visible guarantees for Ask semantics, implicit ReplyTo context, and deferred reply behavior.

## Ask Interface Contract
- Runtime MUST provide Ask-style request-response call semantics with timeout-bounded wait.
- Ask MUST complete with exactly one terminal caller outcome: success, timeout, canceled, or delivery failure.
- Ask timeout MUST return deterministic timeout error and release the wait path.

## Implicit Reply Context Contract
- Every Ask-originated message MUST include reply-routing context.
- Reply-routing context MUST include:
  - `request_id`
  - `reply_to` PID
- Receiver MUST be able to extract this context without payload-specific conventions.

## Reply Routing Contract
- Responder MUST be able to send reply using only `reply_to` PID and reply payload.
- Reply correlation MUST use `request_id` so concurrent Ask calls do not cross-wire.
- At most one reply may complete a pending Ask request.

## Asynchronous Delegation Contract
- Receiver MAY defer work and reply later through `reply_to` from outside the immediate handler frame.
- Deferred reply path MUST preserve actor loop responsiveness for unrelated messages.
- Deferred replies remain subject to Ask timeout/cancellation lifecycle.

## Late and Invalid Reply Contract
- Replies arriving after Ask completion timeout/cancel MUST NOT reopen or mutate completed wait state.
- Late replies MUST be rejected or ignored deterministically and produce observable non-success outcome.
- Invalid or unreachable `reply_to` destinations MUST emit deterministic delivery-failure outcome.

## Supervision Compatibility Contract
- Responder actor failures before reply follow existing supervision policy (restart/stop/escalate).
- Ask semantics MUST not alter existing supervision decision taxonomy.

## Observability Contract
Required terminal Ask outcomes:
- `ask_success`
- `ask_timeout`
- `ask_canceled`
- `ask_reply_target_invalid`

Required auxiliary outcomes:
- `ask_late_reply_dropped`

Required fields:
- `event_id`
- `request_id`
- `actor_id` (caller or responder context as applicable)
- `reply_to` (if available)
- `outcome`
- `timestamp`
- `reason_code` (for non-success outcomes)

## Invariants
- Every Ask request has one and only one terminal caller outcome.
- Ask correlation is isolated per request and never shared implicitly across calls.
- Reply routing remains PID-based and location-transparent.
- Deferred reply behavior must not force synchronous completion within actor handler execution.
