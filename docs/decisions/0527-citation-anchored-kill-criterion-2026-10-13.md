# Decision 0527: the anchored citation tier must beat mechanical producers (kill criterion K2b)

Date: 2026-10-13. Status: accepted under the owner's standing auto-accept of lane requirements
(2026-10-11), conditional on Codex PASS. Amends decision
`0523-cem-1-0-rc3-verifier-computed-support-2026-10-11.md` §5 (kill criteria) by adding K2b.
Lane V1-1336 (with V1-1330 and the registration half of V1-1337); requirements `DCW-V0-051..056`.

## Context

Decision 0523's K2 asks only that a rubber-stamp map and a careful map of the same patch differ in
their `citation` block. The 2026-10-13 CEM 1.0 confidence panel found that this is too weak: a
forged-ID map (every hunk breaks the requirement it cites behind a `// satisfies ID` comment) and a
path-inventory map (every hunk cites a line that only names its file) both score `anchored` on
24 of 24 hunks, above the honest careful fixture's 16; a grep bot that reads only the diff anchors
far more hunks than authored maps do; and the pooled anchored signal over a derangement null is
2.1% [1.7%, 2.5%] of supported hunks once diff-adjacent bases are excluded. `anchored` therefore
measures lexical coincidence between a hunk and a span, which a producer can manufacture at no
cost. Decision 0523 already numbers K3 to K6, so the new criterion is named K2b: it refines K2 and
does not renumber anything.

`tools/cem-citation-baseline` now scores three arms with the verifier's own tier rules
(`authored`, `grepbot`, `preimage`), a 200-derangement null seeded from each map's `patchSha256`,
and the self-citation members (`DCW-V0-051..056`). Its test fixtures pin the forged-ID and
path-inventory defect as current behaviour.

## Decision

1. K2b: on the prospective set that
   `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-13-cem-citation-prospective-registration.md` defines (the next 30
   `change`-class `cem/1.0` maps sealed after this lane merges, scored by the registered scorer),
   authored maps MUST exceed BOTH mechanical arms (`grepbot` and `preimage`) on pooled
   `anchoredExcessExclAdjacent`, and the authored 95% interval's lower bound MUST lie above the
   grep bot's point estimate. The registration's decision rules (kill when the authored excess is
   at most 2.1% or the diff-adjacent base share exceeds 25%; improve when the lower bound exceeds
   both 2.5% and the grep bot's estimate) are the procedure; nothing is chosen after the maps are
   read.
2. The held falsifier stays: decision 0523 K1 (fewer than 25% of non-mechanical Go code hunks
   `held` across the next 30 dogfood changes after P2 lands) is unchanged and is reported on the
   same set as the registration's secondary metric.
3. If K2b fails, or the registered run is inconclusive, the anchored tier is recorded as "lexical
   coincidence, not support" and is removed from the reviewer-facing summary (the report row and
   any reviewer text that presents `anchored` as evidence of support) until a redesigned tier
   passes a new registration. The verifier's citation block keeps computing the tier for
   measurement.
4. The committed-corpus numbers below are the pre-registration baseline. They are not the test:
   the corpus was authored before the arms and this criterion existed, and 615 of its 630 maps
   differ from their bind commit's sidecar (V1-1277 rewrote them in place), so they were scored at
   `CEM-V1-104` T' targets.

## Pre-registration baseline (committed corpus)

Source: `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/evidence/v1-1336/citation-baseline-summary.json` (member `change`, which
equals `all`; no committed map is `chore` or unclassified), from the scorer at this decision's
commit over the 630 maps under `.corvint/changes/` at f8ea0758e. 629 maps were scored with arms
and no `armsIssues`; f0fda542 has no status (`unsupported-resource-limit`, 877 hunks); 14 maps
with a single supported hunk have no null, so the pooled rows cover 615 maps and 22,639 of the
22,653 supported hunks.

| Arm | Hunks with bases | Anchored | Pooled excess [95%] | Excess, diff-adjacent excluded [95%] |
|---|---|---|---|---|
| authored | 22,653 | 1,249 (5.5%) | 2.03% [1.62%, 2.44%] | 2.04% [1.64%, 2.44%] |
| grepbot | 16,985 | 16,980 (75.0%) | 67.4% [65.2%, 69.5%] | 67.0% [64.8%, 69.1%] |
| preimage | 18,467 | 282 (1.2%) | −0.43% [−0.72%, −0.14%] | 0.00% [0.00%, 0.00%] |

Kill statistic: the grep bot anchors at least as many hunks as the authored map in 629 of 629
scored maps (100%). Self-citation: 2,534 of 25,137 authored bases (10.1%) are same-path and 799
(3.2%) are diff-adjacent, on 2,408 (10.6%) and 790 (3.5%) of supported hunks. Tiers: `anchored`
1,249, `cited` 21,404, `unknown` 43, `held` 0, `mechanical-proved` 0.

Read against the registration's rules, the committed corpus would already be killed (E = 2.04%
≤ 2.1%; L = 1.64% is below both 2.5% and G = 67.0%), and the held share is 0. That is the
baseline the prospective set must move, not its result.


## Amendment 2026-10-13: record maps leave the authored arm (V1-1344)

Accepted under the owner's standing auto-accept (2026-10-11); V1-1344; `DCW-V0-057..059`,
`CEM-V1-170`. A map whose hunks change only `docs/build-log/` or `.agent-evidence/` paths, with
at most the CEM sidecar paths beside them, records evidence about other work rather than changing
the product, and commonly cites one BASE decision for every hunk. `cem status --citation` now
reads such a map as class `record`, and the authored arm of K2b counts `change`-class maps only,
so record maps are excluded from it. The kill threshold and every other rule of point 1 are
unchanged. The registration pins the scorer at the V1-1336 merge, where a record map still reads
class `change`, and this amendment edits `tools/cem-citation-baseline/main.go`, so the exclusion
reaches the prospective set only through a new dated registration made before any of its maps
is read; until then the registered run stands as registered.

## Consequences

- Reviewer-facing text cannot call an `anchored` hunk supported on the strength of this tier until
  K2b passes; until then the tier is a measurement, not a claim.
- Any change to `tools/cem-citation-baseline/main.go` after the merge needs a new registration
  before it scores the prospective set; the scorer is pinned by blob OID.
- A tier redesign changes the fixture numbers in
  `TestBaselineDCWV0052ArmsPinTheAnchoredTierDefect` deliberately, with the test.

## Rollback

Revert this decision with lane V1-1336's merge: decision 0523's K1 to K6 stand alone again, the
tool loses its arms and members, and the registration file goes with it. No map, wire contract or
verifier behaviour changes with this decision, so nothing else needs migrating.
