# Affected readers, gateselect go.work arguments, draft bookkeeping paths

Date: 2026-10-23. Lane: claude/wizardly-wozniak-zwhb4n (base 77a49dfa). Tickets: V1-1817,
V1-1803, V1-1006, V1-2370, V1-2423, V1-2294, V1-2153. Requirements `AFP-V0-400..403`
(`docs/specs/affected-plan-v0.md`) and `DCW-V0-333`
(`docs/specs/daily-change-evidence-workflow-v0.md`) are accepted under the owner's standing
auto-accept (2026-10-11). V1-2294 and V1-2153 change tests only and add no requirement.

## Decisions

- V1-1817 (AFP-V0-400): shell units reuse `namesFileOnlyWithin` with the script's own directory.
  Its bases (the repository root, the unit subtree, and ancestors once a token climbs) match how
  shell tests run: from a make target or CI step at the root, or after `cd "$(dirname "$0")"`.
  A `cd docs/specs` literal puts both components in the vocabulary, so a script that reaches a
  directory by name keeps its reader edge. Other non-Go namespaces keep AFP-V0-072 (3).
- V1-1803 (AFP-V0-401): literal `filepath.Glob` patterns become a new `Unit.PathGlobs` member
  and a `PATH_GLOB_READER` witness with a `glob` member. They are not folded into `PathTokens`,
  because the lexer splits a pattern at `*` into lone fragments that name nothing. A full parse
  runs only for files whose text contains `Glob(`. A one-component pattern after a run-time root
  is not recorded. A first cut placed it like a lone token, and `filepath.Join(dir, "*")` in
  `cmd/corvint` then selected that 591-second package for almost every repository path.
- V1-1006 (AFP-V0-402): positions are classified from the AST, and the default keeps the edge.
  The only data positions are a closed list where a literal cannot reach a process: comparison,
  case, index, string predicates and test messages. A per-file `os/exec` import check was
  rejected: most consumers build through a helper in another file of the package (for example
  `internal/companionrelease`), so it would have dropped real edges. On this repository the rule
  drops 3 of 87 edges, all data.
- V1-2370 (AFP-V0-403): gateselect keeps mirroring the planner rather than importing
  `affected/golang`. The argument checks are ported verbatim into `internal/gateselect/workspace.go`.
  Parity is proven by the existing `CommittedReadScopes` comparison table. The typed
  `WorkspaceRefusal` stays internal, because the selector's output for an unknown base is
  unchanged.
- V1-2423 (DCW-V0-333): drafts exclude only the bookkeeping paths. The ticket also asked to
  exclude every file the diff touches. That would remove the `DCW-V0-150` nearby-line fallback
  for every hunk, so it is not done here and is recorded as a non-goal.
- V1-2153: the expected SOL-V0 clauses come from `docs/specs/REQUIREMENTS.tsv`. The Perl
  generator writes that file independently of the OCM parser the test exercises, so a new clause
  passes once the index is regenerated, and a clause the OCM stops enumerating still fails.

## Evidence

- Each new test fails on the old code: `TestLoneFileNameShellReaderMustPlaceThePath_AFPV0400`,
  `TestLiteralGlobSelectsTheReaderOfAnAddedFile_AFPV0401`,
  `TestDataOnlyCommandLiteralCreatesNoEdge_AFPV0402`, the four AFP-V0-403 cases of
  `TestBaseReadScopesFollowCommittedReadScopes_AFPV0050`, and
  `TestProposeCitationsDCWV0333SkipsBookkeepingPaths`. Two tests cover behaviour that already
  existed, so they were checked differently.
  `TestChangeDCWV0117StagedSecondPlanIsAddedToCitations` fails when `citedOver` requires two
  earlier citations. The rewritten `TestSelfObservationSpecEnumeratesEveryObligation` passes with
  a probe clause added to both the spec and the index (the old test failed on "fourteen"). It
  fails with a clause removed from the spec alone.
- Repository scans on 77a49dfa: 39 Go packages carry literal globs. Planning each of the 3433
  paths the last 39 merges changed on its own added no `PATH_GLOB_READER` selection. Command
  edges went from 87 to 84 (55 to 53 consumer packages).
- `TestCoreVerbsEmitTheFrozenProfiles`: the `init_summary`, `adopt_summary` and
  `init_full_receipt` subtests fail on a fixture commit revision identically on clean 77a49dfa
  in this container, so they are environmental and not caused by this change. The affected
  goldens pass.

## Limits

AFP-V0-401 does not read `os.ReadDir`, `fs.Glob` or patterns passed through a variable; those
paths stay `UNOWNED_DIRTY_PATH` unknowns. AFP-V0-402 still over-selects on a data literal outside
its closed positions. Exec consumers' importers and environment-supplied binaries stay retained
AFP-V0-037 limits. Dogfood binding, CEM sealing and the Tasks completion writes were not run in
this environment.
