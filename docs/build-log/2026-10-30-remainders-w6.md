# Lane remainders-w6 — V1-2411, V1-3192, V1-2808, V1-1930, V1-1587

Delivers the unmet acceptance criteria that earlier lanes left on five tickets. Each change is a
remainder of an accepted contract; IDs come from the lane's assigned blocks.

## Contract and implementation

- CCE-V0-310 (V1-2411 AC2): `corvint-test e2e harvest --source [ALIAS:]DIR` binds a proposal the
  application map leaves screen-less through the site-knowledge IR. `appmap.LineScreens` walks
  each test's own body steps, carrying the screen of the last visit or page-object action joined
  to a screen; an assertion takes that screen (`site-knowledge-step-screen`) and a visit takes its
  resolved URL screen (`site-knowledge-visit`). The map's goto, flow-outcome locator and
  `flow-ambiguous` results still win; the IR ranks above `test-file-screen` and the map's unbound
  reasons. An unreadable declared source refuses `e2e-site-knowledge-unavailable`. The CCE-V0-050
  trace row is no longer partial.
- LCP-V0-060 (V1-3192): an interrupted `dogfood finish` (SIGTERM or SIGINT through the real CLI)
  still writes its decision envelope to stdout, and the strict path returns the evaluation with
  its final-check error. The rerun reuses the retained observations (AC3).
- STR-V0-062 (V1-2808): the argument refusal names `--confine` and its companions. Test only.
- TSA-V0-870 / TSA-V0-164 (V1-1930): an EITHER-OR-OUTCOME exemption is recorded with its reason
  (`RuleExemption`) and proposes no value. The with-receipt proposal stays `NOT_PRODUCED`: receipts
  carry operand sizes only (LPCV-V0-120).
- TSA-V0-871 (V1-1587 AC6): `tests setup-select --format lines` is the provider hand-off.
  A live test hands it to a real Playwright run as the setup project list. Only the selected setup
  runs, and an ALL control runs every setup.

## Executed evidence

- `TestCCEV0310HarvestBindingSiteKnowledge`, `TestCCEV0310BindProposalIRPrecedence`,
  `TestCCEV0310HarvestSourceFlag`, `TestCCEV0050*`, `TestCCEV0088*`, `TestCCEV0118SubcommandHelp`:
  PASS.
- `TestLocalCompletionRealEvidenceWorkflow` (signal sub-check through the CLI): PASS (Mac).
- `TestSTRV0062ArgumentRefusalNamesTheFault`, `^TestEitherOrOutcome`: PASS.
- `TestTestsSetupSelectProviderHandoffLive`: PASS once on the Mac (2026-10-30) with
  `@playwright/test` 1.61.1 and Node 22. Without `CORVINT_PLAYWRIGHT_MODULES` it skips as
  `NOT_RUN`, so CI does not exercise it.
- `make capability-catalog-check`: rc=0.

## Limits

The live Playwright hand-off ran once against a fixture, not against an adopter suite. IR binding
covers only lines written in the test body; helper-body (`Via`) steps are not bound.

## Review fixes (orchestrator Codex review of f7449ea)

- CCE-V0-310: an opaque visit inside a joined page-object method (a dynamic `page.goto`) now
  forgets the screen before the page object's own screen is considered, so the assertion after it
  stays unbound. `TestCCEV0310OpaqueVisitInJoinedPageObjectForgetsScreen` failed before the fix.
- TSA-V0-870: a contract value line's own trailing comment (YAML ` #`, ` //`, Markdown `<!--`) is
  read apart from the expected value, so an inline annotation is recorded as an exemption and an
  unannotated trailing comment no longer hides the value's alternatives. The inline cases in
  `TestEitherOrOutcomeExemptionRetainsReason` failed before the fix.
- LCP-V0-060: the CLI signal assertion also runs on the pre-seal strict finish path, which is
  the path the `finish.go` change covers; with that change reverted it fails with empty stdout.
