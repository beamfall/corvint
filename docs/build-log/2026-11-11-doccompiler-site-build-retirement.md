# 2026-11-11: retire the test-only doc compiler site build (V1-4828)

Owner accepted the deletion on 2026-11-06 (expert-panel restructuring plan, step 24c). Base and
rollback source: `b55635262788687c3eea9a08a94378e4ab8856a4`. No new requirement IDs and no
product wire change.

## Reachability

`golang.org/x/tools/cmd/deadcode` (v0.49.0, built offline from the module cache) over `./cmd/...`
lists every function of `internal/doccompiler/build.go` as unreachable from the product commands,
and `git grep` finds no caller of `build`, `BuildDigest` or `QualifyOffline` outside the package.
The same run also lists most of admission, admitted plan, config observation, discovery and the
experimental plan as unreachable from `./cmd/...` (111 functions in all); only the site build is
in this ticket's scope, so the rest is untouched and recorded here as a finding.

## What was removed

- `internal/doccompiler/build.go` whole: `build`, `isolatedOutputParent`, `stageCorpus`, `copyTree`,
  `writeStageFile`, `validateSite`, `BuildDigest`, `QualifyOffline`, `applyBytePatch`,
  `scanGeneratedAssets`, `hasFetchBearingReference`, `sourceLimit`.
- Code only the site build used: `verifyExecutable` (`paths.go`), `descendantCleanupQualification`
  in the three platform files, the `BuildOptions` and `OfflineAttestation` types and the
  `defaultCorpusBytes`, `defaultOutputBytes`, `defaultOutputFiles` and `defaultBuildDuration`
  constants. A `deadcode -test` run over the package after the change finds nothing newly
  unreachable (the three functions it lists, `hasPyMdown`, `sortedInventory` and `PrecheckDraft`,
  were listed before it too; `PrecheckDraft` is reached from `internal/docstools`).
- Site-only tests: `TestBuildRefusesUncontainedUnixExecution`,
  `TestValidateSiteRejectsFalseStrictPassAndOutputBounds`,
  `TestOfflineQualificationCannotBePromotedAndGeneratedAssetsAreScanned`,
  `TestUnixContainmentRemainsUnqualifiedForSessionEscapes` and their helpers.
- Counts in `internal/doccompiler`: 670 lines deleted, 52 added, net 618.

## What stayed, and why

- `validateTrustAttestation` moved unchanged into `discover.go`: discovery calls it before the
  authoritative config load.
- `BuildResult` stays: receipts bind it, and receipt construction and verification still refuse
  every offline `PASS`.
- `TestPlanProducesExactDocumentAndNavBytePatchesForIsolatedCandidate` kept its assertion that the
  planned byte patches produce the rendered document and the proposed nav. It used `stageCorpus` to
  apply them; it now applies them with a test helper that checks the range and both digests the
  patch binds, and also checks the proposed document digest. The older `claude/deadcode` branch
  kept `stageCorpus` and three helpers in production for this test alone; moving the check into
  the test removes the last test-only production path.
- `TestProductionEntrypointsCannotReachNativeExecution` no longer lists `build` as a sink and fails
  if a function named `build` comes back.

## Spec

`docs/specs/human-documentation-compiler-v0.md`: HDCV0-033 to HDCV0-039 (site execution) are marked
retired with IDs and bodies kept; the header, digest and traceability table say so. HDCV0-040 and
HDCV0-042 stay live on the receipt code. In the secondary failure-code table, the 20 codes only the
site build emitted (19 in `build.go` and `stale-toolchain` in `verifyExecutable`) left the closed
set, with a note naming them; `environment-untrusted`, `isolation-failed`, `path-escape` and
`symlink-rejected` now cite their remaining first site with a pinned anchor and their new site
counts. The 23 unpinned `build.go` rows of `script/line-citation-unpinned-inventory.txt` are
deleted (rows are only ever deleted). `docs/specs/diagnostic-repair-contract-v0.md` re-cites the
trust check at its new site and describes the removed containment refusal by its retirement base.

## Verification

- `GOTOOLCHAIN=local go build ./...`: pass. `go vet` and `GOOS=windows go vet` over
  `internal/doccompiler`, `internal/docviews`, `internal/docmaintain`, `internal/docstools` and
  `cmd/corvint-docs`: pass; `GOOS=darwin go vet ./internal/doccompiler/`: pass.
- `corvint affected --base b55635262788687c3eea9a08a94378e4ab8856a4` (a local build of this
  checkout; the installed binary refused `self-binary-stale`): `scope=UNKNOWN dirty=23
  selected=237 excluded=413 unknown=58 LANGUAGE_FRONTIER=56 UNSELECTED_BINARY_CONSUMER=2`.
  Documentation units it selected, each run whole through `script/heavy.sh`, all `ok`:
  `internal/doccompiler` (6.4 s), `internal/docviews` (3.8 s), `internal/docstools` (5.1 s),
  `internal/diagnostic` (1.4 s), `internal/doccitations` (0.03 s), `internal/docmaintain` (3.3 s),
  `internal/doccorpus/selfcorpus` (12.4 s), `cmd/corvint-docs-mcp` (4.8 s), `cmd/corvint-docs`
  (25.9 s); `script/check-error-code-ownership_test.sh`: 8 cases passed. The other selected units
  (non-documentation readers selected by path literals and frontier sites) were not run.
- `make error-code-ownership-check`, `make documentation-paths-check`, `make line-citations-check`:
  pass.
