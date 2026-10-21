# Lane scopes-1021d: `make spec-requirements-resolve` for generated spec-index conflicts (V1-1758)

Date: 2026-10-21. Lane: claude/spec-index-resolve-1021d (base 0106d06cd). SRG-V0-041 is accepted
under the owner's standing auto-accept of lane requirements (2026-10-11).

## Problem

When two lanes each add a new spec, both append at the end of `docs/specs/INDEX.json` and of the
`docs/specs/README.md` table, and `REQUIREMENTS.tsv` conflicts on the adjacent rows. SRG-V0-016
told the operator to regenerate, but there was no single command, and nothing guarded against
regenerating while a source spec was itself still conflicted.

## Decision

`make spec-requirements-resolve` (`script/spec-requirements-resolve.sh`) refuses when there is no
unmerged path, or when any unmerged path is not one of the three generated files. A spec conflict
therefore has to be resolved and staged first. Otherwise it checks out `ours` for each unmerged
generated file, then regenerates all three files from the merged specs with
`script/gen-spec-requirements.sh --all` and stages them. Generation stays the only writer, so the
output is the same as on a clean tree.

## Evidence

`script/spec-requirements-conflict_test.sh` prints `ok`:

- The one-section case is now resolved through the target, which first refuses while the spec is
  still conflicted.
- The new two-new-specs case merges in both directions. The conflicts are only in generated files,
  the resolved output is byte-equal to a fresh regeneration, and both specs and both IDs are indexed.

## Other tickets in this lane

The coordinator found that V1-2037, V1-2117, V1-1520 and V1-1302 are already in flight in #1400,
and V1-0988 in #1423. They are not rebuilt here. V1-2118's make targets are already delivered by
decision 0547. The `tools/**`/CI identity part (V1-2194) needs an owner decision. V1-0389 needs a
`.github/workflows/ci.yml` edit: add `traceability-tests-test` after `traceability-tests-check` in
the doc-gates line.

NOT_RUN: `make gate` (owner standing preference for scoped work).

## Rollback

Revert the merge commit. The target and script are additive, and SRG-V0-016's regeneration rule is
unchanged.
