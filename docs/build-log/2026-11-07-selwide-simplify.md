# Selwide simplification: Go's `./...` plus walked hidden packages

Base: `e19823d91`. Tickets: V1-5018, V1-4941, V1-4710. Requirements: AFP-V0-9012
to AFP-V0-9014 in `docs/specs/affected-plan-selection-v0.md` (IDs unchanged).
This entry supersedes the FULL-universe design in the earlier
`2026-11-07-selwide-*` entries (shared discovery, full-universe, full-context,
full-completeness, self-review execution and runtime build-context repairs);
their V1-4941 and V1-4710 decisions and reproductions still stand.

## Decision

After five review rounds the explicit FULL universe (graph-driven package
directories, build-context omission rules, symlink certification, declared
output exclusions and receipt-supplied universes) kept producing new P1s. The
orchestrator required a redesign: FULL keeps Go's own `./...` as the authority
and appends only the package directories it skips, found by a narrow walk.

- `tools/corvint-pr-tests/universe.go`: `hiddenPackages` walks the source root
  (skipping `.git`, `testdata`, `vendor`) and maps Go files to `./dir`
  arguments below a `.` or `_` element, outside nested modules. FULL argv is
  `./...` followed by those directories. Sharded placement, reruns and shadow
  observations list the same argv with `go list`.
- Symlinked sources, build constraints and packages Go sees when it lists are
  Go's decision, not ours. Unsharded FULL runs the existing clean-source check
  after the walk, so a hidden package added after it refuses with source drift.
- Container FULL receipts are checked against the hidden packages of the
  target commit's `git ls-tree`, never against the receipt.
- CI: placement keeps `go list ./...` in both `narrow.sh` (unchanged) and the
  shard job, so every shard partitions the same universe. Shard 0 always runs
  the hidden packages from `corvint-pr-tests --mode packages` whole, outside
  placement and narrowing; a discovery failure fails the shard.

Removed from the branch: the explicit universe in `internal/sessiontest`, the
`walk.go`/`source.go` symlink and source-kind certification, the graph package
directory export and its tests, the runtime exclusion allowlist and build
context checks, and `tools/ci-forward-row/hidden_universe_test.go`.

## Review findings

- `testGoSlices` arity in `cmd/corvint/core_freeze_digest_selection_test.go`:
  fixed; the test is renamed `TestCoreFreezeDigestSelection_AFPV09014` so the
  `CoreFreeze` run selects it.
- `long-lines-check` on `.corvint/change.cem.json`: the bound unsealed CEM;
  the seal moves it. No allow or exclude entry was added.
- Self-review P1: the earlier CI design placed with `go list ./...` plus all
  hidden packages in the shard job but only the selected ones in the pinned
  `narrow.sh`. The cost-greedy partition moves packages when the universe
  changes, so a selected package could be recorded for one shard and missing
  from the partition the other shard ran. Replaced by the shard-0 run above;
  `narrow.sh` is back to the base.
- `TestHiddenUnrepresentableNames_AZU_V0_005` failed on darwin because APFS
  refuses invalid UTF-8 names; the disk cases now run only where the
  filesystem stores them.

## Evidence

New regressions: `TestFullExecutionHiddenAndSymlinked_AFPV09012` (failing
tests in a hidden package, a symlink-only package and a hidden symlink-only
package each fail FULL with their package outcome recorded) and
`TestFullSourceDriftHidden_AFPV09012` (a hidden package created just before
the post-walk status read refuses with source drift). Whole-package runs via
`script/heavy.sh`: `internal/liveverify/affected`, `.../affected/golang`,
`tools/corvint-pr-tests`, `tools/ci-forward-row`, and `cmd/corvint` with
`-run 'CoreFreeze|CoreVerbs'` all pass.

## Independent review (Codex)

- Accepted P1: discovery listed packages without `-race` while execution runs
  `go test -race`, so a package whose only test is `//go:build race` was never
  placed under sharded FULL. `goUniverse` and `--mode packages` now list with
  the build flags of `testArgs` (`-race`); shard 0 of `ci.yml` runs every
  discovered package that placement's `./...` lacks, so race-only packages run
  there too. `TestFullShardedRaceOnly_AFPV09012` fails with the flag removed
  (fault injection) and passes with it.
- Rejected: packages under `testdata/` are fixtures Go never runs; still pruned.

## Limits

- Over-inclusion fails visibly, never silently: a hidden directory whose Go
  files are all build-constrained out, or declared hidden output holding Go
  files, becomes a FULL argument that Go rejects.
- A gitignored hidden package created mid-run is not seen by `git status`.
- Shard 0 runs the hidden packages even when narrowing would skip them (four
  small packages here); macOS and other runners keep their own `./...` lists.
