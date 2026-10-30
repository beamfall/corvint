# Decision 0536 — Pool sweep observation bounds scale with the sweep budget

Date: 2026-10-16. Status: accepted. Authority: owner delegated to the orchestrator (2026-10-16),
which chose option (a) for V1-1492. Adds `PSR-V0-023` to
`docs/specs/corvint-tasks-pool-safe-reuse-v0.md`; no accepted requirement's outcome changes.

## Context

After PR #965 made the CLI tests' own waits event-driven, `TestPSRForegroundSignals` and
`TestPSRDispatchNativeAdapter` still failed under host load 56-88 on fixed product bounds in
`internal/tasks/store`: the sweep Git observation's 2 s bound (member QUARANTINED `TIMEOUT; safe
reuse failed`), the runner-identity probe's 1 s bound (`QUIESCENCE_UNPROVED`), and the sweep
command's 1 s pipe delay in the cleanup proof (`UNKNOWN; safe reuse failed`). The V1-1324 lift is
package-private and the CLI is re-exec'd, so CLI tests cannot use it (evidence:
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-store-test-reliability-1464-1000-1465.md`).

Options were (a) scale the bounds from the invocation's `--timeout-seconds` budget, (b) an exported
or environment test seam, and (c) record the limit under V1-0849 with measured failure rates.

## Decision

Option (a), as `PSR-V0-023`. Inside a sweep, each fixed observation bound is multiplied by
budget/10 s, with the fixed value as the floor and 5 times it as the cap:

| Bound | Fixed (floor) | Budget 15 s | Budget 30 s | Cap (budget >= 50 s) |
| --- | --- | --- | --- | --- |
| Sweep Git probe | 2 s | 3 s | 6 s | 10 s |
| Runner-identity and group-listing probe | 1 s | 1.5 s | 3 s | 5 s |
| Probe pipe delay | 250 ms | 375 ms | 750 ms | 1.25 s |
| Sweep command pipe delay | 1 s | 1.5 s | 3 s | 5 s |

- Every probe still shares the attempt and invocation deadlines, so a scaled bound never extends
  the sweep's budget.
- The group/pipe cleanup allowance stays 5 s and `cleanupAllowanceMillis` stays 5000 (the snapshot
  codec requires it). Only the group probes inside that allowance keep the scaled bounds.
- An expired scaled bound records exactly what the fixed bound records, and the member stays
  QUARANTINED.
- Pool preparation and cleanup outside a sweep have no budget and keep the fixed bounds.
- The budget travels on a package-private context key set only by `store.PoolSweep`. No flag,
  environment variable or exported seam is added. The V1-1324 package-test lift still applies, but
  only above the scaled bound, and never reaches the cleanup proof.

## Consequences and rollback

- A slow host now spends up to the scaled bound on an observation that used to fail at the fixed
  bound. That spend stays inside the sweep's own budget, and the cap holds it to 5 times the fixed value.
- Rollback: revert the commit. Recorded bytes are unchanged, so no data migration is needed.
