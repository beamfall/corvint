# Decision 0465 — local-completion envelope ok mirrors the exit status (LCP-V0-017) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("accept LCP-V0-017 too"), covering native ticket V1-1012.

## Context

V1-1012 makes `corvint dogfood verify` and `finish` report a top-level `ok:false` in their `corvint-local-completion/0` envelope whenever they exit non-zero, so a failing selected check can no longer print `ok:true`. The proposed requirement is `LCP-V0-017` in `docs/specs/local-completion-policy-v0.md`. AGENTS.md invariant 8 keeps acceptance
human-owned.

## Decision

The owner accepts `LCP-V0-017` as written. Its status changes from `(proposed; V1-1012)` to `(accepted, decision 0465; V1-1012)`, together with
any status summary in the spec header that names it.

## Limits

This decision settles intent only. Delivery stays `experimental`, and V1-1012 is not completed by this
decision.

## Rollback

Revert this decision, restore the `(proposed; V1-1012)` text, then regenerate `docs/specs/REQUIREMENTS.tsv`.
