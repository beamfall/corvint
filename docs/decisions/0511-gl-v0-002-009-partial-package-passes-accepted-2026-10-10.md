# Decision 0511 — Gate ledger partial package passes (GL-V0-002, GL-V0-009 amendments) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner answer in chat, 2026-10-11 UTC
("Those 8 PRs, finish them"), confirming acceptance of the listed requirements for pull requests
#758, #760, #761, #762, #764, #765, #766 and #767.

## Context

PR #764 (V1-0245) amended `GL-V0-002` and `GL-V0-009` in `docs/specs/gate-ledger-v0.md`: after a resolved Go package batch exits, including on a nonzero exit, each package whose complete, valid `go test -json` stream proves a pass is recorded under its own key, while any malformed, truncated or contradictory stream refuses every record from that batch. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts the `GL-V0-002` and `GL-V0-009` amendments as written in the spec text carried by this pull request. Their status text
changes from "proposed, pending owner acceptance" to "accepted, decision 0511".

## Limits

The evidence is the gate-ledger tests named in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-10-gate-ledger-partial-pass.md`. Live qualification and `make gate` are `NOT_RUN`. This decision is not a standing rule for other requirements.

## Rollback

Revert this decision and return the two amendment status texts to proposed. To withdraw the behavior, also revert the per-package recording of PR #764 so a failed batch records nothing again, then regenerate `docs/specs/REQUIREMENTS.tsv`. The record format stays additive (GL-V0-009); no migration is defined.
