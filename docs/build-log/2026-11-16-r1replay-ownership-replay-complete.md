# 2026-11-16: ownership replay complete through the lane base (V1-4685)

Requirements: AZU-V0-002, AZU-V0-8900 (the rule replayed), AZU-V0-009 (replay retention).
This entry finishes V1-4685 acceptance criterion 3. It replays all 117 required merge trees
through `df315e35056d15c7933b5ff97c605cedf5ef4919` with a before/after pair that differs only
in the ownership rule, and with every row compared: 117 of 117, **zero dropped units**. It
supersedes the "81 trees not compared" result of
`2026-11-16-r1docs-ownership-replay-through-base.md`, whose 36 historical comparisons it keeps.

## Why a new pair

The historical pair (source `8e2c13aa9` before, `2eaaf9fba` after) refuses rows 37 to 117 with
`unsupported-affected-graph`: those trees carry declarations it cannot read. It also measures
more than the ownership rule. The range `8e2c13aa9..2eaaf9fba` adds the driver claimant
(AZU-V0-001, V1-4684), inbound caller closure (AZU-V0-004 and AZU-V0-8902, V1-4687) and the
ownership rule together, and its historical baseline already has the test-selection owner
mechanism, so that pair never switched that mechanism off.

The ownership rule entered selection in two places:

- `71b1c2f98` (AZU-V0-002) added `graph.nonSourcePaths` to `selectPlan`. It reclassifies an
  `UNOWNED_DIRTY_PATH` held by `.corvint/non-source-paths.json` as `DECLARED_NON_SOURCE`. It
  also added `NonSourcePathsPath` to `declarationPath`, so that declaration file is classified
  as a Corvint declaration.
- `29db5f084` (AZU-V0-8900) assigned the remaining tracked data verification owners in
  `.corvint/test-selection.json` (first added to the tree there). They go through the existing
  owner matching: `ownersOf` adds the owner's tests, `ownedPaths` reclassifies the path as
  `DECLARED_TEST_OWNER`, and `declaredOwnersOf` reports it. `2eaaf9fba` only adds a prefix
  fast path to `ownerPatternMatches`, with the same result.

## Pair design and the evidence-only patch

- **after**: `corvint` built from this lane's base `c7dbf717e2b2d25dd83b3f26fb7ac7def95587b6`,
  unmodified.
- **before**: the same source, plus the patch below, which switches those five call sites off.
  The patch is not committed to product code. It is stored as
  `r1replay-evidence/ownership-rule-off.patch`, SHA-256
  `9d3a56e64426948d8ad107b9aa7034ef520d7e7bf739eef9bc04c097e3989f20`.

Each source was extracted with `git archive c7dbf717e2b2d25dd83b3f26fb7ac7def95587b6` into its
own scratch tree under `/home/russell/lanes/r1replay-evidence/` (`src-after`, `src-before`;
the patch was applied to `src-before` only). Each was built with
`GOTOOLCHAIN=local script/heavy.sh go -C SRC build -buildvcs=false -o BINARY ./cmd/corvint`,
go1.27.4 linux/amd64. Both builds exited 0.

| Binary | Source | SHA-256 |
|---|---|---|
| `corvint-own-after` | `c7dbf717e` unmodified | `b1d286f8852c496e2d98b31e3e97a9ab3b58f3b0c1bf08cff8457ca6e1eea9b9` |
| `corvint-own-before` | `c7dbf717e` + patch | `a6c98bf8ba2fa8d8c8ff32dac908209d8cabcb1249525cd97ac090018a869239` |
| `corvint-ctlpre` (control) | `800b844cb4a2964c1980417d55e934b53483d0ae` (`71b1c2f98^`) | `67a036c79809d17b4adfad8f3570df8ff44c60318433fd59f860d7a45450d5ad` |
| `corvint-ctlrule` (control) | `71b1c2f9843564277d36ab72452b6148805bd13a` | `ea1677e761f295fee6171219260b94818a01d148dc7b501d32932ac6e2e84f88` |
| `corvint-azu-base` (historical) | `8e2c13aa9` | `90ce2940aaf7b6db53856c31d31db84a95cf05fe96fc4e52c967596b311e6f24` |
| `corvint-azu-candidate` (historical) | `2eaaf9fba` | `5e77baf05a3d8678b3d45996c3567e0bde7e7a089b1f2c3d797a024b7477b5c6` |

The control binaries were built the same way from `git archive` of their commits.

```diff
diff --git src-after/internal/liveverify/affected/documentation.go src-before/internal/liveverify/affected/documentation.go
index 72ebdac..e50b78a 100644
--- src-after/internal/liveverify/affected/documentation.go
+++ src-before/internal/liveverify/affected/documentation.go
@@ -157,7 +157,7 @@ func (graph *Graph) documentationPaths(unknown []Unknown) ([]NonSource, []Unknow
 func (graph *Graph) declarationPath(unknown []Unknown) ([]NonSource, []Unknown) {
 	var paths []string
 	kept := unknown
-	for _, declaration := range []string{DocumentationPathsPath, RuntimeReadersPath, NonSourcePathsPath, UnboundedReaderExclusionsPath, DeclaredSelectionPath} {
+	for _, declaration := range []string{DocumentationPathsPath, RuntimeReadersPath, UnboundedReaderExclusionsPath, DeclaredSelectionPath} { // r1replay evidence-only "before": NonSourcePathsPath removed (AZU-V0-002)
 		index := slices.IndexFunc(kept, func(entry Unknown) bool {
 			return entry.Reason == UnknownUnownedDirtyPath && entry.Detail == declaration
 		})
diff --git src-after/internal/liveverify/affected/non_source_paths.go src-before/internal/liveverify/affected/non_source_paths.go
index 80ab693..dce7745 100644
--- src-after/internal/liveverify/affected/non_source_paths.go
+++ src-before/internal/liveverify/affected/non_source_paths.go
@@ -147,6 +147,8 @@ func (graph *Graph) declaredUnindexedOutput(path string) bool {
 // kept out of the frontier rules' non-source set,
 // so a declared path still bears the AFP-V0-511 cascade like an unowned one.
 func (graph *Graph) nonSourcePaths(unknown []Unknown) ([]NonSource, []Unknown) {
+	// r1replay evidence-only "before": the AZU-V0-002/AZU-V0-8900 ownership rule is off.
+	return nil, unknown
 	if len(graph.nonSource) == 0 {
 		return nil, unknown
 	}
diff --git src-after/internal/liveverify/affected/testselection.go src-before/internal/liveverify/affected/testselection.go
index b8176cf..cad380c 100644
--- src-after/internal/liveverify/affected/testselection.go
+++ src-before/internal/liveverify/affected/testselection.go
@@ -338,6 +338,8 @@ func fixtureTest(test string) bool {
 // of that owner not already reached, witnessed by the first such dirty path
 // (AFP-V0-2482). It is terminal: nothing is traversed from the unit.
 func (graph *Graph) ownersOf(reached map[string]Witness, dirty []string) {
+	// r1replay evidence-only "before": the AZU-V0-002/AZU-V0-8900 ownership rule is off.
+	return
 	if graph.testSelection.empty() {
 		return
 	}
@@ -360,6 +362,8 @@ func (graph *Graph) ownersOf(reached map[string]Witness, dirty []string) {
 // those paths, in dirty-path order, as one ClassDeclaredTestOwner group
 // (AFP-V0-2482). No other unknown is ever removed.
 func (graph *Graph) ownedPaths(unknown []Unknown) ([]NonSource, []Unknown) {
+	// r1replay evidence-only "before": the AZU-V0-002/AZU-V0-8900 ownership rule is off.
+	return nil, unknown
 	if graph.testSelection.empty() {
 		return nil, unknown
 	}
@@ -407,6 +411,8 @@ func (graph *Graph) mustRunOf(reached map[string]Witness, dirty []string) []Must
 // dirty path, sorted by unit and then pattern; nil when there is none
 // (AFP-V0-2489).
 func (graph *Graph) declaredOwnersOf(dirty []string) []DeclaredOwner {
+	// r1replay evidence-only "before": the AZU-V0-002/AZU-V0-8900 ownership rule is off.
+	return nil
 	if graph.testSelection.empty() {
 		return nil
 	}
```

**What the patch disables**: non-source reclassification and the non-source declaration's own
`declarationPath` classification (AZU-V0-002), plus owner selection, owner reclassification
and the `declaredOwners` report (AZU-V0-8900). Every other step of `c7dbf717e` runs unchanged
in both binaries.

**What it cannot isolate, stated plainly**:

- The owner switch turns off the whole `corvint-test-selection/0` owner mechanism
  (AFP-V0-2482 to AFP-V0-2489), not only the owners AZU-V0-8900 added. In this repository the
  two coincide: `.corvint/test-selection.json` first enters the tree in `29db5f084`. It is absent
  from rows 1 to 29 and present in rows 30 to 117. Every owner entry a replayed tree holds was
  therefore written under the AZU-V0-8900 ownership regime. The file's `mustRun` entries stay
  on in both binaries.
- Three later consumers of the same declarations stay on in both binaries, so they cancel out
  of the comparison:
  - AZU-V0-9921 still uses `.corvint/non-source-paths.json` during graph construction and in
    `claimantsOf`, `unownedUnknown` and `GoPackageFile`: declared hidden Go output is not
    indexed, and declared output is not claimed by a suffix;
  - AZU-V0-8910 (V1-3480) still admits control declarations as runtime readers;
  - AZU-V0-9900 (V1-4729) still selects the ownership ratchet test.

  These are separate requirements, and they change the graph or add a selection. They are not
  the ownership matching this criterion asks about.
