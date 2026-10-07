# 2026-10-07: compact corvint-tasks agent output (V1-0935, V1-0941)

## Intent

Issue 650 (ticket V1-0935) and its follow-up V1-0941 record that agents print whole
`corvint-tasks` JSON results into their transcripts. On the 900-ticket Corvint store:

- `queue status` was 163 KB, almost all of it the per-ticket `retries` array;
- a 100-of-901 `ticket search` page was 93 KB, with 597 B per item;
- `blockers` on completed tickets were 33% of a search item, and a repeated ATTEMPT_LIVE unknown
  another 20%;
- `roadmap` was 35 KB;
- the installed build refused `<verb> --help`, so agents read the 6 to 8 KB full help.

The change adds proposed CAL-V0-165 to CAL-V0-174 to
`docs/specs/corvint-tasks-agent-leases-v0.md`, pending owner acceptance.

## Decisions

- `--fields KEY[.SUB],...` and `--summary` are applied only at command dispatch, after the read
  built its result. Internal callers (criterion captures, blocker reads) keep the full output and
  the read path is unchanged, so the reads stay read-only.
- An unknown or malformed field refuses with the existing `MALFORMED` code, where `fields`, rather
  than a new code. Syntax errors refuse before the store is opened; a name that no returned item
  carries refuses after the read, with no items and the read's snapshot.
- `--summary` shapes are fixed per read and documented in CAL-V0-167 and CAL-V0-174.
  `plan preview --summary` refuses `--selected-only` because the summary already narrows entries.
- `queue status` omits `retries` unless `--retries` is given (wire change, amends CAL-V0-049).
  The criterion-binding capture passes `--retries`, so captured bytes do not change.
- `--help` is terse; `--help --verbose` returns the previous full text (amends CAL-V0-047 and the
  help content of CAL-V0-050, 106, 121 and V1-0751). The current source already answered
  `<verb> --help` for all 103 verbs; the MALFORMED refusal came from an older installed binary.
- List items (`ticket list`, `ticket search`) drop `record`, and COMPLETED or ARCHIVED items drop
  `blockers` and `unknowns`. The ATTEMPT_LIVE unknown that a journal-absent inventory read attaches
  to every item is hoisted into one envelope warning.
- The default page size stays 100 (`internal/tasks/wire/limits.go`). Lowering it would change
  pagination for every existing consumer; the compact flags and the smaller items cover the need.

## Measurements

Base 79536edd against this change, both binaries built from source and run in place on a journaled
360-ticket fixture (every third ticket COMPLETED, page 100). Bytes are the full JSON result.

| Read | Base | Default | `--summary` |
|---|---|---|---|
| `queue status` | 74,487 | 1,275 (74,487 with `--retries`) | 738 |
| `plan preview` | 119,505 | 119,505 | 24,411 |
| `ticket list` | 67,079 | 50,396 | 15,322 |
| `ticket search --label agent-memory` | 67,136 | 50,453 | 15,324 |
| `ticket search --status OPEN` | 64,093 | 51,048 | not measured |
| `roadmap` | 25,951 | 25,951 | 14,461 |

On the two-attempt lease fixture:

| Read | Base | New |
|---|---|---|
| `attempt show` / `--summary` | 2,345 | 2,345 / 800 |
| `ticket show` / `--summary` | 2,408 | 2,408 / 789 |
| `queue status` / `--summary` | 2,624 | 2,000 / 1,161 |
| `release --help` / `--help --verbose` | 5,206 | 731 / 5,206 |
| `ticket create --help` | 1,812 | 827 |
| `policy update --help` | 639 | 463 |

`TestCALV0171_CompactOutputBytesOnLargeQueue` and `TestCALV0174_SearchAndRoadmapCompact` keep the
ratios as regression bounds on a 358-ticket in-test fixture (`queue status --retries` 110,473,
default 1,271, summary 734).

## Limits

- The page size is unchanged at 100.
- A `--fields` name is checked only against the returned page. An empty page accepts any
  well-formed name; a sub-key absent from every element of non-empty containers is refused.
- The hoisted store-wide unknown is a warning string, not a structured field.
- `plan preview`, `ticket show` and `roadmap` default output is unchanged; only their compact
  flags shrink it.
- No `corvint-tasks` skill text exists under `integrations/`; guidance is in
  `docs/TASKS-EXTERNAL-AGENTS.md`.
- Measurement on the live 900-ticket store with the new binary is NOT_RUN.
