# Selection Hardening V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted (decision 0596, G14; V1-5401)
Delivery status: experimental; partially delivered
Health: KEEP
Authoritative inputs: change folder `docs/specs/openspec/changes/selection-hardening/`
(`proposal.md`, `design.md`, `tasks.md` and `specs/selection-hardening/spec.md`),
`docs/specs/PRODUCT-PLAN-1.0.md` (PROD-TEST-001, PROD-TEST-002, PROD-TEST-009, PROD-TEST-014,
PROD-TEST-015, PROD-TEST-016) and `docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md`.

## Agent digest
- Claim: Python and JavaScript selection raises a named frontier where it drops an edge today; new narrowing rules ship tested but held off until fault injection.
- Status: accepted (decision 0596, G14; V1-5401) / experimental; partially delivered
- Exists: `python:absolute-import-unresolved`, glob and mock edges, the literal-config reader and its gates, the coverage hook in `internal/liveverify/affected`, the selection side of the SLF-V1-006 fault cases, `sessiontest.SelectionVerdictOf` (SLF-V1-007) and the `summary.tests`/`summary.drivers` members (SLF-V1-008).
- Blocked on: the SLF-V1-006 FIH-V1 reports, which need the harness's pytest, Vitest and Jest adapters (FIH-V1-008) and a sealed pre-registration on a pinned proof repository (FIH-V1-001, FIH-V1-009); until then every narrowing gate stays off.
- Read next: Requirements; Narrowing gates; Delivery record.

## User and boundary

This spec is the canonical home of requirements SLF-V1-001 to SLF-V1-008, copied from the change's
`specs/selection-hardening/spec.md`. Decisions, failure modes and rollback stay in the change's
`design.md`, which owns them. The existing rules stay in `affected-plan-python-playwright-v0.md`,
`typescript-javascript-affected-adapter-v0.md` and `affected-plan-selection-v0.md`; this spec
adds requirements and changes none of theirs. The change was written against the integration
checkout `9ec3a29d0`; where `origin/main` differs, the build-log entry
`docs/build-log/2026-11-11-selection-hardening.md` names the difference.

Non-goals are the change's: no Go dependency change, no graph-cache incremental update, no Go
line-selection rework, no per-plan Git spawn change, no one-shot line evidence, no change to which
lockfiles, runner configs or snapshot paths force a full run, and no evaluation of computed
JavaScript configuration.

## Requirements

- `SLF-V1-001`: (accepted, decision 0596 G14; V1-5401) Python import roots from configuration and layout

The Python plugin MUST resolve absolute imports against these roots: pytest `rootdir`, each
configured `pythonpath` entry, the nearest non-package ancestor of each `testpaths` entry and of
each source file, and the fixed roots `src`, `lib` and `python`. Adding a root MUST only add
aliases and MUST NOT remove an edge that the fixed roots produce. This rule MUST NOT discharge an
open frontier until SLF-V1-006 passes for it. (PROD-TEST-002)

Built, held disabled (Narrowing gates). `rootdir` is the directory of each pytest configuration
file (`pytest.ini`, `.pytest.ini`, `pyproject.toml` with `[tool.pytest.ini_options]`, `tox.ini`
with `[pytest]`, `setup.cfg` with `[tool:pytest]`). With the gate on, those files are resolution
inputs of the plugin, so a dirty one keeps every Python frontier entry.

**Scenario: Configured pythonpath**
- GIVEN `pyproject.toml` sets `pythonpath = ["app"]` and `tests/test_x.py` imports `billing`
  from `app/billing.py`
- WHEN `app/billing.py` changes and `corvint affected` runs
- THEN `tests/test_x.py` is selected

- `SLF-V1-002`: (accepted, decision 0596 G14; V1-5401) Unresolved in-repository absolute import

When an absolute import's first dotted component names a directory or `.py` file in the tracked
tree and the import resolves to no unit, the Python plugin MUST raise the frontier
`python:absolute-import-unresolved`, declared OUTBOUND at the importing file. The plan MUST then
select each unit that bears the importing file or report UNKNOWN. A changed module in the same
project MUST NOT be reported as Uncovered while such a frontier is open; it MUST be reported as
unknown `NO_SELECTABLE_TEST`. An import whose first component names nothing in the tree is
third-party and raises nothing. (PROD-TEST-002)

