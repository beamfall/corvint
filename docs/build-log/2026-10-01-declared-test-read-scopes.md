# Declared test read scopes take root locators off the affected floor

Owner request (2026-10-01, V1-0246, V1-0081): make CI run only the tests a change needs. The owner
chose to shrink the AFP-V0-012 rule (d) floor first, before the AFP-V0-013/014 qualification
campaign or a merge queue. AFP-V0-023 adds the project-owned `.corvint/test-read-scopes.json`. It
lists, for each root-locating test package, the repository paths that package's tests read. The
planner and the selector then select such a package through that scope and no longer through the
unbounded-reader floor. Full CI runs every declared package under the Landlock wrapper
`.github/testconfine`, so a declaration that is too narrow fails in CI rather than under-selecting.

## How the declaration was derived

The declaration is measured rather than inferred, in three passes. All three ran in a
`golang:1.27.1` container with `--init`, with the repository mounted as a tmpfs snapshot of the
committed tree.

1. **Proposal.** Each root-locating package's tests ran under `strace -f`. Successful read opens
   under the root, outside the package's own directory, were bucketed into proposed entries. This
   pass only proposes entries.
2. **Confined run.** The package's test binary ran through the wrapper with the proposed entries.
3. **Comparison.** Per-test `go test -json` outcomes, confined against unconfined, were compared,
   covering pass, fail and skip for every test and subtest. A package was declared only when every
   outcome was identical.

The proposal produced 106 candidate packages. Their outcomes were then compared:

- **Rejected for confinement (6).** These pass unconfined and fail under the proposed scope:
  - `internal/analyzernativebridge`, in `TestNativeBridgePreReviewEarlyReturnCleanup`;
  - `internal/dashboard/repository`, in its real-repository, containment and
    executable-replacement tests;
  - `internal/flowcoverage` and `internal/flowdocs`;
  - `internal/tasks/archive` and `internal/tasks/intent`, including subdirectory and
    linked-worktree resolution.

  The traced proposal under-covered each of them, for example through reads by a child process or
  reads the trace could not resolve. They stay on the floor until each has a measured scope.
- **Rejected because it fails anyway (1).** `tools/gate-ledger` fails
  `TestGoTestKeysResolvedPackagesPerPackage` both confined and unconfined.
- **Declared (99).** The other 98, plus `internal/corpusindex`. The first unconfined run of
  `internal/corpusindex` missed the time budget of `TestIndexedCorpusCapacityQualification` under
  parallel load. A serial rerun gave identical outcomes in both modes: 36 tests, 0 skips.

The declaration holds 75 distinct entries across the 99 packages. Fifteen further root locators
fail unconfined in this container, mostly from missing toolchains or services, so they were not
compared and stay on the floor (NOT_RUN):

- `cmd/corvint-analyzer-c-jni`, `cmd/corvint-go-test-provider`, `cmd/corvint-postmerge-metrics`
- `conformance/release-artifact-v0`
- `internal/authoritystore`, `internal/behaviorfalsify`, `internal/companionrelease`,
  `internal/console`, `internal/dashboard/source`, `internal/doccorpus`,
  `internal/jstestprovider`, `internal/lspstdio`, `internal/opencodequalification`,
  `internal/tasks/authority`, `internal/tasks/store`

## Landlock link and rename finding

The first wrapper handled only READ_FILE and READ_DIR (ABI 1). Several declared packages then failed
with `EXDEV` on ordinary `os.Rename` and `os.Link` between two granted directories. A ruleset that
does not handle REFER refuses every cross-directory reparenting. The wrapper therefore requires ABI
2. It handles REFER and grants it on `/`. Landlock still refuses a reparenting that would widen a
file's read access. `TestExecConfinedDeniesUndeclaredRepositoryReads_AFPV0023` covers three cases:
a move within a granted tree, a hard link between granted trees, and a refused move out of a denied
tree.

## Selection effect

The replay covered 16 recent merges of `origin/main`. Each merge ran `corvint affected --base
<first parent>` and then `tools/gate-affected-select`, with the declaration grafted onto both ends,
and was costed with serial per-package test times. The full suite is 253 packages and about 5518
package-seconds.

At merges after `d693d285`, packages declared in the newer tree do not exist yet, so the
declaration was pruned to the packages present at each merge (95 or 96 packages). Without that
pruning the declaration is invalid there, and the selector correctly falls back to the full run.

| Measure | Without declaration | With declaration | Change |
| --- | --- | --- | --- |
| Mean packages selected | 200 | 166 | −17.1% |
| Mean package-seconds | 5023 | 4543 | −9.6% |

Per merge, the cut ranged from 13 to 56 packages and from 2% to 14% of the cost. The remaining
cost is mostly import-graph selection of expensive hub packages, such as `cmd/corvint`, which
this change does not touch. Reducing that cost belongs to qualifying narrower runs under
AFP-V0-013/014, not to the floor.

No member of a selection carried a `declared` line. In every replayed merge, each declared package
with a dirty path in scope was already selected by an import edge or a literal reader.

## Graph digest compatibility

`readScoped` (`omitzero`) and `readScope` (`omitempty`) appear in the AFP-V0-005 digest projection
only for a declared unit. An undeclared graph therefore keeps its digest, and the frozen
`affected-plan/0` goldens are unchanged. JSON v2's `omitempty` does not omit `false`, which is why
`readScoped` uses `omitzero`. `TestSelectionOnTheLiveDirtyWorktree` now resolves a
`DECLARED_READ_SCOPE` witness against the declared unit's scope.

## Residuals and NOT_RUN

- **Hosted run.** The hosted confined full CI run is NOT_RUN until the admin `ci-control-plane`
  approval admits the protected `.github` change at the exact head.
- **What Landlock does not see.** Landlock does not mediate metadata or the existence of a path. A
  test that tolerates a denied read is also not detected. V1-0635 tracks auditing for hidden skips
  and stale entries.
- **Brittle validity.** Deleting or renaming a declared package makes the whole declaration invalid,
  and selection then falls back to the full run. This fails closed but is brittle, and is retained
  as a known limit.
- **Unconfined runs.** The AFP-V0-013 driver runs are not confined (V1-0633), and `gate-ledger`
  does not read the scopes (V1-0634).
- **Select budget.** The live-worktree select budget overran under host load (V1-0636).
