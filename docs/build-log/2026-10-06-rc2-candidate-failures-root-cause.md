# 2026-10-06: Why 1.0.0-rc.2 took four candidates

## Result

Each of the first three rc.2 candidates failed a hosted `full-gate` job, and each failure exposed
defects the previous run had not shown. Candidate 1 also failed the ubuntu leg (Windows
`cross-vet`); candidates 1 and 2 also failed the local `make gate`; candidate 3 failed only the
hosted macOS leg. The cause is structural: one run could report only part of the gate, and
neither macOS nor `cross-vet` ran anywhere between releases.

| Candidate | Hosted macOS `go-test` | Later macOS gate steps |
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
3. **macOS and `cross-vet` are exercised only at release time.** PR CI runs on ubuntu only and
   does not run `cross-vet` (V1-0359, V1-0400). `release-gates.yml` runs only when the owner
   dispatches it for a release. Defects that landed after rc.1 were first seen by a candidate:
   Windows `cross-vet` (V1-0765, V1-0797, V1-0839), symlinked `TMPDIR` (V1-0840, V1-0842),
   descriptor exhaustion (V1-0841) and the S0E Git path (V1-0846; V1-0843 was its first, wrong
   diagnosis). The workflow change also schedules a nightly run of every hosted job on `main`.
4. **The local runbook does not predict the hosted runner.** The release host differs in
   `RLIMIT_NOFILE`, the `/usr/bin/git` version and the `TMPDIR` layout. Candidate 3 passed the
   local `make gate` and failed on the hosted runner; candidate 2's descriptor and S0E failures
   were hosted-only too.
5. **Fix-to-candidate latency.** Each fix went through review, merge and a new dispatch; the hosted
   macOS leg alone takes about three hours. Twice, a completed auto-merge woke no follow-up, and
   the next candidate started 1.5 and 8.5 hours late. The operator now drives each wait from a
   command whose exit resumes the work, not from a merge notification.

## Evidence

`TestGoTestRunsUnresolvedAfterResolvedFailure` fails on the previous `goTest`: the unresolved
packages do not run. It passes with the fix. The candidate counts come from the downloaded hosted
logs named in the table.
