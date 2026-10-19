# 2026-10-18 V1-1868: reviewed test-knowledge records in the documentation corpus

Ticket: V1-1868 (the docs corpus ingests reviewed test-knowledge records from any test framework,
with closed trust levels, joined to claims, flows and source anchors). GitHub #1162.

## Decision

`DCP-V1-086` to `DCP-V1-092` are added to `docs/specs/documentation-corpus-v1.md` and accepted under
the owner's standing auto-accept of lane requirements (2026-10-11). The slice extends the existing
corpus path; it adds no parallel ingest, store or service.

- A new explicit provider schema, `corvint-corpus-test-knowledge/1`, carries one closed
  `test_knowledge` member: a set `revision` plus up to 4096 records. A record has a closed kind,
  verdict (`bound`, `partial`, `unrelated`, `defect`), docs state and trust level
  (`run+reviewed`, `run-unreviewed`, `read-only`, `legacy-reference`). Unknown members and values
  are refused by the strict decoder and the admission rules. The record format is framework-neutral:
  the framework is a bounded label, and every fact is a pinned anchor.
- Every free-text value passes the shared secret-shaped text scan (`internal/secretscreen`).
- The build joins records after all providers are imported. Anchors are compared with the declared
  inputs at the corpus revision. A later-revision set whose file changed is reported `stale`, an
  undeclared path or receipt is `unanchored`, and neither is joined. Any other mismatch is a forged
  binding and refuses the build. Claim joins use path and line overlap with existing claim evidence;
  claims and all other records are never rewritten.
- Trust is labelled, never promoted. Only a joined `run+reviewed` `bound`/`partial` record counts
  toward coverage or becomes a selection candidate. A joined `defect` at any trust level withholds
  every candidate on the path. Selection stays `full-relevant-suite-required`; no record marks a
  claim, flow or journey verified, and each join carries that limitation.
- The native reader exposes joins through `get`, `inventory`, `flow`, `claims`, `locate`,
  `coverage`, `gaps` and `recommend-tests`. The corpus MCP tools, the indexed companion and the
  HTTP companion call the same reader, so no transport change is needed.

## Evidence

- Seven acceptance tests in `internal/doccorpus/test_knowledge_test.go`, one per requirement. The
  query test calls `ReadQuery`, the function the corpus MCP bridge calls.
- Mutation checks, each reverted after the run:
  - counting `run-unreviewed` records: `TestDCPV1090` and `TestDCPV1091` fail;
  - dropping the defect block: both fail;
  - joining stale anchors: `TestDCPV1089` fails;
  - removing the secret scan: `TestDCPV1087` fails;
  - dropping test knowledge from the read-bound check: `TestDCPV1092` fails.
- The full `internal/doccorpus` package and the `internal/corpusrepublish` parity tests pass.

## Residuals (proposed tickets, not filed by this lane)

- Render a generated per-flow "what the tests demonstrate" section from counted joins.
- A standalone `corvint docs corpus` validate command for test-knowledge records.
- Reviewer authentication and run-receipt re-verification.
- Semantic (non-line-overlap) claim joins.
- An MCP bridge-level test for the test-knowledge rows.

## Rollback

Revert the change. The provider schema, the artifact member and the reader rows are removed together.
Corpora without a test-knowledge provider build the same bytes before and after.
