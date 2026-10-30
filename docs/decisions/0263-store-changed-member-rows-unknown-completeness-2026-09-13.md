# Decision 0263 — STORE_CHANGED retained-member rows carry UNKNOWN completeness

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

The `RETAINED_MEMBER` metric rule in `docs/specs/local-observability-dashboard-v0.md` said that on a
partial or invalid trace store every retained-member denominator is null and completeness is
`PARTIAL`, and that `STORE_CHANGED` makes the member values null. `model.Compile` and
`VerifyCanonical` (`deriveDataMetrics`, `retainedMemberMetric`) instead copy the aggregate source
completeness into the two `STORE_CHANGED` member rows. The adapter sets that completeness to
`UNKNOWN` for `STORE_CHANGED` (`internal/dashboard/adapters/scan.go`, `traceRootFailureCode`), so
the rows were null `NOT_OBSERVED` with `completeness: UNKNOWN`, contradicting the spec text.

The call: the code is right and the spec is amended. A `STORE_CHANGED` store observed neither its
member set nor its candidate count. `PARTIAL` asserts that part of a universe was observed and part
was excluded, which is certainty the scan does not have. Product invariant 2 (missing evidence
produces uncertainty, never invented certainty) and the aggregate source's own `UNKNOWN`
completeness both point to `UNKNOWN`. A stable partial or invalid store, which did observe its
candidate universe, keeps `PARTIAL`.

Making the code emit `PARTIAL` was rejected: it would give the member rows a stronger completeness
claim than the source they are derived from, and a verifier reading the snapshot could not tell a
changed store from a stable one with excluded members.

Consequence: no snapshot bytes change, because the code already emitted `UNKNOWN`. The
`RETAINED_MEMBER` paragraph (metrics requirements `LOD-V0-007..013`) now states the `STORE_CHANGED`
case explicitly, and `TestStoreChangedEmitsUnobservedMemberUniverse` asserts the completeness. No
requirement IDs added.

Rollback: revert the commit. That restores the spec sentence and drops the test assertion; the code
is unchanged either way.
