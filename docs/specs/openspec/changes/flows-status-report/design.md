# Design: `flows-status-report`

## Decisions

- One home per verb (G5): `flows status` and `flows impact` run natively in `corvint`; their
  `corvint-docs` copies are removed, the impact copy with a notice (review docs F2, applied now).
- The CLI and `corvint mcp` return the same body; the tool rows follow the table of change
  `mcp-tool-table` (owner decision R30, 2026-11-09). `corvint.flows.impact` is kept, not folded into
  `corvint.impact`; change `change-report-impact` reuses the native answer for its flows section.
- Results come only from the `corvint test` receipt and saved `test-run-evidence/0` files; any other
  test is `NOT_OBSERVED` (invariant 2).
- A flow-file edit selects the tests linked to the changed flows (owner decision R31, 2026-11-09);
  `flows impact` supplies that set.
- The join of flows with results lives in the tests layer, which imports `internal/appflows`, never
  the reverse (owner answer 14).

## Decisions made while building

Builder decisions (2026-11-10), recorded while merging `claude/layers-integ`; checked against
`internal/appflows/status.go`, `query.go` and `unlinked.go`. They are not owner decisions and stand
until the owner amends them.

- `corvint flows status` writes profile `application-flow-status/0` (AFU-V1-160 to AFU-V1-165):
  each flow, its variations, the tests attached to each and each test's last recorded result.
  - Bounds: at most 8192 test rows (`MaxStatusTests`); later rows are dropped in report order with
    `truncated: true`, and `totals` still counts every flow, variation and test. With a base, the
    report holds exactly the flows `flows impact` names, with its impact graph and scope.
  - Last-result rule: the last record, in evidence order, bound to the evaluated commit and tree
    from a clean worktree; else the last record of any other tree, with `stale: true` and its
    `source_commit`; `STATIC` records never count; with no record the result is `NOT_OBSERVED`.
- `application-flow-gaps/1` keeps its name; its inventory members are additive. `inventory.state`
  is one of `absent`, `complete`, `partial`, `stale` or `unverified`; an absent inventory lists no
  unlinked test and is never a complete answer. A flow's own `status` stays `complete` or
  `incomplete`.

## Docs command group names (AFL-V1-008)

Builder decision (2026-11-16, V1-5623), recorded before the rename commit as AFL-V1-008 requires;
it stands until the owner amends it. The three look-alike families move into one group,
`corvint-docs docs pages`, which owns every command that writes or checks a generated
documentation page. Each old spelling stays an alias for one release: it runs the same handler with
the same flags and prints one stderr notice naming the new spelling.

| Old name | New name | What it does |
|---|---|---|
| `docs flows generate` | `docs pages generate` | generate the bounded source-derived flow pages from a corpus |
| `docs flows check` | `docs pages check` | check those generated pages against regeneration |
| `docs flows finalize` | `docs pages finalize` | bind the generated pages to a provider revision |
| `flows docs` | `docs pages flow` | render one flow page and its claims sidecar; `--check` compares |
| `flows docs maintain` | `docs pages flow-maintain` | preview or apply maintenance of that committed page pair |
| `docs maintain` | `docs pages maintain` | refresh one human page's generated blocks from source |

The notice is one line: `corvint-docs: OLD is renamed NEW; OLD is an alias until the next release
(AFL-V1-008)`. `corvint docs ...` and `corvint flows docs ...` forward to `corvint-docs`
unchanged, so they print the same notice.

## Failure modes

- No result source given and no default receipt: every result is `NOT_OBSERVED`, exit 0, with the
  missing sources listed.
- A result recorded at a commit that differs from the report's commit: shown with `stale: true` and
  `source_commit`.
- A receipt or evidence file that is malformed, over a bound or of an unknown profile: typed
  refusal of the whole report, exit 2.
- `.corvint/flows.json` with an unknown profile version: typed refusal, exit 2 (G5).
- `corvint flows impact` cannot resolve reach for a changed path: it names every flow of the
  affected runner with reason `reach-unknown`, matching APK-V0-010 in `affected-plan-kinds`.
- A script still calls `corvint-docs flows impact` after the copy is removed: the docs binary
  prints that the verb moved to `corvint flows impact` and exits 2.
- A receipt recording a file-level outcome for a test that a link names by title: the file's outcome
  is shown with `granularity: file`, never presented as a per-title result.

## Acceptance evidence

- Fixtures: no result source; a `corvint test` receipt; an ingested Playwright report; a stale
  result; a file-level outcome for a title link; an unknown flow-file profile. Go tests named for
  V1-5257 on the command and the MCP tool.
- A release smoke test running `corvint flows status` and `corvint flows impact` with `corvint-docs`
  absent from `PATH` (G5).
- A test comparing `--base` output with `flows impact` output on the same fixture.

## Rollback

Revert the change's commits; the `corvint-docs` copy can be restored from the reverted diff. No
stored data is written. IDs are retired, not reused.

## Open questions for the owner

Still undecided by the owner; not settled by owner decisions R30 or R31.

- `flows ingest` joins test results with flows. Under the G5 boundary rule (joins with test results
  belong to tests) it would move into `corvint` too. Confirm the move or keep it in `corvint-docs`.
- Should `corvint test` write per-title outcomes for Playwright so the report can show a per-test
  result without `flows ingest`? This touches change `runner-contract` (RNC-V1-005).
