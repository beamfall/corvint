# Corvint Tasks attempt runner V0

Owner: Russell Lewis
Date: 2026-10-04 (first drafted 2026-10-02 as a private Gate A proposal; detached runs added 2026-10-06)
Intent status: proposed (owner issues 481 and 627; technical profile not owner-accepted); ATR-V0-020..039 accepted (standing auto-accept 2026-10-11; V1-1547, V1-1549; re-homed from the agent leases spec, owner-approved 2026-10-17); ATR-V0-050..053 and ATR-V0-066 accepted (standing auto-accept 2026-10-11; V1-1558, issue 1026); ATR-V0-054..055 accepted (standing auto-accept 2026-10-11; V1-1575); ATR-V0-056..057 accepted (standing auto-accept 2026-10-11; V1-1562); ATR-V0-058..059 accepted (standing auto-accept 2026-10-11; V1-1600); ATR-V0-060..062 accepted (standing auto-accept 2026-10-11; V1-1595); ATR-V0-063..065 accepted (standing auto-accept 2026-10-11; V1-1597); ATR-V0-080..084 accepted (standing auto-accept 2026-10-11; V1-1608, V1-1673); ATR-V0-085..089 accepted (standing auto-accept 2026-10-11; V1-1647); ATR-V0-095 accepted (standing auto-accept 2026-10-11; V1-1671)
Delivery status: experimental
Authoritative inputs: owner request [issue 481](https://github.com/beamfall/corvint/issues/481); native ticket V1-0677;
owner request [issue 627](https://github.com/beamfall/corvint/issues/627) and native ticket V1-0856 (detached runs);
owner requests [issue 1005](https://github.com/beamfall/corvint/issues/1005) and [issue 1007](https://github.com/beamfall/corvint/issues/1007) and native tickets V1-1547 and V1-1549 (ATR-V0-020..039);
owner request [issue 1026](https://github.com/beamfall/corvint/issues/1026) and native ticket V1-1558 (ATR-V0-050..053, ATR-V0-066);
native ticket V1-1575 (ATR-V0-054..055);
native ticket V1-1562 (ATR-V0-056..057);
native ticket V1-1600 (ATR-V0-058..059, issue 1043);
native ticket V1-1595 (ATR-V0-060..062, issue 1038);
native ticket V1-1597 (ATR-V0-063..065, issue 1040);
native tickets V1-1608 and V1-1673 (ATR-V0-080..084, issues 1050 and 1068);
native ticket V1-1647 (ATR-V0-085..089, issue 1066);
native ticket V1-1763 (ATR-V0-101..107, issue 1109);
`docs/specs/corvint-tasks-agent-leases-v0.md` (external leases, heartbeat and renewal); the creation-owned
process-group API in `internal/groupreap/owner.go`; the process start identity in
`internal/tasks/supervisor` (`ProcessIdentity`); the in-tree sources under `internal/tasks/transaction`,
`internal/tasks/store` and `internal/tasks/cli`.

## Agent digest
- Claim: An experimental attempt runner keeps an externally leased attempt's heartbeat current, retires its owned process group and records an observational outcome.
- Status: proposed (owner issues 481 and 627; technical profile not owner-accepted); ATR-V0-020..039 accepted (standing auto-accept 2026-10-11; V1-1547, V1-1549; re-homed from the agent leases spec, owner-approved 2026-10-17); ATR-V0-050..053 and ATR-V0-066 accepted (standing auto-accept 2026-10-11; V1-1558, issue 1026); ATR-V0-054..055 accepted (standing auto-accept 2026-10-11; V1-1575); ATR-V0-056..057 accepted (standing auto-accept 2026-10-11; V1-1562); ATR-V0-058..059 accepted (standing auto-accept 2026-10-11; V1-1600); ATR-V0-060..062 accepted (standing auto-accept 2026-10-11; V1-1595); ATR-V0-063..065 accepted (standing auto-accept 2026-10-11; V1-1597); ATR-V0-080..084 accepted (standing auto-accept 2026-10-11; V1-1608, V1-1673); ATR-V0-085..089 accepted (standing auto-accept 2026-10-11; V1-1647); ATR-V0-095 accepted (standing auto-accept 2026-10-11; V1-1671); experimental. ATR-V0-001..007 are implemented with focused tests on Darwin; ATR-V0-008..013 (detached runs) are implemented with focused tests on Darwin; ATR-V0-014 is implemented without a test; ATR-V0-015 (retirement of ended runs of terminal attempts, V1-0931) is accepted by decision 0440 and implemented with a focused test on Darwin. Dispatcher integration of detached runs (V1-0936, V1-0858) is proposed as CAL-V0-145..154 in the agent leases spec and implemented with focused tests on Darwin; the run side is unchanged. V1-1549 ATR-V0-030..039 (dispatch context thresholds: per-role warn, hand-off and hard token thresholds over recognised host usage events, a per-worker context record and notice, and a hard-stop hand-off, issue 1007) accepted under the owner's standing auto-accept (2026-10-11), implemented with focused tests, live host qualification NOT_RUN; V1-1547 ATR-V0-020..029 (detached `claim` and `pool acquire` under the request id with attach, verbatim kept result and a 600-second ceiling, issue 1005) accepted under the owner's standing auto-accept (2026-10-11), implemented with focused tests on Darwin, Linux NOT_RUN (re-homed from the agent leases spec, owner-approved 2026-10-17). V1-1558 ATR-V0-050..053 and ATR-V0-066 (contended or interrupted store writes report retryable `LOCK_TIMEOUT`, never `MALFORMED`; outcome-write retry; one run item once started; a kept detached `LOCK_TIMEOUT` is retried by a same-id `--detach`, issue 1026) accepted under the owner's standing auto-accept, implemented with focused writer-contention tests on Darwin, Linux NOT_RUN. V1-1575 ATR-V0-054..055 (a context record of another profile version is preserved, never overwritten, and shows `UNKNOWN`; a damaged record alerts once and is replaced) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin. V1-1562 ATR-V0-056..057 (`dispatch status --worker ID` shows a worker's context notice from its record; the detached-run marker, now `taskman-dispatch-detached-run/1`, carries the `CONTEXT` hand-off detail across a restart) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin. V1-1600 ATR-V0-058..059 (a controlled dispatcher writes a drain-pending signal, named to every worker as `CORVINT_DISPATCH_DRAIN_FILE`, while its service control is `DRAINING`, and removes it when the drain lifts; a claim result carries the drain notice while the signal is present, issue 1043) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin. V1-1595 ATR-V0-060..062 (opt-in `shortSessions` holds a ticket's next launch after repeated short dispatcher sessions, kept in a session-churn record, shown in events and `dispatch status`, and a doctor SESSION_CHURN finding, issue 1038) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin. V1-1597 ATR-V0-063..065 (host suspend gaps detected from wall-over-monotonic jumps, recorded once with an alert, excluded from wall and idle caps, the lease keeper's idle check and the expired-lease grace, and shown in `dispatch status`, issue 1040) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin; keep-awake and a doctor finding are residuals. V1-1608 and V1-1673 ATR-V0-080..084 (an opt-in policy `progressSignal` `HANDOFF_EVIDENCE`: a changed hand-off `--evidence` reference is progress for loop detection, the dispatcher's no-progress backoff and the doctor, recorded as loop evidence and named in holds, issues 1050 and 1068) and V1-1647 ATR-V0-085..089 (`ticketBudget.overrides` per ticket, label or kind with precedence and the effective limit and selector in `dispatch status`, issue 1066) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin; live qualification NOT_RUN. V1-1671 ATR-V0-095 (a repeat `--detach` after a kept `LOCK_TIMEOUT` waits out the finishing child and relaunches even while its exiting process is still visible) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin, Linux NOT_RUN.
- Exists: `corvint-tasks run --attempt ID --generation G --timeout SECONDS [--detach] -- COMMAND...` (detached: survives the launching session), `corvint-tasks run --attach --attempt ID [--run RUN] [--wait SECONDS]`, the `RUN_OUTCOME` lease verb and its closed outcome document, the private run record of a detached run, and CLI, store and transaction tests.
- Blocked on: owner acceptance of this technical profile, Linux and composed native qualification, required CI, integration and native completion of V1-0677 and V1-0856; owner acceptance of the dispatcher integration CAL-V0-145..154 (V1-0936, see "Detached runs").
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
lease-holder transfer to a successor session, restart or adoption of a lost
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

### V1-1547 detached claim and pool acquire (issue 1005)

CAL-V0 IDs in this subsection are defined in `corvint-tasks-agent-leases-v0.md`.

Human-owned input: GitHub [issue 1005](https://github.com/beamfall/corvint/issues/1005) (2026-10-17,
native ticket V1-1547). An agent's tool call has its own timeout, often shorter than a claim that
waits for the preparation gate. When the host kills the call, the agent cannot tell whether the
claim committed, and a blind retry under a new request id can start a second mutation. Intent: let
a caller start `claim` or `pool acquire` in its own process session under the caller's
`--request-id`, return at once with that id as the handle, and later read the outcome. This builds
on the S19 request-id replay (CAL-V0-069) and the detached attempt runs (ATR-V0-008..015).

- `ATR-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1547)
  `claim ... --detach [--wait SECONDS]` and `pool acquire ... --detach [--wait SECONDS]` MUST
  require `--request-id`, which is the handle. They MUST run the unchanged lease command in a child
  started with `setsid`, with no inherited stdin or stdout, its stderr written to a private
  `child.log` (bounded and rotated per run, ATR-V0-028), and its own process group, so the caller's death or signals do not reach it.
  `claim --attach --request-id ID [--holder LABEL] [--wait SECONDS]` and
  `pool acquire --attach --request-id ID [--wait SECONDS]` MUST only read. The records live in
  `<git common dir>/taskman-requests/<sha256(request id)>/` beside the journal, never in the store:
  `request.json` (`taskman-detached-lease/0`: command, request id, holder and an argv digest),
  `child.json` (pid, process identity, start and ceiling times), `result.json` (the child's
  result envelope), `child.log` and `child.1.log`, and the `lock` and `running` lock files. Each is written by temporary file, fsync and rename, and read bounded with
  unknown members refused. `taskman-detached-lease/0` is in the CAL-V0-131 live format set, so a
  `request.json` naming another version is refused `UNSUPPORTED_VERSION` before it is decoded. `--detach`, `--attach` and `--wait` are refused `MALFORMED` before any
  record or start when combined wrongly (both modes, a repeat, `--wait` without a mode, a wait
  outside whole seconds 0..600, `--attach` with any other flag or a positional, `--detach` without
  `--request-id`). On a host other than Darwin or Linux they are refused `UNSUPPORTED`.
- `ATR-V0-021`: (accepted, V1-1547) A repeated `--detach` with the same command, request id and
  arguments MUST NOT start a second mutation. A launch takes an exclusive lock on the record's
  `lock` file, bounded at 10 seconds (`LOCK_TIMEOUT` past it); with the lock held it starts a child
  only when no `result.json` exists and no child is live. A child is live while it holds the
  exclusive `running` lock, which it takes before running and keeps until it exits, or, before
  that, while its recorded child is not proven ended (ATR-V0-027). A child that cannot take
  `running`, or finds `result.json` once it has it, exits without running. The same request id with another command, holder or argument digest is refused
  `REQUEST_ID_CONFLICT` without a start; so is an attach naming another command or holder. A
  child that ended without a result is reported by attach as `REFUSED` `SUPERVISOR_LOST`, state
  `LOST`, not retryable; a repeated `--detach` then starts a new child under the same request id,
  and the store's request-id replay (CAL-V0-069) returns a committed claim's attempt and
  generation instead of a second claim. The child checks that `request.json` matches its own
  argv before running.
- `ATR-V0-022`: (accepted, V1-1547) The child MUST keep the lease command's complete result
  envelope as `result.json`. `--detach` and `--attach` MUST return those bytes unchanged, with
  exit 0 for `OK` and 1 otherwise: a claim's `attemptId` and `generation`, or a refusal's
  outcome, codes, warnings and items exactly as the plain command would emit them, never a
  generic timeout or `SUPERVISOR_LOST`. Repeated attaches return the same bytes.
- `ATR-V0-023`: (accepted, V1-1547) The child MUST be bounded by a 600-second ceiling: the lease
  command runs under a context deadline at the ceiling, which bounds its lock waits and writer
  (the ceiling exceeds the 300-second `--lock-wait` maximum, CAL-V0-216), and a process still
  running 60 seconds past the ceiling exits 124 without a result, so attach reports it `LOST`.
  The command's own result at the deadline is kept like any other (ATR-V0-022); it is not `OK`.
  `child.json` records `ceilingAt`.
- `ATR-V0-024`: (accepted, V1-1547) `--wait SECONDS` MUST return within its bound plus the record
  poll (200 ms) and process start: the kept result when it appears, otherwise `REFUSED` with one
  item whose `state` is `PENDING` (request id, command, holder, child pid, start and ceiling
  times), exit 75. `--wait 0` or no `--wait` returns at once. The caller's wait never bounds or
  cancels the child. The item's advisory `progress` member (rounds, audit time and the walk
  split) is PSR-V0-112 and PSR-V0-152 in `docs/specs/corvint-tasks-pool-safe-reuse-v0.md`.
- `ATR-V0-025`: (accepted, V1-1547) A finished record (one with `result.json`) older than 7 days
  MUST be pruned, best effort and bounded, when a later `--detach` starts; a live or unfinished
  record is never pruned. An attach of an unknown or pruned handle is `REFUSED`
  `MISSING_EVIDENCE`.
- `ATR-V0-026`: (accepted, V1-1547) Without `--detach`, `--attach` or `--wait` the `claim` and
  `pool acquire` commands MUST be byte-identical to before: the same route, envelope and exit
  code, and no `taskman-requests` record. A lease flag's value (`--branch --detach`) is a value,
  not a detach flag. `claim --help` and `pool acquire --help` document the modes, the ceiling and
  the platforms.
- `ATR-V0-027`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1547 review
  repair) A recorded child that does not hold `running` MUST be reported ended (`LOST`, or
  restartable by a repeated `--detach`) only on positive evidence: no process at its recorded pid,
  a different process identity at that pid, or, when its identity cannot be confirmed (the launcher
  recorded `unknown`, or the probe fails now), no `running` lock once a 60-second startup window
  from `startedAt` has passed. Inside that window an unconfirmed child is `PENDING` and is not
  replaced, so a child still starting is never reported `LOST` and then commits.
- `ATR-V0-028`: (accepted, V1-1547 review repair) `child.log` MUST be bounded: each launch moves
  the previous run's `child.log` to `child.1.log` (replacing it) and starts an empty `child.log`,
  and what the child itself writes there is capped at 64 KiB, the rest dropped. Restarts therefore
  keep at most two runs' logs; both live in the record directory and are removed with it.
- `ATR-V0-029`: (accepted, V1-1547 review repair) Retention (ATR-V0-025) MUST page through the
  whole `taskman-requests` directory, 256 entries at a time and at most 65,536 entries per prune,
  and remove every expired finished record it has seen after the scan, so unfinished or lost
  records listed first never hide finished ones.

Non-goals: returning a holder's live attempt to a claim under a different request id (an exact
retry under the same id already replays it); detaching other lease verbs; a daemon, queue or
retry loop in the child; changing the store's request-id replay, its retention or any receipt;
Windows support.

Failure modes: a caller killed mid-launch, after the child started but before `child.json` was
written, leaves no `child.json`; the child is still found live by its `running` lock, so attach
reports `PENDING` and a repeated `--detach` starts nothing. If the caller dies before the child
has taken `running`, attach reports `PENDING` while the child is confirmed live or inside its
startup window (ATR-V0-027); should a repeated `--detach` still start a second child, only one of
the two takes `running` and runs, and the other exits without running. A host reboot or a `SIGKILL`
of the child is likewise `LOST`. A `result.json` that cannot be written leaves the record `LOST`
after the child exits, with the reason in `child.log`. Process identity unavailable on the host is
recorded as `unknown`; such a child is `PENDING` through its 60-second startup window and then
live only while it holds `running` (ATR-V0-027). A Go runtime crash report is written to
`child.log` by the runtime itself, outside the 64 KiB cap; it is bounded by the child's few
goroutines and by the per-run rotation (ATR-V0-028). More than 65,536 records in
`taskman-requests` leave the rest for a later prune (ATR-V0-029). Store refusals and cancellations
write nothing to the store (S19), so the kept `result.json` is the only copy of a refusal; it is
pruned after 7 days like the rest of the record.

Acceptance evidence (`internal/tasks/cli`, Darwin; the Linux path shares the code):
`TestATRV0021_DetachedClaimSurvivesKilledCaller` starts a detached claim from a launcher process
while the preparation gate is held, waits for the child, `SIGKILL`s the launcher's process group,
checks the child is alive and attach reports `PENDING` (exit 75), removes `child.json` and
checks a repeated `--detach` stays `PENDING` without starting a child, releases the gate, and reads
`OK` with `attemptId` and `generation` from attach; a second attach and a repeated `--detach`
return the same bytes with `head.json` unchanged, and the plain claim with the same request id
replays the same attempt. `TestATRV0021_LostChildRestartsWithoutASecondClaim` removes the kept
records, reads `SUPERVISOR_LOST`, refuses a changed holder, argument, attach holder and command
`REQUEST_ID_CONFLICT` without a start, and restarts to the replayed attempt with `head.json`
unchanged. `TestATRV0022_RefusalIsReturnedVerbatim` compares a detached refusal of a held ticket
and of a `pool acquire` with the plain commands' outcome, codes, warnings, exit and bytes.
`TestATRV0024_WaitReturnsWithinItsBound` times `--detach --wait 1`, `--attach --wait 1` and
`--attach` against the held gate. `TestATRV0023_CeilingEndsTheDetachedClaim` lowers the ceiling to
1.5 seconds and reads a kept, non-`OK` result. `TestATRV0026_WithoutDetachNothingChanges` checks no
record for plain claims, the `--branch --detach` value, every `MALFORMED` combination without a
record, `MISSING_EVIDENCE` for an unknown handle, and pruning inside and past retention.
`TestATRV0027_StartingChildIsNeverLost` makes the identity probe fail and delays the child's start
by 1.5 seconds: `--detach` and an immediate attach read `PENDING` with identity `unknown`, and the
child then commits; recorded children without `running` read `LOST` for a reaped pid, `PENDING` for
an unconfirmed identity inside the window and `LOST` after it.
`TestATRV0028_ChildLogIsCappedAndRotated` has the child write 4 KiB past the cap on each of three
restarted runs and reads a 64 KiB `child.log` and a 64 KiB `child.1.log` after each.
`TestATRV0029_PruneReachesEveryFinishedRecord` interleaves 100 unfinished and 100 expired finished
records and checks that every finished one is pruned and every unfinished one kept; the earlier
65-entry listing could remove at most 65. See `docs/build-log/2026-10-17-v1-1547-claim-detach.md`.

Rollback: revert the code and this amendment. The store, receipts, request-id replay and policy
are unchanged; leftover `taskman-requests` directories are inert and can be deleted.

### V1-1549 dispatch context thresholds (issue 1007)

CAL-V0 IDs in this subsection are defined in `corvint-tasks-agent-leases-v0.md`.

Human-owned input: GitHub [issue 1007](https://github.com/beamfall/corvint/issues/1007) (2026-10-17,
native ticket V1-1549). A dispatched session kept working long after its context grew too large to
work well. Nothing told the worker or the operator, and nothing handed the attempt to a fresh
session. Intent: each role declares context thresholds. The dispatcher estimates a running
session's context from the host event stream it already reads (CAL-V0-157), shows the estimate and
its age, and tells the worker once per crossing. Past the hard threshold it stops the worker and
hands the attempt off with a recorded reason. An estimate the dispatcher cannot read is unknown,
and an unknown estimate never stops a worker.

- `ATR-V0-030`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1549) A role MAY
  declare `context`, an object with optional `warnTokens`, `handoffTokens` and `hardTokens`. Each
  member, when present, is a JSON integer from 1 to 1000000000. At least one member is present.
  Present members strictly ascend in the order warn, handoff, hard. The object is decoded exactly:
  an unknown, repeated or case-folded member, `null`, `0` or a non-integer is refused with the
  configuration. `context` requires the role's `usageFormat` to be `claude-code` or `opencode`.
  Codex reports no per-call figure (ATR-V0-031), and a role without a usage format reads no usage.
  `hardTokens` requires `heal.handoff` to be `true`, so the hard stop always hands the attempt off.
- `ATR-V0-031`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1549) A worker's
  context estimate comes only from the CAL-V0-157 incremental read of its `stdout.log`, from these
  recognised lines:
  - `claude-code`: an `assistant` event whose top-level members are exact and unrepeated, whose
    `parent_tool_use_id` is present and `null`, and whose `message.usage` carries integer
    `input_tokens` and `output_tokens`, plus `cache_creation_input_tokens` and
    `cache_read_input_tokens` when present. The estimate is their sum. An event with a non-null
    `parent_tool_use_id` is a subagent's message and leaves the estimate unchanged.
  - `opencode`: a `step_finish` event that CAL-V0-077 reads. The estimate is that step's input
    (including cache read and write) plus its output (including reasoning).

  Each recognised line replaces the estimate; it is never summed with an earlier one. The estimate
  becomes UNKNOWN on any of these:
  - a recognised line that cannot be read: a member missing, `null`, repeated, case-folded or not a
    non-negative integer, or a sum that overflows
  - a `claude-code` `assistant` event without `parent_tool_use_id`
  - a line the usage reader marks malformed (CAL-V0-157) because it may be a context line

  Every other line leaves the estimate unchanged, including `codex` events, other event types and
  result totals. The dispatcher MUST NOT infer an estimate from output size, elapsed time, a
  tokenizer, or any line it does not recognise. The estimate's observation time is the tick that
  read the line.
- `ATR-V0-032`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1549) `dispatch
  status` MUST give each running worker a `context` object with these members:
  - `tokens`: the estimate in decimal, or `UNKNOWN`.
  - `observedAt` and `ageSeconds`: when the estimate was observed and how long ago. Both are
    `UNKNOWN` when the worker has no observation.
  - `level`: the highest recorded crossing (`NONE`, `WARN`, `HANDOFF` or `HARD`), or `UNKNOWN`
    when the record cannot be read.
  - `warnTokens`, `handoffTokens` and `hardTokens`: the applied configuration's thresholds for the
    worker's role, `NONE` when unset.

  A worker whose role reads no context estimate shows `tokens` `UNKNOWN` and level `NONE`. An
  unknown or unread estimate is never shown as `0`.
- `ATR-V0-033`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1549) For a worker
  with a context-capable usage format, the dispatcher keeps a record at
  `<stateDir>/<program>/workers/<worker>/context.json` (profile `taskman-dispatch-context/0`). It
  holds the estimate, its observation time, the applied thresholds, the level and the recorded
  crossings. The record is closed, bounded to 64 KiB and written atomically. On each supervising
  tick, for each threshold of the applied role whose value the observed estimate meets or exceeds
  and whose level has no crossing recorded for this worker, the dispatcher MUST:
  - append one crossing (level, threshold, estimate and time) to the record
  - after the record is written, append one `context` event naming the worker, role, ticket, level,
    threshold and estimate

  A level crossed once is never recorded or reported again for that worker, across dispatcher
  restarts, even if the estimate later drops or the threshold is lowered and crossed again. An
  UNKNOWN estimate crosses nothing. A record that cannot be written raises one alert per run of
  failed writes, records no crossing and emits no `context` event; the next tick tries again. A
  record that cannot be read raises an alert once and is replaced by a fresh one. The CAL-V0-131
  live format set gains `taskman-dispatch-context/0`, so replacing this build in place with an
  earlier one needs a drain (CAL-V0-130).
- `ATR-V0-034`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1549) A worker
  launched with a context-capable usage format MUST receive `CORVINT_DISPATCH_CONTEXT_FILE`, the
  absolute path of its ATR-V0-033 record. The record is the worker's notice. A worker that finds
  level `HANDOFF` should end its turn and leave its attempt for hand-off. Until the hard stop, the
  dispatcher takes no action on a crossing beyond the record and the event. The record is first
  written on the worker's first supervising tick; until then, and while `tokens` is `UNKNOWN`, no
  estimate is known.
- `ATR-V0-035`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1549) When a
  supervising tick observes a running worker whose estimate is known and meets or exceeds the
  applied role's `hardTokens`, and the worker is not already stopping, the dispatcher MUST stop its
  whole process tree with reason `CONTEXT`, through the same TERM-then-KILL path and kill grace
  (from the worker's launch configuration) as the wall cap. After the tree is gone, heal MUST hand
  off each live attempt the worker holds through the fenced release (CAL-V0-104). The `handoff`
  event, or the `handoff-refused` event with its exit recovery, carries `reason` `CONTEXT`, plus
  `contextTokens` and `hardTokens` from the record (`UNKNOWN` when unreadable). Every later event of
  the same hand-off carries them too: each exit-recovery retry (`handoff` or `handoff-refused`),
  and a detached-run deferral's `handoff-deferred` event and its eventual hand-off. The `finished`
  event keeps `killReason` `CONTEXT`. The decision uses the estimate, not the recorded crossings,
  so a restart between the stop and the hand-off still stops and hands off. An UNKNOWN estimate,
  or a role without `hardTokens`, never stops a worker.
- `ATR-V0-036`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1549) Context
  thresholds come from the applied configuration on every tick, not from the worker's launch
  configuration. A CAL-V0-127 reload that adds, raises, lowers or removes a role's `context` takes
  effect at the next tick for every running worker of that role. This is the one exception to
  CAL-V0-127's rule that a reload does not stop a running worker: a reload that sets a `hardTokens`
  at or below a running worker's known estimate stops it under ATR-V0-035. A role removed by a
  reload has no thresholds. Its running workers record no further crossings and are not stopped
  for context, and their earlier crossings stay recorded.
- `ATR-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1549) The estimate
  also becomes UNKNOWN when the read may have missed a context line:
  - an `opencode` line that the CAL-V0-077 reader refuses for a repeated or case-folded member,
    since such a member may hide an escaped `step_finish` type
  - output the read skipped under the CAL-V0-143 cut
  - a `stdout.log` shorter than the read offset

  An earlier known estimate, however high, then stops nothing until a later recognised line
  replaces it.
- `ATR-V0-038`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1549) An estimate
  read back from a ATR-V0-033 record (after a dispatcher restart) is shown by `dispatch status` but
  is not acted on. It records no crossing and makes no ATR-V0-035 stop until the dispatcher reads a
  fresh recognised line for that worker. A record written after a later stop or hand-off may
  therefore hold an estimate the current run never observed; that estimate stops nothing.
- `ATR-V0-039`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1549) This
  amends ATR-V0-032's threshold members. `warnTokens`, `handoffTokens` and `hardTokens` show the
  thresholds the dispatcher last applied to the worker, read from its ATR-V0-033 record, and are
  `NONE` when that record holds none. For a worker with a context-capable usage format whose record
  is absent or cannot be read, they are `UNKNOWN`. They are never taken from the configuration
  file, which may be invalid or edited but not yet reloaded. A worker whose role reads no context
  estimate shows `NONE`.

Non-goals: a Codex context estimate; a tokenizer or inferred estimate; a notice inside
`corvint-tasks` verb output (the worker reads its file); forcing a turn end at `WARN` or `HANDOFF`;
changing the CAL-V0-157 totals or budgets; a dispatcher ledger change (the record is beside the
worker logs and the ledger stays `taskman-dispatch-state/5`).

Failure modes:
- A host that changes its event shape leaves the estimate UNKNOWN or unchanged, so no stop happens;
  `ageSeconds` shows the staleness.
- A Claude Code session that compacts its context reports a smaller estimate on its next call.
  Recorded crossings stay recorded, so lowering does not re-arm a notice.
- A worker that ignores `HANDOFF` runs on to `hardTokens`, or to its wall or idle cap.
- A damaged record loses its crossings, so a level may be reported once more after the alert.
- A record that cannot be written leaves the worker without a notice until a write succeeds. The
  hard stop still follows the estimate.
- After a restart, a crossing whose record write failed before the restart is reported only after
  the worker's next recognised line (ATR-V0-038).
- After a restart, a detached-run deferral whose worker is no longer in the ledger hands off
  without the context detail. The strict detached marker cannot carry it, and the deferral's
  `handoff-deferred` event already did.
- An `opencode` line refused for a repeated member leaves the context estimate UNKNOWN, but the
  CAL-V0-157 usage totals still skip it as before.

Acceptance evidence: `TestATRV0030_ContextConfigIsExactAndBounded`,
`TestATRV0031_ContextEstimateFromRecognisedLinesOnly`,
`TestATRV0033_CrossingsRecordedOncePerSession`, `TestATRV0034_WorkerGetsContextFile`,
`TestATRV0035_HardStopHandsOffWithReason`, `TestATRV0035_UnknownEstimateNeverStops`,
`TestATRV0035_DeferredAndRecoveredHandoffsKeepContext`, `TestATRV0036_ThresholdsHotReload`,
`TestATRV0037_HiddenOrLostContextLinesAreUnknown` and
`TestATRV0038_ReloadedEstimateActsOnlyAfterAFreshLine` (`internal/tasks/dispatch`), and
`TestATRV0032_DispatchStatusShowsContextEstimate` and
`TestATRV0039_DispatchStatusShowsAppliedThresholds` (`internal/tasks/cli`). See
`docs/build-log/2026-10-17-v1-1549-dispatch-context-thresholds.md`. Live qualification with real
Claude Code or OpenCode sessions is NOT_RUN.

Rollback: remove `context` from every role, drain the dispatcher (its live format set differs,
CAL-V0-130), then revert the code and this amendment. The ledger version does not change. An
earlier build ignores the `context.json` records and refuses a configuration that still declares
`context`.

### V1-1558 contended store writes (issue 1026)

Human-owned input: GitHub [issue 1026](https://github.com/beamfall/corvint/issues/1026) (2026-10-17)
and native ticket V1-1558. Under heavy writer contention an attached `run --timeout N` exited 1 with
`ERROR`, `MALFORMED`, the warning `context deadline exceeded` and no item; a detached `claim` whose
ceiling expired, or that was interrupted, reported the same `MALFORMED`. A bare context error reached
`wire.CodeOf`, which maps every uncoded error to `MALFORMED`. Intent: a store write that ran out of
time or was interrupted reports one documented retryable code, and a run that started a command
never ends `MALFORMED`.

- `ATR-V0-050`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1558) A store
  write whose context expired or was cancelled MUST report `LOCK_TIMEOUT`, marked retryable, and
  MUST NOT report `MALFORMED`. This covers a write budget, the detached ceiling (ATR-V0-023) and an
  interrupt. It applies at the store write boundary (`administrativeWriteWith`), to the runner's
  pre-launch writes and reads, and to the lease command. Every context check in a write runs before
  that write commits. A command repeated with the same `--request-id` replays any step that
  committed earlier, so the retry is safe. The error still matches the context error under
  `errors.Is`. An error that already carries a code keeps it. A pre-launch failure of this kind
  exits 1, starts nothing, records nothing and carries no item.
- `ATR-V0-051`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1558) The runner's
  outcome write MUST use one request id. It retries a retryable write error (such as
  `LOCK_TIMEOUT`, `SNAPSHOT_MOVED` or `RESOURCE_EXHAUSTED`) with a 250 ms backoff inside one
  60-second write budget. The lease's coverage margin (ATR-V0-002) keeps that budget inside the
  lease. An outcome that is still not recorded when the budget ends exits 127, or 2 under `HOLD`,
  with the class's own code plus the write's code. A timeout whose outcome is recorded still exits
  124. Once the command has started, the run never reports `MALFORMED` for a store write.
