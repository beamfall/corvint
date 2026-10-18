# V1-1782: carry a no-progress session's final message into the next session (issue 1122)

Ticket V1-1782, GitHub issue 1122. Requirements ATR-V0-110..114 in
[corvint-tasks-attempt-runner-v0](../specs/corvint-tasks-attempt-runner-v0.md), accepted under
the owner's standing auto-accept of lane requirements (2026-10-11). This is not a new owner
decision; it applies that standing acceptance to this lane's requirements.

## Decisions

- **Prompt block, not a path.** The V1-1763 hand-off note (ATR-V0-104) is passed by path and covers
  only the hard stop. Issue 1122 asks for the prior session's own words in the next prompt, so the
  block is appended to the rendered role prompt. It is appended after `Render`, so the prior text
  is never placeholder-expanded.
- **Reuse, not duplication.** The message is read with the V1-1763 helpers: the stdout tail window,
  the main-session text parser, the line-based secret screen, the line-boundary tail cut and the
  no-symlink, no-FIFO regular-file read. The end reason reuses the existing kill reasons and the
  V1-1549 context crossing record. No new event kind is added; `launched` and `finished` gain
  open detail members.
- **No ledger change.** The record is a per-ticket file in the program's state directory, in a new
  closed profile `taskman-dispatch-carry-over/0` classified for CAL-V0-131. The finished event's
  count is read from a per-worker marker written before spawn, so the dispatch state profile and
  the `Worker` record are unchanged.
- **Host restriction.** A prompt that carries untrusted text must reach the agent as data. An
  enabled role's host must pass `{prompt}` as one whole argv element and must not run an
  interpreter or take a code-string option, as `{operatorNote}` already requires. Unlike
  `{operatorNote}`, the other placeholders whose values come from the configuration (`{model}`,
  `{effort}`, `{contextHookSettings}` and the like) stay allowed outside argv[0], because real
  hosts pass `--model {model}`; ticket-derived placeholders are still refused.
- **Progress clears it.** A progress session of any role removes the ticket's record, so a later
  stall starts cold. A failed removal alerts and stops the offer in that dispatcher, which keeps
  AC4 even when the file cannot be removed.
- **Bounds.** Defaults are `enabled` true, `maxBytes` 4096 and `sessions` 1. Fixed bounds are
  16 KiB per recorded message, 5 entries and 128 KiB per record. The frame phrase is lower-cased
  inside entry text, so an entry cannot close the block early.

## Evidence

Focused tests `TestATRV0110`..`TestATRV0114` in `internal/tasks/dispatch` (8 tests), the
CAL-V0-131 reader row in `internal/tasks/cli/format_version_test.go`, and a mutation check:
dropping the prompt append fails `TestATRV0113_NextSessionPromptCarriesBoundedLabelledBlock`.
Live host qualification and a measured progress-rate comparison are NOT_RUN.
