# Decision 0463 — sandbox launcher refusal reported as unavailable (TCQ-V0-059) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("accept TCQ-V0-059 too"), covering native ticket V1-0651.

## Context

V1-0651 makes a mutation run whose whole runner output is a macOS sandbox launcher refusal report `ErrSandboxUnavailable` (an inconclusive, unavailable result) instead of a surviving or killed mutant, without echoing the refusal text. The proposed requirement is `TCQ-V0-059` in `docs/specs/test-claim-qualification-v0.md`. AGENTS.md invariant 8 keeps acceptance
human-owned.

## Decision

The owner accepts `TCQ-V0-059` as written. Its status changes from `(proposed 2026-10-08, ticket V1-0651)` to `(accepted 2026-10-08, decision 0463; ticket V1-0651)`, together with
any status summary in the spec header that names it.

## Limits

This decision settles intent only. Delivery stays `experimental`, and V1-0651 is not completed by this
decision.

## Rollback

Revert this decision, restore the `(proposed 2026-10-08, ticket V1-0651)` text, then regenerate `docs/specs/REQUIREMENTS.tsv`.
