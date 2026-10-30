# Decision 0505 — Strict shard logs and early verify validation (AFP-V0-043, DCW-V0-034) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner answer in chat, 2026-10-11 UTC
("Those 8 PRs, finish them"), confirming acceptance of the listed requirements for pull requests
#758, #760, #761, #762, #764, #765, #766 and #767.

## Context

PR #758 (V1-1110, V1-1123) proposed `AFP-V0-043` in `docs/specs/affected-plan-v0.md` (cost-table `refresh` and plain `check` validate every shard log as a completed stream before writing) and `DCW-V0-034` in `docs/specs/daily-change-evidence-workflow-v0.md` (`dogfood change` validates verification lines with the recorder validator before invoking it). The owner first accepted both on 2026-10-10 ("acceptance is given"). AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `AFP-V0-043` and `DCW-V0-034` as written in the spec text carried by this pull request. Their status text
changes from "proposed, pending owner acceptance" to "accepted, decision 0505".

## Limits

The evidence is the `TestAFPV0043*` tests in `tools/ci-shard-costs` and the `DCW-V0-034` tests named in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-10-batch-tools-validation.md`. Live qualification and `make gate` are `NOT_RUN`. This decision is not a standing rule for other requirements.

## Rollback

Revert this decision and return the `AFP-V0-043` and `DCW-V0-034` status text to proposed. To withdraw the behavior, also revert the strict reader selection in `tools/ci-shard-costs/main.go` and the early validation in `internal/dogfoodflow/change.go` and `internal/trace/record.go`, then regenerate `docs/specs/REQUIREMENTS.tsv`. No stored state changes.
