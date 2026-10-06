# 2026-10-06: Why 1.0.0-rc.2 took four candidates

## Result

Each of the first three rc.2 candidates failed only the hosted macOS `full-gate` job, and each
failure exposed defects the previous run had not shown. The cause is structural: one hosted macOS
run could report only part of the gate, and nothing ran the gate on macOS between releases.

| Candidate | Hosted macOS `go-test` | Later gate steps |
|---|---|---|
| `0ae3f88d` (run 37358656556) | 206 `ok`, `internal/tasks/service` hit the 1800s timeout; the 184 unresolved packages never ran | not run |
| `da78c157` (run 37381427566) | 374 `ok`; `cem/verify`, `localcompletion`, `tasks/store` failed | not run |
| `7a83f52d` (run 37403195661) | 376 `ok`; `cem/verify` failed (V1-0846) | not run |

All three logs show `PARTITION 129 resolved packages under per-package keys, 184 unresolved under
the tree key`. The first log has no `RUN go-test-unresolved` line.

## Causes

1. **`ledger/go-test` stopped after the resolved batch failed.** `goTest` returned the resolved
   batch's status before it ran the unresolved packages. So candidate 1 hid the failures that sank
   candidate 2. That broke GL-V0-004, which requires every package `go test ./...` would run.
   Fixed here (V1-0847): the unresolved packages still run, and the step exits with the resolved
   batch's status.
2. **`make gate` stops at the first failed step.** For all three candidates, nothing after
   `go-test` ran on hosted macOS: `go-vet`, `cross-vet`, `interop-gate` and the doc checks. The
   hosted workflow change that keeps the gate going is a separate `.github/` change (decision
   0390 merge rule, V1-0848).
3. **macOS is exercised only at release time.** PR CI runs on ubuntu only. `release-gates.yml`
   runs only when the owner dispatches it for a release. Four macOS-only defects landed after
   rc.1 and were first seen by a candidate: V1-0841 (descriptor exhaustion), V1-0842 (symlinked
   `TMPDIR`), V1-0843 and V1-0846 (the S0E case). The workflow change also schedules a nightly run
   on `main`.
4. **The local runbook does not predict the hosted runner.** The release host differs in
   `RLIMIT_NOFILE`, the `/usr/bin/git` version and the `TMPDIR` layout. The local `make gate`
   passed on every candidate.
5. **Fix-to-candidate latency.** Each fix went through review, merge and a new dispatch; the hosted
   macOS leg alone takes about three hours. Twice, a completed auto-merge woke no follow-up, and
   the next candidate started 1.5 and 8.5 hours late. The operator now drives each wait from a
   command whose exit resumes the work, not from a merge notification.

## Evidence

`TestGoTestRunsUnresolvedAfterResolvedFailure` fails on the previous `goTest`: the unresolved
packages do not run. It passes with the fix. The candidate counts come from the downloaded hosted
logs named in the table.
