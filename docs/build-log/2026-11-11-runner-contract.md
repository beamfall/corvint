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
- a deleted manifest name (AFP-V0-980) and a read-scope declaration edit (AFP-V0-050): these
  fixtures now declare the path, as this repository does (documentation, runtime reader), so their
  narrowing is still proven.

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
