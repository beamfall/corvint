# Spec-metadata selection witnesses for the index contract check

Date: 2026-10-23. Lane: claude/cloud-v1-0515 (base 368b80d4). Ticket V1-0515. Requirement
`SRG-V0-045` in `docs/specs/spec-requirement-index-generator-v0.md`.

## Problem

On an rc.1 build, a change to one `docs/specs/INDEX.json` claim (217 characters, not carried by the
Agent digest or the README row) passed the focused tests and the generator byte-compare, then
failed `internal/specindex` `TestIndexCoversSpecsAndHeaders` in CI.

## Reproduction on the base

With `corvint affected --base 368b80d4` built from the base, each of the three metadata-only
changes already selects `internal/specindex` with a `PATH_LITERAL_READER` witness: an INDEX.json
claim edit, a line edit in a spec's Agent digest block, and a `docs/specs/README.md` row edit. The
selection gap the ticket observed does not reproduce on the base. The read scope `docs/specs/`
and, since beamfall/corvint#496, `spec-requirements-check` running `./internal/specindex` after the
byte-compare, closed it. Nothing pinned either one, so a narrowed scope would only show up again
in CI.

## Decision

- No selector rule changes. Instead, a new witness in `internal/specindex` checks the committed
  declaration through the selector's own `affected.InReadScope` predicate. It covers INDEX.json,
  README.md, REQUIREMENTS.tsv and every indexed spec, plus one control path that must stay outside.
  A copy of the predicate was rejected because it could drift away from the selector. As a cost,
  `internal/specindex` now imports `internal/liveverify/affected`, which brings five small internal
  packages into its dependency closure, so a change to those packages now selects it too.
- The contract check becomes `checkIndex(root, report)`, so the same code runs on the repository
  and on a fixture. The negative fixture reproduces the incident: it reports exactly the length,
  digest and README findings. The aligned fixture reports none.
- The generator byte-compare is still described as a requirements check only. The plan keeps
  `plan.scope` `UNKNOWN` and its unknowns.

## Rollback

Delete `internal/specindex/selection_test.go`, its `.corvint/test-read-scopes.json` and
`.corvint/test-read-scope-literals.json` entries, and SRG-V0-045.
