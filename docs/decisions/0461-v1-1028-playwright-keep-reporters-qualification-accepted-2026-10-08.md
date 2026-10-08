# Decision 0461 — Playwright keep-reporters qualification (PWP-V0-014..018) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("accept the specs for batch H too"), covering native ticket V1-1028 (GitHub #686).

## Context

V1-1028 lets an adopter who must keep the project's own Playwright reporters record local
evidence that they did not change what the provider observed. The proposed requirements are
`PWP-V0-014..018` in `docs/specs/playwright-external-provider-v0.md`: provider-first reporter
order, the `qualify-keep-reporters` command, the `--keep-reporters-qualification` flag, the
closed qualification record, and abstention on missing evidence. AGENTS.md invariant 8 keeps
acceptance human-owned.

## Decision

The owner accepts `PWP-V0-014..018` as written, including provider-first order as the answer to
the `PWP-V0-013` ordering question. Their status lines change from `proposed (V1-1028; GitHub
#686)` to `accepted (decision 0461; V1-1028; GitHub #686)`. `PWP-V0-009` and `PWP-V0-010..013`
stay `proposed`.

## Limits

This decision settles intent only. Delivery stays `experimental`. Live Playwright qualification
of the command remains `NOT_RUN`, no passing keep-reporters projection is promoted, and V1-1028
is not completed by this decision.

## Rollback

Revert this decision, restore the `proposed (V1-1028; GitHub #686)` text on the five requirements,
the section heading, the digest, the traceability row and the owner question, then regenerate
`docs/specs/REQUIREMENTS.tsv`.
