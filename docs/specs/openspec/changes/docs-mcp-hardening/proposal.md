# Change proposal: honest docs checks and usable docs MCP tools

Change name: `docs-mcp-hardening`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-DOCS-001, PROD-DOCS-003, PROD-DOCS-005, PROD-DOCS-007 and
PROD-DOCS-009 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/docs-mcp-hardening-v1.md`, a new spec with its own prefix
`DMF-V1`
Requirement IDs this change adds: DMF-V1-001 to DMF-V1-011 and DMF-V1-013 (DMF-V1-012 moved to
`how-rewrite` as HOW-V1-012)
Owner answers and decisions this change implements: R42 (panel pick) B1 to B5. It acts on the
verified findings in `tricks/docs-mcp.verdicts.md` (review of 2026-11-09, checked against integ
9ec3a29d0).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/docs-mcp-hardening/spec.md`.

## Why

The docs product makes two kinds of promise. Its checks say whether documentation still matches the
code (PROD-DOCS-003, 005). Its MCP server gives an agent tools to search and use that documentation
(PROD-DOCS-001, 009). A review found places where both promises break:

- A waived flow-doc claim still prints as PROVEN on the page. A reader sees a claim the product knows
  is not proven (finding 2).
- No single command tells a user whether the repository's docs are stale. `docs maintain --preview`
  exits 0 even when a block is stale, and `docs flows check` reports clean without comparing pages
  unless `--output-dir` is given (findings 1, 13).
- The product citation check skips a missing top-level file, checks citations inside code fences,
  never counts unpinned citations and prints text only (finding 3).
- Checks read HEAD or the index and do not say when the worktree differs, so a user editing a file
  gets an answer about a different version (finding 4).
- A flow-doc check failure says only that bytes differ, not which claim changed (finding 14).
- The 20 `corvint.docs_*` tools share one description, so an agent cannot choose between them; one of
  them answers test selection with a second engine beside `corvint.affected` (findings 5, 16).
- Tool errors often have a code and no message, use the code as the message, drop the child's
  stderr, or return `-32603 Internal error` for a busy server or a cancelled call (finding 6).
- After the corpus artifact is rebuilt, every docs tool refuses until the server restarts
  (finding 8).
- `how` gaps say a cited span changed but not which one, and an aliased source is always UNKNOWN with
  no reason given (finding 9).

## What exists today

All paths are in integ 9ec3a29d0.

- Waivers: `internal/appflows/docs.go:637-669` puts the committed claim back when a waiver applies;
  `stateMark` (`:518-524`) prints nothing for PROVEN; the waiver appears only in the report's
  `Waived` list.
- `docs maintain`: `cmd/corvint-docs/docs_maintain.go:101` needs one page, source and package;
  `:133-157` exits 0 unless an error occurs; staleness is reported as `blocks[].eligible`.
- `docs flows check`: `internal/flowdocs/check.go:219-227` compares pages only when `--output-dir`
  is set; the manifest (`internal/flowdocs/generate.go:73-85`) records no output directory.
- Citations: `internal/doccitations/check.go:126-131`, `:119-120`, `:193-198`;
  `cmd/corvint-docs/docs.go:150`.
- Byte failure: `internal/appflows/docs.go:681-686`.
- Tool descriptions: `internal/mcp/corpusbridge/corpusbridge.go:304-308`.
- `corvint.docs_recommend_tests`: `corpusbridge.go:147`, answered by `internal/doccorpus/query.go:693`.
- Errors: `internal/mcp/cli/main.go:568-578`, `internal/mcp/toolgroup/toolgroup.go:51`,
  `internal/mcp/corpusbridge/group.go:60-70`, `internal/mcp/docsbridge/docsbridge.go:177-192`,
  `internal/mcp/server/server.go:219-221`, `internal/mcp/toolgroup/affected.go:112-127`.
- Artifact pin: `internal/mcp/corpusbridge/corpusbridge.go:424-429`; registry cached in
  `group.go:223-240`.
- `how` gaps: `internal/appmap/site_knowledge_how.go:515-531`, `internal/appmap/project.go:208-215`.

## What Changes

- Waived claims render as WAIVED, with the expiry date and the current state, in both the page and
  the sidecar (DMF-V1-001).
- A new `corvint-docs docs check` runs every docs check in one pass, prints one JSON report and exits
  1 on any stale item; `docs maintain --check` exits non-zero when a block is stale (DMF-V1-002, 003).
- The citation check reports every missing file, skips fenced code, counts unpinned citations and
  gains `--json` (DMF-V1-004).
- Every check report says whether the worktree is clean (DMF-V1-005).
- A flow-doc byte failure names the changed claims (DMF-V1-006).
- `docs flows check` never reports clean without comparing pages (DMF-V1-007).
- Each docs tool gets its own description and property descriptions (DMF-V1-008). The 19 tools
  left after `corvint.docs_recommend_tests` is removed are re-described, not pruned (decided
  2026-11-09 under R42, B9).
- `corvint.docs_recommend_tests` is removed; docs tool descriptions point to `corvint mcp`
  (DMF-V1-009; R42 (panel pick) B2).
- One actionable error shape for every MCP tool (DMF-V1-010).
- A rebuilt corpus artifact is verified and served without a restart (DMF-V1-011).
- `docs consume` keeps strict binding and its stale refusal names what moved (DMF-V1-013; R42
  (panel pick) B5). The `how` gap item moved to `how-rewrite` as HOW-V1-012 (R42 B1).

## Capabilities

### New Capabilities

- `docs-mcp-hardening`: honest docs-check results and agent-usable docs MCP tools.

### Modified Capabilities

- `mcp-tool-table` (change `mcp-tool-table`): no edit here. The verdict file suggests adding a
  description column and the DMF-V1-010 error members to its table, and recording the
  `corvint.docs_recommend_tests` disposition as a row.
- `how-rewrite`: owns the `how` gap detail change as HOW-V1-012 (formerly DMF-V1-012; R42 (panel
  pick) B1).
- `flows-status-report`: owns the docs verb family renames as AFL-V1-008 (R42 (panel pick) B4).
- `change-report-impact`: no edit here. It may later read the DMF-V1-002 report.

## Impact

- `cmd/corvint-docs` (`docs.go`, `docs_maintain.go`, `docs_flows.go`, a new `docs check` verb).
- `internal/appflows`, `internal/flowdocs`, `internal/doccitations`, `internal/docmaintain`.
- `internal/mcp/cli`, `internal/mcp/server`, `internal/mcp/toolgroup`, `internal/mcp/corpusbridge`,
  `internal/mcp/docsbridge`, `internal/mcp/bridge`.
- `internal/appmap` (gap detail text only).
- Golden outputs of flow docs, MCP tool lists and `how` gaps change and are re-recorded.

## Non-goals

- No citation repinning and no move detection in the product check (finding 3, LATER).
- No change to how results are sized, truncated or duplicated in MCP replies (findings 7, 15).
- No full output schemas for tools (finding 17).
- No caching, memoising or watch-loop changes (findings 10, 12, 18, 19).
- No relaxed draft binding for `docs consume` (finding 11); cited-files mode is 1.x (R42 B5).
- No renaming of the docs verb families here (finding 20); renames are owned by
  `flows-status-report` AFL-V1-008 (R42 (panel pick) B4).
- No new ranking, synonyms or evaluation change for `how` (finding 9 part c).
- No failure for unanchored documents and no progress notifications (finding 21).
