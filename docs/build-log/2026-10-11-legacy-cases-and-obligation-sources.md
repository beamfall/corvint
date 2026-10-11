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
  faults are refused: unknown sections or suites, cycles, duplicate IDs and missing titles. A case
  naming a suite is refused when the export has no such suite, including an export with no suites
  (independent review round 1).

Limits:
- HTML stripping is a bounded tag scanner, not a parser. A tag ends at the first `>` outside a
  quoted attribute value, and an unterminated tag, comment or `script`/`style` body drops the rest
  of the field (independent review round 1).
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

## Issue #813: obligation review sources (TOL-V0-037..046)

An obligation entry, and an `OBLIGATIONS_SET` change, may now carry an optional `sources` list:
`{kind: legacy-case|legacy-spec|manual-case, ref, digest}` for a reviewed source, or `{kind: none,
query, searched, at}` for a recorded empty search. The member is absent when unused, so existing
entries and set requests keep their bytes, and the event profile stays
`taskman-obligation-event/0`, following the TOL-V0-034 alias precedent.

The policy key `obligations` gains an optional `requireSourceReview`. With it set, `preflight` adds
`SOURCE_REVIEW_MISSING` for each OPEN, DEFECT or BLOCKED obligation without sources, and
`SOURCE_REVIEW_NONE_UNSEARCHED` for a `none` with a blank query or an empty searched set. Each
finding names the obligation and carries a remedy.

`show` and `plan` add `sourceReview` (stale and unresolved lists) only when some entry has sources.
`legacy-case` and `manual-case` refs resolve by case ID in a `corvint-legacy-cases/1` record passed
as `--legacy-cases FILE`, which joins #812's import to the ledger. `legacy-spec` refs resolve as
repository paths at `HEAD`, from Git objects.

Design choices:
- The empty-query and empty-searched refusals sit in preflight, not the codec, so turning the policy
  on never makes a stored ledger unreadable.
- `plan`'s `ok` is unaffected by staleness.
- Review sources never reach witness evidence, counts or high water. The issue asks for review
  records, not credit.
- `workerWitness` stays required inside the `obligations` object. Only `requireSourceReview` is
  optional, which keeps the policy change additive.

Limits:
- A `none` source's searched ids are recorded, not checked against a corpus.
- Staleness of `legacy-spec` is read at `HEAD` only.
- `qualify` does not read sources (the issue names preflight, show and plan).
- `--legacy-cases` reads only a regular file, without blocking, and the record must be exactly one
  JSON value with no repeated member (independent review round 1).
- An older binary meets a sources-bearing set request at its closed reader and reports the ledger
  `UNKNOWN`. This is the same open CAL-V0-131 profile question as aliases.

Evidence:
- `TestTOLV0037_ReviewSourceCodec`
- `TestTOLV0039_LegacyLedgerBytesUnchanged` (`internal/tasks/ticket`)
- `TestTOLV0039_PolicyLeavesLedgerBytes`
- `TestTOLV0040_PreflightRequiresSourceReview`
- `TestTOLV0042_StaleSourcesListed`
- `TestTOLV0043_LegacyCasesFIFORefused` (`internal/tasks/cli`)
