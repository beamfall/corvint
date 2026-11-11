# Selection hardening: Python and JavaScript selection frontiers (V1-5401)

Date: 2026-11-11
Ticket: V1-5401
Change: `docs/specs/openspec/changes/selection-hardening/` (accepted automatically under G14,
decision 0596)
Spec: `docs/specs/selection-hardening-v1.md` (SLF-V1-001 to SLF-V1-008)

## What was built

Tasks 1 to 6, 6a and 9 of the change's `tasks.md`; tasks 7 (SLF-V1-007) and 8 (SLF-V1-006) wait
for changes `runner-contract`, `source-removal` and `fault-injection-harness`, and task 10 is the
orchestrator's.

- SLF-V1-002 (on): `python:absolute-import-unresolved`, OUTBOUND and source-bound, at the importing
  file. A name that only prefixes observed modules (a namespace package) raises nothing, nor does a
  third-party name.
- SLF-V1-001 (held): pytest rootdir, `pythonpath`, `testpaths` and layout roots, behind
  `importRootsNarrowing`.
- SLF-V1-003 (frontier on, expansion held): glob calls now raise `typescript:dynamic-import`;
  expansion and the membership reader sit behind `globExpansionNarrowing`.
- SLF-V1-004 (on): manual-mock edges and `typescript:mock-target-unresolved` (DISCOVERY).
- SLF-V1-005 (markers on, evaluation held): the five A6 key-shaped markers; the literal-config
  evaluator sits behind `literalConfigNarrowing`.
- The never-Uncovered rule of SLF-V1-002 and SLF-V1-004 is one shared hook,
  `affected.CoverageUnproven`, consulted by the coverage proof (`uncovered.go`).

## Differences from the proposal's tree (9ec3a29d0)

- The proposal's line citations (`python.go:43`, `typescript.go:1079-1120`, `select.go:395-410`,
  `affected_summary.go:43`) point at code that has moved; the rules they describe still hold on
  `origin/main` af393f1b1, and the change was built against that.
- `import.meta.glob` and `require.context` were not read as computed loading on this tree: the
  scanner read `import.` and `require.` as no call at all, so a glob import was a silent drop. The
  held form of SLF-V1-003 is therefore a fail-closed fix that ships on.
- SLF-V1-002's "names a directory or `.py` file in the tracked tree" is decided over the Python
  file set (directory components of `.py` paths and module stems), since the plugin lists only
  `.py` files and a directory without Python source cannot satisfy an import.
- SLF-V1-005's evaluator is stricter than the design (spread or relative import anywhere, unknown
  parent object, negated glob, capturing mapper on a package-shaped key: all keep the frontier).
  The `typescript:config-evaluation-mismatch` check against an executing lister is not built: the
  static planner runs no runner.
- SLF-V1-008's JSON members are not emitted: the affected-plan/1 and /0 documents are frozen
  (CCF-V1-006) and the lane rules forbid regenerating their core-freeze goldens.

## Finding 11 settle record (task 6a, R42 (panel pick) A8)

Question: does a Go-only edit admit the TypeScript plugin and its full suite?

Method: (1) fixture test `TestFinding11GoOnlyEditDoesNotAdmitTypeScript`
(`internal/liveverify/affected/selection_hardening_test.go`): a Go module beside a Vitest project
with an executable config and a dynamic import; (2) `corvint affected --full --path P` on this
repository with a `corvint` built at 29b5ddcfb, for four Go-only paths
(`internal/repopath/repopath.go`, `internal/specindex/requirements.go`,
`internal/liveverify/affected/select.go`, `cmd/corvint/main.go`).

Result: not confirmed. In the fixture the plan is BOUNDED, names no TypeScript unknown and selects
no TypeScript test; the TypeScript edit control is UNKNOWN. On the repository every Go-only path
lists the TypeScript plugin in `frontierExcluded` ("no dirty path is this plugin's or unowned, and
the change reaches its units only as reader or binary-exec selections"), names no TypeScript
unknown, and selects 5 of 71 TypeScript test units: four Playwright specs through their
`typescript:e2e-runtime-dependency` sites (AFP-V0-818, the cross-language runtime rule, which G11
keeps) and `tools/web-flows/test/fixture/flows.spec.ts` through a real BINARY_EXEC edge to
`cmd/corvint-web-flows`. No fix is needed before the Go replay.
