# V1-1564: SQL build-error qualification claim

Codex review of PR #1000 found the platform matrix saying both SQL profiles refuse `build-error`.
Retained receipts under `/private/tmp/cem10-build/platform-2026-10-17/runs/`:

- `sql-pgtap/build-error` and `sql-pgtap/pre-0076/build-error`: psql exit 3, `complete: false`,
  problems `SQL_PROCESS_ERROR`, `no-tests`, `runner-exit`. Incomplete; the claim holds.
- `sql-sqllogictest-sqlite/build-error`: `SELECT FROM WHERE` ran, native exit 1, `complete: true`,
  one FAILED `UNKNOWN` suite `broken.slt`, no problems, empty receipt error. Not refused.

No receipt exists under `/private/tmp/cem10-build/sql*`. Change: corrected the spec text and added a
retained release blocker naming the missing distinction between a rejected statement and an
assertion mismatch (TRE-V0-009: the matrix states only what witnesses show). No refusal was added
to `internal/testrunner/sql`; it would need a native-report signal that the retained report does not
carry. No new requirement IDs. Rollback: revert the doc edits. `make gate` and Go tests NOT_RUN
(docs-only).
