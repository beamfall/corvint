# Decision 0537 — AFP-V0-029/030 amended: a hosted set-up failure is an `INFRA` forward row

Date: 2026-10-16. Status: accepted by the owner. Authority: owner approval in chat to the
orchestrator, 2026-10-16 (option (a): a new forward-row class `INFRA` for hosted pre-step failures,
excluded from the AFP-V0-030 `INCOMPLETE` trip count and from `complete`, with AFP-V0-016 consent
for the `.github/forward-rows/` and `.github/workflows/ci-control-plane.yml` edits). Ticket: V1-1514.

## Context

Decision 0509 accepted AFP-V0-029/030: a forward row whose shard documents do not cover every
shard is `INCOMPLETE`, and three `INCOMPLETE` rows in the latest 20 trip the kill switch. Merge
queue run 38686348902 (PR #908, queue sha `cab1c638a36038aa06847550717158f9d1050026`) lost
`go-product-shard (5)` at the hosted runner's `Set up job` step, before any step of the job ran.
Its row was `INCOMPLETE` on the sole reason that the outcomes documents covered shards 0..4 and
6..7 of 8. A hosted pre-step failure is not evidence that the forward selection was unsafe, yet it
counted toward both the kill switch and away from the 20 complete rows of the bar.

## Decision

A row gains the class `INFRA` (AFP-V0-052) when every gap it has is explained by shard jobs whose
first step, `Set up job`, failed, and it has no other incomplete reason and no non-`FLAKY` miss.
An `INFRA` row takes no place in the latest-20 window, is never complete, and never counts as
`INCOMPLETE`; the bar reports how many `INFRA` rows lie between the window's rows. Every other gap,
including one beside a hosted failure, stays `INCOMPLETE`.

## Limits

Only `go-product-shard (N)` jobs of the outcomes run are read. A failure of a later step, a
cancelled job, a runner lost mid-job, or a run whose every shard failed at set-up stays
`INCOMPLETE`. No hosted `INFRA` row has been observed after the change (`NOT_OBSERVED`).

## Rollback

Revert this decision, the AFP-V0-029/030 amendment sentences and AFP-V0-052, and the `infra`
reading in `.github/forward-rows/collect.sh`, `classify.sh`, `row.jq` and `bar.jq`, then regenerate
`docs/specs/REQUIREMENTS.tsv`. Earlier `INFRA` artifacts then match no bar name and are not counted.
