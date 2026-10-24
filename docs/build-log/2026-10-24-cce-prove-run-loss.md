# 2026-10-24 CCE prove report keeps each lost run's cause (V1-2620, CCE-V0-116)

Lane `claude/cce-live-attempt-reasons-1024`, base origin/main `be7ba12f8`. Ticket V1-2620; the
orchestrator closes it.

## Problem

CI cce-live run 40129429349 failed `TestCCEV0009LivePreserves` with `INCONCLUSIVE`
`control-not-killed:control:omitted-event:setup-1`. The control proof held only BBF's aggregate
`one-or-more-attempts-lost-to-infrastructure`. The qualify reason, the run-level error's message and
its location were discarded in the hook, so the log could not say what lost the attempt. The test
passes locally.

## Change

- Report shape (additive, `corvint-test e2e prove` JSON): `head.loss` / `base.loss` on an unqualified
  side, and `controls[].lost_attempts[]` for each BBF attempt that reads `infrastructure_failed`.
  Fields: `attempt`, `attempt_reasons`, `reason`, `error_location` (`path:line`,
  `outside-test-tree` or `unlocated`), `error_message` (ANSI-stripped, roots made relative, other
  absolute paths `<path>`, secret-screened, at most 512 bytes) and `hook_process` for an attempt
  without a receipt. Every field is `omitempty`; a qualified run adds nothing, so older reports and
  their readers are unaffected. No golden or digest pins the report shape.
- Hook detail encoding: the control hook's infrastructure receipt `detail` is now
  `reason\nlocation\nmessage` when the run kept an error, else the reason alone, as before. The
  prover splits on the first two newlines, so a one-line detail written by an older hook reads as
  the reason alone and CCE-V0-087's `teardown-failed at <file>:<line>` match still reads the first
  line. The detail stays under BBF's 4096-byte bound (reason, location and message each bounded).
  BBF's wire schema is unchanged.
- Classification is unchanged: no verdict, note, status, reason list or score reads the new fields.
- `requireVerdict` in the live tests appends every loss to its failure message.

## Cause of the CI failure

Not determined. Candidates are an unlocated reporter-global-error inside the hook's Playwright
run, or the hook process itself failing or timing out (no receipt, `hook-process-failed`). The new
fields tell these apart on the next CI failure; no decisive evidence was available in this lane.

## Rollback

Revert the commit. No stored state migrates.
