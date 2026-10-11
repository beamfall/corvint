# 2026-10-10 — Retirement token wait, PSR flake evidence, refusal-table repin (V1-1152, V1-1153, V1-1154)

Lane `claude/w6-flakes`, base `55aa6bb9f` (origin/main). Umbrella V1-0849.

## V1-1152 — groupreap/testrunner "owner token unreadable ... invalid argument" (CONFIRMED, product)

Reproduced on Darwin arm64 with two fork loops (`while :; do /usr/bin/true; done`) and one CPU loop
beside `go test -count=6 ./internal/groupreap ./internal/testrunner -run 'Retire|Retirement'`:
`TestRetirerRetiresDetachedDescendants`, `TestExecuteRetiresDetachedDescendants` and
`TestRetirementSurvivesLaterExecutionFailure` failed with `Problems:[owner token unreadable for pid
N: invalid argument]`; all pass unloaded.

Cause: `KERN_PROCARGS2` returns `EINVAL` for a process that is exiting (or has not yet executed)
while `kern.proc.pid` still reports it alive and not a zombie. A scratch probe over 400 short-lived
`/usr/bin/true` children measured that window at up to 9.9 ms idle (383 samples) and 7.3, 16.9,
117.4, 14.1, 10.4 and 29.8 ms across six loaded runs. `Retirer.token` retried five times with a
nominal 2 ms sleep (about 10 ms), so an unrelated same-uid process caught in that window on a
loaded host became a retained problem and the phase an `execution-boundary` failure.

Fix (`internal/groupreap/retire.go`): the attempt count becomes a bounded wait counted in nominal
sleep time, 5 ms polls for at most 1 s per identity and 2 s per `Retire` call; an identity that
disappears during the wait is neither owned nor a problem, and one that persists past the bound is
still retained as a problem, so no uncertainty is converted to exit or non-ownership. The fake
sleep in tests records nominal time, which keeps `TestRetirerWaitsOutRefusedTokenReads` and
`TestRetirerKeepsReadFailuresAsUncertainty` deterministic. `TRE-V0-030` is amended with the bound
and its reason, proposed pending owner acceptance (V1-1152). No decision file.

Evidence: `go test -race ./internal/groupreap -run TestRetirer` and `./internal/testrunner -run
'Retire|Retirement'` pass; the same load repro rerun after the fix is recorded in the lane report.
The scratch probe was not kept; the measured numbers above are its only record.

## V1-1153 — store PSR tests under `-race` (SUSPECTED, not reproduced; diagnostic only)

Original evidence (`w4-race.log`, 2026-10-10, `-race` over the whole module with the store package
running 1800 s before the 30 m alarm): `TestPSRSweepStderrIsNotStdout` (14.41 s) saw the entry
`QUARANTINED` with reason `TIMEOUT; safe reuse failed`, so the 3 s `safeReuse.timeoutSeconds` phase
budget expired before `/bin/sh -c 'printf expected >&2'` finished; the product classified the
phase correctly. `TestPSRAmbiguousResponseReconciliation/confirm-response` (12.10 s) failed its
bounded-reconciliation assertion in the same run; its dump of `out` as bytes hid the entry class.
An older run (2026-10-07) shows a different class, `UNSUPPORTED: sweep git: bounded observation
failed: UNKNOWN`, from the 1 s runner-identity probe.

This lane ran `go test -race -count=3 ./internal/tasks/store -run 'TestPSRSweepStderrIsNotStdout|
TestPSRAmbiguousResponseReconciliation'` under the same two-fork-loop plus CPU-loop load: PASS,
62.7 s. The suspicion is host oversubscription exceeding fixed fixture budgets, not a product
defect; the budgets were not changed because no reproduction shows which bound matters. The two
assertions now print each pool entry's state and sweep reason (`psrEntryStates`) so the next failure
classifies itself as `TIMEOUT`, `INTERRUPTED`, `STDOUT_MISMATCH` or a probe refusal.

## V1-1154 — `local-completion-policy-v0.md` refusal-table citations

On `55aa6bb9f` the ticket's count did not hold: `lifecycle.go` has 15 line citations in the table,
5 wrong, all among the 6 rows w4 already repins (`claude/batch-2026-10-11-w4`, commit `0fd75071`).
The real debt was 48 wrong rows out of 94 first citations, 43 of them into `storage.go`,
`finish.go`, `cmd/corvint/local_completion_event.go` and
`internal/contextindex/local_completion_context.go`. This change repins 40 rows to the line that
returns the code, each with a `@<hex>` content anchor (V1-0992 precedent) so the gate now fails on
drift instead of pointing at unrelated code. Eight rows are left untouched because w4 also edits
them and the merge must stay trivial: `check-executable-unavailable`,
`enrollment-generation-bound-exceeded`, `invalid-local-state-directory`, `invalid-tree-listing`,
`report-set-stale`, `repository-identity-changed`, `uncommitted-work`, `verification-cancelled`.
All repinned lines lie outside w4's hunks in those files (`storage.go` shifts start at line 457,
`local_completion_event.go` at 179), so the anchors survive the batch merge. The gate has no
symbol-anchor form; `@<hex>` is the available mechanical anchor. `make line-citations-check` passes.
