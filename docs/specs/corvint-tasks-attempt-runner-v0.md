# Corvint Tasks attempt runner V0

Owner: Russell Lewis
Date: 2026-10-04 (first drafted 2026-10-02 as a private Gate A proposal; detached runs added 2026-10-06)
Intent status: proposed (owner issues 481 and 627; technical profile not owner-accepted); ATR-V0-020..039 accepted (standing auto-accept 2026-10-11; V1-1547, V1-1549; re-homed from the agent leases spec, owner-approved 2026-10-17); ATR-V0-050..053 and ATR-V0-066 accepted (standing auto-accept 2026-10-11; V1-1558, issue 1026); ATR-V0-054..055 accepted (standing auto-accept 2026-10-11; V1-1575); ATR-V0-056..057 accepted (standing auto-accept 2026-10-11; V1-1562); ATR-V0-058..059 accepted (standing auto-accept 2026-10-11; V1-1600); ATR-V0-060..062 accepted (standing auto-accept 2026-10-11; V1-1595); ATR-V0-063..065 accepted (standing auto-accept 2026-10-11; V1-1597); ATR-V0-080..084 accepted (standing auto-accept 2026-10-11; V1-1608, V1-1673); ATR-V0-085..089 accepted (standing auto-accept 2026-10-11; V1-1647); ATR-V0-095 accepted (standing auto-accept 2026-10-11; V1-1671); ATR-V0-101..107 accepted (standing auto-accept 2026-10-11; V1-1763, issue 1109); ATR-V0-110..114 accepted (standing auto-accept 2026-10-11; V1-1782, issue 1122); ATR-V0-120..124 accepted (standing auto-accept 2026-10-11; V1-1833, issue 1147); ATR-V0-125..127 accepted (standing auto-accept 2026-10-11; V1-1834, issue 1148); ATR-V0-133..135 accepted (standing auto-accept 2026-10-11; V1-1852, issue 1157); ATR-V0-150..152 accepted (standing auto-accept 2026-10-11; V1-1914 and V1-1916, issues 1181 and 1183); ATR-V0-153 accepted (standing auto-accept 2026-10-11; V1-2103, issue 1116); ATR-V0-154 accepted (standing auto-accept 2026-10-11; V1-0934); ATR-V0-160..162 accepted (standing auto-accept 2026-10-11; V1-2123, V1-0951, V1-1767); ATR-V0-163..166 accepted (standing auto-accept 2026-10-11; V1-1857, V1-1853, V1-1854, V1-0748); ATR-V0-167..169 accepted (standing auto-accept 2026-10-11; V1-0769, V1-1793, V1-1792); ATR-V0-170 accepted (standing auto-accept 2026-10-11; V1-0832); ATR-V0-171 accepted (standing auto-accept 2026-10-11; V1-0933); ATR-V0-180..183 accepted (standing auto-accept 2026-10-11; V1-0925, V1-0924, V1-0934, V1-0948; re-homed from the agent leases spec, owner-approved 2026-10-17); ATR-V0-190..192 accepted (standing auto-accept 2026-10-11; V1-2263, V1-1964); ATR-V0-193 accepted (standing auto-accept 2026-10-11; V1-1692); ATR-V0-210..212 accepted (standing auto-accept 2026-10-11; V1-1321, V1-1237, V1-1441); ATR-V0-213 accepted (standing auto-accept 2026-10-11; V1-2189); ATR-V0-214..216 accepted (standing auto-accept 2026-10-11; V1-2709, issue 1818); ATR-V0-240..241 accepted (standing auto-accept 2026-10-11; V1-0831, V1-0775); ATR-V0-270..275 accepted (standing auto-accept 2026-10-11; V1-0830, V1-0828, V1-1874, V1-1438); ATR-V0-280..281 accepted (standing auto-accept 2026-10-11; V1-1964 as narrowed by decision 0552); ATR-V0-296..298 accepted (standing auto-accept 2026-10-11; V1-0826); ATR-V0-310..312 accepted (standing auto-accept 2026-10-11; V1-2305, V1-1559, V1-2173); ATR-V0-320..329 accepted (standing auto-accept 2026-10-11; V1-1634, V1-0911, V1-2368; re-homed from CAL-V0-361..363 and CAL-V0-365..371); ATR-V0-340..344 accepted (standing auto-accept 2026-10-11; V1-2649, issue 1754); ATR-V0-350..354 accepted (standing auto-accept 2026-10-11; V1-2694, issue 1799)
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
native ticket V1-1782 (ATR-V0-110..114, issue 1122);
native ticket V1-1833 (ATR-V0-120..124, issue 1147);
native ticket V1-1834 (ATR-V0-125..127, issue 1148);
native ticket V1-1852 (ATR-V0-133..135, issue 1157);
native tickets V1-1914 and V1-1916 (ATR-V0-150..152, issues 1181 and 1183);
native ticket V1-2103 (ATR-V0-153, issue 1116);
native ticket V1-0934 (ATR-V0-154);
native tickets V1-2123, V1-0951 and V1-1767 (ATR-V0-160..162);
native tickets V1-1857, V1-1853, V1-1854 and V1-0748 (ATR-V0-163..166);
native tickets V1-0769, V1-1793 and V1-1792 (ATR-V0-167..169);
native ticket V1-0832 (ATR-V0-170);
native ticket V1-0933 (ATR-V0-171);
native tickets V1-0925, V1-0924, V1-0934 and V1-0948 (ATR-V0-180..183, re-homed from CAL-V0-350..353);
native tickets V1-2263 and V1-1964 (ATR-V0-190..192);
native ticket V1-1692 (ATR-V0-193);
native tickets V1-1321, V1-1237 and V1-1441 (ATR-V0-210..212); native ticket V1-2189 (ATR-V0-213); native tickets V1-2305, V1-1559 and V1-2173 (ATR-V0-310..312); native tickets V1-1634, V1-0911 and V1-2368 (ATR-V0-320..329, re-homed from CAL-V0-361..363 and CAL-V0-365..371); native tickets V1-0830, V1-0828, V1-1874 and V1-1438 (ATR-V0-270..275); native ticket V1-0826 (ATR-V0-296..298); native ticket V1-2649 and issue 1754 (ATR-V0-340..344); native ticket V1-2694 and issue 1799 (ATR-V0-350..354);
`docs/specs/corvint-tasks-agent-leases-v0.md` (external leases, heartbeat and renewal); the creation-owned
process-group API in `internal/groupreap/owner.go`; the process start identity in
`internal/tasks/supervisor` (`ProcessIdentity`); the in-tree sources under `internal/tasks/transaction`,
`internal/tasks/store` and `internal/tasks/cli`.

## Agent digest
- Claim: An experimental attempt runner keeps an externally leased attempt's heartbeat current, retires its owned process group and records an observational outcome.
- Status: proposed (owner issues 481 and 627; technical profile not owner-accepted); ATR-V0-020..039 accepted (standing auto-accept 2026-10-11; V1-1547, V1-1549; re-homed from the agent leases spec, owner-approved 2026-10-17); ATR-V0-050..053 and ATR-V0-066 accepted (standing auto-accept 2026-10-11; V1-1558, issue 1026); ATR-V0-054..055 accepted (standing auto-accept 2026-10-11; V1-1575); ATR-V0-056..057 accepted (standing auto-accept 2026-10-11; V1-1562); ATR-V0-058..059 accepted (standing auto-accept 2026-10-11; V1-1600); ATR-V0-060..062 accepted (standing auto-accept 2026-10-11; V1-1595); ATR-V0-063..065 accepted (standing auto-accept 2026-10-11; V1-1597); ATR-V0-080..084 accepted (standing auto-accept 2026-10-11; V1-1608, V1-1673); ATR-V0-085..089 accepted (standing auto-accept 2026-10-11; V1-1647); ATR-V0-095 accepted (standing auto-accept 2026-10-11; V1-1671); ATR-V0-101..107 accepted (standing auto-accept 2026-10-11; V1-1763, issue 1109); ATR-V0-110..114 accepted (standing auto-accept 2026-10-11; V1-1782, issue 1122); ATR-V0-120..124 accepted (standing auto-accept 2026-10-11; V1-1833, issue 1147); ATR-V0-125..127 accepted (standing auto-accept 2026-10-11; V1-1834, issue 1148); ATR-V0-133..135 accepted (standing auto-accept 2026-10-11; V1-1852, issue 1157); ATR-V0-150..152 accepted (standing auto-accept 2026-10-11; V1-1914 and V1-1916, issues 1181 and 1183); ATR-V0-153 accepted (standing auto-accept 2026-10-11; V1-2103, issue 1116); ATR-V0-154 accepted (standing auto-accept 2026-10-11; V1-0934); ATR-V0-160..162 accepted (standing auto-accept 2026-10-11; V1-2123, V1-0951, V1-1767); ATR-V0-163..166 accepted (standing auto-accept 2026-10-11; V1-1857, V1-1853, V1-1854, V1-0748); ATR-V0-167..169 accepted (standing auto-accept 2026-10-11; V1-0769, V1-1793, V1-1792); ATR-V0-170 accepted (standing auto-accept 2026-10-11; V1-0832); ATR-V0-171 accepted (standing auto-accept 2026-10-11; V1-0933); ATR-V0-180..183 accepted (standing auto-accept 2026-10-11; V1-0925, V1-0924, V1-0934, V1-0948; re-homed from the agent leases spec, owner-approved 2026-10-17); ATR-V0-190..192 accepted (standing auto-accept 2026-10-11; V1-2263, V1-1964); ATR-V0-193 accepted (standing auto-accept 2026-10-11; V1-1692); ATR-V0-210..212 accepted (standing auto-accept 2026-10-11; V1-1321, V1-1237, V1-1441); ATR-V0-213 accepted (standing auto-accept 2026-10-11; V1-2189); ATR-V0-214..216 accepted (standing auto-accept 2026-10-11; V1-2709, issue 1818); ATR-V0-240..241 accepted (standing auto-accept 2026-10-11; V1-0831, V1-0775); ATR-V0-270..275 accepted (standing auto-accept 2026-10-11; V1-0830, V1-0828, V1-1874, V1-1438); ATR-V0-280..281 accepted (standing auto-accept 2026-10-11; V1-1964 as narrowed by decision 0552); ATR-V0-296..298 accepted (standing auto-accept 2026-10-11; V1-0826); ATR-V0-310..312 accepted (standing auto-accept 2026-10-11; V1-2305, V1-1559, V1-2173); ATR-V0-320..329 accepted (standing auto-accept 2026-10-11; V1-1634, V1-0911, V1-2368; re-homed from CAL-V0-361..363 and CAL-V0-365..371); ATR-V0-340..344 accepted (standing auto-accept 2026-10-11; V1-2649, issue 1754); ATR-V0-350..354 accepted (standing auto-accept 2026-10-11; V1-2694, issue 1799); experimental. ATR-V0-001..007 are implemented with focused tests on Darwin; ATR-V0-008..013 (detached runs) are implemented with focused tests on Darwin; ATR-V0-014 is implemented without a test; ATR-V0-015 (retirement of ended runs of terminal attempts, V1-0931) is accepted by decision 0440 and implemented with a focused test on Darwin. Dispatcher integration of detached runs (V1-0936, V1-0858) is proposed as CAL-V0-145..154 in the agent leases spec and implemented with focused tests on Darwin; the run side is unchanged. V1-1549 ATR-V0-030..039 (dispatch context thresholds: per-role warn, hand-off and hard token thresholds over recognised host usage events, a per-worker context record and notice, and a hard-stop hand-off, issue 1007) accepted under the owner's standing auto-accept (2026-10-11), implemented with focused tests, live host qualification NOT_RUN; V1-1547 ATR-V0-020..029 (detached `claim` and `pool acquire` under the request id with attach, verbatim kept result and a 600-second ceiling, issue 1005) accepted under the owner's standing auto-accept (2026-10-11), implemented with focused tests on Darwin and, since V1-1559, Linux (re-homed from the agent leases spec, owner-approved 2026-10-17). V1-1558 ATR-V0-050..053 and ATR-V0-066 (contended or interrupted store writes report retryable `LOCK_TIMEOUT`, never `MALFORMED`; outcome-write retry; one run item once started; a kept detached `LOCK_TIMEOUT` is retried by a same-id `--detach`, issue 1026) accepted under the owner's standing auto-accept, implemented with focused writer-contention tests on Darwin, Linux NOT_RUN. V1-1575 ATR-V0-054..055 (a context record of another profile version is preserved, never overwritten, and shows `UNKNOWN`; a damaged record alerts once and is replaced) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin. V1-1562 ATR-V0-056..057 (`dispatch status --worker ID` shows a worker's context notice from its record; the detached-run marker, now `taskman-dispatch-detached-run/1`, carries the `CONTEXT` hand-off detail across a restart) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin. V1-1600 ATR-V0-058..059 (a controlled dispatcher writes a drain-pending signal, named to every worker as `CORVINT_DISPATCH_DRAIN_FILE`, while its service control is `DRAINING`, and removes it when the drain lifts; a claim result carries the drain notice while the signal is present, issue 1043) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin. V1-1595 ATR-V0-060..062 (opt-in `shortSessions` holds a ticket's next launch after repeated short dispatcher sessions, kept in a session-churn record, shown in events and `dispatch status`, and a doctor SESSION_CHURN finding, issue 1038) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin. V1-1597 ATR-V0-063..065 (host suspend gaps detected from wall-over-monotonic jumps, recorded once with an alert, excluded from wall and idle caps, the lease keeper's idle check and the expired-lease grace, and shown in `dispatch status`, issue 1040) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin; keep-awake and a doctor finding are residuals. V1-1608 and V1-1673 ATR-V0-080..084 (an opt-in policy `progressSignal` `HANDOFF_EVIDENCE`: a changed hand-off `--evidence` reference is progress for loop detection, the dispatcher's no-progress backoff and the doctor, recorded as loop evidence and named in holds, issues 1050 and 1068) and V1-1647 ATR-V0-085..089 (`ticketBudget.overrides` per ticket, label or kind with precedence and the effective limit and selector in `dispatch status`, issue 1066) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin; live qualification NOT_RUN. V1-1671 ATR-V0-095 (a repeat `--detach` after a kept `LOCK_TIMEOUT` waits out the finishing child and relaunches even while its exiting process is still visible) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin, Linux NOT_RUN. V1-1763 ATR-V0-101..107 (warn and hand-off notices delivered into a claude-code session through a dispatcher-written PostToolUse hook, delivery mode in `dispatch status` with UNSUPPORTED for opencode, a bounded secret-screened hand-off note salvaged at the hard stop and offered to the next launch on the ticket, and a CONTEXT_EXHAUSTED state with one alert after `exhaustedAfter` consecutive hard stops, issue 1109) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin; live host qualification NOT_RUN (V1-1563). V1-1782 ATR-V0-110..114 (per-role `carryOver`: a no-progress session's screened final-message tail and end reason recorded per ticket, removed on progress, and appended to the next launch's prompt on the ticket as a bounded block labelled untrusted prior-session output, with `carryOver` members in the `launched` and `finished` events, issue 1122) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin; live host qualification NOT_RUN. V1-1833 ATR-V0-120..124 (opt-in `holdPool`: the dispatcher claims a pool ticket with its member before launch, names the hold record to the session, returns the allocation when the worker ends or a launch fails, sweeps orphaned claims and shows `poolHold` in `dispatch status`, issue 1147) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin; live qualification NOT_RUN. V1-1834 ATR-V0-125..127 (a host-neutral polled context-notice file for any host that declares `contextNotices: "file"`, reported as delivery `FILE`; `dispatch salvage --attempt A` writes the secret-screened hand-off note of a running or externally stopped session; `launched` and `finished` events record the delivery mode, the levels delivered, the offered note and the salvage, issue 1148) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin; live host qualification NOT_RUN. V1-1852 ATR-V0-133..135 (a carryOver host argv may hold any argv placeholder after argv[0], store-derived ones included, rechecked as rendered at each launch; `dispatch status` exits 1 on an INVALID config file; a restart on an invalid file refuses and runs no dispatcher, issue 1157) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin; live host qualification NOT_RUN. V1-1914 and V1-1916 ATR-V0-150..152 (a per-tick queue observation `admission.json` naming each run-only hold's fence and each role's queue depth; opt-in `runOnly`/`bypassFocus` and `capMax` at calm pressure, issues 1181 and 1183) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin; live dispatcher qualification NOT_RUN. V1-2103 ATR-V0-153 (the runner's owned group repeats its SIGKILL while a member forked alongside the stop stays live, issue 1116) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin; Linux NOT_RUN. V1-2649 ATR-V0-340..344 (per-ticket working directory for dispatch roles, issue 1754) accepted under the owner's standing auto-accept, implemented with focused tests on Darwin and Linux; live OpenCode qualification NOT_RUN.
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
  entries in each. ATR-V0-154 chooses which 1,024.

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
- `ATR-V0-021`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1547) A repeated `--detach` with the same command, request id and
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
- `ATR-V0-022`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1547) The child MUST keep the lease command's complete result
  envelope as `result.json`. `--detach` and `--attach` MUST return those bytes unchanged, with
  exit 0 for `OK` and 1 otherwise: a claim's `attemptId` and `generation`, or a refusal's
  outcome, codes, warnings and items exactly as the plain command would emit them, never a
  generic timeout or `SUPERVISOR_LOST`. Repeated attaches return the same bytes.
- `ATR-V0-023`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1547) The child MUST be bounded by a 600-second ceiling: the lease
  command runs under a context deadline at the ceiling, which bounds its lock waits and writer
  (the ceiling exceeds the 300-second `--lock-wait` maximum, CAL-V0-216), and a process still
  running 60 seconds past the ceiling exits 124 without a result, so attach reports it `LOST`.
  The command's own result at the deadline is kept like any other (ATR-V0-022); it is not `OK`.
  `child.json` records `ceilingAt`.
- `ATR-V0-024`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1547) `--wait SECONDS` MUST return within its bound plus the record
  poll (200 ms) and process start: the kept result when it appears, otherwise `REFUSED` with one
  item whose `state` is `PENDING` (request id, command, holder, child pid, start and ceiling
  times), exit 75. `--wait 0` or no `--wait` returns at once. The caller's wait never bounds or
  cancels the child. The item's advisory `progress` member (rounds, audit time and the walk
  split) is PSR-V0-112 and PSR-V0-152 in `docs/specs/corvint-tasks-pool-safe-reuse-v0.md`.
- `ATR-V0-025`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1547) A finished record (one with `result.json`) older than 7 days
  MUST be pruned, best effort and bounded, when a later `--detach` starts; a live or unfinished
  record is never pruned. An attach of an unknown or pruned handle is `REFUSED`
  `MISSING_EVIDENCE`.
- `ATR-V0-026`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1547) Without `--detach`, `--attach` or `--wait` the `claim` and
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
- `ATR-V0-028`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1547 review repair) `child.log` MUST be bounded: each launch moves
  the previous run's `child.log` to `child.1.log` (replacing it) and starts an empty `child.log`,
  and what the child itself writes there is capped at 64 KiB, the rest dropped. Restarts therefore
  keep at most two runs' logs; both live in the record directory and are removed with it.
