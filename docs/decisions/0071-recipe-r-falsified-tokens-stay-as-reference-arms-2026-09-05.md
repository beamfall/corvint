# Decision 0071 — Recipe R's falsified tokens stay as reference arms; the code-first variant is the held-out candidate; the default order is unchanged

Date: 2026-09-05. Status: accepted (2026-09-05, delegated call). Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that ranking lanes land once measured and that open calls be resolved and recorded. This record applies decision 0070's fold rule to the recipe R lane's measurement.

## Context

Recipe R (`TCP-V0-018`, proposed) was built opt-in behind `CORVINT_CONTEXT_RECIPE` with the
default packet byte-identical to the accepted order (golden test at 2a76e40). Measured on all
four v2 positive subsets and both repository folds against the `bench-b1b` baselines
(`docs/plans/context-bm25f-recipe-2026-09-05.md`):

- `r` lifts recall@5 above grep on every subset and MRR everywhere, but recall@20 falls 0.061
  below the accepted order on code2test fold A and 0.081 on trace2code fold B; the clause's own
  falsifier (more than 0.02 below on either fold) is met. Cause: documentation rows flood the
  top 20 once TCP-V0-013's quota is off.
- `r+anchor` is worse on trace2code fold B (recall@20 0.518 against 0.721): a trace's named path
  is the test file, so its directory and references outrank the implementation gold.
- `r+doctail` (code rows before documentation rows when the task is not documentation-shaped)
  has no recall@20 cell below the accepted order on either fold, recall@5 and MRR intervals above
  zero on four cells and below on none, and no fold sign flip. It was chosen after both folds of
  `r` were read, so on v2 data it is a consistency check, never a held-out result.

The clause as written removed the flag on falsification, which would also remove the only
reference arms the surviving variant's held-out run needs.

## Decision

1. `r` and `r+anchor` are falsified for promotion. They stay selectable behind the flag only as
   reference arms and carry no promotion path.
2. `r+doctail` is the candidate default for the retrieval shape. It is promotable only through a
   registered run on an unobserved partition (`v2_abstention` or blind-v4) under decision 0070's
   fold rule and the ladder arms; no v2 positive result counts.
3. The default packet order does not change. `TCP-V0-018` stays proposed with the measurement
   disclosed in its status line; the flag and every token are removed together when the recipe is
   accepted or retired.

## Evidence and falsifier

The lane's fold tables and paired intervals are in
`docs/plans/context-bm25f-recipe-2026-09-05.md`; the default-path golden is
`internal/contextindex/testdata/context-recipe-default-golden.json`. The candidate is falsified
if, on the held-out partition, recall@5 falls below the grep arm on any positive subset, recall@20
falls more than 0.02 below the accepted order on either fold, or the no-gold selective-success
endpoint on `v2_abstention` falls.

## Consequences

- The measured story behind "score-ordered documentation" is settled: the quota TCP-V0-013 keeps
  is what protects recall@20, and any recipe that drops it needs a code-first order instead.
- Anchor inference from named paths is retired for the trace shape until the named path's role
  (test or implementation) is classified first.

## Rollback

Delete the recipe block at the end of `internal/contextindex/taskcontext.go`, the
`CORVINT_CONTEXT_RECIPE` read, `TCP-V0-018`, and the golden fixture; the default path is unchanged
by construction, so rollback changes no packet.
