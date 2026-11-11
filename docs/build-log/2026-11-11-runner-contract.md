# 2026-11-11: runner contract, first lane (V1-5416)

Change `runner-contract`: tasks 1 to 4, 6 and 9 are built. Tasks 5, 7 (waits on the NMS-V1-008
parser), 8, 10, 11 and 12 are not. Requirements RNC-V1-001 to RNC-V1-018 are in
`docs/specs/runner-contract-v1.md`, where each requirement's Delivery line names what is built and
what is not yet built. This change is third in the R38 testing order. It builds on
`canonical-test-id` (V1-5398) and `test-inventory` (V1-5406, tiv lane head `f2922029a`), and leaves
`internal/testid` and `internal/testinventory` unchanged.

## What changed

- `internal/testrunner/contract` (tests layer, under the existing `internal/testrunner` prefix of
  `tools/import-ratchet/areas.json`) holds the compiled-in interface, derived from
  `registry.RunnerPlugin`, and one implementation per blocking runner. Go, Vitest and Playwright
  select and parse. Jest, pytest, RSpec and Minitest run their full command with
  `runner-not-implemented`.
- `corvint test` reads `.corvint/test-commands.json` (runner and instance entries, optional `go`)
  with built-in defaults per runner instance. It refuses a malformed file with
  `test-commands-invalid` and prints `DEFAULT` and `NOTICE` lines.
- `UNOWNED_DIRTY_PATH` is off the allowlist. An unowned path plans FULL with `unowned-path` and a
  removed runner with `runner-removed`.
- Acceptance-log row `runner-contract` (kind `change`, mode `auto`): the change implements the
  accepted PROD-TEST-006 and PROD-TEST-011 (decision 0596, G14).

## Differences from the proposal, followed by intent

1. **Where the interface lives.** The design derives one interface from `registry.RunnerPlugin`.
   It is a new package, `internal/testrunner/contract`, beside the registry, so that its tests run
   alone and the registry's protocol-era plugins stay untouched until task 10. `Command` keeps
   `CommandRequest`/`CommandResponse`, keyed by canonical test ID and instance. `Superset` becomes
   `exact` with a reason and a tier. `Parse` returns one result per canonical test ID with a
   duration and granularity. `Inventory` leaves the run path for the inventory's lister.
   `Pieces` and `Evidence` stay outside, as qualification and evidence do.
2. **Listers are named, not moved.** "Each runner supplies its lister to refresh" is met by each
   runner naming the inventory's own lister identity and kind (`testinventory.CurrentLister`).
   The listers stay in `internal/testinventory`, which cannot import the contract without a cycle,
   so refresh runs the same listers and none is duplicated.
3. **The `corvint test` narrowed path is not switched.** Go still runs through the STR-V0 adapter,
   and Vitest and Playwright through `corvint-js-test-provider`. The contract's selection argv,
   report parsing and reconciliation are proven by fixtures but are not yet what `corvint test`
   executes for a narrowed selection. Switching them belongs with task 5 (receipt per-test results)
   and task 11 (retiring the provider). FULL runs of every runner instance already resolve through
   the contract's defaults.
4. **The `.corvint/test-commands.json` shape.** The file keeps profile `corvint-test-commands/0`,
   adds `runners` and `instances` and makes `go` optional. The old path keeps every rule it had,
   `go` required included. `runners.go` is refused: Go's full command stays the typed `go` member,
   because the Go adapter needs its environment and flags. With no `go` member, Go's built-in
   default (`go test` with no declared environment or flags, over the planned universe or
   `./...`) runs only where the tree declares a Go instance (a `go.mod`). A language entry under
   `languages` still covers all of that language's runners. Runner and instance entries apply to
   a language with no entry.
5. **"Corvint's own repository".** The repository whose root `go.mod` declares
   `module github.com/Beamfall/corvint`.
6. **Moving Corvint's own declaration is not done.** The Makefile (`make go-test` through
   `tools/test-commands-run`), `tools/test-commands-check` and the CI workflows read
   `.github/cishards/test-commands.json`. Moving it means deleting that file and editing those
   readers, which edits `.github/`, and this lane does not edit `.github/`. `corvint test` here
   therefore reads the old path with the notice, and `tools/test-commands-run` now reads that file
   directly rather than through `ReadDeclared`. Remaining step: copy the file to
   `.corvint/test-commands.json`, point the Makefile, `tools/test-commands-check`,
   `tools/test-commands-run` and the workflows at it, delete the old file, and after one release
   delete the old-path branch of `sessiontest.ReadDeclared`.
