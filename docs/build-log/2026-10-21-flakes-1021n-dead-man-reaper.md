# 2026-10-21 flakes-1021n: dispatch dead-man reaper (V1-2189), Pi startup witness (V1-2245), flake triage

Lane `flakes-1021n`, started on `58d4ed2d309d5dc1afc21c127647d2e22b1bed3f` and merged once with
origin/main; BASE `ef2a01db52d5436c262725d136060c9eb95e8580`. Test-only changes. All runs were on
the shared 12-CPU host through `heavy.sh` with other lanes running; 1-minute load averages are
recorded. ATR-V0-213 is accepted under the owner's standing auto-accept (2026-10-11).

## V1-2189: dispatch fixture workers outlive a killed test binary (ATR-V0-213)

Problem. ATR-V0-183's per-test cleanups and `TestMain` leak check run inside the test binary. A
binary killed by SIGKILL runs neither, so the `host.sh` worker groups it launched (CAL-V0-127 and
CAL-V0-143 fixtures) kept running on the host.

Decisions.
- `TestMain` starts a dead-man reaper before `m.Run`: the test binary re-executed with
  `CORVINT_DISPATCH_TEST_DEAD_MAN=1`, `Setsid`, stdin the read end of an `os.Pipe` whose
  close-on-exec write end only the binary holds. The existing `launchObserved` test hook also writes
  `pid identity` lines to the pipe. The kernel closes the write end when the binary dies by any
  means; at EOF the reaper sends SIGKILL to each group whose leader still matches its recorded
  start identity, the same ownership rule as `reapLaunched`. A normal run closes the pipe after its
  own leak check and waits, bounded, for the reaper.
- The reaper repeats SIGKILL while a killed group stays present (bounded, 10 s). Two of roughly
  150 early repeats at load 40-62 left the group present after 30 s with its leader already gone.
  Inferred cause (not directly observed): a child `sh` was forking while the group signal was
  delivered and missed it, the class ATR-V0-153 fixed for the runner. While the group stays present
  its ID cannot name another group, so re-signalling is safe.
- No product code changed; `internal/tasks/dispatch/process_unix.go` already exposes the hook.

Evidence.
- `TestATRV0213_FixtureWorkersDieWithTheirTestBinary` re-executes the binary as a helper that
  launches a worker tree, SIGKILLs it, and requires the group gone within 30 s. With
  `startDeadMan` disabled it fails: "worker group 4409 outlived its SIGKILLed test binary (owned
  leader: true)".
- With the fix: 540/540 passes (`-count=60` x 9) at loads 14-64; full `./internal/tasks/dispatch`
  passes (201.5 s). No reaper process remained after the runs.

## V1-2245: Pi host startup witness bound

`integrations/pi/host.test.mjs` waited at most 1.5 s for the startup native child witness. The
wait now ends when the witness appears or Pi exits; the child timeout of `run` stays the hang
bound. Mutant: a `PI_BIN` wrapper that delays Pi startup by 2 s fails the base test and passes the
fix. Six repeats of the AHI-002 and host-check interrupt tests passed at loads 24-32. The test file
is not shipped in the package, so no version bump.

## Already delivered on main (no change)

- V1-1302: PR #914 (`liftedSourceDeadline`; build log 2026-10-15-flake-batch-1394-1393-1302).
- V1-1545: `49eca8f62`, `a9825e937` (PR #1411). V1-1955: `1504fb35b` (PR #1411).
- V1-2121: `f8c33fba7` (V1-1771); the failing run's head `0fce98369` predates it.
- V1-2122: `7f46e42dc` (V1-1780); the failing run's head `714393264` predates it.
- V1-2250: the V1-2103 fix `00c904e78`; failing nightly 39416257984 ran head `f3ca72248`, which
  predates it; nightly 39594856562 (contains it) passed `internal/tasks/cli` on macos-15.
- V1-2255: the symlink case of `TestSTRV0060PathDigestRefusesSwap` is fixed by `04188fef3` (type
  change check; ext4 inode reuse) and `TestPGOV0019ShadowCancelKillsTheGroupBeforeTheReap` by
  `821c66be2` (pipe closed after the run returns). Both commits postdate the failing heads.
  Repeats on darwin: PGO-V0-019 `-race -count=200` passed (load 71); STR-V0-060 `-count=200`
  passed 200/200 (load 64). The ext4 inode-reuse case is NOT_RUN locally (Linux only).

## NOT_RUN

Linux runs of ATR-V0-213 and STR-V0-060 (darwin host); `affected` tier 2 (batch CI).

## Rollback

Revert the lane commits. The ATR-V0-183 cleanups and leak check are unchanged; the Pi test returns
to its 1.5 s bound.
