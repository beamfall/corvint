# 2026-10-06: V1-0668 Linux zombie-only group retirement

## Intent

Ticket V1-0668 (1.0.0-rc.3). On Linux, `kill(-pgid, 0)` succeeds while a process group holds only
zombies, including the owner's own unreaped leader, so a signal-0 quiet wait before the reap can
never end. `internal/groupreap` therefore used `ReapAfterSuccessfulSignal` on Linux, and Linux had
no pre-reap proof that the group was quiet. The stable/candidate lifecycle verifier cases ran on
Darwin only. The goal is a Linux pre-reap proof that keeps leader and identity ownership, exactly
one pre-reap group signal and no numeric signal after the reap, and that reports HOLD when the proof
cannot be obtained.

## Change

- `PGO-V0-006` (new): `procGroupQuiet` in `internal/groupreap/owner_proc.go` makes the
  `RequirePreReapQuiet` observation from `/proc` and sends no signal.
  - It runs only after the exit observation (`waitid(WEXITED|WNOWAIT)`) and the single group
    SIGKILL, while the leader is still unreaped and so pins its PID and PGID.
  - It first checks leader identity: `/proc/<leader>/stat` must show state Z or X, `ppid` equal to
    this process and `pgrp` equal to the leader.
  - It then scans the numeric `/proc` entries. A process whose `pgrp` is the leader is `ProbeLive`
    if its state is not Z or X, or if it is a zombie whose `status` `Threads:` count is above one.
  - It reports `ProbeQuiet` only when the scan saw the leader and the identity re-check passes
    after the scan.
  - It is bounded at 2^22 entries per directory, 4096 bytes per stat and 16384 bytes per status. Entries that vanish during
    the scan are skipped.
  - Every failure wraps `errProcProof` and the owner HOLDs. Failures include a missing or unreadable
    `/proc`, an identity mismatch, malformed or oversized stat, and an exceeded bound. The proof
    never reports `ProbeAbsent`.
- `Primitives.QuietProof` (new field) is the pre-reap observation. If it is nil it falls back to an
  injected `ProbeGroup`, otherwise to the platform default: signal 0 on Darwin and `procGroupQuiet`
  on Linux.
- `PGO-V0-002` is amended: the Linux default is now `RequirePreReapQuiet`.
  `ReapAfterSuccessfulSignal` remains available only when a caller selects it explicitly. Release
  after the reap still requires a signal-0 ESRCH.
- `internal/cem/verify/stable_repository_unix_test.go`: the stable lifecycle conformance cases now
  run on Linux as well as Darwin. Other platforms skip them as NOT_RUN.

## Decisions

- **Why one scan is sound.** After the group SIGKILL, every member is dying, and Linux refuses
  `fork` from a task with a pending fatal signal. So no member can create a new live member. A
  process that exists for the whole `readdir` is listed, so a single scan sees every member that
  outlives it.
- **Joins from outside the group are a stated limit.** Because the leader is unreaped, no new
  process can take the leader's PID as its PGID. A same-session process can still join with
  `setpgid`. If it joins after the scan read its entry, or after the scan, the proof does not see it
  and the reap can follow; Darwin signal 0 has the same window. Such a process was never signalled.
  The post-reap signal-0 check HOLDs while it remains a member. This is recorded in Non-goals.
  (The first draft wrongly claimed the scan always sees such a joiner; see the review.)
- **Thread count, not a task walk.** A thread-group leader can show Z while its other threads still
  run. `/proc/<pid>/task` is enumerated by ordinal after a cached TID, so threads exiting between
  reads can skip a surviving thread. The proof therefore reads the `Threads:` count from `status`
  for each zombie member, and any count above one is live. The top-level `/proc` listing is
  enumerated by PID number, so no process that exists for the whole scan is skipped.
- **Known limits, in Non-goals.** Zombies left by a non-reaping PID 1 or subreaper, and members
  hidden by `hidepid`, end in HOLD. They never reach RELEASED, because release still needs a
  signal-0 ESRCH after the reap.
- **Signal 0 kept on Darwin.** It is the observed Darwin primitive, and Darwin has no `/proc`.

## Evidence