- `.corvint/non-source-paths.json` is absent from rows 1 to 9 and present from row 10 on.
  Rows 1 to 9 hold neither declaration, so the rule cannot act on them in any binary.

## Validation (rows 1 to 8, then 9 to 36)

For validation, a historical control pair isolates the same rule in the original history:
`71b1c2f98^` (pre-rule) against `71b1c2f98` (the AZU-V0-002 commit). It ran with the same
planner argv on rows 1 to 36.

- **Rows 1 to 8**: the control pair shows zero delta on every row (no removed or added unknown,
  no growth, no drop), and so does the new pair. They agree. The historical pair's rows 1 to 8
  delta (3 to 13 removed entries, 77 to 224 added units) has none of its removals in an
  ownership class and none of its growth witnessed `DECLARED_TEST_OWNER`, judged by the
  candidate plan's own `nonSource` classes and witnesses. Its removals are `LANGUAGE_FRONTIER`
  entries, one `NO_SELECTABLE_TEST` (row 3), and `UNOWNED_DIRTY_PATH` for `Makefile` and
  `.github/workflows/*.yml`, which the driver claimant now owns. The rows 1 to 8 decomposition
  places all of it outside the ownership commit:
  - `8e2c13aa9` to `71b1c2f98^` (driver claimant): 0 to 78 removed, 74 to 222 added units;
  - `71b1c2f98^` to `71b1c2f98` (the ownership rule): 0 removed, 0 added units on all 8 rows;
  - `71b1c2f98` to `2eaaf9fba` (inbound closure and repairs): 1 to 4 added units.

  So the new "before" behaves as a pre-rule planner on rows 1 to 8. The historical pair's delta
  there belongs to V1-4684 and V1-4687, which are on in both new binaries. No patch fix was
  needed.
- **Rows 9 to 36, where the rule acts**: on rows 17, 34 and 36 the control pair removes 4, 1 and
  20 `UNOWNED_DIRTY_PATH` entries (`DECLARED_NON_SOURCE`). The new pair removes exactly those
  entries, with no control-only entry. The new pair also removes entries the control pair does
  not on five rows: rows 30, 31, 33, 35 and 36, with 3, 1, 2, 2 and 1 entries. It also adds
  selections on rows 30, 32, 33 and 36 (19, 19, 21 and 27 units). Every one of those removals is
  `DECLARED_TEST_OWNER`, and every added unit is witnessed `DECLARED_TEST_OWNER`. Both control
  binaries and both historical binaries already had the owner mechanism. Their plans classify
  those same paths `DECLARED_TEST_OWNER` on both sides, so the mechanism cancels out of their
  comparisons; only the new "before" switches it off.
- The historical baseline was rerun on rows 1 to 36 to get its wall-clock, which the harness
  did not retain. Each rerun plan has the same selected set and the same `(reason, detail)`
  unknown set as the retained plan.

| # | Control removed / added / growth / dropped | New pair removed / added / growth / dropped | New-pair-only entries |
|---:|---|---|---|
| 1 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 2 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 3 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 4 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 5 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 6 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 7 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 8 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 9 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 10 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 11 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 12 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 13 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 14 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 15 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 16 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 17 | 4 / 0 / 0 / 0 | 4 / 0 / 0 / 0 | none |
| 18 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 19 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 20 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 21 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 22 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 23 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 24 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 25 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 26 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 27 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 28 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 29 | 0 / 0 / 0 / 0 | 0 / 0 / 0 / 0 | none |
| 30 | 0 / 0 / 0 / 0 | 3 / 0 / 19 / 0 | 3 DECLARED_TEST_OWNER |
| 31 | 0 / 0 / 0 / 0 | 1 / 0 / 0 / 0 | 1 DECLARED_TEST_OWNER |
| 32 | 0 / 0 / 0 / 0 | 0 / 0 / 19 / 0 | none |
| 33 | 0 / 0 / 0 / 0 | 2 / 0 / 21 / 0 | 2 DECLARED_TEST_OWNER |
| 34 | 1 / 0 / 0 / 0 | 1 / 0 / 0 / 0 | none |
| 35 | 0 / 0 / 0 / 0 | 2 / 0 / 0 / 0 | 2 DECLARED_TEST_OWNER |
| 36 | 20 / 0 / 0 / 0 | 21 / 0 / 27 / 0 | 1 DECLARED_TEST_OWNER |

## Replay method

- Required merges: the 8 lines of `script/affected-zero-unknown-merges.txt` (SHA-256
  `23e52a095a97042e9480d7e0c5b537a8e3507771d3b353bff45125d83abc5264`), then
  `git rev-list --first-parent --merges --reverse 8e2c13aa926fcaf1731fae71b27c41c41c8f09af..df315e35056d15c7933b5ff97c605cedf5ef4919`
  (109 merges), 117 distinct merges in all. Each is planned against its first parent.
- Planner argv, as `invokePlanner` in `tools/affected-zero-unknown-replay` runs it:
  `nice BINARY --root SNAPSHOT affected --no-cache --full --base FIRST_PARENT`. A
  `self-binary-stale` refusal gets one `--accept-stale-binary` retry; no retry occurred, and the
  wall time is that of the accepted invocation. A plan counts only when `ok` is true and its
  `selected`, `unknown` and `excluded` are arrays. Unknown entries are compared as
  `(reason, detail)` and selections by unit ID.
- SNAPSHOT is one of four standalone clones
  (`git clone --no-local --no-checkout --single-branch --branch claude/r1replay`, no
  alternates), checked out detached at the merge with a clean status. Each worker runs every
  side of one merge in turn, so at most 4 planners ran at once. Host: `nproc` 32; the 1-minute
  load average was between 3.25 and 13.92 during the runs (other lanes were active).
- The resumable script `replay-r1.py` and its fold `fold-r1.py` retain every raw stdout, stderr
  and per-plan meta record (exit, wall ms, argv, binary digest, load average): 1026 files.

## Result

- **117 of 117 rows compared** by the new pair; every plan on both sides exited 0, with no
  refusal and no stale retry. Rows 1 to 36 are also compared by the historical pair. Rows 37 to
  117 have no historical comparison: both historical binaries refuse them with
  `unsupported-affected-graph`, on `.corvint/runtime-edges.json` for rows 37 to 61 and on
  `.corvint/runtime-readers.json` for rows 62 to 117. Those refusals are retained, not counted.
- **Dropped baseline-selected units: 0 on all 117 rows** for the new pair, and 0 on rows 1 to 36
  for the historical pair. No unit that reads, executes, imports or embeds a changed path is
  dropped; with no drop at all, no per-unit tree analysis was needed.
- **Removed unknown entries: 297 on 40 rows (220 distinct)**. Every one is an
  `UNOWNED_DIRTY_PATH` that "after" reclassifies: 32 distinct as `DECLARED_NON_SOURCE` and 188 as
  `DECLARED_TEST_OWNER`. No `LANGUAGE_FRONTIER` or other reason is removed, as AZU-V0-8900
  requires.
- **Selection growth: 0 to 36 units per row**, 396 in all, on 25 rows. Every added unit is
  witnessed `DECLARED_TEST_OWNER`. Historical pair, rows 1 to 36: 77 to 235 per row (mostly the
  driver claimant, as validated above).
- **Added unknown entries: 6**, on rows 109 and 117 (3 each): `LANGUAGE_FRONTIER`
  `python:absolute-import-unresolved:+22`, `python:conditional-import` and
  `python:dynamic-import`. The owner-selected Python tests under `.github/forward-rows/` make
  Python a participating plugin (`frontierBearers` via `PARTICIPATING_PLUGIN`), so its
  frontier now bears. That is more uncertainty, never a dropped selection.
- Wall-clock per plan: 6.0 s to 49.5 s (row 71: 49.5 s before and 45.5 s after). The two sides
  totalled 1059.3 s and 1048.1 s. This shared host is not a performance qualification.
- `accepted` stays **false** for AZU-V0-009: every row still has unknowns, and the historical
  full-suite and changed-line mutation receipts are unavailable (`NOT_PRODUCED`). That is
  AZU-V0-009's own promotion condition. V1-4685 criterion 3 asks for the before/after replay,
  retained with removed unknowns and selection growth named and no reader, executor, importer or
  embedder dropped. This entry meets that criterion.

## All 117 rows (new pair)

Reason codes: LF `LANGUAGE_FRONTIER`, UDP `UNOWNED_DIRTY_PATH`, NST `NO_SELECTABLE_TEST`, UBC
`UNSELECTED_BINARY_CONSUMER`, USP `UNINDEXED_SOURCE_PATH`, E2E `E2E_UNMAPPED_CHANGE`. "=" means
the two sides have the same count for every reason. "Removed" and "Added unknown" count the
`(reason, detail)` set differences.

