# Design: test-inventory

## Decisions

1. **One file.** `.corvint/test-inventory.json`, canonical JSON, profile
   `corvint-test-inventory/0`, committed. It holds one record per runner instance. Each instance
   lists its tests by canonical test ID (CTI-V1-001) with the file each test lives in.
   Source: owner decision R10 (2026-11-09); review T-B5.

2. **One writing verb.** `corvint inventory refresh [--runner R] [--root DIR]` is the only writer.
   It runs the listers, then replaces the file by write-to-temporary and rename. `corvint affected`,
   `corvint test` (planning), `corvint watch` and the MCP tools read it and never write it; watch
   refreshes nothing. The GitHub Action runs `corvint inventory refresh` before planning.
   Source: owner decision R10 (2026-11-09); review T-M4.

3. **Runner instances.** An instance is runner + root directory + config file. The instance key is
   `<runner>@<root>` with the config path recorded beside it. Several instances of one runner are
   allowed. `CORVINT_SELECTION=full:<runner>` (owner decision R13, 2026-11-09) applies to every
   instance of that runner.
   Source: owner decision R15 (2026-11-09); review T-M8.

4. **Listers.**

   | Runner | Lister | Kind | Granularity |
   | --- | --- | --- | --- |
   | Go | parse `_test.go` files with `go/ast` for top-level `Test`, `Fuzz` and `Example` functions, per package | static | test function (subtests appear only in results) |
   | Vitest | `vitest list --json` | executing | test |
   | Jest | `jest --listTests`, then titles from a `--json` report when one is recorded | executing | file, test when titles are known |
   | Playwright | `playwright test --list --reporter=json`, converted to `playwright-discovery/0` | executing | test per project |
   | pytest | `pytest --collect-only -q` | executing | node ID |
   | RSpec | static classification of `_spec.rb` files (RBR-V1-001) | static | file |
   | Minitest | static classification of `_test.rb` files and `test_` methods (RBR-V1-001) | static | method when parseable, else file |

   An `executing` lister runs project code; it runs only inside `corvint inventory refresh`.
   Source: review T-B5; runner-contract listing table. The Ruby rows are open to the owner (Open 2).

5. **Tree binding.** Each instance records the commit, the config file digest, the lockfile digest
   when one exists, the lister name and version, and a blob digest per listed test file.

6. **Staleness.**
   - A test file whose blob digest differs, or a new test file under the instance root, makes that
     file's tests `unassigned`: they always run (review T-M2).
   - A change to the config file, the lockfile, the lister version or the inventory profile makes the
     whole instance `inventory-stale`: every test of that instance runs.
   - No inventory file, or no record for an instance the commands declare, means `inventory-absent`:
     the runner's declared full command runs (review T-M3).
   - An unreadable file, a schema error or an unknown profile means `inventory-invalid`: every test
     of every runner runs, and the reason is printed. The reason name is proposed (Open 3).

7. **Refresh safety.** Refresh records HEAD and the source digest before and after each lister and
   refuses with `inventory-refresh-drift` if either moved. Output is capped (proposed 8 MiB for the
   file, refusal `inventory-too-large`). A lister that fails leaves that instance absent in the new
   file, with the failure recorded, so readers run its full command.

## Failure modes

| Failure | Behaviour |
| --- | --- |
| Inventory file missing | Each runner runs its declared full command, reason `inventory-absent`. |
| Config or lockfile changed since refresh | That instance runs all its tests, reason `inventory-stale`. |
| One test file changed or added | That file's tests run as `unassigned`; other files keep their entries. |
| File malformed or unknown profile | Every test runs, reason `inventory-invalid`. |
| Lister fails during refresh | Instance absent in the new file with the error; readers run its full command. |
| HEAD or sources move during refresh | Refresh refuses with `inventory-refresh-drift`; the old file stays. |
| Runner declared but no instance in the file | That runner runs its full command, reason `inventory-absent`. |
| Runner instance in the file but not declared | Ignored for selection and reported. |
| Duplicate canonical IDs in one listing | Kept with ordinal suffixes per CTI-V1-010. |

## Acceptance evidence

- Unit tests for the inventory reader: absent, stale per cause, per-file `unassigned`, invalid.
- One fixture per runner where `corvint inventory refresh` writes the expected instance record and a
  second run on the same tree writes identical bytes.
- A drift test that changes a source file during an executing lister and sees the refusal.
- A read-only test that runs `corvint affected` and `corvint watch` with an absent and a stale
  inventory and checks that no file under the repository changed (product invariant 4).

## Rollback

Revert the change's commits and delete `.corvint/test-inventory.json`. Readers then see an absent
inventory and run each runner's full command with `inventory-absent`, which is the safe fallback.
TIV-V1-001 to TIV-V1-012 stay listed as retired if the change is withdrawn.

## Open

1. The file name `.corvint/test-inventory.json` is proposed, not decided.
2. Ruby listers are static per RBR-V1-001; the owner has not answered whether that stays.
3. The reason name `inventory-invalid` is proposed.
4. Whether an inventory-only change counts as a `.corvint` config change that forces a full run
   under owner decision R31.
5. Split of text with `e2e-inventory-discovery`: this change owns the file and listers; that change
   owns E2E selection. Any overlap found during the lane is resolved in favour of this change for the
   file and that change for selection.