- `ATR-V0-052`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1558) Whenever the
  command was started, the envelope MUST carry exactly one run item, whether or not the outcome was
  recorded. An unrecorded outcome has `outcomeReceipt` `null` and `exitStatus` `127`.
- `ATR-V0-053`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1558) A detached
  `claim` or `pool acquire` (ATR-V0-020) whose ceiling expires, or whose child receives SIGINT or
  SIGTERM, while its store write waits MUST keep a result coded `LOCK_TIMEOUT`, marked retryable,
  never `MALFORMED`. The child takes its interrupt handling before its first store read, so an
  interrupt from then on keeps that result. An interrupt that arrives before then ends the child
  without a result, and `--attach` reports `SUPERVISOR_LOST` as before (ATR-V0-022).
- `ATR-V0-066`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1558) A kept
  detached result coded `LOCK_TIMEOUT` (ATR-V0-053) MUST NOT answer a repeat `--detach` under the
  same request id: once no child still runs the record, the launcher moves it to `result.1.json`
  and starts a new child, and the store's request-ID replay keeps the mutation exactly once. While
  a child still runs the record, the kept result is returned as before. `--attach` still returns
  the kept result verbatim, and any other kept result still answers a repeat `--detach`.

Non-goals: a new wire code; a change to the lock wait budgets (`--lock-wait`, CAL-V0-263); retrying
pre-launch writes inside one command (the caller retries `LOCK_TIMEOUT`); and changing the
`authority` lock functions, which still return the bare context error to their callers.

