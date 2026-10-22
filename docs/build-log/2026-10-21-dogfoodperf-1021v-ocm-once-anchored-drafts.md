# Lane dogfoodperf-1021v: one OCM status per check, anchored citation drafts

Tickets: V1-2274 (P3), V1-2355 (P3), V1-2343 (P2). Requirements `DCW-V0-160` and `DCW-V0-161` are
accepted under the owner's standing auto-accept (2026-10-11). The branch is stacked on #1513
(head `4401db439`), which adds `propose-citations`; BASE for this lane's CEM is that head.

## V1-2274: `dogfood-ocm status` ran once per verifier

**Problem.** `dogfood check` (and `seal`, and `finish-lane`'s check-seal step through it) ran
`dogfood-ocm status` with the base and tree verifiers and any override, and required byte-identical
output. Under a `DCW-V0-047` declaration it ran the base once and the tree twice. A check-seal with an
override therefore read every OCM map three times.

**Decision (owner BUILD-NOW, relayed in the lane task).** The aggregate depends only on the target
tree and its frozen scope set. It is now computed once, by the tree verifier, and that result stands
for every role (`DCW-V0-160`). Independence is kept in two places:

- the byte comparison with `.corvint/change.ocm-status.json`, which `dogfood change` computed with the
  change's own executable (the override when set);
- the unchanged `cem status` cross-verifier agreement.

**Accepted narrowing.** A base or override verifier whose OCM read alone disagrees no longer fails
`verifier-disagreement`. The prior lane (#1472) rejected removing runs for this reason. The owner
decision overrides that, and the spec's failure modes record it. `DCW-V0-047`'s OCM sentence is
marked superseded.

**Evidence.**

- `TestCheckDCWV0160ComputesOCMStatusOnce` counts one tree run, with no base or override run, both
  declared and undeclared, including when the base would refuse.
- Without the change, the test fails with `verifier-disagreement`.
- Wall times: TIMINGS.

## V1-2355: drafts were never anchored

**Reproduction.** `propose-citations` (`DCW-V0-150`) cites a nearby heading or declaration in the
hunk's own file. Such a line almost never contains the hunk path or defines an ID the hunk adds, so
`dogfood check` warned `tier-below-anchored` (`CEM-V1-081`) for every drafted row. The new test
failed against #1513's drafter: it cited `internal/x/x.go`, not the spec line that defines the ID.

**Decision (`DCW-V0-161`).** The drafter first tries lines that earn the hunk an anchor, judged by the
checker's own rule through a new `verify.SpanAnchors`:

1. the line that defines an ID the hunk adds;
2. then a line naming the hunk path, with `docs/specs` lines first.

The candidates come from one bounded `git grep -F` of BASE and must pass the `DCW-V0-150` stability
rule. When no candidate passes, the row falls back to a nearby line, and stderr records
`row N cited, not anchored (CEM-V1-081): REASON`. The plan grammar has no field for a reason, so it
stays four fields. Basename matches are not used, because they are too often unrelated.

**Evidence.** `TestProposeCitationsDCWV0161DraftsAnchoredRows` tiers the drafted rows with
`verify.ComputeCitation` and `belowAnchoredWarnings`: only the fallback row warns. The test fails
without the change.

## V1-2343: warning on new files citing a direct dependency (refuted as a defect)

`CEM-V1-081` defines exactly two anchor kinds: a defined ID and the path. A direct dependency is not
an anchor, so a new file citing the function it calls is `cited`. Its warning already names the rule
and the fix.

`TestCEMV1081DirectDependencyIsNotAnAnchor` pins the following:

- citing the dependency gives `cited`;
- citing a span naming the path gives `anchored`;
- `SpanAnchors` agrees with both.

Adding a dependency anchor kind would change the citation block, which the portable consumer
mirrors, and is not a small fix. The drafter remedy above removes the warnings for drafted rows.

## NOT_RUN

- Tier 2 of `corvint affected` (batch CI).
- `make gate`.

## Rollback

Revert `verifyOCM` to restore per-verifier OCM runs. Revert the anchored preference and
`verify.SpanAnchors` to restore nearby-line drafts. No map, plan, report or sealed CEM format
changes.
