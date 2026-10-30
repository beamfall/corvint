# Decision 0066 — The context lexical slot ranks by BM25 over three fields

Date: 2026-09-05. Status: accepted (2026-09-05, delegated call; measurement complete). Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that open calls be resolved through the expert panel, and this record applies `docs/reviews/b1-retrieval-breakthroughs.md` section B1 (the candidate-universe direction the panel ranked first) to the `context` verb only, after independent review of its cited evidence.

## Context

The `context` packet's lexical slot ordered hits by the count of distinct task terms, then
occurrences, then path (`internal/contextindex/taskcontext.go`, `lexicalRows`). That order has no
notion of rarity: `docs/reviews/b1-retrieval-breakthroughs.md` section 1b traces the blind-v3
misses to terms that "share common terms with many files", and the ordering by raw counts lets a
long file that repeats common words outrank the one file that names the task's identifier. Two
tokenisation gaps compound it: the body term table splits camel case, so the whole identifier the
task names (`stripFinalNewline`) was never a term, and question words (`how`, `retrieve`) were
terms whose rarity in code is an artefact of phrasing, not evidence.

`query` is oracle-pinned; changing its result kinds is the separate owner decision section B1
names. `context` is Go-only under `docs/specs/task-context-packet-v0.md`, so the same mechanism
lands there first and is measured where the retrieval gate failed.

## Decision

1. `TCP-V0-014`: the lexical slot scores every source with BM25 (k1 1.2, b 0.3) over three
   fields (body terms with length normalisation, path terms, whole identifiers as written against
   the identifier vocabulary) and orders by score, then distinct terms, then path. Task terms gain
   each whole camel-case or underscore identifier lowercased and lose a fixed English stop list.
2. `TCP-V0-013`'s five-code-row head counts rows the earlier slots took, so documentation can
   reach a small packet that definitions already fill.
3. Document lengths are derived on first use from the counted postings; the snapshot format,
   `query`, `eval`, every oracle-pinned command and the V4 admission gate (which scores `query`)
   are unchanged.

## Evidence and falsifier

The length-normalisation weight `b` was chosen on the development subsets, not held out: at the
text default 0.75, trace2code (101 samples, stack-trace queries whose gold is a large module)
fell from recall@20 0.820 to 0.795 and recall@10 0.680 to 0.465 because short configuration files
with one rare token outranked the module; at 0.3 trace2code reaches hit@k 0.901 and recall@20 0.833
(grep 0.891 and 0.827; the previous order 0.871 and 0.820) with recall@10 0.543 (grep 0.540,
previous order 0.680). `k1` stays at the text default 1.2. Neither value was tuned per case or per
repository, and blind-v4 stays sealed.

Measured on the landed tree before this change and with it, file-level critical selectors of the
three blind-v3 `query` cases through `context` at the cases' own limits: 5 of 6 missed before,
1 of 6 after (the remaining miss is `docs/api.md` for execa, outranked by two shorter documents).
At k=20: 3 of 6 before, 0 after. The Agent Retrieval Bench `context`-arm run against the grep arm
on the four positive subsets is the promotion gate written into `TCP-V0-014`. The run (2026-09-05,
binary built from this change, `tools/retrieval-bench` at limit 20, positives only) met it on every
subset, so this record is accepted:

| subset | n | context recall@5 / @10 / @20 | grep recall@5 / @10 / @20 | previous order @5 / @10 / @20 |
|---|---|---|---|---|
| code2test | 106 | 0.225 / 0.356 / 0.485 | 0.057 / 0.170 / 0.259 | 0.187 / 0.263 / 0.385 |
| comment2context | 80 | 0.221 / 0.344 / 0.479 | 0.106 / 0.138 / 0.273 | 0.204 / 0.256 / 0.375 |
| edit2ripple | 58 | 0.323 / 0.509 / 0.608 | 0.412 / 0.450 / 0.543 | 0.399 / 0.500 / 0.568 |
| trace2code | 101 | 0.421 / 0.543 / 0.833 | 0.356 / 0.540 / 0.827 | 0.517 / 0.680 / 0.820 |

Disclosed losses: grep still leads recall@5 on edit2ripple, and the previous order led recall@5
and recall@10 on trace2code; the gate is recall@20 and the top-rank losses are the next lane's
falsifier, not this record's. The reports are the bench's own JSON output; the previous-order
column is the same tool on the pre-change binary.

Implementation note (2026-09-05, evaluation-protocol lane, not owner text): paired bootstrap
differences of `context` minus `grep` (B = 4000) resolve the recall@20 gain on code2test
(+0.225 [+0.131, +0.322]) and comment2context (+0.206 [+0.113, +0.302]) but not on edit2ripple
(+0.065 [−0.017, +0.152]) or trace2code (+0.007 [−0.058, +0.068], 15 wins against 86 ties). Against
the stronger `bm25-ident` baseline `context` loses trace2code recall@5 and MRR. Decision 0070
makes the ladder and the intervals the gate for every successor lane; this acceptance stands on
the falsifier it stated.

## Consequences

- The packet's lexical order changes for every repository; slot caps, relations, reserved rows
  and the documentation quota are unchanged.
- The evidence reason now names the rarest matched term with its inverse document frequency and
  the score, so a reader can see why a file ranked.

## Rollback

Revert the `lexicalRows`, `taskLexicalTerms` and `documentLengths` change and restore the
distinct-terms order; no wire, snapshot or oracle migration is required.