Failure modes:
- A writer that holds the lock longer than the outcome budget leaves the outcome unrecorded. The
  run exits 127 with `LOCK_TIMEOUT`, and the attempt shows no outcome; reissuing `run` would run the
  command again.
- A context that ends after a write committed but before its reply (a post-commit read) still
  reports `LOCK_TIMEOUT`. The retry with the same request id replays the committed step.
- An interrupt that reaches a detached child before its handler is installed loses the result
  (`SUPERVISOR_LOST`), as before.

Acceptance evidence: the writer-contention tests hold the store writer lock and preparation gate
from a helper process: `TestATRV0050_ContendedPrelaunchWriteIsLockTimeout`,
`TestATRV0051_ContendedTimedOutRunExits127WithOneItem`,
`TestATRV0051_OutcomeWriteRetriesUntilTheLockFrees` and
`TestATRV0053_DetachedClaimCeilingAndInterruptAreLockTimeout` (`internal/tasks/cli`), plus
`TestATRV0050_ContextEndedIsRetryableLockTimeout` (`internal/tasks/wire`). Run against origin/main
072a306f1 with only the write-budget test hook ported, the contention tests fail with `MALFORMED`
`context deadline exceeded` (see `docs/build-log/2026-10-17-v1-1558-contended-store-writes.md`).