- `ATR-V0-029`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1547 review repair) Retention (ATR-V0-025) MUST page through the
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
65-entry listing could remove at most 65. See `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-v1-1547-claim-detach.md`.

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
  replaces it. The refused `opencode` line also marks the CAL-V0-157 usage totals malformed, so
  they are reported `PARTIAL` (or `UNKNOWN` with no counted line), never `KNOWN`, because it may
  have been an uncounted `step_finish` (V1-1571 amendment).
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
changing the CAL-V0-157 totals or budgets (beyond the V1-1571 malformed mark in ATR-V0-037); a dispatcher ledger change (the record is beside the
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
  the worker's next recognised line (ATR-V0-038). This delay is accepted (V1-1571): reporting it
  earlier would act on an estimate the restarted run never observed, which ATR-V0-038 forbids, and
  the hard stop still follows the in-memory estimate.
- After a restart, a detached-run deferral whose worker is no longer in the ledger hands off with
  the context detail its `/1` marker carries (ATR-V0-057). Only a `/0` marker written before that
  upgrade still hands off without it.
- An `opencode` line refused for a repeated member leaves the context estimate UNKNOWN and marks
  the CAL-V0-157 usage totals malformed (ATR-V0-037, V1-1571).

Acceptance evidence: `TestATRV0030_ContextConfigIsExactAndBounded`,
`TestATRV0031_ContextEstimateFromRecognisedLinesOnly`,
`TestATRV0033_CrossingsRecordedOncePerSession`, `TestATRV0034_WorkerGetsContextFile`,
`TestATRV0035_HardStopHandsOffWithReason`, `TestATRV0035_UnknownEstimateNeverStops`,
`TestATRV0035_DeferredAndRecoveredHandoffsKeepContext`, `TestATRV0036_ThresholdsHotReload`,
`TestATRV0037_HiddenOrLostContextLinesAreUnknown` and
`TestATRV0038_ReloadedEstimateActsOnlyAfterAFreshLine` (`internal/tasks/dispatch`), and
`TestATRV0032_DispatchStatusShowsContextEstimate` and
`TestATRV0039_DispatchStatusShowsAppliedThresholds` (`internal/tasks/cli`). See
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-v1-1549-dispatch-context-thresholds.md`. Live qualification with real
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
Native ticket V1-1582, filed from the same issue, is its duplicate; its acceptance criteria are
these requirements and their tests (build log `2026-10-20-impact-deadline-retry`).

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
`context deadline exceeded` (see `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-v1-1558-contended-store-writes.md`).

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
The other lease-reap paths at a wake, and the Linux clock basis of the detector, are stated by
ATR-V0-320 and ATR-V0-321 (V1-1634): only the reap of a running worker's expired lease and the
worker's own timers exclude a gap.

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
- `ATR-V0-193`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1692) Rollback below the build that added the ATR-V0-081 `progress` member (merge `d9aa46b9e2a97219af9e70821fa1518d1a49f318`, PR #1085). A claim recorded while the policy does not declare `progressSignal` MUST write each `loopEvidence` with exactly the keys `candidateTreeOid`, `disposition`, `gateResults` and `reviews`, which an earlier build reads, so a binary rollback of a store that never declared the signal stays readable once the key is removed from the policy. An earlier build refuses a `loopEvidence` that carries `progress` as `MALFORMED` (closed object, unknown key `progress`), and it decodes every attempt record on each lease write, so after any claim recorded under the signal the store is unreadable to it. Attempt records and their journal afterimages are not rewritten, so the only supported recovery is to run a build at or after that merge; rolling back below it after such a claim is unsupported.

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
then refuses the policy at an earlier build, so remove the key first. A store that never recorded a
claim under the signal is then readable by the earlier build (ATR-V0-193). Once any claim recorded a
`progress` member, the earlier build's attempt reader refuses it as `MALFORMED: closed object
violated (unknown keys: progress)` on every lease write (recorded experiment, V1-1692 build-log
entry `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-attempt-1021h-tasks-attempt-sweep.md`); attempt records are not
rewritten, so the recovery is to keep a build at or after merge `d9aa46b9e` (PR #1085).

### V1-1647 ticket budget overrides (issue 1066)

Human-owned input: native ticket V1-1647, from GitHub issue 1066 (2026-10-17), a follow-up to
issue 652 and V1-0937. `ticketBudget` (CAL-V0-155) is one uniform limit for every ticket, while
observed queues need about 56 sessions a day for a productive ticket and 6 to 8 for a stuck one;
operators keep an external per-ticket cap file that dispatch status cannot show. Intent: per-ticket,
per-label and per-kind limits under the same rolling-window rules, visible in dispatch status.

- `ATR-V0-085`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1647) A dispatch configuration's `ticketBudget` MAY carry `overrides`, an array of at most 256 objects, each with exactly one selector, `ticket` (a ticket ID or local key), `label` or `kind` (a string of 1..512 bytes), and `sessionsPerDay` (1..1000) and/or `tokensPerDay` (1..2^50). Only `ticketBudget` takes overrides. Amended in place (standing auto-accept 2026-10-11; V1-1835, issue 1149): a `ticketBudget` with at least one override MAY omit both default limits. A ticket no override selects then has no ticket limit: it MUST never be held by the ticket budget, and `dispatch status` and the spend report MUST list as ticket scopes only the tickets an override selects, plus any ticket that still carries a recorded hold (its `selector` then `NONE`). A `ticketBudget` with neither a default limit nor an override is still refused, and a role budget still needs a limit.
- `ATR-V0-086`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1647) An override that is not an object, repeats or misspells a member, has no selector or more than one, has no limit, or has an out-of-range or null limit, an `overrides` value that is not an array (JSON `null` included) or an entry that is `null`, more than 256 overrides, and overrides on a role budget MUST refuse the configuration with the stable reason `BUDGET_OVERRIDE_MALFORMED` in the refusal message. Two overrides with the same selector type and value MUST refuse with `BUDGET_OVERRIDE_AMBIGUOUS`. An override `tokensPerDay` needs a `usageFormat` on every ticket role, as the default's does. A refused configuration is handled like any other CAL-V0-127 refusal.
- `ATR-V0-087`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1647) A ticket's effective limit MUST be chosen by precedence: a `ticket` override matching its ID or local key, then a `label` override matching one of its labels, then a `kind` override matching its kind, then the default. Within one precedence class, the first matching override in configuration order wins, so a ticket with two overridden labels resolves deterministically. The winner's limits replace the default's, and a limit the winner leaves unset stays the default's. A ticket key the observation did not supply resolves by its key alone, so only a `ticket` override can select it.
- `ATR-V0-088`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1647) With overrides configured, each ticket scope in `dispatch status`'s `budget.scopes` MUST show its effective `sessionsPerDay` and `tokensPerDay`, `selector` (`DEFAULT`, `ticket:ID`, `label:NAME` or `kind:KIND`), `selectorObserved` (false when the store did not supply the ticket's labels and kind) and the existing `held` hold reason. A ticket `budget` event MUST carry detail `selector`, `sessionsPerDay` and `tokensPerDay` and name the selector in its message. `dispatch status` reads the store's tickets only when overrides are configured. A store that cannot be read leaves every ticket unobserved.
- `ATR-V0-089`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1647) The CAL-V0-155..158 rolling-window, exhaustion, release and hold-recording rules MUST apply per effective limit. Without overrides, the configuration, holds, events and `dispatch status` output MUST be unchanged.

Non-goals: overrides for role budgets, a selector combining labels or kinds, an override that
removes a default limit (a default may instead be omitted, V1-1835), an explicit `unlimited` value, and a pool-scoped budget; the pool admission and acquire paths are untouched.

Failure modes: a ticket's labels or kind changing in the store moves it to another effective limit at
the next tick; the history counts against the new limit at once. A hold recorded for a ticket the
observation no longer supplies is judged by its key alone, so a label or kind override no longer
applies to it.

Acceptance evidence: `TestATRV0085_TicketBudgetOverridesDecode`,
`TestATRV0086_MalformedOrAmbiguousOverridesRefuse`, `TestATRV0087_EffectiveLimitPrecedence`,
`TestATRV0088_HoldsAndReportNameTheSelector` and `TestATRV0089_RollingWindowPerEffectiveLimit`
(`internal/tasks/dispatch`); `TestATRV0088_DispatchStatusShowsEffectiveLimitAndSelector`
(`internal/tasks/cli`); and the existing CAL-V0-155..160 budget tests unchanged. The V1-1835
amendment of ATR-V0-085: `TestATRV0085_OverridesWithoutDefaultValidate` and
`TestATRV0085_UnmatchedTicketsNeverHeld` (`internal/tasks/dispatch`) and
`TestATRV0085_DispatchStatusListsOnlyOverrideScopes` (`internal/tasks/cli`). Rollback of that
amendment alone: give every override-only `ticketBudget` a default limit first, because an earlier
build refuses it.

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
`--detach`. A concurrent launcher of the same record waits for the launch lock only up to the
same deadline (V1-1721): with `--wait 0` or no `--wait` it tries the lock once, it never tries
after the deadline, and at the deadline it returns `LOCK_TIMEOUT` having recorded and started nothing, and the caller repeats
`--detach` or `--attach`. A child started by
a launcher that died before recording it may still take the running lock after the move and run
the request; the new child then exits without a result, and the store's request-ID replay keeps
the mutation exactly once, as under ATR-V0-021. Attach and `--wait` stay `PENDING` while the stale
child holds the running lock (ATR-V0-021 liveness) and then return its one kept result; they never
report `SUPERVISOR_LOST` for the new child's exit (V1-1716, ruled out by
`TestATRV0095_StaleStartingChildAfterRelaunchKeepsOneResult`, which fails with `SUPERVISOR_LOST`
when the running-lock liveness check is removed).

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

### V1-1763 context delivery, hard-stop salvage and repeated hard stops (issue 1109)

Human-owned input: native ticket V1-1763, from GitHub issue 1109 (2026-10-18), a follow-up to issue
1007 and V1-1549 (ATR-V0-030..039). The context record and notice exist, but a worker that never calls
a dispatcher verb never sees them. The hard stop discards the session's last reasoning. A role that
cannot finish a ticket within hardTokens is stopped and relaunched without limit. Intent: push the
warn and hand-off notices into the running session through the host's own hook mechanism, and report
where that is impossible. Salvage a bounded, secret-screened hand-off note at the hard stop that the
next attempt on the ticket can read. Mark a ticket and role context-exhausted, with an alert, after
repeated hard stops.

- `ATR-V0-101`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1763) `corvint-tasks dispatch context-hook` MUST read the context record named by `CORVINT_DISPATCH_CONTEXT_FILE` (ATR-V0-034). It MUST also read at most 64 KiB of hook payload on stdin, whose `hook_event_name` (default `PostToolUse`) it echoes. For each `WARN` or `HANDOFF` crossing in the record that has no delivery marker, it MUST create the marker `context-delivered-<LEVEL>` exclusively in the record's directory. When it created at least one marker, it MUST write exactly one Claude Code hook object `{"hookSpecificOutput":{"hookEventName":...,"additionalContext":...}}` to stdout. That context names each newly delivered level, the estimate, the threshold and the expected next step. Otherwise it writes nothing to stdout. Each level is therefore delivered at most once per worker. `HARD` is never delivered. The verb MUST exit 0 so that it never blocks the session. A missing variable or an unreadable or newer-version record MUST write nothing to stdout and one line to stderr. A marker that cannot be created MUST stop delivery with one line on stderr; the levels this call already marked are still written. A marker created but not fully written MUST be removed so the next run delivers that level; when it cannot be removed, its notice is written in this run. The verb never writes the context record.
- `ATR-V0-102`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1763) A host's argv or env MAY use the placeholder `{contextHookSettings}`, for example `--settings {contextHookSettings}`. A role on such a host MUST set `usageFormat` `claude-code`, or the configuration is refused. Before each launch on that host, the dispatcher MUST write `context-hook-settings.json` into the worker's directory. This Claude Code settings file registers one `PostToolUse` command hook (matcher `*`) that runs the dispatcher's own executable as `dispatch context-hook`, and the placeholder renders to its path. A settings file that cannot be written fails the launch before spawn, like any other launch failure.
- `ATR-V0-103`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1763) Each `dispatch status` worker row of a role with a context-reporting `usageFormat` MUST carry `contextDelivery` with a `mode` and a `delivered` list. `mode` is `HOOK` for a claude-code role whose host uses `{contextHookSettings}`. It is `UNKNOWN` for a claude-code role whose host does not, because an operator-registered hook may or may not exist. It is `UNSUPPORTED` for `opencode`, which offers no shell hook surface in V0. `delivered` lists the levels whose delivery markers exist. The member is absent for any other role.
- `ATR-V0-104`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1763) Every launch MUST name the worker's evidence directory `<worker directory>/evidence`, created before spawn, as `CORVINT_DISPATCH_EVIDENCE_DIR`. When a worker stopped for `CONTEXT` (ATR-V0-035) finishes, the dispatcher MUST write `handoff-note.json` in its worker directory, in the closed profile `taskman-dispatch-handoff-note/0`. The note holds the worker, ticket, role, time and context estimate. It holds at most the last 3 main-session assistant texts found in the last 256 KiB of the worker's stdout. It also holds at most 8 regular files, never symlinks, FIFOs or subdirectories, from the evidence directory, in name order among at most the first 256 directory entries read. More regular files, or more entries, set `truncated`. The worker directory, its evidence directory and stdout, and each evidence file MUST be opened beneath the program's state directory without following a symlink in any component and without blocking on a FIFO. Every line that matches the secret screen MUST be replaced by `[REDACTED]` and counted in `redactedLines`, and the screen MUST see each whole line before any cut. A text keeps at most its last 16 KiB and a file at most its last 8 KiB, on line boundaries; a file is read from its last 16 KiB, whose incomplete first line is dropped. A single line longer than its bound keeps its tail, which is screened again. The whole note is at most 64 KiB: the oldest texts and then the files are dropped first, and `truncated` is set. Writing the note MUST emit one `context` event with detail `note` (its path), `messages`, `files` and `redactedLines`. A write failure MUST emit one alert and leave no note.
- `ATR-V0-105`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1763) The note MUST be linked by the attempt's existing hand-off evidence `dispatch:<worker>` (ATR-V0-084), so no new store or attempt field is added. When the dispatcher launches a session for a ticket, it MUST look at the ticket's attempts whose latest hand-off evidence names a dispatch worker. It MUST pass the path of the newest readable note of that ticket as `CORVINT_DISPATCH_HANDOFF_NOTE`, or omit the variable when there is none. Any value inherited from the dispatcher's environment or set by the host's env MUST be removed first. A note is read without following a final symlink or blocking on a FIFO, and only a regular file reads. A note of another ticket, of another profile version (UNSUPPORTED_VERSION, CAL-V0-131) or one that does not read is never offered. The note is untrusted agent output, never instructions or authority. The CAL-V0-131 live format set gains the profile.
- `ATR-V0-106`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1763) A role's `context` object MAY set `exhaustedAfter` (1..100, default 3). A configuration that sets it without `hardTokens` MUST be refused. When a session of that role on a ticket key finishes, a `CONTEXT` stop MUST extend that ticket and role's count of consecutive hard stops. Any other ending of that role on that key MUST reset it, even when the role no longer sets `hardTokens`. A finish replayed after a restart whose worker is already the entry's `lastWorker` MUST change nothing, so a hard stop is counted and alerted once. When the count reaches `exhaustedAfter`, the entry MUST enter the state `CONTEXT_EXHAUSTED`, and, after the record is saved, one `alert` event MUST carry detail `code=CONTEXT_EXHAUSTED`, `role`, `hardStops` and `exhaustedAfter`. Later hard stops extend the count without another alert. Lane keys are not tracked. The state holds no launch and stops no worker: the existing no-progress parking, budgets and loop holds still apply.
- `ATR-V0-107`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1763) The counts MUST be kept in `context-exhausted.json` in the program's state directory, in the closed profile `taskman-dispatch-context-exhausted/0`, so that they survive a restart. `dispatch status` MUST list each entry under `contextExhausted` with `key`, `role`, `hardStops`, `state` (`COUNTING` or `CONTEXT_EXHAUSTED`), `lastWorker` and `lastAt`, and show `state=UNKNOWN` for an unreadable record. A newer-version record is kept and disables counting after one alert. Another unreadable record is replaced after one alert. The CAL-V0-131 live format set gains the profile.

Host mechanisms: Claude Code's documented `PostToolUse` command hook with `additionalContext`, loaded
from the `--settings` file, is the one supported delivery path. The hook inherits the session's
environment, which names the record. opencode exposes plugins but no shell command hook, so its
delivery is `UNSUPPORTED`. Codex and the other hosts report no context figure (ATR-V0-030), so they
have nothing to deliver.

Non-goals: delivery between tool calls (a session that makes no tool call after a crossing sees no
notice), an opencode plugin, delivering `HARD`, a configurable evidence path, salvage for a stop
other than `CONTEXT`, a launch hold for a context-exhausted ticket, and carrying a note across a
candidate hand-off, which records no evidence reference.

Failure modes: the marker is created before the notice is written, so a hook that dies in between
loses that level's notice, and it is never repeated. A worker directory retired by the
worker-directory retention (CAL-V0-144) loses its note, and the next launch then gets no
`CORVINT_DISPATCH_HANDOFF_NOTE`. The secret screen also replaces the lines of a secret wrapped
across lines (ATR-V0-169), within its stated bound. The streak counts only dispatcher sessions, and a dispatcher restart keeps it through
the record.

Acceptance evidence: `TestATRV0101_WorkerWithoutDispatcherVerbReceivesNoticesThroughHook`,
`TestATRV0101_ContextHookFailsQuietly`, `TestATRV0103_StatusReportsContextDelivery` and
`TestATRV0107_DispatchStatusShowsContextExhausted` (`internal/tasks/cli`);
`TestATRV0102_LaunchWritesHookSettingsAndPlaceholder`, `TestATRV0102_PlaceholderNeedsClaudeCode`,
`TestATRV0104_HardStopSalvagesBoundedScreenedNote`,
`TestATRV0105_NextAttemptReadsSalvagedNote`, `TestATRV0106_ExhaustedAfterNHardStopsNotNMinusOne`,
`TestATRV0106_ExhaustedAfterValidation` and `TestATRV0107_RecordVersions` (`internal/tasks/dispatch`);
and the CAL-V0-131 rows of `format_version_test.go`. Live host qualification is NOT_RUN
(cross-reference V1-1563).

Rollback: revert the code and this amendment. First remove `{contextHookSettings}` and
`exhaustedAfter` from any configuration, because an earlier build refuses them. Leftover notes,
markers, settings files and the record are inert for an earlier build.

### V1-1782 no-progress carry-over (issue 1122)

Human-owned input: native ticket V1-1782, from GitHub issue 1122 (2026-10-18). In one program 13 of
16 no-progress sessions ended at the context hand-off before any edit, and three consecutive sessions
on one ticket repeated the same plan until the stall hold parked it, because each session started
cold. Intent: carry a bounded tail of a no-progress session's final message, with its end reason,
into the next session's prompt on the same ticket, labelled as untrusted prior-session output,
configurable per role, and visible in dispatch events so that progress with and without carry-over
can be compared. ATR-V0-104's hand-off note covers only the hard stop and is passed by path; this
carry-over covers every no-progress ending and is written into the prompt.

- `ATR-V0-110`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1782) A ticket role MAY set `carryOver` with `enabled` (default `true`), `maxBytes` (512..16384, default 4096) and `sessions` (1..5, default 1). A role without `carryOver`, or with `enabled` false, neither records nor receives carry-over. A configuration MUST be refused when a value is out of range, an explicit `0` included (an absent member takes its default), when a lane role sets `carryOver`, or when an enabled role's host does not pass `{prompt}` as one whole argv element, splices `{prompt}` into another argv element or an activity path, or runs an interpreter or takes a code-string option, as `noteSafeHost` refuses for `{operatorNote}`. Other placeholders in the host argv are allowed as ATR-V0-133 bounds them.
- `ATR-V0-111`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1782) When a session of an enabled role on a ticket key finishes as a no-progress session (CAL-V0-057) and leaves no detached run running, the dispatcher MUST append one entry to the ticket's carry-over record. The entry holds the worker, role, finish time, end reason and final message. The end reason is `HARD_STOP` for a `CONTEXT` stop, the kill reason (`WALL`, `IDLE`, `ORPHANED` or `LEASE_EXPIRED`) for another stop, `HANDOFF_THRESHOLD` when the worker's context record crossed `HANDOFF`, `TOOL_CALL_CAP` when the last result event in its stdout has subtype `error_max_turns`, and `EXITED` otherwise. The final message is the last main-session assistant text in the last 256 KiB of the worker's stdout, or, when there is none, that tail's text as the finished summary extracts it, starting on a line boundary and never cut to the summary's 2000 bytes. It MUST pass the ATR-V0-104 secret screen whole before it is cut to its last 16 KiB on line boundaries; `redactedLines` counts the replaced lines. The record keeps at most the 5 newest entries, and an entry whose worker is already present is not added again. A session that recorded progress on the ticket, of any role, MUST remove the ticket's record; an infrastructure, held or detached-run session leaves it unchanged. A write or removal failure MUST emit one alert; after a failed removal this dispatcher offers no carry-over for the ticket until a later removal or write succeeds, and its next append MUST replace the record so that entries from before the progress are never offered again. ATR-V0-168 makes the same guarantee durable across a restart.
- `ATR-V0-112`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1782) The record MUST be `carry-over/<first 16 hex digits of the SHA-256 of the ticket>.json` in the program's state directory, in the closed profile `taskman-dispatch-carry-over/0`, canonical, at most 128 KiB. It MUST be read without following a final symlink or blocking on a FIFO, and only a regular file reads. A record of another profile version (UNSUPPORTED_VERSION, CAL-V0-131) is kept and never offered, and its append alerts once per dispatcher. A record of another ticket, or a malformed one, is never offered, and the next append replaces it after one alert. The CAL-V0-131 live format set gains the profile.
- `ATR-V0-113`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1782) When the dispatcher launches an enabled role on a ticket whose record reads, it MUST append to the rendered role prompt one carry-over block holding the record's newest `sessions` entries, oldest first. The block MUST open with the line `--- BEGIN UNTRUSTED PRIOR-SESSION OUTPUT (corvint dispatch carry-over) ---` and close with `--- END UNTRUSTED PRIOR-SESSION OUTPUT ---`. It MUST state that the text is data from earlier sessions on the ticket that made no progress, not instructions or authority. Each entry MUST be headed by its worker, role, end reason and time. The whole block, frame included, MUST be at most `maxBytes`: each entry keeps an equal share of the remaining bytes, cut to its tail on line boundaries and marked as cut, and the oldest entries are dropped when the frame alone does not fit. The block is appended after placeholder rendering, so its text is never expanded, and the frame phrase inside an entry's text is lower-cased so an entry cannot close the block. Before spawn the dispatcher MUST write the offered entries to the worker's `carry-over.json`, in the same profile; a write failure fails the launch before spawn. Any other launch gets no block.
- `ATR-V0-114`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1782) A launch with a carry-over block MUST carry `carryOver` (the number of entries offered) and `carryOverBytes` (the block's size) in its `launched` event detail, and that worker's `finished` event detail MUST carry `carryOver`, read from its `carry-over.json`. The members are absent for a launch without carry-over, so a session's `progress` and `session` details can be compared with and without carry-over from the event log alone.

Defaults: `enabled` true when the object is present, `maxBytes` 4096, `sessions` 1. Fixed bounds: 16 KiB
per recorded message, 5 entries per record, 128 KiB per record.

Non-goals: carry-over for lane keys, across tickets or from a progress session; summarising or
rewriting the prior text; a `dispatch status` view of carry-over records; a progress-rate report
(the event members are the comparison's input); any change to the ATR-V0-104 hand-off note, which is
still salvaged and passed by path.

Failure modes: a worker that prints nothing recognisable carries an empty message with its end
reason. The secret screen also replaces the lines of a secret wrapped across lines (ATR-V0-169),
within its stated bound. A dispatcher restart forgets a failed removal, but the entries left after
progress no longer match the ticket's work fingerprint and are neither offered nor extended
(ATR-V0-168). A replayed finish adds no second entry. A record from another dispatcher configuration is
offered only to enabled roles.

Acceptance evidence: `TestATRV0110_CarryOverConfigDefaultsAndBounds`,
`TestATRV0111_NoProgressSessionRecordsEndReasonAndTail`, `TestATRV0111_ProgressRemovesRecord`,
`TestATRV0111_PlainOutputScreenedBeforeCut`, `TestATRV0111_AppendAfterFailedRemovalDropsOlderEntries`,
`TestATRV0112_RecordVersions`, `TestATRV0113_NextSessionPromptCarriesBoundedLabelledBlock`,
`TestATRV0113_NoCarryOverForOtherTicket`, `TestATRV0113_BlockBoundAcrossSessions` and
`TestATRV0114_EventsMarkCarryOverSessions`
(`internal/tasks/dispatch`), and the CAL-V0-131 row of `format_version_test.go`. Live host
qualification is NOT_RUN.

Rollback: revert the code and this amendment. First remove `carryOver` from every role, because an
earlier build refuses it. Leftover records and worker markers are inert for an earlier build.

### V1-1833 dispatcher-held pool members (issue 1147)

Human-owned input: native ticket V1-1833, from GitHub issue 1147 (2026-10-18). A dispatched session
on a ticket that requires a pool claimed its member itself; when the session was killed mid-run the
member stayed allocated until the attempt's lease expired, and `dispatch status` could not say which
worker held which member. Intent: for a role that opts in, the dispatcher takes the member before
launch, hands it to the session, shows it per worker, and returns it when the attempt's worker ends,
whatever the session did, without changing the native waitlist order.

- `ATR-V0-120`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1833) A role with `match.pool` MAY set `holdPool: true`; a configuration that sets it
  on a role without `match.pool` MUST be refused. Before it spawns a worker of such a role on a ticket
  that requires the pool, a dispatcher whose queue has the pool-hold capability MUST claim the ticket
  natively, as `claim TICKET --holder WORKER --pool POOL --request-id ID` with the assignment's
  `--stage` and the role's `keepLease.leaseMinutes` when set, so the store allocates the member under
  its own admission, room and waitlist rules (PSR-V0-130..136, PSR-V0-140..143). A refused claim MUST
  launch nothing, MUST emit one `pool-hold` event naming the refusal per ticket and distinct refusal,
  and is retried under a fresh request ID on a later tick. The dispatcher never names a member. A role
  without `holdPool`, a lane key, or a queue without the capability is unchanged.
- `ATR-V0-121`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1833) After a successful claim and before spawn, the dispatcher MUST write the worker's
  hold record `workers/<worker>/pool-hold.json` in the closed profile `taskman-dispatch-pool-hold/0`
  (worker, state `HELD`, attempt, generation, ticket, pool, member, allocation and acquire time), and
  MUST name it to the session as `CORVINT_DISPATCH_POOL_FILE`, replacing any inherited value; a role
  without a hold gets no such variable. A record that cannot be written fails the launch before spawn
  and the claim is returned as ATR-V0-122 states. The CAL-V0-131 live format set gains the profile.
- `ATR-V0-122`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1833) When a worker with a `HELD` record ends, by exit or kill, the dispatcher MUST,
  before any hand-off of its attempt, return the exact allocation with the native `pool release`
  (CAL-V0-201) and record state `RELEASED`; the attempt itself is then handed off as before, or left
  to its lease when hand-off is off. When the observed attempt has already ended, the member was
  quarantined natively (CAL-V0-202): the record becomes `ATTEMPT_ENDED` and nothing is written to the
  store. A launch that fails before spawn MUST return the allocation and release the attempt. A
  refused return is reported in a `pool-hold` event and the attempt's end quarantines the member.
- `ATR-V0-123`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1833) `dispatch status` MUST show, for each worker with a hold record, a `poolHold`
  object with its state, attempt, generation, pool, member, allocation and acquire time.
- `ATR-V0-124`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1833) On each heal the dispatcher MUST return the allocation of, and release, a live
  attempt that holds a pool member, whose holder is not a recorded worker, and whose `HELD` record in
  this program names that attempt, such as a claim whose spawn or ledger write a crash interrupted.
  An attempt held by a recorded worker is never swept.

Non-goals: dispatcher-held members for lane roles or for sessions that claim on their own; choosing
or reserving a member ahead of the native order; per-member health or reuse policy; the
dispatcher-side pool sweep (V1-1573) and pool hosting on other machines (V1-0800).

Failure modes: a dispatcher that dies between the claim and the hold write leaves a live attempt
with no record; its lease expiry ends it and quarantines the member (CAL-V0-202). A spawn that may
have started (an unidentified tree) keeps the hold until the ATR-V0-124 sweep returns it. A store
that refuses `pool release` leaves the member to the attempt's end. An earlier build refuses a
configuration with `holdPool`.

Acceptance evidence: `TestATRV0122_KilledSessionLeavesNoOrphanedAllocation`,
`TestATRV0122_EndedAttemptIsNotReleasedAgain`, `TestATRV0122_FailedLaunchReturnsTheClaim`,
`TestATRV0124_RefusedClaimKeepsNativeOrder`, `TestATRV0124_OrphanedClaimIsReturned` and
`TestATRV0120_HoldPoolIsOptIn` (`internal/tasks/dispatch`); `TestATRV0124_PoolClaimKeepsWaiterOrderCLI`
(a real store: a ticket waiting on the pool keeps its place and the other ticket's claim is refused
naming it) and `TestATRV0123_DispatchStatusShowsPoolHold` (`internal/tasks/cli`); and the CAL-V0-131
row of `format_version_test.go`. Live dispatcher qualification and Linux are NOT_RUN.

Rollback: remove `holdPool` from every role, then revert the code and this amendment. Leftover hold
records are inert for an earlier build; members still allocated end with their attempts.

### V1-1834 host-neutral context notices and on-demand salvage (issue 1148)

Human-owned input: native ticket V1-1834, from GitHub issue 1148 (2026-10-18). Context warnings
reached only claude-code sessions through the PostToolUse hook (ATR-V0-101); every other host showed
`UNSUPPORTED` and its sessions ran into the hard stop unwarned. A note could be salvaged only at the
dispatcher's own hard stop (ATR-V0-104), so a session killed by anything else left none, and the
events did not say whether either had happened. Intent: one delivery path any host can poll, an
operator verb that salvages the note of a named attempt, and events that record both.

- `ATR-V0-125`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1834) A host MAY
  declare `contextNotices: "file"`; any other non-empty value MUST be refused. A worker of a role whose
  `usageFormat` carries context estimates (`claude-code` or `opencode`) on such a host MUST be launched with `CORVINT_DISPATCH_CONTEXT_NOTICE_FILE` naming
  `workers/<worker>/context-notices.txt`, replacing any inherited value; no other worker gets it. On
  each tick the dispatcher MUST append, opened beneath the worker directory, one notice line per WARN
  or HANDOFF crossing without a delivery marker, and only after the append succeeds create that
  level's `context-delivered-<LEVEL>` marker (ATR-V0-101) and emit one `context` event with delivery
  `FILE`. A failed append or a non-regular notice file marks nothing, alerts once while it keeps
  failing, and is retried on the next tick. A level the hook marked first is never repeated. For such
  a host the context delivery mode of `dispatch status` (ATR-V0-103) MUST be `FILE`, never
  `UNSUPPORTED`; a claude-code role with the hook placeholder stays `HOOK`.
- `ATR-V0-126`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1834)
  `dispatch salvage --program ID --config FILE --attempt A` MUST read attempt A from the native store
  of the configuration's `workRoot`, take its lease holder as a worker of the program, running in the
  ledger or stopped with its worker directory still present, and write that worker's
  `handoff-note.json` with the bounds and secret screen of ATR-V0-104, naming the attempt's ticket. It
  MUST NOT change the attempt or the store, and MUST refuse a missing `--attempt`, a holder that names
  no worker of the program, and a `workRoot` of another store. The result, profile
  `taskman-dispatch-salvage/0`, reports the attempt and its phase, the worker and its state, the note
  path and its message, file and redacted-line counts. The note is offered under ATR-V0-105.
- `ATR-V0-127`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1834) The
  `launched` event MUST record `contextDelivery` for a role whose `usageFormat` carries context estimates and `handoffNote`
  (the offered note's path, or `NONE`). The worker-stop event, which is `finished` (the dispatcher's
  own `stopped` event names no worker), MUST record `contextDelivery`, `contextDelivered` (the
  delivered levels comma-joined, or `NONE`) and `salvage`: `HARD_STOP` for a context kill, `ON_DEMAND`
  when the worker's own note exists otherwise, else `NONE`.

Non-goals: hook delivery for hosts other than claude-code; parsing a non-Claude host's usage stream
(V1-1768); proof that a session read the notice file; salvage of a worker whose directory was
retired; the dispatcher-side pool sweep (V1-1573); V1-1792 and V1-0800.

Failure modes: a session that never polls the file still runs to the hard stop, which is unchanged.
Delivery is at least once: a crash between the append and the marker repeats that line on the next
tick. A salvage that runs while the session still writes captures the log as of that read. An
earlier build refuses a configuration with `contextNotices` and does not know the verb.

Acceptance evidence: `TestATRV0125_NoticeFileDeliversOnAnyHost` (both levels delivered once to the
file, `FILE` for claude-code and opencode formats, `UNSUPPORTED` without the declaration, an
on-demand salvage of the running session with the secret redacted, and an externally killed
session whose `finished` event records `FILE`, `WARN,HANDOFF` and `ON_DEMAND`),
`TestATRV0125_NoticeFileValidationAndRetry` and `TestATRV0127_FinishedRecordsSalvage`
(`internal/tasks/dispatch`); `TestATRV0126_SalvageAttemptWritesScreenedNote` (a real store: a
running attempt whose session stopped, the same attempt after a dispatcher hand-off, a foreign
holder and a missing `--attempt`) and the CAL-V0-131 output-only row (`internal/tasks/cli`). Live
host qualification and Linux are NOT_RUN.

Rollback: remove `contextNotices` from every host, then revert the code and this amendment. Notice
files, markers and salvaged notes are inert for an earlier build.

### V1-1852 carry-over host argv and invalid config files (issue 1157)

Human-owned input: native ticket V1-1852, from GitHub issue 1157 (2026-10-18). A host whose argv
names the session (`--title "lane {role} {ticketLocal}"`) could not enable `carryOver`, because the
build refused every store-derived placeholder in its argv although ATR-V0-110 allowed other
placeholders. An edit that made the file invalid was reported by `dispatch status` with exit 0, and
the next restart then refused the file. Intent: allow ordinary argv placeholders, refuse only shapes
that can run the carried text or a store value as code and name them, let a script detect an invalid
file from the exit code, and say what a restart on an invalid file does.

Why the restriction existed: the carry-over block is untrusted prior-session output placed in
`{prompt}`, so V1-1782 reused the `noteSafeHost` rule (ON-V0-011): the host must not run an
interpreter or take a code-string option. That check sees only the literal configured argv. A
store-derived value (`{ticket}`, `{ticketLocal}`, `{state}`, `{pool}`, `{member}`, `{nextStage}`)
is a token, but it can still be `sh` or `python3`, or turn `-{ticketLocal}` into `-c`, and the
configuration check cannot see it. V1-1782 therefore refused those placeholders outright.

- `ATR-V0-133`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1852) With
  `carryOver` enabled, an argv element after `argv[0]` MAY hold any argv placeholder, the
  store-derived ones included, alone or inside literal text. The configuration MUST still be refused,
  naming the shape, when `{prompt}` is spliced into another element (`splices {prompt} into argv
  element N`), when no element is a whole `{prompt}` (`never passes {prompt} as a whole argv
  element`), when `argv[0]` holds any placeholder (`argv element 0 holds the placeholder P`), when an
  element's literal text, with its placeholders removed, names an interpreter (`runs the interpreter
  X`) or is a code-string option (`takes the code-string option X`), so `-c{ticketLocal}` is refused
  and `-t{ticketLocal}` is not, or when an activity path holds `{prompt}`. `{operatorNote}` and
  `{detachedRun}` stay refused in argv, and a role using `{operatorNote}` keeps the literal-argv rule
  of ON-V0-011. At each launch that would offer a carry-over block, the dispatcher MUST apply the
  interpreter and code-string checks to every placeholder-bearing element after `argv[0]` as
  rendered for that launch; when one fails, the launch proceeds without the block, its `launched`
  event carries no `carryOver`, and one `alert` event names the host and the element.
- `ATR-V0-134`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1852) This extends
  CAL-V0-127's status view. When the `--config` file read by `dispatch status` does not decode or
  validate but still names a state directory (its `configFile.state` is `INVALID`), the command MUST
  still return the status item, and MUST finish with outcome `ERROR`, code `MALFORMED` and exit status 1,
  with a warning that names the refusal reason. A file that validates keeps outcome `OK`; an unreadable file, or one that names no state
  directory, is refused as before. A running
  dispatcher's handling of the file (CAL-V0-127) is unchanged.
- `ATR-V0-135`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1852) A dispatcher
  started on an invalid file MUST refuse before it reads or writes the ledger: exit status 1, outcome
  `ERROR`, the reason in the warning. It does not fall back to the last applied configuration, because
  the ledger records only that configuration's digest and time, not its bytes. A restart on an
  invalid file therefore leaves the program with no dispatcher until the file is fixed; ATR-V0-134
  is the check that predicts it.

Non-goals: a `dispatch validate` verb (status already reads and validates the file); persisting the
applied configuration's bytes for a restart fallback; detecting a program that evaluates a plain
argument as code without an interpreter name or code option (ATR-V0-164 states that limit and the
operator's part); a check of `argv[0]` against a rendered value, since `argv[0]` holds no
placeholder.

Failure modes: the interpreter list and code-option rule are per element and name-based, so a host
program that itself evaluates an argument is not detected, as before (ATR-V0-164). While a ticket's
rendered argv stays unsafe, every launch on it offers no block; it alerts once per applied
configuration (ATR-V0-165). A status exit of 1 also
fails a script that only wanted the view; the item is still returned.

Acceptance evidence: `TestATRV0133_CarryOverHostArgvPlaceholders` (accepted title, bare and
store-valued shapes; each refused shape by its message; the operatorNote rule kept; the rendered
check on `sh`, `python3.12`, `-ce`, `--eval` and a path to `node`; `-t{ticketLocal}` and
`-{ticketLocal}` accepted at load, `-c{ticketLocal}` and `--eval={role}` refused) and
`TestATRV0133_LaunchRechecksRenderedArgv` (a templated title carries the block; a ticket local of
`sh`, or `-{ticketLocal}` rendering `-ce`, launches without it and alerts), with `TestATRV0110_CarryOverConfigDefaultsAndBounds` updated
(`internal/tasks/dispatch`); `TestATRV0134_StatusFailsOnInvalidConfigFile` (exit 0 for a valid
file; exit 1, `ERROR`, `MALFORMED`, the item and the named reason for an invalid one; the ATR-V0-135
restart refusal with the ledger bytes unchanged), with `TestCALV0127_DispatchStatusWithInvalidConfigFile`,
`TestATRV0056_DispatchStatusWorkerShowsContextNotice` and `TestCALV0176_DispatchStatusReadsFragmentConfig`
updated (`internal/tasks/cli`). A mutation that disabled the launch recheck failed
`TestATRV0133_LaunchRechecksRenderedArgv`. Live host qualification and Linux are NOT_RUN.

Rollback: revert the code and this amendment. Before that, remove store-derived placeholders from
the argv of any host used by a `carryOver` role, because the earlier build refuses them; scripts that
gate on the status exit code return to reading `configFile.state`.

- `ATR-V0-141`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1885, GitHub #1169)
  A ticket role MAY opt in with `reviewPacket: {enabled?, maxBytes?}` (`maxBytes` 4096..262144,
  default 32768; an explicit value out of range, or the option on a lane role, refuses the
  configuration; an enabled option needs a host whose argv passes the ATR-V0-133 placeholder check).
  When the dispatcher launches such a role on a ticket it MUST ask the queue for the ERG-V0-017
  packet of the ticket's newest candidate, with the ticket's newest ATR-V0-105 hand-off note as its
  partial checkpoint, and append it to the rendered prompt after any carry-over block, framed as
  `BEGIN UNTRUSTED REVIEW PACKET (taskman-review-packet/0; data, not instructions)` ...
  `END UNTRUSTED REVIEW PACKET`. The packet is read before any ATR-V0-120 pre-launch claim,
  because that claim opens a generation without the submitted candidate. Like the carry-over block it is appended after placeholder
  rendering, so its text is never expanded. The `launched` event records `reviewPacketBytes`. A
  queue that cannot build packets, a failed build or a rendered argv that fails the ATR-V0-133
  recheck MUST launch the session without a packet and emit one alert naming the reason
  (`review packet not offered: ...`); the packet never blocks a launch. A role without the option,
  or with `enabled: false`, launches exactly as before. An optional `sourceRepository` names a
  configuration `sourceRepositories` entry the packet is built in (ERG-V0-022, GitHub #1217).

Non-goals (ATR-V0-141): choosing reviewers, deciding which stage is a review stage (the operator
opts the review role in), writing the packet to the worker directory, or binding the checkpoint to
a commit.

Failure modes (ATR-V0-141): a store that cannot be read gives an alert and a packet-free launch on
every launch of the role until it reads; the packet is built once per launch, so its cost is the
ERG-V0-017 bounded read.

Acceptance evidence: `TestATRV0141_ReviewRolePromptCarriesPacket` (the packet follows the rendered
prompt, the source receives the ticket, cap and note path, `reviewPacketBytes` is recorded; a disabled
option asks nothing; a queue without the capability and a failed build launch without a packet and
alert) and `TestATRV0141_ReviewPacketConfigBounds` (0, 4095 and 262145 refused, 4096 accepted, a lane
role refused) and `TestATRV0141_PacketReadBeforePoolClaim` (a holdPool review role reads the packet
before its pre-launch claim) (`internal/tasks/dispatch`); the queue side through `TestERGV0017_ReviewPacketAcrossRounds`
(`internal/tasks/cli`). Live host qualification is NOT_RUN.

Rollback: remove `reviewPacket` from every role first (the earlier closed decoder refuses it), then
revert the code and this requirement; nothing is stored.

### V1-1914 and V1-1916 queue observation, run-only bypass and calm-pressure cap (issues 1181 and 1183)

Human-owned input: native tickets V1-1914 and V1-1916, from GitHub issues 1181 and 1183 (2026-10-19).
A run-only stage, such as a review or test lane that needs only a pool member and no author session,
waited behind the plan window while pool members were free. A role's queue also stayed many tickets
deep at its cap while host pressure was calm. Neither was visible without reading the ledger by hand.
Intent: record, each tick, which rule kept such work out and how deep each role's queue is, so the
doctor can name both (TQD-V0-043, TQD-V0-044). Let an operator opt a run-only role past the plan
window while its pool has a free member, and opt a role into a higher cap only while pressure is
calm. Both policies are off unless configured.

- `ATR-V0-150`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1914 and V1-1916)
  After each tick's launch roster the dispatcher MUST rewrite `<programDir>/admission.json` atomically
  with one `taskman-dispatch-admission/0` object of at most 1 MiB holding `program`, `at`,
  `tickSeconds`, `pressure`, `launchesHeld`, `globalCap`, `running`, `roles`, `held` and
  `heldTruncated`. `pressure` is `NOT_CONFIGURED` (no pressure block or no recorded pressure state),
  `UNKNOWN` (the CAL-V0-068 state is unknown), `CALM` (level and pending level 0) or `RAISED`.
  `roles` holds, for each matching role with a cap, `role`, `cap`, any `capMax`, `effectiveCap`,
  `running` (the workers recorded after this tick's launch loop) and `waiting` (candidates neither
  running nor backing off; a planned launch that control admission, a pool claim or the spawn
  refused is still waiting).
  While pressure is `CALM` and `running + waiting` exceeds `effectiveCap`, it also holds the
  consecutive-tick streak `deepTicks` and `deepSince`. `oldestWaitTicket` and `oldestWaitSince` name
  the waiting ticket seen longest. `held` covers each ticket of a `runOnly` role that neither runs,
  launched nor waits out a backoff. It names the first fence that kept the ticket out: `PLAN_WINDOW`
  with the native plan reason, else in roster order `GLOBAL_CAP`, `ROLE_CAP`, `TIER_CAP`, `BUDGET`
  or `PRESSURE`. It also names the ticket's pool, that pool's free-member count, up to 64 sorted
  free-member names, and the streak `ticks` and `since`. A hold is recorded only while the pool has
  at least one free member. `held` keeps at most 256 entries, longest streak first, and sets
  `heldTruncated` when more were held or when `held` was halved, longest streaks kept, until the
  object fits 1 MiB. Streaks live in the dispatcher's memory and restart with it.
  A tick whose launches are held before the roster (CAL-V0-273, or an unreadable pressure source)
  writes `launchesHeld` true with empty `roles` and `held`, and restarts every streak. A failed
  write MUST NOT stop the tick; it emits one `alert` until a write succeeds again. The file is a
  derived observation, never authority, ranking or evidence, and no dispatcher input reads it. A
  reader MUST refuse another profile as `UNSUPPORTED_VERSION`, and an unknown member, another
  program, a value outside these bounds or trailing data as `MALFORMED`.
- `ATR-V0-151`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1914) A role MAY
  declare `runOnly`, which needs `match.pool`: its sessions only run work on a member of that pool
  and need no author session. Only the tickets of such roles are recorded in `held`. A `runOnly`
  role MAY also declare `bypassFocus`, which needs `runOnly` and is off by default. With it, a ticket
  the role would match but for `match.planSelected` (the native plan has not selected it) is a
  candidate while that pool has at least one free member. Otherwise the ticket is recorded as held
  by `PLAN_WINDOW`. A relaunch of such a ticket's next stage keeps the bypassing role. The bypass
  lifts only the plan-window match. Every cap, budget, pool-room and pressure fence still applies,
  and so does native claim admission. A role without `runOnly` never bypasses, so author-session
  work keeps the plan window.
- `ATR-V0-152`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1916) A matching
  role with `cap` at least 1 MAY declare `capMax`, with `cap < capMax <= 64`; it is off by default.
  While the tick's pressure is `CALM` and the role's queue depth exceeds `cap`, the role's effective
  cap for that tick is `min(capMax, depth)`. The depth is its running sessions plus its waiting
  candidates before this tick's launches. Otherwise the effective cap is `cap`, including at
  `RAISED`, `UNKNOWN` and `NOT_CONFIGURED` pressure. The global cap, tier caps, budgets and the
  pressure budget still apply. A lower effective cap never stops a running session; it only admits
  fewer new launches, so the role returns to `cap` as its queue drains.

Non-goals: dispatching run-only work without a free pool member; scaling any cap when pressure is not
observed as calm; persisting streaks across a dispatcher restart; a new ledger version (the
observation is a separate derived file); changing `cap` or `globalCap` automatically.

Failure modes: a restart restarts every streak, so the doctor's findings pause until their
thresholds are met again. The free-member count comes from the tick's pool observation. If another
program takes a member between the observation and the claim, a bypassed launch waits on the native
claim as before. `capMax` raises launches only within `globalCap`, so a full global cap still holds
the queue, and the doctor names that headroom. A failed write leaves the previous observation in
place, and the doctor reports it as stale after three ticks.

Acceptance evidence: in `internal/tasks/dispatch`, `TestATRV0150_RosterNamesFences` (each fence
recorded for a run-only ticket; author-session tickets at their role cap never recorded),
`TestATRV0150_ObservationStreaks`, `TestATRV0150_RefusedLaunchStaysWaiting` (a refused planned
launch stays waiting; running counts recorded workers), `TestATRV0150_OversizeHeldIsTruncated`,
`TestATRV0150_WriteFailureAlertsOnce`, `TestATRV0150_PressureLabel`,
`TestATRV0150_TickWritesObservation` and `TestATRV0150_DecodeAdmissionRefusals`.
`TestATRV0151_RunOnlyBypassFocus` covers a ticket held by the plan window with no free member or no
opt-in, dispatched with a free member and `bypassFocus`, author-session work that never bypasses,
and a relaunch that keeps the bypassing role; `TestATRV0151_ConfigValidation` covers the
configuration refusals. `TestATRV0152_CapMaxScalesAtCalmPressure` covers scaling up while deep at
`CALM`, bounded by depth and `capMax`, the return to `cap` as the queue drains, and no scaling when
pressure is not calm or no pressure input is given; `TestATRV0150_PressureLabel` maps `RAISED`,
`UNKNOWN` and `NOT_CONFIGURED` states to not calm. In `internal/tasks/cli`,
`TestCALV0131_EveryLiveFormatRefusesANewerVersion` and
`TestCALV0131_LiveFormatsCoverEveryDecodedProfile` cover the new profile. One mutation per behaviour
failed its test (build log `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1914-1916-doctor-held-idle-queue-deep.md`).
Live dispatcher qualification and Linux are NOT_RUN.

Rollback: remove `runOnly`, `bypassFocus` and `capMax` from every dispatcher configuration, because
an older build refuses them as unknown members. Then revert the code and this amendment.
`admission.json` may then be deleted; nothing else reads it.


### Group stop under a concurrent fork (ATR-V0-153)

Owner context: the scheduled release-gates run 39416257984 (macos-15, main at f3ca72248) failed
`TestATRV0004_InterruptStopsTheRun` with `CLEANUP_HOLD` after the 10-second bound (issue 1116;
the same symptom was V1-0768). The test stops `/bin/sh -c "touch F; sleep 60"` as soon as `F`
appears, which is when the shell forks `sleep`. Darwin's group SIGKILL can miss a child that a
fork running alongside it adds to the group. A local probe of 300 such stops left `sleep 60` alive
three times, still in the group and reparented to launchd, so the single `PGO-V0-001` signal could
never retire it. Hosted-runner load widens the window. A command that forks while it is interrupted,
times out or loses its lease meets the same race.

- `ATR-V0-153`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2103) The runner
  MUST start its owned group with `groupreap.Primitives.RepeatKillWhileLive`. With it, the Owner's
  `RequirePreReapQuiet` retirement repeats the group SIGKILL each time its quiet proof observes a
  live member, and only then. Every repeat is sent while the exited leader is held unreaped, so its
  PID pins the group ID, and none is sent once the reap has begun. A failed repeat HOLDs without a
  reap, as a failed first signal does. The 10-second `FinishBounded` bound, the post-reap signal-0
  absence proof and every other `ATR-V0-003` rule are unchanged. `PGO-V0-017` names the other
  Owner callers that set it and the one that keeps the single signal of `PGO-V0-001`.

Non-goals: a descendant that left the group (for example with `setsid`) stays outside the runner,
as before. A member that keeps forking faster than the repeats can still reach the bound and HOLD.
The other Owner callers are decided separately by `PGO-V0-017` (V1-2111): the dispatcher reader, the
CEM Git runner and the dogfood aggregate worker set the same opt-in, and the Linux service helper
retirement keeps the single signal.

Failure modes: a repeated signal that fails HOLDs (`SURVIVORS`), as before. A same-session process
that joins the group with `setpgid` before the reap is signalled by a repeat, where the single
signal might have missed it. The group is the runner's own, so this is an accepted consequence.
The same race can instead spare the leader while it forks and kill its new child. A shell leader
then exits 128+9 on its own for that child, and the outcome keeps that exit status rather than a
signal (`ATR-V0-004`). Under CPU load with `GOMAXPROCS=2` this happened once in 40 interrupts, and
the class is still `INTERRUPTED` with exit 128+15.

Acceptance evidence: in `internal/groupreap`, `TestOwnerRepeatKillWhileLiveSignalsOnlyBeforeReap`
(a repeat on each pre-reap live observation, none after the reap),
`TestOwnerRepeatKillFailureHoldsWithoutReaping`, and the real-process
`TestOwnerRepeatKillRetiresMemberForkedDuringStop` (150 stops of a shell as it forks, no live
survivor). With the repeat disabled the two fake-host tests fail; the real-process test is a
regression guard and does not reliably reproduce the race on an idle host. In `internal/tasks/cli`,
`TestATRV0004_InterruptStopsTheRun` at `-count=20` and under CPU load with `GOMAXPROCS=2`
(build log `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-v1-2103-darwin-fork-group-kill.md`). Linux NOT_RUN.

Rollback: revert the commit. The runner then sends one group signal again and the race returns as
an occasional `CLEANUP_HOLD`; no record, receipt or wire shape changes.

### Resumable run retirement (ATR-V0-154)

Owner context: V1-0934. An ATR-V0-015 pass read the first 1,024 attempt directories in directory
order. When those held no removable run (live attempts, empty attempt directories, runs not proven
ended), every pass read them again and the backlog behind them was never reached.

- `ATR-V0-154`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0934) An
  ATR-V0-015 pass MUST list at most 65,536 names of `taskman-runs/` (names only), sort them, and
  examine a window of at most 1,024 attempt directories: the attempts of the retained runs carried
  from the previous pass, then the names after the cursor's, in name order. A window that reaches
  the last name ends the sweep, and the next window starts from the first name. After a pass that
  read every terminal record it needed, it MUST replace the cursor `taskman-runs/.retire-cursor`
  in one rename. The cursor holds the last name the window examined and the attempt directories of
  the newest 16 ended runs it kept, which are carried into the next window. A run is retained only
  when it is among the newest 16 of its window and the carried attempts. The newest 16 of all are
  in every window's newest 16, so they are never removed. One sweep leaves at most 16 ended runs of
  terminal attempts among the names it reached. The window and the cursor check share one name
  rule: an attempt directory name, or a `.retired-` name, which the window passes over but may
  record as its last name. A cursor that does not read back under that rule, or that carries a
  name twice, is ignored, and the sweep starts over. Each writer writes the cursor through its
  own temporary file in `taskman-runs/` before the rename.

Bound: a pass examines at most 1,024 attempt directories and makes one terminal-record read, as
before, and a second only when it removes runs (ATR-V0-171). Listing up to 65,536 names reads directory entries only and stats none of them. A backlog
of n attempt directories is swept within ceil(n / (1,024 - 16)) passes, one per finished detached
run.

Non-goals: names beyond the first 65,536 a listing returns are not reached; directory order decides
which ones those are. Empty attempt directories are kept, as before. Concurrent supervisors may each
write the cursor; the last rename wins, and a window may be examined twice.

Failure modes: a cursor that cannot be written leaves the old one, so the next pass repeats the
window, as an ATR-V0-015 pass did. A failed terminal-record read writes no cursor and repeats the
window. An attempt removed or added between passes shifts nothing: the cursor is a name, not an
offset.

Acceptance evidence: `TestATRV0154_RetirementSweepsABacklogBeyondOneWindow` (`internal/tasks/cli`)
lowers the window to 6 and the retention to 2. It plants 30 attempts with one quiet finished run
each, and 10 empty attempt directories. Every pass examines at most 6 attempts and makes at most two
terminal lookups (the second is ATR-V0-171's). Within 11 passes, which is ceil(40 / (6 - 2)) + 1, only the newest 2 runs of all
remain, and the empty directories are kept. A malformed cursor reads as a fresh start.
`TestATRV0154_DuplicateCarryIsMalformed` refuses a cursor that carries a name twice.
`TestATRV0154_CursorWriteUsesItsOwnTemporaryFile` shows a writer holding the old fixed temporary
path cannot alter the cursor, and no temporary file remains.
`TestATRV0154_RetiredBacklogAdvancesTheCursor` plants 1,024 undeletable `.retired-` directories
ahead of three attempts: the first pass records a `.retired-` name, and within 3 passes only the
newest attempt's run remains.
`TestATRV0015_SupervisorRetiresEndedRunsOfTerminalAttempts` passes unchanged.

Rollback: revert the commit. A left cursor file is an ordinary non-directory entry that the
ATR-V0-015 pass skips.

### Typed post-launch write failures, exact dispatch configuration and nil-safe status (ATR-V0-160..162)

Owner context: V1-2123, V1-0951 and V1-1767. ATR-V0-051 says a started run never reports
`MALFORMED` for a store write, but `finish` coded the outcome write's failure with `wire.CodeOf`, which
gives an uncoded error (a filesystem error) `MALFORMED`, and `refusal` gives a receipt refused without
a code `MALFORMED` too. `DecodeConfig` decoded with `encoding/json`, which keeps the last of a repeated
member, so a dispatch configuration could run differently from how it reads. `dispatchStatusValueFor`
dereferenced its configuration, so a nil one panicked.

- `ATR-V0-160`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2123) After the
  command has started, an outcome write that fails without a closed code other than `MALFORMED` (an
  uncoded error such as a filesystem error, or a receipt refused without a code) MUST be reported as
  `UNCERTAIN_EFFECT`, not retryable: the command ran and whether its outcome is recorded is not
  proved. The exit status stays 127 (2 under `HOLD`), the run item and `outcomeReceipt` `null` stay
  as ATR-V0-052 says, and the warning keeps the original error text. A coded write failure keeps its
  own code. Before launch nothing changes.
- `ATR-V0-161`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0951)
  `DecodeConfig` MUST refuse a dispatch configuration in which any object, at any depth, repeats a
  member name, even with the same value. The match is exact after JSON unescaping, so `"tick\u0053econds"`
  repeats `"tickSeconds"`. The error names the repeated member's JSON pointer, for example
  `repeated member "/roles/0/usageFormat"`. Two spellings that differ only by case stay the strict
  decoder's concern. The check runs after the strict decode, so a member whose own decoder refuses a
  repeat, such as a budget override or a prompt part, keeps that decoder's error.
- `ATR-V0-162`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1767) The dispatch
  status view MUST render a nil configuration as the configuration-less fallback
  `&dispatch.Config{StateDir: dir}` that `dispatchAuxConfig` already gives production callers, never
  panic.

Bound: ATR-V0-161 adds one token pass over the configuration, which is at most `MaxConfig` bytes.

Non-goals: ATR-V0-160 adds no new code; `UNCERTAIN_EFFECT` is already in the closed set and in use.
It does not change how the outcome write retries (ATR-V0-051). ATR-V0-161 does not refuse case-folded
aliases of one field; a lone alias stays the strict decoder's concern.

Failure modes: an outcome that was in fact recorded but whose report was lost reads as
`UNCERTAIN_EFFECT`; the journal remains the authority. A configuration with a syntax error is reported
by the strict decoder, as before.

Acceptance evidence: `TestATRV0160_UnrecordedOutcomeIsNeverMalformed` (`internal/tasks/cli`)
injects an uncoded `*fs.PathError` into the outcome write and, through
`SetAttemptOutcomeReportFault`, a receipt refused without a code. Both exit 127 with
`UNCERTAIN_EFFECT`, carry no `MALFORMED`, keep the run item with a null receipt, and keep the original
text. `TestATRV0161_RepeatedConfigMemberRefuses` (`internal/tasks/dispatch`) refuses a repeated
top-level member, a repeated member inside a role, and an escaped spelling of a member, each naming its
path. `TestATRV0162_DispatchStatusRendersForNilConfig` (`internal/tasks/cli`) renders a nil
configuration as the empty one. Each new test fails with its fix reverted: no refusal, a nil-pointer
panic, `MALFORMED`. `TestATRV0004_UnrecordedOutcomeExits127` and `TestATRV0003_CleanupHoldOutranksStatus`
keep their coded `LOCK_TIMEOUT`.

Rollback: revert the commit. No record, receipt or stored format changes. A reverted runner reports
`MALFORMED` again on those paths, and a reverted decoder keeps the last repeated member again.

### Host check precision, its stated limit, alert repetition and the stdin contract (ATR-V0-163..166)

Owner context: V1-1857, V1-1853, V1-1854 and V1-0748. The ATR-V0-133 load check matched the
interpreter list against an element's literal text with its placeholders removed, so
`{ticketLocal}sh` was refused as the interpreter `sh` although a ticket local of `t1` renders `t1sh`.
The check is name-based, which ATR-V0-133 lists as a non-goal without saying who covers the gap. A
ticket whose rendered argv stayed unsafe alerted on every launch. Under a cleanup `HOLD` the leader is
left unreaped, so the `os/exec` goroutine copying a non-file stdin is never joined, and the output-gate
sentence said copying "stops" where later writes are in fact refused.

- `ATR-V0-163`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1857) At load,
  the interpreter list MUST be matched only when an argv element's last path segment (the text after
  its last `/`) holds no placeholder. The literal text of that element, with its placeholders removed,
  is then matched as before, so `{workRoot}/bin/python3` and `{ticketLocal}/sh` stay refused. An
  element whose last segment holds a placeholder, such as `{ticketLocal}sh`, is left to the
  ATR-V0-133 launch recheck of its rendered value. The code-string option rule still applies to every
  element's literal text, so `-c{ticketLocal}` and `-{model}e` stay refused.
- `ATR-V0-164`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1853) The
  `{operatorNote}` (ON-V0-011) and carry-over (ATR-V0-133) host checks are name-based and per
  element. They refuse a listed interpreter anywhere in argv, including after `env` (itself listed)
  and through a path, and a code-string option. They cannot see a program that evaluates a plain
  argument without a listed name or a code-string option: a wrapper script, a renamed interpreter, or
  an agent that runs its prompt as a command. The operator who configures a host for those roles is
  responsible for naming a program that treats `{prompt}` as data. The check does not inspect the
  program file.
- `ATR-V0-165`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1854) When the
  ATR-V0-133 launch recheck withholds a carry-over block, the dispatcher MUST emit the `alert` at most
  once per ticket under one applied configuration digest. Another ticket, or a newly applied
  configuration, alerts again. Every such launch still proceeds without the block, and its `launched`
  event carries no `carryOver`. The record is held in memory only, so a dispatcher restart may
  alert once more.
- `ATR-V0-166`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0748) When the
  caller's stdin is not an `*os.File`, `os/exec` copies it to the command through a goroutine that
  only the reap joins. After a run that ends in `HOLD` (exit 2, `SURVIVORS`) that goroutine may still
  read the caller's reader after `run` returns, so a caller MUST NOT reuse a non-file stdin after
  such a run. The CLI's own entry point passes an empty reader, which reaches EOF at once. When the
  caller's stderr is not a file, later command writes are refused once `run` returns, so group
  survivors that keep writing may see `EPIPE` or `SIGPIPE`.

Bound: ATR-V0-165 keeps one in-memory flag per alerted ticket, cleared when the applied
configuration changes.

Non-goals: gating a non-file stdin. A gate cannot interrupt a read already blocked in the caller's
reader, so ATR-V0-166 states the contract instead. Inspecting a host program's contents. Persisting
the ATR-V0-165 record.

Failure modes: a configuration that was refused only because of a `{ticketLocal}sh`-shaped element
now loads, and a launch whose value renders an interpreter withholds the block and alerts
(ATR-V0-133, ATR-V0-165). A misconfigured evaluating wrapper is not detected (ATR-V0-164).

Acceptance evidence (`internal/tasks/dispatch`):
- `TestATRV0163_LoadMatchesOnlyALiteralProgramName` accepts `{ticketLocal}sh` and refuses
  `{workRoot}/bin/python3`, `{ticketLocal}/sh`, `-c{ticketLocal}` and `-{model}e`. The launch recheck
  still refuses `{ticketLocal}sh` that renders `sh`, and accepts it rendering `t1sh`.
- `TestATRV0164_HostCheckIsNameBased` records, for both checks, that `/usr/bin/env python3` and an
  `env` argument are refused, while a wrapper and a renamed program are accepted.
- `TestATRV0165_UnsafeArgvAlertsOncePerTicketAndConfig` relaunches ticket `sh` twice with one alert.
  Ticket `bash` alerts again, and so does `bash` under a new applied digest. No launch carries the
  block.
- Each of `TestATRV0163_LoadMatchesOnlyALiteralProgramName` and
  `TestATRV0165_UnsafeArgvAlertsOncePerTicketAndConfig` fails with its fix reverted.
- `TestATRV0133_*` and `TestATRV0110_*` pass unchanged.
- ATR-V0-166 is a stated contract and has no maintained test.

Rollback: revert the commit. The load check refuses `{ticketLocal}sh` again, the alert repeats per
launch, and no stored format changes.

### Interrupted progress accounting, durable carry-over invalidation and wrapped secrets (ATR-V0-167..169)

Owner context: V1-0769, V1-1793 and V1-1792. The declared-progress save removed a granted worker
from the ledger and the unparked key's backoff, but the CAL-V0-057 escalation reset and the
infrastructure-retry settlement of that progress were applied later, by `finish` and the unparked
loop, and saved only at the tick's end. A dispatcher interrupted between the two saves lost them, so
the ladder kept escalating a ticket that had progressed. A carry-over record whose removal failed
after progress was guarded only by an in-memory mark, so a restart offered its stale entries once.
The ATR-V0-104 secret screen matched one line at a time, so a PEM block's body or a token a model
broke across lines survived in the salvage note and the carry-over record.

- `ATR-V0-167`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0769) The ledger
  save that admits declared progress MUST also apply, in the same write, the CAL-V0-057 escalation
  reset and the infrastructure-retry progress settlement of every granted worker it removes and the
  escalation reset of every key it unparks. `finish` and the unparked loop apply them again, which
  changes nothing. An interrupt after that save and before `finish` MUST NOT leave an escalation
  entry or a running infrastructure episode for a key whose progress was admitted.
- `ATR-V0-168`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1793) Each
  carry-over entry MUST record the CAL-V0-057 work fingerprint the session launched on, as
  `fingerprint`. A launch MUST offer only the entries whose fingerprint equals the ticket's current
  work fingerprint, and an append MUST first drop the entries recorded at another fingerprint. An
  entry without a fingerprint, written before this requirement, is never offered. The guarantee holds
  across a dispatcher restart that loses the ATR-V0-111 in-memory mark after a failed removal.
- `ATR-V0-169`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1792) Besides
  every line the secret screen matches alone, the ATR-V0-104 screen MUST replace every non-empty line
  that a match spanning more than one line touches in either of two passes. The first pass screens
  the whole text with its line breaks. The second screens each run of two or more consecutive lines
  that do not match alone, joined after removing their surrounding blanks, tabs and carriage returns.
  `redactedLines` counts each replaced line once.

Non-goals: a secret that the screen does not recognise when whole. A token whose wrapped pieces are
interleaved with lines that match alone. Rewriting the stored records of an earlier format.

Failure modes: the second pass can replace a benign line that continues a wrapped token's
characters, or prose lines whose concatenation is secret-shaped; the screen fails closed. A secret
split across a line that matches alone keeps its pieces on the other lines. A carry-over entry
written before ATR-V0-168 is dropped rather than offered. An interrupt before the progress save
leaves the progress unadmitted, and the next tick admits it as before.

Acceptance evidence (`internal/tasks/dispatch`):
- `TestATRV0167_InterruptAfterProgressSaveKeepsAccounting` cancels the tick inside the progress save
  for a granted worker and for an unparked key. The saved ledger holds no escalation entry and, for
  the worker, a recovered infrastructure episode, with no `finished` or `unparked` event. Both
  subtests fail with the fix reverted (the ladder survives).
- `TestATRV0168_ProgressInvalidatesCarryOverAcrossRestart` fails a removal after progress, reopens
  the dispatcher and shows that no earlier entry is offered or extended, and that an entry without a
  fingerprint is not offered. It fails with the fix reverted.
- `TestATRV0169_ScreenRedactsSecretsWrappedAcrossLines` uses a synthetic PEM block and a token broken
  by a newline. It shows that the screen, the salvage note and the carry-over record replace exactly
  the six lines holding them, keep the lines around them, and keep none of the fragments. It fails
  with the fix reverted.
- `TestATRV0104_*`, `TestATRV011*`, `TestCALV0057*`, `TestCALV0064*` and `TestCALV0194*` pass.

Rollback: revert the commit. The progress accounting is applied only by `finish` again, the record
loses its `fingerprint` member (the reverted reader treats a record holding it as unreadable and
offers no carry-over until the next write replaces it), and the screen returns to single lines.

### A read-only stage that edits the candidate (ATR-V0-170)

Owner context: V1-0832. A review or integrate stage runs over the bound candidate and must not change
it. When such a stage exited cleanly after editing the candidate worktree, the supervised role
returned a retryable `MALFORMED` "read-only stage changed candidate" before journaling the stop. The
attempt stayed `STOPPING` with `worker=true`, the program kept its owner, and a rerun of the role was
refused `RESOURCE_COLLISION`.

- `ATR-V0-170`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0832) When a
  review or integrate stage stops with proved quiescence and its worktree is dirty, unreadable, or
  fails the composite candidate check, the role MUST still journal the stop. The stop MUST NOT be
  accepted and MUST carry no claims. It MUST carry an operator question that names the change and asks
  for the worktree to be restored to the bound candidate, so the attempt reaches `WAITING` with no
  worker and the program is `FINISHED` with its owner released. The role MUST then return a
  non-retryable `DIRTY_WORKTREE` that keeps any stage error's text. After the operator restores the
  worktree and answers, the role runs again. A stop without proved quiescence keeps its earlier
  `SURVIVORS` classification (CAL-V0-086), which takes precedence.

Non-goals: restoring the candidate automatically, or keeping the stage's edits as a new candidate.
An implement stage, which may change the worktree, is unchanged.

Failure modes: if the operator answers without restoring the worktree, the rerun's stop meets the
same check again and waits again. A journal failure while recording the stop returns that error, as
before.

Acceptance evidence (`internal/tasks/store`):
- `TestATRV0170_ReadOnlyStageEditIsTypedAndRecoverable` implements to `BUILT`, then runs a review stage
  that edits the candidate and exits cleanly. It shows a non-retryable `DIRTY_WORKTREE`, an attempt
  `WAITING` with no worker and a question, and the program `FINISHED` with its owner released. After
  the worktree is restored and the question answered, the review reaches `READY_FOR_INTEGRATION`. It
  fails with the fix reverted (`MALFORMED`).
- `TestCALV0086_UnprovedStopIsNotFinished` (including its review case) and the program workflow tests
  pass.

Rollback: revert the commit. The role returns the retryable `MALFORMED` again and the attempt is left
`STOPPING`; no stored format changes.

### A retry racing run retirement (ATR-V0-171)

Owner context: V1-0933. An ATR-V0-015 pass read the attempts' terminal records without the store
lock and then removed runs. A retry that revived an attempt between the read and the removal lost
that attempt's earlier-generation runs, though a live attempt's runs are never to be pruned.

- `ATR-V0-171`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0933) A pass MUST
  first move each run it would remove out of its attempt directory, in one rename, to
  `taskman-runs/.retiring-<attempt directory>-<run ID>`. It MUST then read the terminal records of
  those attempts again (`store.AttemptRecords`), and that read begins after every move. A run is
  removed only when this second read finds its attempt terminal at the run's generation or a later
  one. Every other moved run, including all of them when the read fails, MUST be moved back into its
  attempt directory. A removed run is first renamed to its `.retired-` name, as before. A pass that
  finds a `.retiring-` entry in its window MUST move it back, and no pass removes one. Ordering
  guarantee: a retry whose generation opens before the second read keeps every run of its attempt.
  A retry that opens after it finds the runs already gone from the attempt directory, as after a
  completed pass.

Non-goals: holding the store lock across retirement. Retirement of a live attempt's runs, which
ATR-V0-015 already excludes.

Failure modes: while a run is hidden, an attach or listing finds no run and refuses
`MISSING_EVIDENCE`, as for a retired run, even when the run is then moved back. A concurrent pass
can move a hidden run back during the check, and the owning pass then keeps it. A `.retiring-` name
that does not parse, or a run that cannot be moved back, is left in place. A moved-back run may look
recently changed and wait another hour.

Acceptance evidence (`internal/tasks/cli`):
- `TestATRV0171_RetryDuringRetirementKeepsItsRuns` keeps 1 run and plants three terminal attempts.
  Inside the second read it shows that the runs being removed have left their attempt directories.
  That read reports one attempt live (the retry) and one terminal at a later generation. The revived
  attempt's run is moved back, the terminal one is removed, and no `.retiring-` or `.retired-` entry
  remains. A failed second read keeps every run, and a hidden run left by a stopped pass is restored.
  With the second read replaced by the first, the revived run is removed and the test fails.
- `TestATRV0154_*` (now allowing two lookups) and `TestATRV0015_*` pass.

Rollback: revert the commit. A left `.retiring-` entry is a non-attempt name that the reverted window
passes over, so its run stays hidden until restored by hand
(`mv taskman-runs/.retiring-A-R taskman-runs/A/R`).

### V1-0925, V1-0924 and V1-0934 dispatcher idle and retirement cost

Re-homed (owner-approved, 2026-10-17): these requirements were first numbered CAL-V0-350..353 in the
agent leases spec; they moved here as ATR-V0-180..183 with the same text so that spec stays within
the 256-obligation bound. Their test names keep the old numbers.

Owner context: three costs the dispatcher paid on every tick, or failed to pay, without need. A
full tick read and hashed the configuration bytes even when the file's identity had not changed
since the last read (V1-0925). A pressure configuration disabled the CAL-V0-139 idle gate, so a
dispatcher with nothing to launch still read the store and sampled the host every tick (V1-0924).
A worker-directory retirement pass read the first 4,096 entries of `workers/` every time, so a
backlog behind entries it could not retire was never reached (V1-0934).

- `ATR-V0-180`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0925) A full tick
  MUST reuse the digest of the configuration bytes it last read, without reading them, only when
  all of these hold: the file's identity (the same file, size, mode and modification time, as
  CAL-V0-127 compares it) is unchanged since that read; the stat succeeds; and the read is less
  than the CAL-V0-139 safety-net interval (15 minutes) old by a clock that has not gone backwards.
  Otherwise it MUST read the bytes, as before. A reused digest that equals a refused configuration
  is not reported again; one that equals the applied configuration keeps it. Any identity change
  reads the bytes, so a replacement or restoration is handled exactly as CAL-V0-127 says. The same
  cache age bounds a pending reload check, which reuses the digest under the same conditions, and
  the idle gate's next full tick, which is due no later than 15 minutes after that read.
- `ATR-V0-181`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0924) Amends
  CAL-V0-139 and CAL-V0-068. A pressure configuration MUST NOT by itself disable the idle gate.
  While the dispatcher supervises no worker and its recorded pressure state is calm (level 0, no
  pending level, a known sample), a tick MUST plan its launches against a deferred pressure budget
  before it samples the host. The deferred budget refuses its first candidate and records that it
  was consulted. A plan that never consults it can launch nothing at any level, since calm is the
  most permissive. That tick then takes no sample and leaves the pressure record unchanged, so the
  idle gate can arm. A consulted plan MUST sample the host, step the hysteresis and plan again
  against the real budget, so no launch ever precedes a sample. Any other recorded state, or a
  running worker, samples every tick as before. The first sample after a deferred tick has no CPU
  baseline, as after `Open`. From that sample until the pending transition resolves, admission
  uses the pending level whenever it is higher than the recorded level, because the deferred
  ticks never advanced the dwell. When CPU is a selected signal and that first sample has no CPU
  interval, it admits no launch that the budget would cap, and the next tick samples a fresh
  interval; an UNKNOWN CPU reading never admits. The recorded state keeps the ordinary hysteresis.
- `ATR-V0-182`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0934) Amends the
  retirement scan of CAL-V0-144. A retirement pass MUST examine one window of names: the directories retained by
  earlier windows, at most 32, carried forward, plus at most 4,096 less those read from the
  `workers/` directory stream where the previous window's read ended. The stream stays open
  between passes, and a short read ends it, so the window after it starts over. The window is held
  until its marks settle, or for at most 4 passes, so CAL-V0-144's two-pass confirmation is
  unchanged within it. A window that settles while the stream continues schedules the next pass
  for the next tick, which the idle gate treats as a deadline. Retention counts the window and the
  carried directories together. The newest 32 of all are in the newest 32 of any window that holds
  them, so they are never removed. One sweep leaves only the newest 32 retirable directories of
  all. `Close` closes the stream.

Non-goals: no change to what is retired, retained or protected, to the mark format or to the
ledger; no persisted retirement cursor for the dispatcher (a restart starts a new sweep); no reuse
of a pressure sample across ticks while work is eligible.

Failure modes: a configuration rewritten within the filesystem's timestamp granularity with the
same size and mode is not seen until the safety-net read, at most 15 minutes later (CAL-V0-127
already relied on that identity for pending reloads). A pressure spike that rises and falls while
the dispatcher is idle is not recorded. Its admission effect is nil, because nothing launches,
but the `throttled` event history no longer shows it. A window that cannot settle within 4 passes,
for example under an unreadable process table, moves on and leaves its marks to be cleared, so
confirmation starts over on the next sweep. Directory entries added or removed while the stream is
open may or may not be returned, as POSIX allows; an unreturned one waits for the next sweep.

Acceptance evidence (`internal/tasks/dispatch`):
- `TestCALV0350_FullTickReusesAnUnchangedConfigRead` forces every tick to be full. It counts five
  reads where every tick would otherwise read: one on the first tick, none over five unchanged
  ticks, one after the safety-net interval, one for an invalid replacement (refused and not
  reported again), one for a valid change, then reuse again, and one after a stat error.
  `TestCALV0350_IdleSkipEndsWhenTheCachedConfigExpires` rewrites the file with the same size and
  preserved timestamps while idle: minute 14 reuses the digest, and by minute 16 the bytes are read
  and the new prompt applies. With both the idle bound and the pending-check age removed it fails.
- `TestCALV0351_IdlePressureConfigSkipsTheSample` takes one sample in 24 idle ticks and makes no
  store observation in the last 20; the idle gate is armed. Eligible work under a critical first
  sample (dwell 2) is held at the pending level. The next calm sample launches the worker after
  its sample. A mutation that removes the pending-level budget fails the test.
  `TestCALV0351_PostIdleHoldLastsUntilThePendingLevelSettles` uses dwell 3 and critical cap 0:
  two critical samples after idle still hold, and the hold clears once calm settles.
  `TestCALV0351_PostIdleCPUSampleWaitsForAFreshInterval` selects CPU only: the first sample after
  idle has no interval and launches nothing, the next is critical and holds, a calm one launches.
  A mutation that holds for one gap sample only, or drops the CPU baseline hold, fails these.
  `TestCALV0351_DeferredBudgetRefuses` covers the budget itself. The CAL-V0-068, CAL-V0-125 and
  CAL-V0-139 tests pass; `TestCALV0125_UnselectedCPULeavesLedgerUnchanged` now counts one sample
  in its two idle ticks.
- `TestCALV0352_RetirementSweepsABacklogBeyondOneWindow` lowers the scan to 10 and the retention
  to 3. It plants 40 quiet directories and 20 inside the quiet window. Every held window is at
  most 10 names, and every pass that leaves a backlog schedules another. Within 2 x 9 + 1 = 19
  passes, only the newest 3 and the 20 quiet directories remain (18 on macOS). Every CAL-V0-144
  test passes unchanged.

Rollback: revert the commit. The dispatcher again reads the configuration on every full tick,
samples pressure every tick and disables the idle gate under a pressure configuration, and scans
the first 4,096 worker directories. No ledger, mark or event format changed.

### V1-0948 dispatch test process hygiene

Orphaned worker trees from dispatch tests (`host.sh` from `TestCALV0143_*`, `TestCALV0272_*` and the
`TestCALV0127_Reload*` `sleep 300` children) outlived the test binary for days and fed host load.

- `ATR-V0-183`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0948) Every
  worker process group a dispatch test launches MUST be stopped with SIGKILL to its process group
  and found empty, within a bounded wait, by a cleanup the test registers before it launches, so
  the cleanup also runs after `t.Fatal`. A group is signalled only when its leader still carries
  the recorded start identity; an absent leader or an identity read error is not ownership. A
  group that is still present without that match is reported as a possible leak that cannot be
  verified, and is not signalled. After all tests, the package MUST fail when any launched group
  that it could verify was still alive. The production hook is a nil function variable that `launch`
  calls with each started leader; it has no effect outside tests.

Non-goals: processes a worker moves out of its process group, and processes started by other test
packages. Failure mode: a group that survives SIGKILL fails the test that launched it.

Acceptance evidence (`internal/tasks/dispatch`): `TestCALV0353_CleanupReapsTheWholeWorkerGroup`
launches a shell with two `sleep` descendants and proves the reap empties the group and the leak
check sees it before and not after. With the `testConfig` cleanup removed, a run of the
`TestCALV0127_Reload*`, `TestCALV0143_*` and `TestCALV0272_*` tests fails with four leaked groups;
with it, the full package passes and no test-launched `host.sh` remains.
`TestCALV0353_UnmatchedLeaderIsReportedNotSignalled` lets a leader exit while its child keeps the
group: the group is reported unverified and the child is not signalled.

Rollback: revert the commit; tests again leave worker trees to the caller.

### V1-2263 and V1-1964 naming the blocking attempt

`ticket complete-manual` was refused `ATTEMPT_LIVE` by an attempt whose lease had expired hours
earlier, and neither that refusal nor `ticket show` named the attempt; a `RESOURCE_COLLISION` claim
refusal named only an attempt ID. Operators read `reservations.json` by hand to find the holder.

- `ATR-V0-190`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2263) An
  `ATTEMPT_LIVE` refusal MUST name the blocking attempt: its attemptId, ticket, generation, phase,
  holder (`none` without a work lease), leaseExpiresAt (`null` without one), holderStatus (the
  CAL-V0-120 classes, derived at the refusal's time) and one recovery: the `reap` command with its
  exact `--attempt`, `--generation` and `--lease-expires-at` when the lease expired; the `release`
  command when the lease is live or the attempt has no work lease; and the supervising program when
  the attempt is supervised. A writer that did not decode the attempt record keeps the identity and
  reports the rest `NOT_OBSERVED`; the CLI then adds the full summary as a `blocking attempt ...`
  warning from a read of the same store. Decision: an attempt whose lease expired still blocks
  `complete-manual` until it is reaped, because membership in the reservation set is the liveness
  contract and no read, refusal or completion reaps implicitly; `reap` is the one-command recovery
  the refusal names.
- `ATR-V0-191`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2263) `ticket
  show` MUST carry `liveAttempt` (beside the existing `currentAttempt` satisfaction state, which is unchanged): `null` when the reservation set holds no attempt for the
  ticket, the string `NOT_OBSERVED` when the store has no journal to read, and otherwise an object
  with `attemptId`, `generation`, `phase`, `holder`, `leaseExpiresAt` (both `null` without a work
  lease), `holderStatus` and `recovery`, with the same values as ATR-V0-190.
- `ATR-V0-192`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1964) Every
  claim, scope-widening and `WHOLE_REPOSITORY` `RESOURCE_COLLISION` refusal MUST name the colliding
  attempt with the ATR-V0-190 detail and append `colliding keys: CLASS:key,...`, listing the
  requested keys that collide with that attempt's reserved resources.

- `ATR-V0-280`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1964 criterion 1
  as narrowed by decision 0552) When the holding attempt's holderStatus is `STALE_HOLDER`, or it has
  no live work lease, the ATR-V0-190 recovery (and so every `ATTEMPT_LIVE` and `RESOURCE_COLLISION`
  refusal and `liveAttempt`) MUST name the exact supported command that frees it: `reap` with its
  exact values when the lease expired, otherwise `corvint-tasks release --attempt <id> --generation
  <g> --request-id NEW_ID`. A `STALE_HOLDER` recovery MUST say the stale heartbeat is advisory and
  frees nothing (CAL-V0-121).
- `ATR-V0-281`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1964 criterion 3
  as narrowed by decision 0552) A supervised attempt holds its reservation with `leaseExpiresAt`
  `null` because its liveness is its supervising program's, not a work lease's, so `reap` (which
  frees only an expired work lease) never frees it. Its explicit release paths are its program
  ending it, or its holder's `release` with the exact attempt and generation, and its recovery MUST
  name that `release` command beside the program.

Non-goals: reaping a `STALE_HOLDER` attempt whose lease is still live, or a supervised attempt
without a work lease (V1-1964 criteria 1 and 3 as filed). The accepted contract says no read,
heartbeat, renew or reap frees such an attempt; decision 0552 keeps CAL-V0-121 and narrows those
criteria to ATR-V0-280 and ATR-V0-281. The summary is advisory text and is never an input to admission. Failure modes: the
post-refusal read fails or the attempt was retired in between; the warning is then omitted and the
refusal keeps its writer-side detail. Holder status is derived at the moment it is read and can
change before the operator acts; the named command still binds the exact generation and lease.

Acceptance evidence (`internal/tasks/cli`): `TestATRV0190_CompleteManualRefusalNamesExpiredAttempt`
claims with a lease that expired three hours earlier, proves `complete-manual` is refused
`ATTEMPT_LIVE` naming the attempt, ticket, generation, holder, `LEASE_EXPIRED` and the exact reap
command, reads the same summary from `ticket show` `liveAttempt`, reaps with the named values,
reads `liveAttempt` `null`, and completes. `TestATRV0192_CollisionNamesHoldingAttempt` proves a
colliding claim names the holding attempt, its ticket, `FRESH_HOLDER`, `colliding keys: PATH:src/a`
and the live-lease recovery. Both tests fail without the change.
`TestATRV0280_RecoveryNamesReleaseForEveryHolderWithoutReap` (`internal/tasks/transaction`) proves
a `STALE_HOLDER` with a live lease, a stale supervised attempt with a `null` leaseExpiresAt and a
lease-less attempt each name the exact `release` command, the stale ones say the heartbeat is
advisory, and none offers a reap; it fails without the ATR-V0-280/281 change.

Rollback: revert the commit; refusals again name only the attempt ID and `ticket show` drops
`liveAttempt`. No stored format changes.

### V1-1321 claim scope, V1-1237 and V1-1441 read-only review sessions
V1-1321: a claim on a ticket that declares `QUALIFIED` touchPaths took the `DECLARED` scope
(CAL-V0-021) and returned OK while it silently dropped every requested `--scope` path, on a first
claim and on a re-claim after release, so a lane learned it at submit (`OUT_OF_SCOPE`). The ticket
allows either reserving every requested path or refusing and naming the unaccepted ones; refusing
keeps the CAL-V0-021 precedence unamended, which is why the 2026-10-15 attempt stopped.
V1-1237 and V1-1441: a Codex review run with `codex exec -s read-only` still wrote the task store.
Root cause (local probe, codex-cli 0.162.0): the read-only seatbelt sandbox denies the write and
passes the parent environment through, so a sandboxed child cannot write `.git/taskman`; the write
came from a command Codex escalated outside the sandbox, which `approval_policy=on-request` (the
default) allows. Codex marks a sandboxed child with `CODEX_SANDBOX=seatbelt` whether it is
`read-only` or `workspace-write`, so that variable cannot say "read-only". The guard is therefore an
explicit signal the review recipe sets, which an escalated child also inherits (inference from the
default `shell_environment_policy`, not live-qualified).
- `ATR-V0-210`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1321) When a ticket
  declares paths and its claim takes the `DECLARED` scope, every requested `--scope` path MUST be
  covered by a declared path, else the claim MUST refuse `BLOCKED` `OUT_OF_SCOPE` before any write,
  naming each uncovered requested path in request order and pointing at `widen` (CAL-V0-025). This
  holds for a first claim and a re-claim after release, in both `--scope a b` and
  `--scope a --scope b` forms. A ticket that declares no path keeps the `REQUESTED` scope.
- `ATR-V0-211`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1321) Every claim
  result MUST carry `scope`: `{source, resources}` for the scope the attempt holds after the claim,
  or `null` when the claim reserved none (a refusal or replay without a held attempt). The verbose
  `claim` help names ATR-V0-210 and ATR-V0-211.
- `ATR-V0-212`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1237, V1-1441)
  While `CORVINT_TASKS_READ_ONLY` is set to a value other than empty or `0`, `corvint-tasks` MUST
  refuse every command outside a fixed allow-list of read commands (`version`, `roadmap`,
  `critical-path`, `pending`, `doctor` without `--refresh`, `queue status`, the `show`, `list`,
  `search`, `status`, `history`, `audit`, `verify` and `readiness` reads and the ticket obligation
  reads) with `ERROR` `UNSUPPORTED`, not retryable, before it opens the store, and the refusal names
  the variable. An unknown or newly added verb is refused until it is listed. Review recipes set
  the variable and pass `-c approval_policy="never"` (`docs/TASKS-SUPERVISION.md`).
Non-goals: a union of declared and requested paths (CAL-V0-021 stays a precedence); detecting a
read-only sandbox without the explicit variable; containing writes that bypass the CLI (a review
that edits `.git/taskman` directly, or unsets the variable, is the host sandbox's responsibility).
Failure modes: a lane that relied on the silent drop now gets a refusal and must claim the declared
scope and widen; an operator who sets the variable for a mutating session gets `UNSUPPORTED`.
Acceptance evidence: `TestATRV0210_ClaimRefusesRequestedPathsOutsideDeclaredScope` and
`TestATRV0211_RequestedScopeWithoutDeclaredPathsIsReported` (`internal/tasks/store`);
`TestATRV0211_ClaimResultListsEffectiveScope` and `TestATRV0212_ReadOnlySessionRefusesMutations`
(`internal/tasks/cli`); each fails with its fix removed. The Codex probe is recorded in the build
log; a live `codex exec -p corvint-review` run (Codex 0.162.0) refused `ticket create` with the store
unchanged (V1-2326, `AHI-206`, 2026-10-23 Codex matrix entry); reviews repeat the profile keys as the
Codex README's `-c` overrides. V1-1441's owner review of the V1-1375 operator note stays with the
owner.
Rollback: revert the commit. Claims again drop uncovered requested paths silently, the `scope`
member disappears from claim results, and the variable has no effect; no stored record changes.
### A program control beside a phase write, and mapped writes after a lease audit (ATR-V0-240..241)

Owner context: V1-0831 and V1-0775. A supervised workflow composed each phase record from an
unlocked read of `programs.json` and wrote it later under the program writer. A `DRAIN` or `CANCEL`
that an operator recorded between that read and the write was overwritten with the stale control,
so the run never drained and `RequestProgramControl` reported the control superseded. Separately, a
lease write reuses its preparation audit's digests for the store files it only reads, and a write
through a shared mapping raises no change event, so a file can change after the audit unseen.

- `ATR-V0-240`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0831) A program
  phase record that a workflow writes (a stage phase, a persisted phase, a run record) MUST carry the
  control that the `programs.json` revision it is written against records for that program, read
  under the program writer. The composed record's own control never replaces it. Operator control
  requests and a resume, which set the control deliberately, are unchanged.
- `ATR-V0-241`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0775) Recorded
  limit: a lease write does not re-read, under the writer lock, the store files its preparation
  audit read and it only reads, so a mapped write to one of them after that audit is not refused by
  that lease write, which commits the decision it derived from the audited content. The receipt
  MUST NOT absorb the mapped content: the next write's audit MUST refuse the store
  (`JOURNAL_FORKED` for a request projection, `INTENT_DIVERGED` for an intent file) while the change
  is in place, and MUST accept it again once the file is restored. Closing the window needs a
  content re-read of every audited file per lease write, which CAL-V0-263 keeps out of the lock;
  that trade is left to the owner.

Non-goals: a mapped write after a write commits, which only the next audit can see.

Failure modes: a lease write that raced a mapped write is committed; the store then refuses every
write until the file is restored or repaired.

Acceptance evidence (`internal/tasks/store`):
- `TestATRV0240_PhaseWriteKeepsControlRecordedMeanwhile` records `DRAIN` between a workflow's
  phase composition and its write; the written phase keeps `DRAIN`. With the unlocked control copy
  restored, it fails with the control cleared.
- `TestATRV0241_MappedWriteAfterLeaseAuditRefusedByNextWrite` (darwin and linux) flips a mapped
  byte of the request projection and of `intent/policy.json` at the `sweep` and `lock` commit
  stages; the claim commits, the next `Mutate` refuses with the stated code and publishes nothing,
  and a `Mutate` after the restore completes. Darwin APFS runs; on Linux (go1.27.2, 2026-10-23,
  V1-0775 lane) it passes on ext4 and on tmpfs (`TMPDIR=/dev/shm`), with the claim committing
  under the mapped write in all four cases, which confirms the limit on Linux. The fix remains the
  owner's trade named above.

Rollback: revert the commit. ATR-V0-241 changes no runtime code.

### V1-2189 dispatch fixture workers die with their test binary

ATR-V0-183's cleanups and leak check run inside the test binary, so a binary killed by SIGKILL (a
CI step timeout, an operator, an out-of-memory kill) leaves every worker group it launched running:
`host.sh` trees from the CAL-V0-127 and CAL-V0-143 fixtures outlived their binary on the host.

- `ATR-V0-214`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2709) A claim
  refused `OUT_OF_SCOPE` under ATR-V0-210 MUST carry `scopeRefusal: {declared, rejected,
  nextAction}` in its result item: `declared` the ticket's declared paths (CAL-V0-021, sorted),
  `rejected` the uncovered requested paths in request order, and `nextAction` the fixed retry
  `transaction.ScopeRetry` (claim without `--scope` or with declared paths only; widen afterwards).
  The refusal detail MUST name the declared paths and the retry, and MUST name each rejected path
  that is a declared directory key without its trailing `/`. Any other claim result carries no
  `scopeRefusal`.
- `ATR-V0-215`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2709) A claim
  whose every requested `--scope` path is covered by a declared path, including a declared key
  itself, MUST be admitted with the `DECLARED` scope; only a path outside the declaration refuses.
- `ATR-V0-216`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2709)
  `ticket show` MUST carry top-level `declaredScope`, the paths ATR-V0-214 would report as
  `declared`, or `null` when the ticket declares none and a claim takes its requested scope.
Non-goals: admitting requested paths outside the declaration (CAL-V0-021 stays a precedence);
normalising a path spelling the caller sent. Failure mode: a caller that matched the old refusal
text sees a longer detail; the code and outcome are unchanged. Rollback: revert the change; the
refusal returns to naming only the rejected paths and `declaredScope` disappears.
Acceptance evidence: `TestATRV0214_OutOfScopeClaimNamesDeclaredPathsAndRetry`
(`internal/tasks/cli`) covers the refusal payload and detail, subset admission, and `declaredScope`
for a declared and an undeclared ticket.

Precise path reservations, ATR-V0-800 and ATR-V0-801, are proposed (V1-3185). A ticket that
declares a directory such as `docs/build-log/` reserves every path beneath it, so tickets that
write distinct files there serialize behind it, and ATR-V0-215 keeps a narrower `--scope` from
shrinking a declared scope. The supported workflow is to declare the exact files before admission:
`ticket create` or `ticket set-effects` with file touchPaths. Collision semantics, ATR-V0-215 and a
live holder's reservation are unchanged; nothing narrows another worker.

- `ATR-V0-800`: (proposed; V1-3185) A claim refused `RESOURCE_COLLISION` against a live attempt MUST, after the
  ATR-V0-192 detail, name the safe next action. When a colliding pair involves a directory key on
  either side, the detail MUST say that a directory key reserves every path beneath it and that
  tickets writing distinct files declare exact file touchPaths with `ticket set-effects` before
  claiming. When the claim took the `DECLARED` scope although `--scope` was given, the detail MUST
  say the claim holds the declared scope, not `--scope` (ATR-V0-215). Code, outcome and the
  absence of a write are unchanged.
- `ATR-V0-801`: (proposed; V1-3185) Two tickets whose declared touchPaths are distinct exact files under one directory
  MUST both be admitted while both attempts are live, including when the two claims race. A third
  claim declaring the same file, or the parent directory key, MUST refuse `RESOURCE_COLLISION`, as
  MUST two exact-file claims that both declare one non-`PATH` resource. A submitted change outside
  an exact-file scope still refuses `OUT_OF_SCOPE` (CAL-V0-024) and a stale generation is still
  fenced (CAL-V0-009).
Non-goals: automatic narrowing of a declared or live scope; changing PATH-prefix exclusion.
Failure mode: a caller matching the old collision text sees a longer detail. Rollback: revert the
detail suffix; the tests of ATR-V0-801 pin existing admission and stay.
Acceptance evidence: `TestATRV0801_DistinctDeclaredFilesAdmitConcurrently` and
`TestATRV0800_CollisionNamesTheNextAction` (`internal/tasks/store`);
`TestATRV0801_CompiledBinaryAdmitsDistinctBuildLogFiles` (`internal/tasks/cli`, the compiled
native qualification on a scratch fixture queue: two distinct build-log files admitted, the same
file and the parent directory refused, the directory refusal naming the set-effects action);
`TestCALV0024_SubmitOutsideTheScopeIsRefused`; `TestCALV0009_StaleGenerationIsFencedAndRecorded`.

Claim scope diagnostics, ATR-V0-806..808, are proposed (V1-3185, V1-3540, V1-3547). They add
warnings and one revision resolution; scope precedence (CAL-V0-021), collision semantics and the
recorded attempt are unchanged.

- `ATR-V0-806`: (proposed; V1-3185) A fresh claim whose scope fell back to `WHOLE_REPOSITORY` (the ticket declares no
  PATH scope, for example coverage `INCOMPLETE` with no touchPaths, and the claim gave no `--scope`
  or `--scope-from-diff`) MUST commit as before and warn `WHOLE_REPOSITORY_FALLBACK`. The warning names the ticket and attempt, says the attempt reserves
  every path so every other claim collides with it until it ends, and names the narrowing routes:
  release and claim again with `--scope PATHS` or `--scope-from-diff REV`, or declare the ticket's
  paths with `ticket set-effects` first. A replay and a declared, requested or derived scope add
  none.
- `ATR-V0-807`: (proposed; V1-3540) A fresh `widen` that commits a requested PATH key without a trailing `/` naming a
  directory at the attempt's base commit MUST commit as before and add the CAL-V0-211
  `EXACT_KEY_NAMES_DIRECTORY` warning for that key. Keys the widen did not request, a widen that
  changed nothing, and a replay add none.
- `ATR-V0-808`: (proposed; V1-3547) `claim --base REV` MUST resolve any revision naming one commit, as
  `--scope-from-diff` resolves its revision, and record the full commit OID in the request and the
  attempt; the replay of a request is keyed on that OID. A revision that names no commit MUST
  refuse `MALFORMED` naming the revision, the checkout and `git rev-parse --verify REV^{commit}`,
  and write nothing. A full OID is used unchanged.
Non-goals: refusing a WHOLE_REPOSITORY fallback; narrowing a live attempt; normalising scope keys.
Failure mode: a caller that treated any warning as failure now sees one on a fallback claim; the
outcome and code are unchanged. A short `--base` that is ambiguous in the checkout refuses as Git
does. Rollback: revert the change; claims fall back silently again and `--base` again needs a full
OID.
Acceptance evidence: `TestATRV0806_WholeRepositoryFallbackWarns`,
`TestATRV0807_WidenWarnsOnExactDirectoryKey` and `TestATRV0808_ClaimBaseResolvesARevision`
(`internal/tasks/store`).
- `ATR-V0-213`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2189) The
  `internal/tasks/dispatch` test binary MUST start, before any test runs, a dead-man reaper: a copy
  of itself in its own session whose standard input is a pipe whose write end only the test binary
  holds (close-on-exec, so no worker inherits it). The test launch hook writes each launched
  leader's pid and start identity to that pipe. At end of file, which the kernel delivers once the
  binary has exited by any means, the reaper sends SIGKILL to each recorded group whose leader still
  carries its recorded identity, and repeats that signal while the killed group stays present,
  within a bound, so a child forked during delivery is also stopped. It never signals a group whose
  leader is gone or unreadable. After a normal run the binary closes the pipe once its own leak check
  has reaped every group, and waits, bounded, for the reaper.
Non-goals: worker groups launched by other test packages (`internal/tasks/cli` is covered by
ATR-V0-310); processes a worker moves out of its group; a reaper that is itself
killed. Failure mode: a worker group still present 30 seconds after its binary is SIGKILLed fails
`TestATRV0213_FixtureWorkersDieWithTheirTestBinary`. Acceptance evidence: that test, which fails
with the reaper start disabled, and its bounded repeat run recorded in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-flakes-1021n-dead-man-reaper.md`. Linux NOT_RUN locally.
Rollback: revert the commit; the leak check and cleanups of ATR-V0-183 are unchanged.

### V1-0830, V1-0828, V1-1874 and V1-1438 refusal and recovery messages
V1-0830: the dispatch verbs' usage line said `[--grant FILE]` while `--grant` takes a grant ID.
V1-0828: a gate run, renew or heartbeat after the lease expired refused `FENCED` naming only the
expiry; nothing said that no command of that generation can take effect (CAL-V0-009/010) or that a
re-claim reaps the expired lease (CAL-V0-011). V1-1874: an `APPROVAL_REQUIRED` ticket refused the
claim naming only the missing `RUN` grant, and `complete` later refused `APPROVAL_MISSING` for the
missing `COMPLETE` grant, after the work was done. V1-1438: a mutation whose `--issued-at` defaulted
and whose output was lost could be recovered by the CAL-V0-250 explanation only while the ticket's
`createdAt` or `updatedAt` stayed within two seconds of it; a later update moved `updatedAt`, so a
lost-output retry of an update found no rebuilt digest.
- `ATR-V0-270`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0830) The usage
  line of `run`, `admit`, `resume`, `retry`, `drain`, `cancel` and `answer` MUST name the flag
  `[--grant ID]`, matching its parse, and never `--grant FILE`.
