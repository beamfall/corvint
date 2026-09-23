# Decision 0369 — Opt-in recency, blame and ownership features in `context`

Date: 2026-09-23. Status: proposed, experimental (ticket V1-0089; TCP-V0-035..038). Amends
nothing; it adds opt-in ranking features to the task-context packet and sets no default.

## Context

The packet's lexical fill orders by BM25 and its `cochange` slot by raw co-change counts. Neither
knows when a file was last changed, so a file rewritten last week and one untouched for two years
rank the same. The ticket asks for a recency feature with a 90-day half-life and a blame-derived
last-touch feature, both from immutable history at the indexed revision, bounded and abstaining
beyond their bounds, named in the row reason, and for any disagreement between CODEOWNERS and
blame to be reported as uncertainty rather than resolved. The ticket says CODEOWNERS is already
parsed; it is not (only `internal/genesis` classifies the file name), so this slice adds the
reader.

## Decision

1. History is read only from Git objects reachable from the indexed commit: the `cochange` slot's
   window (newest 200 non-merge commits, shallow boundaries dropped), the indexed commit's
   committer time as "now", and `git blame --porcelain` bounded to that window, to the first 10
   lexical candidates and to 4 MiB. No worktree, clock, index, snapshot or pack input.
2. Recency is `0.5^(age/90 days)` of a path's newest window commit; blame freshness is the decayed
   share of its current lines last changed inside the window. The lexical fill is reordered by
   BM25 x (1 + 0.25 recency + 0.25 blame) within the positions each of code and documentation
   already hold; the `cochange` slot by its decay-weighted count. Order changes only: no row,
   score, kind or authority changes, and reserved and syntax slots are untouched.
3. Every affected row names each feature, with its value or its abstention reason
   (`beyond the 10-file blame bound`, `no commit in the N-commit window`, `history-unreadable`,
   `no-indexed-commit`, `blame-unreadable`). An abstaining feature contributes 0.
4. CODEOWNERS (the first of `.github/`, root, `docs/`) is matched with GitHub's rules. An owner
   who matches no in-window blame author is a disagreement, reported in the row reason and in
   `coverage.recency.ownership` with state `disagrees` (all owners are emails) or `unverifiable`
   (a handle or team cannot be mapped to a commit email locally). It never changes ranking.
5. Gating: the features stay behind `CORVINT_CONTEXT_RECENCY=on` (the TCP-V0-019/022 pattern);
   unset or any other value preserves the packet bytes (recipe golden). The ticket's rule allowed
   default-on only if the frozen evaluation showed no recall@20 regression on every subset. The
   frozen `agent_retrieval_bench` releases rebuild each snapshot as one commit, so every recency is
   1 and every blame line is a root boundary: the features cannot reorder anything there, and a
   no-regression reading on that data is not evidence about real histories (invariant 2).
   `corvint eval` does not run `context`, so its on and off readings are equal by construction.
   With no corpus able to show either a gain or a loss, the default is not changed.

## Consequences

Operators who opt in get history-aware ordering and ownership uncertainty with every feature
named; the default product is byte-identical. The off/on readings are in `docs/BUILD-LOG.md`
(V1-0089). Promotion needs a frozen corpus whose snapshots keep their commit history, measured
with the same no-recall@20-loss rule, plus decision 0070's paired ladder. Mapping GitHub handles
and teams to authors is out of scope: it needs host account data.
