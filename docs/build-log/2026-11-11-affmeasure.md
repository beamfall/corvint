# affmeasure: measured remainders of V1-5033, V1-5174 and V1-5184

Lane charter of 2026-11-11 (orchestrator, v1-0, P1). The code for all three tickets is on
`origin/main` `4c3c203336a6cc4a9772af39b1a771f34236138e`. An audit on that date found that each
ticket still lacked a test or a measurement its acceptance criteria name. This lane adds two
tests and the measurements. It adds no capability, no requirement ID and no production change.
The ticket bodies could not be read on this host: the staged queue snapshot predates them. The
criteria quoted below are the charter's.

All `corvint` runs use binaries built through `script/heavy.sh`. Raw plans and logs are outside
the repository in `/home/russell/lanes/affmeasure.tmp/`. Probe edits were never committed, and
each one was restored with `git checkout --` right after its run.

## V1-5033 criterion 2: exemplar `36e2da4e8` replay

`git cat-file -e 36e2da4e8^{commit}` returned 0, so the exemplar is reachable and no substitute
merge was needed. PR 145 (`5fb19e9ef`) was a 162-commit batch. The V1-5033 code is exactly
`2fc8637fa` and `3b17b8939`, stacked on `205d18826`, and they change only the driver files and
the avdrv tests. So the replay compares binaries built from `205d18826` (before) and
`3b17b8939` (after), each from a `git archive` export. Method, as in the selfloor and rsdecl
replays: a private local clone is checked out at the PR head `faaec550f` (second parent of
`36e2da4e8`), and each binary runs `affected --base dba1f4d6c9a33098470ccf57794fb66f989f03c8
--format json`, where `dba1f4d6c` is the merge base of the two parents. The dirty set is the
exemplar's three paths: its CEM, its build-log entry and `internal/sessiontest/balanced_test.go`.
Both runs exit 0. For reference, a third column runs the current-main lane binary on the same
tree with `--accept-stale-binary`, because without that flag it refuses with `self-binary-stale`
(exit 2).

| Selected units by witness | before `205d18826` | after `3b17b8939` | main `4c3c20333` (degraded) |
|---|---:|---:|---:|
| BINARY_EXEC | 9 | 9 | 9 |
| DECLARED_READ_SCOPE | 24 | 24 | 24 |
| DEPENDENCY_PATH | 80 | 80 | 82 |
| DIRECT_TEST_CHANGE | 1 | 1 | 1 |
| FRONTIER_SITE | 126 | 126 | 127 |
| IMPORT_GRAPH_READER | 14 | 14 | 14 |
| PATH_LITERAL_READER | 11 | 11 | 11 |
| UNBOUNDED_READER | 55 | 55 | 55 |
| **Selected units** | **320** | **320** | **323** |
| Unknown entries (all LANGUAGE_FRONTIER) | 67 | 67 | 4 |
| `driver:computed-invocation` sites | 216 | 216 | 218 |

The before and after plans are byte-identical: SHA-256 `1c60e2c1c954cdc4…` for both. No unit was
dropped or added, no witness changed, and no frontier site or unknown changed.

**Disposition: NOT_MET as worded.** The replay drops no truly affected unit, but it also drops
no frontier claim. That follows from AFP-V0-9402 as accepted: "Existing computed fallbacks and
their `FRONTIER_SITE` witnesses SHALL remain intact"
(`docs/specs/affected-plan-selection-v0.md:2764`). The file-set edge `/*` is applied only to a
target or job that has no computed-invocation fallback (`internal/liveverify/affected/driver.go:345`).
On this tree every literal enumerator also runs an uncertified command, so none loses its
frontier. A crude text check of the root `Makefile` at `faaec550f` finds 8 of the 115 root
`FRONTIER_SITE` targets with `git ls-files`, `git grep`, `find`, `rg` or `go run` in their own
recipe. They are `capabilities-check`, `declaration-compat-check`, `flow-guide-check`,
`rg-fallback-test`, `tasks-profiles-check`, `test-exec-writes-check`, `test-read-scopes-check`
and `walk-skipdir-check`, and all 8 keep `FRONTIER_SITE` in both plans. This is a recipe-text
grep, not a parse. The criterion and AFP-V0-9402 conflict; reconciling them is a spec decision,
and this lane leaves it to the owner.