Rollback: revert the code and this amendment. No record, receipt, profile or wire code changes.

### V1-1575 context record versions

Human-owned input: native ticket V1-1575 (2026-10-17). The dispatcher replaced any unreadable
`context.json` (ATR-V0-033), including a record a newer build wrote with another profile version,
so a downgrade or a mixed-version dispatcher destroyed state it could not read. Intent: a record
of another version is never overwritten, and the damaged-record behaviour is specified.

- `ATR-V0-054`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1575) When a
  worker's context record carries another version of the `taskman-dispatch-context` profile
  (`UNSUPPORTED_VERSION`, CAL-V0-131), the dispatcher MUST NOT write, replace or remove it. It
  emits one `alert` event per dispatcher process naming the record. It records and reports no
  crossing for that worker. The hard stop (ATR-V0-035) still follows the in-memory estimate.
  `dispatch status` shows the worker's `context` members as `UNKNOWN`.
- `ATR-V0-055`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1575) A record
  of this profile version that is damaged, meaning not JSON, truncated, without a profile, or not
  in the closed canonical form, MUST cause one `alert` event. The dispatcher then replaces it
  with a fresh record on the next supervising tick, and later crossings are recorded and reported
  as for a new session. Until it is replaced, `dispatch status` shows `UNKNOWN`.

Non-goals: migrating a newer record, reading its fields, and a ledger or profile version change.

Failure modes: a worker whose record a newer build owns gets no WARN or HANDOFF notice from this
build, only the hard stop. Its record stays as the newer build left it, so status shows `UNKNOWN`
until that build, or a new session, writes it.

Acceptance evidence: `TestATRV0054_NewerContextRecordIsPreserved` and
`TestATRV0055_CorruptContextRecordIsReplacedAfterOneAlert` (`internal/tasks/dispatch`), and
`TestATRV0054_DispatchStatusShowsNewerRecordUnknown` (`internal/tasks/cli`). Without the change,
the first fails because the newer record is overwritten with a `taskman-dispatch-context/0` record.

Rollback: revert the code and this amendment. No record, profile or wire code changes.

### V1-1562 context notice verb and detached hand-off detail

Human-owned input: native ticket V1-1562 (2026-10-17), from the V1-1549 residuals. No
`corvint-tasks` verb showed a worker its context notice; the worker had to read the record file.
And a detached-run deferral whose worker had left the ledger after a dispatcher restart handed off
without the `CONTEXT` detail, because the strict marker could not carry it.

- `ATR-V0-056`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1562)
  `dispatch status` MUST accept `--worker ID`. When `ID` names a worker in the program's ledger,
  the status item gains a `contextNotice` object read from that worker's ATR-V0-033 record. It holds
  the ATR-V0-032 `context` members, `worker`, `file` (the record path) and `crossings`, each with
  `level`, `threshold`, `tokens` and `at`, in record order. An absent, unread or other-version
  record gives `UNKNOWN` members as ATR-V0-032 and ATR-V0-054 state, and an empty `crossings`
  array. An `ID` the ledger does not name is a usage error. The verb writes nothing.
- `ATR-V0-057`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1562) The
  CAL-V0-147 marker profile becomes `taskman-dispatch-detached-run/1`. It adds one optional
  closed member, `context`, with exactly `reason` (`CONTEXT`), `contextTokens` and `hardTokens`
  (each a canonical decimal or `UNKNOWN`). A deferral of a context-stopped worker MUST record it.
  The hand-off made from that marker, including after a restart when the worker has left the
  ledger, MUST carry those members as ATR-V0-035 states. A `/0` marker is still read, and it must
  not carry `context`. A marker with any other `context` value does not decode (CAL-V0-147). The
  CAL-V0-131 live format set lists `/1` in place of `/0`, so an earlier build holds a `/1` marker
  unread and hands its run off without a deferral, and replacing this build in place needs a
  drain (CAL-V0-130).

Non-goals: a notice push to the worker, an `attempt show` member, and migrating `/0` markers.

Failure modes: an earlier build that meets a `/1` marker hands off without the deferral, as
CAL-V0-147 states for another version. A `/0` marker written before the upgrade still hands off
without the `CONTEXT` detail when its worker has left the ledger.

Acceptance evidence: `TestATRV0056_DispatchStatusWorkerShowsContextNotice` (`internal/tasks/cli`),
`TestATRV0057_DeferredContextHandoffSurvivesRestart` and
`TestATRV0057_MarkerVersionsAndContextMember` (`internal/tasks/dispatch`), and the CAL-V0-131
`/1` row of `format_version_test.go`.

Rollback: revert the code and this amendment. A `/1` marker left behind is held unread by the
reverted build, whose run then hands off without a deferral.

### V1-1600 drain-pending signal

Human-owned input: native ticket V1-1600, from GitHub issue 1043 (2026-10-17). When a service
drain started, live workers did not know, kept starting long commands and held the drain up.
Intent: a documented signal that workers launched before the drain can read, a notice on
`claim`, and a signal that disappears when the drain lifts.

- `ATR-V0-058`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1600) Every
  worker MUST receive `CORVINT_DISPATCH_DRAIN_FILE`, the path `drain-pending.json` in the
  program's state directory, whether or not a drain is pending. At the start of each tick, a
  dispatcher whose launch control reports a drain (the service control is `DRAINING`) MUST
  write that file, once per drain, in the closed profile `taskman-dispatch-drain-pending/0`
  with `program`, `since` (RFC 3339) and a fixed `notice`. It emits one `alert` event with
  detail `drain=PENDING`. When the control no longer drains, or the dispatcher is uncontrolled,
  the file MUST be removed, with one `alert` event `drain=LIFTED` when it existed. A failed control
  read leaves the file as it is. A failed write or removal alerts once. The CAL-V0-131 live
  format set gains the profile.
