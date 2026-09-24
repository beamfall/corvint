# Decision 0379: the release tag names the notes commit on the gated revision

Date: 2026-09-24. Status: proposed; owner acceptance NOT_PRODUCED. Ticket V1-0223. Amends
`ARTIFACT-RDY-V0-003` in `docs/specs/release-artifact-integrity-v0.md` and steps 9 and 10 of
`docs/RELEASE-RUNBOOK.md`.

## Context

`RELEASE-RUNBOOK.md` step 9 commits the release notes after the full gate, and step 10 tags that
notes commit. `script/release-checklist` passed its tag row only when the tag pointed at HEAD, the
gated `FULL_COMMIT`. Run from `FULL_COMMIT` after tagging, the row failed; run from the notes
commit, the full-gate receipt for HEAD was lost. Both published tags follow the runbook:
`v0.7.0` is `678c1b1`, whose only parent is `41f2b68`, and `v0.8.0` is `516439f`, whose only
parent is `31d68b4`. The `v0.8.0` notes commit also adds decision 0375, a documentation file.

## Decision

The release tag names the notes commit. Run from the gated `FULL_COMMIT`, the checklist's tag row
passes only when all of the following hold:

- the tag commit has exactly one parent, and that parent is HEAD;
- the tag commit changes `docs/RELEASE-NOTES.md`;
- every path it changes relative to HEAD is a Markdown file under `docs/`.

No file under `docs/` is a build input: no `//go:embed` directive names one. The archives, the
build number and the full-gate receipt therefore all bind to `FULL_COMMIT`, and the tag adds only
documentation. A tag on HEAD itself, on a later commit, or on a notes commit that changes any
other path fails the row. The checklist still never creates, moves or deletes a tag.

## Evaluation of the published tags

With this rule, `script/release-checklist` run from `41f2b68` reports `PASS tag` for `v0.7.0`, and
run from `31d68b4` reports `PASS tag` for `v0.8.0`. Neither tag moves.

## Rollback

Restore the HEAD-equality tag row in `script/release-checklist`, its test and the previous
`ARTIFACT-RDY-V0-003` and runbook wording. The pre-promotion checklist then fails the tag row for
every release that follows the runbook again.
