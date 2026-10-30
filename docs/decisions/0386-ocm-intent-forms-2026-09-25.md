# Decision 0386 — OCM reads declared ADR and roadmap intent forms (experimental)

Date: 2026-09-25. Status: proposed. Authority: the owner approved route (b) of ticket V1-0259 and
delegated the contract; acceptance of the intent remains the owner's.

## Context

`OCM-V0-001` accepts only one `## Requirements` section, which conflicts with invariant 6 for a
repository such as Beamfall, whose intent lives in ADR `## Decisions` items and roadmap tickets with
Acceptance lines. The contract is `docs/specs/ocm-intent-forms-v0.md` (`OIF-V0`).

## Decision (proposed)

- The caller declares the form with `corvint ocm prepare --intent-form
  requirements|adr-decisions|roadmap-acceptance`. Nothing infers it from a path or content.
- A non-default form is recorded as `intentScope.form`, so every later action re-derives with the
  same grammar and default-form maps stay byte-identical.
- ADR decision items become `ADR-NNNN-D<N><a>` from the front-matter `adr:` number; roadmap
  tickets with Acceptance become obligations under their own ticket IDs, and tickets without one
  are listed in `excludedTickets`.
- Every parse fails closed: ambiguous sections, unnumbered or malformed items, and duplicates are
  refused, never skipped.
- LRF and the change universe refuse declared forms until `internal/lrf` has a statement grammar
  for them.

## Alternatives rejected

- A repository-owned mapping file: a new configuration format, glob matching, and one more pinned
  authority input.
- A `form:path` prefix on `--intent`: Git paths may contain `:`.
- Content sniffing: guessed authority, contrary to invariant 2.

## Consequences and rollback

The dogfood wrapper cannot pass a form yet; that needs its own change. Reverting leaves
default-form maps untouched and makes form-carrying maps fail `unknown-field`.