- `ATR-V0-059`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1600) While
  the file named by `CORVINT_DISPATCH_DRAIN_FILE` reads as that profile, every `claim` result,
  plain or detached, admitted, replayed or refused, MUST carry the warning
  `DRAIN_PENDING since <since>: <notice>`. The claim itself is unchanged: during a drain the
  dispatcher launches no new worker (SERVICE500-003), so a claim comes from a worker already
  running, which the notice tells to finish, release and end. A signal that is absent or does not
  read adds nothing.

Non-goals: stopping or signalling live workers, refusing claims, a notice on verbs other than
`claim`, and a drain for an uncontrolled dispatcher.

Failure modes: a worker that ignores the notice still holds the drain until its own caps end it.
The signal can lag the control by one tick. A dispatcher killed during a drain leaves the file
until its next tick, which removes it if the drain has lifted.

Acceptance evidence: `TestATRV0058_DrainSignalReachesLiveWorkerAndClears` and
`TestATRV0058_UncontrolledDispatcherClearsLeftoverSignal` (`internal/tasks/dispatch`),
`TestATRV0058_LaunchControlReportsDrain` (`internal/tasks/service`),
`TestATRV0059_ClaimCarriesDrainNotice` (`internal/tasks/cli`), and the CAL-V0-131 row of
`format_version_test.go`.

Rollback: revert the code and this amendment. A leftover signal file is inert for an earlier
build.

### V1-1595 short-session cooldown

Human-owned input: native ticket V1-1595, from GitHub issue 1038 (2026-10-17). A ticket whose
sessions kept ending within seconds was relaunched at once, again and again, spending sessions with
no result. Intent: an opt-in hold after repeated short sessions on one ticket, visible in events,
`dispatch status` and the doctor, that never stops a running session.

- `ATR-V0-060`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1595) A dispatch configuration MAY set `shortSessions` with
  `seconds` (1..86400), `count` (2..100) and `holdSeconds` (1..86400), globally or on a role; a
  role's own setting replaces the global one for that role's sessions. When a ticket's session
  finishes, the applied setting of its role (from the configuration applied at that tick,
  CAL-V0-127) MUST extend the ticket's streak when the session lasted less than `seconds` from
  launch to finish, and MUST reset it otherwise. When the streak reaches `count`, the dispatcher MUST
  hold new launches of that ticket until `holdSeconds` after that session's end; a later short
  session extends the streak and renews the hold. When no role and no global setting is applied,
  the dispatcher MUST track nothing and hold nothing, and a reload that removes the setting lifts
  every hold on its next tick. Lane keys are not tracked.
- `ATR-V0-061`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1595) The streaks MUST be kept in `session-churn.json` in the program's
  state directory, in the closed profile `taskman-dispatch-session-churn/0`, so that they survive a
  dispatcher restart and `dispatch status` can read them. Each hold MUST emit one `cooldown` event
  with detail `reason=SHORT_SESSIONS`, `sessions`, `medianSeconds`, `firstSeen` and `holdUntil`.
  `dispatch status` MUST list each streak under `shortSessions` with `key`, `sessions`,
  `medianSeconds`, `firstSeen`, `lastEnd`, `held`, `reason` and `holdUntil` when set, and shows
  `state=UNKNOWN` for an unreadable record. A hold MUST NOT stop, signal or reap any running worker.
  A newer-version record is kept and disables tracking after one alert; another unreadable record
  is replaced after one alert. The CAL-V0-131 live format set gains the profile.
- `ATR-V0-062`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1595) The doctor MUST report `SESSION_CHURN` for a ticket that is not
  COMPLETED, ARCHIVED or HELD whose newest ended generations in the scan, at least three in a row,
  each lasted less than 300 seconds from the receipt that began the generation to the receipt that
  ended it. The finding names the ticket in `who`, and its detail gives the count, the median length
  and the first-seen time, which is also its `firstSeen`. A generation that began before the scan
  has an unknown length and ends the run. A ticket already reported as `NO_PROGRESS_HANDOFF` in the
  same run is not reported again. The finding carries its own remedy and no authority.

Non-goals: stopping or shortening running sessions, a dispatch-configured doctor threshold (the
doctor reads the journal, not dispatch configuration), and tracking pool-lane keys.

Failure modes: the streak counts dispatcher sessions, so sessions run outside the dispatcher do
not count toward a hold, while the doctor counts every ended generation. A record that cannot be
written alerts and the streak is kept only in memory until the next write. The doctor's 300-second
bound is fixed.

Acceptance evidence: `TestATRV0060_ShortSessionsHoldThenLongSessionResets`,
`TestATRV0060_AbsentSettingIsUnchangedAndRoleOverrides`,
`TestATRV0060_ReloadLiftsHoldAndRunningWorkersContinue`, `TestATRV0060_ShortSessionsValidation`
and `TestATRV0061_RecordVersions` (`internal/tasks/dispatch`),
`TestATRV0061_DispatchStatusShowsShortSessionHold` and `TestATRV0062_DoctorSessionChurn`
(`internal/tasks/cli`), and the CAL-V0-131 row of `format_version_test.go`.

Rollback: revert the code and this amendment. A configuration that sets `shortSessions` is then
refused as an unknown member; a leftover record file is inert for an earlier build.

### V1-1597 host sleep awareness

Human-owned input: native ticket V1-1597, from GitHub issue 1040 (2026-10-17). When the host
suspended, wall-clock timers kept counting, so on wake idle and wall caps fired and expired leases
were reaped for time no worker could use, and nothing reported the suspend. Intent: detect each
suspend, report it once, and keep it out of the dispatcher's own timers.

- `ATR-V0-063`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1597) Each dispatcher tick MUST, before any cap or lease
  decision, compare the wall-clock and monotonic time elapsed since the previous tick of the same
  process; when the wall clock advanced at least 30 seconds more, the difference MUST be recorded
  once as a suspend gap with `from`, `to` and `seconds`, in the record
  `taskman-dispatch-host-sleep/0` under the program's state directory (newest 64 gaps), and MUST
  emit one `alert` event with detail `hostSlept` (seconds), `from` and `to`. A tick without such a
  difference records nothing. A newer-version record is kept unchanged after one alert and gaps are
  then held in memory only; another unreadable record is replaced after one alert. The CAL-V0-131
  live format set gains the profile.
- `ATR-V0-064`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1597) The time of recorded gaps MUST be excluded from a running
  worker's wall cap (from launch), idle cap and lease-keeper `maxIdleSeconds` (from last activity),
  and from the CAL-V0-191 expired-lease grace (from the lease's expiry), so that none fires because
  of a suspend and each counts again from the wake. Without a recorded gap these timers are
  unchanged.
- `ATR-V0-065`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1597) `dispatch status` MUST show a `hostSleep` member with
  `lastGap` (`from`, `to`, `seconds`) and `gaps` (the number kept) when the record exists, and
  `state=UNKNOWN` with the reason when it does not read.

Non-goals: extending or renewing a store lease for the gap (a lease that expired during the
suspend is left to the reaper's grace, counted from the wake), a doctor finding for suspends, and a
keep-awake service option; the last two are residuals of issue 1040.

Failure modes: detection needs two ticks of one process, so a suspend across a dispatcher restart
is not seen; a wall-clock step of at least 30 seconds is indistinguishable from a suspend and is
excluded as one; the gap is attributed to the start of the interval between the two ticks.

Acceptance evidence: `TestATRV0063_SuspendIsRecordedAndExcludedFromTimers` (a three-hour gap is
recorded once, neither the wall nor the idle cap fires and the expired lease is not reaped, then
the grace counts from the wake), `TestATRV0063_NoGapLeavesTimersUnchanged` and
`TestATRV0063_RecordVersions` (`internal/tasks/dispatch`),
`TestATRV0065_DispatchStatusShowsLastHostSleep` (`internal/tasks/cli`), and the CAL-V0-131 row of
`format_version_test.go`. The lease keeper's idle check uses the same exclusion and has no
separate test.

Rollback: revert the code and this amendment; a leftover record file is inert for an earlier
build.

### V1-1608 and V1-1673 adopter-declared progress signal (issues 1050 and 1068)

Human-owned input: native tickets V1-1608 and V1-1673, from GitHub issues 1050 and 1068
(2026-10-17). Loop detection (CAL-V0-102), the dispatcher's no-progress backoff (CAL-V0-057) and the
doctor kinds NO_PROGRESS_HANDOFF, REPEAT_REFUSAL and SETUP_ONLY_PROOF judge progress only by queue
artefacts: a new candidate tree, a gate result or a review. A workflow that progresses through
branch commits, external test evidence or status files and hands off without submitting cannot
enable loop detection without every ticket being held. Intent: one opt-in, adopter-declared
progress signal that every no-progress judgment honours, with defaults unchanged.

Choice among the three options of issue 1068: the second, a changed `--evidence` reference on the
clean HANDOFF release counts as progress. The reference is already recorded on the attempt, bounded
(an Identifier of 1..128 bytes) and written by the agent at the moment it hands off, so no new verb,
receipt kind or journal format is needed and nothing runs a shell. The first option (a new
`attempt progress` verb) would add a native write path and receipt kind for the same fact; the third
(a policy-selected signal set) would let a policy remove existing artefact signals, which this
change does not need. The policy enum leaves room to add other signals later.

- `ATR-V0-080`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1608, V1-1673) The native policy MAY declare `progressSignal`, a closed enum whose only member is `HANDOFF_EVIDENCE`; any other value refuses with the policy's existing closed-reader refusal. When it is declared, the progress fingerprint of an ended generation is its clean hand-off's recorded `--evidence` reference, and no fingerprint when it recorded none. When it is absent, every judgment in ATR-V0-081..084 and TQD-V0-040..041 is unchanged.
- `ATR-V0-081`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1608) While `progressSignal` is declared, a claim MUST record `loopEvidence` for each prior generation, also under a policy without `loopDetection`, and it MUST carry the optional member `progress` `{signal, fingerprint}`: the declared signal and that generation's fingerprint, `null` when it recorded none. A fingerprint requires a HANDOFF or REVIEW_RETURNED disposition, otherwise the attempt record is MALFORMED. Without the declaration the member is absent and the record is byte-identical to before; a policy with neither declaration records no `loopEvidence`.
- `ATR-V0-082`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1608, V1-1673) Under a declared signal, the CAL-V0-102 no-progress run MUST count a generation only when it is no progress under the existing rule and its fingerprint is unchanged: none recorded, or the same as the latest earlier recorded fingerprint. A first or changed fingerprint is progress and ends the run. A generation whose `loopEvidence` has no `progress` member, or one recorded under another signal, is UNKNOWN under the declared signal and never counts, so declaring the signal never extends a streak recorded before it.
- `ATR-V0-083`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1608, V1-1673) A NO_PROGRESS loop hold judged under a declared signal MUST name it: `progressSignal` in the hold object of ticket blockers and plan preview, and `(progress signal S)` in the blocker text; the dispatcher's LOOP_DETECTED event carries detail `progressSignal` and names the signal in its message. A hold judged without one is unchanged.
- `ATR-V0-084`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1608) Under a declared `HANDOFF_EVIDENCE` signal the dispatcher's CAL-V0-057 progress fingerprint of a ticket MUST include each observed attempt's latest recorded hand-off reference: the current generation's, else the newest prior generation's ATR-V0-081 fingerprint, carried across generations that record none because a claim clears the current one. Only a hand-off with a reference different from that latest one resets the no-progress backoff; a repeated reference, or a claim and release that record none, does not; the cooldown and parked events then carry detail `progressSignal` and name the signal. Without the signal the fingerprint is computed exactly as before.

