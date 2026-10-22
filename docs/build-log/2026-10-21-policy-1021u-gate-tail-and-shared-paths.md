# Lane policy-1021u: failed-gate output tail, shared CEM paths, closeout-route findings

Tickets: V1-2196, V1-2241, V1-1840, V1-2254, V1-0549. Base `f11ac58d3`. TCD-V0-165..166 are accepted
under the owner's standing auto-accept (2026-10-11).

## V1-2196: a failed gate is diagnosable without a rerun (DELIVERED)

**Problem.** `focused-docs` and `full-gate` failures showed only `GATE_FAILED`, so the closer reran
the whole gate to learn which check failed.

**Finding.** The policy `evidence: []` label does not suppress output capture. `runGate` always
records an `output` evidence entry. `LeaseGateRun` always posts the captured bytes, within the 16 MiB
bound, to `evidence/<sha256>`. On the live store, the FAILED `focused-docs` record `b0248975`
(attempt `694bb1eb`) names output blob `c04e7d9e`. That blob is 30377 bytes and ends in the
`line-citations-check` failure. The gap was presentation: no result named or quoted the output.

**Decisions.**
- The `gate run` result adds `gateResult` (TCD-V0-165). A result that did not pass carries a
  20-line tail, a failing line and one warning with the output path. Each line is cleaned, bounded
  to 240 bytes and secret-screened through the existing `obligation` import edge (decision 0397
  #712). A line is withheld when its raw line, or a screen match over the whole tail, covers it, so
  truncation cannot expose a cut secret.
- No store policy change (TCD-V0-166). Adding `"evidence": ["output"]` to the two gates would be
  descriptive only, and it changes each gate's definition digest, so their earlier results would go
  stale. The orchestrator may skip the policy update that criterion 1 names. If one is wanted anyway,
  the fragment is `"evidence":["output"]` on the `focused-docs` and `full-gate` gate definitions.
- The tail appears only in the executing call. Re-rendering it in `attempt show` and `ticket show`
  is left as a residual.

## V1-2241: reservations.sharedPaths for the spec index (ALREADY-DELIVERED, code side)

The code side is NTP-V0-026, delivered by PR #1428 (`8def072e0`). It covers `readReservations`,
`CollideShared`, the claim, widen, expired-blocking and plan checks, and
`TestNTPV0026_SharedSpecIndexDoesNotCollide`. What remains is the store write. The orchestrator
applies it with a `corvint-tasks` binary that contains #1428:
`{"reservations":{"sharedPaths":["docs/specs/INDEX.json","docs/specs/README.md","docs/specs/REQUIREMENTS.tsv"]}}`.

## V1-0549: isolated lanes serialized on mandatory shared evidence paths (findings, DELIVERED)

**Reproduction.** `TestNTPV0026_SharedCEMPathsAdmitIsolatedLanes` runs with capacity 4. A second
lane listing `.corvint/change.cem.json`, `.corvint/changes/` and `docs/specs/REQUIREMENTS.tsv` is
refused `RESOURCE_COLLISION`. With those paths declared shared, it is admitted. A hand-authored path
and the uncovered `.corvint/` key still collide, so real scope checks and peer safety are kept.

**Phased admission recipe.**
1. **Admission.** Declare the mandatory paths shared. Every lane still reserves them, and submit's
   `OUT_OF_SCOPE` check is unchanged. The orchestrator's fragment, byte-sorted:
   `{"reservations":{"sharedPaths":[".corvint/change.cem.json",".corvint/changes/","docs/specs/INDEX.json","docs/specs/README.md","docs/specs/REQUIREMENTS.tsv"]}}`
   (it supersedes the V1-2241 fragment).
2. **Integration-time ownership.** The generated spec index is regenerated with
   `make spec-requirements` at each merge, and `spec-requirements-check` refuses a stale index.
   `make dogfood-seal` moves `.corvint/change.cem.json` to `.corvint/changes/<sha>.cem.json`. Sealed
   lanes therefore never share final content: each added and then removed the transient path.
3. **Final content-bound evidence.** Each lane's `dogfood-check` and seal bind its own final
   commit. The batch's own CI and doc gates bind the merged tree. No final effect is omitted and no
   gate is lowered.

**Limit.** Two unsealed lanes that both commit `.corvint/change.cem.json` conflict at `git merge`,
which NTP-V0-026 already states. The merge refuses the conflict mechanically, so nothing is lost
silently. The recipe needs every lane to seal before integration.

## V1-1840: cem-verdict touched-anchored closeout route (SKIPPED)

This needs two things: a binder route that gates the delivered code of an anchored addendum, and a
pre-claim preflight that substitutes `{attempt.baseCommit}` and `{claim.verdictPaths}`. Today
`preflight --deep --gate` performs no substitution and needs an obligation ledger. That is store and
CLI work in the lease and gate areas that peer lanes hold, and acceptance needs a live
`cem-verdict` run, which this lane may not perform. Interim route: the anchored build-log addendum
(`docs/build-log/2026-10-18-v1-1694-sta-intent-forms-closeout.md`) closed V1-1694, V1-1695, V1-1665
and V1-1697. `corvint cem gate-verdict` already gives a manual read-only dry run. Residual: a
preflight with attempt placeholder substitution.

## V1-2254: list OPEN tickets referenced by main commits (ALREADY-DELIVERED)

- `ticket closeout-debt` (V1-1615, TCD-V0-001..010) lists OPEN tickets cited by intent-branch
  commits, with the commit evidence, as a pure read.
- `ticket batch-delivered` and `ticket delivery-screen` (PR #1442, `a78fb4c5f`, TCD-V0-159..164)
  cover batches.
- Tests: `TestTCDV0005_CandidatesThroughTheCLI`, `TestTCDV0163_BatchDeliveredListsLaneTicketsWithGreenRuns`
  and `TestTCDV0159_DeliveryScreenVerdicts`.

## Evidence, NOT_RUN and rollback

- **Evidence.** The focused tests are named in the TCD traceability rows and the NTP-V0-026 row.
- **NOT_RUN.**
  - A failing live `focused-docs` gate run with the new result.
  - The tier 2 `corvint affected` units, left to batch CI.
  - `make gate`.
- **Rollback.** Revert the change. There is no store format, wire codec, receipt or policy change.
  The shared-path fragments are store policy writes that the orchestrator owns, removable by a
  later `policy update`.
