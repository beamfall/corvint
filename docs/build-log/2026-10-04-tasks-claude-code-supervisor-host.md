## 2026-10-04 V1-0755: Claude Code supervised host for corvint-tasks programs

Human-owned intent: native ticket V1-0755, split from issue 354 by the owner's scope decision of
2026-10-04. Its acceptance criteria are:
- `corvint-tasks run --host claude-code` drives the three roles under the Codex host's lifecycle
  guarantees;
- an unusable executable is refused with a named code before any lease;
- token usage is observed or `NOT_OBSERVED`, never invented.

Requirements: CAL-V0-074 and CAL-V0-075, slice S22 of
`docs/specs/corvint-tasks-agent-leases-v0.md`. CAL-V0-073 is left to coordinated unlanded work.
The sibling OpenCode lane (V1-0756) allocates after CAL-V0-075 and S22.

Decisions:

- The host is selected in policy. `supervision.host` is the only selector, `claude-code` is its
  only admitted value, and absent means Codex. The config carries the same `host`, and its
  digest binds it. Making the policy the selector keeps one owner decision authoritative. The
  transaction layer has a single supervised runtime slot, named `taskman-codex-supervisor/0`,
  so one policy pins one binary for one host, and mixed hosts are out of scope. The runtime ID
  and profile keep their names because they identify the protocol, not the vendor. Explicit
  `"codex"` is refused in the policy, config and capsule, so a Codex program has exactly one
  encoding and its bytes are unchanged.
- The seam is a vocabulary of four parts. `supervisor.HostVocabulary(host)` returns the
  result decoder, session reader and usage reader, and `store` holds one argv builder per host.
  The lane leader, the stage journal and the transaction layer all select a vocabulary by host
  name: the leader from the capsule, the journal from the config, and the transaction layer from
  the policy. Adding a host such as V1-0756 adds one vocabulary entry and one argv builder. The
  fork, the process group, the output caps, the WAIT/resume flow and the transitions are
  untouched. A host whose lifecycle is a plugin inside a long-lived process still needs a
  foreground process that the lane leader can own and reap. Whether such a host fits this
  process boundary is V1-0756's decision; this slice does not pre-build that boundary.
- Usage is re-derived from retained output in the policy host's vocabulary. Both
  `transaction/program.go` and `transaction/supervisor.go` already re-derive usage from retained
  output and refuse a mismatch. Selecting the vocabulary from the policy rather than from caller
  data keeps that re-derivation independent of the writer. If the policy host changes during a
  stage, that stage's finish is refused. The failure is visible, and the spec records it.
- The Claude Code argv was checked against `claude --help` of Claude Code 2.1.267:
  - `-p --output-format json`, which yields a single result object, with the prompt on stdin;
  - the S13 effort passed through `--effort`;
  - `--setting-sources project` and `--strict-mcp-config`, so user settings and MCP servers do
    not leak in;
  - `--permission-prompts none`, so nothing blocks on a prompt;
  - `acceptEdits` for implement;
  - `dontAsk` plus `--disallowedTools Edit,Write,NotebookEdit` for review and integrate;
  - `--resume S` to continue a WAIT;
  - the variadic `--add-dir` last, listing the sorted S21 siblings.

  Bash is not contained. Read-only stages still rely on the existing check, made after the host
  exits, that the tree is unchanged.
- The result object must be exactly one JSON object with `type` `result`, `subtype` `success`,
  `is_error` false and a session of 1..128 bytes. Its `result` string must strictly decode to the
  S10 handoff. Prose, a Markdown fence, a second object or trailing text all make the stage
  `INVALID_RESULT`. Usage needs integer input and output counters. The cache creation and cache
  read counters are added to input when present. A null, string, fractional or negative counter,
  or an overflow, makes the turn `NOT_OBSERVED`.
- The pin refusal now has a name. A missing, unreadable or unpinned executable, or a digest
  mismatch, is `CAPABILITY_UNAVAILABLE` for both hosts; Codex previously got an unnamed
  `MALFORMED`. A host mismatch is `UNSUPPORTED`, both at the CLI `--host` check and in the store.
- A defect fixed in passing: only the stage that continues an answered WAIT resumes its session.
  Before this change, once a WAIT had been answered, the answer persisted past `BUILT`. Review
  and integrate then also resumed the author's session: `exec resume S` for Codex, `--resume S`
  for Claude Code. That gave the review the author's session, which review independence refuses,
  or a resumed host returned the author's handoff kind. `TestCALV0075_ClaudeCodeProgramFakeHost`
  failed on this before the fix. Codex argv for the defective path changes; every other Codex
  path is unchanged.

