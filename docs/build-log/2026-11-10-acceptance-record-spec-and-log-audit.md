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

- The audit exits 0 unless `--strict` is given; the make target passes `--strict`.
- The log check lives in its own script. `script/spec-acceptance-audit.sh` keeps its one job.
- `kind` takes a third value, `change`, checked against the change folders, live or archived.
- The spec states the no-key fallback (owner decision H14 (2026-11-10)) in its own section and
  leaves the requirement wording of ACR-V1-011 as the change wrote it.

## Finding and the owner's answer

The first run reported 77 `MODE_MISMATCH` lines: the log lists the 77 PROD requirements as
`owner` (decision 0596) and the plan marked each `(proposed 2026-11-09)`. Rewriting the markers
changes the plan file that decision 0596 names by blob. Asked how to reconcile the two, the
owner answered "Ignore markers in pin" (owner decision H17, 2026-11-10).

Delivered on that answer:

- The plan's 77 PROD requirement lines now read `(accepted, decision 0596; V1-5272)`; 76 of
  them were re-wrapped to stay within the line limit. No other word of the plan changed.
- `script/acceptance-plan-pin-check.sh` compares the plan with the pinned blob over words and
  ignores the acceptance marker. It also ignores citation line numbers and the record's own
  path. Those two are lane decisions: the plan on `main` already differed from the blob in
  exactly those ways (three repinned citations and the record's rename from its proposed name).
- The log audit reports 0 findings and now runs with `--strict` in `make spec-acceptance-audit`.

## Limits

- The audit cannot tell who wrote a row. Identity is by convention and `NOT_VERIFIED`.
- An accepted change with no `change` row is not detected.
- ACR-V1-009 and ACR-V1-010 (release record format and workflow read) and the stale-quote
  audit are `NOT_RUN`.
- The plan pin no longer covers acceptance markers, citation line numbers or formatting.

## Rollback

Revert the commit. No stored state or wire format changes.
