# 2026-10-08: Batch G integration (V1-0519, V1-0588/0589, V1-0662, V1-0672, V1-1025)

## Intent

Integrate five finished, independently reviewed lanes as one batch branch,
`claude/batch-g-2026-10-08`, on top of batch F (`claude/batch-f-2026-10-08` head `b9d8bc40`). Each
lane is merged with `git merge --no-ff`, so its reviewed commits stay intact. The lanes are based on
`origin/main` `ecfff8e0` (PR #678, V1-1016), which batch F (based on `2a93b5a1`) did not contain, so
the first merge also brings in V1-1016 (`CAL-V0-191`) and its conflicts with batch F.

## Merges

| Order | Lane branch | Lane head | Renumber commit | Merge commit | Conflicts |
| --- | --- | --- | --- | --- | --- |
| 1 | `claude/v1-0519-tour-destinations` | `1cfa85dd` | none | `6f90e9ca` | leases spec, `INDEX.json`, `README.md`, `REQUIREMENTS.tsv` (all from main's V1-1016 against batch F's V1-1021) |
| 2 | `claude/v1-0588-0589-release-smoke` | `7bd9a48f` | none | `bc0a4d4c` | none |
| 3 | `claude/v1-0662-fixture-maintenance` | `21cf4e02` | none | `ede4785a` | `internal/liveverify/affected/typescript/mocha_qualification_test.go` |
| 4 | `claude/v1-0672-ledger-save` | `35b60927` | `d8613f3b` | `00ea66ef` | leases spec, `INDEX.json`, `README.md`, `REQUIREMENTS.tsv` |
| 5 | `claude/v1-1025-killed-receipt` | `6cfbb6ae` | none | `94848cee` | `test-runner-execution-v0.md`, `REQUIREMENTS.tsv`, `internal/testrunner/execute_unix_test.go` |

## Requirement renumbering

| Lane | Ticket | Lane ID | Integrated ID |
| --- | --- | --- | --- |
| Proof-tour destinations | V1-0519 | `PT-V0-008` | unchanged |
| Release smoke receipt JSON | V1-0588/0589 | `PUB-V0` amendment | unchanged |
| Fixture maintenance | V1-0662 | `AFP-V0-039` | unchanged |
| Tick ledger save failure | V1-0672 | `CAL-V0-192` | `CAL-V0-194` |
| Killed-run receipts | V1-1025 | `TRE-V0-040..041` | unchanged |

Batch F's V1-1021 holds `CAL-V0-192..193`. The renumbering is one commit (`d8613f3b`) on branch
`claude/batch-g-v0672-renumber`, made on the lane head before the merge. It rewrites every
`CAL-V0-192` and `CALV0192` occurrence in the files the lane changed: spec bullet, traceability
rows, failure-mode row, delivery status, authoritative inputs, `INDEX.json`, `README.md`, the lane
build-log entry, `loop.go` comments, test comments, and the test functions now named
`TestCALV0194_FailedTickSaveIsReportedAndRetried` and `TestCALV0194_UnsavedTickKeepsRunningWorkers`.
Main at `ecfff8e0` had no `CAL-V0-192`, so every rewritten occurrence was lane-owned.

## Conflict resolutions

- `docs/specs/corvint-tasks-agent-leases-v0.md`, `INDEX.json`, `README.md`: the delivery status
  keeps every clause in requirement order (V1-1016 `CAL-V0-191`, V1-1021 `CAL-V0-192..193`, V1-0672
  `CAL-V0-194`). The authoritative-inputs line takes the lane side, which prepends V1-0672 and
  V1-1016 to batch F's otherwise identical line. The amendment sections are kept in the same order:
  V1-1016, then V1-1021, then V1-0672.
- `internal/liveverify/affected/typescript/mocha_qualification_test.go`: batch F moved the fixture's
  `git` helper into a per-scenario loop. Batch F's structure is kept and V1-0662's
  `-c maintenance.auto=false -c gc.auto=0` safeguard is applied to the moved helper, so
  `TestLiveVerifyGitFixturesDisableDetachedMaintenance` covers it.
- `docs/specs/test-runner-execution-v0.md`: batch F's Jasmine section and V1-1025's killed-run
  section are both kept, in that order.
- `internal/testrunner/execute_unix_test.go`: batch F's `TestExecuteRefusesUnprovenRetirement` and
  V1-1025's `TestKilledRunReceiptRoundTrip` are both kept.
- `docs/specs/REQUIREMENTS.tsv`: generated; regenerated with `make -s spec-requirements` after
  staging.

## Semantic checks

- V1-1025 admits a negative document number only as an `exitCode` member equal to `-1`. Batch F's
  testrunner additions (`retireDetachedDescendants`, `retirement` with process IDs, the Jasmine and
  Boost.Test profiles) carry no other negative number, and batch F's executor still records `-1`
  for a phase without an exit status.
- V1-0672 changes only `internal/tasks/dispatch`, which batch F did not touch; it already composes
  with V1-1016's dispatcher changes on its own base.

## Verification

Darwin, Go 1.27.1 (`GOTOOLCHAIN=local`), shared loaded host, `GOMAXPROCS=3 go test -p 1 -count=1
-timeout 30m`: 36 packages `ok`, none failed: `internal/testrunner/...`, `internal/cemcandidate`,
`cmd/corvint-test-runner`, `cmd/corvint-cem-candidate`, `internal/tasks/dispatch`,
`internal/tasks/service`, `internal/tasks/cli`, `internal/tasks/store`, `internal/tasks/transaction`,
`internal/liveverify/...`, `internal/companionrelease`, `internal/specindex`. `go vet` on the same
packages and `gofmt -l` on the changed Go files are clean. The doc gates
(`spec-requirements-check`, `requirement-definitions-check`, `traceability-tests-check`,
`decision-numbers-check`, `line-citations-check`, `error-code-ownership-check`,
`unbounded-readers-check`, `use-case-receipts-check`, `diagnostic-coverage-check`) pass.
`script/proof-tour-test.sh` with a freshly built `interop/cem01-go` verifier and a copy of the
V1-0519 lane's paused synthetic tour prints 31 PASS lines and exits 0.

Independent review: Codex (`gpt-6-astra`, read-only) over `b9d8bc40..8e77f8d1` reported no P0 to P2
findings; it could not run Go tests in its sandbox.

NOT_RUN: `go test ./...`, `make gate`, the `interop/cem01-go` test suite, live dispatcher
qualification for `CAL-V0-191` and `CAL-V0-194`, the opt-in live Mocha, Jasmine, SwiftPM and
release-smoke qualifications, and owner acceptance of the proposed requirements (`PT-V0-008`,
`AFP-V0-039`, `CAL-V0-191`, `CAL-V0-194`, `TRE-V0-040..041`).