Built and on. The plugin reads only `.py` files, so "names a directory or `.py` file" is decided
over the directories that hold a tracked `.py` file and the `.py` file stems: a directory with
no Python source below it cannot satisfy a Python import. Each site is the importing file; at
most 16 sites are named per plan and one `python:absolute-import-unresolved:+N` entry counts the
rest, every site being recorded for the graph. The reason is source-bound (it binds only a `.py`
path the plugin owns). The never-Uncovered rule is the `affected.CoverageUnproven` hook.

**Scenario: Import from an unconfigured root**
- GIVEN `tests/test_y.py` imports `core.rates` and `core/` sits under `services/`, which no
  root names
- WHEN `services/core/rates.py` changes and `corvint affected` runs
- THEN the plan names `python:absolute-import-unresolved` at `tests/test_y.py` and either
  selects `tests/test_y.py` or is UNKNOWN, and `services/core/rates.py` is not listed as Uncovered

- `SLF-V1-003`: (accepted, decision 0596 G14; V1-5401) Glob imports

The TypeScript plugin MUST expand `import.meta.glob` and `require.context` calls whose arguments
are literals against the tracked tree at the plan's revision, create one edge per matching file,
and add a membership reader on the pattern to the importing file. A call with any computed
argument, or whose expansion exceeds the plan's path budget, MUST raise `typescript:dynamic-import`.
(PROD-TEST-002)

Built, expansion held disabled (Narrowing gates). Before this change both calls were read as no
import at all. Now every call raises `typescript:dynamic-import` at its line unless the gate is
on and its arguments are literal. The path budget is 1024 matched files per call. A literal
pattern is relative (`./` or `../`); a root-relative, aliased or negated pattern, a `base` option,
a `require.context` directory that is not a string or a mode or regular expression the planner
cannot read counts as computed. The membership reader is the pattern itself as an anchored
`PathGlobs` entry, or its literal directory when the pattern holds `**`, an extglob, a negated
class (`[!x]`, which `path.Match` would read as the characters `!` and `x`) or a POSIX class.
A brace range such as `{1..3}` or a POSIX class such as `[[:alpha:]]` counts as computed, since
reading it as literal text would narrow without notice. A non-recursive `require.context` at the
repository root reads `/*`, which every path matches, so a new file at the root reselects the
importer.

**Scenario: Literal glob**
- GIVEN `src/routes.ts` calls `import.meta.glob("./pages/*.tsx")` and a test imports
  `src/routes.ts`
- WHEN `src/pages/about.tsx` changes
- THEN the test is selected

**Scenario: New file matching a glob**
- GIVEN the same project
- WHEN `src/pages/contact.tsx` is added
- THEN the test is selected through the membership reader

**Scenario: Computed glob**
- GIVEN `import.meta.glob(base + "/*.ts")`
- WHEN the plan is built
- THEN it names `typescript:dynamic-import` at the calling file

- `SLF-V1-004`: (accepted, decision 0596 G14; V1-5401) Manual mocks

In a project whose runner is Jest or Vitest, the TypeScript plugin MUST add an edge from each
importer of module `m` to the file under the `__mocks__/` directory adjacent to `m`, and from each
importer of bare package `p` to the root `__mocks__/p` file. A literal `jest.mock(x)` or
`vi.mock(x)` call MUST add an edge from the calling file to `x` and to its `__mocks__` sibling. A
call with a computed target MUST raise `typescript:mock-target-unresolved`, declared DISCOVERY at
the calling file. A changed mock file with no modelled consumer MUST be reported as unknown
`NO_SELECTABLE_TEST`, never as Uncovered. (PROD-TEST-002)

Built and on: every rule here only adds edges. The plugin adds the mock edges whatever the
runner, which can only widen a plan: design decision 6 limits them to Jest and Vitest projects,
and a project under another runner gains edges it may not need, never loses one. A root
`__mocks__/p` is looked up in the importing file's directory and every ancestor. `jest.doMock`,
`jest.setMock`, `jest.unstable_mockModule`, `jest.requireMock`, `vi.doMock` and `vi.importMock`
name their module as `jest.mock` does and are read the same way: a literal target is an edge, a
computed one raises `typescript:mock-target-unresolved`.

