## 2026-10-05 V1-0786: resource-aware selection in the default plan

Human-owned intent: [issue 584](https://github.com/beamfall/corvint/issues/584) (ticket V1-0786).
`SELECTED` tickets waiting for a pool member filled the `maxActiveAttempts` window while lane-free
work deferred `LIMIT_EXCEEDED`. Owner answer 2026-10-05 (D1) chose option (a): cap pool-needing
selections at the pool's free eligible members, defer the excess `RESOURCE_COLLISION` outside the
window, add no policy setting, and report a per-pool resource-deferred count.

Requirement: `CAL-V0-097` (S24) in `docs/specs/corvint-tasks-agent-leases-v0.md`, with TCP-00
amendment A19. The ID is at least three above CAL-V0-073, the highest on
`origin/claude/batch-2026-10-05`. On integration the coordinator renamed the slice from S22 to S24,
because the batch's S22 is the Claude Code supervised host; CAL-V0-097 and A19 are unchanged.

### Change

- `transaction.choose`: in the default plan (no `--pool`), a ticket with a declared `requiresPool`
  is `SELECTED` only while earlier selections requiring the same pool are fewer than its free
  eligible members. Otherwise it defers `RESOURCE_COLLISION` with blockers `[<pool>]`, before the
  window check, so it never counts against `maxActiveAttempts`. Unobserved occupancy (nil pool state)
  defers. `poolAvailable` blocks only an undeclared pool in the default plan; the `--pool` branch
  and `poolSlots` keep their old behaviour, now sharing `freePoolMembers`.
- `TicketPlan.Pools` carries one `PoolSelection` per policy pool for a default plan.
  `plan preview` renders it as the additive top-level `resourceDeferred` array
  (`poolId`, `availability`, `freeEligibleMembers`, `selected`, `deferred`). The member is absent
  for a `--pool` plan, a policy without pools and `--selected-only`.
- `planInput` treats an audited absent `pools.json` as the empty occupancy, matching `loadPools`
  on the claim path, so a fresh pool is `OBSERVED` with every member free.
- `TicketPlan.ClaimNext(pool)`: without `--pool`, `claim --next` skips `SELECTED` pool entries,
  because a pool-less request consumes no pool (CAL-V0-029); when every `SELECTED` entry needs one,
  it refuses `RESOURCE_COLLISION` naming the first. `--pool` claim-next is unchanged.
- `RecordedClaimability` (`ticket show`) returns `null`/`NOT_OBSERVED` for a pool ticket whose
  member state is unobservable; with free members a pool ticket is now `true`.
- Operator guide notes that only tickets recording `requiresPool` benefit (V1-0754, V1-0758, V1-0759).

### Wire compatibility

`taskman-plan/0` gains an optional member and a pool-ID blocker in `DEFERRED` entries. Neither
appears unless the policy declares pools and the plan is the default one, so existing plan bytes
are unchanged. Following the A9/A15 additive /0 pattern there is no profile bump (A19). Core's
closed history decoder (`internal/taskman/decode.go` `decodePlan`) refused such plans in the first
revision (4ee27f86), which was recorded as needing owner confirmation. That limitation is resolved:
the round-two fix (b4aa8599) teaches `decodePlan` the optional member and pool blockers; see Core
compatibility below.

### Non-goals

Option (b) class windows or any policy setting; inferring `requiresPool`; stage-aware counting
without `--stage`; health probes; dispatcher launch-order changes.

### Evidence

- `internal/tasks/transaction/plan_resource_test.go`: the issue 584 fixture (capacity 16, 8 live, 6
  pool-waiting P0 tickets on a fully occupied pool, 28 lane-free) selects 8 lane-free tickets and
  defers 6 `[lanes]`; the cap at 2 free members; `NOT_OBSERVED` deferral and claimability; an
  undeclared pool still blocks; the `--pool` plan is unchanged.
- `internal/tasks/cli/plan_resource_test.go`: native preview rows and `resourceDeferred`, preview
  purity (state and intent trees unchanged), `claim --next` without a pool takes the first lane-free
  ticket, and an explicit `--pool` claim admits a `SELECTED` pool ticket.
- `go test ./internal/tasks/...`, vet on the touched packages, gofmt, the Windows cross-build and the
  CI doc gates; results are in the change report.

Limits: live dispatcher or multi-agent qualification is NOT_RUN. The fixture would also select 8
lane-free tickets under the old code, which blocked the pool tickets instead. The observed
starvation came from pool-waiting tickets without `requiresPool`, which this change cannot detect.

Dispatcher routing (independent review, Codex CHANGES_REQUIRED): the first revision left
`dispatch.Ticket` without `requiresPool`, so a generic `planSelected` role matched the newly
`SELECTED` pool tickets, its worker's claim without `--pool` was refused, the ticket cooled down and
parked, and the pool ticket kept holding the dispatcher's window from lane-free work. The fix
carries `requiresPool` into the dispatcher's ticket view, adds the role field `match.pool` (a pool
ticket matches only a role naming its pool, which binds `{pool}`; a role naming a pool takes only
that pool's tickets), and plans the dispatcher's view with `ClaimablePools` set to the pools the
ticket roles name, so an unclaimable pool's tickets defer before they count against
`maxActiveAttempts`. `TestCALV0097_DispatchRoutesPoolTicketsAndKeepsLaneFreeProgress` runs real
dispatcher ticks and CLI workers on a native store (`maxActiveAttempts` 1, one free member, a P0
pool ticket, a P2 lane-free ticket, `parkAfter` 1): the generic role claims the lane-free ticket and
never names the pool ticket in launch, cooldown or park events, and a `match.pool` role then
receives the pool ticket and its `--pool` claim is admitted. With the routing and plan changes
reverted the test fails at the first tick, where the generic role launches on the pool ticket.
`plan preview` and `claim --next` plans are unchanged by this (nil `ClaimablePools`).

Core compatibility: Core's closed `taskman-plan/0` decoder refused the new `resourceDeferred` member
and pool blockers. It now validates the rows (pool label, unique; `OBSERVED` with a count or
`NOT_OBSERVED` with null; counted `selected` and `deferred`) and accepts a pool blocker only on a
`DEFERRED RESOURCE_COLLISION` entry whose pool has a row
(`TestCALV0097_CoreDecodesResourceDeferredPlan`).

Numbering: the requirement was first drafted as the next free CAL number, which V1-0780 also took;
it is now `CAL-V0-097`, defined inside `## Requirements` so an OCM over the spec enumerates it
(`TestAgentLeasesSpecEnumeratesCALV0097`).

Rollback: revert the planner, claim-next filter, `resourceDeferred` member, dispatcher routing and
`match.pool`, and the Core decoder rows together. No store, journal or wire state depends on them.
