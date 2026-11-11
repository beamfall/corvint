# Change proposal: close Python and JavaScript selection misses and give one verdict per plan

Change name: `selection-hardening`
Ticket: V1-5401 (milestone `v1-0`)
Product requirement served: PROD-TEST-001, PROD-TEST-002, PROD-TEST-009, PROD-TEST-014,
PROD-TEST-015 and PROD-TEST-016 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/selection-hardening-v1.md`, a new spec with its own prefix
`SLF-V1`; existing rules stay in `affected-plan-python-playwright-v0.md`,
`typescript-javascript-affected-adapter-v0.md` and `affected-plan-selection-v0.md`
Requirement IDs this change adds: SLF-V1-001 to SLF-V1-008
Owner answers this change implements: G3, G4 (`owner-answers-2026-11-09.md`), and owner decisions
R16 and R32 (2026-11-09)
Intent status: accepted automatically under G14 (decision 0596; logged 2026-11-11), since it only
implements accepted plan requirements
Delivery status: in delivery (V1-5401); see `docs/specs/selection-hardening-v1.md`
Written before dispatch: yes. Lane `selhard` builds tasks 2 to 6a and 9 (2026-11-11).
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/selection-hardening/spec.md`).

## Why

A review of the affected planner found three ways a required test is left out or a narrowable plan
is forced to full, and two reporting faults. The verdicts are in
`scratchpad/tricks/selection.verdicts.md`. PROD-TEST-002 forbids leaving out a required test with
no unknown. PROD-TEST-016 (G4) requires Vitest, Jest and pytest to narrow at least 60% of replayed
PRs.

### What exists today (verified in `integ` at 9ec3a29d0)

- Python import roots are fixed to `src`, `lib` and `python`
  (`internal/liveverify/affected/python/python.go:43`). An absolute import that does not resolve
  under them is dropped without a frontier; only an unresolved relative import raises one
  (`python.go:186-207`). The configured pytest `pythonpath` is read
  (`cmd/corvint/affected_pytest.go:273-298`, `:663-719`) but used only for the advisory command
  (`:137-139`). A changed module whose tests import it from another root is then untested, and
  `coverageGap` lists it as Uncovered, which does not widen the plan
  (`internal/liveverify/affected/uncovered.go`, `select.go:395-410`).
- The TypeScript plugin reads static imports, literal `require` calls and reference paths
  (`internal/liveverify/affected/typescript/typescript.go:1079-1120`, `:1310`). It has no rule for
  `import.meta.glob`, `require.context`, `__mocks__` directories, `jest.mock` or `vi.mock`.
- Any Vitest or Jest config containing a marker such as `include:` raises
  `typescript:executable-config-unresolved` (`typescript.go:44`, `:475-490`, `:2190-2203`), even
  when the value is a literal. The frontier is DISCOVERY (`frontier_reach.go:77-87`) and stays
  whenever a TypeScript path below the config is dirty (`unit.go:295-301`), so every TypeScript
  change under it runs the full TypeScript suite. Only Playwright config is evaluated.
- `corvint affected` reports BOUNDED or UNKNOWN (`cmd/corvint/affected_audit.go:52-60`), while
  `corvint test` passes some unknowns through `gateselect.AllowedUnknown` and adds suites through
  `FrontierSuite` (`internal/gateselect/allowlist.go:24-34`, `:51`) and reports NARROWED
  (`internal/sessiontest/manifest.go:34`). The two commands can disagree on one plan.
- The summary line prints `selected=` as `len(plan.Selected)` (`cmd/corvint/affected_summary.go:43`),
  which includes driver units appended at `select.go:411-419`.

### What is missing

- Import roots taken from the project's own configuration and layout, and a named unknown when an
  in-repository absolute import still does not resolve.
- Edges for glob imports and manual mocks, with a named unknown for computed forms.
- Evaluation of literal Vitest and Jest configuration values.
- Fault-injection cases that prove each new narrowing rule before it discharges a frontier.
- One per-unknown effect and one per-runner verdict shared by affected, test and MCP.
- Separate counts for test units and driver units.

## What Changes

- Python: derive import roots; raise `python:absolute-import-unresolved` for an unresolved
  in-repository absolute import, declared OUTBOUND at the importing file.
- TypeScript: model literal glob imports and manual mocks; raise `typescript:dynamic-import` or
  `typescript:mock-target-unresolved` for computed forms.
- TypeScript: evaluate literal Vitest and Jest configuration; keep the frontier for anything else.
- Gate every new narrowing rule on FIH-V1 cases.
- Compute the unknown effect and runner verdict in one function used by all three surfaces.
- Split the selected count.

## Capabilities

### New Capabilities

- `selection-hardening`: the requirements above.

### Modified Capabilities

- `affected-plan-python-playwright`: import roots and a new frontier.
- `typescript-javascript-affected-adapter`: glob, mock and literal-config rules.
- `affected-plan-selection` and `session-test-runner`: shared verdict and counts.

## Non-goals

- No change to Go dependency handling: a `go.mod` or `go.sum` change still selects everything
  (PROD-TEST-019, NCM-V1-005).
- No graph-cache incremental update (finding 4); proposed as an amendment to WCH-V1-007.
- No Go line-selection rework (finding 9) or removal of the second Go attribution (finding 8).
- Finding 7 (per-plan Git spawns): ticketed, not in this change (decided 2026-11-09 under R42,
  M9).
- No one-shot line evidence for uncommitted changes (finding 10); proposed as an amendment to
  `line-evidence`.
- No change to which lockfiles, runner configs or snapshot paths force a full run; that is
  NCM-V1-005 and RNC-V1-004.
- No evaluation of computed JavaScript configuration.

## Impact

- `internal/liveverify/affected/python`, `internal/liveverify/affected/typescript`,
  `internal/liveverify/affected` (frontier directions, counts), `internal/gateselect`,
  `internal/sessiontest`, `cmd/corvint`, and the MCP plan tool.
- SLF-V1-002 and SLF-V1-004 can only widen plans. SLF-V1-001, SLF-V1-003 and SLF-V1-005 narrow
  plans and are held behind SLF-V1-006.
- G4 risk: without SLF-V1-005, Vitest and Jest projects with a literal config cannot narrow.
