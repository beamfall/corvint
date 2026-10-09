# Decision 0458 — step-level negative controls (LPCV-V0-057..070) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("accept the specs for V1-1022/1023/1024"), covering native ticket V1-1024 (GitHub #682).

## Context

V1-1024 asks for generated step-level negative controls by fault injection for Playwright tests. The
proposal is `LPCV-V0-057..070` in `docs/specs/live-proof-carrying-verification-v0.md`; the rest of
`LPCV-V0` is already accepted by decision 0047. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `LPCV-V0-057..070` as written. Their status text, the section heading, the spec
header (intent line 5) and digest, `docs/specs/README.md` and `INDEX.json` change from
`LPCV-V0-057..070 proposed (V1-1024; GitHub #682)` to
`LPCV-V0-057..070 accepted (decision 0458; V1-1024; GitHub #682)`.

## Limits

This decision settles intent only. Delivery stays `not-started`: nothing is implemented, no result may
join the strength axis before the `LPCV-V0-070` live matrix is retained, and V1-1024 is not completed
by this decision.

## Rollback

Revert this decision, restore `proposed (V1-1024; GitHub #682)` on the fourteen requirements and in the
spec header, digest, section heading, README row and INDEX entry, then regenerate
`docs/specs/REQUIREMENTS.tsv`.
