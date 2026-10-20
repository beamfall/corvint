# V1-1993: supplied test runs, legacy case links, supplied gaps in capabilities (GitHub #1244)

## Problem

V1-1987 (DCP-V1-185..192) gave supplied gaps, readiness status and manual case links a typed home.
Three items from GitHub #1244 remained:

- An adoption provider could not supply a typed test-run receipt.
- A legacy case ID from an external case export (the `docs_search_cases` input) could not be linked
  to a corpus subject.
- Supplied gaps were missing from the `gaps` capability and the query index, so `docs_get` by a
  supplied gap ID found nothing.

Separately, the incremental path counted every claim as `claim/<claim_kind>`, but a cold build
counts status claims as `claim/status`. An incremental shard with V1-1987 status claims therefore
failed with `cached import parity incomplete`. The new test-run claims would have hit the same bug.

## Decisions (DCP-V1-210..215)

- **Test-run receipts.** A test-run receipt is a `test_run` claim detail on the subject that was
  run. It has a closed outcome set (`passed|failed|skipped|errored`), a run identifier, and an
  optional pinned revision and time.
  - `origin` is required and must be `supplied`. The label is therefore part of the input itself:
    the compiler does not rewrite details, and the incremental same-input check is unaffected.
  - Test-run receipts follow the same supplied-only route as status: `claim/test_run` in
    `import_parity` and `supplied_test_run:OUTCOME` rows in `docs_coverage`.
  - A receipt never changes a computed gap, journey state or trust (AGENTS.md invariant 3). The
    compiler does not execute or verify the run.
- **Legacy case links.** A `legacy_case {id}` detail on a `manual_case` subject declares the
  canonical external ID (`C<digits>`, the `legacyCaseID` form `docs_search_cases` already uses).
  - The ID must be unique across subjects.
  - `docs_get C123` falls back to a legacy resolution only when the ID names no record. It returns
    the declaring subject and its `manual_case_for` links.
  - Otherwise `docs_get` returns a `legacy-case-unlinked` gap that says why.
  - The corpus never reads the export, so it does not verify that the case exists there.
  - `docs_search_cases` is unchanged, because DCP-V1-077 requires the MCP to return the CLI's exact
    bytes.
- **Capabilities and query index.** The `gaps` capability appends supplied gap IDs after computed
  gap offsets. The query index adds record kind `supplied_gap` under the record limit. Every index
  consumer already filters by kind, so no other query changes.
- **Incremental fix.** `claimParityKind` is now the single function that both the cold path and the
  incremental path use to count claim kinds.
- **Error codes.** No new codes: all refusals are `corpus-refused`, reusing the provider-invalid
  paths.

## Measurements and evidence

- Byte identity (DCP-V1-215) was measured with a scratch test, run once against the base
  (0ea7f47f) sources and once against the head sources, with Git dates fixed. The scratch test was
  removed afterwards. Two fixtures were measured: the adoption fixture and the plain provider
  fixture. For both, these hashes were identical at base and head:
  - the artifact
  - the query index
  - the `info`, `gaps`, `coverage`, `search`, `get` and `trace` receipts, each with and without the
    runtime index
- **Expected identity change.** A corpus carrying V1-1987 supplied gaps (`suppliedHomes`) changes
  its artifact, index and receipt bytes, because the `gaps` capability now lists the gap IDs. Such
  corpora have existed only since V1-1987 merged earlier today. A persisted index built for one
  needs a rebuild, which the existing re-derive comparison on open enforces.
- **Behavior change.** `docs_get` with a canonical `C<digits>` ID that names no record now returns
  the `legacy-case-unlinked` gap instead of an empty result.
- **Focused tests**, all passing:
  - `TestDCPV1210SuppliedTestRunReceipt`
  - `TestDCPV1210ResidualRecordRefusals`: 12 cases, each asserting its refusal reason
  - `TestDCPV1212LegacyCaseIDLinksToSubject`: with and without the index
  - `TestDCPV1214SuppliedGapsInCapabilitiesAndIndex`
  - `TestDCPV1215IncrementalCountsSuppliedClaimKinds`
- **Fail-without-fix.** Eight mutations were applied one at a time and each restored afterwards.
  Each made its test fail:
  1. Test-run parity kind changed to `unspecified` (1210).
  2. Coverage prefix renamed (1210).
  3. Origin check removed (refusals).
  4. `getLegacyCase` call removed (1212).
  5. `manual_case` kind check removed (refusals).
  6. Capability append removed (1214).
  7. Index loop emptied (1214).
  8. `incremental.go` restored to base (1215 fails with `cached import parity incomplete`).
- **Package runs**, all ok:
  - `internal/doccorpus` (384 s)
  - `internal/corpusindex`
  - `internal/corpusrepublish`
  - `internal/mcp/corpusbridge`
  - `cmd/corvint-corpus-mcp`
  - `internal/corpusserve`
  - `internal/doccorpus/selfcorpus`
  - `internal/mcp/docsbridge`
  - `tools/fixture-maintenance-check`
- **Static checks.** `go vet` and `gofmt -l` are clean.

## NOT_RUN and limits

- **NOT_RUN under the lane policy.** `corvint affected` selected 83 packages, with unknowns for
  Go build-constraint variants and the nested-module frontier. The ones not run are:
  - `cmd/corvint` package tests (about 591 s)
  - the remaining packages `corvint affected` selected only through binary-exec, unbounded-reader or
    transitive dependency edges
  - `make gate`
- **Known limits.**
  - A legacy case link is a producer declaration. It does not prove that the export contains the
    case.
  - Test-run receipts are not reconciled with the test-validity format or with Corvint's own test
    evidence.
  - The dogfood terminal outcome is recorded in the PR, because it is produced after this commit.

## Rollback

Revert the commit. The new detail fields are `omitempty`, and corpora without them keep their
bytes. Corpora with supplied gaps return to the V1-1987 capability and index bytes after a rebuild.
No stored format migrates.
