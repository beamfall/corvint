# 2026-11-12: sealed CEMs are change evidence, not Corvint configuration (V1-5424)

## Finding

PR 197's `selection-baseline-check` failed: 1 of 43 merged changes narrowed, below the floor of 33.
The 2026-11-11 baseline before the non-code map (`2026-11-11-affstatus.md`) narrowed 36. The replay
moved 34 merged changes from NARROW to FULL with the first reason `corvint-config
.corvint/changes/<commit>.cem.json`. Nearly every merged change commits its sealed CEM, so treating
it as configuration (NCM-V1-006, design open question 1) made almost every change a full run. The
cause is this batch's NCM work, not the environment or the recorded history.

## Decision

PR 197 repair call, made to meet the owner floor without lowering it; the orchestrator may revisit it. A
sealed CEM directly under `.corvint/changes/`, named by a 40- or 64-digit lowercase hexadecimal
commit, is change evidence. `noncodemap.Decide` gives it no line, so the selector keeps its
ADN-V0-011 attribution: the readers the plan already selects, else rule (c). Any other name under
`changes/`, the sidecar `.corvint/change.cem.json`, and every other `.corvint/` file stay
`corvint-config`. `gateselect.sealedEvidence` now delegates to `noncodemap.SealedEvidence`, so the
two rules share one shape.

R31 forces a full run only for configuration. A sealed CEM configures nothing: no planner or
selector reads it as a declaration, and ADN-V0-011 already bounds what its change runs.

## Evidence

- `TestNCMV1006CorvintConfigSelectsEverything` (`internal/noncodemap`): a sealed name gets no line.
  Upper-case and nested names still run everything.
- The gateselect tests that pinned the sealed CEM to `corvint-config` now pin both selectors to the
  attribution verdict (`requireCorvintConfig`). `TestAddedRecordEntryBothSelectors_AFPV3423` keeps
  the sealed CEM's narrowing.
- `TestADNV0010And0011ProducerToConsumer` and `TestDriverSelectionCoversTheShadow_V13067` are back
  to their origin/main assertions.
- The `selection-baseline-check` result is recorded in the PR 197 repair report.

## Rollback

Revert the commit. Sealed CEMs then select everything again, and the selection baseline stays below
its floor.
