## 2026-10-04 V1-0780: command results state their retryability

Human-owned intent: ticket V1-0780, the owner's follow-up to issue 494. Under about 8,900 receipts
and 13 concurrent sessions, lease renew and heartbeat returned `ERROR`/`LOCK_TIMEOUT` while the
lease was FRESH and the client runner killed healthy runs. The client now pattern-matches a private
list (`LOCK_TIMEOUT`, `SNAPSHOT_MOVED`, `STALE`, `REDO_PENDING`, `STORAGE_FAILED`, `HEAD_MOVED`,
`JOURNAL_SATURATED`). The owner asked Tasks to document the retryable codes and mark each result.

Requirement: `CAL-V0-078` and amendment A19 in `docs/specs/corvint-tasks-agent-leases-v0.md`
("V1-0780 retryable result amendment"). `CAL-V0-074..077` are taken by the concurrent V1-0755,
V1-0756 and V1-0781 branches, so this change uses the first ID none of them claims.

### Decision

- One table, `wire.RetryOf` (`internal/tasks/wire/retry.go`), classifies all 71 §11 codes from
  their in-tree producers, not their names. Retryable: `LOCK_TIMEOUT`, `SNAPSHOT_MOVED`,
  `REDO_PENDING`; every producer reports them before anything is decided or written. Everything
  else is false with a recorded reason: fencing codes always; mixed, capacity or uncertain codes
  (`LIMIT_EXCEEDED`, `JOURNAL_SATURATED`, `UNSUPPORTED_FILESYSTEM`) by the fail-closed rule; 19
  reserved codes because they have no producer.
- The `taskman-command-result/0` envelope gains an absent-only optional boolean `retryable` on a
  non-`OK` result with codes, true only when every code is retryable. `OK` and uncoded envelopes
  keep their bytes, decoders accept legacy coded bytes, so no profile bump. Precedent: A18's
  absent-only `handoffEvidence`.
- `Result.NotRetryable` lets a command report false despite retryable codes. `attempt run` sets it
  once its child has run, because an unrecorded outcome (`ERROR`/`LOCK_TIMEOUT`, exit 127) would
  otherwise invite a retry that runs the child again.
- Independent review found the same hazard in `gate run`: the gate program runs before the
  recording `Lease` call, and lock contention there returned a retryable `LOCK_TIMEOUT`, while a
  same-request retry reruns the gate before replay detection. `store.Report.Unretryable` now records
  that a program started; `GateRun` sets it on every return after the gate starts, `PoolCommand`
  after the member's program starts (a retry there replays the prepare and never records the
  observation), and the lease CLI maps it to `NotRetryable`. A regression holds the store lock
  after the gate program has run and checks `LOCK_TIMEOUT` with `Unretryable`.
- A second review found two committed-but-unfinished paths. `PoolCommand` read the snapshot after
  its preparation or cleanup receipt committed, and a `REDO_PENDING` or `SNAPSHOT_MOVED` there came
  back retryable, while a same-request retry replays that receipt and reports success with the
  allocation left `PREPARING` or `CLEANING`. `Unretryable` is now set as soon as that receipt
  commits. `PoolSweep` lost an executed phase's observation write to contention and also came back
  retryable, while a same-request retry only reconciles committed receipts. `PoolSweepReport`
  now carries `Unretryable` for every error after a fresh sweep commits its owner, and the sweep
  CLI maps it to `NotRetryable`. Regressions inject a failure right after the preparation commit
  (`SetPoolPreparedFaultForTest`) and `LOCK_TIMEOUT` on the first observation write; each checks
  the flag and that a same-request retry neither runs the program again nor finishes the step.
  Both fail with the fix removed.
- A third review found the supervised path dropped that fact: `Workflow.RunRole` passed the
  `GateRun` report through `transitionOK`, which returns only the error, so `run --role reviewer`
  reported a recording `LOCK_TIMEOUT` retryable although a repeat skips the now-`CHECKING` attempt
  and returns `OK` without finishing its gates. A `wire.Error.NotRetryable` mark (`WithoutRetry`)
  now carries the fact through the error, and `errorResult` maps it to `NotRetryable`. A
  single-repository fake-host program regression fails the gate record after the gate runs; it
  fails with the mark removed.
- The same review noted the sweep regression above simulates a lost response: its hook runs after
  the real writer commits. It is renamed for that. A request hook before the writer adds two
  pre-commit cases. Contention before the owner commits leaves no owner and no run, stays
  retryable, and a same-request retry runs the phase and commits once. Contention before an
  executed phase's observation commits leaves the observation absent but the owner committed, so
  it stays not retryable: the same-request retry goes to `reconcileSweep`, stays pending and never
  reruns the phase or commits that observation. The review expected that case to be retryable with
  a retry committing once; the code and the regression show the retry cannot.
- A fourth review found the round 3 mark too narrow. It covered only the failing `GateRun`, so a
  READY step that failed after a gate passed, or a later gate refused after an earlier one ran,
  still came back retryable. Once review leaves the attempt `CHECKING`, a repeated `run --role
  reviewer` skips it, because selection accepts only `BUILT` (`cli/program.go`). Every error in
  that block is therefore not retryable, including a first gate refused before any gate ran. A
  test hook at each gate and before READY drives three regressions, which fail with the marking
  removed. The same review found that the failure-mode paragraph of the spec amendment called
  `SNAPSHOT_MOVED` not retryable; it now separates `LIMIT_EXCEEDED` from the stale-input
  exception of a retryable `SNAPSHOT_MOVED`.
- Review also found `CAL-V0-078` defined outside `## Requirements`, where the OCM reader
  (`internal/lrfrepo` `requirementsFromBlob`) does not enumerate it. The normative text now sits in
  a Requirements subsection; a scratch enumeration of the spec lists it. `CAL-V0-073` (V1-0751)
  has the same placement and is still not enumerated.
- `STALE`, `STORAGE_FAILED` and `HEAD_MOVED` from the client list are not §11 codes and cannot be
  classified; `JOURNAL_SATURATED` is deliberately not retryable.

### Limits and open questions

- A consumer built before this change that decodes the closed envelope exactly refuses coded
  non-`OK` envelopes carrying the member. No in-tree closed reader consumes such envelopes.
- `SNAPSHOT_MOVED` keeps three input-mismatch producers (release candidate head, attestation
  candidate, criterion capture wrapping a refused read) that repeat until the caller's input
  changes; recoding them is an owner decision.
- The transient `LIMIT_EXCEEDED` cases (64 live preparation slots, active-attempt cap) stay not
  retryable until split into their own code.
- A claim that runs pool health probes before a later refusal is not overridden; a retry of that
  claim may probe again.

Evidence: `TestCALV0078_*` in `internal/tasks/wire`, `internal/tasks/cli` and `internal/tasks/store`; focused package runs
recorded in the change evidence.
