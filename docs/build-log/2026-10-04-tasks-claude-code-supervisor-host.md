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

Rollback: cancel or drain every `claude-code` program with its original config while the policy pin
is in force, then remove `host` from the policy, then start new programs from configs without
`host`. Codex bytes are unchanged.
