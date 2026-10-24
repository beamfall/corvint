# docs_search ranking leftovers of GitHub #1665

Date: 2026-10-24. Lane: claude/gh1665-rank-leftovers (base 0949876eb). Issue: GitHub #1665, the
re-test comment after #1683. Requirements `DCP-V1-540..546`
(`docs/specs/documentation-corpus-v1.md`) are proposed and experimental and await acceptance.

## Decisions

- **A query word counts once (DCP-V1-540).** The evidence tokenizer yields both `breaks` and
  `break` for the word "breaks", and each scored separately, so a short name holding one plural
  question word outranked a flow holding two subject words. Terms are now grouped by the query
  word that yields them and a group scores its best term. The frozen tokenizer and index are
  unchanged; only the read-time sum changed.
- **Tiers, not length normalization.** The issue suggested length normalization. The observed
  failures were not length effects: the file and the feature name were both phrase matches, and
  the shorter-name tie-break (`DCP-V1-477`) then preferred the feature. Splitting the phrase tier
  by where the phrase is held (`identifier`, `path-phrase`, `phrase`, `id-phrase`,
  DCP-V1-541/542) and adding `named` for questions (DCP-V1-543) fixes the reproduced shapes
  without moving any score, so no frozen score changed.
- **No encoding change.** The tiers are computed per read; no artifact, index key or derivation
  version changes, and old artifacts serve the new order.

## Frozen evaluation outcomes (linux/WSL2, heavy.sh)

| Fixture | Base 0949876eb | After |
| --- | --- | --- |
| `search-ranking-gh1665-leftovers` (new, DCP-V1-546) | 5 of 8 (question 2/3, identifier 3/5) | 8 of 8 |
| `search-ranking-gh1665` row 4, tightened to the strict file subject | 0 of 1 (the `index` declaration) | 1 of 1 |
| `search-ranking-gh1665` other cases | 7 of 7 | 7 of 7 |
| `search-ranking-gh1597` (DCP-V1-404) | 12 of 12 | 12 of 12 |

Changed expectations, and why:

- `search-ranking-gh1665` row 4 previously accepted any result anchored to
  `on_deck_controller.rb`; the `index` declaration satisfied it while the file ranked third
  (V1-2549). It now names the file subject.
- `TestDCPV1479SearchServesRankGroups` expected `terms` absent on `docs_search`. The re-test
  reported `terms: 0` on every result as unhelpful, so `docs_search` now serves the held query
  word count (DCP-V1-544).

## Limits

- The fixture reproduces the reported shapes synthetically; the owner's re-test on the original
  corpus is still required.
- A question naming a single-word subject gets no `named` tier.
- Row 5 derivation in multi-repository corpora (finding 1 of the re-test) is out of scope and
  filed as a ticket.
