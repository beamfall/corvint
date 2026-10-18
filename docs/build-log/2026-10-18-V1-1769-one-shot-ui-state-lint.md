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

## Review fixes (PR #1119, one Codex round)

All nine findings were fixed. None were deferred. No new requirement IDs were added: TSA-V0-029,
030 and 031 are amended in place.

- A one-shot finding spans the whole multi-line assertion, and an exact-text finding spans the whole
  locator call. Changed-line scoping uses that span, and suppression still anchors on the first line.
- Returned locals are followed with a type annotation, through parentheses and `await`, and
  `return await local` included.
- A standalone helper named like a read (`isVisible(page)`) is resolved as a helper. Only member
  reads are treated as reads.
- A `return` in a nested function, arrow or method body is not attributed to the enclosing helper.
- Exact-text resolution emits `resolution: UNKNOWN` for a helper with no definition and for a fourth
  hop. It collects and dedupes every reachable exact locator, not only the first.
- The `exact` option is parsed fully: bare and quoted keys, last property or spread wins, and only
  the literal `true` is exact. A non-literal value, spread, shorthand or non-object options argument
  is UNKNOWN.
- Parse uncertainty from imported helper modules is listed in `uncertain`, sorted.
- The linted files and all import levels share one cumulative 5,000-file / 64 MiB budget.

Tests added: `TestOneShotLocalsAndNesting`, `TestCrossVariantFollowing`,
`TestLintMultiLineChangeScope`, `TestLintImportedHelperUncertainty` and
`TestLintImportBudgetIsCumulative`.

The scope limit is unchanged. A change only inside a helper's definition does not bring the
calling finding into the changed-line scope.
