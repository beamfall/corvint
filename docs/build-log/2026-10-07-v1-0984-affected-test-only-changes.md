# 2026-10-07: V1-0984 a Go test-only change no longer reaches its package's importers

## Intent

Ticket V1-0984 (suspected bug, no GitHub issue): `corvint affected --base <sha>` treated a change
touching only `_test.go` files of Go package P as a change to P and selected every importing unit.
A Go test file compiles only into P's own test binary and cannot be imported, so the correct
selection is P's tests plus whatever an explicit read rule (path literal, unbounded reader,
declared read scope) requires.

## Reproduction (before)

Base `0b5096ca0896351a584bca271812d9e1dd34f06c`, one scratch commit adding only
`internal/contextindex/zz_v1_0984_scratch_test.go` (an empty `Test` function in
`package contextindex`), clean worktree, `corvint --root . affected --base 0b5096ca…`:

| | selected | excluded | `DEPENDENCY_PATH` | `UNBOUNDED_READER` | `PATH_LITERAL_READER` | `DECLARED_READ_SCOPE` | `DIRECT_TEST_CHANGE` | scope |
|---|---|---|---|---|---|---|---|---|
| before | 99 | 278 | 72 | 19 | 4 | 3 | 1 | `UNKNOWN` (Go build-constraint and nested-module frontiers) |
| after | 43 | 334 | 0 | 30 | 9 | 3 | 1 | `UNKNOWN` (same two frontiers) |

The 72 `DEPENDENCY_PATH` selections were the reverse-import closure of `internal/contextindex`
(for example `cmd/corvint-readiness-record` via `docmaintain`, `companionrelease`,
`releasecandidate`). Reproduced: the suspected bug is confirmed.

After the fix, units that were previously reached by dependency and also read unbounded or name
the path are still selected, now under their reader witness (19 to 30 and 4 to 9). Those read
rules are unchanged and stay conservative; the frontier unknowns and `UNKNOWN` scope are unchanged.

## Change

`internal/liveverify/affected/select.go`: `reach` computes `testOnlyGoUnits`, the indexed Go units
every dirty path of which is one of their own declared `_test.go` files and which no structural
rule (AFP-V0-012 rule (a) unindexed Go path, rule (b) traversed data path) also changes. Those
units are removed from the traversal roots, so neither `traverse` (importers) nor `testUsersOf`
(units whose tests import them) expands from them, and are then restored with their
`DIRECT_TEST_CHANGE` witness. A unit reached from another dirty unit is still expanded normally.
`seed` now prefers a unit's dirty source path over a test path as its witness, so a mixed change
that propagates names a path that can reach the dependents. Non-Go plugins are unchanged.

Spec: `AFP-V0-035` in `docs/specs/affected-plan-v0.md`, proposed (V1-0984).

## Evidence

- `TestGoTestOnlyChangeSelectsItsPackageButNotItsImporters_V1_0984` fails on the pre-fix
  `select.go` (test-only case selects `go:mid` and `go:testuser` by `DEPENDENCY_PATH`) and passes
  after.
- `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/liveverify/affected/...` (go1.27.1):
  every language subpackage passes; in the root package every functional test passes and the two
  wall-clock budget tests (`TestIncrementalSelectionMeetsTheLiveBudget`,
  `TestSelectionOnTheLiveDirtyWorktree`, 100 ms) fail on a host at load average 50 to 62 on 12
  cores. Interleaved A/B over three rounds with test binaries built from the base and the fixed
  `select.go`: incremental p95 base 199 / 139 / 221 ms, fix 136 / 184 / 190 ms; both arms fail
  the same budgets, so the failure is host load, not this change. Run alone earlier, the fixed
  incremental test passed (p95 94.7 ms). The change adds one pass over the dirty set per plan.
- No `cmd/corvint/testdata` golden carries a `DIRECT_TEST_CHANGE` witness, so the source-first
  witness preference cannot change one; `cmd/corvint` tests were not run.

## Not done

- `tools/gate-affected-select` (the `make gate-affected` fast tier, AFP-V0-011/012) has its own
  selector and was not inspected or changed.
- An unindexed or deleted `_test.go` path keeps rule (a): it is a direct source change of the
  package in its directory and stays `UNINDEXED_SOURCE_PATH` unknown scope.

Rollback: revert the `select.go` change and the test; drop `AFP-V0-035`.