| # | Exact merge | First parent | Compared by | Selected before / after | Growth | Dropped | Unknown before / after | Unknown by reason, before → after | Removed | Added unknown | Wall ms before / after |
|---:|---|---|---|---:|---:|---:|---:|---|---:|---:|---:|
| 1 | `8e2c13aa926fcaf1731fae71b27c41c41c8f09af` | `39c031980` | new, historical | 594 / 594 | 0 | 0 | 79 / 79 | = LF 73 NST 2 UDP 2 UBC 2 | 0 | 0 | 9904 / 9539 |
| 2 | `39c03198086681fb24f509fbd00b34696d3268c4` | `d436dff76` | new, historical | 401 / 401 | 0 | 0 | 7 / 7 | = LF 6 NST 1 | 0 | 0 | 7542 / 7227 |
| 3 | `d436dff76de1b8e75c427069ae7f80003ae99bc2` | `6c33df33b` | new, historical | 561 / 561 | 0 | 0 | 76 / 76 | = LF 59 NST 10 UDP 5 UBC 2 | 0 | 0 | 10729 / 10290 |
| 4 | `6c33df33bc5670131482f5263b410cb980580657` | `ea770fa17` | new, historical | 527 / 527 | 0 | 0 | 58 / 58 | = LF 55 UDP 1 UBC 2 | 0 | 0 | 9313 / 8826 |
| 5 | `ea770fa17992b970aeda570ed74686a16dbc441e` | `24d8c04f9` | new, historical | 331 / 331 | 0 | 0 | 62 / 62 | = LF 59 UDP 3 | 0 | 0 | 6807 / 7047 |
| 6 | `24d8c04f9f5ffaaed86759951e520cb775dc2eff` | `f13f52b16` | new, historical | 259 / 259 | 0 | 0 | 3 / 3 | = LF 3 | 0 | 0 | 6852 / 6917 |
| 7 | `f13f52b1607222033b089144a744fedae0d35239` | `9fdc2f02e` | new, historical | 276 / 276 | 0 | 0 | 77 / 77 | = LF 71 NST 4 UDP 2 | 0 | 0 | 7289 / 7108 |
| 8 | `9fdc2f02efc11ce9ed582d8529b63fe0478fcad2` | `6f12f4704` | new, historical | 256 / 256 | 0 | 0 | 3 / 3 | = LF 3 | 0 | 0 | 6838 / 6913 |
| 9 | `cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419` | `8e2c13aa9` | new, historical | 576 / 576 | 0 | 0 | 201 / 201 | = LF 87 NST 4 USP 7 UDP 101 UBC 2 | 0 | 0 | 11637 / 11604 |
| 10 | `28a3ff86a2c3d17662765da12cc9251c06466f5f` | `cdeed7ee6` | new, historical | 607 / 607 | 0 | 0 | 83 / 83 | = LF 73 NST 3 UDP 5 UBC 2 | 0 | 0 | 12416 / 12451 |
| 11 | `c5379b4cf039fa8dd2df2aa4ee6e5b7b897cf514` | `28a3ff86a` | new, historical | 416 / 416 | 0 | 0 | 4 / 4 | = LF 4 | 0 | 0 | 7202 / 7375 |
| 12 | `9db7a94169821d9b28701eaa88866aee2fec1083` | `c5379b4cf` | new, historical | 382 / 382 | 0 | 0 | 3 / 3 | = LF 3 | 0 | 0 | 7080 / 7154 |
| 13 | `9208cc7e25bb82503ef9d61b0ce6a5f02039e649` | `9db7a9416` | new, historical | 558 / 558 | 0 | 0 | 59 / 59 | = LF 56 UDP 1 UBC 2 | 0 | 0 | 8720 / 8608 |
| 14 | `dba1f4d6c9a33098470ccf57794fb66f989f03c8` | `9208cc7e2` | new, historical | 432 / 432 | 0 | 0 | 3 / 3 | = LF 3 | 0 | 0 | 7228 / 7238 |
| 15 | `f50a0e3447a2bbadbfadbe7f8cc1019ae1636ac5` | `dba1f4d6c` | new, historical | 435 / 435 | 0 | 0 | 79 / 79 | = LF 75 NST 2 USP 1 UDP 1 | 0 | 0 | 7724 / 7450 |
| 16 | `36e2da4e800ff1f35c3931c50f01ea020cbec1db` | `f50a0e344` | new, historical | 416 / 416 | 0 | 0 | 6 / 6 | = LF 6 | 0 | 0 | 7158 / 7507 |
| 17 | `8fc0cf37642e7012f4c26671c9ed97a8706d6e09` | `36e2da4e8` | new, historical | 564 / 564 | 0 | 0 | 73 / 69 | LF 64 UDP 7 UBC 2 → LF 64 UDP 3 UBC 2 | 4 | 0 | 11075 / 10784 |
| 18 | `d886ec70adaf7e4aa7494b72212a8288d8e5cc8a` | `8fc0cf376` | new, historical | 499 / 499 | 0 | 0 | 6 / 6 | = LF 6 | 0 | 0 | 7853 / 7913 |
| 19 | `9d0c56665c039876b78eb71b877122f86c4ccd25` | `d886ec70a` | new, historical | 521 / 521 | 0 | 0 | 6 / 6 | = LF 6 | 0 | 0 | 8123 / 8022 |
| 20 | `7fd70eab2f6475074dd1c85c62a43cb5a6a7ff0e` | `9d0c56665` | new, historical | 463 / 463 | 0 | 0 | 6 / 6 | = LF 6 | 0 | 0 | 7663 / 7254 |
| 21 | `654df5053cc3a5f36aa4ecb48af36c5eaea0a86c` | `7fd70eab2` | new, historical | 499 / 499 | 0 | 0 | 6 / 6 | = LF 6 | 0 | 0 | 7825 / 7740 |
| 22 | `044058f1bde98a27be3819c5aa9193951b241d24` | `654df5053` | new, historical | 427 / 427 | 0 | 0 | 8 / 8 | = LF 6 NST 2 | 0 | 0 | 7420 / 7251 |
| 23 | `093750bb2247c49237f3cf8ba69485224791f482` | `044058f1b` | new, historical | 597 / 597 | 0 | 0 | 8 / 8 | = LF 6 UBC 2 | 0 | 0 | 8350 / 8488 |
| 24 | `670559b9ec3ffbcd59aa6c6d02029ec0cb206a93` | `093750bb2` | new, historical | 517 / 517 | 0 | 0 | 83 / 83 | = LF 78 NST 4 UDP 1 | 0 | 0 | 9605 / 9332 |
| 25 | `13342d85f6245f897c58023891824212617fe0a7` | `670559b9e` | new, historical | 576 / 576 | 0 | 0 | 71 / 71 | = LF 63 UDP 6 UBC 2 | 0 | 0 | 9262 / 9426 |
| 26 | `000d4c56c725f0f963b84bf503196f897239dddb` | `13342d85f` | new, historical | 419 / 419 | 0 | 0 | 5 / 5 | = LF 5 | 0 | 0 | 7199 / 7505 |
| 27 | `539b55331ab2c10a35963e324428b30746fb4b85` | `000d4c56c` | new, historical | 426 / 426 | 0 | 0 | 63 / 63 | = LF 62 UDP 1 | 0 | 0 | 7496 / 7470 |
| 28 | `d74f54b2783b77d6e46d204b7c7164430dc3cf4e` | `539b55331` | new, historical | 405 / 405 | 0 | 0 | 6 / 6 | = LF 5 NST 1 | 0 | 0 | 7652 / 7339 |
| 29 | `1a401eb5ee68ff4a0541534b33a5d6538793e9ea` | `d74f54b27` | new, historical | 429 / 429 | 0 | 0 | 75 / 75 | = LF 74 UDP 1 | 0 | 0 | 7278 / 7268 |
| 30 | `b75053a5b23cb0a4737a593d1fc1912aa3b79436` | `1a401eb5e` | new, historical | 582 / 601 | 19 | 0 | 70 / 67 | LF 62 NST 1 UDP 5 UBC 2 → LF 62 NST 1 UDP 2 UBC 2 | 3 | 0 | 10368 / 10669 |
| 31 | `4052b8540b5fff818a819db37b55371ea5428c2c` | `b75053a5b` | new, historical | 403 / 403 | 0 | 0 | 66 / 65 | LF 65 UDP 1 → LF 65 | 1 | 0 | 7909 / 8158 |
| 32 | `538e823f4a1d220a3fb5b12c0529248f18924823` | `4052b8540` | new, historical | 558 / 577 | 19 | 0 | 8 / 8 | = LF 6 UBC 2 | 0 | 0 | 8785 / 8843 |
| 33 | `59b31f12ed984802627f3ef44fa918f3022045cd` | `538e823f4` | new, historical | 564 / 585 | 21 | 0 | 76 / 74 | LF 74 UDP 2 → LF 74 | 2 | 0 | 9045 / 9054 |
| 34 | `fe73dc77d169b79bf0d1b2791bda50cb9ca077fe` | `59b31f12e` | new, historical | 474 / 474 | 0 | 0 | 63 / 62 | LF 61 UDP 2 → LF 61 UDP 1 | 1 | 0 | 9154 / 9107 |
| 35 | `cb403735903b1e54a8469373489960c0b2b89d99` | `fe73dc77d` | new, historical | 555 / 555 | 0 | 0 | 81 / 79 | LF 73 NST 4 UDP 2 UBC 2 → LF 73 NST 4 UBC 2 | 2 | 0 | 9023 / 9037 |
| 36 | `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943` | `cb4037359` | new, historical | 620 / 647 | 27 | 0 | 88 / 67 | LF 62 NST 3 UDP 21 UBC 2 → LF 62 NST 3 UBC 2 | 21 | 0 | 14374 / 13893 |
| 37 | `e19823d919cfa653ee0ed05708d03f5fd12ca929` | `c3b29cbb5` | new (historical refused) | 731 / 755 | 24 | 0 | 120 / 85 | LF 66 NST 17 UDP 37 → LF 66 NST 17 UDP 2 | 35 | 0 | 14944 / 14613 |
| 38 | `b0ee3282035edd255e97364b3b0b06f759d40186` | `e19823d91` | new (historical refused) | 465 / 465 | 0 | 0 | 5 / 5 | = LF 5 | 0 | 0 | 8568 / 8718 |
| 39 | `131363c695f4278126bc2f79b48bd65b0aca71c9` | `b0ee32820` | new (historical refused) | 505 / 508 | 3 | 0 | 4 / 4 | = LF 4 | 0 | 0 | 8653 / 8715 |
| 40 | `857f3b46c147074781d256e9f5f3a075ef80fd51` | `131363c69` | new (historical refused) | 425 / 428 | 3 | 0 | 5 / 5 | = LF 5 | 0 | 0 | 8072 / 8167 |
| 41 | `ae20b398c260933a152e6285de9cfc385a2580c7` | `857f3b46c` | new (historical refused) | 463 / 463 | 0 | 0 | 4 / 4 | = LF 4 | 0 | 0 | 8803 / 8634 |
| 42 | `e4f095d2b4d45bdd558b0e2fd460b59a60600791` | `ae20b398c` | new (historical refused) | 566 / 566 | 0 | 0 | 77 / 76 | LF 73 NST 1 UDP 1 UBC 2 → LF 73 NST 1 UBC 2 | 1 | 0 | 8761 / 8380 |
| 43 | `c6b6cbb0a862c36d3e2d3c28b0bd0dc0da800175` | `e4f095d2b` | new (historical refused) | 427 / 460 | 33 | 0 | 65 / 64 | LF 64 UDP 1 → LF 64 | 1 | 0 | 8014 / 7809 |
| 44 | `a7d38e73236cea3a237ce972f3bd50e11e723c29` | `c6b6cbb0a` | new (historical refused) | 587 / 587 | 0 | 0 | 5 / 5 | = LF 5 | 0 | 0 | 9186 / 9523 |
| 45 | `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734` | `a7d38e732` | new (historical refused) | 407 / 407 | 0 | 0 | 4 / 4 | = LF 4 | 0 | 0 | 8038 / 8308 |
| 46 | `9761ea1f459c0d3b3c963953e24a76eaeda16370` | `8b80e6020` | new (historical refused) | 576 / 576 | 0 | 0 | 5 / 5 | = LF 5 | 0 | 0 | 9584 / 10238 |
| 47 | `6339f980e13e36c8ff51a593568da3d935ffae77` | `9761ea1f4` | new (historical refused) | 311 / 311 | 0 | 0 | 4 / 4 | = LF 4 | 0 | 0 | 8186 / 8548 |
| 48 | `16ba0021ab0aa92d244ad1b8d54b607d12bc4ed3` | `6339f980e` | new (historical refused) | 505 / 505 | 0 | 0 | 5 / 5 | = LF 5 | 0 | 0 | 9720 / 9772 |
| 49 | `b76d89e80d7df5f0deb4dd7d134d9358106d5dbc` | `16ba0021a` | new (historical refused) | 268 / 268 | 0 | 0 | 4 / 4 | = LF 4 | 0 | 0 | 8134 / 8082 |
| 50 | `b9b2b878569b1bd3b52d30992d7cda1216f2d72a` | `b76d89e80` | new (historical refused) | 574 / 598 | 24 | 0 | 91 / 85 | LF 75 NST 8 UDP 8 → LF 75 NST 8 UDP 2 | 6 | 0 | 14741 / 15033 |
| 51 | `817817272f8b34a5753e1d5424f8c917ba31fcc4` | `b9b2b8785` | new (historical refused) | 311 / 311 | 0 | 0 | 69 / 68 | LF 67 NST 1 UDP 1 → LF 67 NST 1 | 1 | 0 | 8342 / 8441 |
| 52 | `8a1935c528bc314057d15e7db7c92f7c36384e01` | `817817272` | new (historical refused) | 548 / 548 | 0 | 0 | 62 / 60 | LF 60 UDP 2 → LF 60 | 2 | 0 | 10568 / 10381 |
| 53 | `5fb19e9efc7149de90ec919209a0047d0aafdd7c` | `8a1935c52` | new (historical refused) | 665 / 685 | 20 | 0 | 70 / 68 | LF 65 NST 2 UDP 3 → LF 65 NST 2 UDP 1 | 2 | 0 | 11592 / 11129 |
| 54 | `19c5a0111d667a11fd3a82ebcc8920f8a848260c` | `5fb19e9ef` | new (historical refused) | 484 / 484 | 0 | 0 | 5 / 5 | = LF 5 | 0 | 0 | 9109 / 8903 |
| 55 | `447bdcbd5963683f5f2b4934a45b8f48fc6742a3` | `19c5a0111` | new (historical refused) | 610 / 610 | 0 | 0 | 61 / 61 | = LF 60 UDP 1 | 0 | 0 | 9864 / 9659 |
| 56 | `1ccefe7c37a6dcbd202029d8b0958c45eb03f1c6` | `447bdcbd5` | new (historical refused) | 481 / 481 | 0 | 0 | 5 / 5 | = LF 5 | 0 | 0 | 9029 / 9040 |
| 57 | `fc47f142487e1506b863131ec301e9c10445dd89` | `1ccefe7c3` | new (historical refused) | 437 / 466 | 29 | 0 | 78 / 76 | LF 75 UDP 3 → LF 75 UDP 1 | 2 | 0 | 10511 / 10882 |
| 58 | `01c7e608617243c4ce7c4c2f88c6255169fb6e53` | `fc47f1424` | new (historical refused) | 650 / 676 | 26 | 0 | 69 / 67 | LF 64 UDP 5 → LF 64 UDP 3 | 2 | 0 | 11356 / 11968 |
| 59 | `8db85461894457c4cfd2f197ad23afb7c7e7808a` | `01c7e6086` | new (historical refused) | 371 / 372 | 1 | 0 | 65 / 64 | LF 64 UDP 1 → LF 64 | 1 | 0 | 10183 / 10038 |
| 60 | `64f6257da673b35b21cc3a29563bcfa6f823aa73` | `8db854618` | new (historical refused) | 263 / 263 | 0 | 0 | 2 / 2 | = LF 2 | 0 | 0 | 9225 / 9195 |
| 61 | `b1a117eb129687f9555fa0c826fadfa0627afe3b` | `64f6257da` | new (historical refused) | 354 / 390 | 36 | 0 | 3 / 3 | = LF 3 | 0 | 0 | 9772 / 9353 |
| 62 | `cb6322613a7b3b99f7ea3d2351e8ffd880735d9d` | `b1a117eb1` | new (historical refused) | 463 / 463 | 0 | 0 | 61 / 60 | LF 60 UDP 1 → LF 60 | 1 | 0 | 9496 / 9807 |
| 63 | `fea1de8587085a4538484b97d216c32511ff3928` | `cb6322613` | new (historical refused) | 656 / 664 | 8 | 0 | 108 / 102 | LF 75 NST 1 USP 6 UDP 26 → LF 75 NST 1 USP 6 UDP 20 | 6 | 0 | 14471 / 14978 |
| 64 | `568f962f973b89e553e20ce27db2779d965a7a5d` | `fea1de858` | new (historical refused) | 279 / 279 | 0 | 0 | 2 / 2 | = LF 2 | 0 | 0 | 8985 / 9264 |
| 65 | `9684e632e49bb54584a00ab44f3a7a27454d67c5` | `568f962f9` | new (historical refused) | 421 / 421 | 0 | 0 | 74 / 71 | LF 71 UDP 3 → LF 71 | 3 | 0 | 9609 / 9498 |
| 66 | `1d1f0670616654a19cbeedbb1e0c3bfcdae86601` | `9684e632e` | new (historical refused) | 570 / 570 | 0 | 0 | 31 / 31 | = LF 29 UBC 2 | 0 | 0 | 16581 / 16298 |
| 67 | `7bf7ffe8de371f29d4a3e4b0c03857732e4c8d45` | `1d1f06706` | new (historical refused) | 236 / 236 | 0 | 0 | 2 / 2 | = LF 2 | 0 | 0 | 9022 / 9352 |
| 68 | `15248276816ad89f62ee22edaa2b93c9e2218855` | `7bf7ffe8d` | new (historical refused) | 277 / 277 | 0 | 0 | 2 / 2 | = LF 2 | 0 | 0 | 9623 / 9377 |
| 69 | `f2fdae7e8fe3c111d3f075a2c6e91af150f224c9` | `152482768` | new (historical refused) | 287 / 287 | 0 | 0 | 2 / 2 | = LF 2 | 0 | 0 | 9463 / 9532 |
| 70 | `b6fda75b7dd0c133cf829b2efa0f3732367c429d` | `f2fdae7e8` | new (historical refused) | 240 / 240 | 0 | 0 | 65 / 64 | LF 64 UDP 1 → LF 64 | 1 | 0 | 9440 / 9301 |
| 71 | `b54ac564d8983e40597eb753a5c49ea092287984` | `b6fda75b7` | new (historical refused) | 550 / 560 | 10 | 0 | 562 / 491 | LF 73 NST 7 USP 94 UDP 388 → LF 73 NST 7 USP 94 UDP 317 | 71 | 0 | 49480 / 45457 |
| 72 | `2913fdc7e73d9e73524ef30f5ef03a69c371cc86` | `b54ac564d` | new (historical refused) | 182 / 182 | 0 | 0 | 2 / 2 | = LF 2 | 0 | 0 | 6181 / 6052 |
| 73 | `867ff4991ec88203f858b37e82bfadbf4f6d3396` | `2913fdc7e` | new (historical refused) | 318 / 348 | 30 | 0 | 3 / 3 | = LF 3 | 0 | 0 | 6148 / 6463 |
| 74 | `a761ad43be18b4bb03de425a1b0b1f6073a8bb8c` | `867ff4991` | new (historical refused) | 323 / 355 | 32 | 0 | 3 / 3 | = LF 3 | 0 | 0 | 6544 / 6492 |
| 75 | `4c2e6b6a24ea251ee33a74410fd9436e7394a767` | `a761ad43b` | new (historical refused) | 206 / 206 | 0 | 0 | 1 / 1 | = LF 1 | 0 | 0 | 6123 / 6215 |
| 76 | `f6be4233cf209f251d7a039515c3e832cdad8c33` | `4c2e6b6a2` | new (historical refused) | 196 / 196 | 0 | 0 | 2 / 2 | = LF 2 | 0 | 0 | 6196 / 6039 |
| 77 | `dc84799051004a22992185e61c54c26a28bb9c4d` | `f6be4233c` | new (historical refused) | 530 / 530 | 0 | 0 | 79 / 64 | LF 53 NST 9 UDP 15 UBC 2 → LF 53 NST 9 UBC 2 | 15 | 0 | 12601 / 12337 |
| 78 | `02c66d0e263a5437f5c4745c64f3b4bd25188b4f` | `dc8479905` | new (historical refused) | 219 / 220 | 1 | 0 | 57 / 55 | LF 55 UDP 2 → LF 55 | 2 | 0 | 6749 / 6502 |
| 79 | `d318e8d4a5154c50f204323af5f8d29c834edaf5` | `02c66d0e2` | new (historical refused) | 185 / 185 | 0 | 0 | 0 / 0 | = none | 0 | 0 | 6280 / 6243 |
| 80 | `993c60eba4d4cfeba7cdb3f7b3f21130596e4247` | `d318e8d4a` | new (historical refused) | 190 / 190 | 0 | 0 | 1 / 1 | = LF 1 | 0 | 0 | 6327 / 6460 |
| 81 | `7b28ab461c057777dadda21580115b92e713b84b` | `993c60eba` | new (historical refused) | 441 / 441 | 0 | 0 | 6 / 6 | = LF 4 UBC 2 | 0 | 0 | 6815 / 7086 |
| 82 | `b13c6fe4c58bc4f075d8790f59400f41ed94cf93` | `7b28ab461` | new (historical refused) | 237 / 237 | 0 | 0 | 2 / 2 | = LF 2 | 0 | 0 | 6659 / 6630 |
| 83 | `87e291a98f659dfcf5bd62ba1132c7972dc9dab5` | `b13c6fe4c` | new (historical refused) | 386 / 386 | 0 | 0 | 19 / 19 | = E2E 12 LF 4 NST 1 UBC 2 | 0 | 0 | 7576 / 7769 |
| 84 | `4c3c203336a6cc4a9772af39b1a771f34236138e` | `87e291a98` | new (historical refused) | 190 / 190 | 0 | 0 | 1 / 1 | = LF 1 | 0 | 0 | 6614 / 6491 |
| 85 | `b55635262788687c3eea9a08a94378e4ab8856a4` | `4c3c20333` | new (historical refused) | 331 / 331 | 0 | 0 | 77 / 71 | LF 64 NST 7 UDP 6 → LF 64 NST 7 | 6 | 0 | 7135 / 6936 |
| 86 | `af393f1b1e20f59d536663276f30469bea688f6a` | `b55635262` | new (historical refused) | 375 / 375 | 0 | 0 | 60 / 60 | = LF 58 UBC 2 | 0 | 0 | 8211 / 7822 |
| 87 | `87c57df657cbe6fe132f2cc8ed31b37824991ac0` | `af393f1b1` | new (historical refused) | 226 / 226 | 0 | 0 | 2 / 2 | = LF 2 | 0 | 0 | 6671 / 6561 |
| 88 | `c863c43c3ca93d9815afdad64843005928cc6ca5` | `87c57df65` | new (historical refused) | 509 / 509 | 0 | 0 | 72 / 70 | LF 68 UDP 2 UBC 2 → LF 68 UBC 2 | 2 | 0 | 10171 / 9941 |
| 89 | `79755fb78d5bffca05f42ad2fcb5640966efe3f5` | `c863c43c3` | new (historical refused) | 550 / 552 | 2 | 0 | 169 / 119 | LF 68 NST 3 USP 3 UDP 93 UBC 2 → LF 68 NST 3 USP 3 UDP 43 UBC 2 | 50 | 0 | 15890 / 15724 |
| 90 | `0727600418e1520bb051732eca25250a2fb785e6` | `79755fb78` | new (historical refused) | 518 / 518 | 0 | 0 | 82 / 59 | LF 53 NST 4 UDP 23 UBC 2 → LF 53 NST 4 UBC 2 | 23 | 0 | 13333 / 12973 |
| 91 | `94e8edd9e334c6df3191f918746b0ae109b589c5` | `072760041` | new (historical refused) | 350 / 350 | 0 | 0 | 6 / 6 | = E2E 2 LF 2 UBC 2 | 0 | 0 | 8606 / 8387 |
| 92 | `4482b164ff0e291eddecdf4faf7df8f5604cfe1d` | `94e8edd9e` | new (historical refused) | 255 / 255 | 0 | 0 | 58 / 58 | = LF 58 | 0 | 0 | 6797 / 6991 |
| 93 | `3091c834ba8c86d2c39ed8abc2a52a8521f1cc78` | `4482b164f` | new (historical refused) | 321 / 321 | 0 | 0 | 65 / 64 | LF 64 UDP 1 → LF 64 | 1 | 0 | 6804 / 6551 |
| 94 | `366408098f741126468695245f461cb4fa9dbf8b` | `3091c834b` | new (historical refused) | 426 / 438 | 12 | 0 | 89 / 76 | LF 72 NST 2 UDP 13 UBC 2 → LF 72 NST 2 UBC 2 | 13 | 0 | 8636 / 9065 |
| 95 | `6c17e38889a275097f01ebb3533027ac8592b2eb` | `366408098` | new (historical refused) | 238 / 238 | 0 | 0 | 4 / 4 | = LF 2 UBC 2 | 0 | 0 | 6776 / 7119 |
| 96 | `9c2bd0fa26c089825984cab3938c62dcb064facd` | `6c17e3888` | new (historical refused) | 291 / 291 | 0 | 0 | 71 / 68 | LF 67 NST 1 UDP 3 → LF 67 NST 1 | 3 | 0 | 6833 / 7085 |
| 97 | `94a0f567c7c1a9f0992d45a511ffc027358c27ba` | `9c2bd0fa2` | new (historical refused) | 296 / 296 | 0 | 0 | 4 / 4 | = LF 2 UBC 2 | 0 | 0 | 7323 / 7145 |
| 98 | `1c837facaee3a0f3bc6ac638e1548b58b03ac733` | `94a0f567c` | new (historical refused) | 455 / 455 | 0 | 0 | 57 / 55 | LF 53 UDP 2 UBC 2 → LF 53 UBC 2 | 2 | 0 | 9045 / 9057 |
| 99 | `294a23b3b580ac902920894143231c2e02883f63` | `1c837faca` | new (historical refused) | 213 / 218 | 5 | 0 | 60 / 59 | LF 58 NST 1 UDP 1 → LF 58 NST 1 | 1 | 0 | 6804 / 6816 |
| 100 | `f19ecc1db4340a66992d64e91724b26e5dc5440c` | `294a23b3b` | new (historical refused) | 345 / 345 | 0 | 0 | 58 / 57 | LF 55 UDP 1 UBC 2 → LF 55 UBC 2 | 1 | 0 | 8063 / 8160 |
| 101 | `39050561619ca426f28ee4699bde87a156abeead` | `f19ecc1db` | new (historical refused) | 296 / 296 | 0 | 0 | 4 / 4 | = LF 2 UBC 2 | 0 | 0 | 7387 / 7335 |
| 102 | `e7dea64795b0ac573b0542c72fadbf7e76b9c031` | `390505616` | new (historical refused) | 496 / 496 | 0 | 0 | 71 / 67 | LF 63 NST 2 UDP 4 UBC 2 → LF 63 NST 2 UBC 2 | 4 | 0 | 9908 / 9659 |
| 103 | `814647cf0e55fcb8f810e3708387dad5a2224cf7` | `e7dea6479` | new (historical refused) | 503 / 503 | 0 | 0 | 4 / 4 | = LF 2 UBC 2 | 0 | 0 | 8738 / 8879 |
| 104 | `ee2dd4e341e2e26e7ecdc7ab7732f93b8d59bcf9` | `814647cf0` | new (historical refused) | 421 / 421 | 0 | 0 | 6 / 6 | = LF 4 UBC 2 | 0 | 0 | 8160 / 8250 |
| 105 | `603a2780782a674fa38f8ea944608547cffefd40` | `ee2dd4e34` | new (historical refused) | 256 / 256 | 0 | 0 | 2 / 2 | = LF 2 | 0 | 0 | 6882 / 7008 |
| 106 | `33954a914b1c48c6427db15bdade5ef6fceee7ec` | `603a27807` | new (historical refused) | 378 / 378 | 0 | 0 | 57 / 56 | LF 54 UDP 1 UBC 2 → LF 54 UBC 2 | 1 | 0 | 9027 / 8390 |
| 107 | `29bd615fdf93d55e7012377d2eac52ad5183fb86` | `33954a914` | new (historical refused) | 210 / 210 | 0 | 0 | 1 / 1 | = LF 1 | 0 | 0 | 7767 / 7151 |
| 108 | `babe97abb110feb735fef9098e0e91765c102e46` | `29bd615fd` | new (historical refused) | 192 / 192 | 0 | 0 | 1 / 1 | = LF 1 | 0 | 0 | 7817 / 7005 |
| 109 | `1a02c5f39ce333278987900d1afacf6324ebab25` | `babe97abb` | new (historical refused) | 196 / 200 | 4 | 0 | 57 / 59 | LF 55 NST 1 UDP 1 → LF 58 NST 1 | 1 | 3 | 8116 / 7041 |
| 110 | `2fcf093a6910c2cc070b2db69b9e2d6693f8f8af` | `1a02c5f39` | new (historical refused) | 247 / 247 | 0 | 0 | 3 / 3 | = LF 3 | 0 | 0 | 6990 / 6723 |
| 111 | `bedd33a6becdaef3d3355fe0d7f39d03ba214418` | `2fcf093a6` | new (historical refused) | 323 / 323 | 0 | 0 | 4 / 4 | = LF 4 | 0 | 0 | 8082 / 7711 |
| 112 | `f43eb7c0132df31962265e9d6ce0d78df70b6ac2` | `bedd33a6b` | new (historical refused) | 187 / 187 | 0 | 0 | 57 / 56 | LF 56 UDP 1 → LF 56 | 1 | 0 | 6860 / 6765 |
| 113 | `bf67be4ea7feae49a46105eec375309f3babae24` | `f43eb7c01` | new (historical refused) | 469 / 469 | 0 | 0 | 4 / 4 | = LF 2 UBC 2 | 0 | 0 | 7869 / 7866 |
| 114 | `469d5fa03bf937125f10e65e4c7af29b1194fa01` | `bf67be4ea` | new (historical refused) | 342 / 342 | 0 | 0 | 58 / 57 | LF 55 UDP 1 UBC 2 → LF 55 UBC 2 | 1 | 0 | 8237 / 7827 |
| 115 | `1cee3c23d51ca632f474816cb6450191893de524` | `469d5fa03` | new (historical refused) | 190 / 190 | 0 | 0 | 1 / 1 | = LF 1 | 0 | 0 | 6836 / 6720 |
| 116 | `4e984cc8591e58dfda172d2d3901b592122053b5` | `1cee3c23d` | new (historical refused) | 211 / 214 | 3 | 0 | 3 / 3 | = LF 3 | 0 | 0 | 6888 / 6467 |
| 117 | `df315e35056d15c7933b5ff97c605cedf5ef4919` | `4e984cc85` | new (historical refused) | 196 / 200 | 4 | 0 | 56 / 58 | LF 55 UDP 1 → LF 58 | 1 | 3 | 6837 / 6017 |

