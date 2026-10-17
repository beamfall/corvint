# V1-1548: dispatcher lease keeper (`keepLease`, issue 1006)

Ticket V1-1548, GitHub issue 1006. Requirements CAL-V0-311..316 in
[corvint-tasks-agent-leases-v0](../specs/corvint-tasks-agent-leases-v0.md), accepted under the
owner's standing auto-accept of lane requirements (2026-10-11). This is not a new owner decision;
it applies that standing acceptance to this lane's requirements.

## Decisions

- **Idle signal.** The dispatcher already records `lastActive` for every worker each tick
  (CAL-V0-056 as amended by CAL-V0-218): growth of the worker's `stdout.log`/`stderr.log`, an
  advanced mtime of a host `activityPaths` entry, or a busy child process. The keeper reuses it and adds no new signal; a
  worker is idle when `now - lastActive > maxIdleSeconds`. An idle worker's attempt is not written,
  so its lease runs out and the existing CAL-V0-191 reap ends it.
- **Heartbeat as well as renew.** A renew alone left the holder `STALE_HOLDER` before V1-1313; the
  store's `renew` now records `lastHeartbeatAt` (CAL-V0-231). When the lease still covers two
  intervals the keeper records only `attempt heartbeat`, which never extends the lease
  (acceptance 4); otherwise it renews. Either write keeps the holder fresh.
- **Placement.** The pass runs in `tick` right after heal, on the observation heal acted on, so a
  worker heal just stopped or reaped is not kept on the same tick.
- **Stops.** `FENCED`, `NOT_HELD` and `GENERATION_CHANGED` stop the binding once with one
  `lease-keeper` event and no alert (acceptance 3). Other failures alert once per episode and retry.
- **Idempotence.** Request IDs derive from attempt, generation, write kind and the due time, so a
  write repeated after a lost ledger save replays instead of writing twice.
- **Ledger version.** The worker `keeper` record moves the ledger to `taskman-dispatch-state/6`;
  version 5 is adopted as recorded. Earlier builds refuse /6, so rollback drains first.

## Evidence

Focused tests: `internal/tasks/dispatch/keeper_test.go` (CAL-V0-311..315 acceptance 1-4 and 6, fake
clock and injected workers, no real sleeps), `internal/tasks/cli/dispatch_keeper_test.go` (the real
store: heartbeat leaves the lease, renew moves it, a stale or released attempt is fenced) and
`internal/tasks/cli/dispatch_keeper_internal_test.go` (acceptance 5, status `keeper`).

Not measured: a live multi-hour dispatch with a real host. The fake-clock tests cover four lease
periods; live qualification stays `NOT_RUN`.