- Linux arm64, run natively in a `golang:1.27.1` container (kernel 6.8.0-117, go1.27.1, git
  2.47.3):
  - `go vet` passes.
  - `internal/groupreap` passes with `-race -v`, with `-count=5`, and as uid 1000 (`setpriv`).
  - New tests pass:
    - `TestLinuxOwnerRetiresZombieOnlyGroup`;
    - `TestLinuxSignalZeroCannotProveZombieOnlyQuiet`, which reproduces the defect: signal 0 HOLDs
      without a reap;
    - `TestLinuxOwnerHoldsWhileLiveMemberRemains`;
    - `TestLinuxOwnerHoldsWhenQuietProofUnavailable`;
    - `TestLinuxDefaultRetirementUsesProcQuietProof`;
    - the classification matrix tests.
  - `linuxSignalRecorder` asserts exactly one KillGroup, issued before the reap, and no kill after
    the reap.
  - `internal/cem/verify -run 'TestStable|TestCandidate'` passes as root and as uid 1000. S0E has
    54 PASS and 2 NOT_RUN (the go1.24.13 toolchain cases).
  - As uid 1000, `internal/cem/gitrun`, `internal/cem/verify`, `internal/tasks/dispatch` and
    `internal/localcompletion` all pass. As root, dispatch and localcompletion have fixture failures
    such as "fixture did not refuse writes": root bypasses the read-only fixtures. These failures
    are unrelated.
- Linux amd64 (`--platform linux/amd64`, emulated): NOT_RUN. The container starts (`uname -m` x86_64, go1.27.1 linux/amd64), but under qemu user emulation the Go toolchain itself fails: `go vet`/`go test` die with SIGSEGV in `cmd/go/internal/modindex`, and a retry with `GODEBUG=goindex=0` left `go test ./internal/groupreap/` hung for more than 50 minutes. The container was removed.
- Darwin arm64:
  - `internal/groupreap` passes with `-race`.
  - `corvint affected --base 07452b07fd8cfd4fc622ad90c673a323b3d6d495` selected 163 Go units (scope
    UNKNOWN). Run with `-count=1 -timeout 30m`: 159 ok and 4 failed. All four are environmental and
    do not involve groupreap:
    - `conformance/release-artifact-v0`: "worktree is not clean" before the commit.
    - `internal/authoritystore`: `protected-authority-unavailable` under the private TMPDIR. It
      fails the same way on the base commit and passes with the default TMPDIR.
    - `internal/liveverify/affected`: 100 ms latency budgets exceeded under load.
    - `internal/tracerecordrepo`: git timeout and TempDir cleanup under load.
    - Rerun after the commit with the default TMPDIR, all four pass.
  - `go vet` passes for GOOS darwin, linux and windows on `internal/groupreap`,
    `internal/cem/verify` and `internal/cem/gitrun`.
- After the review fixes, the Linux arm64 checks were rerun and all passed: vet; groupreap with
  `-race -v`, `-count=5` and as uid 1000; the verify Stable/Candidate cases with S0E 54 PASS and
  2 NOT_RUN as root and as uid 1000. On Darwin, groupreap `-race` and vet for darwin, linux and
  windows also passed.

## Review

Independent review used `codex exec -m gpt-6-astra -s read-only` on
`07452b07..3a352248`. It reported two P2 findings and found nothing in signal ordering, leader PID
pinning, the bounds or the build tags. Neither finding allows a false RELEASED.

1. P2: a same-session process can join the group with `setpgid` after the scan has read its entry,
   and the proof still reports quiet.
   - Disposition: accepted as a limit. No scan can close this window, and Darwin signal 0 has the
     same one.
   - The build log's incorrect claim was corrected, and the limit was added to the spec's
     Non-goals. The post-reap absence check still HOLDs while such a process remains a member.
2. P2: `task/` is enumerated by ordinal, so threads exiting between reads can hide a surviving
   thread.
   - Disposition: fixed. The proof now reads the `status` `Threads:` count, bounded at 16384 bytes.
   - New classification cases cover a zombie leader with a live thread, a missing `status`, a
     `status` with no thread count, and an oversized `status`.


## NOT_RUN

- Linux amd64: qemu emulation could not run the Go toolchain; a native amd64 host is needed.
- Linux hosts with a non-reaping init or `hidepid`. They are expected to HOLD, but this was not
  observed.
- `make gate`, not requested. Dogfood CEM binding, which belongs to the orchestrator.

## Rollback

Revert this change. Linux then returns to `ReapAfterSuccessfulSignal` with no pre-reap proof, and
the verifier's stable lifecycle cases become Darwin-only again. The change adds no persisted state
or wire format.
