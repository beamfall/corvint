# 2026-10-31 cem defer counts an oversized intent

Lane win-w2 hit this while resealing. `docs/specs/affected-plan-v0.md` had 263 requirements, over
the 256-obligation OCM per-map bound, so it could not be declared as an intent. The documented
remedy, a `deferred` citation row, also failed: `cem defer` refused with `invalid-arguments` and
`--intent "docs/specs/affected-plan-v0.md" is not an intent document: too-many-obligations`.

- Cause: the counter `corvint` installs for `cem defer` (`countIntentObligations` in
  `cmd/corvint/ocm.go`) used `lrfrepo.IntentRequirements`, which applies the OCM per-map bound.
  CEM-V1-644 already admits counts 1..65536, so the code was stricter than its contract.
- Decision: `lrfrepo.IntentObligationCount` applies the OCM-V0-001 Requirements rule with no
  obligation bound, and the counter uses it. Every other rule refusal (no Requirements heading,
  no IDs, duplicate IDs, non-UTF-8) is unchanged. The bounded path is unchanged for every OCM
  caller: `requirementsFromBlob` now wraps a `scanRequirements` that takes the bound. That scanner
  keeps the same lines in `internal/lrfrepo/ocm.go`, so existing line citations stay valid.
- The status-side check (CEM-V1-645) uses the same counter, so a recorded oversized deferral now
  verifies its count rather than reporting not-an-intent.
- Spec: CEM-V1-644 now states that the counter does not apply the per-map bound. Its trace row
  adds `TestCEMV1644CounterAdmitsAnIntentOverTheOCMBound` (`cmd/corvint`), which counts 257
  obligations and still refuses a non-intent; it fails without the fix.
- Rollback: revert the fix commit; the spec sentence then overstates the behavior and should be
  reverted with it.