- `ATR-V0-271`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0828) A `FENCED`
  refusal for an expired lease MUST name its remedy after the expiry: no command of the generation
  takes effect and renew cannot revive it (CAL-V0-010); claim the ticket again, which reaps the
  expired lease (CAL-V0-011), then submit and run the gates at the new generation; renew or
  heartbeat before expiry to keep a lease through an idle gap.
- `ATR-V0-272`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1874) The
  `APPROVAL_MISSING` claim blocker of an `APPROVAL_REQUIRED` ticket MUST name every grant the ticket
  lacks through completion (`RUN` and `COMPLETE`, in that order) at its acceptance revision, not only
  `RUN`.
- `ATR-V0-273`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1874) `ticket show`
  and `ticket blockers` MUST carry `missingGrants`: the operations among `RUN` and `COMPLETE` with no
  unrevoked grant at the ticket's acceptance revision, in that order, and an empty array for a ticket
  that is not `APPROVAL_REQUIRED`.
- `ATR-V0-274`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1874) A fresh claim
  that a `RUN` grant admits on an `APPROVAL_REQUIRED` ticket MUST warn, naming the acceptance
  revision, when the ticket holds no `COMPLETE` grant, because `complete` will refuse
  `APPROVAL_MISSING`. The claim still succeeds; the warning reads the claimed ticket file only when
  its digest equals the attempt's `ticketRecordSha256`.
