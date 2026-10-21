# V1-0925, V1-0924 and V1-0934: dispatcher idle and retirement cost

Date: 2026-10-20. Base: `a148e3a92b78105cc57542829d95f963244adc45`. Branch
`claude/feat-dispatch-perf`. Spec: `docs/specs/corvint-tasks-agent-leases-v0.md` CAL-V0-350..352
and `docs/specs/corvint-tasks-attempt-runner-v0.md` ATR-V0-154, accepted under the owner's standing
auto-accept of lane requirements (2026-10-11). CAL-V0-320..349 are held by peer branches, so the new
IDs start at 350.

## Decisions

- **V1-0925 config read reuse (CAL-V0-350).** The digest is cached by the identity CAL-V0-127
  already compares (`sameConfigStat`: same file, size, mode, mtime). Any change, a stat error, a
  clock that went backwards, or a read older than the 15-minute idle safety net reads the bytes
  again (fail closed). That safety net covers a same-size rewrite within the timestamp granularity.
  A refused digest that is reused is not reported again.
- **V1-0924 deferred pressure sample (CAL-V0-351).** The idle gate no longer excludes a pressure
  configuration. Deferral applies only with no worker running and a recorded calm state. Calm is
  the most permissive label, so a roster that never reaches the pressure budget at calm launches
  nothing under any sample. In that case the tick samples nothing, and the ledger bytes stay
  unchanged so the gate can arm. A consulted deferred budget refuses, samples, and re-plans.
  Rejected alternatives:
  - deferring at a raised level: a stale level would then never be resampled;
  - deferring with workers running: CAL-V0-068 dwell tracking would stall while the host is busy.

  Two fail-closed details:
  - the first sample after a gap has no CPU baseline;
  - its budget uses the pending level when that is higher, because the skipped ticks never
    advanced the dwell. A mutation that drops the pending-level rule launches under a critical
    first sample, and the test catches it.
- **V1-0934 dispatcher (CAL-V0-352).** The dispatcher keeps an open `workers/` stream between
  passes. It holds each window until its marks settle (at most 4 passes) and carries the retained
  32 into the next window. Window plus carried names stay within 4,096. The first attempt carried
  only the retained names, not the window's earlier carry. A carried directory displaced in a
  later window then lost its mark on the confirming pass, and the test caught it: the held window
  now includes the carried names it started with.
- **V1-0934 runs (ATR-V0-154).** The supervisor is one-shot, so its cursor is persisted:
  `taskman-runs/.retire-cursor` holds the last name examined and up to 16 carried attempts, and is
  replaced by rename. It is a name cursor over a sorted, names-only listing of at most 65,536
  entries. That listing exceeds the strict per-pass entry count, but stats nothing. The examined
  set and the store lookups stay at the ATR-V0-015 bound: 1,024 directories and one lookup.
  Accepted limit: names beyond the first 65,536 listed are not reached.

## Measurements (deterministic counts from the focused tests; host macOS, darwin/arm64)

| Quantity | Before (base code, inferred from source) | After (measured by test) |
|---|---|---|
| Config byte reads per full tick, identity unchanged | 1 | 0 (5 reads across the test's 15+ ticks, each forced) |
| Idle ticks with pressure configured: store observations / pressure samples per tick | 1 / 1 (the gate is disabled by `Pressure != nil` in `idleEligible`) | 0 / 0 over 20 ticks |
| Worker dirs: 60 dirs, 7-name windows, retention 3 | the first window only (starves when it holds nothing removable) | everything retired in 18 passes (bound 19) |
| Run attempts: 40 dirs, 6-name windows, retention 2 | the first window only | everything retired in 10 passes (bound 11) |

The before column is inferred from the base source, not measured: the new tests use fields that
do not exist at the base. The wall-clock effect on a live dispatcher was not measured (NOT_RUN).

## Friction

- `heavy.sh` waits of up to 202 s pushed foreground test commands past the 180 s default tool
  timeout, which moved them to the background. Pass an explicit timeout of 600000.
- CAL ID collisions: peer branches hold CAL-V0-320..349 without merged specs. A requirement-ID
  reservation file would avoid scanning every remote branch.

## V1-0948 test process hygiene (CAL-V0-353)

Decision: tests record each launched worker leader through a nil-by-default `launchObserved`
hook in `launch`, and `testConfig` registers a cleanup that SIGKILLs each recorded process group
and waits until it is empty. A `TestMain` sweep fails the package if any group is alive after all
tests. Killing by group follows from `launch` making every worker a session and group leader. A
recorded pid is signalled only while it is free or still carries its start identity, so pid reuse
cannot hit an unrelated group.

Evidence: with the cleanup disabled, the `TestCALV0127_Reload*|TestCALV0143_*|TestCALV0272*` run
fails with 4 leaked groups. The full package passes in 181.7 s with no test-launched `host.sh`
left. Friction: two foreign orphans were present before the run (a `TestTOLV0021_*` `host.sh` 4 days
old and a `TestCALV0143_*` one from a peer lane's run on old code). They were left for the
orchestrator.
