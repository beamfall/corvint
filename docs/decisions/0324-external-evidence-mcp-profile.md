# Decision 0324 — Accept a bounded local MCP evidence profile

Date: 2026-09-20. Status: accepted.

## Authority and decision

After review of the three open issues including #11, the owner directed “you do it. and all the
issues should be resolved,” then “work in parallel to get all three done.” Issue 11 requires accepted
transport profiles. This accepts `docs/specs/external-evidence-provider-mcp-v0.md` and supersedes
decision 0317's deferral for exactly that profile. It does not accept arbitrary MCP features.

The owner subsequently answered “appoved” to the explicit request accepting and shipping the
bounded MCP-stdio and separately built opt-in HTTPS profiles while keeping Core offline.

MCP needs an interactive session rather than the closed stdin used by command providers. Admit
bounded stdio through the existing process-group lifecycle, without a network client, and preserve
the existing strict record decoder. One fixed tool, three outbound frames, two responses, exact
protocol version and a single text block bound the additional protocol. No server request or
notification is executed, no capability is granted and no record becomes Core authority.

## Rollback

Remove the MCP selector and session support, restoring 0317's deferral. File and command transports
remain byte-compatible. There is no persistence to migrate.
