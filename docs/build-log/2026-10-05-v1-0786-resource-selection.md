## 2026-10-05 V1-0786: resource-aware selection in the default plan

Human-owned intent: [issue 584](https://github.com/beamfall/corvint/issues/584) (ticket V1-0786).
`SELECTED` tickets waiting for a pool member filled the `maxActiveAttempts` window while lane-free
work deferred `LIMIT_EXCEEDED`. Owner answer 2026-10-05 (D1) chose option (a): cap pool-needing
selections at the pool's free eligible members, defer the excess `RESOURCE_COLLISION` outside the
window, add no policy setting, and report a per-pool resource-deferred count.

Requirement: `CAL-V0-078` (S22) in `docs/specs/corvint-tasks-agent-leases-v0.md`, with TCP-00
amendment A19. The ID is at least three above CAL-V0-073, the highest on
`origin/claude/batch-2026-10-05`; the coordinator may renumber it (and S22/A19) on integration.

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
closed history decoder (`internal/taskman/decode.go` `decodePlan`) refuses such plans rather than
dropping fields, so it fails closed. This needs owner confirmation.

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

Dispatcher exposure: `dispatch.Ticket` carries no `requiresPool`, so a role with `planSelected`
now matches `SELECTED` pool tickets that were previously `BLOCKED`. A worker command that claims
without `--pool` is refused and counts toward cooldown and parking. No dispatcher code changed;
this is an open owner question.

Rollback: revert the planner, claim-next filter and `resourceDeferred` member together. No store,
journal or wire state depends on them.
