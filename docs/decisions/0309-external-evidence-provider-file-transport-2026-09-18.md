# Decision 0309 — External evidence providers start as a file-transport impact section

Date: 2026-09-18. Status: accepted (delegated call on the owner's own feature request
Beamfall/corvint#1; owner review of the intent is welcome and changes only the digest line).

## Decision

The feature request for generic external evidence providers is accepted in intent and delivered in
slices. Slice one is `docs/specs/external-evidence-provider-v0.md` (`EEP-V0`): a provider is one
JSON record file, selected with `corvint impact --provider FILE`, and its output appears only in a
separated `context.external` section with Core-assigned authority, Git-ancestry freshness,
per-path reference verification, typed relations preserved verbatim, and counted omissions.

Three parts of the request are deferred to their own slices rather than narrowed silently:

- **Command, MCP, and remote transports.** Invariant 7 and the accepted Analyzer Capability
  Contract exclude daemons, network services, and in-process plugins from Core; an executed
  provider joins as an ACC-V0 profile family.
- **Fail-closed test selection and external obligations.** These extend the affected plan's advice
  member and the Change Frontier respectively. The CEM 0.2 decoder refuses unknown fields, so the
  requested "optional CEM extension" would be a wire change and is not made.
- **Repository identities and cross-repository relationships.** Core has one root per invocation;
  a second root needs its own pinning decision first.

Two requested semantics are refused outright: a provider cannot state its own authority (Core
assigns `external-provider`), and `learned` is not a provider evidence kind (invariant 5 makes
learning local and gated).

## Rollback

Remove `internal/extevidence`, the `--provider` option and its help text, the spec, its index rows,
and this record. No other wire changes.
