# V1-1130: `--lock-wait` for pool acquire and other agent writes

Human-owned intent: V1-1130 / [issue 772](https://github.com/beamfall/corvint/issues/772).
Under about seven concurrent agent sessions, 4 of 26 `pool acquire` calls returned
`LOCK_TIMEOUT` after the fixed 30-second preparation admission while a member was `FREE`;
unlike `release`, `pool acquire` had no caller-chosen wait.

Contract: CAL-V0-216 in `docs/specs/corvint-tasks-agent-leases-v0.md`, **proposed, pending owner
acceptance (V1-1130)**. Delivery is experimental. Base `97c3a4bd03bb97b29e7378dfa116ffde2280cadb`.
No decision file or native ticket-store write. Built by the lane driver directly (owner rule
2026-10-11: Codex for review only).

## Decisions

- Reuse CAL-V0-111 unchanged: the same parser, bound (1..300 s), context plumbing
  (`store.WithLeaseLockWait`) and help note. The change is the CLI verb set: `claim`, `renew`,
  `submit`, `pool acquire`, `pool confirm-safe` and `pool release` join `release` and
  `attempt heartbeat`. All six already run through `store.Lease`, whose preparation admission
  and writer-lock acquisitions read the caller wait, so no store change is needed.
- Excluded: `reap`, `widen`, `complete`, `gate run`, `health`, `pool cleanup`, `pool recover`
  and batch verbs. The refusal names every accepting command.
- Writer-checkpoint refresh and the recovery probe keep default lock options, as for `release`.

## Evidence

- `TestCALV0216_LockWaitAdmissionVerbs` (`internal/tasks/cli`, `-race`): PASS. It fails
  against the previous verb set (MALFORMED misplaced-flag refusal).
- `TestCALV0111_LockWaitFlag`, `TestCALV0112_*`, CAL-V0-111 contention tests and help tests: PASS.
- Documentation gates: recorded with the lane's final checks.

Limits: live multi-agent contention qualification NOT_RUN; `make gate` and `./...` NOT_RUN.
Rollback: revert the verb set, help lines, tests and the CAL-V0-216 section together.