## V1-5033 criterion 3: FRONTIER_SITE witnesses name their edge or reason

`TestDriverFrontierSiteWitnessNamesEdgeOrReason_AFPV09402`
(`internal/liveverify/affected/driver_frontier_witness_test.go:15`) builds a fixture with
computed make and workflow sites, a literal enumerator, a caller of both and unrelated
controls. It plans five dirty paths: new, owned source, owned test, unindexed and a CEM. For
every `FRONTIER_SITE` selection, the test requires three things: a `Via` chain that ends at the
unit, a dirty path, and a chain start that is a graph unit with a non-empty frontier reason. It
also requires that the enumerator and the callers keep real `PATH_GLOB_READER` or
`DEPENDENCY_PATH` edges, and that the controls stay unselected.

The code already guarantees this. All seven places that construct a `FRONTIER_SITE` witness
set `Via: []string{id}`, for example `internal/liveverify/affected/driver.go:67`, so there is
no production change. Mutation check: with `Via: nil` at `internal/liveverify/affected/driver.go:67`, the test failed with
"names no edge ending at the unit" for both computed sites. The original was then restored.

Observation, not changed: in the JSON plan a `FRONTIER_SITE` selection carries only
`viaLength: 1`. A computed-invocation site's reason is discharged from `unknown` and appears
neither in `frontierBearers` nor per unit: in the exemplar, 0 of 126 `FRONTIER_SITE` units are
named in `frontierBearers`. The reason exists in the graph unit's `Frontier` field and as the
aggregate `frontierSites` count. Exposing it per selection would touch `select.go`, which this
lane may not edit.

**Disposition: MET.**

## V1-5174 criterion 3: a test per guard predicate

`TestRepositoryTreePredicatesSelectOnlyMatchingEdits_AFP_V0_3300`
(`internal/liveverify/affected/liverepo/readscope_tree_predicates_repo_test.go:30`) reads
`.corvint/test-read-scope-trees.json` and builds the repository graph once. It lives in
`liverepo`, following `2230dd6f3`. For each content predicate it first checks that the graph
admitted the predicate and that the guard-derived sample matches. Then, under the tree, it
asserts three outcomes. Adding the sample selects the guard; removing it selects the guard; an
edit that matches at neither endpoint does not. All 8 entries pass: `fixture-maintenance-check`
`cmd/`, `conformance/`, `internal/`, `interop/` and `tools/`, and `flow-guide-check` `cmd/`,
`internal/` and `tools/`. The test fails if the file declares no predicate. Mutation check: with
the HEAD endpoint removed from `treeScopeBears`, the "sample added" cases failed. The original
was then restored.

**Disposition: MET.**

## V1-5174 criterion 4: whole-tree count and converted-guard selection

Entries in `.corvint/test-read-scope-trees.json`:

| Revision | Packages | Entries | Reason strings | Predicate objects | Listing-only |
|---|---:|---:|---:|---:|---:|
| `2f451d63a^` (before V1-5174) | 15 | 37 | 37 | 0 | 0 |
| `2f451d63a` (V1-5174) | 15 | 37 | 23 | 14 | 0 |
| `4c3c20333` (this base) | 11 | 26 | 18 | 8 | 0 |

The count did fall from 37, to 26 entries (18 still plain). The fall since `2f451d63a` comes from
packages removed under decision 0594, for example `internal/obscorpus` and its 6 predicates, not
from further conversions. Under AFP-V0-3302 every entry, predicate or not, still names a whole
top-level tree with a reason.

