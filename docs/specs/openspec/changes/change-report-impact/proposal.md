# Change proposal: `corvint impact` as the cross-product change report (preview)

Change name: `change-report-impact`
Ticket: none filed yet (to be filed in `.taskman/` before dispatch).
Product requirement served: PROD-INT-028 (the change report), PROD-TEST-003 and PROD-TEST-010,
PROD-INT-001 and PROD-INT-002 in `docs/specs/PRODUCT-PLAN-1.0.md` (PROD-INT-028 added 2026-11-09,
review docs F8)
Owning capability spec: a new spec `docs/specs/change-report-v1.md` (prefix `CRI-V1`). It does not
exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: CRI-V1-001 to CRI-V1-009 (CRI-V1-009 added 2026-11-09 for owner
decision R30). `CRI-` has no entry in
`docs/specs/REQUIREMENTS.tsv` (checked 2026-11-09 on `claude/layers-integ` at `9ec3a29d0`).
Owner answers and decisions this change implements: G6, G16, G25, owner answer 13, owner decision R4
(2026-11-09) and owner decision R30 (2026-11-09)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started. Release label: preview for 1.0; it never blocks the release (G25).
Written before dispatch: yes. G25 requires a committed proposal before any build.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/change-report/spec.md`.

## Why

G25 item 1: `impact` becomes the cross-product change report (tests to run, flows and variations
touched, docs made stale, tickets and requirements touched), built on the `affected` engine plus
flows and docs joins; the separate impact engine in `contextindex` is removed. G25 notes the plan
was not yet checked against code; this proposal does that check.

### Today's state, from the code at `9ec3a29d0`

- `impact` is a top-level command (`cmd/corvint/main.go:104-105`, `:644`). It reads the
  `contextindex` snapshot (`main.go:873-878`).
- Its engine is in `internal/contextindex`: `impact.go` (1304 lines), `range_impact.go` (1327),
  `module_impact.go` (529), `non_go_impact.go` (133), `impact_callers.go` (106),
  `impact_capabilities.go` (74), `impact_freshness.go` (98). Command glue:
  `cmd/corvint/impact_tworoot.go` (269), `cmd/corvint/impact_range_ancestry.go` (24) and
  `cmd/corvint/index_snapshot.go` (276). Line counts from `wc -l`; the full set of dependants
  (for example `extevidence.Section` and `PathImpactSyntax` in `eval_query.go`) is NOT_VERIFIED by a
  build.
- The MCP tool `corvint.impact` is `ToolImpact` (`internal/mcp/bridge/bridge.go:29`).
- The affected engine (`internal/liveverify/affected`, `cmd/corvint/affected.go`) already produces
  the test plan, with E2E reasons and scopes (`internal/liveverify/affected/e2eplan.go:12-41`).
- Docs freshness exists in `corvint-docs`: `docs flows check --revision FULL_SHA --previous FILE`
  reports each stable claim as fresh, stale or unresolved (`cmd/corvint-docs/docs_flows.go:160-170`).
  Its output profile is NOT_VERIFIED.
- Tickets carry a `requirementRefs` member (seen in `.taskman/tickets/*.json`). How a change range
  is tied to tickets (commit message IDs, claim scope) is NOT_VERIFIED against the Tasks schema.

## What Changes

`corvint impact --base FULL_SHA [--head FULL_SHA]` prints one report with five sections, each with
an explicit state.

### Inputs

1. The base and head commits (head defaults to the working tree's `HEAD`).
2. The affected plan for that range, computed by the affected engine (not a second engine).
3. `.corvint/flows.json` and its link records (change `affected-plan-kinds`).
4. Optionally `--docs-check FILE`: a saved output of `corvint-docs docs flows check` for the head
   revision. Loose integration through files (owner answer 13); absent means the docs section is
   `NOT_OBSERVED`.
5. Optionally the Tasks store, read only: tickets whose IDs appear in commit messages of the range
   and tickets whose claim scope covers a changed path; absent store means `NOT_OBSERVED`.
6. Requirements: IDs from those tickets' `requirementRefs` and requirement IDs on changed lines of
   `docs/specs/*.md`.

### Facts reported

- `tests`: the selected tests per runner and kind, with reasons, scope and the `unassigned` count
  (copied from the affected plan).
- `flows`: flows and variations touched, each with its tests and whether any variation has no test.
  This section reuses the native `corvint flows impact` answer of change `flows-status-report`
  (AFL-V1-007); `corvint flows impact` and the `corvint.flows.impact` tool stay (review docs F2).
- `docs`: pages or claims made stale by the range, from the docs check file.
- `tickets`: ticket IDs, titles and statuses, each with how it was matched (commit message or claim
  scope).
- `requirements`: requirement IDs, each with its source (ticket or changed spec line).

### Output shape

Profile `corvint-change-report/0`: `{profile, base, head, sections: {tests, flows, docs, tickets,
requirements}}`, where each section is `{state: OBSERVED|NOT_OBSERVED|INCOMPLETE, reason?,
items: [...], total, truncated}`. Text output prints the same sections in the same order.

### Consumers

- The CLI.
- `corvint.impact` on `corvint mcp`, with the same body. Its tool name, input schema, output profile
  and error codes come from its row in the table of change `mcp-tool-table`; testing tools stay on
  `corvint mcp` and docs tools on `corvint-docs mcp` (owner decision R30, 2026-11-09). The old
  tool's error codes are listed in that row as removed, not dropped silently (CRI-V1-009).
- The GitHub Action PR comment, preview (G16).

## Capabilities

### New Capabilities

- `change-report`: the cross-product change report.

### Modified Capabilities

- The existing `impact` command and MCP tool are replaced; their old output is removed.

## Impact

- Removed: the `contextindex` impact engine files listed above, `impact_tworoot.go`,
  `impact_range_ancestry.go`, and the `impact` read of `index_snapshot.go` (the snapshot itself is
  decided in change `watch-seven-runners`), with their tests.
- Added: a report builder in the tests layer that imports the affected engine and the shared flow
  readers (`tools/import-ratchet/areas.json:4`).
- Docs: `docs/CAPABILITIES.md`, `docs/MCP-SERVER.md`, help text.

## Non-goals

- Any ranking, symbol search or caller graph (the removed engine's features).
- Running tests or `corvint-docs` from inside `impact`.
- Blocking the release: this is preview. No release-blocking gate or report may require it; its
  docs section is advisory (owner decision R4, 2026-11-09).
- Removing `corvint flows impact` or `corvint.flows.impact` (review docs F2).
