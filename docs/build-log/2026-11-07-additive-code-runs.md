<!-- SPDX-License-Identifier: Apache-2.0 -->
# Additive narrowing follows every traversal class (V1-4983)

Lane `unbrd`, fix round 5 over `2026-11-07-binary-runners`, base
`4052b8540b5fff818a819db37b55371ea5428c2c`, starting at `ec11d63c3`. The fix-only review confirmed
the eight earlier findings fixed and raised one blocking P1. Requirements AFP-V0-3140 to
AFP-V0-3143 (`docs/specs/affected-plan-selection-v0.md`).

## Finding (held)

Round 4 added a fourth class to the dependency walk: `traverseWhere` follows a code-run edge from a
reached command to each unit whose code may run its binary (AFP-V0-2993). `reachedThrough` still
listed only the three import classes. It is the check that demotes a `NARROWED` additive record
whose package the walk reaches from another changed unit (ADN-V0-004). The review's case: a change
edits `quiet`, a command that locates nothing, and adds an unused exported `Fresh` to `harness`,
whose existing `Run` executes `quiet`. `harness` kept its `NARROWED` record, so `consumer`, which
calls `harness.Run` but names no new declaration, was excluded, although its tests observe the
changed command.

The regressions were written first and run at `ec11d63c3`:

- `TestAdditiveNarrowingFollowsCodeRuns_AFPV03140` (planner, real Go plugin and records), the
  review's case exactly, once with an `exec.Command("go", "run", ...)` literal and once with a
  `.corvint/test-binary-execs.json` entry. Both failed: the record stayed `NARROWED` and `consumer`
  was listed in `plan.goAdditiveExcluded`.
- `TestNarrowedRunnerFollowsTheChangedCommand_AFPV03143` (CI selector, `SelectStructural`). The
  declared case failed: `quiet`'s rule (a) reached no runner, because a declared run is no namer
  edge, and `harness`'s narrowed path dropped `consumer`. The module-path literal case passed, since
  the selector rewrites `example.com/fixture/quiet` to `/quiet`, which `namers` already follows.

## Approach

1. **One list of walk classes (AFP-V0-3142).** `traversalEdges` returns the four reverse edge maps
   indexed by named class constants (`classImport`, `classAdvisory`, `classRouteDispatch`,
   `classCodeRun`). `traverseWhere`, `reachedThrough` and `follows` all take their classes from it.
   A class added to the walk is in the check by construction. The guard test injects one edge of
   each listed class from a changed unit to the narrowed package and requires the fall-back. It
   also fails when the list's length changes, so the new class is named in the spec row.
2. **The code-run class demotes (AFP-V0-3140).** With the shared list, `reachedThrough` sees the
   code-run edge from `quiet` to `harness`. The record falls back with
   `REACHED_THROUGH_ANOTHER_CHANGED_UNIT: go:example.test/m/quiet`, and the walk is rerun without
   it, so `consumer` is selected.
3. **Precision is kept (AFP-V0-3141).** A test-only run is no followed edge: `execUsersOf` adds the
   runner one level past the walk. When only `harness`'s tests run `quiet`, the record stays
   `NARROWED` and `consumer` stays out. The same holds when `harness`'s additive change is the only
   change.
4. **The CI selector walks code runners (AFP-V0-3143).** The selector's rules (a) and (b) (`reach`)
   now follow `codeRunners`, including declared runs, as the planner's build reach does. Its
   `attributeNarrowed` starts from the narrowed package's code runners too, because a record prunes
   import edges only, as `follows` does. The plugin never narrows a `main` package (ADN-V0-004), so
   that second change matters only for a stale or forged receipt. The selector does not trust a
   receipt to prove it.

A relative climbing literal (`../quiet` in `lib/harness`) was considered as a third selector case
and dropped. In library code such a literal resolves against the process's working directory, so
the plugin already makes the holder and its importers rule (d) units. The plan's
`UNBOUNDED_READER` selections carry `consumer` whatever narrowing applies.

Fault injection, one mechanism at a time, each reverted afterwards:

| Mechanism removed | Regressions that fail |
|---|---|
| code-run class in `reachedThrough` (`classes[:classCodeRun]`) | `…FollowsCodeRuns_AFPV03140` (both cases), `…CoversEveryTraversalClass_AFPV03142` (class 3) |
| `codeRunners` in the selector's `reach` | `…FollowsTheChangedCommand_AFPV03143` (declared case) |
| code-runner start of `attributeNarrowed` | `…NarrowedCommandKeepsItsRunners_AFPV03143` |

The controls, `…KeepsUnrunRecords_AFPV03141` and `…NarrowedRunnerKeepsItsNarrowing_AFPV03143`,
pass before and after.

## Repository impact

Measured on 2026-11-07. Both binaries were built from this branch's tree: "before" with
`go build -overlay` mapping the four changed sources to their `ec11d63c3` bytes, "after" with
this round's code.

- **Path-set plans.** `corvint affected --worktree --paths-from FILE` was run over the last 12
  first-parent merges of `origin/main`, the three canonical single-path changes
  (`.github/workflows/ci.yml`, `script/heavy.sh`, `docs/TICKET-PRIORITY.md`) and three Go edits
  (`internal/liveverify/affected/select.go`, `internal/gateselect/select.go`,
  `cmd/corvint/main.go`). All 18 plans were identical before and after: same units, Go packages
  and unknowns. `gate-affected-select` over each after-receipt printed the same verdict with both
  selector builds.
- **Base-relative plans, where additive records exist.** Each of the 12 merges was checked out in a
  temporary detached worktree, removed afterwards, and run with `--base M^1`. The 12 plans hold 26
  additive records, 2 of them `NARROWED` (`13342d85f`, `b75053a5b`). Selections, record states and
  `goAdditiveExcluded` were identical before and after.
- **This branch.** The plan against the base is unchanged: 440 units, 177 Go packages, 72 unknowns.
  It holds four `FALLBACK` records.

The fix therefore narrows nothing that the measured history narrowed. It only bites where a
narrowed package's code runs a command the same change reaches.

## Limits

- The selector's rules (a) and (b) still leave out a command's test-only runners. The planner
  selects them one edge past a rebuilt command (`execUsersOf`), and CI unions the plan's packages
  (AFP-V0-3143 non-goal).
- No ticket was opened for this finding. It completes V1-4983's execution-edge work, and its rows
  cite that ticket.
