# V1-2000: no sound pre-term index refusal exists (GitHub #1254, failed evaluation)

## Problem

V1-1995 (DCP-V1-193..197) left one residual from #1254's second acceptance test: the first
index-backed call on an over-bound corpus pays one full query-index derivation before it refuses.
Later calls and batch paths reuse the memoized refusal in under 50 ms. This lane evaluated a cheap
bound check that runs before term extraction, could refuse an over-bound corpus in under 50 ms on
the first call, and would never refuse a corpus that the full derivation admits.

## Evaluation

A sound early refusal needs a lower bound on the postings or the unique key bytes that
`BuildQueryIndex` charges. Only the term postings need `contextindex.EvidenceTerms`. Every other
posting can be counted from record fields alone: one identity per record, the distinct anchor path
and symbol keys per record, one subject per claim, and one or two endpoints per relation. Term
extraction has no sound cheap lower bound, because text length bounds the distinct-term count only
from above. For example, a long claim that repeats one word yields one term.

The term-free bound was measured in a scratch test (not committed) on darwin/arm64, non-race, using
the issue shape (`padIndexCapacity` with 60k subjects, 13.5k claims and 9.8k relations), with three
runs each:

| Measure | Value | Time |
|---|---|---|
| Term-free posting lower bound | 282,981 | 4.3–4.8 ms |
| Sum of record identity bytes (key-byte lower bound) | 3,457,044 B | same pass |
| Full derivation (admitted) | 1,070,274 postings, 7,662,092 key bytes | 427–450 ms |
| Full derivation, posting bound lowered to 1,070,273 | refused, names 1,070,274 | 371–387 ms |

Terms are 74% of the postings at the issue shape. The term-free bound is fast, but it sees about a
quarter of what the index holds.

## Why the term-free bound can never refuse in production

Every artifact that reaches `BuildQueryIndex` encodes within its codec bound: `Open` re-encodes the
rebuilt artifact through `Encode`, which refuses anything over `MaxCorpusBytes` (128 MiB) for `/2`
or `MaxBytes` (4 MiB) for `/1`. `Anchor` has no `omitempty` field, so each anchor encodes to at
least 178 bytes, and a `/2` artifact holds at most 754,032 anchors, which is at most 1,508,064
path and symbol postings. Add at most 404,096 identities (four families at 100,000 records plus
4,096 journeys), 100,000 claim subjects and 200,000 relation endpoints. The term-free count is then
at most 2,212,160, below the 8,000,000-posting bound (DCP-V1-193). A precheck on it could refuse
only when a test lowered the bound, so in production it would be dead code. The identity-byte bound
is in the same position: 3.4 MB at the issue shape, against 64 MiB.

Over-bound corpora are therefore over because of terms. Showing that a corpus is over the bound
means counting more than 5.8 million distinct subject-term pairs. At the measured ~400 ns per
posting, that takes seconds on one core. V1-1995 measured 1.4–2.5 s for 3.2 million postings at the
family maxima. Parallel counting would not bring this under 50 ms either. An early refusal from a
lower bound would also break DCP-V1-194, which requires the message to name the full observed
total.

## Decision

Ship no code. A pre-term check that is both sound and cheap cannot reach the posting bound for any
artifact the codec admits. Nothing in the artifact or the read path can carry a precomputed count:
the artifact bytes are frozen by DCP-V1-197, and invariant 4 forbids a read-side cache write.

The first call also cannot meet 50 ms at the issue's shape for a second reason. The first call in a
session rederives the artifact through `Open` (`Build`, `Encode` and a byte comparison), which the
reporter measured at 8.1 s for the build alone, before any index work. The attainable reading of
#1254's second acceptance test is "refused once, naming the bound and the observed size, and every
later call or batch path in under 50 ms". V1-1995 delivered and measured that. The first-call
latency claim needs an owner decision: either accept that reading, or file a separate
performance ticket for the derivation (allocation-heavy per-claim `EvidenceTerms` maps), measured
on both admitted and refused corpora.

## Identity

There are no source, spec or wire changes. Artifacts, indexes and receipts are unchanged.

## NOT_RUN

- No package tests were run beyond the scratch measurement, because no Go source changed.
- There was no live run on the reporter's corpus.

## Rollback

Delete this entry.
