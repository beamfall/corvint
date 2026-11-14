# Spec delta: selection-hardening

The lane copies each statement below into `selection-hardening-v1.md`, inside its
`## Requirements` section, as a numbered definition line that begins
`(proposed 2026-11-09; delivery not-started)`.

## ADDED Requirements

### Requirement: SLF-V1-001 Python import roots from configuration and layout

The Python plugin MUST resolve absolute imports against these roots: pytest `rootdir`, each
configured `pythonpath` entry, the nearest non-package ancestor of each `testpaths` entry and of
each source file, and the fixed roots `src`, `lib` and `python`. Adding a root MUST only add
aliases and MUST NOT remove an edge that the fixed roots produce. This rule MUST NOT discharge an
open frontier until SLF-V1-006 passes for it. (PROD-TEST-002)

#### Scenario: Configured pythonpath

- **GIVEN** `pyproject.toml` sets `pythonpath = ["app"]` and `tests/test_x.py` imports `billing`
  from `app/billing.py`
- **WHEN** `app/billing.py` changes and `corvint affected` runs
- **THEN** `tests/test_x.py` is selected

### Requirement: SLF-V1-002 Unresolved in-repository absolute import

When an absolute import's first dotted component names a directory or `.py` file in the tracked
tree and the import resolves to no unit, the Python plugin MUST raise the frontier
`python:absolute-import-unresolved`, declared OUTBOUND at the importing file. The plan MUST then
select each unit that bears the importing file or report UNKNOWN. A changed module in the same
project MUST NOT be reported as Uncovered while such a frontier is open; it MUST be reported as
unknown `NO_SELECTABLE_TEST`. An import whose first component names nothing in the tree is
third-party and raises nothing. (PROD-TEST-002)

#### Scenario: Import from an unconfigured root

- **GIVEN** `tests/test_y.py` imports `core.rates` and `core/` sits under `services/`, which no
  root names
- **WHEN** `services/core/rates.py` changes and `corvint affected` runs
- **THEN** the plan names `python:absolute-import-unresolved` at `tests/test_y.py` and either
  selects `tests/test_y.py` or is UNKNOWN, and `services/core/rates.py` is not listed as Uncovered

### Requirement: SLF-V1-003 Glob imports

The TypeScript plugin MUST expand `import.meta.glob` and `require.context` calls whose arguments
are literals against the tracked tree at the plan's revision, create one edge per matching file,
and add a membership reader on the pattern to the importing file. A call with any computed
argument, or whose expansion exceeds the plan's path budget, MUST raise `typescript:dynamic-import`.
(PROD-TEST-002)

#### Scenario: Literal glob

- **GIVEN** `src/routes.ts` calls `import.meta.glob("./pages/*.tsx")` and a test imports
  `src/routes.ts`
- **WHEN** `src/pages/about.tsx` changes
- **THEN** the test is selected

#### Scenario: New file matching a glob

- **GIVEN** the same project
- **WHEN** `src/pages/contact.tsx` is added
- **THEN** the test is selected through the membership reader

#### Scenario: Computed glob

- **GIVEN** `import.meta.glob(base + "/*.ts")`
- **WHEN** the plan is built
- **THEN** it names `typescript:dynamic-import` at the calling file

### Requirement: SLF-V1-004 Manual mocks

In a project whose runner is Jest or Vitest, the TypeScript plugin MUST add an edge from each
importer of module `m` to the file under the `__mocks__/` directory adjacent to `m`, and from each
importer of bare package `p` to the root `__mocks__/p` file. A literal `jest.mock(x)` or
`vi.mock(x)` call MUST add an edge from the calling file to `x` and to its `__mocks__` sibling. A
call with a computed target MUST raise `typescript:mock-target-unresolved`, declared DISCOVERY at
the calling file. A changed mock file with no modelled consumer MUST be reported as unknown
`NO_SELECTABLE_TEST`, never as Uncovered. (PROD-TEST-002)

#### Scenario: Adjacent manual mock

- **GIVEN** `src/__mocks__/api.ts` and a test that imports `src/api.ts` under Jest
- **WHEN** `src/__mocks__/api.ts` changes
- **THEN** the test is selected

#### Scenario: Computed mock target

- **GIVEN** a test calls `vi.mock(pathFor("db"))`
- **WHEN** the plan is built
- **THEN** it names `typescript:mock-target-unresolved` at that test file

### Requirement: SLF-V1-005 Literal Vitest and Jest configuration

