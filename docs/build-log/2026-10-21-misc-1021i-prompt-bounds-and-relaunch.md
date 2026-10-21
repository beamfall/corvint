# 2026-10-21 misc-1021i: Pi affected witness, detach relaunch race, prompt-bounds refusal code

Lane misc-1021i, base `8def072e08e9730a6102657c887655f62993b31d`. Tickets V1-2244, V1-1716, V1-1976.
New requirement `LCP-V0-034` is accepted under the owner's standing lane auto-acceptance (2026-10-11).

## V1-2244: core-host PWV-V0-012 affected selection returned undefined

Problem: `integrations/pi/core-host.test.mjs` read `plan.selected[].tests`, which the compact
`affected-plan/1` default (AFP-V0-035, V1-0943) replaced with `testCount`. Decision: the witness was
stale, not the product; assert the profile and `testCount`. Evidence: the test failed with
`undefined` before and passes 1/1 against the lane-built binary after. The manual qualification
file is not in the shipped package files, so no package version bump.

## V1-1716 (suspected): stale starting child after a relaunch over a kept LOCK_TIMEOUT

Refuted. `TestATRV0095_StaleStartingChildAfterRelaunchKeepsOneResult` keeps a LOCK_TIMEOUT, relaunches
with a slow-starting new child, runs a stale child that takes the running lock and blocks on the held
preparation lock, and waits for the new child to exit. Attach stays `PENDING` (never
`SUPERVISOR_LOST`) while the stale child holds the running lock, then returns its one kept result,
and a repeat `--detach` is byte-identical. Mutation proof: disabling the running-lock check in
`leaseDetachChildLive` makes the test fail with `SUPERVISOR_LOST`. Passes `-count=5 -race`. Recorded
in the ATR-V0-095 failure-mode text and traceability row; no runtime change.

## V1-1976: Claude Code user-prompt `dogfood-event-unavailable` had no cause

Diagnosis: the local ledger held 72 uncaused `corvint-event-rejected:dogfood-event-unavailable`
user-prompt rows. Feeding a long lane prompt (many paths) to `corvint adapter claude-code user-prompt`
reproduced it; a debug build showed `*contextindex.Error{Code: unsupported-dogfood-context-bounds}`
(more than `localPromptMaxAnchors = 32` derived anchors). `dogfoodReadFailureCode` only mapped
`gokernel.Error`, so every prompt-contract refusal collapsed into unavailable. That long prompts are
the dominant ledger cause is an inference; the old rows carry no cause.

Decision: `LCP-V0-034` maps the three prompt-contract codes to one fixed `dogfood-event-prompt-bounds`
and `SOL-V0-010` admits it. Bounds are unchanged (residual: cap or trim the over-bound derived query
to the anchor profile so long prompts degrade to a partial packet instead of a refusal).

Evidence: `TestV11976PromptContractRefusalHasNamedCode` and `TestAdapterDegradationAdmitsPromptBounds`
fail without the change and pass with it; `./internal/observations` and the cmd/corvint adapter and
dogfood tests pass.

## NOT_RUN

`make gate` (owner preference for scoped work); affected tier 2 units left to batch CI.

## Rollback

Revert the lane commit: the Pi test assertion, the ATR test plus spec text, and the
`dogfoodReadFailureCode` branch plus registry entry (refusals return to `dogfood-event-unavailable`).
