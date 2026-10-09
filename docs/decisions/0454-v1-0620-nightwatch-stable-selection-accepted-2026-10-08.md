# Decision 0454 — Nightwatch stable selection (TRE-V0-021..023) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("approve"), answering the questions raised for native ticket V1-0620.

## Context

V1-0620 adds a closed `expectedSelection` expectation so a Nightwatch run can be pre-admitted
even though each native test ID carries a fresh WebDriver session. The proposed requirements are
`TRE-V0-021..023` in `docs/specs/test-runner-execution-v0.md`, built in `internal/testrunner`
(`docs/build-log/2026-10-08-v1-0620-nightwatch-stable-selection.md`). AGENTS.md invariant 8 keeps
acceptance human-owned. The owner answered two questions:

1. Accept `TRE-V0-021..023` as written.
2. Keep `expectedTests` and `expectedSelection` mutually exclusive: a request that carries both
   refuses before launch (`TRE-V0-022`), rather than one silently taking precedence.

## Decision

The owner accepts `TRE-V0-021..023` as written, including the mutual exclusion. Their status
lines change from `proposed (V1-0620)` to `accepted (decision 0454; V1-0620)`, and the spec intent
in the header, digest, `docs/specs/README.md` and `INDEX.json` records the accepted subset. The
rest of `TRE-V0` stays `proposed`.

## Limits

This decision settles intent only. Delivery stays `experimental`. No runner qualification, CEM 1.0
promotion or release claim is implied, and V1-0620 is not completed by this decision.

## Rollback

Revert this decision, restore `proposed (V1-0620)` on the three requirements and `proposed` in the
spec header, digest, README row and INDEX entry, then regenerate `docs/specs/REQUIREMENTS.tsv`.
