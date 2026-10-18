# V1-1588, V1-1593, V1-1602: ticket worktrees, defect register, adopter value screen

One change covers three Corvint Tasks tickets (GitHub #1031, #1036, #1045). Intent: the owner's
standing auto-accept of lane requirements (2026-10-11) accepts `TWT-V0-001..007`
([ticket worktrees](../specs/corvint-tasks-ticket-worktrees-v0.md)), `TDR-V0-001..010`
([defect register](../specs/corvint-tasks-defect-register-v0.md)) and `KHN-V0-028..031`
([know-how notes](../specs/corvint-tasks-know-how-notes-v0.md), adopter value screen). No
requirement was added to the agent-leases spec.

Decisions:

- `worktree ensure` is a Git operation bound to policy `worktrees`, with no Tasks journal write.
  Reuse is keyed by the branch, which comes only from the ticket's local id and the pattern. A path
  is resolved through symbolic links and must lie strictly inside the resolved root before any Git
  write. A dirty worktree is reported and never touched.
- The defect register is a separate JSONL file. Its default home is
  `<git-common-dir>/taskman-defects/`, outside every worktree, the Tasks state directory and
  `archive export`. That keeps it out of published and backed-up artefacts unless the adopter
  declares `defects.path`. Writers hold `flock` on a sibling lock file; a torn tail is ignored on
  read and sealed by the next writer.
- The adopter value screen (`knowHow.screen`) adds exact-value matching and opt-in email, phone
  and card (Luhn) classes to the shared credential screen. It covers know-how, operator notes and
  obligation text. Refusals stay `SECRET_DETECTED` and name only the rule, never the value.

Deviations, recorded rather than hidden:

- No new error codes were added. Distinctions travel as detail prefixes such as
  `DEFECT_SECRET_SCREENED` under the existing codes.
- Preflight treats a spec line holding a `test.fail(` call as an expected-fail annotation, as well
  as lines the existing expected-fail title matcher accepts.
- `internal/tasks/intent/adopter_policy.go`, `internal/tasks/mutation/value_screen.go` and
  `internal/tasks/store/value_screen.go` were added to the know-how reader set in
  `TestKHNV0007`, because they name the know-how policy key.

Residuals:

- The worktree head is not recorded on release or completion.
- `defects harvest` from Playwright or CI reports is not implemented.
- The CLI pre-parse know-how argument screen does not apply declared values; the screen runs at
  apply time.
- The verbs live in `corvint-tasks`, not Core, because the Core verb surface is frozen.
- On reuse, an existing ticket branch that no longer descends from the base is reported with a
  warning rather than refused. Attach refuses it.

Evidence: focused `internal/tasks/{intent,mutation,defects,store,cli}` tests on Darwin. Linux,
multi-process and live host qualification are `NOT_RUN`; `make gate` is `NOT_RUN` (owner's scoped
preference). Rollback: revert the change and remove `worktrees`, `defects` and `knowHow.screen` from
policy. No Tasks record depends on them.
