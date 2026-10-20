# V1-1995: scaled query-index bounds, per-artifact index memo and build-time refusal (GitHub #1254)

## Problem

Nine index-backed corpus MCP tools (`docs_concept`, `docs_flow`, `docs_claims`,
`docs_dependencies`, `docs_recommend_tests`, `docs_gaps`, `docs_intent`, `docs_navigation`,
`docs_vocabulary`) refused every call on an 83,000-record adoption corpus with
`corpus-refused: indexed postings or key byte bound exceeded`. Three causes, verified in source:

- `BuildQueryIndex` bounded the index at 1,000,000 postings and 16 MiB of unique key bytes. Those
  bounds were sized for the 25,000-claim companion qualification, not for the `/2` record family
  bounds (100,000 per family). The issue's corpus sits at about 1.0 million postings.
- Claim-derived terms were charged twice: once in the claim loop and again in the final terms loop.
  The count was larger than what the index stores.
- The corpus MCP bridge memoized the artifact (DCP-V1-143) but not the index. `QueryContext`
  rebuilt the index lazily on every typed call and for every batch path, so a refusal cost a full
  derivation each time (20 paths took 11.9 s).

`docs corpus build` also wrote such artifacts without complaint.

## Decisions (DCP-V1-193..197)

- Raise the bounds to 8,000,000 postings and 64 MiB of unique key bytes. That is about twenty
  postings and 160 key bytes per record the `/2` family bounds admit. At the measured ~44 live bytes
  per posting, the bound allows a ~350 MiB index heap, which is under the 1 GiB open/query ceiling.
  Making the index not need a bound was rejected: it is the DCP-V1-040 companion's
  serialization contract, and unbounded growth would break invariant 7's bounded local path.
- Count every stored posting exactly once. Subject terms are charged in the subject loop and new
  claim terms in the claim loop. The final loop only appends.
- Past a bound, stop storing postings but keep counting, and refuse with the existing
  `corpus-capacity-exceeded` code (the DCP-V1-157 family): `indexed postings N exceeds the limit
  of 8000000`. No new error code. The per-key, family and fanout refusals keep `corpus-refused`.
- The bridge keeps one index, or its deterministic `*doccorpus.Error` refusal, per memoized
  artifact. It is derived before freshness, guarded by the existing load slot, and cleared when the
  artifact memo is replaced. Cancellations are not cached.
- `docs corpus build` uses the new `doccorpus.BuildAdmitted`, which refuses a capacity-over index.
  `Build`, which `Open` rederives through, is untouched, so artifacts written before still open
  for the reads that need no index.

## Identity

Artifact bytes are unchanged because `Build` is untouched. `TestDCPV1196...` asserts that
`BuildAdmitted` and `Build` encode to the same bytes. For in-bound corpora, the index content and
append order are unchanged. Only the place where the counting happens moved. The existing typed
query conformance and goldens, the corpusindex companion tests, and parity tests pass unchanged.

## Measurements

These are synthetic `padIndexCapacity` shapes, measured in a scratch test (not committed) with the
bounds raised for measurement, on darwin/arm64, non-race:

| Shape (subjects/claims/relations) | Postings | Unique key bytes | Derivation | TotalAlloc | Live heap |
|---|---|---|---|---|---|
| 60k/13.5k/9.8k (issue shape) | 1,010,489 | ~9.0 MB | 0.45–0.49 s | 324 MiB | ~45 MiB |
| 60k/25k/15k (DCP-V1-193 test) | 1,260,162 | 8,220,892 B | ~0.5 s | 380 MiB | ~50 MiB |
| 100k/100k/100k (family maxima) | 3,205,718 | ~25.6 MB | 1.4–2.5 s | 983 MiB | ~140 MiB |
| 100k/100k/100k, 400-word claims | same | — | 9.3 s | 5.4 GiB | — |

- The first index-backed bridge call pays one derivation, and later calls and batch paths reuse it.
  Cached refusals of all nine tools and a 20-path `docs_claims` batch each completed in under 50 ms
  (asserted outside `-race`), with one derivation in total.
- Test cost: `TestDCPV1193` takes 1.9 s, or 5.5 s under `-race`. The boundary tests lower the
  bounds through a test hook (`withIndexBounds`) so their fixtures stay small. The full
  `./internal/doccorpus/` package took 356 s.

## Evidence

- New tests, all passing: `TestDCPV1193HundredThousandRecordCorpusAnswersIndexedReads`,
  `TestDCPV1194IndexRefusalNamesBoundAndObservedCount`,
  `TestDCPV1195IndexRefusalDerivedOncePerArtifact`, `TestDCPV1195IndexDerivedOncePerArtifact`,
  `TestDCPV1196BuildAdmittedRefusesOverBoundIndex`. `TestCorpusQueryIndexBounds` now exercises the
  guards at the former bounds.
- Each test was shown to fail through mutations, each restored afterwards:
  - The former constants failed 1193.
  - The former `corpus-refused` message failed 1194.
  - Re-adding the final-loop double count failed 1194 (`indexed postings 11411 exceeds the limit of
    6899` at the exact stored bound).
  - `BuildAdmitted` without the check failed 1196.
  - Disabling the memo failed both 1195 tests (10 and 11 derivations).
- Full package runs passed: `internal/doccorpus`, `internal/mcp/corpusbridge`, `internal/corpusindex`,
  `internal/corpusrepublish`, `internal/corpusserve`, `cmd/corvint-corpus-mcp`,
  `cmd/corvint-corpus-http` and `tools/fixture-maintenance-check`. In `cmd/corvint`, the
  `-run 'Corpus|DCPV1'` subset passed. The new tests also pass under `-race`. `go vet` and `gofmt`
  are clean.

## NOT_RUN

- The other 84 units that `corvint affected` selected, and `make gate`, were not run (lane policy:
  focused tests).
- There was no live run on the reporter's 83k-record corpus. The issue shape was reproduced
  synthetically.
- The ordinary allocation budget under `-race` was not run.

## Residuals

- The first index-backed call still pays one derivation of about 0.5 s at the issue's shape, so
  only later calls meet the issue's 50 ms. A truly over-bound corpus pays a full derivation before
  its first refusal; a cheap pre-term lower bound could refuse sooner. Per-claim `EvidenceTerms` maps dominate allocation
  (5.4 GiB with long claims).
- `BuildAdmitted` reports only capacity refusals at build time. Other index refusals (identity,
  fanout, per-key bound) still surface at query time.

## Rollback

Revert this change. That restores the former constants and term charging, `Build` in
`docs corpus build`, and the bridge without an index memo. No artifact, index or wire format
changes, so nothing written needs migration.
