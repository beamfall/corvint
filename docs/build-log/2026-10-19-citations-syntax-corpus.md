# 2026-10-19 V1-1972: configurable citation syntax and corpus-scale citation checks

Ticket: V1-1972 (GitHub #1224).

## Decision

`TSA-V0-380` to `TSA-V0-387` are added to `docs/specs/test-source-analysis-v0.md` and accepted under
the owner's standing auto-accept of lane requirements (2026-10-11). `TSA-V0-252` (bounds) and
`TSA-V0-255` (schema `/0` to `/1`) are amended in place with a V1-1972 note.

- Syntax: a citation syntax is a marker (what counts as a candidate) plus an anchored citation
  grammar. `inline-code` keeps the V1-1934 grammar; `html-sub` reads `<sub>` markers carrying a
  path and `line N`; `--pattern` takes an RE2 expression with named groups `repo`, `path`, `start`,
  `end`, `symbol` and `rev`. RE2 keeps matching linear in hostile documents.
- Unparseable: a candidate that the marker selects but the grammar or field validation rejects is
  reported with document, line and text, and keeps the exit status at 1. Silent skipping hid
  broken anchors in the adopter's corpus.
- Bounds: the 256-document and 4096-citation caps become 65,536 documents and 262,144 candidates
  per run. Memory is bounded by reading documents in chunks (256 documents or 16 MiB), a 64 MiB
  cited-file cache, 65,536-entry memos and 256 nearest-revision lookups, so it does not grow with
  the corpus. Revisions resolve in `cat-file --batch-check` batches and paths in literal-pathspec
  `ls-tree` batches, so 13,000 citations cost a handful of Git processes, not 13,000.
- Selection: `--manifest` reads a committed list and `--glob` matches committed regular files; both
  stay inside the object store (`TSA-V0-256`).

## Evidence

`TestCitationsLargeCorpus` generates 2,000 documents and 13,000 `html-sub` citations in
`t.TempDir()` and checks them in one call. No heap profile on a real corpus and no live
qualification on the adopter's corpus were run (NOT_RUN).
