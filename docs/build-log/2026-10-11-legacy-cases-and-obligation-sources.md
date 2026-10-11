# 2026-10-11 legacy test-case import and obligation sources

Branch `claude/issues-812-813-legacy-cases`, from origin/main `645304a2f`.

## Issue #812: test-management case import (DCP-V1-070..079)

`corvint docs corpus import-cases --format testrail-json|csv --input EXPORT [--content
ids-paths-titles|full]` reads a caller-exported TestRail API document or CSV export. It joins every
case with its suite and section chain and emits `corvint-legacy-cases/1`. Each case is a
`BehaviorLegacyCase` keyed by `C<n>` with state `manual`, plus its path, a full-content digest and an
explicit `bounded` list. `search-cases` matches query-word prefixes over the title, suite/section
path, preconditions and steps. It returns only the case ID, path, title and matched fields, inside a
receipt that names the query, its words, the fields and the searched export, so a `none-found` can
be audited. The separate `corvint-corpus-mcp` lists `corvint.docs_search_cases` only when started
with `--legacy-cases`.

The legacy model change is one word: `manual` joins the admitted legacy states. That keeps it
additive for lane #811. The existing runtime check already gives a non-`executable` case no
baseline, so manual cases report `legacy-runtime-unknown` and never runtime parity.

Design choices:
- The default `ids-paths-titles` policy never emits precondition, step or expected-result text. The
  digest still covers them, so a step edit is visible.
- `full` is marked `local_only`.
- Unknown TestRail members are ignored because exports carry many tool-specific fields. Structural
  faults are refused: unknown sections or suites, cycles, duplicate IDs and missing titles.

Limits:
- HTML stripping is a bounded tag scanner, not a parser.
- Expected results are not searched.
- Imported anchors carry no repository revision, so a behavior registry does not admit them until
  the caller binds them to a commit.
- One export is read per call.
- No live TestRail export was available, so the fixtures are synthetic and follow the API's
  documented shape. External qualification is `NOT_OBSERVED`.

Evidence:
- `TestImportLegacyCasesFormats`
- `TestLegacyCasesIDsPathsTitlesWithholdsSteps`
- `TestImportLegacyCasesFullStripsHTML`
- `TestImportLegacyCasesBoundsOversized`
- `TestSearchLegacyCasesReceipt`
- `TestLegacyCasesRefusals`
- `TestBehaviorLegacyParity` (`manual case baseline`)
- `TestDocsCorpusLegacyCasesCLI`
- `TestDCPV1077CorpusMCPSearchCases`