- `ATR-V0-275`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1438) The
  CAL-V0-250 `REQUEST_ID_CONFLICT` explanation MUST also rebuild the request with the `recordedAt` of
  the receipt the request-index entry names (and up to two seconds before it), which equals a
  defaulted issuedAt and survives later updates, before the ticket's `createdAt` and `updatedAt`.
  It still names issuedAt only when a rebuilt digest equals the recorded one, and never replays or
  writes. Successful mutation results do not carry `issuedAt`: the result wire contract stays
  closed, and output that was lost would lose that member too, so recovery goes through the
  recorded receipt instead.
Non-goals: `INTEGRATE` grants (supervised programs only); granting approvals automatically; a
lease that revives after expiry; adding `issuedAt` to mutation results; recovering issuedAt when
the named receipt is unreadable (the explanation falls back to the generic digest inputs).
Failure modes: a compacted or missing receipt yields no receipt base, so recovery falls back to
`createdAt` and `updatedAt` as before; a ticket file whose digest no longer matches the claim omits
the grant warning rather than guessing.
Acceptance evidence: `TestATRV0270_DispatchUsageNamesGrantID`,
`TestATRV0271_GateRunAfterExpiryNamesRemedy`, `TestATRV0272_ApprovalGrantsNamedUpFront` (covers
ATR-V0-272..274) and `TestATRV0275_LostOutputRetryAfterLaterUpdateRecovers` (`internal/tasks/cli`);
each fails with its fix removed.
Rollback: revert the commit. The usage line, the expiry refusal and the claim blocker return to
their earlier text, `missingGrants` and the claim grant warning disappear, and the conflict
explanation again tries only `createdAt` and `updatedAt`; no stored record changes.

