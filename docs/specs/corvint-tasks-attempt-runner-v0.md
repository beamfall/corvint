# Corvint Tasks attempt runner V0

Owner: Russell Lewis
Date: 2026-10-04 (first drafted 2026-10-02 as a private Gate A proposal; detached runs added 2026-10-06)
Intent status: proposed (owner issues 481 and 627; technical profile not owner-accepted)
Delivery status: experimental
Authoritative inputs: owner request [issue 481](https://github.com/beamfall/corvint/issues/481); native ticket V1-0677;
owner request [issue 627](https://github.com/beamfall/corvint/issues/627) and native ticket V1-0856 (detached runs);
`docs/specs/corvint-tasks-agent-leases-v0.md` (external leases, heartbeat and renewal); the creation-owned
process-group API in `internal/groupreap/owner.go`; the process start identity in
`internal/tasks/supervisor` (`ProcessIdentity`); the in-tree sources under `internal/tasks/transaction`,
`internal/tasks/store` and `internal/tasks/cli`.

## Agent digest
- Claim: An experimental attempt runner keeps an externally leased attempt's heartbeat current, retires its owned process group and records an observational outcome.
- Status: proposed (owner issues 481 and 627; technical profile not owner-accepted); experimental. ATR-V0-001..007 are implemented with focused tests on Darwin; ATR-V0-008..013 (detached runs) are implemented with focused tests on Darwin; ATR-V0-014 is implemented without a test; ATR-V0-015 (retirement of ended runs of terminal attempts, V1-0931) is accepted by decision 0440 and implemented with a focused test on Darwin.
- Exists: `corvint-tasks run --attempt ID --generation G --timeout SECONDS [--detach] -- COMMAND...` (detached: survives the launching session), `corvint-tasks run --attach --attempt ID [--run RUN] [--wait SECONDS]`, the `RUN_OUTCOME` lease verb and its closed outcome document, the private run record of a detached run, and CLI, store and transaction tests.
- Blocked on: owner acceptance of this technical profile, Linux and composed native qualification, required CI, integration and native completion of V1-0677 and V1-0856; dispatcher integration of detached runs (see "Detached runs").
- Read next: Requirements; Exit status and envelope; Failure modes; Traceability.

## Intent and boundary

A calling agent needs to run a long local command while keeping its native attempt signal and lease
current. The command must stop useful work on timeout, interruption or observed lease loss; owned
descendants must be retired before cleanup is reported. A command observation is not a gate,
completion, physical isolation or authority grant.

This profile is experimental, supports externally leased attempts on Darwin and Linux only, and
preserves the existing program/config `run` mode. Supervised attempts and platforms without the owned
process-group API refuse before any effect. No daemon, account, network dependency, global
reservation hold or automatic retry is included. The detached mode (ATR-V0-008..014) adds exactly one
attempt-scoped supervisor process per run, which exits when its run ends and is never restarted.
Existing external release and reap can still change the attempt; the runner observes that through a
refused heartbeat and stops the command.

It amends the external-agent lease verb set of `corvint-tasks-agent-leases-v0.md` (and through it
TCP-00) with one additive observation verb, `RUN_OUTCOME`. No wire code, receipt kind, journal
namespace or attempt field is added.

Non-goals for V0, each a candidate for a later accepted amendment: a pre-spawn `RUN_BEGIN` receipt;
a private crash/restart quarantine marker; caller-supplied request IDs and same-request replay;
periodic renewal while the command runs (coverage is established once before launch); interactive
terminal input (the command runs in its own background process group); environment filtering (the
command inherits the caller's environment, as any wrapper does); Windows. For detached runs, also:
dispatcher integration, lease-holder transfer to a successor session, restart or adoption of a lost
supervisor, signalling a run from `--attach`, output streaming or following, and pruning of the run directories of a live attempt or on a read
path (ATR-V0-015 retires only ended runs of terminal attempts, from a supervisor).

## Requirements

- `ATR-V0-001`: `corvint-tasks run` MUST enter attempt mode only when `--attempt` appears before the first `--`, and MUST then accept exactly `--attempt ID --generation G --timeout SECONDS [--lease-minutes N] [--role ROLE] -- COMMAND...`. Unknown, repeated or program-mode flags, a missing delimiter or command, a timeout outside 1..86270 seconds, lease minutes outside 5..1440 (default 60) or an attempt of another queue are usage refusals before any effect. The command's argv is executed directly without a shell, in the caller's working directory, with the caller's stdin and environment. Program mode is unchanged.
- `ATR-V0-002`: Before launch the runner MUST heartbeat the attempt, then renew it only when the current lease does not cover the timeout plus the 10-second cleanup bound plus a 120-second scheduling and writer margin, using the larger of the requested minutes and the minutes that coverage needs. It MUST NOT shorten a longer lease, and it MUST refuse when the lease still does not cover timeout plus cleanup. A refused heartbeat or renewal starts nothing and records no outcome.
- `ATR-V0-003`: The command MUST be started once through `groupreap.Start` as the leader of an owned process group. On timeout, interruption or lost lease the runner MUST `Stop` the group, and in every started case it MUST retire it through `FinishBounded` with a 10-second bound; it never signals a numeric group ID after the reap. Only a `RELEASED` result reports cleanup as released; any other result is `HOLD`.
- `ATR-V0-004`: The runner MUST preserve the command's exit status and distinguish the outcome classes `EXIT`, `SIGNAL`, `TIMEOUT`, `LOST_LEASE`, `INTERRUPTED`, `SPAWN_FAILED` and `CLEANUP_HOLD` with the exit statuses in "Exit status and envelope".
- `ATR-V0-005`: Every started run, and every spawn failure, MUST be recorded by one `RUN_OUTCOME` lease receipt that posts only the canonical outcome document under `evidence/<sha256>`. The verb requires an attempt that exists and a generation between 1 and its current generation, but not a live or unexpired lease, so a fenced run's outcome is kept; it changes no attempt, lease, reservation or generation and renews nothing. The document must name the same attempt and generation.
- `ATR-V0-006`: While the command runs the runner MUST heartbeat about every 240 seconds with at most one heartbeat in flight. A refused heartbeat is a lost lease: the runner stops the group and records `LOST_LEASE` with the refusal code. A heartbeat write error is a warning only, because coverage was established before launch. After a released run that kept its lease, one final heartbeat is sent; its refusal records `LOST_LEASE`.
- `ATR-V0-007`: A platform without the owned process-group API MUST refuse with `UNSUPPORTED` before any write or start, and a supervised attempt is refused by the existing lease writer's heartbeat refusal before launch.
- `ATR-V0-008`: `--detach` before the first `--` (no value, at most once) MUST select the detached mode with the flags and usage refusals of ATR-V0-001. The launcher MUST create a fresh private run directory (ATR-V0-010), refusing `LIMIT_EXCEEDED` when the attempt already has 64 run entries; it counts again after creating its own directory and gives that directory up when the count exceeds 64, so concurrent launchers never keep more than 64, and it lists at most 65 entries. It then starts one supervisor (a re-execution of the same binary in the internal `--supervise RUN` form) in a new session, in the caller's working directory and environment, with stdin and stdout on the null device and stderr on the run's `supervisor.log`. It MUST wait at most 150 seconds for the supervisor's readiness and never for the command, and then return: `OK`, exit 0, with one run item when the record shows the command running and the launcher's supervisor has not exited and still has its recorded start identity (ATR-V0-012); the supervisor's kept result and status when the run already finished, as for a pre-launch refusal; `ERROR`, `SUPERVISOR_LOST`, exit 1, when the supervisor ended without a result; otherwise `REFUSED`, exit 75, without stopping anything. A failure before the supervisor starts removes the run directory and starts nothing. The command's stdin is the null device.
- `ATR-V0-009`: The supervisor MUST run the attached runner of ATR-V0-001..007 unchanged: pre-launch heartbeat and renewal, one owned process group, timeout, heartbeats, cleanup bound, final beat and one `RUN_OUTCOME`. Its envelope is captured and the command's output goes to the run's output files. It refuses unless its run directory exists without a record. It MUST write a `STARTING` record with its own PID and start identity before any lease write, a `RUNNING` record with the command's PID and start identity once the command starts, off the runner's path so that record I/O never delays the timeout, heartbeats or signal handling, and then signal readiness by closing an inherited descriptor that is close-on-exec, so neither Git nor the command inherits it. At return it MUST wait for the `RUNNING` write, keep the envelope as `result.json`, then write the `FINISHED` record with the exit status, the result's SHA-256 and the output totals, and exit with the run's status; a pre-launch refusal is kept the same way.
- `ATR-V0-010`: Run state MUST live under `<git-common-dir>/taskman-runs/<first 16 bytes of SHA-256 of the attempt ID, hex>/<16-hex run ID>/`, mode 0700, outside the journal. The record has profile `taskman-attempt-run-record/0`, at most 4096 bytes, exactly the keys in "Detached runs", is written only by its supervisor through a temporary file, fsync and rename, and stores the argv's SHA-256, never the argv. Readers MUST refuse unknown, repeated, missing or differently cased keys, a non-scalar value, trailing data, another profile (another version of `taskman-attempt-run-record` is `UNSUPPORTED_VERSION`, checked first, as amended by CAL-V0-131) or facts that disagree with the state (a command or result on a `STARTING` record, a result on a `RUNNING` one, an exit status outside 0..255, a result digest that is not lowercase SHA-256 hex, dropped output above the total) as `MALFORMED`, and MUST open the record and kept result without blocking or following a final symlink, refusing anything but a regular file as `MALFORMED`. The kept result is at most 64 KiB. Command output MUST be kept in at most two segments of 8 MiB (`output.log` and the previous `output.1.log`), with older bytes dropped and counted; an output write error never fails the command. Run state is never a receipt, evidence, gate input, ranking input or authority; `RUN_OUTCOME` stays the only native record of a run.
- `ATR-V0-011`: `run --attach --attempt ID [--run RUN] [--wait SECONDS]` MUST only read: it writes no lease, receipt, record or file and sends no signal. Without `--run` it selects the attempt's only run, listing at most 65 entries; none refuses `MISSING_EVIDENCE`, several refuse with one item per run, more than 64 refuse `LIMIT_EXCEEDED`. For a `FINISHED` run it MUST read the kept result bounded, check it against the record's SHA-256 and decode it as a command result before writing its bytes unchanged to stdout and returning the run's exit status, so every attach of a finished run is byte-identical; a mismatch or undecodable result is `ERROR`, `MALFORMED`. While the run is unfinished and its supervisor live, attach polls about once a second for up to `--wait` seconds (0..86400; omitted waits until the run ends) and then refuses with exit 75 and the run item.
- `ATR-V0-012`: A detached run's process MUST be judged live only when its recorded PID currently has its recorded start identity (`supervisor.ProcessIdentity`), never by a PID alone. When the supervisor is not live and a second read still shows no `FINISHED` record, attach MUST refuse with `SUPERVISOR_LOST`, exit 1, add a warning when the recorded command identity is still live (it runs unsupervised), and signal neither process.
- `ATR-V0-013`: A detached run MUST stay fenced by its lease as an attached run is: the supervisor heartbeats about every 240 seconds, and a release, reap or generation change refuses the next beat, which stops and retires the command's group and records `LOST_LEASE` (ATR-V0-006). The supervisor MUST exit when its run ends (command exit, timeout, lost lease, interruption or pre-launch refusal) after its finish writes, so its life is bounded by the timeout, the 10-second cleanup bound and the bounded store writes. Its run-file writes have no time bound: the `FINISHED` writes start only after the command's group is retired and its outcome recorded, so a stalled filesystem delays the supervisor's exit and the attach result, never the timeout or the fence. It is never restarted, and the command's descendants are retired as in ATR-V0-003, with the same limits.
- `ATR-V0-014`: A platform without new-session support (any other than Darwin and Linux) MUST refuse `--detach` and `--attach` with `UNSUPPORTED` before any write or start.
- `ATR-V0-015`: (accepted by decision 0440; V1-0931) After a detached supervisor has written
  its own run's `FINISHED` record, it MUST retire the ended runs of terminal attempts beyond the
  newest 16. The pass reads at most 1,024 attempt directories under `taskman-runs/` and at most 65
  entries in each.

  A run directory qualifies only when all of these hold:
  - its record decodes, names that run ID, and hashes to that attempt directory;
  - it has ended: its state is `FINISHED`, or neither its supervisor nor its recorded command is
    live by start identity (ATR-V0-012);
  - none of its files changed in the last hour.

  When more than 16 qualify, one audited store read (`store.AttemptRecords`) MUST still find each
  qualifying run's attempt in a terminal phase, at the run's recorded generation or a later one. A
  retry keeps the attempt ID and opens a newer generation. Qualifying runs beyond the newest 16 are
  then removed.

  Each removed run MUST first leave its attempt directory in one rename, to
  `taskman-runs/.retired-<attempt>-<run>`, and is then deleted. A later pass deletes any such
  directory that an interrupted pass left behind. An attempt directory left empty is removed.

  If attach has already read a `FINISHED` record and then finds the whole run gone, it MUST refuse
  `MISSING_EVIDENCE`, as for any absent run.

  The pass MUST keep every run of a live or absent attempt. It MUST also keep runs it cannot prove
  ended: a malformed record, a missing command identity, or an unreadable liveness check. It writes
  no lease, receipt or journal entry, and a failure is left for a later pass. `--attach` and every
  other read path MUST NOT retire anything (invariant 4).

## Outcome document

Profile `taskman-attempt-run-outcome/0`, at most 4096 bytes, compact canonical JSON with one trailing
LF and exactly these keys in this order: `profile`, `runId`, `attemptId`, `generation`, `argvSha256`
(SHA-256 of the argv joined by NUL; the argv itself is never stored), `timeoutSeconds`, `startedAt`
(null exactly for `SPAWN_FAILED`), `endedAt`, `class`, `exitCode`, `signal` (never both set),
`cleanup` (`NOT_STARTED` exactly for `SPAWN_FAILED`, else `RELEASED` or `HOLD`), `heartbeats`,
`renewals`, and `lostLease` (a closed code or null). Unknown keys, non-canonical bytes and
contradictory facts refuse as `MALFORMED`. Command output and exec errors are not native evidence.

Class consistency (contradictory facts). `exitCode` is 0..255 and `signal` 1..127 when set; `endedAt`
is not earlier than `startedAt`. A released group was reaped, so exactly one of `exitCode` and
`signal` is known; `signal` is the signal the child died from, never the runner's own.

| Class | `startedAt` | `cleanup` | `exitCode` / `signal` | `lostLease` |
| --- | --- | --- | --- | --- |
| `EXIT` | set | `RELEASED` | `exitCode` only | null |
| `SIGNAL` | set | `RELEASED` | `signal` only | null |
| `TIMEOUT`, `INTERRUPTED` | set | `RELEASED` | exactly one | null |
| `LOST_LEASE` | set | `RELEASED` | exactly one | the refusal code |
| `SPAWN_FAILED` | null | `NOT_STARTED` | neither | null |
| `CLEANUP_HOLD` | set | `HOLD` | neither | null, or the refusal code that stopped the run |

## Exit status and envelope

The command's stdout and stderr go to the caller's stderr; the one `taskman-command-result/0`
envelope goes to stdout, as for every other verb except `archive export`.
When the caller's stderr is not a file, the runner stops copying command output when it returns,
so output after a `HOLD`, which leaves the leader unreaped, never reaches the caller's writer.

| Case | Exit status | Envelope |
| --- | --- | --- |
| Command exited | its status | `OK` |
| Command killed by a signal it did not get from the runner | 128 + signal | `OK`, class `SIGNAL` |
| Timeout | 124 | `REFUSED`, `LIMIT_EXCEEDED` |
| Lost lease (pre-launch FENCED refusal, refused beat or final beat) | 125 | `REFUSED` with the writer's refusal code (`FENCED`, `TICKET_STATE`, ...; `MALFORMED` for a refusal without a code); FENCED is never inferred |
| Spawn failure | 126 | `ERROR`, `NOEXEC` |
| Runner interrupted by SIGINT, SIGTERM, SIGHUP or SIGQUIT | 128 + the runner's signal | `REFUSED`; `childSignal` is the child's own (usually 9, the group SIGKILL) |
| Cleanup `HOLD` | 2 (outranks every other status) | `ERROR`, `SURVIVORS` |
| Outcome not recorded after a released run | 127 | `ERROR` plus the write's code |
| Usage, other pre-launch refusal, unsupported platform | 1 | `REFUSED`/`ERROR` |
| Detached launch: command running | 0 | `OK`, one run item with `state` `RUNNING` |
| Detached launch or attach: run finished | the run's own status | the kept envelope, byte for byte |
| Detached launch or attach: run not finished (launch not ready, or `--wait` elapsed) | 75 | `REFUSED`, no code, one run item |
| Detached supervisor gone without a result | 1 | `ERROR` (launch) or `REFUSED` (attach), `SUPERVISOR_LOST` |
| Attach: no run, or several runs without `--run` | 1 | `REFUSED`, `MISSING_EVIDENCE`; or one item per run |

Pre-launch refusals carry no item. A recorded run carries one item with `runId`, `attemptId`,
`generation`, `class`, `cleanup`, `exitStatus`, `childExit`, `childSignal`, `lostLease`, `heartbeats`,
`renewals`, `outcomeSha256` and `outcomeReceipt`. A caller tells a command's own status 124..127 or 2
from a runner status by `class`. A detached run's recorded item also carries `output`, `outputBytes` and
`outputDroppedBytes`; a run item of an unfinished run carries `runId`, `attemptId`, `runDir`, `output`,
`generation`, `state`, `launchedAt`, `supervisorPid` and `commandPid`. Exit 75 (`EX_TEMPFAIL`) always
means "not finished, attach again"; a caller tells it from a command's own 75 by the envelope.

## Detached runs

A dispatcher or agent host may stop a session's whole process tree when the session ends. A long
attempt-bound command run with `--detach` keeps running after its launching session: the launcher
starts a supervisor in a new session and returns as soon as the command has started. The supervisor
is the attached runner of ATR-V0-001..007, so heartbeats, renewal, timeout, the owned group, fencing
and `RUN_OUTCOME` are unchanged. The next session for the same attempt runs `run --attach` to wait for
and read the result.

Run record (`record.json`), keys in this order: `profile`, `runId`, `attemptId`, `generation`,
`argvSha256`, `timeoutSeconds`, `state` (`STARTING`, `RUNNING` or `FINISHED`), `launchedAt`,
`supervisorPid`, `supervisorIdentity`, `commandPid`, `commandIdentity`, `endedAt`, `exitStatus`,
`resultSha256`, `outputBytes`, `outputDroppedBytes`. `RUNNING` requires `commandPid`; `FINISHED`
requires `endedAt`, `exitStatus` and `resultSha256`. The run directory also holds `result.json` (the
kept envelope), `output.log`, `output.1.log` and `supervisor.log` (the supervisor's own diagnostics).

Relation to CAL-V0-089. CAL-V0-089 continues a *supervised* stage past its stage wall by resuming the
recorded host session in its preserved worktree after proved quiescence, as an ordinary ANSWER and
DISPATCH under every cap. A detached run is unrelated machinery for *externally leased* attempts: the
attempt runner refuses supervised attempts (ATR-V0-007), so the two never apply to the same attempt.
A detached run does not resume, answer or dispatch anything, and it does not extend any wall: its own
`--timeout`, its lease and the lease's fencing still bound it. A later session that attaches only reads
the outcome; it neither takes over the lease nor continues the command.

Dispatcher interaction and remaining work. A new session takes the supervisor out of the launcher's
session and process group, so stopping the launcher's group or session does not reach it. The
continuous dispatcher's worker tree (CAL-V0-056, `refreshTree` in `internal/tasks/dispatch`) also adds
the children of recorded members by parent PID, so a supervisor observed while its launcher still
runs becomes a recorded member, and a later worker tree stop sends it and the command SIGTERM, which
interrupts the run (class `INTERRUPTED`; the outcome is recorded unless the dispatcher's SIGKILL grace
ends first). Independently, the dispatcher's hand-off releases the live attempts of an ended
worker, which fences a detached run at its next heartbeat. Both stop the run, so neither breaks
fencing, but they defeat the purpose under the dispatcher. Remaining work, each needing its own owner
decision and a dispatcher change: exclude a live, identity-verified run supervisor and command of the
worker's attempt from the worker tree; defer hand-off while such a run is live, or transfer the lease
holder to the successor session; and report detached runs in queue and dispatcher status. Until then,
detached runs serve externally leased attempts driven by agent hosts outside the dispatcher.

## Failure modes

| Failure | Behavior |
| --- | --- |
| Attempt fenced, released or of another generation before launch | Heartbeat refused; exit 125 for `FENCED`, otherwise 1; nothing starts or is recorded (ATR-V0-002). |
| Lease too short for the timeout | One renewal before launch; never shortens a longer lease (ATR-V0-002). |
| Lease released or reaped while the command runs | Next heartbeat refused; group stopped and retired; `LOST_LEASE` recorded (ATR-V0-006). |
| Heartbeat write error while running or at the final beat | One envelope warning with the count and last error; the up-front coverage still holds (ATR-V0-006). |
| Further signal during cleanup or the outcome write | Absorbed: the runner keeps its handler until it returns, so cleanup and the outcome write finish (ATR-V0-003). |
| Backgrounded descendant in the group | Killed with the group before cleanup is reported (ATR-V0-003). |
| Descendant that left the group (for example `setsid`) or holds the output pipes | Not retired by this runner; output copying is bounded by a one-second `WaitDelay`. An explicit limit. |
| Group cannot be confirmed retired within 10 seconds | `CLEANUP_HOLD`, exit 2; recorded as an observation, not as released (ATR-V0-003). |
| Outcome write fails or is refused | Exit 127 (or 2 under `HOLD`); the command's status stays in the envelope; no native completion is implied (ATR-V0-005). |
| Runner killed with SIGKILL or otherwise crashed (panic, OOM kill) | The command's group is never signalled: it keeps running, unsupervised and past its timeout and lease, until it exits. No outcome is recorded and no quarantine marker exists in V0; the lease expires normally. A host crash ends both. An explicit limit. |
| Launcher's session or process group stopped after a detached launch | The supervisor and the command run in another session and are not reached (ATR-V0-008). |
| Launcher killed before readiness, or readiness not reported within 150 seconds | The supervisor continues in its own session; a slow launcher exits 75 without stopping anything; a later attach reads the run (ATR-V0-008, ATR-V0-011). |
| Detached supervisor killed or crashed | As for a crashed runner: no restart and no signal. Attach reports `SUPERVISOR_LOST`, warns when the command is still live and unsupervised, and notes that a `RUN_OUTCOME` may already be in the journal (ATR-V0-012). |
| Supervisor PID reused by an unrelated process | The start identity differs, so the run is not taken for live: `SUPERVISOR_LOST` (ATR-V0-012). |
| Supervisor exited but not yet reaped | A finished record is read before liveness, so a finished run is unaffected. An unfinished, unreaped supervisor keeps its identity and looks live until its parent (init or a subreaper once the launcher has exited) reaps it; `--wait` bounds an attach. An explicit limit. |
| Detached launch under the continuous dispatcher | The worker tree may adopt the supervisor and stop it with the worker, and hand-off releases the attempt, which fences the run; the run stops and normally records its outcome. Dispatcher integration is remaining work ("Detached runs"). |
| Several runs of one attempt | Independent, as attached runs are; attach without `--run` refuses and lists them; at most 64 run directories per attempt; a live attempt's runs are never pruned, and after the attempt is terminal its ended runs beyond the newest 16 quiet ones are retired by a later supervisor (ATR-V0-010, ATR-V0-011, ATR-V0-015). |
| Concurrent detached launches of one attempt near the cap | Each launcher counts again after creating its directory and gives it up when over 64, so at most 64 stay; concurrent launchers may each refuse (ATR-V0-008). |
| Run-state filesystem stalls | The `RUNNING` write runs beside the runner and never delays its timeout, beats or fence; the launcher then exits 75 at its readiness bound. A stalled `FINISHED` write delays only the supervisor's exit; attach waits or reports `SUPERVISOR_LOST` if the supervisor is killed (ATR-V0-009, ATR-V0-013). |
| Supervisor dies just after its `RUNNING` record | The launcher does not report `OK`: it rereads the record and replays a finished run or reports `SUPERVISOR_LOST` (ATR-V0-008, ATR-V0-012). |
| Retired run attached or listed | A retired run is gone: attach refuses `MISSING_EVIDENCE` as for any absent run, and its `RUN_OUTCOME` stays in the journal. Retirement waits for an hour of quiet and a terminal attempt, and runs only when some supervisor finishes, so a host with no further detached runs keeps its old runs (ATR-V0-015). |
| A retry claims a terminal attempt between the store read and the removal | The store read takes no lock. A retry that revives the attempt can therefore lose some runs. They are only that attempt's earlier-generation runs, ended and quiet for an hour. No run of the new generation is removed, because removal requires the run's generation to be no newer than the terminal record that was read (ATR-V0-015). |
| More than 1,024 attempt directories | A pass reads the first 1,024 in directory order and completes retirement within them. Attempts beyond that window wait until the window shrinks. If the window holds mostly live or absent attempts, or runs that cannot be proven ended, the attempts behind it stall (ATR-V0-015). |
| Output volume or an output write error | Older output is dropped and counted; the command is never failed by its output (ATR-V0-010). |
| Run record or kept result altered or malformed | `MALFORMED`; nothing is replayed (ATR-V0-010, ATR-V0-011). |
| Run record written by a build with another run-record version | `UNSUPPORTED_VERSION`; nothing is replayed (ATR-V0-010 as amended by CAL-V0-131). |
| Finished record or result not written | The supervisor logs it to `supervisor.log` and exits; attach then reports `SUPERVISOR_LOST`, and the `RUN_OUTCOME` receipt, when written, stays the native record (ATR-V0-009, ATR-V0-012). |

## Acceptance and rollback

Acceptance evidence: the focused tests in Traceability on Darwin, `go vet`, the spec-index check and
one independent review of the delivered diff. Linux qualification, composed native qualification on a
live store, required CI, owner acceptance of the profile, integration and native completion of V1-0677
remain open and are not implied by this document. For detached runs (V1-0856) the same holds, plus
Linux qualification of the new-session launch and `/proc` start identity, and the dispatcher
integration listed in "Detached runs".

Rollback: revert the task-owned commit. Program mode, existing lease verbs and existing receipts are
unaffected. Already-recorded `RUN_OUTCOME` receipts stay in journals and a binary without the verb
may refuse to audit them (inference, not tested), so a revert either keeps the verb's audit path or
applies only to stores that recorded none. No state is erased or rewritten. For detached runs, a
revert removes `--detach` and `--attach`; supervisors already running finish within their own bound.
After none is live, `<git-common-dir>/taskman-runs/` may be deleted: it is private runtime state, never
evidence or authority, and every recorded run's `RUN_OUTCOME` stays in the journal. Reverting
ATR-V0-015 alone stops retirement; run directories already removed stay removed, and no record,
receipt or wire shape changes.

## Traceability

| Requirement | Implementation | Evidence |
| --- | --- | --- |
| ATR-V0-001 | `internal/tasks/cli/attempt_run.go` (`attemptMode`, `parseAttemptRun`), `internal/tasks/cli/cli.go` | TestATRV0001_UsageRefusesBeforeEffects, TestATRV0004_ExitStatusPassesThrough |
| ATR-V0-002 | `internal/tasks/cli/attempt_run.go` (`prepare`, `covers`), `internal/tasks/store/attempt_run.go` (`AttemptRecord`) | TestATRV0002_RefusedAuthorityStartsNothing, TestATRV0002_RenewsOnlyWhenCoverageIsShort |
| ATR-V0-003 | `internal/tasks/cli/attempt_run.go` (`attemptRun`, `execute`, `finish`, `runStatus`), `internal/groupreap/owner.go` | TestATRV0003_TimeoutRetiresDescendants, TestATRV0003_CleanupHoldOutranksStatus, TestATRV0004_HangupStopsTheRun, TestATRV0004_UnrecordedOutcomeExits127 |
| ATR-V0-004 | `internal/tasks/cli/attempt_run.go` (`execute`, `finish`, `runStatus`) | TestATRV0004_ExitStatusPassesThrough, TestATRV0004_SignalAndSpawnClasses, TestATRV0004_InterruptStopsTheRun, TestATRV0004_HangupStopsTheRun, TestATRV0004_UnrecordedOutcomeExits127 |
| ATR-V0-005 | `internal/tasks/transaction/attempt_run.go` (`EncodeRunOutcome`, `DecodeRunOutcome`, `runFactsDisagree`, `planRunOutcome`), `internal/tasks/transaction/lease.go`, `internal/tasks/store/attempt_run.go` (`RecordRunOutcome`) | TestATRV0005_OutcomeBindsItsAttemptGeneration, TestATRV0005_OutcomeDocumentIsClosed, TestATRV0005_OutcomeFactsAgree |
| ATR-V0-006 | `internal/tasks/cli/attempt_run.go` (`execute`, `finish`) | TestATRV0006_LostLeaseKillsTheRun, TestATRV0006_HeartbeatWriteErrorWarns |
| ATR-V0-007 | `internal/tasks/cli/attempt_run.go` (`attemptRun`), `internal/tasks/transaction/lease.go` (`planLease`) | NOT_PRODUCED: no test runs on a platform without the owned group API or against a supervised attempt |
| ATR-V0-008 | `internal/tasks/cli/attempt_run.go` (`parseAttemptRun`, `attemptRun`), `internal/tasks/cli/attempt_detach.go` (`launch`, `supervisorStillLive`), `internal/tasks/cli/attempt_detach_unix.go` (`detachAttr`) | TestATRV0008_DetachedRunSurvivesItsLauncher, TestATRV0011_DetachAndAttachUsage, TestATRV0010_DetachedRunsPerAttemptAreCapped |
| ATR-V0-009 | `internal/tasks/cli/attempt_detach.go` (`superviseRun`), `internal/tasks/cli/attempt_run.go` (`run`, `execute`, `finish`), `internal/tasks/cli/attempt_detach_unix.go` (`protectReadiness`) | TestATRV0009_DetachedPreLaunchRefusalIsReplayed, TestATRV0008_DetachedRunSurvivesItsLauncher |
| ATR-V0-010 | `internal/tasks/cli/attempt_detach.go` (`runRecord`, `decodeRunRecord`, `checkRunRecordKeys`, `readBounded`, `writeRunFile`, `runsDir`, `listRuns`, `runOutput`) | TestATRV0008_DetachedRunSurvivesItsLauncher, TestATRV0012_AttachRefusesAProcessIdentityMismatch, TestATRV0010_RunRecordFactsMustAgreeWithItsState, TestATRV0010_RunRecordKeysAreExact, TestATRV0010_DetachedRunsPerAttemptAreCapped, TestCALV0131_RunRecordFromAnotherBuildRefusesUnsupportedVersion |
| ATR-V0-011 | `internal/tasks/cli/attempt_detach.go` (`attemptAttach`, `soleRun`, `replayRun`) | TestATRV0008_DetachedRunSurvivesItsLauncher, TestATRV0009_DetachedPreLaunchRefusalIsReplayed, TestATRV0011_DetachAndAttachUsage, TestATRV0012_AttachRefusesAProcessIdentityMismatch |
| ATR-V0-012 | `internal/tasks/cli/attempt_detach.go` (`processLive`, `supervisorLost`), `internal/tasks/supervisor` (`ProcessIdentity`) | TestATRV0012_AttachRefusesAProcessIdentityMismatch |
| ATR-V0-013 | `internal/tasks/cli/attempt_detach.go` (`superviseRun`), `internal/tasks/cli/attempt_run.go` (`execute`) | TestATRV0013_FencedAttemptKillsTheDetachedCommand, TestATRV0008_DetachedRunSurvivesItsLauncher |
| ATR-V0-014 | `internal/tasks/cli/attempt_detach_other.go`, `internal/tasks/cli/attempt_detach.go` (`launch`, `attemptAttach`) | NOT_PRODUCED: no test runs on a platform without new-session support; `GOOS=windows go vet` only |
| ATR-V0-015 | `internal/tasks/cli/attempt_runs_retire.go` (`retireEndedRuns`, `runEnded`), `internal/tasks/cli/attempt_detach.go` (`superviseRun`, `replayRun`), `internal/tasks/store/attempt_run.go` (`AttemptRecords`) | TestATRV0015_SupervisorRetiresEndedRunsOfTerminalAttempts, TestATRV0015_AttachOfARunRetiredMidReadIsMissing |
