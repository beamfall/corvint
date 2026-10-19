# V1-1971: symbol-named anchors re-resolve in the live tree

Date: 2026-10-19
Ticket: V1-1971 (GitHub issue beamfall/corvint#1223).
Requirements: DCP-V1-140..142 (`docs/specs/documentation-corpus-v1.md`) and AFU-V1-090
(`docs/specs/application-flow-understanding-v1.md`), accepted under the owner's standing
auto-accept of lane requirements (2026-10-11). No accepted requirement is amended.

## Decisions

- **Reuse the index parsers.** The region normalizer drives the existing Ruby line scanner (which
  now also records its literal spans and comment column) and the existing web lexer. Symbol
  extents come from `contextindex.SymbolExtents`. No grammar is added, so only Ruby and
  TypeScript/JavaScript re-resolve; every other language is `unresolved` with a reason.
- **Normalize code, never literals.** Comments, blank lines and whitespace outside literals are
  dropped; string, template and heredoc text stays byte for byte, so an edit inside a string is
  still `changed` and a reformat is `live` or `moved`.
- **Identity is proven, not guessed.** The same path is searched first. Another path is accepted
  only when its region digest is equal; a same-name symbol whose region differs is `unresolved`.
  `gone` requires a complete same-family inventory: a dirty, unparsed, excluded or noted file of
  that family withholds it.
- **Overlay, not a new freshness value.** The artifact `freshness` value is unchanged. Live
  readers add `symbol_anchors` to the receipt through `LiveFreshness` and `QueryLive`; readers
  without a live tree omit it. `docs flows check` uses the symbol state to keep a moved or
  reformatted imported anchor `fresh`, and an `unresolved` symbol keeps the exact-span verdict.

## Evidence

- `TestSymbolAnchorReresolution` (Ruby and TypeScript): a 20-line insertion is `moved` with the
  range shifted by 20; a one-token edit is `changed`; a deletion is `gone`; a reformat is `live` or
  `moved` with an equal digest; a move to another path is `moved`; a dirty path is `unresolved`.
- `TestSymbolAnchorUncertainty`, `TestSymbolAnchorReaderReceipt`, `TestNormalizedCodeLines` and
  `TestFlowDocsImportedSymbolAnchorReresolution`.

## Limits

- CRLF and CR-only sources are normalized to LF before comparison. Other languages, cross-repository
  anchors and working-tree edits stay `unresolved`; no live runtime or external qualification is
  claimed.
- `analyzerSchemaID` is bumped to `corvint-analyzer/129` because the IDX-SNAP-V0-017 audit digests
  every `internal/contextindex` source. The Ruby scanner's extraction output is unchanged; it only
  records spans when the normalizer asks for them.
