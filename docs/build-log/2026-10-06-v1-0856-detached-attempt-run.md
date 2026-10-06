# 2026-10-06: V1-0856 detached attempt runs

## Intent

Ticket V1-0856, GitHub [beamfall/corvint#627](https://github.com/beamfall/corvint/issues/627). When an
agent session ends, its host or the dispatcher stops the session's process tree. That kills an
attempt-bound `corvint-tasks run --attempt` command with it. The command should keep running,
heartbeating and fenced. The next session for the same attempt should attach and read its outcome.
Invariant 7 rules out a permanent daemon.

## Change

- Spec `docs/specs/corvint-tasks-attempt-runner-v0.md` adds ATR-V0-008..014, a "Detached runs" section
  (run record, relation to CAL-V0-089, dispatcher interaction and remaining work), new exit and envelope
  rows, failure modes, rollback and traceability. The intent header now names issue 627.
- `run --attempt ... --detach -- COMMAND` starts one supervisor in a new session and returns once the
  command has started. The supervisor re-executes the same binary through the internal `--supervise RUN`
  form. It runs the unchanged attached runner (heartbeats, timeout, owned group, `RUN_OUTCOME`) and keeps
  its envelope and bounded output in `<git-common-dir>/taskman-runs/<attempt hash>/<run>/`.
- `run --attach --attempt ID [--run RUN] [--wait SECONDS]` is read-only. It replays a finished run's
  kept envelope after checking the SHA-256 recorded for it, and returns the run's status. While the run
  is unfinished it exits 75. It refuses `SUPERVISOR_LOST` when the recorded PID no longer has its
  recorded start identity, and it never signals.

## Decisions

- **Re-execute the binary instead of using a helper daemon.** The supervisor is scoped to one attempt.
  It exits when its run ends, so its life is bounded by the timeout plus the cleanup and write bounds.
  It is never restarted.
- **Fencing stays the existing heartbeat refusal (ATR-V0-006).** A release, reap or generation change
  stops the detached command within one beat interval. No new fence path was added.
- **Readiness is a close-on-exec pipe on fd 3, closed after the `RUNNING` record is written.** The
  launcher never waits for the command. A pre-launch refusal is kept and replayed with the attached
  runner's status.
- **Liveness is the recorded PID plus `supervisor.ProcessIdentity`, never a bare PID.** The run record
  is private runtime state: it is never evidence or authority, and `RUN_OUTCOME` stays the native record.
- **Independent review (Codex, read-only) of the first commit found five issues, all fixed:**
  - The `RUNNING` record write and fsync ran before the timeout and heartbeat timers existed. It now
    runs beside the runner, and `FINISHED` waits for it. The spec now says that run-file writes have
    no time bound but start only after the group is retired.
  - The launcher reported `OK` for a `RUNNING` record without checking that its supervisor was still
    live. It now checks the exit of its child and the recorded identity.
  - The record decoder accepted trailing `]` or `}` and terminal facts on unfinished records. It now
    requires end of input and state-consistent facts.
  - The 64-run cap could be exceeded by concurrent launches, and directory listings were unbounded.
    Listings now read at most 65 entries, and a launcher recounts after reserving its directory.
  - The fence test measured from before the launch. It now measures from the release.
- **A second Codex review (of `0811d1bb`) found two issues, both fixed:**
  - `readBounded` opened the record and kept result with a blocking `open`, so a FIFO put in their
    place hung even `--attach --wait 0`. It now opens with `O_NONBLOCK|O_NOFOLLOW` and refuses a
    non-regular file `MALFORMED`.
  - `encoding/json` accepted a repeated key (the last wins), a differently cased key and a missing
    key (left zero). `checkRunRecordKeys` now requires each record key exactly once, spelled exactly,
    with a scalar value. `TestATRV0010_RunRecordKeysAreExact` covers both; removing the key check or
    restoring the blocking open each fails it.
  - Not changed: PID and start identity do not prove liveness of a zombie under a non-reaping
    adopter, as the spec's failure table already records.
- **Dispatcher integration is deferred.** It is listed as remaining work in the spec. CAL-V0-056
  `refreshTree` adopts children of recorded members by parent PID, and the heal hand-off releases an
  ended worker's attempt, which fences the run. Changing either one needs an owner decision on
  identity-verified exclusion and on deferred hand-off or holder transfer. Another lane is also editing
  `internal/tasks/dispatch`. Under the dispatcher a detached run is therefore stopped, never left
  unfenced.

## Evidence

All runs were on Darwin with `GOMAXPROCS=2 GOTOOLCHAIN=local go test -p 1 -count=1 -timeout 30m`.

- `-run 'TestATRV00|TestCALV0047|TestPSRPublicRouteAndHelp|TestTMV0008_AS07' ./internal/tasks/cli/`
  PASSED, including:
  - TestATRV0008_DetachedRunSurvivesItsLauncher: the launcher's group is gone, the command lives,
    the attempt heartbeats after the launcher exited, and attach returns 75 and then 3 with the
    recorded outcome and bounded output. The replay is byte-identical.
  - TestATRV0013_FencedAttemptKillsTheDetachedCommand: release gives 125 `FENCED`, `LOST_LEASE` is
    recorded, and the group is gone.
  - TestATRV0009_DetachedPreLaunchRefusalIsReplayed.
  - TestATRV0012_AttachRefusesAProcessIdentityMismatch.
  - TestATRV0011_DetachAndAttachUsage.
  - TestATRV0010_RunRecordFactsMustAgreeWithItsState and TestATRV0010_DetachedRunsPerAttemptAreCapped
    (added after review).
- `-count=3 -run 'TestATRV0008_|TestATRV0013_'` PASSED, and again after the review fixes with
  `TestATRV0010_|TestATRV0012_` included.
- Mutation checks, each reverted afterwards:
  - Dropping `Setsid` failed TestATRV0008 ("launcher group still has members").
  - Trusting a bare PID failed TestATRV0012.
  - Restoring the old `dec.More()` trailing-data check failed TestATRV0010_RunRecordFactsMustAgreeWithItsState.
- `go vet ./internal/tasks/cli/` PASSED for GOOS darwin, linux and windows. `gofmt -l` was clean.
- These checks PASSED: `make spec-requirements-check requirement-definitions-check
  line-citations-check traceability-tests-check unbounded-readers-check error-code-ownership-check
  use-case-receipts-check diagnostic-coverage-check`.

## Not run

- NOT_RUN: Linux execution (`/proc` start identity and setsid launch), Windows execution (ATR-V0-014 is
  vet only), `make gate`, the full `internal/tasks/cli` and `cmd/corvint` suites, live dispatcher and
  agent-host qualification, CEM dogfood binding, and native completion of V1-0856.
- Not covered by a test: a supervisor that dies just after its `RUNNING` write (no hook was added
  for it), and a concurrent launch at the cap.

## Rollback

Revert the commit. Program mode, attached runs and existing receipts are unchanged. Supervisors that
are already running finish within their own bound. After none is live,
`<git-common-dir>/taskman-runs/` may be deleted; every recorded run's `RUN_OUTCOME` stays in the journal.
