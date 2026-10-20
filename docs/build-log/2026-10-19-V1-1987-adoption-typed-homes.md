# V1-1987: typed homes for supplied gaps, status and manual case links (GitHub #1244)

## Problem

Adoption providers could not carry producer-curated gaps, readiness/proof status or links from
imported manual test cases to flows, so adapters folded them into prose claims or dropped them. Under
`/2` manifests, observations kept the 4,096 bound while the other record families reached 100,000.

## Decisions (DCP-V1-185..192)

- Supplied records are producer claims, not authority (AGENTS.md invariant 3). Supplied gaps carry
  `origin: supplied`, are kept in `supplied_gaps`, and `docs_gaps` lists them after the computed gaps.
  They never add, remove or rewrite a computed gap. Evidence passes the ordinary provider trust ceiling:
  `reviewed` becomes `generated` plus `reported_trust`, and `verified` is refused.
- A readiness/proof status is a typed `status` detail on a claim (`blocked|ready|proven`, a reason,
  and joined supporting ids). It never changes computed results. `docs_coverage` reports it only as
  `supplied_status:STATE` rows labelled supplied.
- A `manual_case` subject linked by `manual_case_for` counts as a manual disposition for the
  target's `missing-manual-cases` gap. This is the one computed result a supplied record can change,
  because it is the same relation-shaped evidence that `journey_for` already supplies.
- No new error codes: refusals reuse the existing provider-invalid paths.
- `/2` adoption observations and supplied gaps use `recordLimit()` (100,000). `/1` stays at 4,096.

## Measurements and evidence

- Focused tests, all passing (`-run 'TestDCPV118|TestDCPV119|Adoption|Provider|QueryIndex'`):
  `TestDCPV1185SuppliedGapAdmittedAndLabelled`, `...NeverRaiseTrust`, `...RecordRefusals` (17 cases),
  `TestDCPV1187SuppliedStatusQueryable`, `TestDCPV1189ManualCaseLinkSuppressesMissingManualCases`,
  `TestDCPV1191SchemaTwoObservationsUseRecordLimit`.
- Each test was shown to fail without the fix through six mutations, each restored afterwards:
  no case-link check (1189), observation bound back to `MaxRecords` (1191), no supplied gaps in
  docs_gaps (1185), no origin label (1185), no status coverage (1187), and no evidence import (trust).
- Byte identity (DCP-V1-192), measured with a scratch test run at base 62fd18a0 and at head with
  fixed Git dates. For both the adoption fixture and the plain provider fixture, the artifact bytes
  and the `gaps` and `coverage` receipt bytes were identical.
- Package runs: `internal/doccorpus` (361 s), `internal/corpusrepublish`, `internal/corpusindex`,
  `internal/mcp/corpusbridge` and `cmd/corvint-corpus-mcp`, all ok. `go vet` was clean.

## NOT_RUN and residuals

- `cmd/corvint` package tests (about 591 s) and `make gate`: NOT_RUN under the lane policy.
- Missing from the issue: typed test-run receipts beyond the existing test-validity format; mapping
  legacy-case ids from an external MCP export (only imported `manual_case` subjects are supported);
  supplied gaps are not reported in capabilities or the query index.

## Rollback

Revert the commit. The new fields are `omitempty`, so artifacts built without supplied records are
unchanged, and no stored format migrates.
