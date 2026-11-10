# 2026-11-10: acceptance record spec and acceptance-log audit (V1-5272)

Change `acceptance-record`, tasks 1.2, 1.3 and 2.6. Requirements ACR-V1-006, ACR-V1-007 and
ACR-V1-008.

## What changed

- `docs/specs/acceptance-record-v1.md` is the canonical spec for ACR-V1-001 to ACR-V1-012,
  created from the change's spec delta with the requirement IDs unchanged. Each requirement is
  marked with the standing auto-accept, not as owner-accepted: the wording is lane-drafted.
- `script/acceptance-log-audit.sh` compares `docs/specs/openspec/acceptance-log.tsv` with
  `docs/specs/REQUIREMENTS.tsv` and the plan's checklist rows. `make spec-acceptance-audit` runs
  it after the mode listing. `script/acceptance-log-audit_test.sh` covers a missing row, a mode
  mismatch, unknown and duplicate rows, and malformed input.
- The log gains one `change` row, `acceptance-record`, mode `auto`.

## Decisions made in the lane

- The audit is a report: it exits 0 unless `--strict` is given. It could not gate at delivery,
  for the reason under "Finding".
- The log check lives in its own script. `script/spec-acceptance-audit.sh` keeps its one job.
- `kind` takes a third value, `change`, checked against the change folders, live or archived.
- The spec states the no-key fallback (owner decision H14 (2026-11-10)) in its own section and
  leaves the requirement wording of ACR-V1-011 as the change wrote it.

## Finding

The audit reports 77 `MODE_MISMATCH` lines on the committed tree. The log lists all 77 PROD
requirements as `owner` (decision 0596), and the plan still marks each one `(proposed
2026-11-09)`, which the index reads as `unrecorded`. ACR-V1-007 wants an `owner` row to sit with
an `(accepted, decision NNNN; V1-NNNN)` marker.

Rewriting the markers changes the plan file, and decision 0596 names the plan by commit and blob
(ACR-V1-001). The lane did not make that edit. It needs the owner's word on one of: re-pin the
record to the plan with rewritten markers, or let the stale-plan audit ignore the acceptance
marker of a requirement line.

## Limits

- The audit cannot tell who wrote a row. Identity is by convention and `NOT_VERIFIED`.
- An accepted change with no `change` row is not detected.
- ACR-V1-009 and ACR-V1-010 (release record format and workflow read) and the stale-plan and
  stale-quote audits are `NOT_RUN`.

## Rollback

Revert the commit. No stored state or wire format changes.
