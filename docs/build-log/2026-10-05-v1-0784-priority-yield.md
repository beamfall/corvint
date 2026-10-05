## 2026-10-05 V1-0784: priority-yield admission for explicit pooled claims

Human-owned intent: [issue 583](https://github.com/beamfall/corvint/issues/583) (ticket V1-0784).
An explicit `claim <ticket> --pool P` took P's last free member even when a higher-priority ticket
needing P was waiting. The owner directed an opt-in, derived priority yield with no new state, no
new result code and no waitlist; V1-0785 (waitlist) is out of scope and only assessed below.

Requirement: `CAL-V0-101` in the "V1-0784 priority-yield admission amendment" section of
`docs/specs/corvint-tasks-agent-leases-v0.md` (inside `## Requirements`, after S22), with TCP-00
amendment A20 for the policy key. Built on the V1-0786 head (662051a0, CAL-V0-097).

### Change

- `intent.Pool.PriorityAdmission`: optional boolean `pools[].priorityAdmission` in the closed pool
  object. Omission keeps the canonical policy bytes; a non-boolean refuses `MALFORMED`. The member
  definition digest does not include it, so toggling it never disturbs an occupied member.
- `transaction.priorityWaiting`: in plan order (`planLess`), the `OPEN` tickets ahead of the claimed
  one that record `requiresPool` P and have no claim blocker (a live attempt is one). Tickets whose
  only blockers are unknown are returned separately and never count.
- `transaction.priorityYield`: yields to the first competitor when the flag is set, the pool has a
  free eligible member, and competitors are at least as many as free members.
- `leaseContext.yieldRefusal`, called from `admit` after the claimed ticket's live-attempt check and
  before collision, capacity, retry and allocation (so before health preparation): refuses
  `BLOCKED RESOURCE_COLLISION` with detail `pool P priority admission: N higher-priority waiting
  ticket(s) for F free eligible member(s); yields to T`. Free members are `poolSlots` of the claim's
  own stage, exclusions and prepared allocation.
- `PriorityFirst`/`choose`: a running per-pool list of non-blocked `OPEN` pool tickets feeds the same
  `priorityYield`. In a `--pool` plan the yield precedes the slot cap; in the default plan it follows
  the unobserved/unclaimable deferral and precedes the free-member cap, sets `poolDeferred` and so
  counts in `resourceDeferred.deferred`. The entry is `DEFERRED RESOURCE_COLLISION` with the yielded-to
  ticket ID as blocker, which Core's `taskman-plan/0` decoder already admits (no Core change). Because
  `claim --next` and the dispatcher take only `SELECTED` entries, neither picks a yielded ticket.
- `RecordedClaimability` (`ticket show`): `null`/`NOT_OBSERVED` when counting the unobservable
  competitors would make the ticket yield.

### Evidence

- `TestCALV0101_ExplicitPooledClaimYields`: refusal shape and exact detail, no posts, an unpooled
  claim and `claim --next --pool` unaffected, no yield to a ticket with a live attempt.
- `TestCALV0101_FlagOffMatchesNMinusOne`: a scenario transcript (posted bytes, refusals, both plans,
  every claimability) with the flag absent equals a digest pinned by running the same fixture file
  on the base source (662051a0) in a temporary worktree. With `false` it is equal after normalizing
  policy identity (the raw policy SHA-256 and digest embedded in attempt posts differ because the
  policy bytes differ); with `true` it differs. Absent-key bytes round-trip, and bad values refuse.
- `TestCALV0101_PlanClaimAndClaimNextAgree`: 400 seeded random queues (1-3 members, flag on 80%,
  mixed priorities, pooled and unpooled tickets, shared paths, `GATE_PASSED` dependencies,
  pre-claims). For every non-blocked entry of the `--pool` plan the explicit claim yields exactly
  when, and to whom, the plan defers; `SELECTED` entries are admitted; `claim --next` never yields
  and matches `ClaimNext`; default-plan pooled entries agree. Disabling the yield in the claim path
  makes it fail.
- `TestCALV0101_UnobservedCompetitorIsNotObserved`: a higher-priority competitor with a `GATE_PASSED`
  dependency causes no refusal or deferral, and `ticket show` reports `NOT_OBSERVED`.
- `TestCALV0101_PriorityYieldThroughTheCLI`: native store, both previews show the deferral and leave
  state and intent trees unchanged, the explicit claim is refused naming the competitor, and
  `claim --next --pool` admits the competitor.

Limits: live multi-agent or dispatcher qualification is NOT_RUN.

### Design notes and owner questions

- Competitors that collide on scope with a live reservation still count (collisions are not claim
  blockers), so a claim can yield to a ticket that cannot start yet.
- The detail names the head-of-line competitor, not necessarily the one that would take the member.
- A competitor that becomes eligible between health preparation and the re-claim makes the re-claim
  yield; the existing observation path then quarantines the prepared member until confirmation.
- Supervisor stage-transition allocation is not a claim and is unchanged.

V1-0785 assessment: still needed if the owner wants a visible queue or hand-off. The yield protects
only competitors that are claim-eligible at the moment of the claim; it reserves nothing across a
gap where a competitor is briefly blocked, unobservable or between attempts, and the plan shows only
per-entry blockers, not a per-pool waitlist.

Rollback: an older binary refuses a policy carrying `priorityAdmission` (even `false`) as `MALFORMED`
through its closed pool decoder. Remove the key with `policy update` on the new binary, then revert
the code. No store, journal, receipt or pool state depends on the flag.