Non-goals: an `attempt progress` verb or receipt; a policy that removes the candidate-tree, gate or
review signals; reading branch commits or status files directly; and changing SETUP_ONLY_PROOF,
whose judgment is obligation witnesses, not progress (TQD-V0-042). The dispatcher ledger's loop
hold does not carry the signal; the event detail names it.

Failure modes: an agent that writes a new reference without real progress defeats the signal, which
is the adopter's declared trade; a reference repeated by mistake while work progressed keeps the
streak counting, as without the signal. Declaring the signal makes earlier generations UNKNOWN, so
an existing streak restarts. A generation that records no reference keeps the latest earlier
fingerprint for comparison.

Acceptance evidence: `TestATRV0080_PolicyProgressSignal` (`internal/tasks/intent`);
`TestATRV0081_LoopEvidenceProgressRoundTrip` and `TestATRV0081_LoopEvidenceWith`
(`internal/tasks/snapshot`); `TestATRV0081_ClaimRecordsProgressFingerprint` and
`TestATRV0082_LoopHoldHonoursProgressSignal` (`internal/tasks/transaction`: a changed reference is
not a no-progress generation, an unchanged or absent one still counts, and without the declaration
the hold is unchanged); `TestATRV0082_CLIProgressSignal` (`internal/tasks/cli`: blockers, plan
preview and the doctor through the real CLI); `TestATRV0084_FingerprintHonoursProgressSignal` and
`TestATRV0083_DispatcherEventsNameProgressSignal` (`internal/tasks/dispatch`); and the existing
CAL-V0-102, CAL-V0-057 and TQD-V0-005..008 tests unchanged.

Rollback: revert the code and this amendment. A store whose policy declares `progressSignal`
then refuses the policy at an earlier build, so remove the key first; recorded `progress` members
are refused as MALFORMED by an earlier build's attempt reader, so a rollback also needs the store's
attempts re-recorded or a build that still reads the member (inference, not tested).

### V1-1647 ticket budget overrides (issue 1066)

Human-owned input: native ticket V1-1647, from GitHub issue 1066 (2026-10-17), a follow-up to
issue 652 and V1-0937. `ticketBudget` (CAL-V0-155) is one uniform limit for every ticket, while
observed queues need about 56 sessions a day for a productive ticket and 6 to 8 for a stuck one;
operators keep an external per-ticket cap file that dispatch status cannot show. Intent: per-ticket,
per-label and per-kind limits under the same rolling-window rules, visible in dispatch status.

- `ATR-V0-085`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1647) A dispatch configuration's `ticketBudget` MAY carry `overrides`, an array of at most 256 objects, each with exactly one selector, `ticket` (a ticket ID or local key), `label` or `kind` (a string of 1..512 bytes), and `sessionsPerDay` (1..1000) and/or `tokensPerDay` (1..2^50). The default limits stay required. Only `ticketBudget` takes overrides.
- `ATR-V0-086`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1647) An override that is not an object, repeats or misspells a member, has no selector or more than one, has no limit, or has an out-of-range or null limit, an `overrides` value that is not an array (JSON `null` included) or an entry that is `null`, more than 256 overrides, and overrides on a role budget MUST refuse the configuration with the stable reason `BUDGET_OVERRIDE_MALFORMED` in the refusal message. Two overrides with the same selector type and value MUST refuse with `BUDGET_OVERRIDE_AMBIGUOUS`. An override `tokensPerDay` needs a `usageFormat` on every ticket role, as the default's does. A refused configuration is handled like any other CAL-V0-127 refusal.
- `ATR-V0-087`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1647) A ticket's effective limit MUST be chosen by precedence: a `ticket` override matching its ID or local key, then a `label` override matching one of its labels, then a `kind` override matching its kind, then the default. Within one precedence class, the first matching override in configuration order wins, so a ticket with two overridden labels resolves deterministically. The winner's limits replace the default's, and a limit the winner leaves unset stays the default's. A ticket key the observation did not supply resolves by its key alone, so only a `ticket` override can select it.
- `ATR-V0-088`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1647) With overrides configured, each ticket scope in `dispatch status`'s `budget.scopes` MUST show its effective `sessionsPerDay` and `tokensPerDay`, `selector` (`DEFAULT`, `ticket:ID`, `label:NAME` or `kind:KIND`), `selectorObserved` (false when the store did not supply the ticket's labels and kind) and the existing `held` hold reason. A ticket `budget` event MUST carry detail `selector`, `sessionsPerDay` and `tokensPerDay` and name the selector in its message. `dispatch status` reads the store's tickets only when overrides are configured. A store that cannot be read leaves every ticket unobserved.
- `ATR-V0-089`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1647) The CAL-V0-155..158 rolling-window, exhaustion, release and hold-recording rules MUST apply per effective limit. Without overrides, the configuration, holds, events and `dispatch status` output MUST be unchanged.

Non-goals: overrides for role budgets, a selector combining labels or kinds, an override that
removes a default limit, and a pool-scoped budget; the pool admission and acquire paths are untouched.

Failure modes: a ticket's labels or kind changing in the store moves it to another effective limit at
the next tick; the history counts against the new limit at once. A hold recorded for a ticket the
observation no longer supplies is judged by its key alone, so a label or kind override no longer
applies to it.

Acceptance evidence: `TestATRV0085_TicketBudgetOverridesDecode`,
`TestATRV0086_MalformedOrAmbiguousOverridesRefuse`, `TestATRV0087_EffectiveLimitPrecedence`,
`TestATRV0088_HoldsAndReportNameTheSelector` and `TestATRV0089_RollingWindowPerEffectiveLimit`
(`internal/tasks/dispatch`); `TestATRV0088_DispatchStatusShowsEffectiveLimitAndSelector`
(`internal/tasks/cli`); and the existing CAL-V0-155..160 budget tests unchanged.

Rollback: revert the code and this amendment. Remove `overrides` from any configuration first,
because an earlier build refuses the unknown member. The spend history and holds are unchanged in
shape.

### V1-1671 relaunch after a kept LOCK_TIMEOUT

Human-owned input: native ticket V1-1671 (2026-10-18). On CI the repeat `claim --detach` of
`TestPSRV0111_CeilingRelaunchesOnSameRequestID` once returned `SUPERVISOR_LOST` instead of the
ATR-V0-066 relaunch. The child that keeps a result releases the record's running lock before its
process exits and is reaped, so a repeat `--detach` in that window found its process live under
its recorded identity, started nothing, and the attach loop then reported the exited child LOST.
In the window before the release, the same repeat returned the kept `LOCK_TIMEOUT` instead. Intent:
the relaunch does not depend on how fast the finished child leaves.

- `ATR-V0-095`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1671; amended
  in place after review, 2026-10-18, so the wait is bounded by the caller's `--wait`) This
  refines ATR-V0-066. A repeat `--detach` that finds a kept `LOCK_TIMEOUT` MUST wait for the
  record's running lock, since a child holding it beside a kept result is only finishing, up to
  the launch-lock bound (10 seconds) or the end of the caller's `--wait`, whichever comes first,
  and MUST move the result to `result.1.json` while it holds that lock. One deadline, set when the
  command starts, covers this wait and the wait for the result, so ATR-V0-024 holds: `--wait 0`
  or no `--wait` tries the running lock once and returns at once, and `--wait N` returns within
  N seconds plus its margins. Having moved the result, the launcher MUST start a new child
  without consulting the recorded child's process liveness (ATR-V0-027): that child released the
  running lock after keeping the result and only exits. When the running lock is still held at
  the bound, nothing is moved or started and the kept `LOCK_TIMEOUT` is returned verbatim; the
  caller repeats `--detach`, with a `--wait` that leaves the child time to finish.

Non-goals: a change to `--attach`, to the LOST classification of a child that ended without a
result, or to the 600-second ceiling; a new wire code or record; waiting for the old process to
be reaped.

Failure modes: a child that holds the running lock beside a kept result for longer than 10 seconds,
or past the caller's `--wait`, leaves the kept `LOCK_TIMEOUT` as the answer, and the caller repeats
`--detach`. A concurrent launcher of the same record still waits up to 10 seconds for the launch
lock, outside the caller's `--wait`, as before this requirement. A child started by
a launcher that died before recording it may still take the running lock after the move and run
the request; the new child then exits without a result, and the store's request-ID replay keeps
the mutation exactly once, as under ATR-V0-021.

Acceptance evidence: `TestATRV0095_RelaunchWaitsOutTheFinishingChild` (the finishing child holds
the running lock for 300 ms and stays visible under its recorded identity; the repeat `--detach`
relaunches and claims, and the first result is kept as `result.1.json`; without ATR-V0-095 it
returns the kept `LOCK_TIMEOUT`), and `TestPSRV0111_CeilingRelaunchesOnSameRequestID` passing
`-count=50` beside 16 CPU-bound processes (50 of 50; the base failed 8 of 50 with
`SUPERVISOR_LOST`), and `TestATRV0095_RunningWaitIsBoundedByWait` (while the running lock is
held, `--wait 0` and no `--wait` return the kept `LOCK_TIMEOUT` at once and `--wait 1` within its
bound, with no `result.1.json`; once it frees, the same `--detach` relaunches; before the review
fix `--wait 0` returned after 10 seconds) (`internal/tasks/cli`). Before the change, a 500 ms delay injected
after the child's running-lock release reproduced the CI `SUPERVISOR_LOST` in 3 of 3 runs; after
it, the same injection and a 500 ms delay before the release both pass.