### V1-0826 supervised stage worktree cleanup (ATR-V0-296..298)

V1-0826: `CleanupWorktrees` had no CLI caller, and an `OpenWorkflow` reassign set the program's
worktree records to nil, so every prior-assignment stage worktree stayed on disk and in Git's
worktree registry with no record left that could find or remove it.
- `ATR-V0-296`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0826)
  `corvint-tasks program cleanup --program ID --config FILE` MUST remove a recorded program's
  registered stage worktrees only when no live attempt can reference them: the program is
  `FINISHED` behind a terminal attempt whose quiescence is `PROVED`, or `ADMITTED` with no current
  attempt. Any other state refuses `ATTEMPT_LIVE` before a write. It takes program ownership as
  `OpenWorkflow` does, refused while another owner lives, and never reassigns, claims or attaches
  work. A worktree is removed only when its identities match the record, its HEAD is the recorded
  commit and its tree is clean; each removal is bracketed by a `WORKTREE_REMOVE` phase write. A
  worktree that fails a check, or whose `git worktree remove` fails, stays registered and is reported
  with its reason, and cleanup continues with the others. The result has one item per unremoved
  record: `path`, `repository` for an extra-repository sibling, `removed` and `reason`.
- `ATR-V0-297`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0826) A reassign
  MUST first remove the prior assignment's stage worktrees under the ATR-V0-296 checks, and MUST keep
  the records of the worktrees it could not remove, so `program show` reports them and
  `program cleanup` can remove them later. Removed records are dropped at the reassign.
