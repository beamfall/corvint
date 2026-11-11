# 2026-11-11: test inventory (V1-5406)

Change `test-inventory`: tasks 1 to 6 and 8 are built, and task 7 (the GitHub Action refresh step) is not.
Requirements TIV-V1-001 to TIV-V1-012 are in `docs/specs/test-inventory-v1.md`. This is the second change of the
R38 testing order. It builds on `canonical-test-id` (V1-5398, ctid lane head `9ff47e301`), and
`internal/testid` is unchanged.

## What changed

- `internal/testinventory` (tests layer; one prefix line in `tools/import-ratchet/areas.json`)
  holds the `corvint-test-inventory/0` schema, the canonical writer and reader, the tree readers
  (commit tree and working tree, by Git blob ID), detection of the runner instances, the seven
  listers, the reader rules and the refresh with its drift and size refusals.
- `corvint inventory refresh [--runner RUNNER] [--root DIR]` is the only command that writes
  `.corvint/test-inventory.json`. Its help, the root help line, the catalog rows, the
  `--require-clean` ratchet form and the MCP `corvint.help` command enum (the tools/list goldens)
  name it.
- The readers are `affected --summary` (`affected-summary/1` `inventory`, which `corvint.affected` also returns),
  `corvint test` (`corvint-test: INVENTORY`, the `inventory:` summary line and the dry-run JSON
  `inventory`) and `corvint watch` (the `Inventory` member of each plan event).
- The spec records Surfaces, Traceability and the acceptance-log row `test-inventory` (mode
  `auto`).

## Differences from the proposal, followed by intent

1. **Declared instances.** No file declares runner instances. `.github/cishards/test-commands.json`
   declares one full command per language and names no root or config. A tree's declared
   instances are therefore the ones its config files declare (`Detect`): one per runner and root,
   the design's `<runner>@<root>` key, with the first config name in a fixed precedence. Refresh
   and every reader use the same static detection. Not detected: a Jest config inside
   `package.json`, a Vitest config inside `vite.config.*` or a workspace file, and any config
   under `node_modules`, `vendor`, `testdata` or a hidden directory. Corvint's own fixture trees
   live under `testdata`, and a refresh must never run their executing listers. A second config
   of one runner in the same root is not a second instance. Any test that is not detected is
   simply absent from the inventory. No selection reads the inventory yet, so nothing narrows.
2. **Where the plan prints its reasons.** The `affected-plan/0` and `affected-plan/1` documents
   are frozen Core output (CCF-V1-006), as the ctid lane recorded. The verdict is therefore in
   `affected-summary/1` and its MCP result, in `corvint test`'s planning output and in watch
   events. "Runs its declared full command" is expressed as `full: true` with the reason. Selecting
   from the inventory belongs to `e2e-inventory-discovery` and `runner-contract`, so no plan
   changes its selection here.
3. **Reader tree.** `affected` judges against HEAD's tree for a based plan, and against the
   working tree for `--worktree` or a plan with no base (the dirty-worktree plan). `corvint test`
   judges at HEAD. `watch` judges the working tree on every save.
4. **TIV-V1-007 "inventory profile".** The file has one profile, and an unknown profile is
   `inventory-invalid` for the whole file (TIV-V1-009). A record whose lister name, version or
   kind differs from this binary's is `inventory-stale`. An oversize file on read is
   `inventory-invalid`, because it cannot be read within the bound. `inventory-too-large` is the
   refresh refusal.
5. **`CORVINT_SELECTION`.** Change `narrowing-measurement` owns the variable's grammar and
   refusal, and no code parses it yet. `Verdict.ForceFull(runner)` makes the per-runner rule hold
   for every instance, so the caller that parses `full:<runner>` gets TIV-V1-003 by construction.
6. **Lister granularity.** Jest is listed at file level, with `test-id-unmapped`, because the
   change defines no place for a recorded `--json` report. `vitest list --json` joins the title
   path with ` > `, and the lister splits it on that separator. A title that contains ` > ` is
   therefore mis-split. Such an ID can only dangle and widen to its file (CTI-V1-011). RSpec and
   Minitest stay static per RBR-V1-001 (Open 2 is unanswered). Minitest lists methods only for a
   class that derives directly from `Minitest::Test` and uses no construct that can add, inherit
   or rename tests. Any other file is listed at file level.
7. **Playwright reuse.** The lister converts its listing with
   `typescript.PlaywrightDiscoveryFromList`, the producer behind `corvint affected discovery`, and
   records the receipt's profile and source digest. It does not apply `affected discovery`'s hold
   on ignored paths. The selhard lane owns that package, which is called here and not edited.
8. **Drift.** The source digest is SHA-256 over every working-tree file Git lists (tracked, and
   untracked but not ignored) with its blob ID, the inventory itself excluded. It is taken before
   the first lister and compared after each one, and HEAD is read after the digest. An executing
   lister that writes an untracked file Git does not ignore is refused as drift. pytest therefore
   runs with `-p no:cacheprovider` and `PYTHONDONTWRITEBYTECODE=1`.
9. **Open 4 (R31).** An inventory-only change is still an unowned dirty path for `affected`
   (`UNOWNED_DIRTY_PATH`, scope UNKNOWN). It widens and never narrows. Exempting the derived file
   would narrow plans, so this change leaves that to an owner decision.
10. **Executing listers in tests.** Node is installed on this host, but Vitest, Jest, Playwright
    and pytest are not. The executing-lister fixtures use stub executables at the paths the listers
    resolve (`node_modules/.bin/*`, `.venv/bin/python`). Each stub refuses any argv but the expected
    one and prints the tool's documented output shape. A live run of each tool is NOT_RUN.
11. **No committed inventory for Corvint.** TIV-V1-001 is a contract for adopting repositories.
    Committing an inventory for this repository is out of scope. A measured static refresh of the
    root Go instance here (`--runner go --root .`, file deleted afterwards) listed 9584 tests in
    2418 files as 1,991,507 bytes in 0.54 s, a quarter of the 8 MiB cap. `affected --summary` on
    this tree reports seven declared instances, all `inventory-absent`.

## Assertions extended

Every plan now prints its inventory line, so five exact expectations name the absent
inventory of their fixtures:
`TestSTRV0018CLIDryRunLaunchesNoRunner` and `TestSTRV0008CLIBalancedShards`
(`session_test_command_unix_test.go`), `TestSTRV0064CLINestedModuleRuns`,
`TestSTRV0063DryRunSlicesTestOnlyChanges` and `TestAFPV0872AffectedSummaryVerdict` (the text
verdict). `TestRequireCleanHonouredOrRefusedByEveryCommand` gains the new verb's form. The three
MCP tools/list goldens were regenerated for the `corvint.help` command enum. No frozen golden was
edited.

## Rollback

Revert the change's commits and delete `.corvint/test-inventory.json`. Readers then see an
absent inventory and run each instance in full with `inventory-absent`. TIV-V1-001 to TIV-V1-012
stay listed as retired if the change is withdrawn.
