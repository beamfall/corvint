# Decision 0590: per-test line coverage may justify skipping a test

- Date: 2026-11-03
- Status: accepted (owner, 2026-11-03: "Yes, amend LPCV")
- Authority: `docs/specs/live-proof-carrying-verification-v0.md` (`LPCV-V0-008`, `LPCV-V0-027`,
  new `LPCV-V0-950..963`); decision 0047 (LPCV acceptance); decision 0003 (LPCV as the home of the
  Wallaby-style continuous-verification plane).

## Context

The owner's goal (2026-11-03): when a line changes, only the tests that line can affect run,
including E2E tests, for every supported language and runner, fast enough for edit-time reruns and
CI test picking. Today's selection is package and path level. On the V1-4361 diff
(`ea09e3db5..2f4447b3c`) CI selected 115 packages.

`LPCV-V0-008` and `LPCV-V0-027` forbade using coverage as an absence certificate, so no accepted
requirement allowed a coverage record to justify skipping a test. `AFU-V1-021` already allowed a
path-level coverage exclusion for app-flow E2E with staleness conditions; it was the only precedent.

## Decision

1. `LPCV-V0-008` and `LPCV-V0-027` gain one narrow exception: a fresh per-test line coverage record
   (`LPCV-V0-950..952`) may justify excluding a test whose recorded execution does not reach a
   changed line. Nothing else changes about what coverage proves.
2. Freshness is per file by Git blob OID; stale, incomplete or missing records widen to the static
   plan. Declaration-level, new-file, helper, fixture and configuration edits always use the static
   plan (`LPCV-V0-953`).
3. E2E records include every instrumented process the test starts (`LPCV-V0-954`); one collector per
   runner family (`LPCV-V0-955`); union over runs (`LPCV-V0-956`); full default-branch runs are the
   miss detector feeding the `LPCV-V0-020` kill switch (`LPCV-V0-957`).
4. The selector is tested by fault injection, not only by comparison with full runs: every test a
   one-line mutant fails must be in the selected set before exclusion is enabled for a runner
   family (`LPCV-V0-963`, at the owner's direction that the selection be tested correctly).
5. The index is derived, immutable, rebuildable state with a p95 ≤ 100 ms selection budget
   (`LPCV-V0-958`). CI and the watch loop consume the same rule (`LPCV-V0-959`, `LPCV-V0-960`).

## Prototype evidence (Go, not a delivery)

A prototype (`go test -c` with `-cover -coverpkg=<module>/...`, one process per top-level test with
`GOCOVERDIR`, `go tool covdata textfmt`, and a gob index) was run on the WSL lane host:

- 8 packages, 382 tests: 70 s to build the map. The V1-4361 diff selected 5 tests in 2 packages,
  plus 2 packages as static fallback for declaration-level test-file edits, against 115 packages
  selected by CI.
- `cmd/corvint`, 1,501 tests: 22 s build plus 303 s run at 12-way parallelism.
  26 of the 1,501 exited nonzero when run alone, the isolation divergence `LPCV-V0-962` names.
- Over all 9 packages (1,883 tests) the V1-4361 diff selected 9 tests in 3 packages plus 2 static
  packages. The gob index is 10 MB, loads in 62 ms, and selection took 154 ms.
- On the 8-package map the index loaded in 30 ms (804 KB gob versus 6.8 s from raw text); selection
  took 87-225 ms, dominated by `git` subprocesses.

Learned gaps the requirements already cover: a test new on the head side must select itself, and
declaration edits need symbol-level narrowing before they can beat the static plan.

## Consequences

- Implementation is tracked as tickets per collector family, the index and selector, CI
  integration, the watch loop and the miss detector.
- Rollback: the `LPCV-V0-020` kill switch disables coverage exclusion; selection returns to the
  static plan, and deleting the index loses only performance.
