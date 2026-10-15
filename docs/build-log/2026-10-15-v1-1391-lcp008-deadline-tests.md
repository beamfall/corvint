# 2026-10-15 V1-1391: force the LCP-V0-008 deadline orderings

Lane V1-1391 (BUG, P1), requirement `LCP-V0-008` (`docs/specs/local-completion-policy-v0.md`,
amended in place; no new requirement ID). Base `c76e3d790969b1a1fb1e8f3d420c4561fb52919d`
(origin/main after PR #902).

## Observed

Two full-gate closeouts on a darwin host at load 20-115 failed in `cmd/corvint`:

1. `TestDogfoodEventStrictInputAndDeadline/LCP-V0-008_expiry_does_not_wait_for_an_uncancellable_read`
   (V1-0285 gate, evidence `fd4cef2f`): `expired event was not reported as a time bound:
   {"code": "dogfood-event-input-unavailable", ...}`.
2. `TestClaudeNativeDogfoodLifecycle/LCP-V0-008_first_and_recursive_Stop_preserve_unresolved_enrollment`
   (V1-0253 gate, evidence `723ebd77`): `first Stop did not block`, via the
   `corvint-event-rejected:dogfood-event-deadline` fallback after 10.20 s.

## Diagnosis

1. Test ordering and a product classification defect. The subtest gave the event a 10 ms deadline
   and assumed it would expire inside the uncancellable read stage. On a loaded host it can expire
   earlier, during the stdin input read. `readInputBounded` then returns the context's error, and
   `runLocalCompletionEvent` reported every input-read failure as `dogfood-event-input-unavailable`.
   LCP-V0-008 already says an expired automatic event reports `dogfood-event-deadline`, even when
   the expiry first surfaces as a failed read. The input-read case was therefore misclassified.
   The neighbouring `LCP-V0-008 deadline` subtest had tolerated `input-unavailable` to hide this.
   In production the same expiry gave a Claude Code hook `dogfood-event-input-unavailable`, and with
   it no deadline remediation notice (`withDeadlineRemediation` keys on the deadline reason).
2. Test only; the product classification is correct. `lifecycleDeadlineContext` lifts the event
   deadline to ten minutes, but the Git repository probe keeps its own fixed 10-second bound
   (`gokernel.gitDeadline`). `probeRepositorySharing` applies that bound to the whole bracket,
   including the read and the in-memory index build of a snapshot miss. With the event deadline
   lifted, that bound becomes the test's latency bound. Under load it expires, and V1-0396
   correctly reports it as `dogfood-event-deadline`, or as `dogfood-event-index-snapshot-stale`
   after a miss. The 10.20 s elapsed time matches the 10 s bound. In production the 1.6 s event
   deadline expires long before it. The same cause failed other lifecycle subtests on this host:
   AHI-003, AHI-045, LCP-V0-002, LCP-V0-010 and HLQ-V1-009.

## Change

- Product (LCP-V0-008 amended 2026-10-15): if the stdin read ends after the deadline has passed,
  the event reports the expiry code (`dogfoodInputFailureCode`, appended at the end of
  `local_completion_event.go` so no pinned citation moves). `dogfood-event-input-unavailable`
  stays for a read that fails before the deadline. The error-code table and the traceability row
  are updated.
- Seam: `gokernel.WithProbeBound` replaces the Git probe's fixed bound for probes run under a
  context. It is appended at the end of `repository.go`, and the three `WithTimeout(parent,
  gitDeadline)` sites read it through `probeBound`. No production path sets it, and the
  production value stays 10 s. `TestProbeBoundReplacesGitDeadline` pins all three probe entry
  points both with and without the seam.
- Tests: `lifecycleDeadlineContext` and the HLQ-V1-009 subtest lift the probe bound beside the
  event deadline (decision 0082). The probe-timeout classification is still covered by the
  `LCP-V0-008 Git probe expiry` subtest. The uncancellable-read subtest now expires its deadline
  only from inside the read stage (`workFinalInterruption`; the one-minute deadline is a hang
  detector). A new subtest, `LCP-V0-008 expiry during the input read is the time bound`, uses a
  stdin reader that expires the deadline and then blocks. The `LCP-V0-008 deadline` loop no longer
  accepts `input-unavailable`. No product deadline and no pass/fail bound was widened.

## Evidence

Contention runs used 64 `yes` burners on this 12-core host, which put the load average at
133-180. Burners started and stopped inside each foreground command. Runs used prebuilt test
binaries for BASE and for the fix.

| Binary | Selection | Iterations | Failing subtests |
|---|---|---|---|
| BASE | StrictInputAndDeadline + ClaudeNativeDogfoodLifecycle + ReadOnlyEnrolledStopAndPrompt | 2 x 3 (unlogged) | 7 |
| BASE | same | 2 x 3 | 15: first Stop 1, HLQ-V1-009 1, AHI-003 5, AHI-045 1, LCP-V0-010 1, ReadOnly 6 |
| BASE | StrictInputAndDeadline + ClaudeNativeDogfoodLifecycle | 1 x 3 | 4: AHI-003 3, LCP-V0-002 1 |
| fixed | three-test set | 2 x 3 | 1: ReadOnly `LCP-V0-001 authority` (see residual) |
| fixed | two named tests | 2 x (1 x 2) | 0 |

The fixed binary had 0 failures in 10 iterations of the two named tests; BASE failed 18 named-test
subtests in 9 iterations. Failure 1 did not reproduce naturally: BASE passed 200/200 of the subtest
under 32 burners. It is reproduced deterministically by mutation M2.

Mutations (each applied alone, restored afterwards):

| ID | Mutation | Failing test |
|---|---|---|
| M1 | `dogfoodEventWithin` waits for the read (drop the `ctx.Done` case) | uncancellable-read subtest, after its one-minute hang detector |
| M2 | restore BASE `dogfood-event-input-unavailable` on any input-read failure | `LCP-V0-008 expiry during the input read is the time bound` |
| M3 | `liftedProbeBound` uses 1 ns (the probe expires first, as under load) | `LCP-V0-008 first and recursive Stop`: `first Stop did not block` with the `dogfood-event-deadline` fallback, the gate symptom |
| M4 | `probeBound` ignores the context value | `TestProbeBoundReplacesGitDeadline` |
| M5 | `probeBound` defaults to `gitDeadline / 2` | `TestProbeBoundReplacesGitDeadline` (unlifted case) |

## Independent review

One Codex round (`codex exec -s read-only`, range `c76e3d79..00b5c168`) reported three findings:

- P2: the classification depends on scheduling. A stdin read that fails just before the deadline
  but is examined after it reports the time bound. This is intended: it is the rule LCP-V0-008
  already applies to every later read ("even when the expiry first surfaces as a failed read").
  The reviewer also found that input which finished reading as the deadline passed but is
  malformed still reports `invalid-dogfood-event-input`. That is a deterministic fault in the input
  itself, not an expiry surfacing as a failed read, so it is recorded and not changed.
- P2: the seam test checked only "more than 10 s left", so a shorter default would have passed.
  Fixed: the test now checks that every Git read's deadline lies between the probe's start plus
  the bound and the read's own time plus the bound, which holds at any host load (mutation M5).
- P3: `WithProbeBound` is exported from a non-test file. It has no non-test caller. This is
  accepted and recorded, on the same footing as the existing `dogfoodEventDeadlineKey` test seam.
  A repository scan test was not added, because it would make `gokernel` an unbounded reader.

## Residual

`TestDogfoodEventReadOnlyEnrolledStopAndPrompt/LCP-V0-001_authority` calls
`gokernel.HandleEvent` with no context, so it cannot carry the probe seam. It failed once under the
fixed binary with `Git repository probe exceeded its 10-second deadline`. That is the same class
but outside this ticket and should be filed separately. Product Stop fail-open under load stays
with V1-1120.

## Rollback

Revert the lane's commits. Reverting brings back the input-read misclassification and the
load-dependent lifecycle tests. No stored data or wire format depends on this change.
