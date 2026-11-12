# Walk selection: deleted shell script admission and walk facts (V1-5191, V1-5193, V1-5194)

Lane walksel. Requirements AFP-V0-9990 to AFP-V0-9995 in
`docs/specs/affected-plan-selection-v0.md`, all proposed, delivery experimental. The branch
holds the affgate head `151e466a4` (AFP-V0-9960 to AFP-V0-9964 and the AFP-V0-9961 closed
grammar) and the reviewed shellwalk lane `1ffdb8f83` (AFP-V0-9970 to AFP-V0-9977), merged in
`afa11627d` with both behaviours kept: the walk loop records a content walk for each wrapper run,
and `pathTokens` takes both the name-listing admission and the AFP-V0-9977 utilities flag.

## Decision

The salvage branch `refs/salvage/del-shell-admit` admitted a deleted script only when no unit was
opaque, where opaque meant a computed command, `eval`, inline code and similar. Its own census
found 137 of 288 shell units opaque, so the admission never fired on this repository. pr-39 stayed
FALLBACK. Its requirement IDs also collide with IDs now on main. Its relation code
(`walks.go`, `walks_code.go`, about 2,300 lines) duplicated what main has since gained through
AFP-V0-9133, AFP-V0-9134 and AFP-V0-9970 to AFP-V0-9977, so it was not ported. Ported by intent:
the admission conditions (`deletedShellScript`), the extension rule, the walk-directory matcher
(`walksPath`), the lone-directory-token observer and the conformance-test adjustment.

This lane makes the gate relative to the deleted path and selects instead of refusing:

- The shell plugin records walk facts per unit (AFP-V0-9990): literal walk directories, the first
  walk opacity reason from a closed list, and computed run names. The census at the code commit
  is 252 units, 168 of them helpers. 176 hold an opacity reason, 130 of them helpers. Counts are
  by first reason: computed-command 55, tree-walk 53, interpreter 19, toolchain 15, awk-command
  11, computed-walk-operand 9, forwarded-command 7, computed-glob 5, eval 1,
  working-directory-listing 1.
- A deleted shell script is admitted on structure alone (AFP-V0-9992).
- Every unit that may have observed it is selected (AFP-V0-9993, AFP-V0-9994). A test-less
  observer is followed to the units that run it, as a test-less literal reader is, so its
  dependents frontier (`shell:foreign-invocation-unmodelled`) bears. That frontier is a
  FrontierSuite entry, so `gate-affected-select` adds the shell suite and keeps the Go shards
  narrowed. A FULL fallback would run everything; this runs the shell observers, their users and
  the shell suite.
- A test's slash-spelled walker operand (`ls script/`, `find ./script`) names a `/dir/` fragment
  (AFP-V0-9991). Before this, only a bare operand did (AFP-V0-9133), and a `script/` token names
  only a file called `script` (V1-0290).

Rejected: refusing the admission for an opaque helper, as the ticket text proposed. On this
repository that is the salvage's outcome: 130 opaque helpers make every deleted-script plan FULL.

V1-5194 needed no new selection code. AFP-V0-9134 already selects every root walker on every
dirty path, and AFP-V0-9963 with the AFP-V0-9960 status already narrows name-only walkers on an
edit. `TestUnboundedWalkerAcceptance_V15194` runs the ticket's three inputs with statuses.
Narrowing content searchers by their literal pattern was not built (NOT_PRODUCED). It changes
only precision, and Select has paths and statuses but no blob content to match a pattern against.

## Review notes and corrections during the build

- `[` and `test` glob their unquoted operands; `[[` does not. The first version skipped all three.
  Mutant S13 showed the skip was redundant, and `[ "$f" = *.sh ]` is a working-directory glob. Now
  only `[[` is skipped, with rows for both (`conditional pattern is no glob`, `test operand globs`).
- The foreign-callers completeness check (TSP-V0-005) treats any candidate file whose text quotes
  a constructed `script/...$` path as naming every script below `script/`. A doc comment in
  `deletedscripts.go` quoting `cat "script/$f"` made the declaration invalid. Three liverepo tests
  then failed, and the reported detail named an unrelated stale mention. The example now reads
  `cat "data/$f"`. Friction item in the handback.
- An interpreter with a planned plugin (python, ruby, node family) run as `INTERPRETER FILE` is
  that file's unit. perl, php, lua, Rscript, tclsh, wish, pwsh and osascript have no plugin, so
  every run of them is opaque.
- Observer selection is terminal (`terminalWitness`), as for a literal reader. Mutant M20 (not
  terminal) is equivalent in practice. An admitted deletion is always an unowned dirty path, so
  every plugin already takes part through it (PARTICIPATING_PLUGIN), and no plan can tell the
  two apart.

## Tests

- `internal/liveverify/affected/shell`:
  - `TestShellWalkFacts_AFPV09990`: 38 rows, one per walker, opacity reason and control.
  - `TestShellWalkFactsTempRoot_AFPV09990`.
  - `TestShellWalkOperandFragments_AFPV09991`.
  - `TestBareOperandListerAcceptance_V15193`: AC1 to AC3 with statuses.
  - `TestUnboundedWalkerAcceptance_V15194`.
  - `TestDeletedScriptObserversEndToEnd_AFPV09994`.