Rollback: revert the code and this amendment; records are unchanged, and an earlier build keeps
its one-shot running-lock probe and liveness check.

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
| Outcome not recorded after a released run (after ATR-V0-051 retries) | 127 | `ERROR` plus the write's code; one run item (ATR-V0-052) |
| Usage, other pre-launch refusal, unsupported platform | 1 | `REFUSED`/`ERROR`; a contended pre-launch write is retryable `LOCK_TIMEOUT` (ATR-V0-050) |
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

Dispatcher interaction. A new session takes the supervisor out of the launcher's session and process
group, so stopping the launcher's group or session does not reach it. The continuous dispatcher's
worker tree (CAL-V0-056) also adds the children of recorded members by parent PID. Before
CAL-V0-145, that adoption meant a supervisor seen while its launcher still ran was stopped with the
worker. A run stopped that way ends as `INTERRUPTED`. Independently, the hand-off released the
attempt, which fenced the run at its next heartbeat. The proposed dispatcher amendment in
`docs/specs/corvint-tasks-agent-leases-v0.md`, CAL-V0-145..154 (V1-0936, V1-0858), changes this
on Darwin and Linux. The dispatcher spares a `RUNNING` run's supervisor, and its session, only after
verifying that supervisor's start identity and session leadership. While the run runs, the
hand-off of the worker's attempt is deferred, bounded by the run's timeout plus 3 minutes. Once the
run finishes, the dispatcher hands the attempt off and launches one session for the same ticket and
role, which receives the outcome and the `run --attach` replay command. `dispatch status` reports
the run's state, read-only. The dispatcher only reads the run record and never writes it, so this
profile's requirements are unchanged.

Remaining work: lease-holder transfer to the successor session; deferral for a run still
`STARTING`; detached runs of dispatcher lane sessions; a compact one-line `run --attach --wait`
output; queue status of detached runs outside the dispatcher.

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
| Detached launch under the continuous dispatcher | On Darwin and Linux, an identity-verified `RUNNING` run's supervisor is spared, and the worker's hand-off is deferred until the run ends or its bound passes. A finished run relaunches one session with its outcome (CAL-V0-145..154). A `STARTING` run, an unverifiable record, a lane session or another platform keeps the earlier behaviour: the worker tree may stop the supervisor, and the hand-off fences the run. |
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
| Dispatcher context estimate unreadable, from an unrecognised host line, a subagent message or a Codex stream, or a context line hidden by a repeated member or lost to a log cut | The estimate is UNKNOWN or unchanged, shown with its age; UNKNOWN crosses no threshold and never stops a worker (ATR-V0-031, ATR-V0-032, ATR-V0-035, ATR-V0-037). |
| Dispatcher context record damaged, unwritable, or the dispatcher restarts mid-session | Crossings are recorded once in the worker's `context.json` before the event; a write failure alerts once and retries; the hard stop follows the estimate; an estimate read back after a restart acts only after a fresh line, and a worker already stopped is still handed off with reason `CONTEXT`, including through exit recovery and a detached-run deferral, whose marker carries the detail across a restart (ATR-V0-033, ATR-V0-035, ATR-V0-038, ATR-V0-057); a damaged record alerts once and is replaced, and one of another profile version alerts once and is never overwritten (ATR-V0-054, ATR-V0-055). |
| Progress signal declared but a hand-off records no `--evidence` reference, or repeats the last one | The generation counts toward the no-progress run as without the signal (ATR-V0-082, ATR-V0-084). |
| Progress signal declared on a store with an existing streak | Earlier generations are UNKNOWN under the signal and never count, so the streak restarts (ATR-V0-082). |
| Malformed or ambiguous `ticketBudget.overrides` | The configuration is refused with `BUDGET_OVERRIDE_MALFORMED` or `BUDGET_OVERRIDE_AMBIGUOUS` (ATR-V0-086). |
| Store unreadable when `dispatch status` resolves overrides | Every ticket is shown unobserved (`selectorObserved` false) and resolves by its key alone (ATR-V0-087, ATR-V0-088). |

## Acceptance and rollback

Acceptance evidence: the focused tests in Traceability on Darwin, `go vet`, the spec-index check and
one independent review of the delivered diff. Linux qualification, composed native qualification on a
live store, required CI, owner acceptance of the profile, integration and native completion of V1-0677
remain open and are not implied by this document. For detached runs (V1-0856) the same holds, plus
Linux qualification of the new-session launch and `/proc` start identity, and owner acceptance and live
qualification of the dispatcher integration CAL-V0-145..154 (V1-0936).

Detached claim and pool acquire (ATR-V0-020..029, V1-1547) and dispatch context thresholds (ATR-V0-030..039, V1-1549): the focused tests in their subsections and in Traceability on Darwin;
Linux qualification of the detached lease commands and live Claude Code or OpenCode qualification of
the context thresholds are NOT_RUN.

