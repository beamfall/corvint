# 2026-10-19 V1-1996: corpus MCP single calls page or refuse at the output bound

GitHub #1255. Requirements DCP-V1-200..203 in `docs/specs/documentation-corpus-v1.md`.

## Problem

`docs_flow` with `limit` 256 on one flow subject of a ~100k-record corpus answered JSON-RPC
`-32603` Internal error; `limit` 64 answered 64 results. Verified cause: the corpus reader bounds
a receipt at 4 MiB (`doccorpus.MaxBytes`, enforced in `Encode`; `pageReceipt` halves a v2 page only
against that bound), but `corvint-corpus-mcp` sends every receipt twice in one result frame
(structured content plus the escaped, enveloped text) under the 1 MiB MCP message bound. A
single call between roughly 0.5 MiB and 4 MiB passed the bridge and was then rejected by
`internal/mcp/server` `writeValueContext` (`errOutputLimit`), which `writeResultContext` maps to
`-32603`. Path batches already measured each row against `MaxBatchBytes` (DCP-V1-145); single calls
did not. The failure was shared by every single-call corpus tool, not only `docs_flow`.

## Decisions

- Fix in the one shared single-call path, `corpusbridge.boundedQuery`, so every corpus tool is
  covered; the server's generic `-32603` fallback stays as the last-resort transport guard.
- Measure as framed (`singleFrameCost`: receipt bytes + JSON string of the enveloped text) against
  `MaxBatchBytes` (1 MiB less a 64 KiB reserve for the result wrapper, `_meta` timing and
  structured re-encoding drift), the same reserve batches use.
- `/2` artifacts: re-ask with a smaller `limit` (`shown*bound/cost`, strictly fewer each round), so
  the answer is the CLI's own receipt for that limit, carrying `next_offset`. No new receipt field,
  so receipts stay CLI-byte-identical (DCP-V1-145 parity).
- `/1` artifacts (no `offset`) and one-record pages: typed tool failure `output-bound-exceeded`
  (code already owned by DCP-V1-145) naming result count, cost and the byte bound.

## Evidence

- `TestDCPV1200SingleCallOverBoundPagesWithNextOffset` (40-page `/2` fixture, bound set to a
  quarter of the unbounded cost): the page fits, carries `next_offset`, equals the same-limit
  receipt; paging reproduces all 40 records in order over >= 2 pages; an unshrinkable bound
  refuses with the bound named. `TestDCPV1202UnpagedSchemaRefusesOverBound` covers `/1`.
- Fail without the fix (call site reverted to `query`): both tests fail (page cost 155178 over
  bound 38794 with no `next_offset`; no refusal).
- Byte identity: fitting receipts take the unchanged path (`TestDCPV1143LazyLoadAndMemo` still
  compares MCP text to the CLI encoding); artifacts are untouched.

## NOT_RUN

- Live reproduction on the ~100k-record corpus from the report (not available in the lane); the
  bound is exercised by lowering `singleFrameBytes` in tests.
- `make gate` (owner preference for scoped work).

## Rollback

Revert this change; single calls return to the unbounded `query` path.
