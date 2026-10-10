# Decision 0503 — VS Code error text and evidence document opening (VSC-V0-077, VSC-V0-078) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("Accept CAL-V0-212 (#750), VSC-V0-077 and VSC-V0-078 (#752), and the decision 0501 record for #749.").

## Context

V1-0439 and V1-0440 changed the VS Code extension adapter: process failures showed unbounded or
unsanitized stderr, and evidence diagnostics opened text documents and discarded evidence beyond the
current file length. The lane proposed `VSC-V0-077` and `VSC-V0-078` in
`docs/specs/vscode-extension-v0.md`. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts both requirements as written:

- `VSC-V0-077`: process failures retain their stable display code prefix and show the first
  non-empty stderr line, decoded as UTF-8 with replacement for malformed bytes and sanitized by the
  shared display policy to at most 200 Unicode scalars; blank lines are skipped; absent or blank
  stderr keeps the "local Corvint process failed" fallback; later lines are never displayed.
- `VSC-V0-078`: evidence diagnostics do not open text documents; after the existing containment
  validation a sane 1-based line builds a zero-based range from character 0 to a large end column,
  leaving document-bound clamping to VS Code; evidence beyond the file length is not discarded;
  severity, identity, caps and stale-generation suppression are preserved.

## Limits

This decision settles intent only. The evidence is `extensions/vscode/test/error-presentation.test.ts`
and `extensions/vscode/test/presentation.test.ts`. Real Electron/UI clamping, installed-host
qualification and `make gate` are `NOT_RUN`; the extension's deferred/FALLBACK status is unchanged.

## Rollback

Revert this decision and return the two status texts to proposed. To withdraw the behavior, also
revert the adapter edits in `extensions/vscode/src/extension.ts` and
`extensions/vscode/src/presentation.ts`, then regenerate `docs/specs/REQUIREMENTS.tsv`. No stored
state changes.