7. **Frozen advice text.** `corvint affected`'s `FOCUSED_TEST_COMMAND_UNDECLARED` unknown is
   frozen Core output (CCF-V1-006, the core-freeze goldens) and still names the old path. It now
   appears only when neither file exists. Its claim that `corvint test --dry-run` "would fall
   back to a FULL scope with no units" no longer holds now that built-in defaults exist, and
   rewording it changes frozen goldens. Left for a CCF decision.
8. **STR-V0-003 for adopters.** For any repository but Corvint's own, an absent declaration no
   longer makes `corvint test` INCOMPLETE: built-in defaults run. A malformed
   `.corvint/test-commands.json` exits 2 instead of 3. STR-V0-003 carries the amendment, and
   `TestSTRV0003CLIUnreadableDeclarationIsIncomplete` became
   `TestRNCV1013CLIMalformedCommandsRefused`. The old-path rules are still pinned in Corvint's own
   module (`TestSTRV0003ReadDeclared`, `TestSTRV0120...`). `corvint adopt --setup` (ADS-V0-802)
   reads the new path.
9. **Instances come from the inventory verdict.** `corvint test` takes the declared instances from
   the HEAD inventory verdict (`testinventory.Detect`) and passes them into the pure manifest
   input. The verdict is now read before the manifest is built. A removed runner is a record in
   `Verdict.Undeclared`, an instance entry with no declared instance, or a runner entry with no
   instance of its runner. Widening for an unavailable runner binary at plan time is not built.
10. **Exactness of file filters.** Vitest and Playwright match file filters by substring or
    pattern. A file selection is `exact` only when the committed inventory lists no other file of
    the instance whose path contains the filter. With no listing it is `exact: false`. A
    test-level ID runs its whole file (`exact: false`), because the title filters apply to every
    file. Runner flags are from public documentation and were not run against real tools
    (NOT_VERIFIED). Each is pinned by a fixture.
11. **Unowned paths and the qualified proof.** `gateselect.SelectQualified` re-derives a proof
    from a Go-only graph, in which every non-Go path is unowned. Taking those entries as
    `unowned-path` would send almost every change (a build-log entry, a `.corvint/` declaration)
    to FULL. The proof therefore ignores its own `UNOWNED_DIRTY_PATH` entries. The planner's
    entries, which see every plugin and declaration, already fell back in `selectPackagesFrom`.

## Narrowing given up by owner decision R32

A path no plugin claims, even one a Go package encloses, embeds or names, now runs every test. On
this repository, `affected --worktree --path` over a test-data file, a build-log entry,
`README.md`, `.github/workflows/ci.yml` and `Makefile` names no `UNOWNED_DIRTY_PATH`, because the
repository declares them. `go.sum` does, so a dependency bump now runs FULL. Bare fixtures lost
their narrowing:

- an assembly file, an embedded data file and a data file beside a package (TSP-V0-006,
  AFP-V0-2377): the expectations now accept the FULL plan, where the package still runs whole;
- an unowned addition with the ownership audit (AZU-V0-9900): the FULL plan keeps the named
  obligation on the whole package;
- a hosted fixture no plugin indexes, under a Go package's `testdata` (AFP-V0-9013 and
  AFP-V0-9310): the planner still classes it `UNOWNED_DIRTY_PATH` ("data"), and the session now
  runs every test, its readers included, instead of only its readers. This conflicts with
  AFP-V0-9013's "without FULL". On this repository the fixtures sampled
  (`internal/analyzerswift/testdata/.../Package.swift`, `internal/docmaintain/testdata/*.py`) are
  not unowned;
- a deleted manifest name (AFP-V0-980) and a read-scope declaration edit (AFP-V0-050): these
  fixtures now declare the path, as this repository does (documentation, runtime reader), so their
  narrowing is still proven.

An alternative was considered and not built: count an unowned path as owned by the Go instance
when a Go attribution rule (an enclosing package, an embed, a path-literal reader) maps it to a
package. That would restore the narrowing above, but it reopens the gap R32 closes: a non-Go test
that reads the path is not attributed. It is left for an owner decision.

## Assertions extended

Fixtures that wrote the old path in a non-Corvint module now write `.corvint/test-commands.json`.
The AFP-V0-825 stale-binary dry runs expect exit 0, because built-in defaults replace the
undeclared INCOMPLETE. The CIN-V0-860 unowned cases expect the `unowned-path` fallback. No frozen
golden was edited.

