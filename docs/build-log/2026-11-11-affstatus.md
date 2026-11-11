# Affstatus: base change status narrows name-listing shell readers (V1-5193, V1-5194)

Owner intent: the affstatus lane charter and the committed proposal
`docs/specs/openspec/changes/affected-change-status/` (intent `proposed`; only the owner accepts
it). Requirements AFP-V0-9960 to AFP-V0-9964, all proposed, delivery experimental. Base
`origin/main` `4c3c203336a6cc4a9772af39b1a771f34236138e`.

## What changed

- AFP-V0-9960. `affected.ChangeStatuses` classifies each changed path against the base commit:
  `git --literal-pathspecs ls-tree -z --full-tree BASE -- PATHS` for the base side (batches of
  256) and the parent directory's listing for the head side. Only an exact name match counts on
  either side, so a case-folding or normalizing file system reads a rename as deleted plus
  added, never modified. `modified` needs a regular file (100644 or 100755) at the base and a
  regular file now; anything else present at both sides is `type-changed`; an unreadable side,
  a path present at neither side, a trailing-slash path and an unreadable base are `unknown`.
  `Graph.WithChangeStatus` carries the result as a plan input (never cached); `Select(graph,
  dirty)` keeps its signature and reads every path as `unknown`. `corvint affected --base`
  (and the review snapshot plan, which shares `planAffectedGraph`) passes it; `--path` and the
  worktree-only form do not.
- AFP-V0-9961. `Unit.NameListings` is the subset of `PathTokens` held only by name-listing
  reads. Every other consumer of `PathTokens` sees the tokens it saw before. The shell plugin
  admits the closed list from the design, and only when every later stage of the command's
  pipeline is a standard-input filter (`wc`, `sort`, `head`, `tail`, non-recursive `grep`, each
  with short flags only) and the file passes a unit gate (below).
- AFP-V0-9962. The path-literal reader rule matches a `modified` path against the unit's tokens
  less its `NameListings`; every other status uses all tokens, so `unknown` plans exactly as
  before.
- AFP-V0-9963. `Unit.UnboundedNamesOnly` marks a `shell:tree-walk` test whose every walking
  command is an admitted name-only form. Rule (d) selects it only for a dirty path whose status
  can change a listing, witnessed by the smallest such path. A unit a root locator also reaches
  is never narrowed.
- AFP-V0-9964. Both fields are in the graph digest (omitted when empty, so graphs without them
  keep their digest) and the cache entry; the cache profile is `corvint-affected-graph-cache/4`.

## Unit gate (a fail-closed tightening of AFP-V0-9961)

While writing the tests I found that the closed list alone drops recall that today's plan has.
`ls script | wc -l` beside `for f in a b; do cat "script/$f"; done` is selected on an edit of
`script/a` today only because of the listing's `/script/` token: the computed read leaves the
token `script/`, which names only a file called `script` (V1-0290). Likewise `cd script && cat
a.sh`, a helper run, or `find . -name '*.md' | wc -l` beside `git ls-files '*.md' | ... cat
"$f"` (selected today only by the walker's unbounded reads). So a file records name-listing
reads only when every command outside an admitted pipeline is an output builtin, an existence or
kind test (`-e -f -d -h -L` and value comparisons; not `-s`, `-nt`, `-x` or other attribute
tests), or an operand reader (`cat`, `head`, `tail`, `wc`, `sort`, `cmp`, `diff` without `-r`,
checksums, non-recursive `grep` without `-e`/`-f`) whose operands and input redirections are
literal path-shaped words, and the file assigns no `PATH`. A sourced file, a function call, a
directory change, `exec`, `run`, `env`, a program or a computed operand anywhere keeps every
listing a content read. The sentence is added to AFP-V0-9961 in the spec and the delta.

Depth never narrows: an added `script/sub/new.sh` creates the `sub` entry `ls script` lists, so
a name-listing read selects on every non-`modified` path below its directory.

## Tests

- `internal/liveverify/affected`: `TestClassifyChange_AFPV09960`, `TestChangeStatuses_AFPV09960`
  (real Git: added, committed addition, deleted, renamed, file to directory, changed symlink,
  modified, untracked, absent at both sides, no base, unreadable base),
  `TestGraphCacheNameListings_AFPV09964` (round trip, digest, plans under both statuses, a `/3`
  entry rebuilt).
- `internal/liveverify/affected/shell`: `TestNameListingForms_AFPV09961` (15 admitted rows, 38
  refused rows: `ls -l`, `--all`, `find -newer`, `-exec`, `-size`, `-o`, `-L`, a computed
  `-name`, `tree -s`, `du`, `grep -r`, a computed operand, `xargs`, `wc --files0-from`, a loop, a
  `while read`, a group, a captured value, a redirection, `tee`, `run`, `env`, a function, a
  shadowed filter, `PATH`, a sourced file, `exec`, a glob, and the gate rows),
  `TestNameListingContentDecides_AFPV09961`, `TestNameListingSelection_AFPV09962`,
  `TestNameOnlyTreeWalker_AFPV09963`; the AFP-V0-9133 and 9134 tests pass unchanged.
- `cmd/corvint`: `TestAffectedChangeStatusCLI_AFPV09960` (edit with `--base`: lister and walker
  not selected, `ls -l` selected; without `--base`: all selected; untracked addition: all
  selected).

Existing assertion changed: `TestTestEmbedGraphCacheRejectsOldProfile_AFPV03352` checked the
literal `corvint-affected-graph-cache/3 ` header before injecting `/1`. AFP-V0-9964 requires the
profile to change, so the literal is now `affected.GraphCacheProfile`; the test's claim (an old
profile is rebuilt) is unchanged. AFP-V0-3303 and AFP-V0-3352 still name `/3` as the profile
their facts need; `/4` carries those facts too, and their text was left for the owner.

