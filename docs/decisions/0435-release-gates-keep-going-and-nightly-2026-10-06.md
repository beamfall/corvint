# Decision 0435 — Release gates keep going and run nightly

Date: 2026-10-06. Status: accepted under the owner's in-task delegation ("you make the decisions
for me", 2026-10-05) for the request "this seems to be struggling to get to RC2. investigate why and
fix it" (2026-10-06). The owner may revert it with the rollback below. Amends decision 0415 items
1 and 3. Ticket: V1-0848.

## Context

`1.0.0-rc.2` needed four candidates. Each of the first three failed the hosted macos-15
`full-gate` job; the first also failed the ubuntu-24.04 leg (Windows `cross-vet`). Each run showed
only part of what was broken (`docs/build-log/2026-10-05-rc2-first-candidate-gate-failure.md`,
`-second-` and `-third-`; root cause in beamfall/corvint#620):

- `make gate` stops at the first failed step. For all three candidates, `go-vet`, `cross-vet`,
  `interop-gate` and every later step never ran on hosted macOS.
- PR CI runs on ubuntu only and does not run `cross-vet`, and this workflow ran only when the owner
  dispatched it for a release. Defects that landed after rc.1 were first seen by a release
  candidate, about three hours of hosted time per round: Windows `cross-vet` (V1-0765, V1-0797,
  V1-0839), symlinked `TMPDIR` (V1-0840, V1-0842), descriptor exhaustion (V1-0841) and the S0E
  Git path (V1-0846, first misdiagnosed as V1-0843).

## Decision

1. **Keep going.** Both `full-gate` legs run `make -k gate` instead of `make gate`.
   - The pass condition is unchanged. `gate` records its receipt only after every prerequisite
     step succeeds, and make exits non-zero when any step fails.
   - So `full-gate EXIT 0` and the decision 0415 item 6 evidence contract mean what they meant
     before.
   - A failing run now also runs every independent later step, so one log names every failing
     step.
2. **Nightly.** The workflow also runs on a daily schedule (06:17 UTC), on `main`'s head.
   - On a scheduled run, `sha` is `github.sha` and the release label is `nightly`, which is used
     only in artifact names.
   - Every job runs, so a scheduled run rehearses the hosted gates on macOS and Linux. The
     `full-gate` jobs skip the `release-checklist` step and its artifact: the checklist fails
     while `VERSION` names a tag already published, which is `main`'s state after every release.
   - A nightly run is not release evidence: an attestation still cites a dispatched run at the
     candidate's sha.
3. Nothing else changes: the permissions, pinned actions, inputs, gate commands other than the
   `-k` flag, and the log format.

## Rollback

Revert `.github/workflows/release-gates.yml` to `make gate`, remove the `schedule` trigger and
the `github.event_name != 'schedule'` conditions. No store record, spec or other workflow
depends on either change.

## What this does not claim

- It does not claim a scheduled run has happened. The first one runs after this change merges.
- It does not make the nightly run a required check, and it does not add macOS to PR CI.
- Free-plan hosted minutes for a public repository are assumed, not measured. The schedule adds
  about three macOS runner-hours a day.
- `make -k` does not continue past a failed prerequisite of a step: a step that depends on a failed
  one still does not run.
