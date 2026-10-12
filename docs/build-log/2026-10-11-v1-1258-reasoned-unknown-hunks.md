# 2026-10-11: reasoned unknown hunks in the daily citation plan (V1-1258, decision 0523 P3)

## Intent

Decision 0523 P3 (`docs/build-log/2026-10-11-cem-1-0-expert-panel.md`): `DCW-V0-019` refused any
citation plan that left a non-bootstrap hunk `unknown`. Every bound change therefore reported
nearly 100% supported hunks. That share came from the workflow, not from the evidence, so the
`docs/CHANGE-EVIDENCE-MAP.md` kill condition (more than 30% of material hunks legitimately
`unknown`) could never fire. Decision 0523 records this as a defect.

This change amends `DCW-V0-019` and adds `DCW-V0-046` to
`docs/specs/daily-change-evidence-workflow-v0.md`, both as proposed. It records no decision.

## Change

- New plan row form `HUNK<TAB>unknown<TAB>REASON<TAB>NOTE`.
  - It is told apart from a citation row by a literal `unknown` second field and a third field
    without `:`. A cite span is always `START:END`, so the two forms cannot be confused.
  - REASON must be `insufficient-evidence` or `conflicting-evidence`. The prepared `no-evidence`
    reason, any other reason, or a NOTE with no letter or digit refuses the whole plan as
    `invalid-citation-plan`.
- `dogfood change` runs `cem mark --disposition unknown --reason REASON` on each such row, in plan
  order and through the same citation stage, so the map and the sealed map record the reason.
  - The rows, notes included, are kept owner-only in `<git-dir>/corvint/cem-unknown-reasons.tsv`.
    The CEM unknown-reason vocabulary is closed, so the note cannot go in the map.
- `cem status --max-unknown` becomes the bootstrap allowance plus the reasoned unknown hunks of the
  committed map, without counting a bootstrap hunk twice. This applies in `dogfood change`,
  `dogfood check` and `dogfood batch-check` (including sealed-CEM admission).
  - The count is read from the map, so all three agree without a new input.
- The check `SUMMARY` line gains `unknownShare=U/M reasonedUnknown=N` after `mechanical=N`.
  - U is the unknown hunks and M the non-mechanical hunks.
  - The field is `NOT_OBSERVED` when a count is absent or inconsistent.
- The batch `CONSTITUENT` line gains `reasonedUnknown=N`.
- The report's `maximumUnknownAfterBootstrap: 0` keeps its wire value. The aggregate report schema
  requires 0, and the value now counts unreasoned unknown hunks.
- `docs/DOGFOOD.md` documents the row form and the summary fields. The kill condition in
  `docs/CHANGE-EVIDENCE-MAP.md` now names its measuring artifact: the `unknownShare` field per
  change, and the batch `CONSTITUENT` `unknown=`/`hunks=` ratio, a lower bound, across sealed
  changes.

## Review round 1 (Codex, FAIL)

- P2: a failed write of the notes file was ignored. It now refuses `cem-cite`
  `unknown-reasons-not-recorded` and cites nothing. The file is written after removing whatever
  was at the path, so it is always created owner-only. The new code is registered in the
  observation reason vocabulary.
- P2: a staged plan sequence could not be replayed. Hunks an earlier plan kept unknown with a
  reason were still treated as owed rows. Those hunks no longer need a row.
- P3: a reasoned unknown hunk on a bootstrap path was left out of `reasonedUnknown`. It is now
  counted there, and the allowance still counts it only once.
- P3: `DCW-V0-015`, the batch-check allowance text and the raw `cem status` example in
  `docs/DOGFOOD.md` named the old ceiling. They now name `DCW-V0-046`.

## Review round 2 (Codex, FAIL)

- P2: a staged plan sequence lost the notes of earlier stages, because the last plan rewrote the
  notes file with only its own rows. The file now holds `HUNK_ID<TAB>REASON<TAB>NOTE` lines in
  map order; a line from an earlier stage is kept while its hunk is still reasoned unknown, and
  the file is removed only when no line remains.
- P3: the staged binding steps in `docs/DOGFOOD.md` still required `unknown=0`. They now admit
  bootstrap and reasoned unknown hunks and require `"complete": true` and `PASS`.
- P3: the batch-check test did not prove the outcome. It is now a pair (reasoned only: ready for
  CI; one more unreasoned unknown: `constituent-cem-policy`), and the built-binary test runs the
  real `dogfood batch-check` after the seal.

## Choices

- The existing `cem mark` verb is reused instead of adding a free-text reason to the CEM wire.
- The check reports the share and does not enforce the 30% threshold. The kill condition is an
  evaluation outcome, not a per-change gate.

## Limits

- The free-text note is not durable in the sealed CEM. Only the reason code is.
- The "emit the stable profile instead of `cem/0.2`" part of P3 is not in this change.
- This change was not bound through its own dogfood loop in the lane, and `make gate` was not run.

## Evidence

Tests and gates are listed in the spec's traceability row for `DCW-V0-046`.
