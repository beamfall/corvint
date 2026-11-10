# Design: selection-hardening

Status: proposed (2026-11-09). Requirement text lives in `specs/selection-hardening/spec.md`.

## Decisions

1. Python import roots come from, in order: pytest `rootdir`, each `pythonpath` entry, each
   `testpaths` entry's nearest non-package ancestor, the nearest non-package ancestor of each
   source file, then the fixed `src`, `lib` and `python`. The configuration search order is the one
   `affected_pytest.go:48` already uses. Roots only add aliases; they never remove one.
2. An absolute import is in-repository when its first dotted component names a directory or `.py`
   file anywhere in the tracked tree. An in-repository import that still resolves to no unit
   raises `python:absolute-import-unresolved`. An import whose first component names nothing in
   the tree is treated as third-party, as today.
3. The Python plugin declares a frontier direction for the new frontier: OUTBOUND at the
   importing file. Site closure then seeds the importing unit (`frontier_closure.go:280-300`), so
   the importer's tests are selected or the plan is UNKNOWN. The existing Python frontiers keep
   their INBOUND default.
4. A changed Python or TypeScript module with no modelled consumer, inside a project that has a
   new-rule frontier open, is reported as unknown `NO_SELECTABLE_TEST`, never as Uncovered.
5. Glob imports: `import.meta.glob` and `require.context` with literal arguments expand against
   the tracked tree at the plan's revision and create one edge per match. The importing file also
   gets a membership reader on the glob, so adding or removing a matching file reselects it. Any
   computed argument raises the existing `typescript:dynamic-import`.
6. Manual mocks: a file under `__mocks__/` adjacent to module `m` gets an edge from every importer
   of `m` in a project whose runner is Jest or Vitest. A root `__mocks__/<pkg>` gets an edge from
   every importer of bare package `<pkg>`. A literal `jest.mock(x)` or `vi.mock(x)` adds an edge
   from the test to `x` and to its `__mocks__` sibling. A computed target raises
   `typescript:mock-target-unresolved` (new, DISCOVERY at the calling file).
7. Literal config: for Vitest and Jest, a config whose recognised markers all have literal values
   (string, array of strings, regex literal, or object of those) is evaluated like Playwright's
   today. Evaluated keys: `include`, `exclude`, `testMatch`, `testRegex`, `testDir`, `setupFiles`,
   `globalSetup`, Jest `roots`, `testPathIgnorePatterns`, `moduleNameMapper`, Vitest `dir`, `root`,
   `projects` (literal paths only). Any other recognised marker, a spread, a function call, an
   identifier or an import in a marker value keeps the frontier. The marker list is never narrowed
   by this change; it gains those five keys, key-shaped (for example `roots:`), per R42 (panel
   pick) A6.
8. A dirty config file still runs its runner instance in full (NCM-V1-005). Evaluation only helps
   when the config is clean and another file changed.
9. Each narrowing rule (decisions 1, 5, 7) ships disabled until its FIH-V1 cases pass: at least
   five faults per rule in a pinned proof repository, each killed by the selected tests. Until
   then the rule's frontier stays open.
10. One function in `internal/sessiontest` (the one `corvint test` uses after R32 removes
    UNOWNED_DIRTY_PATH from the allowlist) maps each plan unknown to an effect: `ignored`,
    `adds-suite:<runner>` or `forces-full`, and each runner to a verdict on the R16 tier
    vocabulary. `corvint affected`, `corvint test` and the MCP plan tool print its output; none
    computes its own. Every unknown carries a non-empty hint.
11. Counts: the summary prints `tests=<n> drivers=<m>`. Driver units are listed only with
    `--verbose` and in JSON under a separate member.

## Failure modes

- pytest configuration unreadable: decision 1 falls back to the layout roots and the fixed roots;
  the advisory check already reports the read failure.
- Namespace packages without `__init__.py`: decision 1's nearest non-package ancestor can be the
  repository root, which adds aliases but cannot drop an edge.
- Two roots map one dotted name to two files: both edges are kept (wider plan, never narrower).
- Glob expansion exceeds the plan's path budget: the glob is treated as computed and raises
  `typescript:dynamic-import`.
- A mock factory requires another module: the factory body is scanned like any code, so literal
  requires inside it create edges; computed ones raise `typescript:dynamic-import`.
- Literal config evaluation disagrees with the runner's own collection: test-inventory's executing
  lister is the authority; a mismatch is reported as `typescript:config-evaluation-mismatch` and
  the instance runs in full.
- Effect function sees an unknown code it does not know: effect `forces-full`.

## Acceptance evidence

- Go unit tests per requirement in the plugin and planner packages.
- Fixture repositories: a Python flat layout with `pythonpath = ["app"]`; a Vite project using
  `import.meta.glob`; a Jest project with adjacent and root `__mocks__` and a computed
  `jest.mock`; a Vitest project with a literal `include`.
- FIH-V1 reports per narrowing rule (SLF-V1-006).
- NMS-V1 narrowing figures for Vitest, Jest and pytest before and after.
- A parity test asserting identical verdicts from affected, test and MCP on one plan.

## Rollback

Revert the change's commits. Requirement IDs are retired, not reused. The kill switch
`CORVINT_SELECTION=full` (NMS-V1-008) covers a bad selection in the field without a revert.

## Decided questions

1. Decided, R42 (panel pick) A6: Jest `roots`, `testPathIgnorePatterns`, `moduleNameMapper` and
   Vitest `dir`, `root` join `dynamicConfigMarkers` (`typescript.go:2190`), key-shaped, and are
   evaluated as literals under SLF-V1-005.
2. Decided, R42 (panel pick) A8: finding 11 (a Go-only edit may admit the TypeScript plugin and its
   full suite) is settled by running the experiment now. If it is confirmed, the fix is a blocker in
   this change and lands before the Go replay.
