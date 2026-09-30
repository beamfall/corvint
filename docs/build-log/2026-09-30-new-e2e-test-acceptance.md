# New E2E test assessment: experimental negative evidence

Date: 2026-09-30
Human intent: GitHub #394, native V1-0541.
Contract: `docs/specs/new-e2e-test-acceptance-v0.md`, proposed/experimental.
Public base: `8f4b866a4ed201a52d8c63196050a9623a449dfe`.

## Decision and qualification

A separate optional companion composes actual provider repeats and the independently pinned
behavior-falsification executable. It freezes closed request bytes, verifies clean immutable source
and tracked inputs around execution, isolates both paths in credential-free self-workers, retains
bounded descendant observations, and renders a fixed PR body. Caller pins never promote validity.

Gate A found that current per-test provider ordinary outcomes carry ClaimFacts without execution
freshness, and that `RequireDescendantCleanup` is an unsupported pre-launch refusal. The plan
therefore preserves Freshness UNKNOWN, uses actual bounded descendant observation, and blocks
stable asserting acceptance. Deduplicated native V1-0556 (receipt1360) records the positive
freshness capability gap. It requires its own observed contract/qualification; no provider source
was changed under this claim. A qualified accepted happy path remains unsupported.

Actual local Playwright assessment passed at Node22.23.3/Chromium153.0.8010.12: two zero-retry
baseline runs each retained all three tests, a stable assertion was killed by a disposable response
mutation but remained BLOCKED for unknown freshness/qualified identity, an unchanged nonasserting
test survived that mutation and was REJECTED, and an alternating test was REJECTED with both
requested order probes retained. Original/reversed CLI filters are not an observed schedule;
owned-provider Schedule stays UNKNOWN. All known browser/runner descendants were observed absent;
20ms sampling can miss fast detach and is not full containment. Writer credential sentinels were
absent from repeat/server/control/cleanup environments. Separate-group interruption has a focused
actual process test. Full browser tuple/import closure and authenticated hook/operator semantics
are unqualified. Node22.23.3 differs from the native external provider qualified Node22.23.2 tuple.

Qualification initially failed closed: scratch root canonicalization, fixture grep selection,
assertion discriminator and staged HOME/browser-cache assumptions were corrected. Raw failed
observations remained private scratch evidence. Final fixture explicitly selects installed cache
and runs the same source tests against the mutated disposable response; it installs no dependency
and executes no remote request. Unit-only success does not imply live browser qualification.

## Verification and remaining delivery

Focused Go tests/vet are retained with the lane checkpoint. Sole independent review found and
repaired direct-worker inherited credentials, provider-axis overwrite, post-allocation Git output
bounding/replacement objects, and missing aggregate verdict. Repair retains explicit worker
environment rejection, bounded immutable Git reads, unchanged provider axes and rejected-over-blocked
aggregate precedence. Sole re-review passed after one repair round; the repaired actual browser
qualification also passed (24.30s): all three test rows in two baselines and two probes, stable/flaky
controls killed, nonasserting control survived, overall REJECTED, stable row BLOCKED. Every
provider row axis is preserved; requested versus observed schedule remains separate.
Native focused-docs, shared spec registry integration, final CEM/OCM binding/strict finish/check/seal,
draft publication and integration/native completion remain pending until their actual receipts.
Peer generation53 owns shared registries/CEM paths; this source-only claim does not bypass it.
Keep V1-0541 open. Rollback stops the optional companion and reverts its isolated source commits;
retain assessment evidence and V1-0556. No Core surface, queue migration or outward connector write.