- `ATR-V0-298`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0826) The program
  transition MUST admit `ADMITTED` to `WORKTREE_REMOVE` and back only while neither the recorded nor
  the next program has a current attempt.
Non-goals: removing a dirty, moved or re-pointed worktree; deleting candidate refs or journal
hand-offs; resuming a removal interrupted after its `WORKTREE_REMOVE` write (the program stayed in
that phase; V1-2368 adds that settlement as ATR-V0-323, and ATR-V0-324
drops removed records at each cleanup rather than only at the reassign); automatic cleanup outside
a reassign.
Failure modes: a kept worktree is reported, never removed by force; a refused or failed phase write
stops the cleanup with that error, and the worktrees already removed stay marked removed.
Acceptance evidence: `TestATRV0297_ReassignRemovesPriorStageWorktrees` (covers ATR-V0-296..298) and
`TestATRV0296_CleanupRefusesLiveAttempt` (`internal/tasks/store`), and
`TestATRV0296_ProgramCleanupRefusalsDoNotMutate` (`internal/tasks/cli`). The reassign test fails
with the reassign change reverted (no record kept) and with the ATR-V0-298 transition removed
(`ADMITTED -> WORKTREE_REMOVE` refused).
Rollback: revert the commit. A reassign again drops every worktree record, `program cleanup`
disappears, and the idle-removal transition is refused; worktree records already kept stay valid
program records.

### V1-2305, V1-1559 and V1-2173 cli test workers, the attach race and detached equivalence

V1-2305: the dead-man reaper of ATR-V0-213 covers only the `internal/tasks/dispatch` test binary, and
two `host.sh` workers launched by tests of `internal/tasks/cli` survived 9 to 12 hours with parent 1.
V1-1559: the detach tests had run on Darwin only, and an attach was seen to report a detached claim
`LOST` for a brief moment while its launcher was still starting the child. V1-2173 (the follow-up
of the V1-1557 decision, `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-detach-claim-semantics-and-prune.md`): `--detach`
is a transport and adds no store semantics, which nothing yet stated or tested.

- `ATR-V0-310`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2305) The
  `internal/tasks/cli` test binary MUST fail when a test leaves a process group running under a
  child of the binary, whichever way the test or the binary ends, and MUST signal a worker process
  only through a handle that pins that process incarnation against pid reuse (a pidfd on Linux);
  no path signals a pid or a process group by number. (1) A test that registers the per-test reaper
  (`ReapWorkersOnCleanup`, called before the test's own cleanups) MUST fail when it ends, including
  after `t.Fatal`, naming every group led by a child of the binary that was not running at
  registration, and MUST retire those it owns. (2) After the last test the binary MUST join,
  within a bound and without signalling, every detached child the tests launched (a detached
  result is readable before its child exits), then retire any group still led by one of its
  children and fail the package. Fixtures that launch detached children MUST join them before the
  test's cleanup ends. (3) Before any test the binary MUST start a dead-man reaper as ATR-V0-213
  does (a copy of itself, standard input a pipe whose write end only the binary holds), as the
  leader of its own process group held unreaped by the binary. Every sample, the reaper MUST
  attribute a process only from a process-table row read after the process's handle is opened,
  with the handle still pinned after the read, so the row is that process's: a group leader when
  the row names the binary as its parent while the binary's own handle is pinned; a member when
  the row puts it in the group of an owned leader that is still pinned, or under an owned, pinned
  parent in that group. An owned incarnation that has been reaped MUST be forgotten, so a later
  process under its pid is judged afresh. At end of file, which the kernel delivers once the binary
  has exited by any means, including SIGKILL and the package timeout, it MUST stop each owned
  leader, kill the owned members, sampling again for members forked meanwhile until none is left
  within a bound, then kill the leader, each through its handle, and MUST exit nonzero, reporting
  on the binary's standard error, when it killed or left anything or a sample failed. The per-test
  and end-of-run checks retire the same way. (4) A failed process-table read, a handle that cannot
  be opened or checked, a signal refused for any reason other than the process having been reaped
  (`EPERM` included), a process in an owned group's number that cannot be attributed, and an
  attributed leader that cannot be pinned MUST fail the check and MUST NOT be taken for gone or
  signalled. Where the platform has no pinning handle (Darwin) nothing is signalled: every leak is
  reported as not signalled and fails. (5) At shutdown the binary MUST close the pipe and wait,
  within a bound, for the reaper; a reaper still running at the bound MUST be killed with its whole
  group while its leader is unreaped; it MUST be reaped before the binary exits, and a reaper that
  failed or had to be killed MUST fail the package. A test binary re-executed by a test as a helper
  MUST NOT arm these checks unless the test clears `CORVINT_CLI_TEST_ROOT` for it.
- `ATR-V0-311`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1559) An attach
  of a detached `claim` or `pool acquire` MUST NOT report `LOST` while a launcher of its request is
  starting a child, nor while one could begin before the answer is chosen. A launcher holds the
  launch lock exclusively from before it starts a child until `child.json` records it, so an
  attach MUST take the launch lock shared, without waiting, before it reads the child's liveness,
  and hold it until it has chosen its answer; when the lock is held it MUST answer `PENDING`. It
  MAY report `LOST` only while holding the lock, with no child live at its liveness read and no
  kept result. A missing lock file counts as no launcher (the launcher creates it before it writes
  `request.json`); the attach opens it read-only and creates nothing. This refines ATR-V0-027 and
  changes no other attach state.
- `ATR-V0-312`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2173) A detached
  `claim` or `pool acquire` MUST have the store outcome of the foreground command with the same
  argv and request ID in the same store state: the kept result returned by `--detach --wait` and by
  `--attach` MUST be the same exit code and envelope (outcome, codes, warnings and items), and
  `--detach` MUST add no store semantics. For a holder whose live attempt has a different request
  ID, a detached claim therefore gets what the foreground claim gets: a second attempt on another
  ticket, `ATTEMPT_LIVE` on the ticket the live attempt holds, and the idempotent replay for its own
  request ID.

Non-goals: signalling on a platform without a pinning handle (Darwin reports and fails instead);
worker groups a process moves out of its group; a group started within one sample (200 ms) of the
binary's death, which only the per-test and end-of-run checks cover; a reaper that is itself
killed; other test packages; pruning of detached records that never finish (V1-1565). Failure
modes: a leaked group that does not fail its test, the package or the dead-man reaper, or that
survives on Linux, fails `TestATRV0310_LeakedWorkerGroupsAreReaped`; attribution that keeps a
reaped incarnation, reads membership from a row not bound to the handle, takes a reused group
number for an owned group, or takes a failed read, open or `EPERM` for gone fails
`TestATRV0310_ReusedWorkerPIDIsTrackedByIncarnation`, `TestATRV0310_MembershipIsReadFromThePinnedProcess`,
`TestATRV0310_NoSignalWithoutAPinnedOwner` or `TestATRV0310_UnobservableCleanupFails`; a stalled
reaper left running with its group fails `TestATRV0310_StalledReaperIsRetiredWithItsGroup`; a
completed detached claim whose child is still exiting at the leak check fails
`TestATRV0310_DetachedChildrenAreJoinedBeforeTheLeakCheck`; an attach that reports `LOST` while a
launch is under way or begins during its decision fails
`TestATRV0311_AttachNeverReportsALaunchCompletingBetweenItsReadsLost`,
`TestATRV0311_LateRelaunchIsNeverReportedLost` or `TestATRV0311_AttachDuringARelaunchIsPending`; a
detached outcome that differs from the foreground one fails
`TestATRV0312_DetachedClaimOutcomeEqualsForeground`.
Acceptance evidence, recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-cli-test-lane-1023.md`, is of three
kinds. Regression witnesses fail on the named earlier code:
`TestATRV0311_LateRelaunchIsNeverReportedLost` (its first case answers `SUPERVISOR_LOST` with the
two-read attach of this change's first review round; the liveness-first attach on `origin/main`
passes it, as it reads liveness once), `TestATRV0311_AttachNeverReportsALaunchCompletingBetweenItsReadsLost`
(with the original liveness-first read order), `TestATRV0310_LeakedWorkerGroupsAreReaped` (with
the reaper layers disabled) and `TestATRV0310_DetachedChildrenAreJoinedBeforeTheLeakCheck` (with
the joins removed). Mutant checks name the defect each reaper unit test detects: keeping a
reaped incarnation (`ReusedWorkerPIDIsTrackedByIncarnation`), reading membership from the
pre-handle snapshot (`MembershipIsReadFromThePinnedProcess`), taking group-number presence as
ownership (`NoSignalWithoutAPinnedOwner`), treating a failed table read or `EPERM` as gone
(`UnobservableCleanupFails`); `StalledReaperIsRetiredWithItsGroup` is witnessed by the previous
shutdown, which left the stalled reaper's child running. Coverage additions pass on the previous
implementation and guard existing behaviour: `TestATRV0312_DetachedClaimOutcomeEqualsForeground`
(mutant: a detached child that rewrites `ATTEMPT_LIVE` fails it),
`TestATRV0311_AttachDuringChildStartupIsNeverLost` and `TestATRV0311_AttachDuringARelaunchIsPending`
(end-to-end and state checks). Linux runs of the detach and reaper tests are recorded there too.
Rollback: revert the commit; the attach reads liveness before the lock again, the cli test binary
has no reaper or detached-child join, `groupreap.ReadProcessTable` is removed, and ATR-V0-020..029
are otherwise unchanged.

### V1-2368 interrupted worktree removal and removed-worktree records

Re-homed (owner rule 2026-10-17; batch 1023a): these requirements were first numbered
CAL-V0-365..366 in the agent leases spec; they moved here as ATR-V0-323..324 with the same text,
because that spec's `## Requirements` section is at the 256-obligation OCM bound. The old IDs are
retired aliases: code comments, messages and test names keep the old numbers.

Human-owned input: native ticket V1-2368, the residual of V1-0826 (ATR-V0-296..298, PR #1531). A
`program cleanup` or reassign interrupted after its `WORKTREE_REMOVE` write left the program in
that phase: `cleanupPhase` refused it `ATTEMPT_LIVE`, and a replacement owner's takeover was refused
`FENCED` (prior program owner not proved stopped) because `WORKTREE_REMOVE` was not a safe takeover
phase, so the program stayed stuck until manual repair. Each removed worktree also kept its record,
marked removed, for the rest of the assignment.

- `ATR-V0-323`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2368) A program
  recorded in `WORKTREE_REMOVE` behind a terminal attempt with proved quiescence, or with no
  current attempt, MUST be settled by the next `program cleanup` or `OpenWorkflow` (reassign or
  run) before anything else: when the worktree path the program names is gone, the removal ran and
  its record is dropped (`REMOVAL_COMPLETED`, `removed` true, with a reason naming `git worktree
  prune` when Git still lists the worktree, since `git worktree remove` deletes the work tree before
  its registration); otherwise nothing was removed and the record stays for the ATR-V0-296 checked
  retry (`REMOVAL_ROLLED_BACK`, with a reason when Git no longer lists the remaining path). Git's
  listing is matched against the recorded path and against it with its parent's symlinks resolved. The settling write
  returns the program to `FINISHED` or `ADMITTED` and, when the recorded owner is gone, is the
  takeover itself: the program transition MUST admit an owner change from `WORKTREE_REMOVE` to
  `FINISHED` (proved terminal attempt, unchanged worktree) or `ADMITTED` (no attempt) with the epoch
  advanced by one. `program cleanup` reports the settled worktree first, with `takeover` set to the
  outcome; every other item has no `takeover`. Any other phase is refused as before.
- `ATR-V0-324`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2368) A cleanup
  MUST drop the record of each worktree it removes instead of marking it removed, and MUST drop
  records an older binary marked removed at its first phase write, so a program's worktree records
  hold only worktrees that may still exist and repeated cleanups within one assignment neither grow
  them nor report a removed worktree again. This amends ATR-V0-297, which dropped removed records
  only at the reassign.

Non-goals: settling a removal while a live attempt may still reference the worktree; removing a
worktree that fails the ATR-V0-296 checks; a typed outcome for an interrupted `WORKTREE_ADD`.

Failure modes: a `git worktree list` that fails refuses the settlement with its error and the
program stays in `WORKTREE_REMOVE`; a worktree whose path remains but which Git no longer registers
is rolled back with a reason and then kept by the checked retry, never removed by force. A binary
without ATR-V0-323 still refuses a parked program; one without ATR-V0-324 reads the shorter records
unchanged.

Rollback: revert the source. Programs settled or compacted by this change keep valid records; a
program parked in `WORKTREE_REMOVE` is again stuck until manual repair.

### PR #1631 review: exclusive, proved worktree removal

Re-homed (owner rule 2026-10-17; batch 1023a): these requirements were first numbered
CAL-V0-367..371 in the agent leases spec; they moved here as ATR-V0-325..329 with the same text,
because that spec's `## Requirements` section is at the 256-obligation OCM bound. The old IDs are
retired aliases: code comments, messages and test names keep the old numbers.

Human-owned input: the independent reviews of PRs #1629, #1630 and #1631 (2026-10-23) found that the
ATR-V0-323 takeover could replace a live cleanup, re-run a removal while the dead owner's Git child
still ran, forget a moved worktree, retry a rolled-back removal in the same invocation, and keep
legacy removed records stored. These requirements amend ATR-V0-323 and ATR-V0-324.

- `ATR-V0-325`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2368) A cleanup MUST
  hold its program exclusively before any filesystem removal: its takeover write and the write that
  enters `WORKTREE_REMOVE` MUST record `ownerReleased` false, whatever a completed stage left, and
  the transition MUST refuse a `WORKTREE_REMOVE` entry that records a released owner. While that
  owner lives, another cleanup or `OpenWorkflow` is refused `ATTEMPT_LIVE` (live owner) before it
  inspects or removes anything.
- `ATR-V0-326`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2368) The
  `git worktree remove` child MUST be started gated on its stdin and released only after the
  `WORKTREE_REMOVE` write records its process identity (`removerPid`, `removerStarted`, present only
  in that phase), so an owner that dies first leaves a child that exits without removing anything.
  A takeover of a `WORKTREE_REMOVE` program MUST first prove that child gone: a recorded child still
  running is refused `ATTEMPT_LIVE` naming `REMOVER_LIVE`, and a record with no child identity (an
  older binary's) `QUIESCENCE_UNPROVED` naming `REMOVER_UNRECORDED`; nothing is inspected or
  written in either case. The transition MUST refuse a `WORKTREE_REMOVE` entry without the identity.
- `ATR-V0-327`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2368) A cleanup
  MUST drop records an older binary kept marked removed in a durable write before any other work,
  also when it removes nothing else or every record is removed. That write keeps the phase, and the
  transition MUST admit it for an idle `ADMITTED` program (whose phase has no other
  self-transition) exactly when it changes nothing but dropping those records.
- `ATR-V0-328`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2368) A worktree a
  takeover settled as `REMOVAL_ROLLED_BACK` MUST NOT be removed again in the same invocation; it stays
  recorded for the next cleanup, so one invocation never reports two outcomes for one worktree.
- `ATR-V0-329`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2368) A missing
  worktree pathname MUST NOT by itself settle `REMOVAL_COMPLETED`: the record is dropped only when no
  other existing registered worktree of that repository has the recorded directory or private
  Git-directory identity. A match keeps the record as `REMOVAL_ROLLED_BACK` naming the new location,
  and a registered worktree whose identity cannot be read keeps it with that reason.

Non-goals: recovering a parked record written by an older binary without a remover identity (it
stays refused, as before ATR-V0-323); following a moved worktree to its new path; supervising
removal children beyond the gate and the recorded identity.

Failure modes: a remover that cannot start or whose identity cannot be read keeps the record with a
`worktree remove:` reason and writes nothing; a write that fails after the child started closes the
gate and reaps it; a killed child that has not yet been reaped keeps `REMOVER_LIVE` until it is;
an identity collision with an unrelated registered worktree keeps the record (the safe direction).

Rollback: revert the source. A program left in `WORKTREE_REMOVE` with `removerPid` is refused by the
older decoder (unknown field) until a current binary settles it, so settle parked programs first;
every other record is unchanged.

### V1-1634 host sleep on Linux and the other lease-reap paths at a wake

Re-homed (owner rule 2026-10-17; batch 1023a): these requirements were first numbered
CAL-V0-361..362 in the agent leases spec; they moved here as ATR-V0-320..321 with the same text,
because that spec's `## Requirements` section is at the 256-obligation OCM bound. The old IDs are
retired aliases: code comments, messages and test names keep the old numbers.

Human-owned input: native ticket V1-1634, the residual of V1-1597 (ATR-V0-063..065), which was tested
with an injectable clock on macOS only.

- `ATR-V0-320`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1634) Only the
  dispatcher's CAL-V0-191 reap of a running worker's expired lease, and the worker's wall,
  idle and lease-keeper `maxIdleSeconds` timers, exclude a recorded suspend gap (ATR-V0-064). Every
  other lease-reap path acts on the clock as it reads at the first tick or write after a wake and
  MUST NOT be gap-aware: the exit recovery of CAL-V0-104, which retries a refused hand-off at the
  first tick after the wake (its backoff is a wall-clock deadline that the suspend passed) and
  reaps at once an attempt whose lease ran out, because no worker is left to use the time; the
  dispatcher's reap of an expired lease that no worker of its own holds; and the store's own
  expiry judgement, the lease timestamp against the clock of the writing call, which a colliding
  claim, `claim-next` and the `reap` verb apply, so that a lease that ran out while its holder's
  host slept is reaped by the first such writer after the wake and the holder's later `renew` or
  `heartbeat` is `FENCED`. The store keeps no suspend record: a gap is one dispatcher's local
  observation of its own ticks (ATR-V0-063). A holder that needs its lease to survive a suspend
  renews before it (`keepLease`, CAL-V0-311) or accepts the reap.
- `ATR-V0-321`: (accepted under the owner's standing auto-accept (2026-10-11), V1-1634) The
  host-sleep detector (ATR-V0-063) MUST rest on a monotonic clock that does not count suspended
  time. On Linux the Go runtime's monotonic clock, the reading `time.Now` carries, is
  `CLOCK_MONOTONIC`, which stops during a suspend, and not `CLOCK_BOOTTIME`, which runs through
  it; a forward step of `CLOCK_REALTIME` that leaves that reading behind, the signature of a suspend
  to the detector, MUST be recorded once, by a dispatcher on the real clocks, as one gap of the
  step's size.

Non-goals: extending a lease for a gap (ATR-V0-063 already declines it), a store-side suspend record,
a doctor finding or a keep-awake option. A real suspend is not qualified here: the container has no
suspend state (`/sys/power/state` reads empty and there is no RTC), so the Linux qualification is a time-namespace proof
of the clock choice and, with `CORVINT_TASKS_CLOCK_STEP=1`, a wall-clock step of the same signature.

Failure modes: a suspend that ends before a second tick of the same dispatcher process is not seen
(ATR-V0-063); an armed idle gate (CAL-V0-139, only a dispatcher with no live worker arms) measures
its 60 s safety net in monotonic time, so the first full tick after a wake, which records the gap,
can come up to 60 s of awake time late, while a lease expiry that the suspend crossed is a
wall-clock deadline and ends the skip at once.

Rollback: revert the tests and this subsection; no behaviour changes.

### V1-0911 Linux qualification of the Tasks scale work: the process table

Re-homed (owner rule 2026-10-17; batch 1023a): this requirement was first numbered CAL-V0-363 in the
agent leases spec; it moved here as ATR-V0-322 with the same text, because that spec's
`## Requirements` section is at the 256-obligation OCM bound. The old ID is a retired alias: code
comments, messages and test names keep the old numbers.

Human-owned input: native ticket V1-0911. The scale work (V1-0893, V1-0645; CAL-V0-135..142) was
measured on Darwin only; the `/proc` process reader (CAL-V0-136), the idle CPU of the supervisor and
dispatcher (CAL-V0-137, CAL-V0-139) and the `ReadDir` listing that relies on directory-entry types
(CAL-V0-138) were not run on Linux.

- `ATR-V0-322`: (accepted under the owner's standing auto-accept (2026-10-11), V1-0911) On a real
  Linux `/proc` the process-table reader of CAL-V0-136 MUST return the pid, parent, process group
  and zombie state that `ps -axo pid=,ppid=,pgid=,stat=` returns for a live tree: a worker in its
  own process group, an unreaped zombie child and an orphan reparented after its parent exited.

Non-goals (ATR-V0-322): new product behaviour. The other Linux measurements (idle CPU, scan cost,
listing cost) are build-log evidence, not obligations, because a duration is not asserted.

Failure modes (ATR-V0-322): a host without `ps` or without `/proc` skips the comparison.

Rollback (ATR-V0-322): revert the test.

### V1-2649 per-ticket working directory (issue 1754)

Human-owned input: native ticket V1-2649 and issue 1754. An adopter runs OpenCode workers through
the dispatcher against a multi-repository product: the Tasks store lives in one repository and each
ticket's code lives in a ticket worktree of another. Every worker started in `workRoot`, so its
session, its shell and the Corvint MCP server it configured all pointed at the store repository.
The dispatcher had no way to start a ticket's worker in that ticket's worktree.

- `ATR-V0-340`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2649) A ticket
  role MAY declare `cwd {template, fallback, label?}`. `template` is 1..4096 bytes, may hold only
  `{worktree}`, `{workRoot}`, `{ticketLocal}`, `{role}` and `{program}`, and must render to a clean
  absolute path. `fallback` is `WORK_ROOT` or `REFUSE`. `label`, when set, is a ticket label prefix
  of 1..63 bytes with no TAB, LF or CR and requires a template that holds `{worktree}`.
  Other values, and a lane role that declares `cwd`, are refused at configuration load. Each ticket
  launch of the role starts in the rendered directory when it names an existing directory. A role
  without `cwd`, and every lane launch, starts in `workRoot` with no new argv, environment or event
  detail.
- `ATR-V0-341`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2649)
  `{worktree}` resolves for the launched ticket from the first of: the worktree of the ticket's live
  attempt (`LIVE_ATTEMPT`); the existing worktree that has the policy's `worktrees` branch for the
  ticket checked out inside the declared repository root (`TICKET_WORKTREE`, TWT-V0-005); the
  remainder of the single ticket label that starts with `label` (`TICKET_LABEL`), joined to
  `workRoot` when relative and refused when it is not local. Each candidate must be a clean absolute
  existing directory. Two labels with the prefix resolve nothing. A template without `{worktree}`
  has the source `TEMPLATE`. The dispatcher only looks a worktree up; it never creates, moves or
  removes one.
- `ATR-V0-342`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2649) When the
  template does not render to an existing directory, `WORK_ROOT` starts the launch in `workRoot`
  with the source `FALLBACK_WORK_ROOT`, and `REFUSE` fails the launch before anything is spawned:
  the pool hold is returned, the relaunch marker is kept, the launch cools down as any other
  pre-spawn failure, and the `launch-failed` event carries `code: CWD_UNRESOLVED`.
- `ATR-V0-343`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2649) The
  `launched` and `launch-failed` events of a role with `cwd` record `cwdSource` (one of
  `LIVE_ATTEMPT`, `TICKET_WORKTREE`, `TICKET_LABEL`, `TEMPLATE`, `FALLBACK_WORK_ROOT`, `NONE`) and,
  when a directory resolved, `cwd`.
- `ATR-V0-344`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2649) Host argv
  and environment MAY hold `{cwd}`, the launch directory, which is rechecked as a store-derived
  value (ATR-V0-133). A launch of a role with `cwd` also receives `CORVINT_DISPATCH_CWD`. A host
  that starts or configures `corvint-mcp` passes `--root {cwd}` so the server serves the ticket
  worktree; a linked worktree is a supported `--root`, and only the MMR-V0-009 notice applies.

Non-goals (ATR-V0-340..344): creating, checking out or cleaning up worktrees (TWT-V0 owns them);
running `corvint-tasks` from the worker's directory when it is outside the store repository (Tasks
resolves the store from its working directory, so such a worker runs Tasks commands from
`{workRoot}`); per-ticket directories for lane roles; a strict MCP profile, which does not exist.

Failure modes (ATR-V0-340..344): an invalid `cwd` refuses the configuration; a missing, ambiguous,
non-local or deleted worktree falls back or refuses as declared; a `REFUSE` launch retries after the
pre-spawn cooldown; `{cwd}` rendered into JSON assumes a path without quote characters.

Rollback (ATR-V0-340..344): revert the commit. Configurations that declare `cwd` are then refused
as holding an unknown field; configurations without it are unchanged. No record or wire shape
outside the two event details changes.

### V1-2694 structured outcome of a finished worker (issue 1799)

Human-owned input: native ticket V1-2694 and issue 1799. A worker stopped by a signal (exit 130 or
143) was reported as `finished ... (exit 130, no progress): <fragment>`, where the fragment was the
tool output the stop interrupted, although the session had made a commit two minutes earlier. A
consumer that holds a ticket after N sessions without progress had to read free text and held live
tickets on false positives. V1-1844 keeps `finished` as the event kind and extends it additively.

