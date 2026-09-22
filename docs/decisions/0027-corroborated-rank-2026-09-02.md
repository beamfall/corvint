# Decision 0027 — the task-context packet ranks corroborated rows first

Date: 2026-09-02. Status: accepted. Authority: repository owner, verbatim instruction "try the
ranking change" (2026-09-02), given on the reading that the agent claims gold rows by rank.

## What was measured

Across the three history-backed unseen runs (run 3 and prologue variants A and B,
`benchmarks/results/`), gold rows in the corvint packet were claimed at 80% when ranked 1 to 5,
62% at 6 to 10, 25% at 11 to 15, and 50% at 16 to 20; gold `sibling` rows sat at median rank 10
to 13 and were claimed 8 times in 24; gold `definition` rows at median rank 3 were claimed 0
times in 9. The slots dropped a candidate a later slot would also have admitted, so a file that
was at once a sibling and a co-change, or a co-change and a term match, carried one relation
and its slot's rank.

Offline, over the 34 unseen tasks at their base commits with history, before and after
corroboration: gold rows in the top 20 unchanged at 91; gold at ranks 1 to 5 from 36 to 54, at
6 to 10 from 31 to 14; 55 of the 91 gold rows are corroborated against 126 of 680 rows overall,
so corroboration marks gold at three times the base rate.

## What is decided

1. `take` records, for a path already chosen, every later relation that would have admitted it;
   after the lexical fill the packet ranks rows by that count (slot order among equals), raises
   `score` by 50 per corroborating relation, and appends "; also <relations>" to `summary`. The
   one evidence row stays the admitting relation's (TCP-V0-003).
2. Slot admission, caps, and confidence are unchanged, so the same rows appear in a different
   order.
3. The readings are the corvint-only reruns on both sets against their reused baselines
   (`cw-trial-unseen-corvint-v1-rank.json`, `cw-trial-heldout-v1-development-run-6.json`).

## What is not decided

Weighting relations differently, and the graph walk that subsumes both slot order and
corroboration (`docs/agent-memory/ideas.md`), wait on those readings.
