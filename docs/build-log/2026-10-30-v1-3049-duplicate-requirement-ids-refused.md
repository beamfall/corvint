# Duplicate requirement IDs are refused — V1-3049

`script/gen-spec-requirements.sh` kept the lowest-ranked definition of an ID and silently
dropped every other one, so a collision between two lanes surfaced only by diffing the merged
`REQUIREMENTS.tsv`.

## Contract and implementation

- SRG-V0-547: every generator mode refuses, before it writes anything, when an ID has more than
  one unwrapped definition line across the staged specs. It prints
  `ID: defined more than once: path:line path:line` and then the recovery step. This makes both
  `make spec-requirements` and `spec-requirements-check` exit non-zero.
- SRG-V0-548: the first draft of the duplicate count found false duplicates. These were wrapped
  prose continuations that happen to start with a marked ID, plus one line counted twice. The
  generator now tracks fences and continuations as RDG-V0-023 does in
  `script/check-requirement-definitions.sh`:
  - A wrapped rank-0 line still counts as a definition, because SRG-V0-003 keeps those rows.
  - It sorts after every unwrapped definition.
  - It is excluded from the duplicate count.
- On main this changes two rows, `AHI-246` and `DCI-V1-030`. Each previously took its title
  from a wrapped citation and now takes it from its real definition.
- `specindex.DefinitionLine`, the Go reader for SRG-V0-017, now uses the same ranking. Without
  that, the specindex position tests flagged exactly those two rows.

## Executed evidence

- `script/gen-spec-requirements_test.sh` passes. Case 12 fails on the pre-change generator
  with `mode 'tsv' accepted a duplicate definition`.
- The focused packages pass: `internal/specindex`, `internal/compactionkernel`,
  `internal/dashboard/roadmap` and `internal/console`.
- `make spec-requirements` on the lane tree reports no duplicates.

## Rollback

Revert the commit and rerun `make spec-requirements`.
