# 2026-10-11: adopter obligation aliases (GitHub #792, V1-1177)

## Intent

GitHub beamfall/corvint#792: an adopter suite names its obligations in its own scheme (`M001`,
`ABC-12.V4`), but the native obligation id grammar is `<PREFIX>-<1..6 digits, no leading zero>`.
Its titles therefore never hold a native id, so nothing credits until every title is renamed. The
coordinating agent chose the issue's option 2, additive aliases: keep the native id and the wire
shape, add an optional alias per obligation, and key everything a run records to the native id.

This change adds TOL-V0-034..036 to `docs/specs/corvint-tasks-obligation-ledger-v0.md` as proposed
requirements, pending owner acceptance. It records no decision. No non-goal or authority rule
changes.

## Change

- Seed item and folded entry gain an optional `alias`: 1..64 bytes of `[A-Za-z0-9._:-]`, starting
  and ending with a letter or digit. It is absent, not null, when unset, so alias-free bytes are
  unchanged. It is secret-screened with the seed titles.
- Seed refuses `DUPLICATE_ID` with `OBLIGATION_ALIAS_DUPLICATE:` for a repeated alias, and with
  `OBLIGATION_ALIAS_COLLISION:` for an alias holding any native id of the ledger prefix as a
  TOL-V0-010 whole token. That reuses closed codes with detail prefixes (resolved decision 7).
- `internal/tasks/obligation/names.go` resolves a native id or an alias to the native id. The
  report reader, the expected-fail rules, preflight, the batch causes and the source presence check
  and its audit use it. An alias matches as a maximal `[A-Za-z0-9_.:-]` run, with trailing `.` and
  `:` dropped. `_` extends the run but is not admitted in an alias, so `M001_extra` never names
  `M001` (review round 1). Alias spans are masked before defect-id scanning.
- The receipt audit folds the chain before the audited witness to learn its aliases.
- `set`, `--ids` and plan documents stay native-id only.

## Choices

- The coordinator's grammar was `[A-Za-z0-9][A-Za-z0-9._:-]*`. This change narrows it to also end
  in a letter or digit, so the trailing-punctuation rule cannot strip part of an alias.
- The collision check covers every native id of the prefix, not just seeded ones. A later seed
  can then never make an alias ambiguous, and native matching never fires inside an alias.
- Version: the `/0` event profile is kept, following the TOL-V0-001 precedent (decision 0456).
  An older binary refuses an aliased seed request at its closed reader, so that ledger is
  `UNKNOWN` (fail-closed). Whether CAL-V0-131 requires `/1` is left as an open owner question. No
  frozen schema or fixture references the obligation formats.

## Evidence

- Focused tests: `TestTOLV0034_*`, `TestTOLV0035_*`, `TestTOLV0036_*` in `internal/tasks/cli` and
  `internal/tasks/obligation`.
- The existing TOL-V0, preflight, run-batch and qualify tests pass unchanged, on synthetic
  Playwright json reports.
- `NOT_RUN`: a live Playwright 1.63 report naming aliases, and the adopter's suite.

## Follow-ups (proposed, not filed)

- Plan documents (TOL-V0-020/026) and `witness --ids` accepting aliases.
