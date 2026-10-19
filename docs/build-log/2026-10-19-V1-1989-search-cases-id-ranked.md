# V1-1989: legacy case search by ID and ranked any-word match

Date: 2026-10-19
Ticket: V1-1989 (GitHub issue beamfall/corvint#1246).
Requirements: DCP-V1-165..169 (`docs/specs/documentation-corpus-v1.md`), accepted under the
owner's standing auto-accept of lane requirements (2026-10-11). DCP-V1-169 widens the
DCP-V1-077 closed argument set (adds `id` and `match`; `query` is no longer always required).

## Problem

`docs corpus search-cases` and `corvint.docs_search_cases` required every query word to prefix a
word of the title, path, preconditions or steps (`legacyMatch`), had no ID field or argument, and
returned unranked case-ID-order results. A natural-language question ("how do I cancel a paid
booking") found nothing because its function words never all appear, and a known case ID could
not be retrieved. Verified against `internal/doccorpus/legacy_cases.go` and
`cmd/corvint-corpus-mcp/legacycases.go` at 99dac5234 before designing.

## Decisions

- **One entry point, old one kept.** `SearchLegacyCasesQuery(format, path, data,
  LegacyCaseQuery{Query, ID, Match, Limit})` carries the modes; `SearchLegacyCases` delegates with
  the default mode, so existing callers are untouched.
- **ID lookup (DCP-V1-165).** `id` reuses the importer's ID form (`legacyCaseID`: `C` optional,
  1..10 digits, no leading zero), so `C12345` and `12345` both resolve; lower-case `c` and padded
  values are refused. The receipt keeps the existing schema with `mode: "id"`, `fields: ["id"]`.
- **Ranked any-word mode (DCP-V1-166).** Each non-stop word scores the best weight among the
  fields it prefixes (title 4, path 2, preconditions 1, steps 1); a stable sort by score keeps
  case-ID order on ties. The stop-word list is a fixed 37-word English function-word set, applied
  only in `any` mode and reported under `stop_words_dropped`. Weights are per token, not per
  occurrence, so repeating a word in a step cannot outrank a title match.
- **Exact `C<digits>` query rule (DCP-V1-167).** The query must be exactly one upper-case
  `C<digits>` word with no other character. It becomes an ID lookup only when a case holds that ID;
  otherwise the query is searched as before. Every other query, including bare digits, `c7`,
  ` C7` and `C7 queue`, keeps its old result. This rule does change the result of the exact query
  `C7` when C7 exists (previously it matched only text words beginning `c7`); that is the requested
  behaviour and is limited to that one query shape.
- **Byte identity (DCP-V1-168).** New receipt members (`id`, `mode`, `stop_words_dropped`,
  hit `score`) use `omitzero`, so a default or explicit `all` search emits the previous bytes.
- **Argument validation (DCP-V1-169).** The MCP schema adds `id` (pattern) and `match` (enum) and
  requires only `format` and `input`; no top-level `oneOf`, because some MCP clients reject it.
  The decoder enforces exactly one of `query`/`id` and `match` only with `query` (invalid params).
  The CLI admits `--query`, `--id`, `--match` as optional table options (so the shared help/flag
  table stays truthful) and checks the combination before reading the export (`invalid-arguments`).

## Evidence

- Byte identity: `TestDCPV1168LegacyCaseAllModeBytesUnchanged` pins sha256 digests of 14 all-mode
  receipts (both fixtures; queries incl. `C99`, `7`, `c7`, ` C7`, `C7 queue`) recorded by running
  the pre-change search at 99dac5234 in a scratch clone, and checks explicit `all` equals default.
- Issue acceptance: `TestDCPV1165LegacyCaseIDLookup` (`C12345`/`12345` exact, `C99` none-found),
  `TestDCPV1166LegacyCaseAnyMatchRanks` (the issue query ranks C12345 first with score 12; `all`
  finds none), `TestDCPV1169CorpusMCPSearchCasesModes`, `TestDCPV1165DocsCorpusSearchCasesModesCLI`.
- Fail without fix: with `score` emitted unconditionally, the query-ID rule disabled and the
  stop-word list disabled, TestDCPV1166/1167/1168 fail; TestDCPV1165/1169 and the CLI test do not
  compile against the old API.

## NOT_RUN

- `make gate` (owner preference for scoped lanes); the full `cmd/corvint` package was not run,
  only the corpus help, flag-matrix and legacy-case tests.
- No replay against the reporter's 3,728-case export (not supplied).

## Rollback

Revert the merge. No stored state, index or committed corpus encodes the new members; default
receipts are unchanged.
