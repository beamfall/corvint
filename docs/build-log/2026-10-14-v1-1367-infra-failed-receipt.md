# 2026-10-14 V1-1367: offline receipts with infrastructure-failed attempts verify

Lane V1-1367, Corvint 1.0 scope `PRS-V1-018` (accepted, owner instruction 2026-10-13).
Requirement `BBF-V0-018` (`docs/specs/browser-behavior-falsification-v0.md`), accepted under the
owner's standing auto-accept (2026-10-11). Lane base 92c298a6e (origin/main after PR #873),
merged forward to 057b19008 (after PRs #874 and #875); the change evidence binds against 057b19008.

## Intent

An offline evidence receipt (`BBF-V0-013`) must stay verifiable when an attempt is lost to
infrastructure. Before this change, a well-formed receipt containing such an attempt could not
verify, so a falsification run with one hook timeout produced unusable evidence.

## Reproduction (real producer)

A scratch test drove `ExecuteReceipt` with the existing live helper hook at BASE 92c298a6e and at
PR #875's head 86239463e (V1-1366). The hook ran the `timeout` scenario with a one-second budget,
so `runPinned` returned a timed-out process. The attempt had no admitted receipt (`Receipt` nil),
`StdoutSHA256` was the digest of empty stdout, and its status was `infrastructure_failed`.
The capture callback ran before admission. It tried to decode stdout itself, found no native
report and appended `native-report-unavailable`. `ExecuteReceipt` then relabelled the attempt
`invalid_control` because an omission was present. `VerifyReceipt` refused the result
(`receipt-invalid-control`) twice: on the omission, and on the decoded-receipt `DeepEqual` against
a nil `Receipt`. A hook crash, an output overflow and undecodable stdout failed the same way.
When command staging was unavailable, `StdoutSHA256` stayed empty but a raw entry was still
appended, so verification failed on `rawIndex != len(RawAttempts)`. PR #875 does not change which
attempts carry a receipt, so the defect is the same on its head.

## Decisions

- Producer (`execute.go`, `receipt.go`). The capture callback now runs after admission and before
  cleanup removes declared native reports, and it receives the `AttemptResult`. It reads a native
  report only through the admitted `Receipt`. It marks `native-report-unavailable` only when an
  admitted receipt has no readable native report. It retains nothing when no hook stdout was
  observed (`StdoutSHA256 == ""`), so the receipt matches the verifier's existing skip path.
- Verifier (`VerifyReceipt`, `unadmittedAttemptVerifies`). The common raw checks run unchanged for
  every attempt that has a raw entry: ordinal, ID, attempt, no omissions, `HookSHA256 ==
  digest(HookBytes) == HookProcess.StdoutSHA256`. Only then does the verifier branch on `Receipt`.
- Outcomes that now skip the decoded-receipt check. Only attempts with `Receipt == nil` skip it.
  `classifyAttempt` gives such an attempt exactly one of two outcomes:
  - `infrastructure_failed`, when the hook process did not succeed (timeout, crash, overflow,
    cancellation). This is safe because the outcome is recomputed from the bound process record
    with reason `hook-process-failed`. That is the executor's own derivation, and it never counts
    as a kill.
  - `invalid_control`, when the hook process succeeded but its stdout did not decode as a receipt
    (`hook-output-invalid`), or cleanup was invalid. This is safe because the verifier decodes the
    bound stdout itself and refuses whenever those bytes decode as a hook receipt, whatever process
    outcome the attempt records. So retained receipt bytes cannot be hidden behind a forged process
    failure. A hook that printed a decodable receipt and then failed still refuses offline
    verification. That fails closed, exactly as BASE did for every unadmitted attempt.

  In both cases the verifier also refuses retained native bytes, a native digest, retained
  artifacts or a workspace baseline other than the plan's. The recomputed status and reasons must
  equal the recorded ones. `killed` and `survived` are refused outright. Those two outcomes always
  require a decoded receipt that matches the retained bytes, a bound native report and a
  reclassification, unchanged from `BBF-V0-013`. `not_run` and `not_supported` attempts, and
  attempts refused for `workspace-baseline-drift`, start no hook and observe no stdout, so they
  never reach this path.
- Attempts with no observed stdout (`unobservedAttemptVerifies`). At BASE this skip path refused
  only `killed` and `survived` and accepted any other status string. A forger could relabel such an
  attempt `not_supported`, or give it a status outside the vocabulary, and `aggregateAttempts`
  would then count the control as killed. The verifier now accepts such an attempt only when its
  hook never started and it carries no receipt, retained artifacts or application (BBF-V0-015).
  Its status must be `infrastructure_failed`, `invalid_control` or `not_run`. Each of those keeps
  `aggregateAttempts` from returning a kill or a survivor. The status is an allow-list, not a
  recomputation, because `internal/testacceptance` builds synthetic unobserved attempts with empty
  process records (`TestPTFV0ParentCompleteAttemptOutcome`), and that package is outside this
  lane's scope. A relabel among the three non-scoring outcomes changes no score.
- Merge with PR #875 (V1-1366, BBF-V0-015..017). `classifyAttempt` now also returns the attempt's
  application. A nil-receipt attempt verifies only with an empty application, so application
  markers cannot be attached to an attempt without an admitted receipt.

## Failure modes and limits

- A forger who replaces the stdout bytes and every digest still yields an unauthenticated
  observation (`BBF-V0-012`). The only thing such a forgery can claim is a non-kill, non-survivor
  outcome, which lowers no mutation score.
- Downgrades cannot be prevented. A forger can still drop an attempt (`attempt-set-incomplete`)
  or replace stdout wholesale, recompute every digest and record a process failure. Either turns
  a kill or survivor into a non-scoring outcome. This was already possible at BASE, and only
  authenticated hook output (`BBF-V0-012`) could close it.
- An unadmitted attempt with any omission, including `retained-byte-bound`, still refuses
  verification. Retaining over-bound stdout is out of scope.
- Unchanged and out of scope: for receipt-bearing attempts, the existing reclassification clears
  reasons, so an admitted attempt that also carries `cleanup-budget-overrun` may still refuse.

## Acceptance evidence

- `TestBBFV0018InfrastructureFailedAttemptVerifies` runs the real producer through four cases:
  hook timeout, crash after partial stdout, undecodable stdout and unavailable command staging.
  Each receipt verifies, with its raw stdout bound to `StdoutSHA256` and no kill or survivor.
- `TestBBFV0018ForgedNilReceiptAttempts` reseals aggregates and digests after each mutation. It
  rejects these nil-receipt forgeries:
  - a forged kill, a forged survivor, and a forged kill with a successful process;
  - a tampered stdout byte, with and without a recomputed raw digest;
  - a hidden admitted receipt;
  - a retained native report or retained artifacts;
  - an injected reason, a changed workspace baseline, and an omission.

  A mutation check against a weakened helper confirmed that each guard is exercised.
- `TestBBFV0018ForgedUnobservedAndHiddenReceipts` rejects five forgeries of an unobserved
  attempt: relabelled `not_supported`, an unknown status, a kill, a started hook and an
  application. It also rejects an admitted kill whose receipt is hidden behind a forged timeout.
  The `not_supported`, unknown-status and hidden-receipt cases fail against the pre-review
  verifier. `TestBBFV0018ForgedNilReceiptAttempts` also rejects an application on a nil-receipt
  attempt.
- `TestBBFV0018PreHookAttemptShapesVerify` confirms that the executor's two pre-hook shapes
  (`workspace-baseline-drift` and `wall-clock-budget-exhausted`) verify.
- Codex review (one round) reported two P1 findings: the not_supported relabel and the hidden
  receipt. Both are fixed as described above.
- Focused tests:
  - The `corvint affected` units passed. Its 43 unbounded readers were not run.
  - On the merged tree (057b19008 plus this change), `internal/behaviorfalsify`,
    `internal/testacceptance` and `cmd/corvint-behavior-falsify` passed.
  - `go test -race` of `internal/behaviorfalsify` and `internal/testacceptance` passed with no
    data race. Earlier `-race` runs under host load averages of 40 to 130 failed only where
    procgroup reported a descendant snapshot as unavailable. That defect belongs to V1-1368 and is
    outside this lane's scope.

## Rollback

Revert the change commit. Receipts produced after this change keep the same wire shape. A rolled
back verifier refuses receipts that contain unadmitted attempts, which is the prior behaviour.
