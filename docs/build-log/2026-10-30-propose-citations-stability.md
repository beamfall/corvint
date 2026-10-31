# propose-citations: no modified-file lines, same-change requirements, refresh

Lane friction-dogfood-w5, tickets `V1-3145`, `V1-3151` and `V1-3240`; immutable base
`c5d5f0829c28756ee7b202caf80a3f3fc37fa85b`. Requirements `DCW-V0-439..442`.

## V1-3145: drafts cited lines inside files the diff modifies

When a hunk had no contract line, the drafter fell back to the nearest unique line of the hunk's
own file. `cem cite` accepts such a line because it lies outside the hunk, but it is not BASE
evidence for the change. A line below any hunk of its file is also shifted by that hunk.

Decision (`DCW-V0-439`): the own-file fallback is never drafted. The hunk gets an `unknown` row that
names the nearest stable line as a review hint only. An anchored (`DCW-V0-161`) line in a modified
file is drafted only when every hunk of that file starts after it, so no hunk overlaps or shifts
the span. Two cases still cite lines in modified files, on purpose:
- an anchored contract line above every hunk of its file;
- the `DCW-V0-385` sealed-proposal rows, which stay on stable lines of `docs/DOGFOOD.md`, as the
  parent lane required.

The ticket's wording ("never proposes lines in files the diff modifies") is therefore met for every
unstable span, not literally for every line.

## V1-3151: whole-file and data rows, same-change requirements

Drafts cited `worklist.json:1`, `.agent-evidence/...json` and
`protocol/cem-1.0/stable/MIGRATION.json:178`: records that merely list a path. They also left a new
test file `unknown` without saying why.

Decision (`DCW-V0-440`): the drafter no longer cites single-line files or `.json`, `.jsonl` and
`.agent-evidence/` records. A hunk that names a requirement ID this same change defines, in ID,
compact or lower-underscore form, keeps an `unknown` row that names the ID and the file adding it.
The 10-30 addendum settles that same-change intent is not BASE evidence, and `cem cite` cites BASE
spans only. The AC's "cites the test to the requirement" is met as an explicit, named uncertainty,
not as a cite of a HEAD line.

## V1-3240: stale hunk IDs after a later edit

A new build-log file edited after the plan was drafted got a new hunk ID. The bind refused with a
bare map mismatch, and authors diffed IDs by hand.

Decision (`DCW-V0-441`): the mismatch names the row, the stale ID and the current candidate hunk,
and the `fix:` line names `--refresh`.

Decision (`DCW-V0-442`): `propose-citations --refresh PLAN` handles each PLAN row as follows:
- a current row is carried verbatim;
- a re-hunked row is re-keyed by path and content, then by path and BASE range, with a note to
  review it;
- any other row is dropped with a note, and its hunk is drafted afresh.

Stale IDs resolve from the first-parent commits since BASE and the HEAD reflog (32 each), which
covers amend and renumber. `docs/CHANGE-FLOW.md` did not exist; it is new and orders the build-log
write before the draft.

## Evidence

- `TestProposeCitationsDCWV0439NeverDraftsModifiedFileLines`: PASS; fails on BASE (own-file and
  shifted spec rows).
- `TestProposeCitationsDCWV0440SameChangeRequirementStaysUnknown`: PASS; fails on BASE
  (`.agent-evidence` and `1:1` cites).
- `TestProposeCitationsDCWV0441V0442RefreshRekeysReHunkedNewFile` (commit and amend): PASS; fails
  on BASE (no `--refresh`).
- Existing `TestProposeCitations*` tests were updated to the unknown-row contract and pass.
- `make capability-catalog` and `make capabilities-check` pass.

## Batch integration (2026-10-30)

Batch `claude/batch-1030c` reconciled this lane with `DCW-V0-393` (V1-3144), which landed on main
first and forbids drafting any line of a modified file. `DCW-V0-393` governs: the anchored-line
exception above and the `shifted` rule were dropped, and `DCW-V0-439` keeps only the
nearest-stable-line review hint in the unknown row's note. `DCW-V0-440`'s one-line skip changed
main's uniform-plan fixture (`index.md` gained a second line).

## Rollback

Revert `propose.go`'s review-hint clause (`preferredAnchor`, `declaration`), `propose_refresh.go`, the `mapIDs` check in
`citationPlanMismatch`, and the `--refresh` flag wiring.
