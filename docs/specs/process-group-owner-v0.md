# Process-group Owner V0

Owner: Russell Lewis
Date: 2026-10-03
Intent status: proposed extraction of existing owner-authorized repair semantics
Delivery status: experimental

## Agent digest
- Claim: An optional process-group Owner preserves leader identity through retirement and reports unproved cleanup as HOLD.
- Status: proposed extraction of existing owner-authorized repair semantics; experimental.
- Exists: Exact reviewed six-file Owner extraction from public PR443 head f3fba61ba0a4b0192e89a0ad9f1128c021ae5da9; no caller or legacy Wait/Run changes.
- Blocked on: Hosted amd64 qualification, integration and full V1-0668 acceptance; existing repair authority does not imply new human acceptance.
- Read next: Requirements, Acceptance evidence, Rollout and rollback.

## Owner context and current state

GitHub #481 (native V1-0677) needs process ownership for attempt-scoped commands;
GitHub #464 (native V1-0654) needs cancellation-safe work-state command handling.
This dependency supplies the ownership primitive, not their heartbeat, lease,
receipt or dispatcher event behavior. Main at dd8cc0ca918a80faaa041c98a783de11550b2bcc
contains legacy groupreap helpers but none of the six Owner files. Existing
helpers are byte-identical to the donor. Public PR443 supplies the complete
reviewed implementation; reimplementation is unnecessary.

The semantics below extract the donor's docs/specs/cem-stable-v1.md retirement
section, its 2026-10-02-cem-linux-retirement-repair build log, and V1-0668 criteria.
Those donor documents are historical evidence, not newly accepted intent on main.
CEM-V1-001/CEM-V1-007 and V1-0668 full qualification obligations remain unchanged.

## Requirements

- `PGO-V0-001`: Own a process group through its unreaped leader; send at most one real group signal before reaping begins and never send a real numeric group signal after reaping begins. Scans and signal-0 probes confer no signal authority.
- `PGO-V0-002`: Preserve Darwin quiet-first retirement and Linux ReapAfterSuccessfulSignal retirement. Preserve Linux signal failures, including ESRCH and EPERM, as failure evidence.
- `PGO-V0-003`: Check the original retirement allowance before new primitives and before accepting absence; intersect it with the fixed two-second post-reap cap. Retain terminal HOLD for unproved cleanup and late asynchronous reap completion.
- `PGO-V0-004`: Serialize concurrent Finish and Stop so signalling and reaping occur once. Refuse invalid modes and unsupported platforms before process creation.
- `PGO-V0-005`: Preserve existing Wait/Run and callers. Only RELEASED proves cleanup; HOLD must remain explicit and must not trigger a weaker fallback or success claim.

## Non-goals and baseline

No lease/heartbeat implementation, dispatcher event changes, aggregate positive
event delivery, CEM promotion, legacy Wait/Run repair, escaped-session containment,
account-wide process management or release qualification. The simpler baseline is
the existing helper; it is retained but does not provide the new Owner contract.

## Trust boundary and failures

Only the owned live handle grants signal authority. Disk records, saved PIDs and
producer assertions grant none. Signal, observation, probe and reap failures keep
cleanup unproved. Expiration cannot be extended by retry or late success. An
unsupported platform refuses before spawn. The private test controller must own,
join and clean up its children even when interrupted; scans are observations only.

## Traceability and acceptance evidence

| Requirement | Implementation and executable witness | Current evidence limit |
|---|---|---|
| PGO-V0-001 | internal/groupreap/owner.go; TestOwnerNormalExitRetiresProbesReapsAndObservesAbsence; TestOwnerStopSignalsOnceAndFinishDoesNotSignalAgain | Exact donor bytes; Darwin tests/race observed; final target checks pending |
| PGO-V0-002 | internal/groupreap/owner_waitid.go; TestOwnerReapAfterSuccessfulSignalSkipsPreReapQuiet; TestLinuxOwnerPreservesKillGroupESRCH | Actual extracted-source Linux arm64 controls pass; hosted amd64 pending |
| PGO-V0-003 | TestOwnerExpiredAllowanceDoesNotStartWork; TestOwnerSignalConsumesAllowanceBeforeReap; TestOwnerPostReapCapWins; TestOwnerSlowReapLateCompletionKeepsStickyHold; TestOwnerLinuxModeRetirementBoundary | Deterministic matrix retained and passes on Darwin and Linux arm64 |
| PGO-V0-004 | TestOwnerRejectsInvalidRetirementMode; TestOwnerConcurrentFinishAndStopSignalAndReapOnce; internal/groupreap/owner_other.go | Darwin race and native checks pass; Windows unavailable-owner compilation passes |
| PGO-V0-005 | Exact six-file source extraction and unchanged legacy package blob comparison | Source equality is not caller integration or runtime proof |

Required candidate checks: focused groupreap tests and vet, race for concurrent
Finish/Stop, actual Darwin and available Linux arm64 lifecycle tests with owned
cleanup, and actual hosted Linux amd64 lifecycle execution. Compilation cannot
replace native evidence. Preserve all original failed runs and historical source
bindings; skipped or unavailable cases remain explicit. Existing broader CEM
results are not rerun merely to extract this dependency. V1-0668 stays OPEN until
its full original criteria, including both Linux architectures and stable/candidate
qualification, are satisfied.

## Rollout and rollback

Register and commit this intent seed before implementation enrollment. The CODE
base is that verified seed commit, descended from the captured remote main; never
enroll against an absent donor-only intent or invent a seed SHA. Preserve exact
six-file source bytes and AGPL-3.0-or-later path terms. Keep the slice experimental.
Use one existing source review for unchanged bytes; assess changed integration
context and final evidence without a new architecture panel. Final delivery needs
clean CEM binding/check/seal, scoped CI, publication and authorized integration.
A docs seed creates no runtime claim. Before consumers land, rollback reverts the
isolated addition; afterward roll back the dependent callers coherently.

## Open decisions and promotion

Owner semantics are unchanged; this proposed extraction does not accept the CEM
technical contract or promote a stable product. Native hosted amd64 qualification,
full V1-0668 criteria and each dependent caller remain separately tracked. Missing
evidence is uncertainty. Do not complete the ticket based on the dependency alone.
