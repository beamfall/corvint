# Decision 0500 — AFP-V0-041 (5) slice default of 8 shards accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accepted", in reply to the request to accept the AFP-V0-041 (5) change from 6 to 8 shards).

## Context

V1-1094 (PR beamfall/corvint#744) raised the `go-product-shard` matrix in `.github/workflows/ci.yml`
from 6 to 8 jobs. `REUSE_SHARDS` (AFP-V0-024), the drift job's `SHARDS` (AFP-V0-040) and the
`pr-tests-qualification.yml` default moved with it. Decision 0488 accepted `AFP-V0-041`, whose clause
(5) names the `tools/ci-test-slices` `--shards` default as "default 6, the go-product-shard matrix".
The lane re-derived `.github/cishards/test-slices.json` for 8 shards and changed that text to 8,
marked proposed. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts the amendment of `AFP-V0-041` (5): the `generate` and `replay` default is 8, the
go-product-shard matrix length. The rest of `AFP-V0-041` and decision 0488 are unchanged.
`TestAFPV0024WorkflowShardCountsAgree` keeps every count source equal to the matrix length.

## Limits

This decision settles intent only. The two hosted 8-shard runs passed, but the slowest shard took
17.75 and 19.7 minutes, so V1-1094's 17-minute target is not met and that ticket stays open. The
partition digest moved; no selection pin or qualification row existed to invalidate. `make gate`
is `NOT_RUN`.

## Rollback

Revert this decision, return the AFP-V0-041 (5) text to "default 6", and revert V1-1094's matrix,
count and slice changes together; then regenerate `docs/specs/REQUIREMENTS.tsv`.