Measurement: the lane binary was built at `176c97b66` (SHA-256 `ff849563eb1ad7aa…`). One comment
line, `// affmeasure probe`, was appended to one `internal/` file, and the binary ran
`affected --format json` against implicit HEAD with that single dirty path. No predicate changed,
so before and after are the same run.

| Probe file | Summary line | `fixture-maintenance-check` | `flow-guide-check` |
|---|---|---|---|
| `internal/clitree/paths.go` | `scope=BOUNDED dirty=1 selected=175 excluded=445 unknown=0` | not selected | selected, PATH_LITERAL_READER, via 1 (predicate match) |
| `internal/analyzerruby/lexer.go` | `scope=UNKNOWN dirty=1 selected=220 excluded=402 unknown=2 LANGUAGE_FRONTIER=2` | selected, PATH_LITERAL_READER, via 1 (predicate match) | selected, PATH_LITERAL_READER, via 2 (not its predicate) |
| control `internal/analyzerdotnet/norace_test.go` | `scope=UNKNOWN dirty=1 selected=146 excluded=474 unknown=1 LANGUAGE_FRONTIER=1` | not selected | not selected |

What matched:
- `paths.go`: the flow-guide predicate matches `pache-2` inside the SPDX header
  `Apache-2.0` on line 1, plus `fixed-position` in a comment. The file has no `git`.
- `lexer.go`: `git` matches inside the `github.com/...` import. The flow-guide predicate does
  not match this file. The guard comes in through the chain
  `typescript:internal/appmap/navigation-events.cjs` → `tools/flow-guide-check`: the
  TypeScript runtime frontier reaches that unit, and the file lies under the guard's literal
  `internal` read. A predicate cannot remove that route.
- Control: the control file matches neither predicate, and neither guard is selected, which
  shows the predicate path itself works.

Neither predicate was tightened. Both fail closed, for these reasons:
- `fixture-maintenance-check`: the guard lexes every `_test.go` file that contains `git`
  (`tools/fixture-maintenance-check/guard_test.go:152`, `tools/fixture-maintenance-check/guard_test.go:159`). Any scanner error fails the
  guard (`tools/fixture-maintenance-check/guard_test.go:83`), whatever the file's string literals say. A predicate narrower
  than `git`, such as `git` together with one of the fact tokens, would miss an edit that adds
  a lexical error to a test file that contains `git` but no fact token. The V1-5174 author kept
  `git` for exactly this reason (`docs/build-log/2026-11-08-tree-scope-predicates.md:55`). The
  predicate also cannot see the `_test.go` file-name filter, so non-test files that import
  `github.com/...` still match.
- `flow-guide-check`: the verdict depends on
  `bytes.Contains(sources, code)` for each refusal code in the guide table
  (`tools/flow-guide-check/main.go:231`). Codes are any kebab or upper-snake token
  (`tools/flow-guide-check/main.go:201`), searched as substrings, plus `refuse`/`Refuse` callee prefixes (`tools/flow-guide-check/main.go:725`).
  The current regex already is that grammar, unanchored. A narrower regex would have to list the
  guide's current codes, and nothing keeps that list in step with the guide. A later code added
  to the guide, followed by a source edit removing it, would then go unselected.

**Disposition: NOT_MET.** The whole-tree count did fall from 37, but both converted guards are
still selected by an ordinary `internal/` comment edit, and no sound tighter predicate exists.

## V1-5184 criterion 2: workflow-YAML LANGUAGE_FRONTIER on the ticket's own change set

The probe appended `# affmeasure probe` to `.github/workflows/release-gates.yml`, the file whose
anchors AZU-V0-214 admitted, and to `.github/workflows/ci.yml`. It then ran
`affected --format json` with the binary built at `b2ae37ad2` (SHA-256 `0f71556c461e5108…`).
The run exited 0, and both edits were restored, so no `.github/` change is committed.

