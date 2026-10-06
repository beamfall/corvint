# 2026-10-06: V1-0625 policy heartbeat TTL and stale-holder advice

## Intent

Ticket V1-0625, GitHub [beamfall/corvint#426](https://github.com/beamfall/corvint/issues/426). Dead
agents' attempts held their tickets and lanes until the work lease expired, and coordinators could
not tell orphaned holders from live ones. The acceptance criteria ask for:

- a generation-fenced heartbeat that does not extend the lease;
- reads that distinguish fresh, stale and unobserved holders, where stale frees nothing;
- focused tests.

## Already on main (base 76f7f2ac)

CAL-V0-048 (commit cdad1b74) already delivered most of the ticket. Ticket V1-0625 was filed against
build 202, before that commit.

- `attempt heartbeat --attempt --generation --request-id` (`transaction.planHeartbeat`) records
  `lastHeartbeatAt` through the journal writer. It uses ordinary generation, terminal and
  lease-expiry fencing, and does not touch the work lease. Claims initialize the signal,
  readmission resets it, and replay does not refresh it. PR #638 (CAL-V0-111) added `--lock-wait`.
- `attempt show` and `queue status.liveAttempts` already derive `holderStatus` on read. The values
  are FRESH_HOLDER, STALE_HOLDER, NOT_OBSERVED (legacy absence), LEASE_EXPIRED, TERMINAL and
  CLOCK_BEFORE_HEARTBEAT, reported with `lastHeartbeatAt`, `observedAt` and a fixed
  `heartbeatTTLSeconds:"600"`.
- Existing evidence: `TestCALV0048_HeartbeatLegacyRoundTrip`,
  `TestCALV0048_HeartbeatFenceReplayAndLeaseInvariant`, `TestCALV0048_HeartbeatCLIReplayAndFence`
  and `TestCALV0048_HolderObservationBoundaries`.

## Change

- CAL-V0-115: the optional top-level policy key `holderLiveness {heartbeatTTLSeconds}`, a closed
  count from 300 to 86400. Omission keeps the policy bytes and the 600-second default.
  `intent.Policy.HeartbeatTTLSeconds()` returns the effective TTL. `addHolderObservation` classifies
  against it, and the reads report it as `heartbeatTTLSeconds`.
- CAL-V0-116: `release --help` states that STALE_HOLDER is advisory input to a coordinator's
  evidence HANDOFF, not authority, and that nothing releases a holder because it is stale. The
  `attempt heartbeat` help names the policy TTL. `docs/TASKS-EXTERNAL-AGENTS.md` documents both.
- Spec: a V1-0625 subsection, TCP-00 amendment A24, a slices row, traceability rows and the status
  line (mirrored in `docs/specs/README.md` and `INDEX.json`). CAL-V0-048 now refers to the effective
  TTL. The changes stay in the intent, cli and docs layers. `store.Mutate`, the audit and the
  journal are untouched, because the V1-0645 lane is changing them concurrently.

## Decisions

- **Keep the `NOT_OBSERVED` spelling.** The lane brief said UNOBSERVED. The wire value has been
  NOT_OBSERVED since CAL-V0-048, so renaming it would break readers. Legacy absence is NOT_OBSERVED
  at every TTL, never STALE_HOLDER.
- **The TTL minimum is 300 seconds.** That keeps the TTL above the 240-second `attempt run`
  heartbeat interval (`attemptBeatInterval`), so a supervised run is never reported stale between
  its own beats.
- **A TTL change stays an ordinary policy change (fail-closed).** The handoff policy-compatibility
  rule lives in the journal audit (`HandoffPolicy.Compatible`), which is out of bounds for this lane.
  So a TTL change fences live evidence handoffs `STALE_POLICY` and gate runs claimed under the old
  policy, like any other policy change. This is documented as a failure mode and raised as an owner
  question.
- **Policy key, not a read flag.** This follows the orchestrator's design. The cost is the A20/A23
  downgrade rule: once written, older binaries refuse the store.
- **Requirement IDs.** CAL-V0-114 is taken by the concurrent V1-0864 lane
  (`origin/claude/v1-0864-projection-order`), so this change uses CAL-V0-115..116 and A24.

## Evidence

- `TestCALV0115_PolicyHolderLivenessOptIn` (`internal/tasks/intent`): default bytes and TTL,
  accepted bounds, and refusal of below/above range, zero, non-count, unknown member, null, `{}` and
  non-object.
- `TestCALV0115_HolderObservationUsesPolicyTTL` (`internal/tasks/cli`): the TTL boundary for 300 and
  1800, and legacy NOT_OBSERVED at 300 and 86400.
- `TestCALV0115_PolicyTTLDrivesHolderReads` (`internal/tasks/cli`): a claim signal about 360 s old
  under a renewed live lease reads FRESH under the default, then STALE after a policy update to 300.
  The reads leave the state tree byte-identical. The attempt keeps its phase and lease, and another
  holder's claim still refuses ATTEMPT_LIVE. An older-generation heartbeat refuses FENCED. A current
  heartbeat makes the holder FRESH again without moving the lease.
- `TestCALV0116_StaleHolderCoordinatorHandoff` (`internal/tasks/cli`): under a steady 300-second
  policy, a STALE_HOLDER attempt is handed off by a coordinator's evidence HANDOFF release through
  the ordinary writer. It then reads TERMINAL, a successor claims, `retries.charged` stays 0, and
  the release help carries the advisory line.
- Negative check: with the source change reverted, the new tests do not compile (missing policy
  API). The CLI policy update carrying `holderLiveness` would refuse MALFORMED on the old decoder.

## Limits and NOT_RUN

- Live fleet qualification under real dead hosts: NOT_RUN.
- `make gate`: NOT_RUN, per the lane rules.
- Not delivered: a holder liveness token (session/PID), an aggregate stale count, and automatic
  reap or hand-off. These are non-goals.
