# Change proposal: one report of flow, variation, attached tests and last recorded result

Change name: `flows-status-report`
Ticket: V1-5257 (open, P1, milestone `v1-0`; the ticket stays in `.taskman/`)
Product requirement served: PROD-TEST-010 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: a new companion spec `docs/specs/application-flow-status-v1.md` (prefix
`AFL-V1`). It does not exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: AFL-V1-001 to AFL-V1-007 (AFL-V1-007 added 2026-11-09 after review,
docs F2). The block was first planned as
AFU-V1-160..165; it moved because `AFU-V1-165` is a string prefix of the existing
AFU-V1-1651..1655 in `docs/specs/REQUIREMENTS.tsv`, and `AFL-` has no entry there (checked
2026-11-09 on `claude/layers-integ` at `9ec3a29d0`).
Owner answers and decisions this change implements: G5, G6, G11, owner answer 14, owner decision
R30 (2026-11-09) and owner decision R31 (2026-11-09)
Intent status: proposed (rewritten 2026-11-09; only the owner can accept it)
Delivery status: not-started for the native report. A `flows status` command already exists in the
`corvint-docs` command group (see Why).
Written before dispatch: yes.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/application-flow-status/spec.md`.

## Why

The owner asked that flows, their variations and the tests attached to them be easy to track and
reportable by the MCP and the CLI. G5 places that report natively in `corvint`, so a team that
installs only the testing product gets it.

Verified today on `claude/layers-integ` at `9ec3a29d0`:

- `flows status` is registered in `internal/command/flows/flows_status.go:13`, in the command group
  that decision 0595 put in `corvint-docs`. `corvint` forwards every `flows` verb except
  `test-evidence` to `corvint-docs`: `runFlowsShim` in `cmd/corvint/flows_shim.go` at integ
  `9ec3a29d0` does the forwarding, and its `test-evidence` branch is the only native verb.
- `flows impact` is one of the forwarded verbs: its command lives in the docs-layer
  `internal/command/flows` group and its logic in `internal/appflows` (AFU-V1-017). No other
  proposal makes it native, although G5 lists impact among the four native reports and
  `affected-plan-kinds` APK-V0-007 compares against it.
- It needs `--flows DIR`, takes optional repeatable `--evidence FILE` and `--base`, and reads each
  evidence file with `appflows.ReadRunEvidence` (`internal/command/flows/flows_status.go`).
- A test row carries `test_key`, `project`, `result`, `stale` and `source_commit`; an unknown result
  is `NOT_OBSERVED` (`internal/appflows/status.go:19`, `:53-60`).
- The evidence files are written by `flows ingest`, which converts a Playwright JSON report, a
  Playwright receipt, JUnit XML or `go test -json` output into `test-run-evidence/0` on stdout
  (`internal/command/flows/flows.go:418`, `appflows.IngestRunFile`). The user saves that output by
  hand.
- `corvint test` writes a `corvint-test-run/0` receipt, by default `<git-dir>/corvint/test-run.json`
  (`cmd/corvint/test.go:168`, `:417-425`, `:477-478`). That receipt records outcomes per selected
  unit. Whether a unit is ever a single test title rather than a file is NOT_VERIFIED.

Contradiction resolved: the first version promised the "last recorded result" for each test. No
code persists results automatically today. This rewrite names the two writers the report may read,
and states `NOT_OBSERVED` for every other test.

## What Changes

- `corvint flows status` runs natively in `corvint` and reports, per flow and variation, the tests
  tied to it by accepted, non-stale link records and the last recorded result of each (G5, G11).
- Results come only from the two named writers: the `corvint test` receipt and saved
  `test-run-evidence/0` files. Everything else is `NOT_OBSERVED`.
- `corvint.flows.status` on `corvint mcp` returns the same report. Testing tools stay on `corvint mcp`
  and docs tools on one `corvint-docs mcp` server; tool names, input schemas, output profiles and
  error codes come from the table of change `mcp-tool-table` (owner decision R30, 2026-11-09).
  Integ 9ec3a29d0 already lists `corvint.flows.status` and `corvint.flows.impact`
  (`internal/capcatalog/mcp.go`, served by the flows bridge).
- `--base` narrows the report to flows and variations the change touches, matching `flows impact`
  for the same inputs.
- The `corvint-docs` copy of `flows status` is removed so the verb has one home.
- `corvint flows impact` also runs natively in `corvint` on the shared readers, its `corvint-docs`
  copy is removed, and the `corvint.flows.impact` MCP tool is kept on `corvint mcp` (AFL-V1-007,
  G5). Change `change-report-impact` reuses this native answer for its flows section and does not
  remove the tool.

## Capabilities

### New Capabilities

- `application-flow-status`: a native flow status report in `corvint`.

### Modified Capabilities

- `application-flow-understanding`: `flows status` and `flows impact` leave the `corvint-docs`
  command group.

## Impact

- Code: a status query on the shared flow model and readers in `internal/appflows`; a native entry in
  `cmd/corvint` (tests layer); a reader for `corvint-test-run/0` receipts; `internal/mcp/bridge`.
- The shared layer keeps only the flow model and readers (owner answer 14). The join of flows with
  results lives in the tests layer, which imports `internal/appflows`, never the reverse
  (`tools/import-ratchet/areas.json:4`).
- Depends on: `.corvint/flows.json` and link records (change `affected-plan-kinds`); flow-file
  versioning from change `flow-schema-version`; the MCP table from change `mcp-tool-table`.
- An edit of `.corvint/flows.json` or its link records selects the tests linked to the changed flows,
  not a full run (owner decision R31, 2026-11-09); `flows impact` is the answer that selection uses.

## Non-goals

- Running tests or persisting results inside the report command (invariant 4).
- Inferring a result from history, CI logs or a test name.
- Authoring flows or links (`corvint-docs`).