- `ATR-V0-350`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2694) A
  `finished` event's detail carries `exitCode` (unchanged: the leader's exit code, `-1` when a
  signal ended it, `NOT_OBSERVED` when its exit was not observed), `exitSignal` (the signal the
  leader's wait status reports, else `SIG<name>` for an exit code 129..159 under the shell's 128+N
  convention, else `NONE`; `NOT_OBSERVED` with an unobserved exit) and `stopReason`. `stopReason`
  is the dispatcher's own stop first: `context-limit` (CONTEXT), `lease-expired` (LEASE_EXPIRED),
  `idle-timeout` (IDLE) or `wall-limit` (WALL); otherwise, from the exit: `completed` for exit 0,
  `operator-stop` for SIGHUP, SIGINT, SIGKILL or SIGTERM, `crash` for any other signal or nonzero
  exit, and `unknown` when the exit was not observed.
- `ATR-V0-351`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2694) The detail
  also carries facts the dispatcher observes itself. `commitCount` and `commits` (comma-separated
  full commit ids, newest first, at most 32, with `commitsTruncated: true` when there were more)
  are the commits reachable from `HEAD` in the worker's launch directory and not from the `HEAD`
  the dispatcher resolved there just before spawn; `commitCount` is `NOT_OBSERVED`, and `commits`
  absent, when the directory is not a Git repository or either resolution failed. `attemptRelease`
  is `held` when the final observation still shows a live attempt held by the worker, else
  `handed-off` when this dispatcher released one for it, else `reaped` when the worker was stopped
  because its attempt's lease expired and was reaped (CAL-V0-191), else `released` when this
  dispatcher observed it holding a live attempt that is gone, else `none`. `handoffNote` is `true` when a
  valid ATR-V0-104 note of the worker exists when the event is written, including the note the
  context stop salvages, which is now written before the event; its `context` event still follows.
- `ATR-V0-352`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2694) The
  accounting fields `progress` and `session` are unchanged: store-owned work state and the declared
  progress signal still decide backoff and parking. The human label, in the message and in
  `progressLabel`, is derived from that class and the ATR-V0-351 facts, never from the output
  stream: a session without recorded progress that made commits is `N commit(s), store state
  unchanged`, and a stop other than `completed` or `unknown` is named first, as `stopped by
  <SIGNAL> (<stopReason>)`, `stopped (<stopReason>)` or `failed (crash)`, so a stopped session is
  never labelled only `no progress`.
- `ATR-V0-353`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2694) The
  message's summary is never a mid-stream fragment. An event stream's unterminated last line is a
  write the stop interrupted and is skipped. A worker whose `stopReason` is not `completed` carries
  only the host's last completed structured final text (a streamed delta reply still open at the
  end of the stream is unfinished and does not count), or `(stopped before summary)` when there is
  none; a completed worker keeps the CAL-V0-058 summary, raw tail included.
