# V1-1990: docs_flow section and record-kind filters

Date: 2026-10-19
Ticket: V1-1990 (GitHub issue beamfall/corvint#1247).
Requirements: DCP-V1-170..174 (`docs/specs/documentation-corpus-v1.md`), accepted under the
owner's standing auto-accept of lane requirements (2026-10-11).

## Problem

A flow query returned the flow subject plus every flow claim, every relation on the flow, every
joined test-knowledge record and every paragraph of every section. Its only arguments were `id`,
`limit`, `offset` and `retirement`, so a caller that needed one section received the whole flow;
the issue measured about 19 times the characters of a reference server on five sampled calls.
The source references in the issue were verified: the flow branch of `typedQuery`, the
`corpusbridge` input schema and accepted-key check, and `FlowSection.Name` in `adoption.go`.

## Decisions

- **Two optional filters, one place.** The flow answer moved out of `typedQuery` into
  `flowQuery` (`internal/doccorpus/flow_filter.go`) so the concurrent gaps lane (#1245) and this
  change touch disjoint code. `Request` gained `Sections` and `Include` (`omitempty`; nil means
  absent). The CLI (`--section`, `--include`, comma-separated, because corpus options cannot
  repeat), the MCP tool (`section` as a string or list, `include` as a list) and the indexed
  companion all reach the same query.
- **Section implies paragraphs.** The issue asks for "the flow header plus only that section's
  paragraphs", so a section filter without `include` selects paragraphs only. A section filter
  whose `include` omits `paragraphs` would silently ignore the section, so it is refused.
- **Unknown section is a typed none-found, not an error.** The receipt has no records,
  `state` `empty`, `miss` `none-found`, and `flow_selection` lists every available section and the
  unknown names, so the caller can retry without another discovery call. `pageReceipt` and the
  schema-1 path now keep a miss the operation set instead of overwriting it with `no-match`; no
  operation set one before, so other receipts are unchanged.
- **No new error codes.** Malformed filters reuse `invalid-arguments` (MCP, CLI) and
  `corpus-refused` (library, indexed companion).
- **Selection counts only when filtered.** `flow_selection` reports sections with paragraph counts
  and, per kind, `available` and `selected` (before paging). It is attached only when a filter is
  present, so an unfiltered receipt keeps its bytes. Records keep the existing deterministic
  record-ID order, so pagination composes with filters unchanged.

## Measurements

- Eight-section fixture flow, eight paragraphs per section plus the fixture's readme paragraph
  (65 paragraphs), two flow claims and one tested relation, `limit` 100: unfiltered receipt 72,437
  bytes; `section: "entry-points"` (the second section, "B") 15,103 bytes, 20.9% of unfiltered
  (`TestDCPV1170FlowSectionFilter` asserts at most 25%). About 6.5 KB of the filtered receipt is the
  fixed trust envelope, capabilities and citations, so the ratio improves as sections grow; the
  issue's corpus averages about 130 paragraphs per flow.
- Byte identity: a scratch test encoded unfiltered flow receipts (default limit, limit 20, a
  middle page, an exhausted page, and an absent flow) on the base 99dac523 and on this change.
  After masking 40-64 hex digests, which differ per fixture build because the fixture commits carry
  wall-clock timestamps, the 89,667-byte outputs were identical. The scratch test was removed.

## Evidence

- `TestDCPV1170FlowSectionFilter`, `TestDCPV1171FlowIncludeFilter`,
  `TestDCPV1172FlowSelectionReceipt`, `TestDCPV1173FlowFilterPagination` (doccorpus);
  `TestDCPV1174DocsFlowToolFilters` (external test in `internal/doccorpus`, through
  `corpusbridge.Call` against a written artifact); `TestDCPV1174CorpusFlowFilterOptions` and the
  updated `TestDocsCorpusOperationFlagMatrix` (cmd/corvint).
- Fail-without-fix: with the filters disabled inside `flowQuery` (include, section and
  unknown-section branches forced off) all five doccorpus-directory tests fail; without the CLI
  option table entries the CLI tests fail with an unsupported option.
- Full package runs passed: `internal/doccorpus`, `internal/mcp/corpusbridge`,
  `internal/corpusindex`, `internal/corpusserve`, `cmd/corvint-corpus-mcp`,
  `cmd/corvint-corpus-parity`, `conformance/mcp-2026-07-28`, `conformance/cli-parity-v0`;
  `cmd/corvint` with `-run 'TestDocsCorpus|TestDCPV1174|Help'`.

## NOT_RUN

- The full `cmd/corvint` package and the remaining `corvint affected` selection (shell scripts
  and transitively dependent packages without corpus flow use); `make gate` per lane charter.
- A live measurement on the issue's 159-flow corpus; that corpus is not available here.

## Rollback

Revert the change. The filters are optional request members and an optional receipt member; no
artifact, index or stored state changes, and unfiltered receipts are byte-identical.
