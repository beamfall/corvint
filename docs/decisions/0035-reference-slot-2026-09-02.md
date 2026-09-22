# Decision 0035 — the `reference` slot admits files that name a symbol the subject defines

Date: 2026-09-02. Status: accepted. Authority: repository owner, verbatim instruction "do 1 then
2" (2026-09-02) on the candidate list whose second item was "a reference slot by stem mention in
both directions and a cross-directory shared-identifier edge, gated by the offline probes", given
after the owner's goal "I want corvint to beat grep by a large margin" and the miss analysis in
`docs/agent-memory/ideas.md` (2026-09-02).

## What is decided

1. A `reference` slot runs after `reverse-import` and before `cochange`, capped at three rows: a
   readable source that names, as a whole word in the term table, a symbol of at least four bytes
   that the subject defines, weighted by the symbol's rarity `log((sources + 1) / referencers)`
   and summed per file; a name more than `contextMaxDefiners` (50) sources use admits nothing.
   Kind `reference`, confidence medium, authority `syntax`, score 600 plus ten times the weight
   clamped to 799, reason "names X, which the subject defines". TCP-V0-004 is amended.
2. The two stem directions the candidate named (a source naming the subject's stem; a source whose
   stem the subject names) are not shipped: probed with a directory-name exclusion, they changed
   no set's gold count against the symbol rule alone (below), and added rows.

## What was measured

Offline gold placement (the real binary, each base commit checked out, limit 20), the term-table
binary (decision 0033) against this one; "hard tasks" are tasks with a gold row beyond the
subject's stem in the top 20:

| set | gold rows in top 20 | tasks with gold | hard tasks |
|---|---|---|---|
| corvint v2 (63) | 244 to 252 | 59 to 61 | 52 to 57 |
| beamfall (60) | 98 to 101 | 54 to 55 | 37 to 39 |
| beamfall-apple (40) | 70 to 68 | 33 to 32 | 33 to 32 |

Variants on the same sets: both stem directions added (cap 3) 252 / 100 / 68; cap 2 with both
directions 256 / 97 / 69. On apple the slot displaces two lexical gold rows past the limit: its
119 rows there carry 13 gold, between the co-change and sibling slots' precision. Net across the
163 tasks: +9 gold rows, +6 hard tasks, one apple task lost. The model trial for this change is
not run; the offline probes gate it, as the owner set.

## What is not decided

Whether the apple loss wants a Swift-specific rule (its subjects define view bodies and modifiers
many files name); whether `hard_gold_in_context` (decision 0034) should replace gold rows as the
probe's gate; a rerun of the pooled trial with this binary.
