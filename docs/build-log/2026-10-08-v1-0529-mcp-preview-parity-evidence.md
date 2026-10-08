# V1-0529: readonly MCP CEM report preview parity confirmed on main

Date: 2026-10-08
Task: V1-0529 / `MCPV0-025` (no new or amended requirement)
Base: 2a93b5a182966f72836cd28bf569dc44c0f5220c

## Finding

The parity gap that V1-0529 recorded no longer exists on main. It was repaired by `27ebec59`
("restore readonly MCP report parity") and `d7749792` (gofmt follow-up), both merged through
PR #385 (`8f4b866a`); see `2026-09-30-mcp-report-preview-parity.md`. At this base
`internal/cem/workflow/read.go` renders the `report-preview` Markdown as
`renderReportText(...) + renderReviewProjection(projection)`, the same composition the published
report uses. Both apply `maxReportBytes` after rendering and return the same `recordSetSha256`,
counts, policy issues, verification and envelope fields. No product code changes here.

## Evidence added

`TestCEMReportToolPreviewsCLIReportWithoutPublishing` (`internal/mcp/bridge`) already compared the
preview Markdown byte for byte, and the metadata field by field, against the published CLI report
for a valid map. It now runs two subtests. The `valid map` subtest is that existing check. The
`invalid map` subtest empties the map's hunks so the derived patch is unexplained. Both subtests
also require that the hunk review projection is present, that the preview keeps the map's
verification validity, and that an invalid map stays `ok=false`. So uncertainty survives the
preview instead of being dropped or upgraded. The invalid-map case was only covered at the
workflow level before, by `TestReviewProjectionEmptyCanonicalDiffRetainsInvalidMap`, which checked
`ok` and the digest but not the Markdown.

Mutation evidence. Each mutant was applied temporarily to `read.go` and then restored. Logs are
under `/private/tmp/claude-501/v0529-td/`.

- Dropping `renderReviewProjection` from the preview (the pre-`27ebec59` shape) makes both
  subtests fail at the Markdown comparison, matching the original CI failure
  (`tools_test.go:101`, now line 72 through the helper).
- Dropping `recordSetSha256` from the preview metadata fails the field comparison.

## Checks

- `go test ./internal/mcp/bridge` PASS. This includes the hostile-argument, symlink/legacy-map
  refusal, budget-abstention and moved-checkout tests.
- `go test -run ReviewProjection ./internal/cem/workflow` PASS.
- `go vet ./internal/mcp/bridge` and gofmt are clean.
- `corvint affected` (1.0.0-rc.2) selected `internal/mcp/bridge` directly and 45 other units. Those
  45 are conservative unbounded-reader, dependency and read-scope witnesses for a `_test.go`-only
  change. The path-literal and declared-scope readers were run. The rest, including
  `cmd/corvint`, are NOT_RUN. The `LANGUAGE_FRONTIER` unknowns are retained.

## Non-goals, failure modes, rollback

Non-goals: no change to preview or report rendering, the MCP schema, budgets or refusal codes.
Failure mode guarded: a future report section added to only one renderer, or a metadata field
added to only one result, fails the parity test. Rollback: revert this commit. The product is
unaffected.