## Mutation evidence

Each mutant was applied to the source, its test run once through `script/heavy.sh env
GOTOOLCHAIN=local go test -count=1 -run '^NAME$' PKG`, and the source restored (driver:
`/tmp/affstatus-mut/run.py`, not committed). Earlier mutants M11 (captured output admitted), M13
(function shadow ignored) and M14 (sourcing test admitted) targeted checks that the unit gate
made redundant; the checks were removed rather than kept as dead code.

| Mutant | Change | Killing test | Result |
| --- | --- | --- | --- |
| M1 | `ListingCanChange`: `added` treated as `modified` | `TestClassifyChange_AFPV09960`, `TestNameListingSelection_AFPV09962` | killed |
| M2 | `ListingCanChange`: `unknown` treated as `modified` | `TestNameListingSelection_AFPV09962` | killed |
| M3 | `ListingCanChange` always false | `TestNameOnlyTreeWalker_AFPV09963` | killed |
| M4 | `ListingCanChange` always true (harmless for recall) | `TestNameListingSelection_AFPV09962` | killed |
| M5 | an absent status defaults to `modified` | `TestNameListingSelection_AFPV09962` | killed |
| M6 | one regular side is `modified` | `TestClassifyChange_AFPV09960` | killed |
| M7 | `ls -l` admitted | `TestNameListingForms_AFPV09961` | killed |
| M8 | `find -newer` admitted | `TestNameListingForms_AFPV09961` | killed |
| M9 | any `find` primary admitted (`-exec`, `-size`, `-o`, `-L`) | `TestNameListingForms_AFPV09961` | killed |
| M10 | `xargs` accepted as a filter | `TestNameListingForms_AFPV09961` | killed |
| M12 | a non-control prefix (`run`, `env`) skipped | `TestNameListingForms_AFPV09961` | killed |
| M15 | a non-admitted walker not marked a content walk | `TestNameOnlyTreeWalker_AFPV09963` | survived: equivalent, the unit gate already refuses every walker that is not admitted |
| M16 | reader rule ignores status | `TestNameListingSelection_AFPV09962` | killed |
| M17 | reader rule drops listings for every status | `TestNameListingSelection_AFPV09962` | killed |
| M18 | rule (d) ignores status for name-only walkers | `TestNameOnlyTreeWalker_AFPV09963` | killed |
| M19 | a content read does not decide | `TestNameListingContentDecides_AFPV09961` | killed |
| M20 | cache profile left at `/3` | `TestGraphCacheNameListings_AFPV09964` | killed |
| M21 | cache drops `NameListings` | `TestGraphCacheNameListings_AFPV09964` | killed |
| M22 | a non-bare operand admitted | `TestNameListingForms_AFPV09961` | killed |
| M23 | a path absent now classified `modified` | `TestChangeStatuses_AFPV09960` | killed |
| M24 | unit gate off | `TestNameListingForms_AFPV09961`, `TestNameOnlyTreeWalker_AFPV09963` | killed |
| M25 | `test -s` admitted | `TestNameListingForms_AFPV09961` | killed |
| M26 | a computed operand admitted by the gate | `TestNameListingForms_AFPV09961` | killed |
| M27 | `cd` admitted by the gate | `TestNameListingForms_AFPV09961` | killed |
| M28 | `corvint affected --base` passes no status | `TestAffectedChangeStatusCLI_AFPV09960` | killed |
| M29 | a `PATH` assignment ignored | `TestNameListingForms_AFPV09961` | killed |
| M30 | a pipeline stage need not be a filter | `TestNameListingForms_AFPV09961` | killed |
| M31 | a helper unit admitted | `TestNameListingForms_AFPV09961` | killed |