Adopter-declared progress signal (ATR-V0-080..084, V1-1608, V1-1673) and ticket budget overrides
(ATR-V0-085..089, V1-1647): the focused tests in their subsections and in Traceability on Darwin;
live dispatcher qualification and Linux are NOT_RUN.

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
| ATR-V0-020 | `internal/tasks/cli/lease_detach.go`, `internal/tasks/cli/lease_detach_unix.go`, `internal/tasks/cli/lease_detach_other.go` | `TestATRV0021_DetachedClaimSurvivesKilledCaller`, `TestATRV0026_WithoutDetachNothingChanges` (`internal/tasks/cli`) |
| ATR-V0-021 | `internal/tasks/cli/lease_detach.go` | `TestATRV0021_DetachedClaimSurvivesKilledCaller`, `TestATRV0021_LostChildRestartsWithoutASecondClaim` (`internal/tasks/cli`) |
| ATR-V0-022 | `internal/tasks/cli/lease_detach.go` | `TestATRV0022_RefusalIsReturnedVerbatim`, `TestATRV0021_DetachedClaimSurvivesKilledCaller` (`internal/tasks/cli`) |
| ATR-V0-023 | `internal/tasks/cli/lease_detach.go` | `TestATRV0023_CeilingEndsTheDetachedClaim` (`internal/tasks/cli`) |
| ATR-V0-024 | `internal/tasks/cli/lease_detach.go` | `TestATRV0024_WaitReturnsWithinItsBound` (`internal/tasks/cli`) |
| ATR-V0-025 | `internal/tasks/cli/lease_detach.go` | `TestATRV0026_WithoutDetachNothingChanges` (pruning) (`internal/tasks/cli`) |
| ATR-V0-026 | `internal/tasks/cli/lease_detach.go` | `TestATRV0026_WithoutDetachNothingChanges` (`internal/tasks/cli`); `docs/build-log/2026-10-17-v1-1547-claim-detach.md` |
| ATR-V0-027 | `internal/tasks/cli/lease_detach.go` | `TestATRV0027_StartingChildIsNeverLost` (`internal/tasks/cli`) |
| ATR-V0-028 | `internal/tasks/cli/lease_detach.go` | `TestATRV0028_ChildLogIsCappedAndRotated` (`internal/tasks/cli`) |
| ATR-V0-029 | `internal/tasks/cli/lease_detach.go` | `TestATRV0029_PruneReachesEveryFinishedRecord` (`internal/tasks/cli`) |
| ATR-V0-030 | `internal/tasks/dispatch/config.go`, `internal/tasks/dispatch/context.go` | `TestATRV0030_ContextConfigIsExactAndBounded` (`internal/tasks/dispatch`); `docs/build-log/2026-10-17-v1-1549-dispatch-context-thresholds.md` |
| ATR-V0-031 | `internal/tasks/dispatch/context.go`, `internal/tasks/dispatch/usage.go` | `TestATRV0031_ContextEstimateFromRecognisedLinesOnly` (`internal/tasks/dispatch`) |
| ATR-V0-032 | `internal/tasks/cli/dispatch.go`, `internal/tasks/dispatch/context.go` | `TestATRV0032_DispatchStatusShowsContextEstimate` (`internal/tasks/cli`) |
| ATR-V0-033 | `internal/tasks/dispatch/context.go`, `internal/tasks/dispatch/loop.go` | `TestATRV0033_CrossingsRecordedOncePerSession` (`internal/tasks/dispatch`); `TestCALV0131_LiveFormatsCoverEveryDecodedProfile` (`internal/tasks/cli`) |
| ATR-V0-034 | `internal/tasks/dispatch/loop.go` | `TestATRV0034_WorkerGetsContextFile` (`internal/tasks/dispatch`) |
| ATR-V0-035 | `internal/tasks/dispatch/context.go`, `internal/tasks/dispatch/loop.go`, `internal/tasks/dispatch/detached.go` | `TestATRV0035_HardStopHandsOffWithReason`, `TestATRV0035_UnknownEstimateNeverStops`, `TestATRV0035_DeferredAndRecoveredHandoffsKeepContext` (`internal/tasks/dispatch`) |
| ATR-V0-036 | `internal/tasks/dispatch/context.go`, `internal/tasks/dispatch/loop.go` | `TestATRV0036_ThresholdsHotReload` (`internal/tasks/dispatch`) |
| ATR-V0-037 | `internal/tasks/dispatch/context.go`, `internal/tasks/dispatch/usage.go` | `TestATRV0037_HiddenOrLostContextLinesAreUnknown` (`internal/tasks/dispatch`) |
| ATR-V0-038 | `internal/tasks/dispatch/context.go` | `TestATRV0038_ReloadedEstimateActsOnlyAfterAFreshLine` (`internal/tasks/dispatch`) |
| ATR-V0-039 | `internal/tasks/cli/dispatch.go`, `internal/tasks/dispatch/context.go` | `TestATRV0039_DispatchStatusShowsAppliedThresholds`, `TestATRV0032_DispatchStatusShowsContextEstimate` (`internal/tasks/cli`) |
| ATR-V0-050 | `internal/tasks/wire/context.go` (`ContextEnded`), `internal/tasks/wire/retry.go`, `internal/tasks/store/policy.go` (`administrativeWriteWith`), `internal/tasks/cli/attempt_run.go` (`prepare`) | `TestATRV0050_ContendedPrelaunchWriteIsLockTimeout` (`internal/tasks/cli`), `TestATRV0050_ContextEndedIsRetryableLockTimeout` (`internal/tasks/wire`) |
| ATR-V0-051 | `internal/tasks/cli/attempt_run.go` (`recordOutcome`, `finish`) | `TestATRV0051_ContendedTimedOutRunExits127WithOneItem`, `TestATRV0051_OutcomeWriteRetriesUntilTheLockFrees` (`internal/tasks/cli`) |
| ATR-V0-052 | `internal/tasks/cli/attempt_run.go` (`finish`) | `TestATRV0051_ContendedTimedOutRunExits127WithOneItem`, `TestATRV0004_UnrecordedOutcomeExits127` (`internal/tasks/cli`) |
| ATR-V0-053 | `internal/tasks/cli/lease.go`, `internal/tasks/cli/lease_detach.go` (`leaseDetachRunChild`), `internal/tasks/cli/cli.go` | `TestATRV0053_DetachedClaimCeilingAndInterruptAreLockTimeout` (`internal/tasks/cli`) |
| ATR-V0-066 | `internal/tasks/cli/lease_detach.go` (`leaseDetachKept`, `leaseDetachLaunch`) | `TestATRV0066_KeptLockTimeoutRelaunchesOnSameRequestID` (`internal/tasks/cli`) |
| ATR-V0-054 | `internal/tasks/dispatch/context.go` (`contextFor`, `noteContext`), `internal/tasks/cli/dispatch.go` (`dispatchWorkerContextValue`) | `TestATRV0054_NewerContextRecordIsPreserved` (`internal/tasks/dispatch`), `TestATRV0054_DispatchStatusShowsNewerRecordUnknown` (`internal/tasks/cli`) |
| ATR-V0-055 | `internal/tasks/dispatch/context.go` (`contextFor`) | `TestATRV0055_CorruptContextRecordIsReplacedAfterOneAlert` (`internal/tasks/dispatch`) |
| ATR-V0-056 | `internal/tasks/cli/dispatch.go` (`dispatchAux`, `dispatchContextNoticeValue`), `internal/tasks/cli/command_help.go` | `TestATRV0056_DispatchStatusWorkerShowsContextNotice` (`internal/tasks/cli`) |
| ATR-V0-057 | `internal/tasks/dispatch/detached.go` (`decodeDetachedMarker`, `validMarkerContext`, `advanceDetached`), `internal/tasks/cli/cli.go` (`LiveFormats`) | `TestATRV0057_DeferredContextHandoffSurvivesRestart`, `TestATRV0057_MarkerVersionsAndContextMember` (`internal/tasks/dispatch`) |
| ATR-V0-058 | `internal/tasks/dispatch/drain_signal.go`, `internal/tasks/dispatch/loop.go` (`tick`, worker env), `internal/tasks/service/fence.go` (`Draining`), `internal/tasks/cli/cli.go` (`LiveFormats`) | `TestATRV0058_DrainSignalReachesLiveWorkerAndClears`, `TestATRV0058_UncontrolledDispatcherClearsLeftoverSignal` (`internal/tasks/dispatch`), `TestATRV0058_LaunchControlReportsDrain` (`internal/tasks/service`) |
| ATR-V0-059 | `internal/tasks/cli/lease.go` (`withDrainNotice`) | `TestATRV0059_ClaimCarriesDrainNotice` (`internal/tasks/cli`) |
| ATR-V0-060 | `internal/tasks/dispatch/session_churn.go`, `internal/tasks/dispatch/config.go` (`shortSessions`), `internal/tasks/dispatch/loop.go` (`finish`, `launchRoster`, `tick`) | `TestATRV0060_ShortSessionsHoldThenLongSessionResets`, `TestATRV0060_AbsentSettingIsUnchangedAndRoleOverrides`, `TestATRV0060_ReloadLiftsHoldAndRunningWorkersContinue`, `TestATRV0060_ShortSessionsValidation` (`internal/tasks/dispatch`) |
| ATR-V0-061 | `internal/tasks/dispatch/session_churn.go`, `internal/tasks/cli/dispatch.go` (`dispatchSessionChurnValue`), `internal/tasks/cli/cli.go` (`LiveFormats`) | `TestATRV0061_RecordVersions` (`internal/tasks/dispatch`), `TestATRV0061_DispatchStatusShowsShortSessionHold` (`internal/tasks/cli`) |
| ATR-V0-062 | `internal/tasks/cli/doctor_churn.go`, `internal/tasks/cli/doctor.go` (`doctorStoreFindings`) | `TestATRV0062_DoctorSessionChurn` (`internal/tasks/cli`) |
| ATR-V0-063 | `internal/tasks/dispatch/host_sleep.go` (`noteHostSleep`), `internal/tasks/dispatch/loop.go` (`tick`), `internal/tasks/cli/cli.go` (`LiveFormats`) | `TestATRV0063_SuspendIsRecordedAndExcludedFromTimers`, `TestATRV0063_NoGapLeavesTimersUnchanged`, `TestATRV0063_RecordVersions` (`internal/tasks/dispatch`) |
| ATR-V0-064 | `internal/tasks/dispatch/host_sleep.go` (`slept`), `internal/tasks/dispatch/loop.go` (caps, `reapRunningExpired`), `internal/tasks/dispatch/keeper.go` | `TestATRV0063_SuspendIsRecordedAndExcludedFromTimers`, `TestATRV0063_NoGapLeavesTimersUnchanged` (`internal/tasks/dispatch`) |
| ATR-V0-065 | `internal/tasks/cli/dispatch.go` (`dispatchHostSleepValue`) | `TestATRV0065_DispatchStatusShowsLastHostSleep` (`internal/tasks/cli`) |
| ATR-V0-080 | `internal/tasks/intent/policy.go` (`ProgressSignal`, `DecodePolicy`), `internal/tasks/cli/command_help.go` | `TestATRV0080_PolicyProgressSignal` (`internal/tasks/intent`) |
| ATR-V0-081 | `internal/tasks/snapshot/attempt.go` (`LoopProgress`, `LoopEvidenceWith`, `readPriorLoop`), `internal/tasks/transaction/lease_claim.go` | `TestATRV0081_LoopEvidenceProgressRoundTrip`, `TestATRV0081_LoopEvidenceWith` (`internal/tasks/snapshot`), `TestATRV0081_ClaimRecordsProgressFingerprint` (`internal/tasks/transaction`) |
| ATR-V0-082 | `internal/tasks/transaction/loop_detect.go` (`LoopHoldOf`, `noProgressRun`, `progressChain`) | `TestATRV0082_LoopHoldHonoursProgressSignal` (`internal/tasks/transaction`), `TestATRV0082_CLIProgressSignal` (`internal/tasks/cli`) |
| ATR-V0-083 | `internal/tasks/ticket/view.go`, `internal/tasks/cli/plan.go` (`loopHoldValue`), `internal/tasks/dispatch/loop.go` (`loopEscalations`, `finish`) | `TestATRV0082_CLIProgressSignal` (`internal/tasks/cli`), `TestATRV0083_DispatcherEventsNameProgressSignal` (`internal/tasks/dispatch`) |
| ATR-V0-084 | `internal/tasks/dispatch/roster.go` (`baseFingerprint`), `internal/tasks/cli/dispatch.go` (`dispatchQueue.Observe`), `internal/tasks/snapshot/attempt.go` (`LatestHandoffEvidence`), `internal/tasks/transaction/lease_claim.go` | `TestATRV0084_FingerprintHonoursProgressSignal`, `TestATRV0083_DispatcherEventsNameProgressSignal` (`internal/tasks/dispatch`), `TestATRV0084_DispatchFingerprintCarriesLatestReference` (`internal/tasks/cli`), `TestATRV0084_LatestHandoffEvidence` (`internal/tasks/snapshot`) |
| ATR-V0-085 | `internal/tasks/dispatch/config.go` (`BudgetOverride`, `Budget.UnmarshalJSON`, `decodeBudgetOverride`) | `TestATRV0085_TicketBudgetOverridesDecode` (`internal/tasks/dispatch`) |
| ATR-V0-086 | `internal/tasks/dispatch/config.go` (`Budget.UnmarshalJSON`, `decodeBudgetOverride`, `validate`) | `TestATRV0086_MalformedOrAmbiguousOverridesRefuse`, `TestATRV0086_NullOverridesRefuse` (`internal/tasks/dispatch`) |
| ATR-V0-087 | `internal/tasks/dispatch/config.go` (`TicketLimit`), `internal/tasks/dispatch/budget.go` (`ticketLimit`, `observe`) | `TestATRV0087_EffectiveLimitPrecedence` (`internal/tasks/dispatch`) |
| ATR-V0-088 | `internal/tasks/dispatch/budget.go` (`recordBudget`, `SpendReportFor`), `internal/tasks/cli/dispatch.go` (`dispatchBudgetValue`, `dispatchBudgetTickets`) | `TestATRV0088_HoldsAndReportNameTheSelector` (`internal/tasks/dispatch`), `TestATRV0088_DispatchStatusShowsEffectiveLimitAndSelector` (`internal/tasks/cli`) |
| ATR-V0-089 | `internal/tasks/dispatch/budget.go` (`budget`, `budgetHeld`), `internal/tasks/dispatch/loop.go` (`tick`) | `TestATRV0089_RollingWindowPerEffectiveLimit` (`internal/tasks/dispatch`) |
| ATR-V0-095 | `internal/tasks/cli/lease_detach.go` (`leaseDetachKept`, `leaseDetachLaunch`) | `TestATRV0095_RelaunchWaitsOutTheFinishingChild`, `TestATRV0095_RunningWaitIsBoundedByWait`, `TestPSRV0111_CeilingRelaunchesOnSameRequestID` (`internal/tasks/cli`); `docs/build-log/2026-10-18-v1-1671-detach-relaunch-race.md` |