The TypeScript plugin MUST evaluate a Vitest or Jest config in which every recognised marker has a
literal value (string, array of strings, regex literal, or object of those) for the keys
`include`, `exclude`, `testMatch`, `testRegex`, `testDir`, `setupFiles`, `globalSetup`, Jest
`roots`, `testPathIgnorePatterns`, `moduleNameMapper`, Vitest `dir`, `root` and literal `projects`
paths. It MUST drop `typescript:executable-config-unresolved` only when every recognised marker in
the file was evaluated. A spread, call, identifier or import in a marker value, or any recognised
marker outside that list, MUST keep the frontier. The recognised marker list MUST NOT be narrower
than `dynamicConfigMarkers` at 9ec3a29d0 and MUST also contain the key-shaped markers `roots:`,
`testPathIgnorePatterns:`, `moduleNameMapper:`, `dir:` and `root:` (R42 (panel pick) A6). A dirty
config file MUST still run its runner
instance in full (NCM-V1-005). This rule MUST NOT discharge the frontier until SLF-V1-006 passes
for it. (PROD-TEST-016, PROD-TEST-002)

#### Scenario: Literal include narrows

- **GIVEN** `vitest.config.js` is `include: ["src/**/*.test.ts"]` and is clean
- **WHEN** `src/math.ts` changes
- **THEN** the plan selects the tests that reach `src/math.ts` and does not name
  `typescript:executable-config-unresolved`

#### Scenario: Jest roots is a recognised marker

- **GIVEN** a clean `jest.config.js` whose only marker is `roots: [process.env.SRC]`
- **WHEN** `src/math.ts` changes
- **THEN** the plan names `typescript:executable-config-unresolved` and keeps the frontier

#### Scenario: Computed value keeps the frontier

- **GIVEN** `include: [process.env.SUITE]`
- **WHEN** any TypeScript file below the config changes
- **THEN** the plan names `typescript:executable-config-unresolved`

#### Scenario: Dirty config

- **GIVEN** a literal config
- **WHEN** the config file itself changes
- **THEN** that runner instance runs in full

### Requirement: SLF-V1-006 Fault injection before a new rule narrows

Each rule that removes tests from a plan (SLF-V1-001, SLF-V1-003, SLF-V1-005) MUST stay disabled,
leaving its frontier open, until at least five pre-registered FIH-V1 faults for that rule in a
pinned proof repository are each killed by the tests the rule selects. A miss MUST keep the rule
disabled. (PROD-TEST-014)

#### Scenario: Rule held until proved

- **GIVEN** SLF-V1-005 is built and its FIH-V1 report has one surviving fault
- **WHEN** a TypeScript file below a literal config changes
- **THEN** the plan still names `typescript:executable-config-unresolved`

### Requirement: SLF-V1-007 One verdict and one effect per unknown

A single function MUST map each plan unknown to one effect, `ignored`, `adds-suite:<runner>` or
`forces-full`, and each runner to one verdict on the R16 tier vocabulary, using the allowlist that
`corvint test` applies after owner decision R32. `corvint affected`, `corvint test` and the MCP plan
tool MUST print that function's output and MUST NOT compute their own. Every unknown MUST carry a
non-empty hint. An unknown code the function does not know MUST map to `forces-full`.
(PROD-TEST-001, PROD-TEST-009)

#### Scenario: Same plan, same verdict

- **GIVEN** a change that yields unknown `NO_SELECTABLE_TEST` and a TypeScript DISCOVERY frontier
- **WHEN** `corvint affected`, the `corvint test` plan step and the MCP plan tool run on it
- **THEN** all three report the same effect for each unknown and the same verdict for each runner

### Requirement: SLF-V1-008 Test and driver counts reported separately

The `corvint affected` stderr headline MUST report the number of selected test units and the
number of selected driver units as separate values, `tests=<n> drivers=<m>`, placed where the
headline's line bound (AFP-V0-3642) cannot cut them. Driver units MUST NOT be counted as tests. The
headline MUST list driver units only in verbose output. The `summary` object of the affected-plan/1
document MUST carry the same two counts as the members `tests` and `drivers`, optional additions
under CCF-V1-006, leaving `summary.line` unchanged. (PROD-TEST-015)

#### Scenario: Driver units excluded from the test count

- **GIVEN** a plan with three selected test units and two driver units
- **WHEN** `corvint affected` prints its stderr headline
- **THEN** it shows `tests=3 drivers=2`
