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

Builder decisions (2026-11-16, V1-5623), recorded while building the native reports; they stand
until the owner amends them.

- Home of the code. The command handlers of `flows status`, `flows impact` and `flows gaps` moved
  from `internal/command/flows` (docs layer) to the new tests-layer package
  `internal/flowreports`, which `corvint flows` dispatches. `flows gaps` moved too: PROD-DOCS-011
  lists it with status, impact and test-evidence as a test-joining report, and FSV-V1-007's smoke
  runs it with `corvint` alone. Each `corvint-docs` copy is a refusal naming `corvint flows VERB`
  (exit 2, `invalid-arguments`).
- The join stays shared. The joins themselves (`FlowStatus`, `FlowImpactResults`, `FlowGaps`) stay
  in `internal/appflows`, where they already were, because the `corvint mcp` flow tools in the
  shared `internal/mcp/bridge` call them; moving them into the tests layer would make the shared
  bridge import upward. The tests layer owns the commands; the shared layer keeps one copy of each
  report, so the command and the tool cannot diverge (AFL-V1-003).
- Result sources. `appflows.ReadResults` reads the run evidence store, then each
  `--evidence FILE`. The receipt reader is the store's (RVD-V1-006): `corvint test` appends every
  receipt it writes to the store (RVD-V1-004), so the report sees each run without opening a
  receipt file. A direct `--receipt FILE` or default-receipt read was built and withdrawn:
  `TestSTRV0011ReceiptsHaveNoOtherReader` admits no receipt reader without an accepted
  requirement, and this change is `pending-owner` in `docs/specs/openspec/acceptance-log.tsv`. A
  receipt whose store append failed (`run-evidence-write-failed`) is therefore not reported.
  `result_sources` lists each source, `read` or `not-read`, with its record count, printing a
  named file as the caller named it, so the command and the tool print the same bytes for the
  same names.
- Granularity. A `corvint-test-run/0` row is file level (RVD-V1-006). It reaches a title link when
  its key is the link's key, its key is the link's test file, or it is the canonical file-level
  test ID of that file. The row carries `granularity: file`, and `slice` when its unit ran only part
  of the file; an ingested record carries `granularity: test`. A receipt row names no Playwright
  project, so it stands for the project of a variation that declares at most one; for a variation
  of several projects every row stays `NOT_OBSERVED` with reason `project-not-recorded`, never one
  `passed` for projects that may not have run (invariant 2).
- Unassigned. `--provider FILE` (and the tool's `provider`) reuses the gaps inventory reader:
  `inventory` and `unassigned` equal gaps' `inventory` and `unlinked_total`; without a provider
  `unassigned` is `null`, never 0.
- `--base`. The report keeps exactly the flows and the variations `flows impact` names, where it
  used to keep every variation of a reached flow.

Builder decisions (2026-11-16, V1-5623 review repair), from the independent review of the first
delivery; they stand until the owner amends them.

- Attached tests are the targets of `reviewed` test links only (AFL-V1-001: "accepted, non-stale
  link records"); a `stale`, `unreviewed`, `inferred` or other unaccepted link attaches nothing,
  and a variation left with none carries `no-test`. AFU-V1-160, the first version of this block,
  read every non-inferred link; it is reconciled to AFL-V1-001. `flows map` and `flows gaps` keep
  their own rules.
- Store records and AFL-V1-006. RVD-V1-010 (accepted) skips a corrupt store record and marks its
  tests `run-evidence-corrupt`; AFL-V1-006 refuses a malformed or unknown-profile receipt whole.
  `flows status` reconciles them by what the store can say: a record damaged after it was written
  (its bytes no longer hash to its name) whose tests the log names is skipped and those tests are
  marked, as RVD-V1-010 says; a record whose bytes hash to its name but do not parse (malformed, or
  a profile such as `corvint-test-run/9`), one larger than any record a writer keeps, a stored
  document this build cannot decode, or a skipped record whose tests the store cannot name refuses
  the whole report (exit 2, `invalid-arguments`, message `run-evidence-corrupt: ...`), as a bad
  `--evidence` file does. The other store readers (`flows map`, `flows gaps`, `flows impact`,
  `corvint test-evidence`) keep RVD-V1-010 unchanged. A marked test is one the skipped record held
  by its key or by a file-level ID of its test file.
- Bounds. The store and the named `--evidence` files are bounded separately, as `RunEvidenceFor`
  bounds them: the store keeps its newest 8192 records (`run_evidence.truncated`), and the named
  files together hold at most 8192 (`run-evidence-record-bound`).
- Unknown reach (APK-V0-010). A changed path with no owning unit in the impact graph that no link
  targets, other than the flow declaration and the intent files, has unknown reach. `flows impact`
  then lists those paths in `reach_unknown`, sets the graph scope `UNKNOWN`, and names every flow
  with all its variations and their test keys and `reason: reach-unknown` (keeping any resolved
  hits); `flows status --base` keeps every flow and variation with the same reason. Both members are
  omitted when every path resolved, so earlier reports keep their bytes.

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

`docs pages` refuses an unknown verb, with or without `--help`, and a positional word after
`docs pages flow` (whose handler would read `maintain` as flow-maintain), as `invalid-arguments`.
Refusals and hints name the new spellings (`refresh with docs pages maintain ...`).
`corvint-docs flows status|impact|gaps --help` states that the verb moved to `corvint`.

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
