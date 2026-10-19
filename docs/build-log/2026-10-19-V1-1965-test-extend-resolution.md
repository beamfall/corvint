# 2026-10-19 V1-1965: resolve extend-derived and re-exported Playwright test bindings

Ticket: V1-1965 / GitHub #1213.

## Decision

`TSA-V0-147` is added to `docs/specs/test-source-analysis-v0.md` and accepted under the owner's
standing auto-accept of lane requirements (2026-10-11). It amends the TSA-V0-143 attribution. The
failure mode that read an aliased test root as globals is replaced.

- Cause. In the issue's spec, `const test = scenarioTest` made the called root `test`, but the
  import bound `scenarioTest`. The root was never seen as imported, so the file was read as
  globals. Globals run only under Jest, Vitest or Mocha, so a Playwright-only repository reported
  it `no-runner-for-framework` (an `invalid` TEST-FILE-UNSELECTED).
- Roots are followed through top-level aliases (`= x`, `= x.extend(…)`, `= x.extend<…>(…)`). An
  imported binding is followed through committed relative modules: named, default and `export *`
  re-exports, `export { a as b }` lists and each module's own aliases. The bounds are depth 8 per
  binding and 256 module reads per report. Every module read is `readOne` of committed content.
- A followed binding gives an explicit framework, so a fixture derived from Vitest is still
  reported in a Playwright-only repository. A binding that cannot be followed stays `indirect`.
  That is the existing attribution, which every modelled runner may run, so it never yields
  `no-runner-for-framework`. An indirect file that no runner's patterns select is still reported
  with the pattern reason, which is true whatever the framework is.
- A test or describe root declared in the file that ends on no import, such as
  `const test = makeTest()`, gets the new `unresolved` attribution. The file is `uncertain` with
  reason `framework-unresolved`, never `invalid`. `expect` is excluded from this rule, so a
  `const expect = chai.expect` file that uses globals keeps its old attribution.
- Bare and `tsconfig` path-aliased specifiers (the issue's `fixtures/scenario.fixture`) are not
  resolved. Reading `paths`/`baseUrl` would add a configuration dialect. The issue's example
  still has no finding, because it is `indirect` and the Playwright `testDir` selects it.
  Resolving path aliases is NOT_PRODUCED.

## Evidence

`TestUnselectedExtendResolutionTSAV0147`, `TestFrameworkResolutionBoundsTSAV0147` and
`TestSelectedSourceValidityExtendTSAV0147` run on synthetic repositories. Live qualification on
the reporting suite was NOT_RUN.
