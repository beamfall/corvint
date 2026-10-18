# V1-1769: advisory one-shot UI-state and exact-text cross-variant lint rules (issue 1114)

Ticket V1-1769, GitHub issue 1114. Requirements TSA-V0-029..032 in
[test-source-analysis-v0](../specs/test-source-analysis-v0.md), with amendments to TSA-V0-013
(closed rule set) and TSA-V0-016 (advisory findings do not change the exit status). They were
accepted under the owner's standing auto-accept of lane requirements (2026-10-11). This applies
that standing acceptance to this lane's requirements and is not a new owner decision.

## Decisions

- **Advisory, not blocking.** Both rules mark findings `advisory: true`. The exit status counts
  only non-advisory findings (`LintReport.Blocking`), so a suite with only advisory findings still
  exits 0. The issue asks for advice, and helper resolution is heuristic.
- **Resolution by name, bounded.** Helpers resolve by name to definitions in the asserting file
  and in relatively imported modules. Those modules are read at the revision through the existing
  committed-object reader, up to 3 import levels. Helper-following stops after
  `MaxHelperDepth` = 3 hops. A missing definition and a fourth hop each give `resolution: UNKNOWN`
  with the reason. Playwright, Promise and built-in names with no visible definition are skipped
  rather than reported, which keeps UNKNOWN for real unresolved helpers.
- **Cross-variant evidence.** `--runs` reuses `playwrightruns.ReadFile` and the declared
  `corvintRunIdentity.testRevision`/`testRootPath`. A report that does not declare the linted
  revision is not used, so evidence from another revision cannot name a cause. Splits are joined on
  literal title paths.

## Limits

- No live qualification was run. There was no real Playwright suite, no real multi-project run, and
  no run receipt. All evidence is synthetic fixtures, synthetic Git repositories and a synthetic
  Playwright JSON report.
- Name-based resolution can follow a same-named method of another class. An import alias, a
  package import or dynamic dispatch is UNKNOWN. An awaited non-UI call to an unresolved helper
  (for example an API client) is UNKNOWN, not silent.
- Locators stored in constructor fields or properties are not followed for `exact: true`. A split
  with no exact-text locator is listed in `crossVariant.splits` without a finding.

## Residuals

- The issue asks to show advisory findings in the run receipt next to the failure. No small hook
  exists for that, so it is not built.

## Evidence

Focused tests: `./internal/testsource` (`TestLintRuleFixtures`, `TestOneShotHelperForms`,
`TestOneShotUnknown`, `TestNewRuleSuppression`, `TestCrossVariantEvidence`,
`TestLintFollowsImportedHelpers`, plus the existing package tests) and `./cmd/corvint`
(`TestTestsCLI`, `TestTestsLintAdvisoryCLI`).
