# 2026-10-09: V1-0844 frontier Stop deadline evidence and live HLQ rerun

## Intent

V1-0844: the `1.0.0-rc.2` candidate `da78c157` (build 358) Claude Code HLQ run reported
`case frontier FAIL enrolled incomplete Stop returned decision <nil>` and discarded the hook output.
Acceptance: a failed frontier case reports the Stop hook's streams, exit code, elapsed time and
degradation; and the intermittent `decision <nil>` is either root-caused from evidence or shown to be
deadline-bound and handled without weakening the blocking claim. Contracts: `HLQ-V1-009`,
`LCP-V0-008`, `LCP-V0-009`, `AHI-017`.

## Starting point

Commit 97968c51 (2026-10-06, `2026-10-06-hlq-time-bound-hook-diagnostics`) already delivered the
reporting (acceptance 1: `TestHookTimeBoundDegradation`) and the harness correction (retry of a
time-bound fail-open, at most three runs, every run named; no deadline widened; frontier still
passes only on `decision=block`). The fail-open on expiry is specified (`LCP-V0-008`: timeouts fail
open with explicit failure; `dogfood-event-deadline` in the `LCP-V0-009` code table), so no product
behavior or deadline changes here. Left open: a deterministic test on an *enrolled* Stop, and the
real-host rerun under `HLQ-V1-009` (`NOT_RUN`).

## Change

- New subtest of `TestClaudeNativeDogfoodLifecycle` (`cmd/corvint/local_completion_claude_test.go`):
  a real enrolled incomplete Stop evaluation completes and decides `block`, then the event deadline
  is forced to expire (witnessed deadline, no wall-clock race, decision 0082). The adapter output
  has no `decision` and only `systemMessage` `Corvint FALLBACK degraded:
  corvint-event-rejected:dogfood-event-deadline; coding continues`, which is the rc.2 symptom and
  the shape `HLQ-V1-009` retries.
- `docs/specs/host-lifecycle-qualification-v1.md`: V1-0844 diagnostic run subsection, the known-gap
  paragraph updated with the live and probe evidence, `HLQ-V1-009` traceability names the subtest.
- Raw report and boundary load reading retained under
  `conformance/host-lifecycle-v1/results/2026-10-09-v1-0844-frontier/` (`HLQ-V1-008` convention).

## Evidence

- New subtest PASS. Mutation (the read returns its result instead of expiring) makes it FAIL with
  `decision:block ... Frontier authority remains unavailable`, so it detects the difference. The
  subtest does not exist on base `02e84575`.
- `go test ./conformance/host-lifecycle-v1`: PASS (existing reporting tests unchanged).
- Live: `go run ./conformance/host-lifecycle-v1 --host claude-code` with a corvint built from
  `02e84575` (not installed to PATH) and the rc.1 build 163 binary as N-1, Claude Code 2.1.293:
  9/9 PASS, no time-bound retry, 8 s, load 9.3.
- Scratch probe (patched copy of the runner, not committed): 40 enrolled incomplete Stop hook runs
  on the same host at one-minute load 20–26 on 12 CPUs all returned `block`; elapsed median about
  320 ms, maximum 553 ms, against the 1.5 s adapter work bound (2 s declared kill − 400 ms reserve
  − 100 ms grace). Repeated enrolled Stops keep blocking, so the continuation limit is stateless
  (`stop_hook_active` only) and the `HLQ-V1-009` retry cannot exhaust it.

## Finding

Deadline-bound, shown deterministically; root cause of the specific rc.2 occurrence is the most
likely hypothesis, not an observation: the only no-decision outputs of an enrolled incomplete
first Stop are the time-bound fail-opens (this subtest, `TestClaudeAdapterStopDeadlineFailsOpenVisibly`,
the watchdog path) and non-load-dependent paths (another session's owner, non-time-bound
rejection, internal error). A roughly 5x slowdown reaches the bound; rc.2 ran at load about 245 on
12 CPUs. The discarded rc.2 output cannot be recovered.

## Limits

- Load-reproduction of the fail-open on the real host: `NOT_RUN` (would need manufactured load on
  a shared host).
- The live run is a newer host version (2.1.293) and an unreleased build; it is not a tuple result.
- `make gate`: `NOT_RUN` (lane policy; focused tests only).

## Rollback

Revert this change's commit (test subtest, spec text, results directory, this entry). No state.