## Rows 1 to 36 (historical pair): unknown by reason and wall-clock

This adds the per-reason and wall-clock columns that the through-base entry's table lacks. The
candidate's wall time is the r1docs harness run's `wallMillis`. The baseline's was not retained,
so that row was rerun with the same binary and argv here; the rerun plans match the retained
ones exactly.

| # | Unknown by reason, base → candidate | Selected base / candidate | Growth | Dropped | Wall ms base (rerun) / candidate (harness) |
|---:|---|---:|---:|---:|---:|
| 1 | LF 81 NST 2 USP 2 UDP 5 UBC 2 → LF 79 NST 2 USP 2 UDP 2 UBC 2 | 266 / 490 | 224 | 0 | 5440 / 9344 |
| 2 | LF 81 USP 2 UDP 1 → LF 71 USP 2 | 66 / 181 | 115 | 0 | 3681 / 6990 |
| 3 | LF 81 NST 10 UDP 6 UBC 2 → LF 79 NST 9 UDP 5 UBC 2 | 233 / 442 | 209 | 0 | 6182 / 9222 |
| 4 | LF 79 UDP 2 UBC 2 → LF 77 UDP 1 UBC 2 | 183 / 388 | 205 | 0 | 5178 / 8091 |
| 5 | LF 60 UDP 3 → LF 75 UDP 3 | 75 / 181 | 106 | 0 | 3185 / 6641 |
| 6 | LF 78 UDP 1 → LF 68 | 38 / 118 | 80 | 0 | 3157 / 9790 |
| 7 | LF 78 NST 4 UDP 4 → LF 76 NST 4 UDP 2 | 46 / 131 | 85 | 0 | 3239 / 6990 |
| 8 | LF 78 UDP 1 → LF 68 NST 1 | 36 / 113 | 77 | 0 | 3106 / 6843 |
| 9 | LF 93 NST 4 USP 7 UDP 102 UBC 2 → LF 92 NST 4 USP 7 UDP 101 UBC 2 | 267 / 485 | 218 | 0 | 6266 / 12035 |
| 10 | LF 80 NST 3 UDP 6 UBC 2 → LF 79 NST 3 UDP 5 UBC 2 | 248 / 465 | 217 | 0 | 7949 / 17006 |
| 11 | LF 79 UDP 1 → LF 69 | 81 / 193 | 112 | 0 | 3332 / 7943 |
| 12 | LF 78 UDP 1 → LF 68 | 43 / 133 | 90 | 0 | 3192 / 12268 |
| 13 | LF 79 UDP 1 UBC 2 → LF 77 UDP 1 UBC 2 | 188 / 411 | 223 | 0 | 4187 / 7810 |
| 14 | LF 1 → LF 67 | 89 / 306 | 217 | 0 | 3185 / 7213 |
| 15 | LF 81 USP 5 UDP 3 → LF 79 NST 1 USP 5 UDP 1 | 80 / 315 | 235 | 0 | 3491 / 7462 |
| 16 | LF 2 → LF 68 | 73 / 174 | 101 | 0 | 3214 / 7196 |
| 17 | LF 83 UDP 7 UBC 2 → LF 82 UDP 3 UBC 2 | 199 / 336 | 137 | 0 | 5951 / 11135 |
| 18 | LF 79 UDP 1 → LF 69 | 94 / 313 | 219 | 0 | 3513 / 8861 |
| 19 | LF 2 → LF 68 | 170 / 271 | 101 | 0 | 3700 / 11802 |
| 20 | LF 2 → LF 68 | 114 / 232 | 118 | 0 | 3374 / 11358 |
| 21 | LF 3 → LF 69 | 113 / 230 | 117 | 0 | 3959 / 12220 |
| 22 | LF 79 NST 2 UDP 4 → LF 69 NST 3 | 83 / 209 | 126 | 0 | 3483 / 11363 |
| 23 | LF 71 → LF 69 | 127 / 244 | 117 | 0 | 4072 / 12864 |
| 24 | LF 82 NST 4 USP 29 UDP 1 → LF 80 NST 4 USP 29 UDP 1 | 147 / 283 | 136 | 0 | 5188 / 16001 |
| 25 | LF 79 UDP 7 UBC 2 → LF 77 UDP 6 UBC 2 | 205 / 339 | 134 | 0 | 4947 / 14841 |
| 26 | LF 1 USP 1 → LF 67 USP 1 | 84 / 203 | 119 | 0 | 3177 / 8157 |
| 27 | LF 78 UDP 1 → LF 76 UDP 1 | 92 / 208 | 116 | 0 | 3545 / 12484 |
| 28 | LF 80 USP 2 UDP 1 → LF 69 USP 2 | 60 / 181 | 121 | 0 | 3376 / 12286 |
| 29 | LF 78 UDP 2 → LF 75 UDP 1 | 82 / 205 | 123 | 0 | 3411 / 21825 |
| 30 | LF 81 NST 1 UDP 2 UBC 2 → LF 78 NST 1 UDP 2 UBC 2 | 198 / 323 | 125 | 0 | 5905 / 23957 |
| 31 | LF 79 → LF 76 | 83 / 203 | 120 | 0 | 3743 / 21021 |
| 32 | LF 81 UDP 1 UBC 2 → LF 70 UBC 2 | 181 / 401 | 220 | 0 | 4162 / 21306 |
| 33 | LF 80 USP 2 UDP 3 → LF 77 USP 2 | 216 / 345 | 129 | 0 | 4130 / 20605 |
| 34 | LF 79 UDP 2 → LF 76 UDP 1 | 106 / 200 | 94 | 0 | 4626 / 21977 |
| 35 | LF 80 NST 3 USP 5 UDP 2 → LF 77 NST 3 USP 5 | 108 / 244 | 136 | 0 | 3760 / 20465 |
| 36 | LF 81 NST 3 UDP 21 UBC 2 → LF 78 NST 3 UBC 2 | 234 / 467 | 233 | 0 | 8141 / 29533 |