- `internal/liveverify/affected`:
  - `TestDeletedShellScriptAdmission_AFPV09992`: the admitted deletion, plus the on-disk,
    extensionless, sparse, unobserved and second-claimant controls.
  - `TestDeletedScriptObservers_AFPV09994`: one unit per relation branch and its control, the
    read-scope exclusion, the carried helper and its bearing frontier, and `Readers`.
  - `TestDeletedScriptRelationHelpers_AFPV09993`.
  - `TestGraphCacheWalkFacts_AFPV09995`, `TestGraphDigestWalkFacts_AFPV09995`.
- Existing assertions changed:
  - `TestSeamWidensRatherThanNarrowingOnUnknownInput`: the shell case writes its new file to
    disk, as the Go case does, because an absent shell path is now a deletion.
  - `TestGraphCacheNameListings_AFPV09964`: checks the current profile `/5`. Its claim, that a
    `/3` entry is rebuilt, is unchanged.

## Mutation evidence

Driver: `/tmp/walksel/mut/run.py`, not committed. Each mutant is one exact string replacement.
It is run once through `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -run PATTERN PKG`,
with `AFPV0999` for `affected` and `AFPV0999|V1519` for `shell`. The source is restored in a
`finally`. Result: 39 mutants, 38 killed, 1 equivalent.

| Mutant | Change | Result |
| --- | --- | --- |
| M1 | admission dropped from `admittedDeletion` | killed: Admission, Observers |
| M2 | sparse checkout ignored | killed: Admission |
| M3 | suffix rule ignored | killed: Admission |
| M4 | first claimant `shell` suffices | killed: Admission |
| M5 | unobserved directory ignored | killed: Admission |
| M6 | a present file admitted | killed: Admission |
| M7 | observer pass off | killed: Observers |
| M8 to M12 | each relation branch off (opacity, walk directories, run names, lone token, token bound) | killed: Observers |
| M13 | read-scope exclusion ignored | killed: Observers |
| M14 | test-less observer not followed | killed: Observers |
| M15 | `Readers` lacks the pass | killed: Observers |
| M16 | digest omits `WalkOpaque` | killed: GraphDigest |
| M17 | cache profile left at `/4` | killed: GraphCacheWalkFacts |
| M18 | `walksPath` matches the file name | killed: RelationHelpers |
| M19 | run-name prefix ignored | killed: Observers, RelationHelpers |
| M20 | observer not a terminal selection | survived: equivalent (above) |
| S1 to S6 | forwarded command, computed command, eval, xargs, interpreter, awk off | killed: WalkFacts |
| S7 | temp-root exemption off | killed: WalkFactsTempRoot |
| S8 to S12 | climbing, tree walk, `ls` with no operand, computed glob, helper toolchain off | killed: WalkFacts |
| S13 | `[[` operands globbed | killed: WalkFacts |
| S14 | glob directories off | killed: WalkFacts |
| S15 | slash fragments off | killed: WalkOperandFragments |
| S16 | run names off | killed: WalkFacts, EndToEnd |
| S17 | perl treated as observed | killed: WalkFacts |
| S18 | working-directory glob off | killed: WalkFacts |
| S19 | `[` operands not globbed | killed: WalkFacts |

## Measurements

The binaries were built `-trimpath -buildvcs=false` on this Unraid host from two commits.
Before is the merge `afa11627d`. After is the code commit, `e88626872`. All runs went through
`script/heavy.sh`. The corpus was split into ten 6-change chunk files, `/tmp/walksel/chunkNN.json`,
because one replay takes about 25 minutes, past the 600 s foreground limit. Each chunk ran with
`selection-baseline --root . --changes CHUNK --corvint SIDE/corvint --selector
SIDE/gate-affected-select --out SIDE-chunkNN.json`, 115 s to 303 s each, rc 0. A chunk prints
its own floor `FAIL` line, which is expected below the corpus size. The totals below are the sums
of the ten chunks.

| Measurement | Before (`afa11627d`) | After (`e88626872`) |
| --- | --- | --- |
| merged changes narrowed | 36/43 | 37/43 (floor 33) |
| misses / test-misses | 0 / 0 | 0 / 0 |
| FALLBACK rows | `pr-39`, `ci-af9d23f80` | none |
| pr-39 | UNKNOWN, FALLBACK, ci FULL; 589 dirty, 549 selected, 133 unknown; reason `UNINDEXED_SOURCE_PATH` | UNKNOWN, run, ci NARROW; 589 dirty, 565 selected, 132 unknown; 307 of 406 packages, cost 0.923 |
| ci-af9d23f80 | UNKNOWN, FALLBACK, ci FULL; 587 dirty, 549 selected | UNKNOWN, run, ci NARROW; 565 selected; its 3 failing packages selected (misses 0) |
| other 56 changes | — | identical plan scope, verdict, selection, unknowns and witness counts |

On pr-39 the 16 added selections are DELETED_SCRIPT_OBSERVER 7 and DEPENDENCY_PATH 9. The
direct observers are `script/check-analyzer-python-ratchets_test.sh`,
`check-goos-filenames_test.sh`, `core-n1-source_test.sh`, `public-release-check_test.sh`,
`seed-planning-store_test.sh`, `select-previous-release_test.sh` and `test-rg-fallback_test.sh`.
The 9 reached through observing helpers are five shell tests and four CI driver jobs.

`corvint --root <pr-39 head worktree> affected --base 8daec98ce434... --no-cache` returns 132
unknowns, none of them `UNINDEXED_SOURCE_PATH` (V1-5191 AC1). AFP-V0-9991 changed no selection
in the corpus.

## Rollback

Revert the code commit, then the spec commit. The graph cache profile returns to `/4`, and a `/5`
entry is rebuilt by the older binary.
