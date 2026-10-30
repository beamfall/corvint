# Decision 0514 — Small-fix amendments (AHI-001, AHI-020, TJAA-V0-013, AFP-V0-020, CAL-V0-057, AFU-V1-025) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner answer in chat, 2026-10-11 UTC
("Those 8 PRs, finish them"), confirming acceptance of the listed requirements for pull requests
#758, #760, #761, #762, #764, #765, #766 and #767.

## Context

PR #767 carries six amendments and no new requirement ID: an `AHI-001` clarification in `docs/specs/agent-harness-integration-v0.md` that `--adapter-version` carries the adapter wire version `0.1.0`, independent of plugin package versions (V1-0444); an `AHI-020` clarification naming `mcp.example.json` and the `skills` paths among shipped files (V1-0452); the unreached Playwright helper amendment to `TJAA-V0-013` in `docs/specs/typescript-javascript-affected-adapter-v0.md` and `AFP-V0-020` in `docs/specs/affected-plan-v0.md` (V1-0211); the cooldown fingerprint-reset amendment to `CAL-V0-057` in `docs/specs/corvint-tasks-agent-leases-v0.md` (V1-0726); and the `review_state` amendment to `AFU-V1-025` in `docs/specs/application-flow-understanding-v1.md` (V1-0281). AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts the amendments to `AHI-001`, `AHI-020`, `TJAA-V0-013`, `AFP-V0-020`, `CAL-V0-057` and `AFU-V1-025` as written in the spec text carried by this pull request. Their status text
changes from "proposed, pending owner acceptance" to "accepted, decision 0514".

The owner was shown this set with the affected-plan amendment labelled `AFP-V0-021`, as in the PR description. The spec text carried by the PR amends `AFP-V0-020` (`NO_SELECTABLE_TEST`); `AFP-V0-021` is unchanged. This decision records the amendment the PR carries, which is the V1-0211 unreached-helper text the owner was shown.

## Limits

The evidence is the tests named in the five PR build logs and the amended traceability rows. Live qualification and `make gate` are `NOT_RUN`. This decision is not a standing rule for other requirements.

## Rollback

Revert this decision and return the six amendment status texts to proposed. To withdraw the behavior, also revert the corresponding source changes of PR #767, then regenerate `docs/specs/REQUIREMENTS.tsv`. Saved navigation packets produced with `review_state` must be regenerated after such a revert. No other stored state changes.
