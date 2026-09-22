# Decision 0317 — The MCP provider transport is a proposed profile and is not shipped

Date: 2026-09-19. Status: accepted (owner call on Beamfall/corvint#11: ship MCP only if it fits
cleanly as stdio JSON-RPC over the same contained subprocess; it does not).

Superseded for the bounded local profile by decision 0324 and owner approval on 2026-09-20.
The history below explains why the initial closed-stdin command slice did not deliver MCP.

## Decision

`EEP-TR-009` records the MCP transport as proposed and not shipped. An MCP stdio session is not a
single closed-stdin capture: the client must send `initialize`, read its response, and send an
`initialized` notification before one `tools/call`; the server may send its own requests and
notifications; and the record arrives inside a `content` or `structuredContent` envelope that needs
a second decoder ahead of the strict record decode. Each of those widens the command transport's
containment and decode contract rather than reusing it.

An operator can reach an MCP server today through a command-transport adapter they supply that
prints one record. An accepted MCP profile needs a fixed tool name and argument shape, message-count
and per-message byte bounds in both directions, refusal of every server-initiated request, one
envelope rule that yields the record bytes unchanged, and the full `EEP-TR-003` to `EEP-TR-008`
containment, proven by the same differential conformance run.

## Rollback

Nothing ships; remove the MCP section of the transports spec and this record.
