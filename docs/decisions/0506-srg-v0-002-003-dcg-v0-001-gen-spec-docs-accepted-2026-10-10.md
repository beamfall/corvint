# Decision 0506 — Generator id and rank rules, citation scan skips accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("acceptance is given").

## Context

V1-0524 proposed SRG-V0-002 and SRG-V0-003 (unmarked definitions need a paragraph boundary), and V1-0178 amended DCG-V0-001 (the checker names skipped basename citations on stderr), in PR #759. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `SRG-V0-002`, `SRG-V0-003` and the `DCG-V0-001` amendment as written in the spec text carried by this pull request. Their status text
changes from "proposed, pending owner acceptance" to "accepted, decision 0506".

## Limits

The evidence is the generator and citation-checker tests named in the batch build log (https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-10-batch-gen-specdocs.md). Live qualification and `make gate` are `NOT_RUN`.

## Rollback

Revert this decision and return the three status texts to proposed; to withdraw the behavior also revert the `script/gen-spec-requirements.sh` and `script/check-line-citations.sh` changes and regenerate `docs/specs/REQUIREMENTS.tsv`. No stored state changes.