- `ATR-V0-354`: (accepted under the owner's standing auto-accept (2026-10-11), V1-2694) The
  `finished` event schema, the keys above with the existing ones, is documented in the dispatch
  reference (`docs/TASKS-SUPERVISION.md`). Every change is additive: no existing key is removed or
  renamed, and the ledger and its version are unchanged.

Non-goals (ATR-V0-350..354): attributing commits in a launch directory several workers share (each
sees the others' commits; a role with `cwd`, ATR-V0-340, gives each ticket its own); commits made in
another directory or branch than the launch directory's `HEAD`; telling an operator's signal from a
steward's; counting commits as store progress, which stays project-owned authority.

Failure modes (ATR-V0-350..354): an unresolved launch `HEAD` or a failed listing leaves
`commitCount: NOT_OBSERVED`; a dispatcher restart keeps the launch commit (it is stored beside the
worker's logs) but forgets earlier hand-offs and holdings, so `attemptRelease` may read `none` for a
worker that spans a restart; an exit the dispatcher did not observe is `unknown` with the
placeholder summary.

Rollback (ATR-V0-350..354): revert the commit. The new detail keys disappear, the label returns to
the four class labels, a context-stop note is again written after the `finished` event, and the
`launch-head.json` file beside each worker's logs is no longer written or read.

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
When the caller's stderr is not a file, the runner refuses later command writes once it returns,
so output after a `HOLD`, which leaves the leader unreaped, never reaches the caller's writer. Group
survivors that keep writing may see `EPIPE` or `SIGPIPE`. A non-file stdin must not be reused after a
`HOLD` (ATR-V0-166).

| Case | Exit status | Envelope |
| --- | --- | --- |
| Command exited | its status | `OK` |
| Command killed by a signal it did not get from the runner | 128 + signal | `OK`, class `SIGNAL` |
| Timeout | 124 | `REFUSED`, `LIMIT_EXCEEDED` |
| Lost lease (pre-launch FENCED refusal, refused beat or final beat) | 125 | `REFUSED` with the writer's refusal code (`FENCED`, `TICKET_STATE`, ...; `MALFORMED` for a refusal without a code); FENCED is never inferred |
| Spawn failure | 126 | `ERROR`, `NOEXEC` |
| Runner interrupted by SIGINT, SIGTERM, SIGHUP or SIGQUIT | 128 + the runner's signal | `REFUSED`; `childSignal` is the child's own (usually 9, the group SIGKILL) |
| Cleanup `HOLD` | 2 (outranks every other status) | `ERROR`, `SURVIVORS` |
| Outcome not recorded after a released run (after ATR-V0-051 retries) | 127 | `ERROR` plus the write's code, `UNCERTAIN_EFFECT` when the failure has no closed code (ATR-V0-160); one run item (ATR-V0-052) |
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
| Member forked while the group is stopped (Darwin's group SIGKILL can miss it) | The group SIGKILL repeats while a live member remains and the leader is held unreaped; a member that still outlives the bound is `CLEANUP_HOLD` (ATR-V0-153). |
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
| A retry claims a terminal attempt between the store read and the removal | The store read takes no lock, so the pass hides the runs it would remove and reads the terminal records again. A retry that revives the attempt before that second read keeps every run; one after it finds them already gone, as after a completed pass (ATR-V0-015, ATR-V0-171). |
| More than 1,024 attempt directories | A pass examines one window that resumes after the previous pass's, plus the attempts of the retained runs (ATR-V0-154), so a window full of live or absent attempts no longer stalls the attempts behind it. Names beyond the first 65,536 listed are not reached. |
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
| Carry-over record damaged, of another ticket or version, or not removable after progress | A damaged or other-ticket record is never offered and is replaced after one alert; a newer-version record is kept, never offered and alerted once; a failed removal alerts and stops the offer until a later write, which replaces the record, succeeds (ATR-V0-111, ATR-V0-112). |
| Pool-hold session killed, launch failed, or dispatcher crashed after a claim | The held allocation is returned before hand-off, on a pre-spawn failure, or by the orphan sweep; an already-ended attempt quarantined it natively; a crash before the hold record leaves it to the lease expiry (ATR-V0-122, ATR-V0-124). |
| Context notice file unwritable, not regular, or never polled by the session | Nothing is marked; one alert while it fails and a retry each tick; an unpolled file leaves the hard stop unchanged; a crash between append and marker repeats the line once (ATR-V0-125). |
| Review packet cannot be built or the rendered argv fails the recheck | The session launches without the packet and one alert names the reason (ATR-V0-141). |
| Queue observation not writable | The tick proceeds; one alert until a write succeeds; the doctor reports the previous file as stale after three ticks (ATR-V0-150). |
| Pressure missing or unknown with `capMax` set | The role keeps `cap` and no deep-queue streak is counted (ATR-V0-150, ATR-V0-152). |
| Outcome write fails without a closed code after launch | Exit 127 with `UNCERTAIN_EFFECT`, never `MALFORMED`; the warning keeps the error text (ATR-V0-160). |
| Dispatch configuration repeats a member | Refused, naming the member's JSON pointer (ATR-V0-161). |
| Ticket role `cwd` resolves no existing directory | `WORK_ROOT` starts in `workRoot` with source `FALLBACK_WORK_ROOT`; `REFUSE` fails before spawn with `CWD_UNRESOLVED`, returns the hold and cools down (ATR-V0-342). |

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

Per-ticket working directory (ATR-V0-340..344, V1-2649): the focused tests in Traceability on Darwin
and Linux; live OpenCode and Corvint MCP qualification in a real multi-repository product is NOT_RUN.

Structured finished outcome (ATR-V0-350..354, V1-2694): the focused tests in Traceability on Linux,
including a worker killed by SIGTERM after a commit; a live dispatcher with a real host is NOT_RUN.

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
| ATR-V0-026 | `internal/tasks/cli/lease_detach.go` | `TestATRV0026_WithoutDetachNothingChanges` (`internal/tasks/cli`); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-v1-1547-claim-detach.md` |
| ATR-V0-027 | `internal/tasks/cli/lease_detach.go` | `TestATRV0027_StartingChildIsNeverLost` (`internal/tasks/cli`) |
| ATR-V0-028 | `internal/tasks/cli/lease_detach.go` | `TestATRV0028_ChildLogIsCappedAndRotated` (`internal/tasks/cli`) |
| ATR-V0-029 | `internal/tasks/cli/lease_detach.go` | `TestATRV0029_PruneReachesEveryFinishedRecord` (`internal/tasks/cli`) |
| ATR-V0-030 | `internal/tasks/dispatch/config.go`, `internal/tasks/dispatch/context.go` | `TestATRV0030_ContextConfigIsExactAndBounded` (`internal/tasks/dispatch`); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-17-v1-1549-dispatch-context-thresholds.md` |
| ATR-V0-031 | `internal/tasks/dispatch/context.go`, `internal/tasks/dispatch/usage.go` | `TestATRV0031_ContextEstimateFromRecognisedLinesOnly` (`internal/tasks/dispatch`) |
| ATR-V0-032 | `internal/tasks/cli/dispatch.go`, `internal/tasks/dispatch/context.go` | `TestATRV0032_DispatchStatusShowsContextEstimate` (`internal/tasks/cli`) |
| ATR-V0-033 | `internal/tasks/dispatch/context.go`, `internal/tasks/dispatch/loop.go` | `TestATRV0033_CrossingsRecordedOncePerSession` (`internal/tasks/dispatch`); `TestCALV0131_LiveFormatsCoverEveryDecodedProfile` (`internal/tasks/cli`) |
| ATR-V0-034 | `internal/tasks/dispatch/loop.go` | `TestATRV0034_WorkerGetsContextFile` (`internal/tasks/dispatch`) |
| ATR-V0-035 | `internal/tasks/dispatch/context.go`, `internal/tasks/dispatch/loop.go`, `internal/tasks/dispatch/detached.go` | `TestATRV0035_HardStopHandsOffWithReason`, `TestATRV0035_UnknownEstimateNeverStops`, `TestATRV0035_DeferredAndRecoveredHandoffsKeepContext` (`internal/tasks/dispatch`) |
| ATR-V0-036 | `internal/tasks/dispatch/context.go`, `internal/tasks/dispatch/loop.go` | `TestATRV0036_ThresholdsHotReload` (`internal/tasks/dispatch`) |
| ATR-V0-037 | `internal/tasks/dispatch/context.go`, `internal/tasks/dispatch/usage.go` | `TestATRV0037_HiddenOrLostContextLinesAreUnknown`, `TestCALV0157_UsageVocabulariesAndStates` (`internal/tasks/dispatch`) |
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
| ATR-V0-064 | `internal/tasks/dispatch/host_sleep.go` (`slept`), `internal/tasks/dispatch/loop.go` (caps, `reapRunningExpired`), `internal/tasks/dispatch/keeper.go` | `TestATRV0063_SuspendIsRecordedAndExcludedFromTimers`, `TestATRV0063_NoGapLeavesTimersUnchanged`, `TestATRV0064_KeeperIdleCheckExcludesSuspend` (the lease keeper's `maxIdleSeconds`, V1-1634), `TestATRV0064_ExitRecoveryActsAtWake` (the exit recovery excludes no gap, ATR-V0-320) (`internal/tasks/dispatch`) |
| ATR-V0-065 | `internal/tasks/cli/dispatch.go` (`dispatchHostSleepValue`) | `TestATRV0065_DispatchStatusShowsLastHostSleep` (`internal/tasks/cli`) |
| ATR-V0-080 | `internal/tasks/intent/policy.go` (`ProgressSignal`, `DecodePolicy`), `internal/tasks/cli/command_help.go` | `TestATRV0080_PolicyProgressSignal` (`internal/tasks/intent`) |
| ATR-V0-081 | `internal/tasks/snapshot/attempt.go` (`LoopProgress`, `LoopEvidenceWith`, `readPriorLoop`), `internal/tasks/transaction/lease_claim.go` | `TestATRV0081_LoopEvidenceProgressRoundTrip`, `TestATRV0081_LoopEvidenceWith` (`internal/tasks/snapshot`), `TestATRV0081_ClaimRecordsProgressFingerprint` (`internal/tasks/transaction`) |
| ATR-V0-082 | `internal/tasks/transaction/loop_detect.go` (`LoopHoldOf`, `noProgressRun`, `progressChain`) | `TestATRV0082_LoopHoldHonoursProgressSignal` (`internal/tasks/transaction`), `TestATRV0082_CLIProgressSignal` (`internal/tasks/cli`) |
| ATR-V0-083 | `internal/tasks/ticket/view.go`, `internal/tasks/cli/plan.go` (`loopHoldValue`), `internal/tasks/dispatch/loop.go` (`loopEscalations`, `finish`) | `TestATRV0082_CLIProgressSignal` (`internal/tasks/cli`), `TestATRV0083_DispatcherEventsNameProgressSignal` (`internal/tasks/dispatch`) |
| ATR-V0-084 | `internal/tasks/dispatch/roster.go` (`baseFingerprint`), `internal/tasks/cli/dispatch.go` (`dispatchQueue.Observe`), `internal/tasks/snapshot/attempt.go` (`LatestHandoffEvidence`), `internal/tasks/transaction/lease_claim.go` | `TestATRV0084_FingerprintHonoursProgressSignal`, `TestATRV0083_DispatcherEventsNameProgressSignal` (`internal/tasks/dispatch`), `TestATRV0084_DispatchFingerprintCarriesLatestReference` (`internal/tasks/cli`), `TestATRV0084_LatestHandoffEvidence` (`internal/tasks/snapshot`) |
| ATR-V0-193 | `internal/tasks/snapshot/attempt.go` (`LoopEvidenceWith`, `priorValue`) | `TestATRV0193_RollbackReadableWithoutProgressSignal` (`internal/tasks/snapshot`); pre-#1085 reader experiment in the V1-1692 build-log entry |
| ATR-V0-085 | `internal/tasks/dispatch/config.go` (`BudgetOverride`, `Budget.UnmarshalJSON`, `decodeBudgetOverride`, `Budget.validate`, `TicketLimit`), `internal/tasks/dispatch/budget.go` (`SpendReportFor`, `recordBudget`), `internal/tasks/cli/dispatch.go` (`dispatchBudgetValue`) | `TestATRV0085_TicketBudgetOverridesDecode`, `TestATRV0085_OverridesWithoutDefaultValidate`, `TestATRV0085_UnmatchedTicketsNeverHeld` (`internal/tasks/dispatch`), `TestATRV0085_DispatchStatusListsOnlyOverrideScopes` (`internal/tasks/cli`) |
| ATR-V0-086 | `internal/tasks/dispatch/config.go` (`Budget.UnmarshalJSON`, `decodeBudgetOverride`, `validate`) | `TestATRV0086_MalformedOrAmbiguousOverridesRefuse`, `TestATRV0086_NullOverridesRefuse` (`internal/tasks/dispatch`) |
| ATR-V0-087 | `internal/tasks/dispatch/config.go` (`TicketLimit`), `internal/tasks/dispatch/budget.go` (`ticketLimit`, `observe`) | `TestATRV0087_EffectiveLimitPrecedence` (`internal/tasks/dispatch`) |
| ATR-V0-088 | `internal/tasks/dispatch/budget.go` (`recordBudget`, `SpendReportFor`), `internal/tasks/cli/dispatch.go` (`dispatchBudgetValue`, `dispatchBudgetTickets`) | `TestATRV0088_HoldsAndReportNameTheSelector` (`internal/tasks/dispatch`), `TestATRV0088_DispatchStatusShowsEffectiveLimitAndSelector` (`internal/tasks/cli`) |
| ATR-V0-089 | `internal/tasks/dispatch/budget.go` (`budget`, `budgetHeld`), `internal/tasks/dispatch/loop.go` (`tick`) | `TestATRV0089_RollingWindowPerEffectiveLimit` (`internal/tasks/dispatch`) |
| ATR-V0-095 | `internal/tasks/cli/lease_detach.go` (`leaseDetachKept`, `leaseDetachLaunch`) | `TestATRV0095_RelaunchWaitsOutTheFinishingChild`, `TestATRV0095_RunningWaitIsBoundedByWait`, `TestATRV0095_StaleStartingChildAfterRelaunchKeepsOneResult`, `TestV11721_LaunchLockWaitIsBoundedByWait`, `TestV11721_LaunchLockNeverTakenAfterTheBound`, `TestPSRV0111_CeilingRelaunchesOnSameRequestID` (`internal/tasks/cli`); `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-v1-1671-detach-relaunch-race.md` |
| ATR-V0-101 | `internal/tasks/cli/dispatch_context_hook.go` (`dispatchContextHook`), `internal/tasks/dispatch/context_delivery.go` (`ContextHookNotice`), `internal/tasks/cli/cli.go`, `internal/tasks/cli/command_help.go` | `TestATRV0101_WorkerWithoutDispatcherVerbReceivesNoticesThroughHook`, `TestATRV0101_ContextHookFailsQuietly` (`internal/tasks/cli`), `TestATRV0101_MarkerWriteFailureKeepsLevelDeliverable` (`internal/tasks/dispatch`) |
| ATR-V0-102 | `internal/tasks/dispatch/context_delivery.go` (`writeHookSettings`), `internal/tasks/dispatch/config.go` (`Placeholders`, `validate`), `internal/tasks/dispatch/loop.go` (launch) | `TestATRV0102_LaunchWritesHookSettingsAndPlaceholder`, `TestATRV0102_PlaceholderNeedsClaudeCode` (`internal/tasks/dispatch`) |
| ATR-V0-103 | `internal/tasks/dispatch/context_delivery.go` (`ContextDelivery`), `internal/tasks/cli/dispatch.go` (`dispatchStatusValueFor`) | `TestATRV0103_StatusReportsContextDelivery` (`internal/tasks/cli`) |
| ATR-V0-104 | `internal/tasks/dispatch/handoff_note.go` (`salvage`), `internal/tasks/dispatch/loop.go` (`finish`, launch) | `TestATRV0104_HardStopSalvagesBoundedScreenedNote`, `TestATRV0104_ScreensBeforeCutting`, `TestATRV0104_SymlinkedEvidenceRefused`, `TestATRV0104_EvidenceEnumerationBounded` (`internal/tasks/dispatch`) |
| ATR-V0-105 | `internal/tasks/dispatch/handoff_note.go` (`ReadHandoffNote`, `handoffNoteFor`), `internal/tasks/dispatch/loop.go` (launch), `internal/tasks/cli/cli.go` (`LiveFormats`) | `TestATRV0105_NextAttemptReadsSalvagedNote`, `TestATRV0105_FIFONoteDoesNotBlock`, `TestATRV0105_InheritedNoteEnvStripped` (`internal/tasks/dispatch`) |
| ATR-V0-106 | `internal/tasks/dispatch/context_exhausted.go` (`noteHardStop`), `internal/tasks/dispatch/context.go` (`ContextLimits`) | `TestATRV0106_ExhaustedAfterNHardStopsNotNMinusOne`, `TestATRV0106_ExhaustedAfterValidation`, `TestATRV0106_ReplayedFinishCountsOnce`, `TestATRV0106_ResetWithoutHardTokens` (`internal/tasks/dispatch`) |
| ATR-V0-107 | `internal/tasks/dispatch/context_exhausted.go` (`ReadContextExhausted`), `internal/tasks/cli/dispatch.go` (`dispatchContextExhaustedValue`), `internal/tasks/cli/cli.go` (`LiveFormats`) | `TestATRV0107_RecordVersions` (`internal/tasks/dispatch`), `TestATRV0107_DispatchStatusShowsContextExhausted` (`internal/tasks/cli`) |
| ATR-V0-110 | `internal/tasks/dispatch/carry_over.go` (`CarryOver`), `internal/tasks/dispatch/config.go` (`promptSafeHost`) | `TestATRV0110_CarryOverConfigDefaultsAndBounds` (`internal/tasks/dispatch`) |
| ATR-V0-111 | `internal/tasks/dispatch/carry_over.go` (`noteCarryOver`, `carryEndReason`, `carryMessage`), `internal/tasks/dispatch/loop.go` (finish) | `TestATRV0111_NoProgressSessionRecordsEndReasonAndTail`, `TestATRV0111_ProgressRemovesRecord`, `TestATRV0111_PlainOutputScreenedBeforeCut`, `TestATRV0111_AppendAfterFailedRemovalDropsOlderEntries`, `TestATRV0113_NextSessionPromptCarriesBoundedLabelledBlock` (`internal/tasks/dispatch`) |
| ATR-V0-112 | `internal/tasks/dispatch/carry_over.go` (`ReadCarryOver`, `CarryOverPath`), `internal/tasks/cli/cli.go` (`LiveFormats`) | `TestATRV0112_RecordVersions` (`internal/tasks/dispatch`), `TestCALV0131_EveryLiveFormatRefusesANewerVersion`, `TestCALV0131_LiveFormatsCoverEveryDecodedProfile` (`internal/tasks/cli`) |
| ATR-V0-113 | `internal/tasks/dispatch/carry_over.go` (`carryOverFor`, `carryBlock`, `writeCarryMarker`), `internal/tasks/dispatch/loop.go` (launch) | `TestATRV0113_NextSessionPromptCarriesBoundedLabelledBlock`, `TestATRV0113_NoCarryOverForOtherTicket`, `TestATRV0113_BlockBoundAcrossSessions` (`internal/tasks/dispatch`) |
| ATR-V0-114 | `internal/tasks/dispatch/carry_over.go` (`carryDetail`, `carriedCount`), `internal/tasks/dispatch/loop.go` (launch, finish) | `TestATRV0114_EventsMarkCarryOverSessions`, `TestATRV0113_NextSessionPromptCarriesBoundedLabelledBlock` (`internal/tasks/dispatch`) |
| ATR-V0-120 | `internal/tasks/dispatch/pool_hold.go` (`claimPoolHold`, `PoolHoldQueue`), `internal/tasks/dispatch/config.go` (`HoldPool`), `internal/tasks/dispatch/loop.go` (launch), `internal/tasks/cli/dispatch_pool_hold.go` (`PoolClaim`) | `TestATRV0120_HoldPoolIsOptIn`, `TestATRV0124_RefusedClaimKeepsNativeOrder` (`internal/tasks/dispatch`), `TestATRV0124_PoolClaimKeepsWaiterOrderCLI` (`internal/tasks/cli`) |
| ATR-V0-121 | `internal/tasks/dispatch/pool_hold.go` (`PoolHold`, `ReadPoolHoldFile`, `writePoolHold`), `internal/tasks/dispatch/loop.go` (launch env), `internal/tasks/cli/cli.go` (`LiveFormats`) | `TestATRV0122_KilledSessionLeavesNoOrphanedAllocation` (`internal/tasks/dispatch`), `TestCALV0131_EveryLiveFormatRefusesANewerVersion`, `TestCALV0131_LiveFormatsCoverEveryDecodedProfile` (`internal/tasks/cli`) |
| ATR-V0-122 | `internal/tasks/dispatch/pool_hold.go` (`releasePoolHolds`, `returnPoolHold`), `internal/tasks/dispatch/loop.go` (heal, launch), `internal/tasks/cli/dispatch_pool_hold.go` (`PoolRelease`) | `TestATRV0122_KilledSessionLeavesNoOrphanedAllocation`, `TestATRV0122_EndedAttemptIsNotReleasedAgain`, `TestATRV0122_FailedLaunchReturnsTheClaim` (`internal/tasks/dispatch`), `TestATRV0124_PoolClaimKeepsWaiterOrderCLI` (`internal/tasks/cli`) |
| ATR-V0-123 | `internal/tasks/cli/dispatch.go` (`dispatchStatusValueFor`) | `TestATRV0123_DispatchStatusShowsPoolHold` (`internal/tasks/cli`) |
| ATR-V0-124 | `internal/tasks/dispatch/pool_hold.go` (`releasePoolHolds`) | `TestATRV0124_OrphanedClaimIsReturned`, `TestATRV0124_RefusedClaimKeepsNativeOrder` (`internal/tasks/dispatch`), `TestATRV0124_PoolClaimKeepsWaiterOrderCLI` (`internal/tasks/cli`) |
| ATR-V0-125 | `internal/tasks/dispatch/context_delivery.go` (`deliverNoticeFile`, `ContextDelivery`, `ContextNoticePath`), `internal/tasks/dispatch/context.go` (`deliverNotices`), `internal/tasks/dispatch/config.go` (`ContextNotices`), `internal/tasks/dispatch/loop.go` (launch env) | `TestATRV0125_NoticeFileDeliversOnAnyHost`, `TestATRV0125_NoticeFileValidationAndRetry` (`internal/tasks/dispatch`) |
| ATR-V0-126 | `internal/tasks/cli/dispatch_salvage.go` (`dispatchSalvage`), `internal/tasks/dispatch/handoff_note.go` (`SalvageWorker`), `internal/tasks/cli/dispatch.go`, `internal/tasks/cli/cli.go`, `internal/tasks/cli/command_help.go` | `TestATRV0126_SalvageAttemptWritesScreenedNote`, `TestCALV0131_LiveFormatsCoverEveryDecodedProfile` (`internal/tasks/cli`), `TestATRV0125_NoticeFileDeliversOnAnyHost` (`internal/tasks/dispatch`) |
| ATR-V0-127 | `internal/tasks/dispatch/context_delivery.go` (`finishContextDetail`), `internal/tasks/dispatch/loop.go` (launch, `finish`) | `TestATRV0127_FinishedRecordsSalvage`, `TestATRV0125_NoticeFileDeliversOnAnyHost` (`internal/tasks/dispatch`) |
| ATR-V0-133 | `internal/tasks/dispatch/config.go` (`promptSafeHost`, `argvCodeRisk`, `renderedArgvSafe`, `carryArgvPlaceholders`), `internal/tasks/dispatch/loop.go` (launch) | `TestATRV0133_CarryOverHostArgvPlaceholders`, `TestATRV0133_LaunchRechecksRenderedArgv`, `TestATRV0110_CarryOverConfigDefaultsAndBounds` (`internal/tasks/dispatch`) |
| ATR-V0-134 | `internal/tasks/cli/dispatch.go` (`dispatchAux`) | `TestATRV0134_StatusFailsOnInvalidConfigFile`, `TestCALV0127_DispatchStatusWithInvalidConfigFile`, `TestATRV0056_DispatchStatusWorkerShowsContextNotice` (`internal/tasks/cli`) |
| ATR-V0-135 | `internal/tasks/cli/dispatch.go` (`dispatchConfig`) | `TestATRV0134_StatusFailsOnInvalidConfigFile` (`internal/tasks/cli`) |
| ATR-V0-141 | `internal/tasks/dispatch/review_packet.go` (`ReviewPacket`, `ReviewPacketSource`), `internal/tasks/dispatch/config.go`, `internal/tasks/dispatch/loop.go` (launch), `internal/tasks/cli/review_packet.go` (`dispatchQueue.ReviewPacket`) | `TestATRV0141_ReviewRolePromptCarriesPacket`, `TestATRV0141_ReviewPacketConfigBounds`, `TestATRV0141_PacketReadBeforePoolClaim` (`internal/tasks/dispatch`) |
| ATR-V0-150 | `internal/tasks/dispatch/admission.go` (`recordAdmission`, `trackAdmission`, `DecodeAdmission`), `internal/tasks/dispatch/roster.go` (`roster`), `internal/tasks/dispatch/loop.go` (`launchRoster`), `internal/tasks/cli/dispatch.go` (pool free members) | `TestATRV0150_RosterNamesFences`, `TestATRV0150_ObservationStreaks`, `TestATRV0150_RefusedLaunchStaysWaiting`, `TestATRV0150_OversizeHeldIsTruncated`, `TestATRV0150_WriteFailureAlertsOnce`, `TestATRV0150_PressureLabel`, `TestATRV0150_TickWritesObservation`, `TestATRV0150_DecodeAdmissionRefusals` (`internal/tasks/dispatch`); `TestCALV0131_EveryLiveFormatRefusesANewerVersion` (`internal/tasks/cli`) |
| ATR-V0-151 | `internal/tasks/dispatch/config.go` (`Role`, `validate`), `internal/tasks/dispatch/roster.go` (`roster`, `outsideFocusOnly`, `relaunchRole`) | `TestATRV0151_RunOnlyBypassFocus`, `TestATRV0151_ConfigValidation` (`internal/tasks/dispatch`) |
| ATR-V0-152 | `internal/tasks/dispatch/config.go` (`Role`, `validate`), `internal/tasks/dispatch/admission.go` (`effectiveCap`, `depths`) | `TestATRV0152_CapMaxScalesAtCalmPressure`, `TestATRV0151_ConfigValidation` (`internal/tasks/dispatch`) |
| ATR-V0-153 | `internal/tasks/cli/attempt_run.go` (`execute`), `internal/groupreap/owner.go` (`Primitives.RepeatKillWhileLive`, `waitPreReapQuiet`) | `TestOwnerRepeatKillWhileLiveSignalsOnlyBeforeReap`, `TestOwnerRepeatKillFailureHoldsWithoutReaping`, `TestOwnerRepeatKillRetiresMemberForkedDuringStop` (`internal/groupreap`), `TestATRV0004_InterruptStopsTheRun` (`internal/tasks/cli`) |
| ATR-V0-154 | `internal/tasks/cli/attempt_runs_retire.go` (`retireEndedRunsIn`, `runRetireWindow`, `readRunRetireCursor`, `writeRunRetireCursor`, `runWindowName`) | `TestATRV0154_RetirementSweepsABacklogBeyondOneWindow`, `TestATRV0154_DuplicateCarryIsMalformed`, `TestATRV0154_CursorWriteUsesItsOwnTemporaryFile`, `TestATRV0154_RetiredBacklogAdvancesTheCursor`, `TestATRV0015_SupervisorRetiresEndedRunsOfTerminalAttempts` (`internal/tasks/cli`) |
| ATR-V0-160 | `internal/tasks/cli/attempt_run.go` (`finish`), `internal/tasks/cli/export_test.go` (`SetAttemptOutcomeReportFault`) | `TestATRV0160_UnrecordedOutcomeIsNeverMalformed`, `TestATRV0004_UnrecordedOutcomeExits127`; accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes; fails with the fix reverted (`MALFORMED`) |
| ATR-V0-161 | `internal/tasks/dispatch/config.go` (`DecodeConfig`, `refuseRepeatedMembers`) | `TestATRV0161_RepeatedConfigMemberRefuses`; accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes; fails with the fix reverted (no refusal) |
| ATR-V0-162 | `internal/tasks/cli/dispatch.go` (`dispatchStatusValueFor`) | `TestATRV0162_DispatchStatusRendersForNilConfig`; accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes; fails with the fix reverted (nil-pointer panic) |
| ATR-V0-163 | `internal/tasks/dispatch/config.go` (`promptSafeHost`) | `TestATRV0163_LoadMatchesOnlyALiteralProgramName`; accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes; fails with the fix reverted |
| ATR-V0-164 | `internal/tasks/dispatch/config.go` (`noteSafeHost`, `promptSafeHost`, `argvCodeRisk`) | `TestATRV0164_HostCheckIsNameBased` (records the stated limit); accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes |
| ATR-V0-165 | `internal/tasks/dispatch/loop.go` (`carryUnsafe`, `carryUnsafeConfig`) | `TestATRV0165_UnsafeArgvAlertsOncePerTicketAndConfig`; accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes; fails with the fix reverted |
| ATR-V0-166 | `internal/tasks/cli/attempt_run.go` (`execute` stdin, `gatedWriter`) | stated contract, no maintained test (prose); accepted under the owner's standing auto-accept (2026-10-11) |
| ATR-V0-167 | `internal/tasks/dispatch/loop.go` (`admitProgress`, `accountOn`), `internal/tasks/dispatch/infraretry.go` (`infraEndedOn`) | `TestATRV0167_InterruptAfterProgressSaveKeepsAccounting`; accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes; fails with the fix reverted |
| ATR-V0-168 | `internal/tasks/dispatch/carry_over.go` (`CarryEntry.Fingerprint`, `noteCarryOver`, `carryOverFor`) | `TestATRV0168_ProgressInvalidatesCarryOverAcrossRestart`; accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes; fails with the fix reverted |
| ATR-V0-169 | `internal/tasks/dispatch/handoff_note.go` (`screen`, `markWrapped`), `internal/tasks/obligation/expected_fail.go` (`SecretScreenSpans`), `internal/secretscreen/secretscreen.go` (`MatchIndexes`) | `TestATRV0169_ScreenRedactsSecretsWrappedAcrossLines`; accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes; fails with the fix reverted |
| ATR-V0-170 | `internal/tasks/store/workflow.go` (`stage` read-only candidate check) | `TestATRV0170_ReadOnlyStageEditIsTypedAndRecoverable`; accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes; fails with the fix reverted |
| ATR-V0-171 | `internal/tasks/cli/attempt_runs_retire.go` (`retireEndedRunsIn` recheck, `retiringRunName`, `restoreRetiringRun`, `runWindowName`) | `TestATRV0171_RetryDuringRetirementKeepsItsRuns`; accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes; fails with the fix reverted |
| ATR-V0-180 | `internal/tasks/dispatch/reload.go`, `internal/tasks/dispatch/idle.go` | `TestCALV0350_FullTickReusesAnUnchangedConfigRead`, `TestCALV0350_IdleSkipEndsWhenTheCachedConfigExpires`, `TestCALV0127_ConfigReloadAppliesAndRefuses` (`internal/tasks/dispatch`) |
| ATR-V0-181 | `internal/tasks/dispatch/loop.go`, `internal/tasks/dispatch/pressure.go`, `internal/tasks/dispatch/idle.go` | `TestCALV0351_IdlePressureConfigSkipsTheSample`, `TestCALV0351_DeferredBudgetRefuses`, `TestCALV0351_PostIdleHoldLastsUntilThePendingLevelSettles`, `TestCALV0351_PostIdleCPUSampleWaitsForAFreshInterval`, `TestCALV0125_UnselectedCPULeavesLedgerUnchanged`, `TestCALV0068_DispatcherThrottlesNewLaunchesOnly` (`internal/tasks/dispatch`) |
| ATR-V0-182 | `internal/tasks/dispatch/worklogs.go`, `internal/tasks/dispatch/loop.go` | `TestCALV0352_RetirementSweepsABacklogBeyondOneWindow`, `TestCALV0144_ActivityDuringTheConfirmingPassKeepsDir`, `TestCALV0144_FailedConfirmingPassIsRetried`, `TestCALV0144_FinishedWorkerDirsAreRetired`, `TestCALV0144_IncompleteSessionReadIsRetaken`, `TestCALV0144_LiveMemberOnConfirmingPassKeepsDir`, `TestCALV0144_MarksAreBounded`, `TestCALV0144_ProtectedDirsTakeNoRetentionSlot`, `TestCALV0144_RemovalNeedsAConfirmingPass`, `TestCALV0144_RestartConfirmsLeftMarks`, `TestCALV0144_StaleMarkStartsOver`, `TestCALV0144_TicksConfirmMarksWithoutAnotherFinish` (`internal/tasks/dispatch`) |
| ATR-V0-183 | `internal/tasks/dispatch/reap_test.go` (`TestMain` leak check) | `TestCALV0353_CleanupReapsTheWholeWorkerGroup`, `TestCALV0353_UnmatchedLeaderIsReportedNotSignalled` and the package `TestMain` leak check (`internal/tasks/dispatch`) |
| ATR-V0-190 | `internal/tasks/transaction/attempt_detail.go`, `internal/tasks/transaction/lease.go`, `internal/tasks/mutation/apply.go`, `internal/tasks/cli/mutate.go` | `TestATRV0190_CompleteManualRefusalNamesExpiredAttempt` |
| ATR-V0-191 | `internal/tasks/transaction/attempt_detail.go`, `internal/tasks/cli/cli.go` | `TestATRV0190_CompleteManualRefusalNamesExpiredAttempt` |
| ATR-V0-192 | `internal/tasks/transaction/attempt_detail.go`, `internal/tasks/transaction/lease_claim.go` | `TestATRV0192_CollisionNamesHoldingAttempt` |
| ATR-V0-210 | `internal/tasks/transaction/lease_claim.go` (`requestedOutsideDeclared`) | `TestATRV0210_ClaimRefusesRequestedPathsOutsideDeclaredScope` (`internal/tasks/store`), `TestATRV0211_ClaimResultListsEffectiveScope` (`internal/tasks/cli`) |
| ATR-V0-211 | `internal/tasks/store/lease.go`, `internal/tasks/cli/lease.go` (`claimScopeValue`), `internal/tasks/cli/command_help.go` | `TestATRV0211_RequestedScopeWithoutDeclaredPathsIsReported`, `TestATRV0210_ClaimRefusesRequestedPathsOutsideDeclaredScope` (`internal/tasks/store`), `TestATRV0211_ClaimResultListsEffectiveScope` (`internal/tasks/cli`) |
| ATR-V0-212 | `internal/tasks/cli/read_only.go`, `internal/tasks/cli/cli.go` | `TestATRV0212_ReadOnlySessionRefusesMutations` (`internal/tasks/cli`) |
| ATR-V0-240 | `internal/tasks/store/program.go` (`programTransition` `keepControl`), `internal/tasks/store/workflow.go` (`persistFenced`, stage journal callback), `internal/tasks/store/program_run.go` (`record`) | `TestATRV0240_PhaseWriteKeepsControlRecordedMeanwhile`; accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes; fails with the fix reverted |
| ATR-V0-241 | `internal/tasks/store/lease_write.go` (recorded limit; no runtime change) | `TestATRV0241_MappedWriteAfterLeaseAuditRefusedByNextWrite`; accepted under the owner's standing auto-accept (2026-10-11); Darwin focused run passes; Linux NOT_RUN |
| ATR-V0-213 | `internal/tasks/dispatch/reap_test.go` (`TestMain` dead-man reaper) | `TestATRV0213_FixtureWorkersDieWithTheirTestBinary` (`internal/tasks/dispatch`) |
| ATR-V0-310 | `internal/tasks/cli/worker_reap_internal_test.go` (`TestMain` dead-man reaper, `reapTracker`, `ReapWorkersOnCleanup`), `worker_reap_linux_internal_test.go` (pidfd pins), `detach_join_internal_test.go`, `internal/tasks/cli/lease_detach.go` and `attempt_detach.go` (`detachLaunched`), `internal/groupreap/table.go` | `TestATRV0310_LeakedWorkerGroupsAreReaped`, `TestATRV0310_ReusedWorkerPIDIsTrackedByIncarnation`, `TestATRV0310_MembershipIsReadFromThePinnedProcess`, `TestATRV0310_NoSignalWithoutAPinnedOwner`, `TestATRV0310_UnobservableCleanupFails`, `TestATRV0310_StalledReaperIsRetiredWithItsGroup`, `TestATRV0310_DetachedChildrenAreJoinedBeforeTheLeakCheck` (`internal/tasks/cli`) |
| ATR-V0-311 | `internal/tasks/cli/lease_detach.go` (`leaseDetachAwait`), `internal/tasks/cli/lease_detach_unix.go` (`leaseDetachShareLaunch`) | `TestATRV0311_AttachNeverReportsALaunchCompletingBetweenItsReadsLost`, `TestATRV0311_LateRelaunchIsNeverReportedLost`, `TestATRV0311_AttachDuringARelaunchIsPending`, `TestATRV0311_AttachDuringChildStartupIsNeverLost` (`internal/tasks/cli`) |
| ATR-V0-312 | `internal/tasks/cli/lease_detach.go` (no detach-specific store semantics) | `TestATRV0312_DetachedClaimOutcomeEqualsForeground` (`internal/tasks/cli`) |
| ATR-V0-270 | `internal/tasks/cli/command_help.go` | `TestATRV0270_DispatchUsageNamesGrantID` (`internal/tasks/cli`) |
| ATR-V0-271 | `internal/tasks/transaction/lease.go` (`fenced`, `expiredRemedy`) | `TestATRV0271_GateRunAfterExpiryNamesRemedy` (`internal/tasks/cli`) |
| ATR-V0-272 | `internal/tasks/ticket/view.go` (`grantState`, `MissingGrants`) | `TestATRV0272_ApprovalGrantsNamedUpFront` (`internal/tasks/cli`) |
| ATR-V0-273 | `internal/tasks/ticket/view.go` (`MissingGrants`), `internal/tasks/cli/cli.go` (`ticketShow`) | `TestATRV0272_ApprovalGrantsNamedUpFront` (`internal/tasks/cli`) |
| ATR-V0-274 | `internal/tasks/store/claim_delivery.go` (`claimGrantWarnings`), `internal/tasks/store/lease.go`, `internal/tasks/cli/lease.go` | `TestATRV0272_ApprovalGrantsNamedUpFront` (`internal/tasks/cli`) |
| ATR-V0-275 | `internal/tasks/cli/request_conflict.go` (`defaultedIssuedAtConflict`), `internal/tasks/store/receipt_time.go` (`ReceiptRecordedAt`) | `TestATRV0275_LostOutputRetryAfterLaterUpdateRecovers` (`internal/tasks/cli`) |
| ATR-V0-280 | `internal/tasks/transaction/attempt_detail.go` | `TestATRV0280_RecoveryNamesReleaseForEveryHolderWithoutReap` (`internal/tasks/transaction`) |
| ATR-V0-281 | `internal/tasks/transaction/attempt_detail.go` | `TestATRV0280_RecoveryNamesReleaseForEveryHolderWithoutReap` (`internal/tasks/transaction`) |
| ATR-V0-296 | `internal/tasks/store/workflow.go` (`CleanupProgram`, `CleanupWorktrees`, `cleanupPhase`), `internal/tasks/cli/program_cleanup.go` (`programCleanup`) | `TestATRV0297_ReassignRemovesPriorStageWorktrees`, `TestATRV0296_CleanupRefusesLiveAttempt` (`internal/tasks/store`), `TestATRV0296_ProgramCleanupRefusalsDoNotMutate` (`internal/tasks/cli`) |
| ATR-V0-297 | `internal/tasks/store/workflow.go` (`OpenWorkflow` reassign) | `TestATRV0297_ReassignRemovesPriorStageWorktrees` (`internal/tasks/store`) |
| ATR-V0-298 | `internal/tasks/transaction/program.go` (`idleRemove`) | `TestATRV0297_ReassignRemovesPriorStageWorktrees` (`internal/tasks/store`) |
| ATR-V0-320 | none (tests pin the existing reap paths) | `TestCALV0361_UnownedExpiredLeaseIsReapedAtWake`, `TestATRV0064_ExitRecoveryActsAtWake` (`internal/tasks/dispatch`), `TestCALV0361_StoreReapAfterWakeIsNotGapAware` (`internal/tasks/store`: colliding claim and `reap`, then a fenced renew and heartbeat) |
| ATR-V0-321 | none (tests pin the clock basis of `internal/tasks/dispatch/host_sleep.go`) | `TestCALV0362_GoMonotonicClockIsNotBoottime` (Linux, in a time namespace whose boot time runs three hours further ahead than the host's own boot-time separation), opt-in `TestCALV0362_RealWallClockStepIsRecordedAsASuspendGap` (`CORVINT_TASKS_CLOCK_STEP=1`, `CAP_SYS_TIME`) (`internal/tasks/dispatch`) |
| ATR-V0-322 | `internal/tasks/supervisor` (`/proc` reader of CAL-V0-136; test only) | `TestCALV0363_ProcRowsAgreeWithPsOnLinux` (asserts the controlled fixture processes; unrelated host processes are logged), `BenchmarkCALV0136_ProcRowsOfThisHost`, `BenchmarkCALV0363_PsFork`, opt-in `TestCALV0137_HostWaitIdleCPU` (accepted only when the fixture outlives the window, the deadline interrupts the run and the stop is clean with the detached server gone) (`internal/tasks/supervisor`) |
| ATR-V0-323 | `internal/tasks/store/workflow.go` (`CleanupProgram`, `OpenWorkflow` settlement), `internal/tasks/transaction/program.go` | `TestCALV0365_TakeoverSettlesInterruptedRemoval` (registered, removed, half-removed and reopen modes, with its child `TestCALV0365_InterruptedRemovalChild`) (`internal/tasks/store`) |
| ATR-V0-324 | `internal/tasks/store/workflow.go` (`CleanupWorktrees`) | `TestCALV0366_CleanupDropsRemovedRecords`, `TestATRV0297_ReassignRemovesPriorStageWorktrees` (`internal/tasks/store`) |
| ATR-V0-325 | `internal/tasks/store/workflow.go`, `internal/tasks/transaction/program.go` | `TestCALV0367_LiveCleanupOwnerIsNotTakenOver` (with its child `TestCALV0365_InterruptedRemovalChild`, `live` mode) (`internal/tasks/store`) |
| ATR-V0-326 | `internal/tasks/store/workflow.go` (`worktreeRemover`), `internal/tasks/store/program.go`, `internal/tasks/snapshot/programs.go` | `TestCALV0368_SurvivingRemoverRefusesTakeover` (`stopped` mode) (`internal/tasks/store`) |
| ATR-V0-327 | `internal/tasks/store/workflow.go`, `internal/tasks/transaction/program.go` | `TestCALV0369_LegacyRemovedRecordsAreDroppedDurably` (FINISHED and idle ADMITTED) (`internal/tasks/store`) |
| ATR-V0-328 | `internal/tasks/store/workflow.go` | `TestCALV0365_TakeoverSettlesInterruptedRemoval` (registered and reopen modes) (`internal/tasks/store`) |
| ATR-V0-329 | `internal/tasks/store/workflow.go` | `TestCALV0371_MovedWorktreeKeepsItsRecord` (`moved` mode) (`internal/tasks/store`) |
| ATR-V0-340 | `internal/tasks/dispatch/cwd.go` (`RoleCwd`, `validate`, `launchCwd`), `internal/tasks/dispatch/config.go`, `internal/tasks/dispatch/loop.go` | TestATRV0340_OpenCodeMultiRepoLaunchesInTicketWorktree, TestATRV0340_NoCwdKeepsWorkRoot, TestATRV0340_CwdConfigIsValidated |
| ATR-V0-341 | `internal/tasks/dispatch/cwd.go` (`ticketWorktree`), `internal/tasks/dispatch/roster.go` (`Observation.TicketWorktree`), `internal/tasks/store/ticket_worktree.go` (`LookupTicketWorktree`), `internal/tasks/cli/dispatch.go` | TestATRV0341_WorktreeSourcesInOrder, TestATRV0341_LookupFindsOnlyAnExistingTicketWorktree, TestATRV0341_ObservationLooksUpTheTicketWorktree |
| ATR-V0-342 | `internal/tasks/dispatch/cwd.go` (`launchCwd`), `internal/tasks/dispatch/loop.go` | TestATRV0342_UnresolvedWorktreeFallsBackOrRefuses |
| ATR-V0-343 | `internal/tasks/dispatch/cwd.go` (`cwdDetail`), `internal/tasks/dispatch/loop.go` | TestATRV0340_OpenCodeMultiRepoLaunchesInTicketWorktree, TestATRV0342_UnresolvedWorktreeFallsBackOrRefuses |
| ATR-V0-344 | `internal/tasks/dispatch/config.go` (`Placeholders`, `carryArgvPlaceholders`), `internal/tasks/dispatch/loop.go` | TestATRV0340_OpenCodeMultiRepoLaunchesInTicketWorktree, TestATRV0344_CwdIsRecheckedAsRendered |
| ATR-V0-350 | `internal/tasks/dispatch/finish_outcome.go` (`stopReason`, `exitSignalText`), `internal/tasks/dispatch/process_unix.go`, `internal/tasks/dispatch/loop.go` (`finish`) | TestATRV0350_SignalStopWithCommitFinishedOutcome, TestATRV0350_StopReasonAndExitSignal |
| ATR-V0-351 | `internal/tasks/dispatch/finish_outcome.go` (`finishCommits`, `attemptRelease`, `hasHandoffNote`), `internal/tasks/dispatch/handoff_note.go` (`saveSalvage`), `internal/tasks/store/batch_run.go` (`CommitsSince`), `internal/tasks/cli/dispatch.go` | TestATRV0350_SignalStopWithCommitFinishedOutcome, TestATRV0351_AttemptRelease, TestATRV0351_CommitsSinceListsNewCommitsNewestFirst |
| ATR-V0-352 | `internal/tasks/dispatch/finish_outcome.go` (`progressLabel`), `internal/tasks/dispatch/loop.go` (`finish`) | TestATRV0350_SignalStopWithCommitFinishedOutcome, TestATRV0352_ProgressLabelFromFacts |
| ATR-V0-353 | `internal/tasks/dispatch/ledger.go` (`extractSummary`, `summaryText`), `internal/tasks/dispatch/finish_outcome.go` (`finishSummary`) | TestATRV0350_SignalStopWithCommitFinishedOutcome, TestATRV0353_SummaryNeverAStopFragment, TestCALV0058_SummaryReadsHostFinalText |
| ATR-V0-354 | `docs/TASKS-SUPERVISION.md` (finished event schema) | TestATRV0350_SignalStopWithCommitFinishedOutcome |
