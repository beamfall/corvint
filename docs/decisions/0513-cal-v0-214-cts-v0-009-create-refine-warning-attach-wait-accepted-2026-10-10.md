# Decision 0513 — Create/refine directory-key warning and attach pending-writer wait (CAL-V0-214, CTS-V0-009, CAL-V0-211 non-goal amendment) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner answer in chat, 2026-10-11 UTC
("Those 8 PRs, finish them"), confirming acceptance of the listed requirements for pull requests
#758, #760, #761, #762, #764, #765, #766 and #767.

## Context

PR #766 proposed `CAL-V0-214` in `docs/specs/corvint-tasks-agent-leases-v0.md` (V1-1125: fresh `ticket create` and `ticket refine` transactions carry the advisory CAL-V0-211 `EXACT_KEY_NAMES_DIRECTORY` warning), narrowed the CAL-V0-211 non-goals so they no longer exclude `ticket create` and `ticket refine`, and proposed `CTS-V0-009` in `docs/specs/corvint-tasks-store-init-v0.md` (V1-1126: the initial head read of `run --attach` waits out a live writer within the CTS-V0-006 patience budget). AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `CAL-V0-214`, `CTS-V0-009` and the `CAL-V0-211` non-goal amendment as written in the spec text carried by this pull request. Their status text
changes from "proposed, pending owner acceptance" to "accepted, decision 0513". The accepted CAL-V0-211 behavior for `set-effects` and `claim` is unchanged.

## Limits

The evidence is the `TestCALV0214_*` and `TestCTSV0009_*` tests and the loaded attach qualification named in the PR build logs. Live qualification against a real store and `make gate` are `NOT_RUN`. This decision is not a standing rule for other requirements.

## Rollback

Revert this decision and return the `CAL-V0-214`, `CTS-V0-009` and CAL-V0-211 non-goal text to proposed. To withdraw the behavior, also revert the create/refine warning in `internal/tasks/cli` and the attach wait in `internal/tasks/cli/attempt_detach.go`, then regenerate `docs/specs/REQUIREMENTS.tsv`. No stored state or wire code changes.
