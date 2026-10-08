# Decision 0451 — flow document maintenance (FDM-V0) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("approve all, and you can self approve these tickets"), covering native ticket V1-0465.

## Context

V1-0465 requires automatic developer and flow documentation for 1.0. Its bounded safety path is
`docs/specs/flow-document-maintenance-v0.md` (`FDM-V0-001..007`): digest-bound preview/apply,
conditional two-file publication and committed `flow-doc-maintenance/0` receipts
(`docs/build-log/2026-10-01-flow-doc-maintenance-port.md`,
`docs/build-log/2026-10-01-flow-doc-maintenance-test-hardening.md`). The spec listed owner
acceptance as a blocker.

## Decision

The owner accepts `FDM-V0-001..007` as written. The spec intent changes from `proposed` to
`accepted (decision 0451; V1-0465)` in the header, digest, `docs/specs/README.md` and `INDEX.json`,
and owner acceptance is removed from the recorded blockers.

## Limits

This decision settles intent only. Delivery stays `experimental`: release-boundary qualification,
separate docs-MCP parity for maintained output and concurrent-process qualification are recorded as
NOT_RUN, so the spec's own promotion condition is not met. HDC rendering, general inferred behavior
and the wider 1.0 documentation obligations stay outside this profile. V1-0465 stays open.

## Rollback

Revert this decision and restore `proposed` and the owner-acceptance blocker in the spec header,
digest, README row and INDEX entry, then regenerate `docs/specs/REQUIREMENTS.tsv`.
