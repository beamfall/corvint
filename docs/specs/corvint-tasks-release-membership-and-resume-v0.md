# Corvint Tasks release membership and lease resume V0

Owner: Russell Lewis
Date: 2026-10-30
Intent status: proposed
Delivery status: experimental

Authoritative inputs: native tickets V1-3193, V1-3189, V1-3190 and V1-3191 (friction filed from
lane self-use), the release mutation contract `taskman-release-mutation/0` (TM-V0-028, TM-V0-029) and
the [agent lease contract](corvint-tasks-agent-leases-v0.md) CAL-V0-021 and CAL-V0-025. The lease
contract's Requirements section is at the 256-obligation OCM limit (`maxObligations`), so these
amendments live here.

## Agent digest
- Claim: Release membership edits alone; widen PAUSED names admission and recovery; a resumed claim restores its widened scope.
- Status: proposed; experimental; TRM-V0-001 to TRM-V0-007 and TRM-V0-800 to TRM-V0-802 are implemented with focused CLI tests.
- Exists: `release membership --add/--remove` (wire op `RELEASE_MEMBERSHIP`), CLI flag-built membership payload sorted (create/update keep V1-0750 strict set order), the widen PAUSED remedy detail, claim scope restoration with a `restoredScope` claim-result field, and the `--admit-under-barrier` claim/widen exemption for one named ticket under an OPERATOR admission barrier.
- Blocked on: nothing.
- Read next: Requirements; Failure modes; Acceptance evidence.

## User and current state

Lanes add tickets to an open release often. Before this change the only route was
`release update`, which takes the whole closed release payload, so a one-ticket edit restated the
title, version, criteria, gates and predecessors, and a resent `ticketIds` array that was not in
canonical byte order was refused `MALFORMED` (V1-3193). Under an OPERATOR admission barrier a
lane that needed one more path got a bare "an admission barrier is present" from `widen`, with no
statement that widen is an admission or what to do (V1-3189). A lane that released a widened
attempt and resumed it could not claim the widened scope back: a `--scope` path outside the
declared scope was refused `OUT_OF_SCOPE`, so the resume took a claim plus a widen, and under a
barrier the widen was refused too (V1-3190).

## Requirements

- `TRM-V0-001`: (proposed; V1-3193) `corvint-tasks release membership --target R
  --expected-revision N (--add T | --remove T)...` MUST apply one `RELEASE_MEMBERSHIP` mutation
  whose closed payload is `{"add":[...],"remove":[...]}` and MUST change only the release's
  `ticketIds`. Every other release field (title, version, acceptance criteria, required gates,
  predecessors) stays byte-identical. Like `RELEASE_UPDATE`, the edit clears any candidate and its
  attestations, because the membership the candidate bound has changed.
- `TRM-V0-002`: (proposed; V1-3193) The mutation MUST be refused `STALE_TICKET` when
  `--expected-revision` is not the release's current revision, absent when the release does not
  exist, and refused on a promoted release, exactly as `RELEASE_UPDATE` is. It MUST be refused
  `MALFORMED` when it adds an existing member or an unknown ticket, removes a non-member, names a
  ticket in both lists, or names none.
- `TRM-V0-003`: (proposed; V1-3193) `RELEASE_MEMBERSHIP` MUST be authorized exactly as
  `RELEASE_UPDATE` is under the role matrix and any policy role row, so a policy that removes
  update from a role also removes membership edits. Policy files need no new operation name.
- `TRM-V0-004`: (proposed; V1-3193) The CLI MUST build the membership payload from repeated
  `--add` and `--remove` flags (local ticket tokens qualify to the queue) and MUST sort each list
  by canonical bytes before validation, so flag order never causes a `MALFORMED` refusal; a
  repeated ticket is still refused `MALFORMED`. A caller-serialized `release create` or
  `release update` payload keeps strict set order with its fix hint (V1-0750), unchanged.
- `TRM-V0-005`: (proposed; V1-3189) Widen on a live attempt under an admission barrier MUST stay
  refused `PAUSED` (CAL-V0-025, TCP-00 §3.4: widen adds paths to a live reservation, which is an
  admission, and project authority over the barrier outranks lane convenience). The refusal detail
  MUST say that widen is an admission and MUST name the recovery: the owner or operator runs
  `corvint-tasks unpause`, widens and restores the barrier with `corvint-tasks pause`; or the
  attempt releases and a later claim restores its widened scope (TRM-V0-006). The attempt's
  existing reservation stays held.
- `TRM-V0-006`: (proposed; V1-3190) A claim whose requested `--scope` paths fall outside the
  ticket's declared scope MUST admit each such path that a PATH resource of the ticket's last
  ended generation covers, when that generation is the one the claim retries (same acceptance
  revision, not COMPLETED) and its scope source was `DECLARED`. Precedence is DECLARED plus
  RESTORED: the restored paths join the declared PATH resources, and the scope source stays
  `DECLARED`, the same scope a widen of that generation produced. Provenance: the claim's ADMIT
  receipt binds the ended generation's attempt record as a Pre entry and records the new
  generation, whose `priorGenerations` names the ended one, as its Post; the claim result carries
  `restoredScope` `{fromGeneration, paths}`. A claim without `--scope` keeps the CAL-V0-021
  declared scope unchanged, and a `--scope-from-diff` path is never restored (TCD-V0-175).
- `TRM-V0-007`: (proposed; V1-3190) A requested path that no attempt of the ticket held MUST
  still be refused `OUT_OF_SCOPE` with the ATR-V0-214 detail, naming only the unrestorable paths.
