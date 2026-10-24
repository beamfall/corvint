# Served rank groups and derived declarations (GitHub #1665 rows 3 and 5)

Date: 2026-10-24. Lane: claude/corpus-rank-1665, stacked on claude/docs-corpus-gh1662 (base
bcbd5ded0). Requirements `DCP-V1-479..486` in `docs/specs/documentation-corpus-v1.md`, proposed and
experimental. Part of GitHub #1665; #1669 delivered rows 1, 2 and 4.

## Problem

- **Row 3:** results carried no score. A caller saw an ID-ordered run of claims and could not
  tell that it was one tie, or that a page cut it at an arbitrary boundary.
- **Row 5:** a method declared in a source input that a records provider owns (`def sync_with_pos`
  in `.rb`) had no subject. An identifier query and `--kind symbol` found nothing.

## Decisions

- **Served member, not a new schema.** `result_ranks` (rank, group, tier, score, and terms for
  claims) is an additive optional member of `corvint-corpus-receipt/2` under `DCP-V1-039`. It is
  derived per read and stored nowhere. It is omitted for other operations and empty results, so a
  reader that ignores it is unaffected. A score is rounded to 6 decimals so that equal scores
  encode equally.
- **Groups follow ranking keys only.** A group changes when the exact flag, the phrase flag or the
  normalized score changes. The shorter-text and natural-ID tie-breaks order a group but never
  split it. Claims group by distinct-term count and summed idf.
- **Derive declarations from the native index the build already has.** The first native provider
  emits a declaration subject and claim for each symbol in a primary-revision `source` input that
  no native provider claims. It uses the native provider's own IDs and evidence shape. No file
  subject or excerpt is added, and the owning provider's records are unchanged.
- **Derivation never refuses a previously buildable manifest.** If a compile that derived
  declarations is refused by a corpus bound or check, it is compiled again without them, with
  the incremental reuse state restored. The artifact then records one `not-collected` gap. This
  replaced an in-place record-count cut, which could not foresee the output byte bound.
- **Derivation version 2.** The same inputs now build different records, so
  `DerivationVersion` is `2`. The verified read refuses a derivation-1 artifact and asks for a
  rebuild. Old artifacts are refused explicitly; nothing is silently reinterpreted.

## Format and cost note

- Receipt bytes: `result_ranks` adds about 70 to 90 bytes per visible result for search and for
  claims with a query. It adds nothing elsewhere. The artifact and query-index encodings are
  unchanged.
- Artifact size: it grows by one subject and one claim per derived declaration. It is bounded
  by the existing record and output limits, and the retry covers both.
- Compile cost: unchanged when derivation is not refused. A refused derived build compiles
  twice. That is the only doubled case, and it ends with the pre-derivation artifact.
- Parity: recordings of `docs_search` answers taken before this change differ by
  `result_ranks`, and by derived symbols where records-owned source exists.
- No benchmark was run. The ranking order is unchanged and no hot path was added to the read.

## Measurement (DCP-V1-486 fixture, `docs corpus eval` MCP path)

| | base bcbd5ded | this change |
| --- | --- | --- |
| row 1 (exact identifier) | 1 of 1 | 1 of 1 |
| row 2 (phrase over shared terms) | 2 of 2 | 2 of 2 |
| row 4 (file subject for a phrase) | 1 of 1 | 1 of 1 |
| row 5 (identifier symbols) | 0 of 4 | 4 of 4 |
| `docs_claims` "admin": ranked results | 0 of 8 | 8 of 8, one tie group of 8 |
| `docs_search` "admin" + claims: ranked results | 0 of 10 | 10 of 10, groups 9 + 1 |

Row 4 is measured by path. Its native declaration symbol (`index`) ranks above the file
subject of the same path. That ordering predates this change and is filed as a ticket.

## Rollback

Drop `result_ranks`, remove the derivation and the retry, and return `DerivationVersion` to `1`.
Artifacts built in between are then refused as another derivation and must be rebuilt. No
stored state needs migration.
