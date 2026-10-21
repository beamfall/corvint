# JSG S1: liveness writes elide the intent tree

Date: 2026-10-20. Tickets: V1-0887, V1-1512. Spec: `docs/specs/corvint-tasks-journal-segments-v0.md`
(`JSG-V0-001`, `JSG-V0-002`, and part of `JSG-V0-014`). Authority: decision 0549 (accepted by the
orchestrator under decision 0548's delegation), fork F1 (null intent digest for liveness).

## Change

- `journal`: a liveness audit (`Reader.liveness`) does not scan `intent/`. It reads
  `intent/queue.json` and `intent/policy.json` by name, leaves the intent tree digest empty, and
  reports `ProjectionAgreement = INTENT_RECORDS_NOT_COMPARED`, which `Checkpoint()` accepts.
- Writer checkpoint profile `taskman-writer-checkpoint/2`: per-entry canonical sizes and an intent
  aggregate (ticket count, release count, payload bytes). The decoder refuses an aggregate that
  differs from the live intent entries and their sizes, and refuses `/1` as unsupported.
- `transaction`: `Elided.Intent` summarises the intent records. Capacity charges them from the
  carried canonical sizes, and any model lookup, `FilesUnder` or `countPrefix` that touches
  `intent/tickets/` or `intent/releases/` marks the inventory incomplete, so the route declines.
- `store`: `bindLiveness` replaces `bindObservation` for liveness. It rechecks `head.json`,
  `queue.json` and `policy.json`, `reservations.json`, `pools.json`, `programs.json` and each attempt
  the plan rewrites against the bytes the model read, then the writer guards and the branch.
  A liveness write retains the read checkpoint only when it also advances the writer checkpoint.

## Forks decided within the design

1. Decoder aggregate check: the carried intent aggregate must equal the counts and payload derived
   from the entry table. Otherwise a forged aggregate could under-charge capacity, so the check is
   fail-closed.
2. Binding set: `pools.json` and `programs.json` are bound on every liveness write, not only when
   the plan rewrites them, because the model reads both whole. A refused liveness result binds the
   same set, minus plan artifacts.
3. Read checkpoint: no rewrite below the advance tail. This is the one intended difference from the
   complete route, and `TestCALV0116_WriterRouteParity` now asserts it through `writerLivenessParity`
   rather than requiring byte parity of that file.
4. Authority session pins: the session still opens each intent directory it may publish into. That
   is an O(1) open that lists and reads nothing. Making the pins lazy would change the authority
   session's mount pinning for no measurable gain, so `JSG-V0-001` names the exception.
5. `JSG-V0-002` holds structurally: lease mutation results carry `snapshot: null`. The CLI test
   pins it.

## Evidence

- Tests: listed in the spec's Traceability rows for `JSG-V0-001`, `JSG-V0-002` and `JSG-V0-014`.
  The no-read test clears the mode of every ticket and release file and plants a symlink in
  `intent/tickets/` that any listing scan refuses. It passes at 70 and 210 tickets, with 0 and 30
  prior heartbeats. A claim over the same seal is the control.
- Mutation checks, run and reverted:
  - Threading `false` instead of `LivenessLease` fails the no-read test, on the planted symlink.
  - A `bindLiveness` that returns nil fails 7 of the 8 moved-input cases. The branch case is caught
    by the separate branch recheck.
  - Charging scanned entries, or dropping the intent-bytes charge, fails the measure parity test.
- Spot measurement, not qualifying: `TestCALV0118_WriterHoldProfile` at 880 tickets and 13,500
  receipts, 5 reps. The 1-minute load was 20 on 12 cores.
  - Heartbeat median: 75.7 ms (baseline 145 ms).
    - observe 34.4 ms, the checkpoint decode, which S2 addresses.
    - commit 27.4 ms, the fsyncs, which fork F2 keeps.
    - model 2.2 ms.
    - bind 1.6 ms.
  - Renew median: 82.7 ms.

## Rollback

Revert the S1 commit. The `/2` writer checkpoint is derived. A `/1` binary refuses it as
unsupported and retains its own from the complete route.