- `TRM-V0-800`: (proposed; V1-3191) `corvint-tasks claim <ticket> --admit-under-barrier` and
  `corvint-tasks widen --attempt A --generation G --admit-under-barrier` MUST be admitted under an
  admission barrier whose scope is `ADMISSION` and whose reason is `OPERATOR`, when the actor role
  is `OWNER` or `OPERATOR`, and MUST otherwise apply every check a claim or widen applies without a
  barrier. The exemption covers only the one named ticket or live attempt of that request; it
  never changes, lifts or restores the barrier record, so every other request stays refused
  `PAUSED` while the barrier holds.
- `TRM-V0-801`: (proposed; V1-3191) The exemption MUST be explicit and recorded: the flag joins
  the lease request preimage as `admitUnderBarrier: true` (and is omitted when absent, so every
  earlier request keeps its digest), so the ADMIT or WIDEN receipt's request index entry binds it
  and a replay without the flag conflicts. `claim --next`, a `WORKER`, `REVIEWER` or other role,
  a barrier of scope `ALL`, and a barrier with reason `CUTOVER`, `EMERGENCY` or `DRAIN` MUST NOT
  be exempted: the flag on `--next` is refused `MALFORMED`, and the other cases stay refused
  `PAUSED` with a detail naming why the exemption does not apply. Without a barrier the flag
  changes nothing.
- `TRM-V0-802`: (proposed; V1-3191) The `PAUSED` refusal of a named claim, and the widen remedy of
  TRM-V0-005, MUST name the `--admit-under-barrier` route alongside unpause and pause.

## Non-goals

No new barrier kind; no exemption other than the explicit, per-request TRM-V0-800 one, and none
for `claim --next` or a non-OPERATOR barrier; no change to the CAL-V0-021 precedence for a claim
without `--scope`; no restoration across acceptance revisions, from a COMPLETED attempt, or from
an attempt whose scope was WHOLE_REPOSITORY, REQUESTED or DERIVED; no new stored attempt or receipt
field; no policy operation name for membership.

## Failure modes

A membership edit racing a full update fails the later one `STALE_TICKET`, as two updates do.
Removing the last member leaves a release with no tickets; readiness then reports it as it reports
any empty release. A restored path is admitted again only if no live attempt collides with it
(the CAL-V0-023 collision check runs on the whole scope), so restoration never steals a path
another lane holds. A restored scope larger than `MaxScopeResources` fails `LIMIT_EXCEEDED`
before any write. Because restoration keys on the last attempt at the same acceptance revision, an
acceptance edit after the widen ends restoration; the claim then refuses `OUT_OF_SCOPE` as before
and the lane widens. The PAUSED widen detail is prose; machine callers keep keying on the
`PAUSED` code. The CLI's default role is `OPERATOR`, so the role check alone does not stop a lane
from passing `--admit-under-barrier`; the gate is that the flag is explicit, per request and bound
in the receipt's request digest, the same trust as an `unpause` a lane may already run, but scoped
to one ticket. An exempted claim still refuses a colliding path (CAL-V0-023), so the exemption never
takes a path another attempt holds.

## Acceptance evidence

- `TestTRMV0001_ReleaseMembershipEditsOnlyMembership` (TRM-V0-001 to TRM-V0-004) in
  `internal/tasks/cli/release_membership_test.go` covers add then remove under
  `--expected-revision`, all other `release show` fields unchanged, a stale revision refused
  `STALE_TICKET` with membership unchanged, unsorted `--add` values accepted, member re-add,
  non-member remove, add-and-remove of one ticket and an empty edit refused, and a clean
  `receipt audit`.
- `TestTRMV0005_WidenUnderBarrierNamesAdmissionAndRecovery` (TRM-V0-005) covers claim, OPERATOR
  pause, widen refused `PAUSED` with the admission statement and recovery, and the same widen
  admitted after unpause.
- `TestTRMV0006_ResumedClaimRestoresWidenedScope` (TRM-V0-006, TRM-V0-007) covers claim, widen,
  release, the measured old resume route (claim plus widen: 2 mutations), release again, a
  never-admitted path refused `OUT_OF_SCOPE` naming only that path, and the new resume route
  (one claim: 1 mutation) restoring the widened scope with `restoredScope`, the same attempt
  identity, the receipt's Pre/Post binding of the attempt record and `priorGenerations`.

- `TestTRMV0800_AdmitUnderBarrierNamesOneTicket` (TRM-V0-800 to TRM-V0-802) in
  `internal/tasks/cli/widen_resume_test.go` covers an OPERATOR admission pause, a plain claim
  refused `PAUSED` naming the route, an exempted claim and widen of one ticket admitted with the
  barrier record byte-identical afterwards, another ticket's plain claim and `claim --next` still
  refused, the flag on `--next` refused `MALFORMED`, a replay of the exempted request id without
  the flag not reusing it, and a clean `receipt audit`.
  `TestTRMV0801_BarrierExemptionAppliesOnlyToOperatorAdmission` in
  `internal/tasks/transaction/lease_barrier_exemption_test.go` covers the planner rule: no barrier,
  no flag, OWNER and OPERATOR admitted, WORKER and REVIEWER refused, and `EMERGENCY`, `CUTOVER`
  and `DRAIN` barriers refused, each with its detail.

## Rollback

Revert the source. A membership edit writes an ordinary release record under an ordinary
`RELEASE` receipt, and release envelopes are decoded only when a new request is planned, so a
store written with TRM-V0-001 reads unchanged under the reverted binary; only a replay of a
membership request ID is refused there, as an unknown operation. A reverted claim refuses restored paths
`OUT_OF_SCOPE` again; attempts admitted with restored scope are ordinary DECLARED-source attempts.
A reverted binary refuses `--admit-under-barrier` as an unknown flag; attempts admitted under the
exemption are ordinary attempts, and their recorded request digests replay only under this binary.
