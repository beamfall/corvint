# External Evidence Provider MCP V0

Owner: Russell Lewis
Date: 2026-09-20
Intent status: accepted (decision 0324)
Delivery status: implemented
Authoritative inputs: issue 11 and the owner's direction to resolve it; decision 0317;
`docs/specs/external-evidence-provider-transports-v0.md`.

## Agent digest
- Claim: `impact --provider-mcp ARGV_JSON` invokes one bounded local MCP tool and preserves the provider record bytes.
- Status: accepted (decision 0324)/implemented; checked by `TestMCPTransportConformance`.
- Exists: `internal/extevidence/mcp.go`, contained interactive subprocess support in `internal/procgroup`.
- Blocked on: no delivery prerequisite; external server interoperability beyond this bounded profile is not claimed.
- Read next: Requirements; Acceptance; Rollback.

## User and boundary

The operator supplies one trusted local stdio MCP executable instead of maintaining a record-file
wrapper. This is an external-provider profile, not an ACC analyzer that can contribute Core authority.
It grants no networking or credentials; the executable retains the operator's privileges, exactly
as the command transport does. Process groups contain cooperating descendants, not hostile processes
that deliberately escape groups. The simpler baseline remains an operator-produced record file.

## Requirements

- `EEP-MCP-001`: `impact --provider-mcp ARGV_JSON` MUST be the sole selector; argv validation,
  shared four-provider bound, repository bindings, and incompatible flags MUST match the command
  profile. File paths never select a session. No shell or PATH lookup is performed.
- `EEP-MCP-002`: The profile MUST pin MCP `2025-11-25`, send initialize with empty client
  capabilities, wait for an exact-version result advertising tools, send notifications/initialized,
  then call exactly `corvint_evidence` with `{}` arguments. Only two server responses are admitted;
  server requests, notifications, error responses, wrong IDs, duplicate keys and other envelopes
  MUST close the session. Three outgoing frames are each at most 4096 bytes. Incoming frames are
  each at most 8 MiB; aggregate stdout is at most 8 MiB and stderr at most 64 KiB.
- `EEP-MCP-003`: The tools/call result MUST contain exactly one text block and no structuredContent,
  with isError absent or false. Its UTF-8 text bytes, at most 1 MiB, MUST pass unchanged to the same
  record decoder, freshness/reference checks, learned exclusion and separated authority as a file.
  MCP metadata, server instructions and stderr MUST never reach the receipt.
- `EEP-MCP-004`: The session MUST share the command's environment and working-directory policy,
  ten-second wall bound, zero retries, process-group containment and cleanup proof. stdin is closed
  after the tool response, and EOF plus a successful process exit are required before record use.
  Failure, cancellation, malformed frames, excess bytes, timeout, unavailable executable and
  unproven cleanup MUST produce a closed provider row with no partial record or digest.

## Acceptance

`TestMCPTransportConformance` runs the existing EEP-V0/V1/V2 and selection fixtures unchanged and
compares full sections after source normalization. `TestMCPTransportFailures` exercises protocol,
timeout, unavailable, output bounds and cancellation. `TestMCPDescendantCleanup` proves normal and
interrupted sessions leave no cooperating descendant. `TestImpactProviderMCP` covers the public flag.
No remote MCP, discovery, auth, sampling, roots, elicitation, resources, streaming, retries, or
general-purpose MCP client is supplied. Unsupported servers fail closed rather than negotiate.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `EEP-MCP-001` | `cmd/corvint/main.go`, `internal/extevidence/mcp.go` | `TestImpactProviderMCP` |
| `EEP-MCP-002` | `internal/extevidence/mcp.go` | `TestMCPTransportFailures` |
| `EEP-MCP-003` | `internal/extevidence/mcp.go` | `TestMCPTransportConformance` |
| `EEP-MCP-004` | `internal/procgroup`, `internal/extevidence/mcp.go` | `TestMCPTransportFailures`, `TestMCPDescendantCleanup` |

## Rollback

Remove the MCP flag and session implementation; existing file/command records and wire formats stay
compatible. Revert decision 0324's MCP supersession. No data migration or persistent service exists.