**Scenario: Adjacent manual mock**
- GIVEN `src/__mocks__/api.ts` and a test that imports `src/api.ts` under Jest
- WHEN `src/__mocks__/api.ts` changes
- THEN the test is selected

**Scenario: Computed mock target**
- GIVEN a test calls `vi.mock(pathFor("db"))`
- WHEN the plan is built
- THEN it names `typescript:mock-target-unresolved` at that test file

- `SLF-V1-005`: (accepted, decision 0596 G14; V1-5401) Literal Vitest and Jest configuration

The TypeScript plugin MUST evaluate a Vitest or Jest config in which every recognised marker has a
literal value (string, array of strings, regex literal, or object of those) for the keys
`include`, `exclude`, `testMatch`, `testRegex`, `testDir`, `setupFiles`, `globalSetup`, Jest
`roots`, `testPathIgnorePatterns`, `moduleNameMapper`, Vitest `dir`, `root` and literal `projects`
paths. It MUST drop `typescript:executable-config-unresolved` only when every recognised marker in
the file was evaluated. A spread, call, identifier or import in a marker value, or any recognised
marker outside that list, MUST keep the frontier. The recognised marker list MUST NOT be narrower
than `dynamicConfigMarkers` at 9ec3a29d0 and MUST also contain the key-shaped markers `roots:`,
`testPathIgnorePatterns:`, `moduleNameMapper:`, `dir:` and `root:` (R42 (panel pick) A6). A dirty
config file MUST still run its runner instance in full (NCM-V1-005). This rule MUST NOT discharge
the frontier until SLF-V1-006 passes for it. (PROD-TEST-016, PROD-TEST-002)

Built, evaluation held disabled (Narrowing gates); the five key-shaped markers are on. Evaluation
only widens what the plugin already models: a file a literal membership pattern (`test.include`,
`testMatch`, `testRegex`) matches below the config becomes a test, and every test of the config
gains an edge to each literal `setupFiles`, `setupFilesAfterEnv`, `globalSetup` and capture-free
`moduleNameMapper` target. `exclude` and `testPathIgnorePatterns` remove nothing. The planner is
stricter than the design in four places, each keeping the frontier: a file with any spread or a
relative import or `require`; a negated (`!(`) glob; a marker in an object other than the known
Vitest and Jest ones; and a capturing `moduleNameMapper` entry whose key could name an npm
package. A membership key is read only under a known parent (`test`, `typecheck` or a Jest
config object): a top-level `include`, which Vitest ignores, keeps the frontier. The dirty-config
rule holds through the existing edge from each test to its nearest config.

**Scenario: Literal include narrows**
- GIVEN `vitest.config.js` is `include: ["src/**/*.test.ts"]` and is clean
- WHEN `src/math.ts` changes
- THEN the plan selects the tests that reach `src/math.ts` and does not name
  `typescript:executable-config-unresolved`

**Scenario: Jest roots is a recognised marker**
- GIVEN a clean `jest.config.js` whose only marker is `roots: [process.env.SRC]`
- WHEN `src/math.ts` changes
- THEN the plan names `typescript:executable-config-unresolved` and keeps the frontier

**Scenario: Computed value keeps the frontier**
- GIVEN `include: [process.env.SUITE]`
- WHEN any TypeScript file below the config changes
- THEN the plan names `typescript:executable-config-unresolved`

**Scenario: Dirty config**
- GIVEN a literal config
- WHEN the config file itself changes
- THEN that runner instance runs in full

- `SLF-V1-006`: (accepted, decision 0596 G14; V1-5401) Fault injection before a new rule narrows

Each rule that removes tests from a plan (SLF-V1-001, SLF-V1-003, SLF-V1-005) MUST stay disabled,
leaving its frontier open, until at least five pre-registered FIH-V1 faults for that rule in a
pinned proof repository are each killed by the tests the rule selects. A miss MUST keep the rule
disabled. (PROD-TEST-014)