Evidence:
- Focused tests, which use only fake executables:
  - the CAL-V0-074 and CAL-V0-075 rows of the traceability table;
  - the end-to-end `TestCALV0075_ClaudeCodeProgramFakeHost`, which runs implement and WAIT,
    then the answer, then the resumed implement and BUILT, then an independent review and
    READY_FOR_INTEGRATION, and checks that program usage is 15/3 across three Claude Code turns.
- Existing Codex argv, effort, wall-time, multi-repository and supervisor tests remain green.
- Two flakes were observed on a host with load average 43 to 53; this change does not explain
  either one:
  - The first end-to-end run was once refused with `MALFORMED: transaction: program transition`.
    The Claude Code test and the Codex multi-repository test then passed 8 of 8 repeats.
  - `TestATRV0004_InterruptStopsTheRun` (the attempt runner's `SURVIVORS`) failed 2 of 15 on the
    unchanged base commit. This change does not touch the attempt runner.

`NOT_RUN`:
- live Claude Code qualification on a disposable program;
- integration through a real grant with the Claude Code host. The integrate argv and vocabulary
  are shared with review, and the S10 integration binding is unchanged;
- a comparison of Claude Code `usage` totals against billed usage.

Review round 1 (Codex, `CHANGES_REQUIRED` on ce4ae150..3992d096); each finding was reproduced
before it was fixed:
- P1, the pinned symlink. Admission followed symlinks when it read and checked the pin, but the
  capsule validation at launch used `Lstat` and required a regular file. A pinned symlink was
  therefore admitted and leased, and then refused at launch. That refusal returned before the
  lane leader's `NO_EXEC` boundary, so the stage stayed `SPAWNING` with its worker unresolved.
  - The fix: `supervisor.LaunchableExecutable` is now the shared launch-time check, covering an
    absolute path, no symlink, a regular file with an execute bit, and the bounded read.
    Admission and `RunProgram` run it before any record, claim or lease, and refuse
    `CAPABILITY_UNAVAILABLE`.
  - A launch refusal after admission now settles the stage as `NO_EXEC`.
  - The decision is to refuse a symlink and pin its target, not to resolve it. The evidence:
    `/opt/homebrew/bin/claude` points to
    `../lib/node_modules/@anthropic-ai/claude-code/bin/claude.exe`, a regular 0755 Mach-O file,
    and `/opt/homebrew/bin/codex` is likewise a symlink to a regular file. A package update
    retargets these links, so a pinned link path would change meaning without its pin changing,
    and the launch contract already refused links. The refusal names the target to pin.
- P2, the existing program. Reopening an existing program checked only the config digest, so after
  a policy host change an idle or completed program reassigned and claimed new work before the
  stage refused. The fix: the reopen now rechecks the host before reassignment, claim or attach,
  unless the current attempt is live and supervised. That attempt keeps drain and cancel only.
  Answer is refused after any policy change, because the attempt's policy binding is stale; this
  behaviour predates the change and does not come from the host check.
- P2, duplicate members. `encoding/json` keeps the last duplicate member and merges a repeated
  `usage` object. The Claude Code reader now refuses a repeated member in any object of the result,
  its usage or the decoded handoff, so the stage is `INVALID_RESULT` with usage unobserved. The
  Codex event reader is unchanged.
- P2, rollback. Existing programs bind their original config digest, so the documented rollback
  now cancels or drains them with that config while the pin is in force, before the policy
  changes.
- New witnesses:
  - `TestCALV0074_AdmissionRunsLaunchCheck`: the symlink and non-executable cases leave the
    inventory unchanged, and a refusal after admission is cleaned up.
  - `TestCALV0074_HostSwitchAndRollback`: the idle and completed reopens are refused unchanged,
    an edited config is refused, a live attempt is refused a stage but cancels, and a new Codex
    program then claims.
  - `TestCALV0075_ClaudeDuplicateMembers`, and a duplicate-`is_error` lane-leader run in
    `TestCALV0074_CapsuleHost`.
  - Each new test failed with its fix disabled.

Rollback: cancel every `claude-code` program, drained ones included, with its original config
while the policy pin is in force, then remove `host` from the policy, then start new programs from
configs without `host`. Codex bytes are unchanged.

### Review round 2

The Codex re-review of 36627d0f returned CHANGES_REQUIRED with six findings. Each is fixed in an
additive commit.

- P1, check-to-exec race. The lane leader validated the pinned path and then executed it by path
  after the acknowledgment, so a replacement in between ran unchecked bytes.
  - The fix: the check opens the path once with `O_NOFOLLOW` and takes type, mode, size (at most
    256 MiB) and bytes from that descriptor. The leader runs the path in place only when the
    canonical path matches the descriptor's device and inode and the file and every ancestor are
    root-owned with no group or other write. Otherwise it writes the verified bytes to a private
    0500 copy in the effect directory, runs that copy, and removes it when the host exits.
  - Rejected alternatives: executing through `/dev/fd/N` is refused "permission denied" on this
    macOS host, and always copying fails because macOS launch constraints SIGKILL (exit 137) a
    copied platform binary such as `/bin/sh`. Copies of `claude.exe` 2.1.267 and the vendored Codex
    native binary 0.153.2 ran `--version` from a copy.
  - Limit: a runtime that loads files relative to its own path, such as Codex's `codex.js`
    wrapper, runs without them from the copy. Pin a self-contained binary. The copy costs one
    write of the runtime per unprotected stage launch.
  - Witness: `TestCALV0074_RuntimeReplacedAtAck` replaces the runtime in place and by rename at
    the `RUNNING` journal write, for both hosts, and the original bytes run.
- P1, empty class is not proof of no spawn. Round 1 settled `NO_EXEC` whenever the run returned an
  error with no outcome class, which also matched failures after the leader forked, such as a boot
  identity mismatch or a refused `RUNNING` journal write.
  - The fix: `supervisor.Run` returns `PrelaunchError` only before the fork, and only that error
    settles `NO_EXEC`. Every other error keeps the drain result, so an unproved drain stays
    `BLOCKED_RECOVERY`.
  - Witnesses: `TestCALV0074_PrelaunchErrorOnlyBeforeSpawn`, and
    `TestCALV0074_SpawnedFailureIsNotNoExec`, in which a test leader forges its boot identity while
    a session child holds output.
- P2, `NO_EXEC` kept the claim. The settled attempt stayed `WAITING` with its lease and
  reservation. The fix: the stage then applies `CANCEL`, so the attempt is `CANCELLED` and the
  ticket can be reclaimed, under the same attempt ID with a new generation.
- P2, case aliases. `encoding/json` matches field names case-insensitively, including the Kelvin
  sign, so `IS_ERROR` could override `is_error` without repeating a member. The fix: the result and
  handoff objects refuse any member that case-folds to a read name but is not it. Escaped
  spellings are covered.
- P2, duplicate handoff kept session and usage. The fix: the result, session and usage readers
  share one check over the result object and a JSON handoff, and the test exemption is removed.
- P2, rollback stranded claims. A drained program keeps a `WAITING` attempt that holds its claim,
  and once the Codex runtime pin replaces the Claude Code one its original config is refused
  `CAPABILITY_UNAVAILABLE`, so it can no longer be cancelled. The fix: rollback step 1 now requires
  cancel, drained programs included. A missed program is recovered by re-pinning its Claude Code
  runtime, then cancelling it, then restoring the Codex pin.
  - Witness: `TestCALV0074_HostSwitchAndRollback` now uses distinct Claude Code and Codex runtimes,
    and covers drain-then-cancel, a stranded drained program, and its recovery.

Not changed: the earlier `NO_EXEC` paths for a refused stage admission, a failed preparation and a
failed `cmd.Start` still settle to `WAITING` without cancel. They predate this slice.

### Review round 3

The Codex review of 36627d0f..e620c35b returned CHANGES_REQUIRED with three findings, each
confirmed by inspection before the fix.

- P1, ACLs. `protectedRuntime` checked owner and mode bits only. On macOS an ACL entry is evaluated
  before the mode, so a root-owned 0755 runtime could still be writable by a user. The fix: the
  runtime and every ancestor must carry no ACL. On macOS the probe reads
  `ATTR_CMN_EXTENDED_SECURITY` with `getattrlist` and counts any entry. On Linux it looks for the
  POSIX, NFSv4 and richacl extended attributes. A read failure counts as an ACL, so the runtime
  falls back to the private copy. On the owner's host `/bin/sh`, `/bin`, `/usr` and `/` carry none,
  and the home directory carries the standard `everyone deny delete` entry.
  - Witnesses: `TestCALV0074_ACLProbe` sets a real ACL and detects it. `TestCALV0074_RuntimeReplacedAtAck`
    marks `/bin/sh`, `/bin` and `/` as ACL-bearing in turn and each refuses direct execution; it
    fails with the ACL check removed.
- P2, cancel after release. `noExec` released the owner before the separate `CANCEL` write, so a
  competing owner could take the program in between and fence the cancel. The fix: the cancel now
  lands before the owner is released. An atomic stop, cancel and release needs a combined attempt
  and program transition: `SupervisorTransition` and `ProgramTransition` are separate journal
  mutations over different records, and adding a combined one changes the transaction contract,
  which is outside this lane. The remaining crash window, after the stop and before the cancel, is
  recorded as a known limit in S22: the attempt stays stopped and `WAITING` with its claim, and a
  reopen with the original config cancels it.
  - Witness: `TestCALV0074_NoExecCancelBeforeRelease` observes the window through a test hook. The
    owner is unreleased, a competing owner is refused as live-owned, and after a simulated crash a
    reopen cancels and the ticket is reclaimed. It fails with the old order.
- P2, root-run test. The test expected `/bin/sh` to be unprotected under UID 0, but the check does
  not depend on the runner. The expectation now comes from a filesystem oracle. The negative case
  and the replacement test use a world-writable directory, so they hold for any runner.

### Review round 4

Codex reviewed `e620c35b..6a86d6c9` and confirmed the ACL layout, the UID-independent test and the
cancel-before-release order. It found one P2, confirmed by mutation before the fix.

- P2, crash recovery. After a launch refusal the program stayed `STOPPING` until the release. A
  replacement process could not take it over: recovery evidence is produced only for an attempt
  that still has a worker, and `STOPPING` is not a safe takeover phase. The round 3 test missed this
  because it reopened under the original process identity. The fix: before the cancel, `noExec`
  records the program `FINISHED` with proved quiescence while the owner still holds it, then
  cancels, then releases the owner with a second `FINISHED` write. `FINISHED` is a safe takeover
  phase, so a replacement takes over once the owner is gone, while a live owner still refuses a
  competitor.
  - Witnesses: `TestCALV0074_NoExecCrashTakeover` runs the owner as a child process that exits at
    the cancel point or at the release point. The parent, a different process, then takes the
    program over. At the cancel point it cancels the attempt and another program reclaims the
    ticket. At the release point it reassigns the program to the released ticket. With the
    `FINISHED` record removed, both takeovers are refused. `TestCALV0074_NoExecCancelBeforeRelease`
    now checks both windows in one process: the program is `FINISHED` and unreleased, the attempt is
    `WAITING` and then `CANCELLED`, and a competing owner is refused as live-owned.

### Review round 5

Codex reviewed `6a86d6c9..83944dba`. It confirmed that both tested interruption points are sound,
that live-owner fencing holds, and that the non-cancelling `noExec` paths are unchanged. It found one
P2, confirmed by test before the decision.

- P2, interruption before the `FINISHED` record. `STOPPED` and the program `FINISHED` record are
  separate writes. An owner that dies between them leaves a `STOPPING` program with a stopped,
  worker-free attempt. A replacement is refused, because `STOPPING` is not a safe takeover phase
  and recovery evidence covers only worker attempts. Earlier windows behave the same, because
  recovering a worker attempt needs the leader's boot record and a refused launch never writes
  one. That earlier window predates this lane: it applies to any owner death between dispatch and
  the leader's boot. Closing either window needs a combined attempt and program transition, or a
  takeover rule that admits a bound, stopped and quiescent attempt from `STOPPING`. Both change the
  transaction contract, and the second also touches the usage-derivation rule for
  `STOPPING` to `FINISHED`. Per the coordinator's instruction, it is recorded as a known limit in
  S22 rather than fixed here, and the coordinator files the follow-up. The spec and failure table
  no longer claim recovery before the `FINISHED` record.
  - Witness: `TestCALV0074_NoExecCrashTakeover/stopped` exits the child owner at the new `stopped`
    hook point. The replacement is refused with "prior program owner not proved stopped". The
    program is unchanged, and another program's claim is refused with `ATTEMPT_LIVE`.
- Unrelated: `TestPSRSweepSuccessReplaySuccessor` failed in the round 3 full store run, with "not
  freed". The same test fails when run alone on base `ce4ae150`, in two of three runs, so the
  failure predates this lane.

