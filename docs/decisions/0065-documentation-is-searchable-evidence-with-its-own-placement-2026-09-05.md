# Decision 0065 — Documentation is searchable evidence with its own placement, not another lexical row

Date: 2026-09-05. Status: accepted. Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that open calls be resolved through the expert panel, and this record applies the panel memo `docs/reviews/panels/panel-1-retrieval.md` after independent review of its cited evidence.

## Context

`textSuffixes` in `internal/contextindex/index.go` excluded `.rst`, `.mdx` and `.txt`, so flask's
79 `.rst` and zod's 18 `.mdx` files were invisible to every verb (`docs/reviews/r1-retrieval.md`
BUG-3, `r12-cold-start.md` F7). The panel measured admission on the development corpus and blind-v3:
no change to recall, precision (0.819964), the V4 gate or any of the 133 cli-parity commands, and
`context` on flask promotes the blind-v3 gold `docs/patterns/streaming.rst` from absent to rank 2.
Admitting documentation at equal lexical weight costs code recall (gold@10 33 to 30 over 43 gold
paths); placing documentation after the first five lexical code rows with a quota of two gives
gold@5 25 against today's 21. `query` cannot emit a plain-text source, so this does not move the
`query`-scored V4 gate and is not sold as a `query` recall fix.

## Decision

1. `.rst`, `.mdx` and `.txt` join `textSuffixes` as searchable text with no symbol extractor.
   Documentation-suffix lexical hits are labelled `documentation`, placed after the first five
   lexical code rows with a quota of two, the remainder after all code rows, with the distinct and
   occurrence sort kept inside each class and the reason naming the class. `TCP-V0-003` and
   `TCP-V0-011` list the relation; the placement rule gets a new stable requirement ID in
   `docs/specs/task-context-packet-v0.md`; `docs/specs/index-snapshot-v0.md` states the admission.
   No divergence record is opened: the parity manifest holds no `context` case and the `query`
   bytes are unchanged by measurement. Documentation is not governing evidence; `documentKind`
   stays `.md`-only and that limit is a backlog item for the domain-model call.
2. Admission gate: `benchmarks/run.py --enforce-v4` green with precision at or above 0.80 and recall
   1.0; `go test ./internal/contextindex/... ./conformance/...` green; the gold@k table re-measured
   against the shipped relation and recorded in the build log; blind-v3 replay unchanged; blind-v4
   untouched.
3. `tools/retrieval-bench` gains a `context` arm, registered before its first run, because `query`
   is the only verb any frozen suite scores and it loses to a grep arm on all four ARB subsets while
   `context` is unscored. The b1 candidates (test-to-code relation, conjunction-absence answerability,
   PRF expansion, BM25 tie-break, anchored graph expansion) and the r11 co-change prior are ranked
   behind that arm as backlog with the memo's falsifiers; the BM25-everywhere claim is partly
   falsified by r1 OPT-3.
4. After decision 22b5b9a made status the sole dirty authority, a Git LFS pointer is indexed as
   pointer text with nothing said. `pinnedFrom` tests the LFS pointer prefix and records an
   `Exclusion` naming the pointer, carried in the snapshot and emitted in the receipt's existing
   exclusion sample; a new stable admission clause in `index-snapshot-v0.md` states it. A bounded
   `ident`/`eol` divergence count carried in the snapshot header is backlog.

## Consequences

`context` gains documentation evidence at no measured cost to the V4 gate or cli-parity. LFS
repositories no longer receive confident packets built from pointer text.

## Rollback

Revert the suffix row, the placement hunk, the exclusion test and the bench arm; no persisted
state, format change, or wire contract is involved.
