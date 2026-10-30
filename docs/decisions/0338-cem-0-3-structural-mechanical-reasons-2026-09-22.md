# Decision 0338 — cem/0.3 structural mechanical reasons

Date: 2026-09-22. Status: accepted (experimental delivery; ticket V1-0087).

## Context

CEM 0.1 and 0.2 register two mechanical reasons, `whitespace-only` and `line-ending-only`, both
proved from a hunk's removed and added bytes. Renames, function moves, import reorders and
formatter runs had no mechanical disposition, so authors either left them `unknown` or cited them
as evidence-backed changes that a reviewer must audit by hand. Ticket V1-0087 asks for a
deterministic classifier that the LLM-free verifier recomputes itself and refuses when it cannot.

## Decision

- Introduce `cem/0.3` as an additive profile: the `cem/0.2` wire shape plus the reasons `rename`,
  `move`, `import-reorder`, and `formatter-only`. 0.1 and 0.2 documents reject that vocabulary;
  `prepare` keeps emitting 0.2 and `mark` upgrades to 0.3 only when a structural reason is used.
- Prove each reason from the base blob at `baseRevision` and the parsed patch with the Go standard
  library only (`go/scanner`, `go/parser`, `go/ast`, `go/format`); no target checkout, external
  command, or author statement is an input. Any parse or format failure refuses the claim as
  `unproven-mechanical`.
- Prove `rename`, `import-reorder`, and `formatter-only` on a single hunk applied in isolation and
  `move` on the whole file group, since a move is a paired removal and insertion.
- Keep `rename` file-local and unexported with the conservative rules in
  `docs/specs/cem-0.3-structural-mechanical.md` (`CEM-SM-007`); exported or selector-bound names
  stay `unknown`.
- Ship no `docs/cem-0.3.schema.json` and no `protocol/cem-0.3` vectors in this change; both sit
  inside the Apache-2.0 boundary enumerated by `LICENSING.md` and are follow-up work, as is
  widening the frontier and dashboard profile sets that still pin 0.1 and 0.2.
- Rollback: drop `Spec03` from `wire.ParseMap`; 0.3 maps then fail as unsupported spec while every
  committed 0.2 map stays valid.
