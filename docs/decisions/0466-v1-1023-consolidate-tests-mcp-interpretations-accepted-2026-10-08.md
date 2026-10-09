# Decision 0466 — TCN-V0-012 MCP tool interpretations accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("accept CAL-V0-197 and the TCN-V0-012 interpretations"), covering native ticket V1-1023 (GitHub
#681).

## Context

Decision 0457 accepted `TCN-V0-001..012` as written. Implementing the optional MCP tool
`corvint.consolidate_tests` (`TCN-V0-012`) required three readings the requirement text does not
state. They are recorded in the Implementation notes of
`docs/specs/test-consolidation-planner-v0.md`. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts the three interpretations as implemented:

1. The tool takes `input` and `plan` as JSON strings, so the input document is decoded strictly,
   byte for byte, by the same decoder as the CLI.
2. The tool takes no revision argument and always evaluates `HEAD`.
3. The tool refuses with `test-plan-bound-exceeded` when the framed text and its structured copy
   would exceed one MCP message less 4 KiB for the JSON-RPC envelope, rather than splitting the
   result.

## Limits

This decision settles intent only. Delivery stays experimental, the owner-run qualification stays
`NOT_RUN`, and V1-1023 is not completed by this decision.

## Rollback

Revert this decision and remove the decision reference from the Implementation notes; the
interpretations then return to unaccepted implementation choices.