## Not run

`openspec validate` (the OpenSpec CLI is not installed); a live run of Vitest, Playwright, Jest,
pytest, RSpec or Minitest (none is installed here; stubs answer at the resolved paths).

## Rollback

Revert the change's commits. `corvint test` then reads `.github/cishards/test-commands.json` in
every repository, an adopter without it is INCOMPLETE on FULL again, and `UNOWNED_DIRTY_PATH`
narrows again. RNC-V1-001 to RNC-V1-018 stay listed as retired if the change is withdrawn.

## Round 2

Merged `claude/tivfix` (head `42b0bd8a0`, the inventory lane's round-2 and round-3 proof fixes) with
`git merge --no-ff` as `83cef2594`. Git reported no conflict and no file both lanes touched, so no
resolution was needed; `make spec-requirements` regenerated nothing.

### Review findings fixed

1. **P1-1, the R32 scope.** Orchestrator ruling: a path a Go selection rule attributes is owned
   and must not force FULL; R32's run-everything rule applies only to a path no runner owns. Round
   1 had taken `UNOWNED_DIRTY_PATH` off `gateselect.AllowedUnknown`, added an unconditional
   fallback in `unknownNarrowing` and a FULL reason in `sessiontest.Build`, and dropped the
   qualified proof's own unowned entries. Now the allowlist admits the entry again (origin
   semantics), and `selectPackagesFrom` records which dirty paths are attributed: rule (b) (an
   enclosing package, an embed), rule (c) (a package whose files name it, after the read-scope and
   record bounds), a read-scope declaration that bears on it, the plan's documentation, evidence
   and witnessed selections (the planner's declared readers: an existence reader, an ownership
   obligation), or an unobserved nested module named for the path. An `UNOWNED_DIRTY_PATH` none
   of those attributes falls back with `unowned-path: ...: <path>` in both selector forms. Rule
   (d) attributes nothing, since it selects for every dirty path. `sessiontest.Build` plans FULL
   with that reason itself only when no Go selection ran (no Go universe), and otherwise admits
   the entry under a `run` or `NOTHING` verdict. The qualified proof's drop-and-promote is reverted.
   Fixtures restored against origin semantics: the CIN-V0-860 Makefile case stays FULL, because no
   package in that fixture encloses or names the root Makefile, with an attributed `core/data.txt`
   case beside it; AFP-V0-9013 and AFP-V0-9310 narrow again (their hosted fixtures are enclosed by
   a package); TSP-V0-006's assembly file, AFP-V0-2370's embedded and beside-package data files,
   AZU-V0-9900's named obligation, AFP-V0-980's deleted manifest name and AFP-V0-050's read-scope
   declaration edit narrow again with their original expectations. The `core-freeze` golden
   `affected-full-committed-range.json` was not edited by round 1: it is regenerated only for the
   advice text (friction item 1 below).
   **go.sum does not narrow.** A dirty root `go.mod`, `go.sum`, `go.work` or `go.work.sum` hits the
   older root-module fallback of `selectPackagesFrom` before any attribution, as it did before the
   lane. The new test pins that it is that fallback and never `unowned-path`. Narrowing a go.sum
   change needs that rule lifted, which R32 does not cover.
2. **P1-2, abnormal Go packages.** Observed with go1.27.3 `go test -json`: a panic fails the test
   that panicked, then the package, and the later tests never appear. A timeout fails the package
   while the running test has no terminal event. `Parse` now always emits the package-level fail
   when a package ends with fail. It emits a package-level `error`, with an `error` result for every
   started test never ended, when the package fails after a panic, ends with a started test still
   open, or never ends. `Reconcile` treats an error coarser than a test as an unfinished unit, so the
   run is incomplete.
3. **P2-1.** `Reconcile` keeps the most severe duplicate (error, fail, pass, skip, NOT_RUN), and
   records `Duplicates` and `Exact: false`. It also records `Failed`.
4. **P2-2.** Jest-shaped reports: a `testResults` entry with status `failed` and no failing
   assertion is a file-level fail, and `success: false` with nothing failing returns a
   `*ReportError` run-level failure. Playwright top-level `errors` return the same.
   `ReconcileParsed` keeps the results and sets `Failed`.
5. **P2-3.** A Go line starting with `{` that does not unmarshal is counted and returned as
   `*ReportError{Malformed}` with the results read. Other lines (build text) are still skipped.
