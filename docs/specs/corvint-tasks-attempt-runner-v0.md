# Corvint Tasks attempt runner V0

Owner: Russell Lewis
Date: 2026-10-04 (first drafted 2026-10-02 as a private Gate A proposal)
Intent status: proposed (owner issue 481; technical profile not owner-accepted)
Delivery status: experimental
Authoritative inputs: owner request [issue 481](https://github.com/beamfall/corvint/issues/481); native ticket V1-0677;
`docs/specs/corvint-tasks-agent-leases-v0.md` (external leases, heartbeat and renewal); the creation-owned
process-group API in `internal/groupreap/owner.go`; the in-tree sources under `internal/tasks/transaction`,
`internal/tasks/store` and `internal/tasks/cli`.

## Agent digest
- Claim: An experimental attempt runner keeps an externally leased attempt's heartbeat current, retires its owned process group and records an observational outcome.
- Status: proposed (owner issue 481; technical profile not owner-accepted); experimental. ATR-V0-001..007 are implemented with focused tests on Darwin.
- Exists: `corvint-tasks run --attempt ID --generation G --timeout SECONDS -- COMMAND...`, the `RUN_OUTCOME` lease verb and its closed outcome document, and CLI, store and transaction tests.
- Blocked on: owner acceptance of this technical profile, Linux and composed native qualification, required CI, integration and native completion of V1-0677.
- Read next: Requirements; Exit status and envelope; Failure modes; Traceability.

## Intent and boundary

A calling agent needs to run a long local command while keeping its native attempt signal and lease
current. The command must stop useful work on timeout, interruption or observed lease loss; owned
descendants must be retired before cleanup is reported. A command observation is not a gate,
completion, physical isolation or authority grant.

This profile is experimental, supports externally leased attempts on Darwin and Linux only, and
preserves the existing program/config `run` mode. Supervised attempts and platforms without the owned
process-group API refuse before any effect. No helper fork, daemon, account, network dependency,
global reservation hold or automatic retry is included. Existing external release and reap can still
change the attempt; the runner observes that through a refused heartbeat and stops the command.

It amends the external-agent lease verb set of `corvint-tasks-agent-leases-v0.md` (and through it
TCP-00) with one additive observation verb, `RUN_OUTCOME`. No wire code, receipt kind, journal
namespace or attempt field is added.

Non-goals for V0, each a candidate for a later accepted amendment: a pre-spawn `RUN_BEGIN` receipt;
a private crash/restart quarantine marker; caller-supplied request IDs and same-request replay;
periodic renewal while the command runs (coverage is established once before launch); interactive
terminal input (the command runs in its own background process group); environment filtering (the
command inherits the caller's environment, as any wrapper does); Windows.

## Requirements

- `ATR-V0-001`: `corvint-tasks run` MUST enter attempt mode only when `--attempt` appears before the first `--`, and MUST then accept exactly `--attempt ID --generation G --timeout SECONDS [--lease-minutes N] [--role ROLE] -- COMMAND...`. Unknown, repeated or program-mode flags, a missing delimiter or command, a timeout outside 1..86270 seconds, lease minutes outside 5..1440 (default 60) or an attempt of another queue are usage refusals before any effect. The command's argv is executed directly without a shell, in the caller's working directory, with the caller's stdin and environment. Program mode is unchanged.
- `ATR-V0-002`: Before launch the runner MUST heartbeat the attempt, then renew it only when the current lease does not cover the timeout plus the 10-second cleanup bound plus a 120-second scheduling and writer margin, using the larger of the requested minutes and the minutes that coverage needs. It MUST NOT shorten a longer lease, and it MUST refuse when the lease still does not cover timeout plus cleanup. A refused heartbeat or renewal starts nothing and records no outcome.
- `ATR-V0-003`: The command MUST be started once through `groupreap.Start` as the leader of an owned process group. On timeout, interruption or lost lease the runner MUST `Stop` the group, and in every started case it MUST retire it through `FinishBounded` with a 10-second bound; it never signals a numeric group ID after the reap. Only a `RELEASED` result reports cleanup as released; any other result is `HOLD`.
- `ATR-V0-004`: The runner MUST preserve the command's exit status and distinguish the outcome classes `EXIT`, `SIGNAL`, `TIMEOUT`, `LOST_LEASE`, `INTERRUPTED`, `SPAWN_FAILED` and `CLEANUP_HOLD` with the exit statuses in "Exit status and envelope".
- `ATR-V0-005`: Every started run, and every spawn failure, MUST be recorded by one `RUN_OUTCOME` lease receipt that posts only the canonical outcome document under `evidence/<sha256>`. The verb requires an attempt that exists and a generation between 1 and its current generation, but not a live or unexpired lease, so a fenced run's outcome is kept; it changes no attempt, lease, reservation or generation and renews nothing. The document must name the same attempt and generation.
- `ATR-V0-006`: While the command runs the runner MUST heartbeat about every 240 seconds with at most one heartbeat in flight. A refused heartbeat is a lost lease: the runner stops the group and records `LOST_LEASE` with the refusal code. A heartbeat write error is a warning only, because coverage was established before launch. After a released run that kept its lease, one final heartbeat is sent; its refusal records `LOST_LEASE`.
- `ATR-V0-007`: A platform without the owned process-group API MUST refuse with `UNSUPPORTED` before any write or start, and a supervised attempt is refused by the existing lease writer's heartbeat refusal before launch.

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

Pre-launch refusals carry no item. A recorded run carries one item with `runId`, `attemptId`,
`generation`, `class`, `cleanup`, `exitStatus`, `childExit`, `childSignal`, `lostLease`, `heartbeats`,
`renewals`, `outcomeSha256` and `outcomeReceipt`. A caller tells a command's own status 124..127 or 2
from a runner status by `class`.

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

## Acceptance and rollback

Acceptance evidence: the focused tests in Traceability on Darwin, `go vet`, the spec-index check and
one independent review of the delivered diff. Linux qualification, composed native qualification on a
live store, required CI, owner acceptance of the profile, integration and native completion of V1-0677
remain open and are not implied by this document.

Rollback: revert the task-owned commit. Program mode, existing lease verbs and existing receipts are
unaffected. Already-recorded `RUN_OUTCOME` receipts stay in journals and a binary without the verb
may refuse to audit them (inference, not tested), so a revert either keeps the verb's audit path or
applies only to stores that recorded none. No state is erased or rewritten.

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
