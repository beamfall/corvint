# 2026-10-19 V1-1988: reachable asserting E2E gap rule

GitHub #1245. `docs_gaps` reported `missing-asserting-e2e-tests` for every flow: the clearing
condition required relation evidence `trust == "reviewed"`, but provider import
(`internal/doccorpus/providers.go`, `checkEvidence`) always rewrites a provider's `reviewed` to
`trust: generated` with `reported_trust: reviewed`, and no other path sets `reviewed`.

## Decisions

- Imported review labels are attributed, not authenticated (`DCP-V1-039`, product invariant 2), so
  the clearing rule is unchanged: only `trust: reviewed` clears the gap (`DCP-V1-150`).
- A flow whose only unmet condition is a provider-reported review now reports the distinct, weaker
  `asserting-e2e-tests-unauthenticated-review` instead of `missing-asserting-e2e-tests`
  (`DCP-V1-151`).
- `missing-asserting-e2e-tests` remains for every other flow, and its reason names the failed
  condition: `no test link`, or the closest link's failures in fixed order (`not asserting`,
  `not exact`, `not supported`, `review not authenticated` / `not reviewed`) (`DCP-V1-152`).
- Gap kinds are open strings: no schema, MCP tool description, `docs_coverage` or test enumerates
  them (searched the tree for both kind strings). The only kind list is the DCP-V1-038 test, which
  asserts presence and still passes. `docs/DOCUMENTATION-CORPUS.md` names the
  new kind.

## Identity

Artifacts are unchanged (gaps are computed at query time). `gaps` receipts change only for flows
with test links or an asserting gap: the reason text, and the kind for provider-reviewed links.

## Evidence

- `TestDCPV1150ProviderReportedReviewYieldsUnauthenticatedGap`: the issue's reproduction through
  real import; the relation reads `generated`/`reported reviewed` and the flow reports the weaker
  gap, not `missing-asserting-e2e-tests`.
- `TestDCPV1151AuthenticatedReviewClearsAssertingGap`: absence of both asserting gaps for a link
  that satisfies the rule (trust set in memory, since no import path can grant it).
- `TestDCPV1152AssertingGapNamesTheFailedCondition`: `declared`/`heuristic` confidence and the
  `navigating` role through import, the issue's `inferred`/`related` values in memory (the adoption
  schema refuses them at import), unsupported, unreviewed and no-link flows each keep
  `missing-asserting-e2e-tests` with the named condition.
- Without the fix, 1150 and every 1152 subtest fail; 1151 passes on the old code too (it pins the
  rule that must not regress).

## Deviations

The issue's `join_confidence: "inferred"` and `role: "related"` are not valid adoption values
(`exact declared heuristic unknown`, `asserting navigating`); they are covered in memory, with the
schema's non-exact values covered through import.

## Rollback

Revert the commit: the gap rule returns to the single unreachable kind; no stored state changes.
