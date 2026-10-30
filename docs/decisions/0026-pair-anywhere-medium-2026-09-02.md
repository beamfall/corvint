# Decision 0026 — a pair counterpart found anywhere in the tree is a medium row, not a high one

Date: 2026-09-02. Status: accepted. Authority: repository owner, verbatim instruction "rate the
elsewhere-in-tree pair medium and rerun" (2026-09-02), given on the run 3 reading.

## What was measured

Run 3 (`benchmarks/results/cw-trial-heldout-v1-development-run-3.json`), corvint arm, `pair` rows by
the relation their reason states: same-directory counterparts 18 rows, 14 holding gold, 3 claimed
`certain`, 1 of those wrong; counterparts "elsewhere in the tree" 35 rows, 4 holding gold, 5
claimed `certain`, 3 wrong; the mirrored-directory counterpart 1 row, claimed `certain`, wrong.
Every one of those rows carried `confidence: "high"`, and the prompt lets the agent say `certain`
when the supplied context establishes the claim, so the packet's word became the agent's.

## What is decided

1. `pairRows` rates a counterpart admitted by stem alone anywhere in the tree `medium`; the
   same-directory, mirrored-directory, and module-directory rows stay `high`. Nothing else in the
   slot order, caps, or scores changes, so the rows the agent sees are the same rows with one
   word changed on 35 of them.
2. The reading is a paired rerun on the same 50 tasks, model, effort, limit, and access (run 4),
   read against run 3 and against grep on clean success and confidently-wrong tasks.

## What is not decided

The mirrored-directory variant stays `high` on one observation. Whether the anywhere-in-tree rows
should be admitted at all, or moved behind `sibling`, is a retrieval question the rerun's gold
placement will inform; this record changes only what the packet says about them.
