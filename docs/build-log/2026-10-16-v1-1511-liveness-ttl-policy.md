# V1-1511: renew-only liveness and the effective heartbeat TTL

Date: 2026-10-16. Base: `ff96980fdf1d5897ee222a38283a2cba2bf76e8c` (origin/main).
Ticket: V1-1511, from a follow-up comment on GitHub issue 982 (receipt retention, V1-1508, is a
separate lane). Spec: `docs/specs/corvint-tasks-agent-leases-v0.md`, CAL-V0-277 and CAL-V0-278,
accepted under the owner's standing auto-accept of lane requirements (2026-10-11). No accepted
requirement is amended.

## What the issue asked and what already existed

The comment asked for two things:

- Help text saying a renew also records liveness, so callers need not pair a heartbeat with every
  renew. The reporting keeper had paired them on 74 of 74 renews.
- A policy-configurable heartbeat TTL.

Both behaviours were already delivered:

- CAL-V0-231 makes a current-generation renew set `lastHeartbeatAt` to its `recordedAt`.
- CAL-V0-120 has offered `holderLiveness.heartbeatTTLSeconds` since V1-0625: bounds 300..86400,
  default 600 when omitted, and older stores read unchanged.
- Every holder read already uses `Policy.HeartbeatTTLSeconds()`: `attempt show`, `queue status`
  and `doctor`.

The gaps were visibility and guidance:

- `policy show` printed only the stored policy. A policy without the key therefore showed no TTL,
  which is why the reporter took 600 to be fixed.
- The heartbeat help said nothing about pairing.
- No test held the renew-only invariant across a timeline.

So this lane adds no policy key and no bound, and changes no writer.

## Contract

- CAL-V0-277 (help). The heartbeat help states:
  - a renew records the same signal;
  - a heartbeat sent in the same tick as a renew changes no `holderStatus`;
  - freshness follows the last signal, not the lease.

  The renew help says not to pair the two. It also states the exact condition under which a
  renew-only holder cannot read `STALE_HOLDER`: `--lease-minutes`*60 is at most the TTL.

  This follows from the existing classification order. The renew sets the signal and `expiresAt`
  from one `recordedAt`, and `LEASE_EXPIRED` wins at equal times (CAL-V0-048). A holder whose lease
  is longer than the TTL and whose renews are more than a TTL apart does read `STALE_HOLDER` inside
  its lease. That is the documented meaning of the observation (a missing recent signal), not a
  defect, and the help now says so.
- CAL-V0-278 (`policy show`). The item gains a sibling `heartbeatTTLSeconds` member holding the
  effective value. It sits outside `policy`, so the policy bytes, `policySha256` and the CAL-V0-050
  item shape stay unchanged for existing policies.
- `docs/TASKS-EXTERNAL-AGENTS.md`:
  - It now carries the same guidance.
  - It corrects a stale sentence that said a TTL change fences live evidence handoffs
    `STALE_POLICY`. CAL-V0-123 made a `holderLiveness`-only change handoff-compatible. `complete`
    of an attempt claimed under the earlier policy still refuses `STALE_POLICY` because the
    completion blocker compares the claim's raw policy digest. The guidance to set the TTL before
    claims therefore stands.

## Evidence

- `TestCALV0277_RenewOnlyHolderNeverReadsStale` (`internal/tasks/cli`) builds a backdated timeline
  through the real lease writer: four holders, four ticks 540 s apart. It then classifies every
  second with the same `holderStatus` function the reads use:
  - The renew-only holder (10-minute lease) never reads stale.
  - The holder that pairs a heartbeat with each renew records the identical `lastHeartbeatAt` after
    every tick and the identical status at every second.
  - A 30-minute-lease holder renewing every 540 s never reads stale.
  - A 30-minute-lease holder renewing every 1080 s reads stale for exactly 1680 s inside its lease.
  - Live `attempt show` reads agree: FRESH, FRESH, FRESH, then STALE under TTL `"600"`.
- `TestCALV0278_PolicyShowEffectiveTTL` checks three things:
  - `policy show` of the default policy keeps the stored bytes and digest, has no `holderLiveness`,
    and reports `heartbeatTTLSeconds` `"600"`.
  - A heartbeat cadence of 1500 s reads stale for 1800 of 3000 seconds at TTL 600 and 0 at TTL 1800.
  - After `policy update` to 1800, `attempt show`, `queue status` and `policy show` all report
    FRESH and `"1800"` on the same unchanged lease.
- BASE against the fix: both new tests run against BASE code fail (2 of 2). TestCALV0277 fails only
  at its help assertion: every timeline assertion passed on BASE, which confirms there was no
  renew-liveness bug. TestCALV0278 fails at the missing `policy show` member. With the fix, 2 of 2
  pass. Load averages: 36.09 40.04 39.20.
- Mutations, each restored afterwards:
  - Dropping `next.LastHeartbeatAt = &at` from the renew planner fails TestCALV0277 at tick 0.
  - Dropping the `policy show` member fails TestCALV0278.
- Focused regression: `-run 'TestCALV0277|TestCALV0278|TestCALV0231|TestCALV0120|TestCALV0121|`
  `TestCALV0050|TestCALV0223|TestCALV0048|Help'` passed (load 14.65 21.45 32.31).

Limits:

- No live fleet or keeper qualification was run (NOT_RUN).
- The keeper's pairing is the caller's code and is not changed here.
- The 240 s `attempt run` heartbeat interval is unchanged.
