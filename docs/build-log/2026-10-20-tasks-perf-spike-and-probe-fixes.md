# Tasks lease performance spike and CAL-V0-224 probe fixes

Date: 2026-10-20.
- Lane: tasks-perf, branch `claude/feat-tasks-perf`, base `2b0cfce94`.
- Tickets: V1-0887 and V1-1512, with V1-0850, V1-0910, V1-0916, V1-0646, V1-0926, V1-1193 and
  V1-1656 related.
- Decision: 0549 (proposed design; forks F1 to F3 open).
- Specs: `CAL-V0-116` and `CAL-V0-224` amended in place, and the V1-1512 section updated, all in
  `docs/specs/corvint-tasks-agent-leases-v0.md`.

## Codex review findings on 332fdd5d8 and 577233874

1. **P2: refresh prefix walk ignored staging.**
   - Finding: `walkPrefix` ignored a staged or temporary name, so a refresh under appends could
     advance `FullSeq` over a stray name. Liveness writes could then continue indefinitely, and
     the 4,096 bound kept resetting.
   - Fix: `walkPrefix` now retains nothing (`SNAPSHOT_MOVED`) when the observation holds a stray, a
     staged name or a stage digest.
   - Test: `TestCALV0265_RefreshUnderAppendsNeverAdvancesOverUnsettledHistory`, with two subtests:
     stray staging, and a missing history name beyond the 256 tail.
2. **P2: equal directory change time did not exclude a rename within one tick.**
   - Finding: a rename from H to H+2 in the same second, at the same size, kept the directory's
     change time equal.
   - Fix: the probe now applies only when `receipts/` was quiescent at the first stat (modification
     and change time at least one second old). Otherwise both captures list exactly.
   - Test: `TestCALV0224_LivenessProbeNeedsQuiescentDirectory`.
3. **P3: the store test's assertion was vacuous.**
   - Finding: the assertion used WIDEN, which never takes the writer route.
   - Fix: the test now runs a CLAIM over a broken history. It asserts a refusal that is neither
     completed nor fast, and an unchanged published mutation.
4. **P3: the spec text did not amend the requirements it relaxed.**
   - `CAL-V0-224` is amended explicitly: the first-capture listing is conditional, and the
     quiescence condition and the refresh fail-closed rule are added.
   - `CAL-V0-116` is amended explicitly: an exception to the exact-names clause, a parity
     exception, and directory stamps as evidence for the probe only.
   - V1-1512 item 2 now cites `PSR-V0-053`.

Mutation evidence, by focused runs: each mutation fails its test.
- Dropping the `walkPrefix` staging check fails the stray-staging subtest.
- Dropping the quiescence check fails `TestCALV0224_LivenessProbeNeedsQuiescentDirectory`.
- Passing `liveness=true` for every lease verb fails the store CLAIM-refusal test.

Citation fix: decision 0547 is the go1.27.3 revision. The V1-0887 delegated disposition is
decision 0548, and the design escalation is decision 0549.

## Profile

Decision 0549 §Context holds the tables. Method:
- the store hold-profile harness and pprof;
- medians of 5, with 1-minute host load recorded (22 to 34 on 12 cores, so no run qualifies as
  quiet);
- scratch timers, removed before commit.

Findings:
- The heartbeat and renew cost is about 105 µs per ticket and flat in receipts. Its parts:
  - intent reads and `TreeDigest` (twice): about 55%;
  - capacity measurement: 21%;
  - commit: about 25 to 38 ms, of which 13 to 20 ms is `F_FULLFSYNC`.
- A widen or reap at 40,000 receipts spends 21 to 27 s in the pre-gate complete audit, most of it
  one `openat` per receipt.
- Under 15 writers, the about 1 s gate hold of a resumed round comes from `RecheckResume`'s `held`
  stat walk over every base receipt and request file. That attribution is inferred from the code;
  it was not timed separately.

## NOT_RUN

- A quiet host and Linux.
- The decision 0549 benchmarks B1 to B3, which wait for the slices.
