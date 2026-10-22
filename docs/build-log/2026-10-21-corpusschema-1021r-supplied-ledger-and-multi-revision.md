# Supplied status, run-ledger, migration-mapping and multi-revision corpus fields (V1-2331..V1-2334)

Lane `corpusschema-1021r`, 2026-10-21. GitHub #1488, #1489, #1491, #1490.

## Problem

Adoption producers could only say `blocked`, `ready` or `proven`, so a provisional or
classification-only result was either dropped or overstated as `proven` (#1488). A supplied test
run had no home for retries, flake verdicts, cleanup, a control outcome or validity axes, and a
supplied gap could not target a run (#1489). A code-migration ledger had no typed relation
between inventory units (#1491). An additional repository could be pinned at only one revision,
and `docs_locate` could not be restricted to one repository (#1490).

## Decisions

- DCP-V1-300..311 are accepted under the owner's standing auto-accept (2026-10-11).
- Status (DCP-V1-300..302): a closed nine-state set plus an optional closed qualifier
  (`provisional`, `classification-only`). Coverage keys a qualified state as `STATE:QUALIFIER`, so
  it never counts under the plain state; `unknown` is a recorded state, while a missing status
  adds to no row.
- Run ledger (DCP-V1-303..305): every field is optional and closed. The issue left the validity
  axes unnamed; this lane chose `revision`, `environment`, `isolation`, `oracle`, `determinism`
  with statuses `passed`, `failed`, `unknown`, `not-applicable`. Adding an axis later is additive.
  `control_outcome` reuses the DCP-V1-210 outcome set.
- Gap targets (DCP-V1-304): a gap targets a run through the claim that carries its `test_run`
  detail. Bare run identifiers are not unique corpus IDs, so they are refused as targets; this
  narrows the issue's "or run id". `docs_gaps` on a subject also returns gaps on its claims.
- Migration mappings (DCP-V1-306..307): one-sided outcomes (`added`, `removed`, `unresolved`) are
  self-loop relations on the one unit; every other outcome joins two distinct units. Readiness is
  an ordinary status claim on the unit. `docs_get`/`docs_trace` append the reachable mapping chain
  (both directions, artifact order, no duplicates).
- Multi-revision repositories (DCP-V1-308..311): `revisions` lists further commits read in the same
  checkout; all pinned revisions count toward the eight-revision bound (typed capacity refusal).
  Freshness keeps comparing only `revision`; no freshness code changed, because the existing
  per-repository member predicate already admits only that revision. The `repository` qualifier
  is a library and MCP argument for `locate`/`recommend-tests`; multi-repository receipts carry
  `result_repositories`. Single-repository manifests, artifacts and receipts keep their bytes.
- Supplied fields never grant trust; unknown values are refused (`corpus-refused`).

## Evidence

- Focused tests: `TestDCPV1300*`, `TestDCPV1303*`, `TestDCPV1304*`, `TestDCPV1306*`,
  `TestDCPV1308*`, `TestDCPV1310*`, `TestDCPV1311*`, plus the existing DCP-V1-175..179, 185..189,
  210..215 and adoption tests, pass in `./internal/doccorpus`.
- Without the fix: on the base sources every new test file fails to compile (missing `Qualifier`,
  ledger fields, `MigrationMapping`, `Revisions`, `ResultRepositories`); with only the multi-revision
  lookup reverted, the DCP-V1-308/310 tests fail (`bad object` for the older revision).
- Sibling and importer package results are recorded in the PR body.

## NOT_RUN

- The `corvint docs corpus` CLI has no `--repository` flag yet (library and MCP only); filed as a
  residual. `cmd/corvint` tests were not run (unchanged).
- Tier-2 affected packages are left to batch CI.

## Rollback

Revert the lane commits. All new members are `omitempty` and absent from existing records, so no
artifact or manifest migration is needed.