## Measurements

All runs on this WSL2 host, through `script/heavy.sh`, foreground unless stated.

| Measurement | Before (base `4c3c20333`) | After (this lane) | Exit |
| --- | --- | --- | --- |
| `selection-baseline` narrow merged / misses | NOT_RUN (see below); recorded floor 33, misses 0 | 36/43 (rate 0.837), misses 0, `OK floor narrow>=33 misses<=0` | 0 |
| `selection-baseline` other summary | - | median cost 0.752, whole 6966, sliced 2, test-misses 0; FALLBACK rows: `pr-39`, `ci-af9d23f80`; `MOVED pr-14 ci FULL -> NARROW` against the recorded verdict | 0 |
| pr-39 replay | `plan=UNKNOWN verdict=FALLBACK ci=FULL`, 589 dirty, 546 selected, 133 unknown | identical: `FALLBACK`, 589 / 546 / 133, same unknown kinds | 0 / 0 |
| Shell tests in this repository with a name-listing read or a name-only walk | - | 0 of 250 shell units (0 tests even use an admitted form on a repository directory; their listings name temporary directories) | - |

Commands:

- After: `script/heavy.sh make selection-baseline-check
  SELECTION_BASELINE_OUT=/tmp/affstatus-baseline-after.json
  BASE=4c3c203336a6cc4a9772af39b1a771f34236138e` (run=1187.0 s, rc=0). It passed the tool's
  600 s foreground limit and was moved to the background by the agent harness; it was not
  started twice. Its planner was built from the worktree whose Go sources the commit
  `Narrow name-listing shell readers by base change status` holds (results `planner`:
  `commit 9086507cb..., modified`).
- Before, full corpus: NOT_RUN. A second 20-minute replay with the base binaries cannot finish
  inside the 600 s foreground limit, and the charter forbids background jobs. The recorded floor
  (`benchmarks/selection-baseline/changes.json`) is 33 narrowed, 0 misses; the last recorded
  replay (`docs/build-log/2026-11-09-selbase-guard.md`, an older base) was 35/43.
- pr-39: a change set holding only `pr-39` (`/tmp/affstatus-pr39/changes.json`, filtered from
  the corpus), binaries built `-trimpath -buildvcs=false` from `git archive 4c3c20333` (before)
  and from this tree (after), then `script/heavy.sh /tmp/affstatus-pr39/after/selection-baseline
  --root . --changes /tmp/affstatus-pr39/changes.json --corvint SIDE/corvint --selector
  SIDE/gate-affected-select --out SIDE.json` (102.3 s and 83.4 s, rc=0 each; the tool's
  one-change floor line `FAIL narrowed merged changes 0 below the floor 33` is expected for a
  one-change set and `--check` was not passed).

Reading: the change narrows nothing in this repository's own plans, so the baseline cannot show
a precision gain here, and it shows no recall loss (misses 0, test-misses 0). The pr-39
`FALLBACK` comes from an `UNKNOWN` plan of a 589-path change with 133 unknown entries, none of
them a name-listing read; it is unchanged by this lane, so the proposal's "no FALLBACK row"
criterion is NOT_MET.

## Rollback

Revert the lane commits. Without `NameListings` and `UnboundedNamesOnly` every listing is a
content read again, which selects more, never less; the profile change makes an older binary
rebuild its cache.
