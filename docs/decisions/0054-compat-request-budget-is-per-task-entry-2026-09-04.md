# Decision 0054 — the compatibility request budget is per task entry

Date: 2026-09-04. Status: accepted. Authority: coordinator call under decision 0052's build
authorisation; the repository owner may reverse this interpretation.

## Correction

One compatibility request is one manifest `tasks[]` entry. Its 120-second request budget is
cumulative across that entry's six repetitions, three per version. Each later entry receives a
fresh 120-second budget derived from the outer caller context; caller cancellation remains global.

A repetition not launched because its own entry's budget expired is `NOT_RUN` with reason
`budget-expired` when no repetition in that entry launched. Expiry does not consume or prevent a
later entry's budget. A manifest-wide interpretation would make the ten-case pilot impossible at
the permitted six launches of up to ten seconds per case.

## Verification and rollback

A short injected budget must terminate a running repetition while a second task entry still runs
all six repetitions under a fresh budget. Revert this decision's implementation and spec amendment
together if the repository owner restores a manifest-wide budget.
