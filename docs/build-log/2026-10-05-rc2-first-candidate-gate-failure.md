# 2026-10-05: 1.0.0-rc.2 first candidate fails the full gate

## Result

Candidate `0ae3f88d0469078052edf088ea2d50e290b0b8d7` (build 357, PR #616) failed the release
runbook's step-4 `make gate` on darwin/arm64 and both hosted `full-gate` jobs (Release gates run
37358656556). It was not tagged. Its runbook evidence is retained under the private release-evidence
directory `1.0.0-rc.2-0ae3f88d`. Every other step there and every other hosted job passed:

- archive, checksums, reproducibility;
- lifecycle and N-1 lifecycle, hostile regressions, `core-n1-replay`;
- the three HLQ tuples, interop gate, focused docs.

A fresh candidate will be cut from main once the fixes below merge, and the runbook rerun on it.

## Defects

Neither defect changes product behavior; both are test-only.

1. **Windows cross-vet (ubuntu full-gate).** Untagged test files use helpers that are defined only
   in `unix` or `darwin || linux` tagged test files:
   - `goRequest` in `internal/criterionexperiment` (V1-0797);
   - `testConfig` in `internal/tasks/dispatch` (V1-0839);
   - `PSRTestResponseFailure`, `psrPools` and `multiCommitted` in `internal/tasks/store` (V1-0765).

   The users now carry the same build tag as the file that defines their helper. They reached main
   because hosted CI does not run `cross-vet` (V1-0359, V1-0400).
2. **Symlinked TMPDIR (darwin full-gate).** Four `internal/tasks/service` tests passed a raw
   `t.TempDir()` to `safeopen.Root`. On macOS that path is under `/var -> /private/var`, and
   `safeopen.Root` refuses a symlink anywhere in a path by contract (V1-0840).
   - Three of the tests failed.
   - `TestSERVICE500_LogSinkIOErrorIsReportedNotBlocking` blocked on an unread gate send until the
     30-minute package timeout.

   The tests now resolve their temp dir, as `lifecycle_test.go` already did, and the wedged-writer
   send is bounded so a failed open fails fast. Ubuntu runners and resolved private TMPDIRs hide
   this defect, which is why the PR CI passed. V1-0753 is the same class in `internal/tasks/store`.

## Gate order note

`make gate` stops at the first failing step. Locally it stopped at `go-test` and on ubuntu at
`cross-vet`, so neither failed `full-gate` run reached the `make gate` steps after `cross-vet` (the
hosted interop and focused-docs jobs ran some of them separately). Before merging the fix, those
steps were run locally with `make -k`.