## Every removed and added unknown entry (new pair)

Each distinct `(reason, detail)` set difference, with the class "after" gives the path and the
rows (numbered as above). Backticks inside details are escaped.

- UNOWNED_DIRTY_PATH: .agent-evidence/cx-affected-ci-failures.json → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/cx-affected-selection-replay.json → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/cx-affected-verification.json → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/docs-mcp-hardening/docs-check-corvint.json → DECLARED_NON_SOURCE (rows 90)
- UNOWNED_DIRTY_PATH: .agent-evidence/docs-mcp-hardening/live-reload.txt → DECLARED_NON_SOURCE (rows 90)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/collisions-test.log → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/completion-checks.json → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/fanout-completion.log → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/fanout-fault.json → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/fanout-fault.log → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/fanout-final.log → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/fanout-race.log → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/fixtures-final.json → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/go-measurements.json → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/lane-scripts-test.log → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/line-citations-final-test.log → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/measure.txt → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/measurements-final.json → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/pi-namespace-fault.log → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/pi-scratch-test.log → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/prechange-final-test.log → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lanetools/reservations-test.log → DECLARED_NON_SOURCE (rows 36)
- UNOWNED_DIRTY_PATH: .agent-evidence/lineidx-pr-measurement.json → DECLARED_NON_SOURCE (rows 52)
- UNOWNED_DIRTY_PATH: .agent-evidence/lineidx-qualification-blocked.json → DECLARED_NON_SOURCE (rows 52)
- UNOWNED_DIRTY_PATH: .agent-evidence/pk-affected-fix-round-1.json → DECLARED_NON_SOURCE (rows 17)
- UNOWNED_DIRTY_PATH: .agent-evidence/pk-affected-verification.json → DECLARED_NON_SOURCE (rows 17)
- UNOWNED_DIRTY_PATH: .claude/skills/corvint-lane-flow/SKILL.md → DECLARED_TEST_OWNER (rows 71,89)
- UNOWNED_DIRTY_PATH: .corvint/.gitignore → DECLARED_NON_SOURCE (rows 34)
- UNOWNED_DIRTY_PATH: .gitattributes → DECLARED_TEST_OWNER (rows 53)
- UNOWNED_DIRTY_PATH: .github/cishards/package-costs.json → DECLARED_TEST_OWNER (rows 33,65)
- UNOWNED_DIRTY_PATH: .github/cishards/test-slices.json → DECLARED_TEST_OWNER (rows 33,65,96)
- UNOWNED_DIRTY_PATH: .github/cishards/test-split-allow.json → DECLARED_TEST_OWNER (rows 65)
- UNOWNED_DIRTY_PATH: .github/forward-rows/bar.jq → DECLARED_TEST_OWNER (rows 85,117)
- UNOWNED_DIRTY_PATH: .github/forward-rows/infra-reclassified.txt → DECLARED_TEST_OWNER (rows 70)
- UNOWNED_DIRTY_PATH: .github/forward-rows/known-flakes.tsv → DECLARED_TEST_OWNER (rows 96,109)
- UNOWNED_DIRTY_PATH: .github/forward-rows/mainmiss.jq → DECLARED_TEST_OWNER (rows 85)
- UNOWNED_DIRTY_PATH: .github/forward-rows/narrow.jq → DECLARED_TEST_OWNER (rows 35,51,85)
- UNOWNED_DIRTY_PATH: .github/forward-rows/row.jq → DECLARED_TEST_OWNER (rows 35,42,71,85)
- UNOWNED_DIRTY_PATH: .github/linecov/qualification.json → DECLARED_TEST_OWNER (rows 57,85)
- UNOWNED_DIRTY_PATH: .gitignore → DECLARED_TEST_OWNER (rows 37,71,77,89)
- UNOWNED_DIRTY_PATH: README.md → DECLARED_TEST_OWNER (rows 31,71,89,94)
- UNOWNED_DIRTY_PATH: benchmarks/README.md → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v2-manifest.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v2/beamfall.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v2/cobra.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v2/execa.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v2/flask.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v2/zod.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v3-manifest.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v3/beamfall.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v3/cobra.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v3/execa.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v3/flask.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v3/zod.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v4-manifest.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v4/beamfall.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v4/cobra.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v4/execa.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v4/flask.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v4/zod.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v6-dev/cobra.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v6-dev/flask.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v6-dev/manifest.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/blind-v7/pool.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/cases/cobra.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/cases/execa.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/cases/flask.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/cases/zod.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/daily-loop-v0/README.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: benchmarks/eval-split-v0.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/lsp-quality/README.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: benchmarks/lsp-quality/public-v0.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: benchmarks/lsp-quality/qualification-freeze-v0.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: benchmarks/manifest.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: benchmarks/results/2026-11-04-ranking-frozen-summary.json → DECLARED_TEST_OWNER (rows 37,89)
- UNOWNED_DIRTY_PATH: benchmarks/runner/README.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: benchmarks/selection-baseline/changes.json → DECLARED_TEST_OWNER (rows 59,62,90,98)
- UNOWNED_DIRTY_PATH: benchmarks/snapshot-reader/go.mod → DECLARED_TEST_OWNER (rows 94)
- UNOWNED_DIRTY_PATH: benchmarks/ticket-brief-task-set/task-set.json → DECLARED_TEST_OWNER (rows 78)
- UNOWNED_DIRTY_PATH: benchmarks/ticket-brief-task-set/task-set.json.sha256 → DECLARED_TEST_OWNER (rows 78)
- UNOWNED_DIRTY_PATH: benchmarks/untouched-repository-v1/README.md → DECLARED_TEST_OWNER (rows 71,89)
- UNOWNED_DIRTY_PATH: cmd/corvint-web-flows/help.json → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: conformance/cli-errors-v0/README.md → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: conformance/cli-parity-v0/README.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: conformance/cli-parity-v0/manifest.json → DECLARED_TEST_OWNER (rows 71,89)
- UNOWNED_DIRTY_PATH: conformance/divergence-register.md → DECLARED_TEST_OWNER (rows 71,100)
- UNOWNED_DIRTY_PATH: conformance/interactive-alpha/README.md → DECLARED_TEST_OWNER (rows 77)
- UNOWNED_DIRTY_PATH: conformance/interactive-alpha/fixture/go.mod → DECLARED_TEST_OWNER (rows 94)
- UNOWNED_DIRTY_PATH: conformance/mcp-2026-07-28/README.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: conformance/mcp-2026-07-28/cases-task-review.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: conformance/mcp-2026-07-28/cases.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: conformance/release-artifact-v0/README.md → DECLARED_TEST_OWNER (rows 94)
- UNOWNED_DIRTY_PATH: conformance/release-artifact-v0/archive-report-v2.schema.json → DECLARED_TEST_OWNER (rows 94)
- UNOWNED_DIRTY_PATH: conformance/release-artifact-v0/archive-report.schema.json → DECLARED_TEST_OWNER (rows 94)
- UNOWNED_DIRTY_PATH: conformance/release-artifact-v0/manifest.json → DECLARED_TEST_OWNER (rows 71,89,94)
- UNOWNED_DIRTY_PATH: conformance/use-cases-v0/README.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: conformance/vscode-extension-v0/README.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: conformance/vscode-extension-v0/cases.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: docs/E2E-ADOPTION.md → DECLARED_TEST_OWNER (rows 63)
- UNOWNED_DIRTY_PATH: docs/EXTERNAL-APP-ACCEPTANCE.md → DECLARED_TEST_OWNER (rows 63)
- UNOWNED_DIRTY_PATH: docs/LSP.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: docs/TASKS-BARRIER-2026-10-30.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: docs/TASKS-EXTERNAL-AGENTS.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: docs/TASKS-SUPERVISION.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: docs/post-merge-workflow/replay.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: examples/evidence-provider/v0/README.md → DECLARED_TEST_OWNER (rows 94)
- UNOWNED_DIRTY_PATH: extensions/vscode/CHANGELOG.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: extensions/vscode/README.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: extensions/vscode/package.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: flows/installed-go-live-tests.json → DECLARED_TEST_OWNER (rows 77,89,94)
- UNOWNED_DIRTY_PATH: flows/installed-js-live-tests.json → DECLARED_TEST_OWNER (rows 77,89-90,94)
- UNOWNED_DIRTY_PATH: flows/installed-test-validity-mcp.json → DECLARED_TEST_OWNER (rows 77,89-90,94,102)
- UNOWNED_DIRTY_PATH: flows/web-flows-browser-gate.json → DECLARED_TEST_OWNER (rows 77,99)
- UNOWNED_DIRTY_PATH: go.mod → DECLARED_TEST_OWNER (rows 94)
- UNOWNED_DIRTY_PATH: integrations/claude-code/.claude-plugin/marketplace.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/claude-code/plugins/corvint/.claude-plugin/plugin.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/claude-code/plugins/corvint/.mcp.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/claude-code/plugins/corvint/compatibility.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/claude-code/plugins/corvint/hooks/hooks.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/claude-code/plugins/corvint/skills/affected/SKILL.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/claude-code/plugins/corvint/skills/context/SKILL.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/claude-code/plugins/corvint/skills/done/SKILL.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/claude-code/plugins/corvint/skills/impact/SKILL.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/claude-code/plugins/corvint/skills/index/SKILL.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/claude-code/plugins/corvint/skills/review/SKILL.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/claude-code/plugins/corvint/skills/status/SKILL.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/codex/plugins/corvint/.codex-plugin/plugin.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/codex/plugins/corvint/.mcp.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/codex/plugins/corvint/compatibility.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/codex/plugins/corvint/hooks/authority-hooks.proposed.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/codex/plugins/corvint/hooks/hooks.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/codex/plugins/corvint/hooks/qualified-direct-hooks.proposed.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/codex/plugins/corvint/hooks/qualified-hooks.proposed.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/codex/plugins/corvint/scripts/authority-hook.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: integrations/codex/plugins/corvint/skills/corvint/SKILL.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: internal/capcatalog/AGENT-GUIDE.md → DECLARED_TEST_OWNER (rows 63,71,77,89,102)
- UNOWNED_DIRTY_PATH: internal/capcatalog/CLI-RATCHET.tsv → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/contextindex/testdata/context-recipe-default-golden.json → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: internal/contextindex/testdata/role-summary-golden.tsv → DECLARED_TEST_OWNER (rows 89)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/g/g.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/g/g_test.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/go.mod → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/lib/lib.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/p/p.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/p/p_test.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/q/q.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/q/q_test.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/r/r.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/r/r_test.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/s/s.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/s/s_test.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/t3/t3.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/t3/t3_test.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/t4/t4.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/t4/t4_test.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/t5/t5.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/t5/t5_test.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/t6/t6.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/t6/t6_test.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/w/w.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/golang/typedreach/testdata/mod/w/w_test.go → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/liverepo/testdata/ownership-audit-baseline.txt → DECLARED_TEST_OWNER (rows 77)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/testdata/boundary-scanner.go.txt → DECLARED_TEST_OWNER (rows 77)
- UNOWNED_DIRTY_PATH: internal/liveverify/affected/testdata/ownership-audit-baseline.txt → DECLARED_TEST_OWNER (rows 30,77)
- UNOWNED_DIRTY_PATH: internal/mcp/cli/testdata/tools-list-default.golden.jsonl → DECLARED_TEST_OWNER (rows 50,71,77,89-90)
- UNOWNED_DIRTY_PATH: internal/mcp/cli/testdata/tools-list-flows.golden.jsonl → DECLARED_TEST_OWNER (rows 50,71,77,89-90,102)
- UNOWNED_DIRTY_PATH: internal/mcp/cli/testdata/tools-list-groups.golden.jsonl → DECLARED_TEST_OWNER (rows 50,71,77,89-90,102)
- UNOWNED_DIRTY_PATH: internal/mcp/cli/testdata/tools-list-import-graph.golden.jsonl → DECLARED_TEST_OWNER (rows 50,71)
- UNOWNED_DIRTY_PATH: internal/mcp/cli/testdata/tools-list-task-review-lsp.golden.jsonl → DECLARED_TEST_OWNER (rows 50,71)
- UNOWNED_DIRTY_PATH: internal/mcp/cli/testdata/tools-list-task-review.golden.jsonl → DECLARED_TEST_OWNER (rows 50,71)
- UNOWNED_DIRTY_PATH: internal/nativelinecov/testdata/live/CMakeLists.txt → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/nativelinecov/testdata/live/dotnet/Probe.csproj → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/nativelinecov/testdata/live/rust/Cargo.toml → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/nativelinecov/testdata/live/swift/Package.swift → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: internal/perfbudget/budgets.json → DECLARED_TEST_OWNER (rows 71,77,89)
- UNOWNED_DIRTY_PATH: internal/sessiontest/testdata/corvint-test-run-0.contract.json → DECLARED_TEST_OWNER (rows 90)
- UNOWNED_DIRTY_PATH: internal/sessiontest/testdata/receipt-before-testid.json → DECLARED_TEST_OWNER (rows 90)
- UNOWNED_DIRTY_PATH: internal/sessiontest/testdata/receipt-with-testid.json → DECLARED_TEST_OWNER (rows 90)
- UNOWNED_DIRTY_PATH: internal/taskman/testdata/audit.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: internal/taskman/testdata/cal-v0-099-prerequisite-refusals.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: internal/taskman/testdata/issue502-optional-keys-record.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: internal/taskman/testdata/policy.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: internal/taskman/testdata/queue.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: internal/taskman/testdata/status.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: internal/taskman/testdata/tickets.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: internal/taskman/testdata/verdict-paths-record.json → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: internal/testid/testdata/go-test.jsonl → DECLARED_TEST_OWNER (rows 90)
- UNOWNED_DIRTY_PATH: internal/testid/testdata/jest.json → DECLARED_TEST_OWNER (rows 90)
- UNOWNED_DIRTY_PATH: internal/testid/testdata/minitest.jsonl → DECLARED_TEST_OWNER (rows 90)
- UNOWNED_DIRTY_PATH: internal/testid/testdata/playwright-mixed.json → DECLARED_TEST_OWNER (rows 90)
- UNOWNED_DIRTY_PATH: internal/testid/testdata/playwright-projects.json → DECLARED_TEST_OWNER (rows 90)
- UNOWNED_DIRTY_PATH: internal/testid/testdata/pytest.json → DECLARED_TEST_OWNER (rows 90)
- UNOWNED_DIRTY_PATH: internal/testid/testdata/rspec.json → DECLARED_TEST_OWNER (rows 90)
- UNOWNED_DIRTY_PATH: internal/testid/testdata/vitest-mixed.json → DECLARED_TEST_OWNER (rows 90)
- UNOWNED_DIRTY_PATH: internal/testvaliditydoc/testdata/recorded/preserves.head.json → DECLARED_TEST_OWNER (rows 63)
- UNOWNED_DIRTY_PATH: internal/testvaliditydoc/testdata/recorded/regression.head.json → DECLARED_TEST_OWNER (rows 63)
- UNOWNED_DIRTY_PATH: interop/spec-toolchains/speckit/requirements.txt → DECLARED_TEST_OWNER (rows 112)
- UNOWNED_DIRTY_PATH: llms.txt → DECLARED_TEST_OWNER (rows 37,53,63,71)
- UNOWNED_DIRTY_PATH: script/affected-zero-unknown-merges.txt → DECLARED_TEST_OWNER (rows 30)
- UNOWNED_DIRTY_PATH: script/ci-least-privilege-exceptions.tsv → DECLARED_TEST_OWNER (rows 57,85,93,96)
- UNOWNED_DIRTY_PATH: script/diagnostic-coverage.count → DECLARED_TEST_OWNER (rows 71,77,89,114)
- UNOWNED_DIRTY_PATH: script/lane-screen_test.pl → DECLARED_TEST_OWNER (rows 36,43)
- UNOWNED_DIRTY_PATH: script/line-citation-drift-waivers.txt → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: script/line-citation-legacy-documents.txt → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: script/requirement-citations-allowlist.tsv → DECLARED_TEST_OWNER (rows 88-90,98)
- UNOWNED_DIRTY_PATH: script/requirement-citations-undefined.txt → DECLARED_TEST_OWNER (rows 88-89)
- UNOWNED_DIRTY_PATH: tests/README.md → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: tools/affected-edit-oracle/README.md → DECLARED_TEST_OWNER (rows 58)
- UNOWNED_DIRTY_PATH: tools/affected-edit-oracle/testdata/corpus.json → DECLARED_NON_SOURCE (rows 58,89)
- UNOWNED_DIRTY_PATH: tools/affected-zero-unknown-replay/README.md → DECLARED_TEST_OWNER (rows 30)
- UNOWNED_DIRTY_PATH: tools/ci-test-slices/testdata/README.md → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: tools/ci-test-slices/testdata/run-42106672783-costs.json → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: tools/ci-test-slices/testdata/run-42106672783-p1.golden → DECLARED_TEST_OWNER (rows 37)
- UNOWNED_DIRTY_PATH: tools/corvint-pr-tests/README.md → DECLARED_TEST_OWNER (rows 94)
- UNOWNED_DIRTY_PATH: tools/fault-harness/README.md → DECLARED_TEST_OWNER (rows 90,106)
- UNOWNED_DIRTY_PATH: tools/fault-harness/revert-rule.json → DECLARED_NON_SOURCE (rows 90)
- UNOWNED_DIRTY_PATH: tools/fault-harness/revert-rule.json.sha256 → DECLARED_NON_SOURCE (rows 90)
- UNOWNED_DIRTY_PATH: tools/flow-guide-check/unhinted-refusals.tsv → DECLARED_TEST_OWNER (rows 71)
- UNOWNED_DIRTY_PATH: tools/str-parity/testdata/local-default.txt → DECLARED_NON_SOURCE (rows 17)
- UNOWNED_DIRTY_PATH: tools/str-parity/testdata/local-full.json → DECLARED_NON_SOURCE (rows 17)

