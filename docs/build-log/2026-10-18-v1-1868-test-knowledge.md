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
  inputs at the corpus revision. A set pinned at any other revision is reported `stale`, an
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

## Review fixes (Codex round on #1165 at 4c494e642)

Four P2 findings were fixed, each with a regression test and a mutation check that was reverted
after the run. `DCP-V1-089`, `DCP-V1-091` and `DCP-V1-092` are amended in the same change, under
the same standing auto-accept.

1. **Claimed revision.** The set `revision` was only format-checked, so a nonexistent or unrelated
   revision with the corpus blobs still joined. Fix: a set must be pinned at the corpus revision.
   Any other revision is reported `stale`, and none of its records joins, counts or is cited. The
   earlier "older set joins while blobs are unchanged" rule is withdrawn.
   - Test: `TestDCPV1089/claimed_revision_other_than_the_corpus_revision_never_joins` covers a
     nonexistent revision and another real commit.
   - Mutation: disabling the check fails both cases.
2. **Test-knowledge-only corpus.** On a corpus whose only provider is test knowledge, the
   capability guard refused `get`, `locate`, `claims` and `recommend-tests`. Fix: while any join
   exists, the guard admits exactly those four operations, with a limitation naming the absent
   capability. Other operations stay `capability-absent`.
   - Test: `TestDCPV1091TestKnowledgeOnlyCorpus`.
   - Mutation: removing the admission fails all four operations.
3. **Unbounded rows.** Trust-level coverage rows and `blocked_by` named every ID without a limit.
   Fix: coverage rows now carry counts only. `blocked_by` lists at most 32 IDs, with
   `blocked_by_count` and `blocked_by_truncated`.
   - Test: `TestDCPV1091/worst-case_rows_stay_within_the_receipt_bound` uses 4096 joined defects
     with 1 KiB IDs.
   - Mutation: an unbounded `blocked_by` exceeds the output bound. An oversized coverage row was
     silently paged away, so the test now requires that the row is present.
4. **Parity encode limit.** Republication parity compared the whole test-knowledge slice through
   a bounded encoder and read the encode error as "changed". Fix: parity keys each join by ID and
   compares streamed SHA-256 digests. An encoding failure is returned as an error, never as a
   change.
   - Test: `TestDCPV1092ParityLargeTestKnowledge` compares an identical collection of more than
     4 MiB, then changes one record.
   - Mutation: restoring the whole-slice key with the bounded comparison reports `Changed`.

## Residuals (proposed tickets, not filed by this lane)

- Render a generated per-flow "what the tests demonstrate" section from counted joins.
- A standalone `corvint docs corpus` validate command for test-knowledge records.
- Reviewer authentication and run-receipt re-verification.
- Semantic (non-line-overlap) claim joins.
- An MCP bridge-level test for the test-knowledge rows.

## Rollback

Revert the change. The provider schema, the artifact member and the reader rows are removed together.
Corpora without a test-knowledge provider build the same bytes before and after.
