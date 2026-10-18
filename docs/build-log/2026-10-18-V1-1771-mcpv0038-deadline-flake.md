# V1-1771: MCPV0-038 caller_deadline subtest no longer races the context timer

Issue [#1116](https://github.com/beamfall/corvint/issues/1116): the macos-15 nightly full gate failed
`TestMCPV0038IndexDeadlineHasDistinctCode/caller_deadline` (`git_timeout_test.go:100`, got
`repository-probe-timeout`, want `cancelled`).

Cause: the subtest used `context.WithTimeout(50ms)` and a fake build that slept 60ms. A timer
context's `Err()` is set only when its timer callback runs, which can lag on a loaded host, so the
bridge saw a live context.

Fix (test only): the `end` hook now receives the request context and the `caller deadline` case
waits on `<-ctx.Done()`, so `Err()` is guaranteed set before the fake build returns. No fixed sleep.

Product code is unchanged. `MCPV0-038` (decision 0533) classifies a Git deadline as
`repository-probe-timeout` only while the request context is live; an ended context is `cancelled`.
The bridge already does that, so no spec or decision wording changed and no requirement ID was added.

Evidence: `-race -count=200 -run TestMCPV0038IndexDeadlineHasDistinctCode ./internal/mcp/bridge` ok;
whole `./internal/mcp/bridge` package ok. The release owner closes the issue after a scheduled run passes.
