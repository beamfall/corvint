# Decision 0070 — `context` ranking lanes promote on a baseline ladder with paired intervals and repository folds

Date: 2026-09-05. Status: accepted (2026-09-05, delegated call). Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that Corvint be best in class across the board and that the panel find where it is not, and that open calls be resolved through the expert panel. This record applies the evaluation-protocol memo (retrieval-evaluation methodologist) after independent review of its probes.

## Context

The retrieval bench's `grep` arm scores every file by substrings of the JSON-serialised query,
so envelope field names (`pr_title`, `changed_file`) and glued escapes (`ngo`, `nfail`) are query
terms, and it never abstains. It is a legitimate deterministic control and weaker than what an
agent types. Two forty-line baselines built from the query values only, `grep-ident` (whole-word
identifier hits, path +1) and `bm25-ident` (BM25 over camel-split identifiers), match or beat the
`context` arm at top ranks on three of the four positive subsets: against `bm25-ident`, `context`
loses trace2code recall@5 (−0.081, 95 % CI [−0.158, −0.003]) and MRR (−0.091 [−0.159, −0.023]) and
wins recall@20 (+0.096 [+0.028, +0.165]); against `grep-ident`, code2test MRR is a borderline loss
(−0.048 [−0.100, +0.001]). Most of the gap between "grep" and `context` is query-side term
selection, not indexing. Paired bootstrap differences (B = 4000) show "beats grep" resolved on
code2test and comment2context only; trace2code's recall@20 pass under `TCP-V0-014` (+0.007) is
15 wins against 86 ties. Every v2 positive has now been observed under at least five
configurations, so under `benchmarks/README.md` the v2 positives are development data forever;
`v2_abstention` (82 samples) and blind-v4 are the unobserved partitions left.

## Decision

1. The bench tool reports a baseline ladder on every run: `grep` (unchanged control),
   `grep-ident`, `bm25` over whole files with term sets `all` and `ident`, and `context`. A
   ranking lane's gate is the maximum per cell of the ladder, not the `grep` control.
2. `summarize` reports, per task and arm pair, the mean paired difference of recall@5/10/20 and
   MRR with a fixed-seed bootstrap 95 % interval and win/loss/tie counts. Wilson intervals are
   not applied to recall. A point estimate with an interval spanning zero is "unresolved", never
   "beats".
3. Repositories are frozen into two folds (A and B, recorded in `benchmarks/README.md`). A
   parameter or mechanism choice is made on one fold, declared in the decision record before the
   other fold runs, and the other fold's first run is preserved unrepaired. A choice whose sign
   flips between folds is not promotable. `b = 0.3` was chosen with both folds visible, so any
   further constant (including a retune of `b`) is reported on both folds and the clean claim needs
   an unobserved partition (`v2_abstention`, blind-v4 or a later release).
4. Every report registers the samples digest and the binary digest so a first run is
   distinguishable from a rerun; per-arm wall time is recorded per sample under the
   `benchmarks/README.md` latency vocabulary.
5. Decision 0066 stays accepted on the falsifier it stated and passed; its record carries the
   paired-interval reading above as a labelled note, and `TCP-V0-014`'s falsifier text is
   rewritten to this rule for every successor lane (`TCP-V0-015` onward).

## Evidence and falsifier

A successor ranking lane is promotable when, on the fold it did not tune on, its paired
difference against the ladder maximum at recall@5 and MRR has an interval above zero on at least
two positive subsets and below zero on none, and the no-gold selective-success endpoint on
`v2_abstention` does not fall. The first target is the disclosed loss: trace2code recall@5 ≥ 0.502
and MRR ≥ 0.372 (`bm25-ident`) without losing code2test recall@5 0.225.

## Consequences

- "Nothing can match us" is not a claim Corvint can make today at top ranks; the honest external
  ceiling without embeddings is RepoMap-class structure-aware retrieval, which `context` is below
  on three subsets at recall@20 and at on trace2code.
- Query-side term selection inside `context` (identifier-aware weighting of what an agent would
  type) is the first ranking mechanism to build, ahead of new index structures.

## Rollback

Remove the ladder arms and paired statistics from the bench tool; old reports keep summarising
because missing arms are tolerated.