Partly built (2026-11-13). Each rule has at least five pre-registered faults as Go test cases,
the selection side of an FIH-V1 case: the faulted tree's plan, with the rule's gate on, is BOUNDED
and selects the test that kills the fault, and with the gate held the rule's frontier is raised and
the killer is selected or the plan is UNKNOWN. SLF-V1-001 has six (`pythonpath` in
`pyproject.toml`, `setup.cfg` and `tox.ini`, a namespace package, a layout root, a `testpaths`
root and a transitive edge), SLF-V1-003 six (`import.meta.glob` star, recursive and array forms,
flat and recursive `require.context`, a file reached through a globbed page) and SLF-V1-005 six
(a Vitest include source and a file it reaches, the `setupFiles` and `globalSetup` targets, a Jest
`roots`/`testMatch` source and a `moduleNameMapper` target). The kill itself, the killer failing
three of three runs on the faulted tree, is not measured, so no FIH-V1 report exists for any rule
(NOT_PRODUCED): `tools/fault-harness` has only its Go adapter, and the pytest, Vitest and Jest
adapters of FIH-V1-008 (change `fault-injection-harness` task 6) and a sealed pre-registration on
a pinned proof repository (FIH-V1-001, FIH-V1-009; that change's task 8) do not exist. Each gate
below therefore stays off, which is the held state this requirement asks for.

**Scenario: Rule held until proved**
- GIVEN SLF-V1-005 is built and its FIH-V1 report has one surviving fault
- WHEN a TypeScript file below a literal config changes
- THEN the plan still names `typescript:executable-config-unresolved`

- `SLF-V1-007`: (accepted, decision 0596 G14; V1-5401) One verdict and one effect per unknown

A single function MUST map each plan unknown to one effect, `ignored`, `adds-suite:<runner>` or
`forces-full`, and each runner to one verdict on the R16 tier vocabulary, using the allowlist that
`corvint test` applies after owner decision R32. `corvint affected`, `corvint test` and the MCP plan
tool MUST print that function's output and MUST NOT compute their own. Every unknown MUST carry a
non-empty hint. An unknown code the function does not know MUST map to `forces-full`.
(PROD-TEST-001, PROD-TEST-009)

Built (2026-11-13) on the R32 allowlist that `runner-contract` task 9 delivered.
`sessiontest.SelectionVerdictOf` maps each entry with `decideUnknown`, the decision
`sessiontest.Build` also applies, so `corvint test` cannot run a scope the verdict does not state.
A non-Go `NO_SELECTABLE_TEST` unit and a bearing non-Go frontier add their language's suite
(`adds-suite:<language>`), a nested Go module adds its own suite (`adds-suite:go`; on the ci
profile it is a still-required check and `ignored`), a binary consumer, an allowlisted entry and a
flow-binding entry are `ignored`, and an unattributed `UNOWNED_DIRTY_PATH` and every other entry
are `forces-full`. A runner is named by its plan language namespace, the name an `adds-suite`
effect carries; its tier is `full` when its suite is added or any entry forces full, `line` for Go
when the GLS-V0-020 line evidence narrowed the plan, `flow` for a Playwright selection, and `file`
otherwise, the least narrow of its selections winning. A hint is the plan's own when it has one
and the effect's otherwise. `corvint affected` prints the verdict as the `selectionVerdict` member
of the affected-plan/1 document and of the affected-summary/1 verdict, which the MCP plan tool
returns byte for byte; `corvint test` prints one `SELECTION` line and one `EFFECT` line per entry
on stderr and the `selectionVerdict` member of its dry-run document. Both members are optional
additions under CCF-V1-006. The `--full` (affected-plan/0) document is unchanged.

Limit: whether an `UNOWNED_DIRTY_PATH` is attributed is decided by the AFP-V0-012 Go selector,
which needs the Go index that only `corvint test` builds. `corvint affected` and the MCP tool pass
the attribution as undecided, so there the entry is `forces-full` with a hint saying so, while
`corvint test` reports `ignored` when its Go selector attributed the path. The verdicts agree on
every plan with no attributed unowned path.

**Scenario: Same plan, same verdict**
- GIVEN a change that yields unknown `NO_SELECTABLE_TEST` and a TypeScript DISCOVERY frontier
- WHEN `corvint affected`, the `corvint test` plan step and the MCP plan tool run on it
- THEN all three report the same effect for each unknown and the same verdict for each runner

- `SLF-V1-008`: (accepted, decision 0596 G14; V1-5401) Test and driver counts reported separately

The `corvint affected` stderr headline MUST report the number of selected test units and the
number of selected driver units as separate values, `tests=<n> drivers=<m>`, placed where the
headline's line bound (AFP-V0-3642) cannot cut them. Driver units MUST NOT be counted as tests. The
headline MUST list driver units only in verbose output. The `summary` object of the affected-plan/1
document MUST carry the same two counts as the members `tests` and `drivers`, optional additions
under CCF-V1-006, leaving `summary.line` unchanged. (PROD-TEST-015)

Amended 2026-11-11 by the orchestrator under the owner's delegation (build-log
`docs/build-log/2026-11-11-selection-hardening-counts-decision.md`): the affected-plan/0 and
affected-plan/1 profiles stay frozen for 1.0 (CCF-V1-006), so the counts are required on the
stderr headline and the JSON members move to the next profile version.

