# V1-1783 / V1-1784: static test-validity rules for serial suites and memoized RSpec blocks

Owner input: [issue 1123](https://github.com/beamfall/corvint/issues/1123) (ISOLATED-IN-SERIAL)
and [issue 1124](https://github.com/beamfall/corvint/issues/1124) (RSpec block expectations on an
already memoized subject or let). TSA-V0-045..052 were added to
`docs/specs/test-source-analysis-v0.md`. TSA-V0-040..044 were left unused as a gap for parallel
lanes. No CAL-V0 ID was added. The requirements are accepted under the owner's standing
auto-accept of lane requirements (2026-10-11). Delivery stays experimental, like the rest of TSA-V0.

## Design

- **Placement.** The brief put both rules in `internal/testvalidity`, but that package imports
  only `tcq` and `mutate`. The JS lexer (`appmap`) and the declaration model (`testsource`) already
  depend on it, so importing them back would make a cycle. The split is:
  - `internal/testvalidity/source_rules.go` holds the vocabulary: `SourceFinding`, the two rule
    IDs and the `invalid`/`advisory` severities. It also holds the verdict wiring,
    `ApplySourceFindings`.
  - `internal/testsource` holds the readers, next to the closest existing rule family (the
    TSA-V0 lint and actor-authorization rules) and its Ruby line helpers.
  - The spec is TSA-V0, which governs that family.
  - `projection.go` is unchanged. Its line numbers are cited by the LPCV projector-reason table,
    and `Project`'s shape is mirrored in the VS Code extension and the shared vectors.
- **Severity.** `ISOLATED-IN-SERIAL` is advisory. A skipped run already projects `SKIPPED`, and a
  run that passed still witnesses the test, so the hygiene axis stays untouched.
  `MEMOIZED-BLOCK-EXPECTATION` is invalid. `ApplySourceFindings` makes hygiene `INELIGIBLE` with
  reason `memoized-block-expectation`, and the test-consolidation witness check already refuses a
  non-`ELIGIBLE` hygiene (`WitnessIneligible`). The refusal is covered end to end by
  `TestWitnessRefusesMemoizedBlockExpectation`.
- **The RSpec reader is bounded.** It reuses `rubyLines`/`rubyBlockEnd` and reads only:
  - group nesting;
  - subject/let/let!/before declarations;
  - example lines.
  It abstains with no findings on any imbalance or on nesting deeper than 64. Names resolve from
  the example's chain, innermost first, which matches RSpec's method override. A hook in an outer
  group that calls a let later redefined in an inner group still memoizes the inner value.
  Transitive let chains are a non-goal.
- **Not wired to a surface.** No CLI, MCP or preflight path calls `ValidityFindings` yet, so no
  existing report or wire contract changes.

## Residuals

- #1123 optional: a runner-facing summary that tells "skipped because an earlier serial test
  failed" apart from a plain skip, naming the failing test. Not delivered.
- #1124 optional: expectations whose target lines had zero coverage in the example's run. This
  needs per-example coverage input. Not delivered.
- No CLI, preflight or run-receipt surface reports these findings, and no producer applies them to
  a projection yet. The library and its verdict wiring are delivered and tested.
- The "creates its own fixture data and session" isolation heuristic from #1123 is not delivered.
  Only explicit annotations and tags count.
