# Decision 0510 — Dogfood plan and trace caps, private cache and no-action reasons (DCW-V0-035..038, LCP-V0-020) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner answer in chat, 2026-10-11 UTC
("Those 8 PRs, finish them"), confirming acceptance of the listed requirements for pull requests
#758, #760, #761, #762, #764, #765, #766 and #767.

## Context

PR #762 proposed `DCW-V0-035` and `DCW-V0-036` (V1-0394: plan-row and trace-path overflow name the limit and observed count), `DCW-V0-037` (V1-1107: staged citation plans for a single large commit) and `DCW-V0-038` (V1-0693: a private Go cache for every dogfood build) in `docs/specs/daily-change-evidence-workflow-v0.md`, and `LCP-V0-020` in `docs/specs/local-completion-policy-v0.md` (V1-0243: every `dogfood status` without a next action carries a reason). `LCP-V0-020` was first proposed as `LCP-V0-019` and renumbered because PR #761 takes that ID. The owner first accepted the set on 2026-10-10 ("I accept"). AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `DCW-V0-035`, `DCW-V0-036`, `DCW-V0-037`, `DCW-V0-038` and `LCP-V0-020` as written in the spec text carried by this pull request. Their status text
changes from "proposed, pending owner acceptance" to "accepted, decision 0510".

## Limits

The evidence is the tests named in the PR build logs and traceability rows. Live qualification and `make gate` are `NOT_RUN`. No bound is increased. This decision is not a standing rule for other requirements.

## Rollback

Revert this decision and return the five status texts to proposed. To withdraw the behavior, also revert the corresponding `internal/dogfoodflow`, `internal/trace`, `internal/localcompletion` and `script/dogfood-*` changes of PR #762, then regenerate `docs/specs/REQUIREMENTS.tsv`. No stored state changes.