Amended again 2026-11-13 (lane charter of V1-5401): CCF-V1-006 classes adding an optional member to
a frozen mode other than the four byte-parity verbs as compatible, as AFP-V0-9987 did for
`summary.unresolvedPackages`, so the JSON members join affected-plan/1 without a new profile and
the two core-freeze goldens gain only these members and `selectionVerdict`.

Built. `corvint affected` counts with `affected.Plan.SelectionCounts`, which
counts a `driver:` unit only as a driver, and prints `tests=<n> drivers=<m>` right after the
summary line's `selected=` total on the headline's first line. That is ahead of the unknown-reason
counts, `revision=` and `base=`, which can push the line past 200 bytes, so a cut line keeps both
counts. The headline lists no driver unit. `summary.tests` and `summary.drivers` come from the same
`SelectionCounts`, so the affected-summary/1 verdict and the MCP plan tool carry them too.

**Scenario: Driver units excluded from the test count**
- GIVEN a plan with three selected test units and two driver units
- WHEN `corvint affected` prints its stderr headline
- THEN it shows `tests=3 drivers=2`

## Narrowing gates

SLF-V1-006 holds each narrowing rule off. Each rule has one Go variable, `false` on `origin/main`,
which is its only enable switch; there is no flag or environment variable, so no run can turn an
unproven rule on:

| Rule | Switch | Off (default) | On |
| --- | --- | --- | --- |
| SLF-V1-001 | `importRootsNarrowing` in `internal/liveverify/affected/python/roots.go` | only the fixed roots resolve; an unresolved in-repository import raises SLF-V1-002's frontier | configured and layout roots add aliases |
| SLF-V1-003 | `globExpansionNarrowing` in `internal/liveverify/affected/typescript/selection_gates.go` | every glob call raises `typescript:dynamic-import` | literal calls expand to edges and a membership reader |
| SLF-V1-005 | `literalConfigNarrowing` in the same file | every Vitest or Jest config with a marker raises `typescript:executable-config-unresolved` | a fully literal config is evaluated and raises nothing |

A rule is enabled by setting its variable to `true` in the commit that retains its passing FIH-V1
report (change task 8). Tests set the variable for one test and restore it.

## Failure modes

The change's `design.md` owns them. Two are not built here: the literal-config mismatch check
against test-inventory's executing lister (`typescript:config-evaluation-mismatch`), because the
static planner runs no runner; and the FIH-V1 reports, which wait for SLF-V1-006.

## Delivery record

Build-log: `docs/build-log/2026-11-11-selection-hardening.md`, the review fixes in
`docs/build-log/2026-11-11-selection-hardening-review-fixes.md` and the orchestrator decision in
`docs/build-log/2026-11-11-selection-hardening-counts-decision.md`. Tests:
`internal/liveverify/affected/python/selection_hardening_test.go`,
`internal/liveverify/affected/typescript/selection_hardening_test.go`,
`internal/liveverify/affected/selection_hardening_test.go`,
`cmd/corvint/affected_counts_test.go`, the SLF-V1-006 fault cases in
`internal/liveverify/affected/python/fault_cases_slfv1006_test.go` and
`internal/liveverify/affected/typescript/fault_cases_slfv1006_test.go`, and the SLF-V1-007 tests in
`internal/sessiontest/effect_slfv1007_test.go` and
`cmd/corvint/selection_verdict_slfv1007_test.go`.

## Rollback

Revert the change's commits. Requirement IDs are retired, not reused. The kill switch
`CORVINT_SELECTION=full` (NMS-V1-008) covers a bad selection in the field without a revert, and
each narrowing rule is off until its gate variable is set.
