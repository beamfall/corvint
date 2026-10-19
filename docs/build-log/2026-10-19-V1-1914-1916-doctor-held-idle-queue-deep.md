# 2026-10-19 V1-1914, V1-1916: doctor findings for held run-only work and deep role queues

Tickets: V1-1914 (RUNNABLE-HELD-IDLE, GitHub #1181) and V1-1916 (ROLE-QUEUE-DEEP and optional
calm-pressure cap scaling, GitHub #1183).

## Decision

`ATR-V0-150` to `ATR-V0-152` are added to `docs/specs/corvint-tasks-attempt-runner-v0.md` and
`TQD-V0-043` to `TQD-V0-044` to `docs/specs/corvint-tasks-doctor-v0.md`. All are new requirements,
accepted under the owner's standing auto-accept of lane requirements (2026-10-11). The findings sit
beside the existing dispatcher-backed doctor findings (`doctorDispatcher1`); there is no new
subsystem.

- **A separate observation file instead of ledger v7.** The dispatcher rewrites
  `<programDir>/admission.json` (`taskman-dispatch-admission/0`) after each roster. The ledger
  stays at its current version, and no dispatcher input reads the file. It is added to the
  CAL-V0-131 live formats with a newer-version refusal row. A missing file means "not observed",
  so an older dispatcher yields no finding rather than a guessed one.
- **Streaks are kept in memory.** A restart restarts them, and so does a tick held before the
  roster. Persisting them would need a ledger format change for advisory data only.
- **Kind names use underscores.** The issues say RUNNABLE-HELD-IDLE and ROLE-QUEUE-DEEP, but finding
  kinds must match `^[A-Z][A-Z0-9_]{0,63}$`, so the kinds are `RUNNABLE_HELD_IDLE` and
  `ROLE_QUEUE_DEEP`.
- **Queue depth** is running sessions plus waiting candidates. A waiting candidate is a matched
  ticket that is not running and not backing off. `capMax` sees the depth before the tick's launches;
  the observation records the queue after them.
- **Only `runOnly` roles' tickets are recorded as held, and only beside a free pool member.** This
  gives the issue's negative cases by construction: author-session work is never a held runner, and
  a hold with no free member is not idle capacity.
- **Only `CALM` pressure is calm.** That means level 0 with pending level 0. `UNKNOWN`, `RAISED` and
  `NOT_CONFIGURED` neither scale `capMax` nor let ROLE_QUEUE_DEEP fire, so missing pressure abstains.
- **Both policies are opt-in and off by default.** `bypassFocus` needs `runOnly`, which needs
  `match.pool`. `capMax` needs a match role with `cap < capMax <= 64`. The bypass lifts only the
  plan-window match; every cap, budget, pool-room and pressure fence still applies.
- The global-cap fence no longer `break`s the roster loop. It continues only to record later
  candidates' fence; no launch decision changes.

## Owner decisions surfaced

1. TQD-V0-020 declares `thresholds` closed. TQD-V0-044 states that `runnableHeldIdleTicks` and
   `roleQueueDeepTicks` extend that list by exactly these members, and TQD-V0-020's text is not
   edited. If the owner prefers, TQD-V0-020 itself can be amended to list them.
2. The doctor spec's non-goal "No new journal, lease or dispatch format" is reworded to "of the
   doctor's own". The new file is dispatcher-owned and specified by ATR-V0-150. This is a reading
   of the non-goal, not a change to any accepted requirement. The owner may prefer that the
   observation live in a future ledger version instead.

## Evidence

- Base `714393264`. Before the change, the new tests do not compile, because the API does not
  exist: 0 of 11 new tests pass at base. After the change, all 11 pass: 9 in
  `internal/tasks/dispatch` and 2 in `internal/tasks/cli`.
- `internal/tasks/dispatch` full package with `-race`: ok in 221.8 s. Load averages were 30.34 at
  the start and 26.99 at the end.
- Mutations, one per behaviour, each run against its named test. All 11 were KILLED, at load
  averages between about 17 and 32:
  - M1: the bypass ignores the free-member check (`TestATRV0151_RunOnlyBypassFocus`).
  - M2: holds are noted for author-session roles (`TestATRV0150_RosterNamesFences`).
  - M3: a hold is recorded with no free member (`TestATRV0150_ObservationStreaks`).
  - M4: `capMax` scales off `CALM` (`TestATRV0152_CapMaxScalesAtCalmPressure`).
  - M5: `capMax` is not bounded by depth (`TestATRV0152_CapMaxScalesAtCalmPressure`).
  - M6: the deep streak counts off `CALM` (`TestATRV0150_ObservationStreaks`).
  - M7: the role-cap fence is not named (`TestATRV0150_RosterNamesFences`).
  - M8: the doctor's deep-queue finding fires off `CALM` (`TestTQDV0044_RoleQueueDeep`).
  - M9: the doctor's held threshold is ignored (`TestTQDV0043_RunnableHeldIdle`).
  - M10: the doctor's stale check is ignored (`TestTQDV0043_RunnableHeldIdle`).
  - M11: the doctor's deep threshold is ignored (`TestTQDV0044_RoleQueueDeep`).
  M2 first SURVIVED. Author-session tickets at their role cap were then added to
  `TestATRV0150_RosterNamesFences`, after which M2 was KILLED.
- NOT_RUN: live dispatcher qualification on a running program, Linux, and `make gate`, per the
  owner's standing preference for focused tests.
