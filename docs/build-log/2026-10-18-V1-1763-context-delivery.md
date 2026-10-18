# V1-1763: context notice delivery, hard-stop salvage and context exhaustion (issue 1109)

Ticket V1-1763, GitHub issue 1109, follow-up of V1-1549 (issue 1007, ATR-V0-030..039).
Requirements ATR-V0-101..107 in
[corvint-tasks-attempt-runner-v0](../specs/corvint-tasks-attempt-runner-v0.md), accepted under
the owner's standing auto-accept of lane requirements (2026-10-11). This is not a new owner
decision; it applies that standing acceptance to this lane's requirements.

## Host mechanisms

| Usage format | Host | Delivery mode | Mechanism |
| --- | --- | --- | --- |
| `claude-code` | argv or env uses `{contextHookSettings}` | `HOOK` | The dispatcher writes a per-worker Claude Code settings file that registers one `PostToolUse` command hook (matcher `*`) running its own executable as `dispatch context-hook`. The hook answers with `hookSpecificOutput.additionalContext`, which Claude Code adds to the session after the tool call. |
| `claude-code` | no placeholder | `UNKNOWN` | An operator may have registered the same hook in user or project settings; the dispatcher cannot see it. |
| `opencode` | any | `UNSUPPORTED` | V0 has no shell hook surface for opencode; a plugin is a residual. |
| `codex` or none | any | absent | These formats report no context figure (ATR-V0-030), so there is nothing to deliver. |

## Decisions

- **Pull, not push.** The hook reads the existing per-worker context record (ATR-V0-034) and
  marks each delivered level with an exclusive marker file. The dispatcher never writes into the
  session, and the record keeps its single writer. A worker that never runs a dispatcher verb
  still receives the notice on its next tool call.
- **PostToolUse, not PreToolUse.** The notice is added context, never a block. The hook always
  exits 0, so a damaged record or a missing variable cannot stop the session.
- **Opt-in placeholder.** A host gains the hook only when its argv or env uses
  `{contextHookSettings}`, for example `--settings {contextHookSettings}`. Existing host argv is
  unchanged. A role on such a host must be `claude-code`, because the settings file is Claude
  Code's format.
- **No new hand-off store.** The note is a file in the worker directory. It is linked by the
  attempt's existing hand-off evidence `dispatch:<worker>` (ATR-V0-084), which the CONTEXT
  hand-off already records, and the next launch on the ticket receives its path as
  `CORVINT_DISPATCH_HANDOFF_NOTE`. Every launch also names `CORVINT_DISPATCH_EVIDENCE_DIR`, the
  directory whose files the note salvages.
- **Event kind.** The event kind set is closed, so the salvage is a `context` event with a
  `note` detail and no `level`. Three existing V1-1549 tests that list context levels now skip
  that event (ATR-V0-035, -036, -038).
- **Exhaustion is a signal, not a hold.** `CONTEXT_EXHAUSTED` raises one alert and shows in
  `dispatch status`. Holding launches would duplicate no-progress parking and the ticket budget,
  which still apply. The count resets on any other ending of that role on that key. Lane keys
  are not tracked. The record follows the session-churn record's version rules (ATR-V0-055 pattern).

## Limits

- Note: 64 KiB total. At most the last 3 main-session assistant texts, each cut to 16 KiB, from
  the last 256 KiB of stdout. At most 8 regular evidence files in name order, each cut to its
  last 8 KiB. Symlinks and subdirectories are skipped.
- Every line matching the shared secret screen (`obligation.MatchesSecretScreen`) is replaced by
  `[REDACTED]`. A secret split across lines, or one the screen does not know, is not caught.
- The hook reads at most 64 KiB of payload on stdin.
- `exhaustedAfter` is 1..100, default 3, and needs `hardTokens`. The record holds at most 4096
  entries; the oldest is dropped first.

## Residuals

- A session that makes no tool call after a crossing never sees the notice. HARD is never
  delivered, because the dispatcher stops the session.
- Candidate hand-offs and other attempts without `dispatch:` evidence carry no note.
- Worker-directory retention (CAL-V0-144) removes old notes along with their worker directories.
- There is no opencode plugin.
- The evidence directory path is fixed, not configurable.

## Verification

- `internal/tasks/dispatch`: the seven ATR-V0-102, -104..107 tests, then the whole package once,
  all passing.
- `internal/tasks/cli`: the four ATR-V0-101, -103, -107 tests, the dispatch tests,
  TestCALV0131, TestCALV0047 and TestTMV0008_AS07, all passing.
- The hook test drives a real dispatcher and a fake host shell. The shell reads the
  dispatcher-written settings file and runs the registered command after each simulated tool
  call; that command is the test binary re-executed as `corvint-tasks dispatch context-hook`.
- Live host qualification against a real Claude Code session is **NOT_RUN**: hook environment
  inheritance, `--settings` merging and the additionalContext rendering are unqualified. This
  is cross-referenced to V1-1563, which owns live dispatch host qualification.
- Linux is NOT_RUN.