- added LANGUAGE_FRONTIER: python:absolute-import-unresolved:+22 (rows 109,117)
- added LANGUAGE_FRONTIER: python:conditional-import (rows 109,117)
- added LANGUAGE_FRONTIER: python:dynamic-import (rows 109,117)

## Evidence bindings

Raw plans, logs, scripts and the folded report are under `/home/russell/lanes/r1replay-evidence/`
(not committed). SHA-256:

- `ownership-rule-off.patch`: `9d3a56e64426948d8ad107b9aa7034ef520d7e7bf739eef9bc04c097e3989f20`.
- `report.json` (117 rows): `c359f05d7d5a4211a96798c6c614f396b7f9ad9be7c046a79ffea99e10a0b141`.
- `validation.json`: `085fffdafc8e868217a07561ac113180150c5096d7ac7dde9f88ae6a36c6d774`.
- `replay-r1.py`: `525d6bee77d6464ee92cebf13e5a72c7be9fb80227e7dcc8978873cf3fe86d25`.
- `fold-r1.py`: `12eb1038ae19a747691e50cd80afdadc46d97e105ae137b61fbb82e3b9dbb517`.
- `gen-md.py`: `2f491ef4229a753c8ba83831f1885f9202846db5ccbb8a37af677c55147f8dd0`.
- `replay-pair.log`: `5a01b4eefa32c1efefcac23d7afdc48b40ce7af1178b3130b0b1ffd3a288bff8`.
- `replay-histbase.log`: `b24cba77eb7ac48c80950d59f8d9669e359d5ada79c7c85f90022e196db67ff9`.
- `replay-control.log`: `e13d353901205dc2d59c937f4adafd7e5454d13b608aaa85567758f5a4f5e415`.
- Historical raw plans, rows 1 to 36: `r1docs-evidence/ownership-replay-review/` (its
  `report.json` `737beeb80649221ceeaaf65b5abd7c74867aa468de87440db8deae9797e14916`).
  Historical refusals: `r1docs-evidence/ownership-replay-all/` (its `report.json`
  `028d9e446a0857778aca14ad3beb89c13657e53420201cd1534306393c2d8a24`).
