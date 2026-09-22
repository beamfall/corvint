# Decision 0010 — accept repository query and Go archive profiles

Date: 2026-08-31. Status: accepted. Authority: repository owner, verbatim instruction "accept
both" (2026-08-31).

## Scope

The instruction accepts exactly these two previously proposed amendments:

1. `GPK-V0-043`, the general `repository`-intent `corvint query` profile in
   `docs/specs/go-production-kernel-migration-v0.md`.
2. `ARTIFACT-GO-V0-001..007`, the Go binary archive profile in
   `docs/specs/release-artifact-integrity-v0.md`.

The accepted text is unchanged except for its status and removal of proposal-only wording.
Implementation must satisfy each profile's named acceptance evidence before either delivery status
may advance.

## Explicit exclusions

This decision does not accept `ARTIFACT-GO-V0-008`, either harness-authority option, create or
accept the sealed 20-PR cohort, select a signing option, authorize a signing key or identity, authorize tagging,
publication, or promotion, or authorize removal of the two Python verification lines from
`AGENTS.md`. The `AGENTS.md` question remains in `docs/agent-memory/questions.md` for an explicit
owner ruling.

## Consequences

Roadmap items 34b and 43 are unblocked for implementation. Item 43 authority stops at local archive
assembly and offline checking. The item-44 checklist remains read-only and non-authorizing. Every
release action beyond that checklist remains owner-gated.