6. **P2-4.** The canonical-test-id spec does not forbid a leading `-` (CTI `CleanFile` accepts
   it), so only argv changed. Vitest and Playwright use the file filter with leading dashes
   dropped: a substring still matching the file, whose wider match the listing check covers. A
   dash-only name runs the full command. Jest, pytest and RSpec pass `./path`. An empty listing is
   no listing (`exact: false`). `--` was not used: Vitest's argument parser puts words after `--`
   into its `--` option, not its filters.
7. **P2-5.** `splitGoTest` models the go build and test flags with their values. It runs the full
   command with `exact: false` for `-args`, `--`, an unknown flag or a value flag with no value. The
   full command now keeps its declared package patterns and adds `-json`.
8. **P3.** The proof-graph drop at `qualified.go` ran only after `selectPackagesFrom` had passed the
   planner's own unknowns (its fallback returns before the proof), so it was not reachable without
   the planner's check. It is reverted with the allowlist: the Go-only proof's unowned entries pass
   `AllowedUnknown` as before the lane. A receipt that omits an unowned entry its own planner would
   emit can still narrow. The qualified selector already trusts the receipt for every
   other-language claim, so this adds no new trust. No fail-closed change was made.

Every new or restored test for a finding was run on `8b0c6ec70` in a detached worktree (removed
afterwards). All fail there with rc=1 except the two go.sum subtests, which pin unchanged origin
behaviour. `TestRNCV1006RunRecordsFailureIncompletenessAndDuplicates` does not compile there,
because `Run.Failed`, `Run.Exact`, `Run.Duplicates` and `ReconcileParsed` are new.

### Tasks

- Task 5: the contract now selects and parses Jest (`--runTestsByPath`, `--json`), pytest (node
  IDs, pytest-json-report) and RSpec (spec files, `--format json`, file-level results), and
  `Kind` reports `e2e` for Playwright (RNC-V1-007). Not ticked: `corvint test` still runs narrowed
  selections through the STR-V0 adapters and `corvint-js-test-provider`, so there is no Jest
  end-to-end run (RNC-V1-010), no receipt with per-test results (RNC-V1-005 scenario) and no plan
  kind signal. Minitest stays full-only, because its per-test results come from Corvint's Ruby hook,
  which the command must load. The hook's `result.error?` split (design decision 12) is not made.
- Task 6: Corvint's commands are now also at `.corvint/test-commands.json`, which `corvint test`
  reads first. `tools/test-commands-check` fails unless the two files are byte-identical. Remaining
  step, which edits `.github/`: point the Makefile, `tools/test-commands-check`,
  `tools/test-commands-run` and the workflows at the new file, delete
  `.github/cishards/test-commands.json`, and after one release delete the old-path branch of
  `sessiontest.ReadDeclared`.
- Task 7: not built. No NMS-V1-008 parser exists on the merged tree (`git grep NMS-V1-008 -- '*.go'`
  is empty).
- Task 10: not built. The six paths have no outside Go importer. Deleting them still means
  rewriting the RSP-V0 traceability of 13 requirements, the generated requirement links, the
  documentation-path generator, the capability catalog and rows, the read scopes, runtime readers,
  import ratchet and ticket vocabulary. `.github/linecov/qualification.json` names two of the
  packages, and this lane does not edit `.github/`. The change also overlaps `source-removal` B3
  (RMV-V1-010, proposed). Left for a lane that can run `make doc-gates`.
- Task 11: not built. No runner's contract is yet what `corvint test` executes for a narrowed
  selection, so retiring `corvint-js-test-provider` would remove Vitest and Playwright narrowing.
  `.github/cishards/walltime_test.go` names the Go provider.
- Task 12: not built. No support table or per-runner release-manifest fields exist yet. The shell
  full-selection fallback changes `corvint affected` output, which is frozen Core.

### Friction fixed

- `FOCUSED_TEST_COMMAND_UNDECLARED` now names `.corvint/test-commands.json` and the built-in
  defaults `corvint test` would plan with. ACX-V0-853 records the amendment, and both core-freeze
  goldens were regenerated with `CORVINT_UPDATE_GOLDEN=1`.
- `inventory-invalid-arguments` is named in the test-inventory spec's refresh surface.

### Not run

`openspec validate` (the CLI is not installed); a live run of Jest, pytest, RSpec, Vitest or
Playwright (none is installed; the flags are from public documentation, NOT_VERIFIED);
`make doc-gates`, dogfood-check and seal (decision 0594, lane rules).
