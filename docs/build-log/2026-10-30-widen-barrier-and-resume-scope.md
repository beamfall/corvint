# Widen under an admission barrier and resumed-claim scope — V1-3189, V1-3190

## V1-3189: keep the PAUSED refusal, explain it

Fork: admit widen under an OPERATOR admission barrier, or keep refusing it. Decision: keep
refusing (TRM-V0-005). CAL-V0-025 and TCP-00 §3.4 already classify scope-expand as an admission,
and the barrier is project authority, which outranks lane convenience (AGENTS.md invariant 3).
The fix is the refusal detail: it now states that widen admits new paths to a live reservation
and names the recovery (operator unpause, widen, pause; or release and a restoring claim).

## V1-3190: restore the ended generation's widened scope on a resumed claim

Decision: a claim's `--scope` paths outside the declared scope are admitted when a PATH resource
of the ticket's last ended generation covers them and that generation is the one the claim
retries (same acceptance revision, not COMPLETED, DECLARED source) (TRM-V0-006). The restored
paths join the DECLARED scope, so the stored scope is the one the widen produced; no new scope
source or attempt field was added, keeping the stored format readable by older binaries.
Provenance is structural: the ADMIT receipt's Pre entry binds the ended generation's attempt record
and its Post records the new generation with `priorGenerations` naming it; the claim result adds
`restoredScope {fromGeneration, paths}`. Restoration is opt-in through `--scope`, so a plain
claim keeps CAL-V0-021 unchanged, and `--scope-from-diff` never restores (TCD-V0-175). A path no
attempt held is still `OUT_OF_SCOPE` (TRM-V0-007).

Measured resume path (`TestTRMV0006_ResumedClaimRestoresWidenedScope`): claim plus widen,
2 mutations, before; one claim after. Under an admission barrier the old route's widen was also
refused, which added an unpause and a pause.

Evidence: `TestTRMV0005_WidenUnderBarrierNamesAdmissionAndRecovery` and
`TestTRMV0006_ResumedClaimRestoresWidenedScope` (WSL2, go1.27.3, PASS).
