# Decision 0351 — An optional `capabilities` declaration in external provider records

Date: 2026-09-22. Status: accepted (ticket V1-0102, a delegated call on Beamfall/corvint#64).
Adds `EEP-TR-012`, `EEP-TR-013`, and `EEP-TR-014`; amends `EEP-V0-001`, `EEP-V0-005`,
`EEP-V1-001`, and `ETS-V0-003`. It changes no existing record's decode or receipt.

## Context

Issue 64 asks for explicit capability negotiation between Core and a provider. The only shipped
transports are the record file and the contained local command (`EEP-TR-001`); MCP is an accepted
profile and not shipped (`EEP-TR-009`), and a remote transport is NO-GO (`EEP-TR-010`). A
transport-level handshake would therefore either invent a protocol the shipped transports cannot
carry or apply to nothing. What every transport already carries is the record bytes.

Today a provider that supports only, say, `declared` evidence has no way to say so; Core silently
composes whatever the record holds, and a consumer that needs `observed` evidence learns nothing.
Conversely a record written for a newer schema is `invalid` with a decode reason rather than a
statement of what the provider supports.

## Decision

- The declaration is one optional top-level record member `capabilities` with the optional lists
  `schemas` and `evidence_kinds`, valid on every record version. Absent means undeclared and
  changes nothing: no existing record decodes or composes differently, which the unchanged
  conformance corpora prove. A present list, even empty, is the complete set the provider
  supports. The member is validated like the rest of the record (identifier bound, at most 32
  entries per list, no duplicates, no unknown member).
- Core checks a declared list against what the invocation requires before anything from the
  record composes: the record's own `schema` must be declared; under `impact` every evidence kind a
  relation uses must be declared; under `affected` `declared` or `observed` must be declared,
  because only those qualify a selection (`ETS-V0-005`). A shortfall yields one closed provider
  row in the new state `unsupported` whose reason is Core-authored and names the provider id and
  the first missing capability. Under `affected` that row blocks the selection
  (`provider-unsupported`).
- A declaration never widens acceptance: `learned` stays excluded whether or not it is declared,
  unknown identifiers are carried and ignored, and nothing in ranking, authority, freshness, or the
  core receipt reads the member.
- The two conformance-selection cases over `testdata/conformance-selection/capabilities.json`
  (`positive-capabilities-sufficient`, `negative-capabilities-unsupported`) and
  `TestCapabilitiesNegotiation` pin present-and-sufficient, present-and-missing, and absent.

## Rollback

Remove `Capabilities` from `Record` and `Record1`, `validateCapabilities`,
`rootRepository.unsupported` and its call in `decodeRecord`, `StateUnsupported`, the fixture and
its two cases, and the new test file; revert `EEP-TR-012` to `EEP-TR-014` and the amended
requirements. A record carrying the member then becomes `invalid` under strict decode, and every
other record is unchanged.
