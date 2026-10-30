# Decision 0457 — test consolidation planner (TCN-V0) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("accept the specs for V1-1022/1023/1024"), covering native ticket V1-1023 (GitHub #681).

## Context

V1-1023 asks for a read-only planner that groups flow variations into fewer focused tests. The
proposal is `docs/specs/test-consolidation-planner-v0.md` (`TCN-V0-001..012`). AGENTS.md invariant 8
keeps acceptance human-owned.

## Decision

The owner accepts `TCN-V0-001..012` as written, including the optional MCP tool of `TCN-V0-012`. The
spec's Owner questions are settled by the positions the requirements take. Per-requirement status
text, the spec header and digest, `docs/specs/README.md` and `INDEX.json` change from `proposed` to
`accepted (decision 0457; V1-1023)`.

## Limits

This decision settles intent only. Delivery stays `not-started`: no planner, command or MCP tool
exists, the owner-run qualification stays `NOT_RUN`, and V1-1023 is not completed by this decision.

## Rollback

Revert this decision, restore `proposed` in the spec header, digest, requirement status text, README
row and INDEX entry, then regenerate `docs/specs/REQUIREMENTS.tsv`.
