# Decision 0508 — Dogfood stop reuses immutable facts accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("acceptance is given"), confirmed by the owner's answer in chat, 2026-10-11 UTC ("Those 8 PRs,
finish them").

## Context

V1-1124 proposed LCP-V0-019 in PR #761: an enrolled dogfood event pins repository identity once and reuses immutable facts while re-reading private inputs and cleanliness. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `LCP-V0-019` as written in the spec text carried by this pull request. Their status text
changes from "proposed, pending owner acceptance" to "accepted, decision 0508".

## Limits

The evidence is the tests named in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-10-v1-1124-stop-git-calls.md. Live qualification and `make gate` are `NOT_RUN`. This decision is not a standing rule for other
requirements.

## Rollback

Revert this decision and return the LCP-V0-019 status text to proposed; to withdraw the behavior also revert the cmd/corvint and internal/localcompletion changes of PR #761 and regenerate `docs/specs/REQUIREMENTS.tsv`. No stored state changes.