| Field | Value |
|---|---|
| Summary line | `scope=UNKNOWN dirty=2 selected=154 excluded=475 unknown=1 LANGUAGE_FRONTIER=1` |
| Selected by namespace | driver 120, go 20, shell 9, typescript 5 |
| Selected by witness | DEPENDENCY_PATH 50, FRONTIER_SITE 47, UNBOUNDED_READER 32, PATH_LITERAL_READER 24, PATH_GLOB_READER 1 |
| `driver:unparsable-file` sites / unknowns | 0 / 0 |
| Unknown entries | 1: LANGUAGE_FRONTIER `shell:foreign-invocation-unmodelled` |

The one remaining unknown is a shell frontier, not workflow YAML. Its bearers are the reached
units `shell:.github/forward-rows/github-guard.sh` (PATH_LITERAL_READER),
`shell:.github/forward-rows/mq-narrow.sh` and `shell:.github/forward-rows/narrow.sh` (both
DEPENDENCY_PATH), each reached from `.github/workflows/ci.yml`. No selection code was changed.

**Disposition: MET.** The two-workflow change set has no workflow-YAML LANGUAGE_FRONTIER
unknown. The single remaining unknown belongs to another language and is listed above with its
units.

## Commands and results

| Command | Result |
|---|---|
| `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -run 'TestDriverFrontierSiteWitnessNamesEdgeOrReason_AFPV09402' ./internal/liveverify/affected/` | rc 0 (first draft rc 1: its fixture's `go test ./...` job had a real import edge, so the job's witness was DEPENDENCY_PATH; the fixture was corrected) |
| `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -run 'TestRepositoryTreePredicatesSelectOnlyMatchingEdits_AFP_V0_3300' ./internal/liveverify/affected/liverepo/` | rc 0, 8 subtests |
| the same tests under the two temporary mutations above | rc 1 each, as intended; sources restored |
| `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'AFPV094\|AFPV0905\|AFPV0312\|FrontierSite\|AFP_V0_33\|AFPV033' ./internal/liveverify/affected/` | rc 0 |
| `script/heavy.sh … -run 'TestRepositoryTreePredicatesSelectOnlyMatchingEdits_AFP_V0_3300\|TestDeclaredReadScopes\|TestFrontierSiteRootMakefile' ./internal/liveverify/affected/liverepo/` | rc 0 |
| `script/heavy.sh … -run 'TestRepositoryGitFixturesDisableDetachedMaintenance\|TestUnguardedDetectsAMissingSafeguard' ./tools/fixture-maintenance-check/` | rc 0 |
| `script/heavy.sh … -run 'TestImportRatchet\|TestCheckAcceptsTheBaselinedGraph' ./tools/import-ratchet/` | rc 0 |
| `script/heavy.sh … -run 'TestASDV0929RepositoryIsClean' ./tools/test-exec-writes-check/` | rc 0 |
| `script/heavy.sh env GOTOOLCHAIN=local go vet` on `./internal/liveverify/affected/` and `./internal/liveverify/affected/liverepo/` | rc 0 |
| `gofmt -l` on both new test files | no output |
| `make changed-checks BASE=4c3c203336a6cc4a9772af39b1a771f34236138e` | rc 0 (unbounded-readers-check and test-read-scopes-check ran) |
| `corvint affected --base 4c3c203336a6cc4a9772af39b1a771f34236138e` for the lane's test commits | rc 0, `scope=UNKNOWN dirty=2 selected=167 excluded=455 unknown=1`, 43 Go packages |

NOT_RUN:
- The other 38 Go packages that the lane's own affected plan selects, mostly whole-tree guards
  and binaries. Only test files changed. The guards that read test-file content were run.
- `make gate` and the exhaustive Verify block, which the charter excludes.
- Independent review: the orchestrator runs it.

`.corvint/test-read-scope-trees.json` is unchanged, so no declaration check beyond
`changed-checks` was required. Rollback: delete the two test files and this entry.
