# Area Import Ratchet V0

Owner: Russell Lewis
Date: 2026-11-06
Requirement prefix: `IRT-V0`
Intent status: experimental
Delivery status: experimental
Authoritative inputs: ticket V1-4800 (step 1 of the 2026-11-06 expert-panel restructuring plan); decision 0397 (the existing Tasks import-direction rule, which this contract does not change); decision 0594 item 8 and ticket V1-5233 (the manifest keyed to the five layers shared, analyzers, tests, docs and release).

## Agent digest
- Claim: A checked-in area manifest layers every module package, and a Go test fails on any wrong-way cross-area import that its shrinking baseline does not list.
- Status: experimental; owner-directed lane implementation under the standing lane auto-accept, pending independent review; re-keyed to the five layers of decision 0594 on 2026-11-09 (V1-5233).
- Exists: `tools/import-ratchet/areas.json` (five layers), `tools/import-ratchet/baseline.json` (1 edge, `internal/sessiontest -> .github/cishards`, after V1-5248 and V1-5249 keyed the affected planner, `doccorpus`, `flowdocs` and `mcp/corpusbridge` shared and split the provider receipt models, the test-validity model and test discovery into shared packages, `docs/build-log/2026-11-11-layers-remaining-edges.md`; 17 edges after decision 0595 forwarded `corvint flows` and `corvint how` to `corvint-docs`; 44 edges at 9cb769243 after the re-key; 70 at origin/main 670559b9e under the eleven-area manifest), `tools/import-ratchet/ratchet_test.go`, `tools/import-ratchet/check_test.go`.
- Blocked on: independent review; the import-rule enforcement, `cmd/corvint` test split, CI sharding and per-binary release follow-ups of decision 0594 shrink the baseline.
- Read next: Requirements, then Failure modes.

## Problem and current state

The panel found the module's internal packages in eleven logical areas that all sit in one
dependency cycle: 78 of the 866 cross-area production imports point from a lower area into a
higher or peer area. Nothing stops a new wrong-way import, so the cycle can only grow while the
restructuring that removes it proceeds. Eight of those 78 were artifacts of the panel's area
assignment, not of the code: five leaf or near-leaf packages (`internal/cem/cemcode`,
`internal/cem/coverprofile`, `internal/cem/witnesspaths`, `internal/mcp/protocol` and
`internal/cem/patch`) sat in high areas. The manifest places them in the lowest area their own
imports allow, so the baseline starts at 70 edges. The job is a cheap, hermetic check that freezes the
current wrong-way set and lets it only shrink. The only existing direction check,
`internal/tasks/boundary_test.go`, covers the Tasks/Core boundary alone.

Decision 0594 (2026-11-09) cut Corvint down to test selection, documentation and flows, and
chose layers over repositories: shared, analyzers, tests, docs and release tooling, with shared
importing no other layer and analyzers, tests and docs not importing each other. Ticket V1-5233
re-keyed the manifest from the panel's eleven areas to those five (`shared` layer 0; `analyzers`,
`tests` and `docs` layer 1; `release` layer 2), so the baseline now measures the cross-layer
imports the follow-ups must remove. A package is keyed by role; a leaf that two products need
(the index's language dialects, `langplugin/decl`, `pythongrammar`) is `shared`; each `cmd`
binary is keyed to its product; `.github`, `tools`, `script`, `conformance`, `benchmarks` and
`examples` are `release`, which may import any layer and is not shipped. The re-key, the 44
measured edges by layer pair and the per-layer test time are recorded in
`docs/build-log/2026-11-09-layers-manifest.md`.

## Requirements

- `IRT-V0-001`: `tools/import-ratchet/areas.json` (profile `corvint-area-manifest/0`) SHALL name
  the module path and a list of areas, each with a unique name, a non-negative integer layer, an
  optional `entry` flag, a summary and module-relative package path prefixes. A package belongs
  to the area whose prefix is its longest path-segment match. Every scanned package MUST match a
  prefix (`UNMAPPED_PACKAGE`), every prefix MUST match a scanned package
  (`STALE_AREA_PREFIX`), and no prefix may be claimed twice or be malformed
  (`MANIFEST_FORMAT`). Unknown JSON members MUST be refused.
- `IRT-V0-002`: An import from package A into package B is wrong-way when their areas differ, A's
  area is not an entry area, and B's layer is greater than or equal to A's layer. Same-area
  imports, imports into a strictly lower layer, and any import from an entry area are allowed.
- `IRT-V0-003`: The import graph SHALL hold the production (non-`_test.go`) imports of module
  packages, read with `go/parser` in imports-only mode without running the go command or
  touching the network. A file counts when `go/build` constraint matching selects it for any of
  linux/amd64, darwin/arm64 or windows/amd64 (cgo off). Packages are those `go list ./...` walks
  (hidden, underscore, `testdata`, `vendor` and nested-module directories are skipped) plus any
  module package they import, transitively, as `-deps` adds. The module root package, when it
  exists, is the node `.`, and an import of the bare module path names it. Unlike `go list`, the walk also
  skips `node_modules` directories, so an installed JavaScript dependency tree (such as
  `tools/web-flows/node_modules`) is never read; a Go package placed under a `node_modules`
  directory is outside the graph unless a walked package imports it.
- `IRT-V0-004`: `tools/import-ratchet/baseline.json` (profile
  `corvint-import-ratchet-baseline/0`) SHALL list the accepted wrong-way edges as sorted, unique
  `"FROM -> TO"` strings. The test MUST fail with `NEW_WRONG_WAY_EDGE` naming the edge, both areas
  and both layers for any wrong-way edge not listed, and with `STALE_BASELINE_EDGE` telling to
  delete the line for any listed edge that is absent or no longer wrong-way. Unsorted or duplicate
  entries fail with `BASELINE_FORMAT`.
- `IRT-V0-005`: On the real graph the test SHALL prove its own effectiveness: adding one
  wrong-way import from a layer-0 package into a package of the highest non-entry layer
  (`release` since V1-5233; `cli` before) yields exactly one `NEW_WRONG_WAY_EDGE` naming it,
  and removing the first baselined import yields exactly one `STALE_BASELINE_EDGE` naming it.
- `IRT-V0-006`: The test SHALL run inside `go test ./...` in well under 10 seconds (measured 0.7 s
  on the lane host over 438 packages; 0.09 s over 298 after decision 0594) and carry `//corvint:read-kinds .go go.mod` (AFP-V0-9000), so affected
  selection reruns it on any Go source or module-boundary edit. The manifest and baseline are
  `go:embed` inputs, so their edits select it as well.

## Non-goals

No package moves, no change to decision 0397 or `internal/tasks/boundary_test.go`, no test-only
(`_test.go`) imports, no per-file exceptions, no CLI or generator for the baseline, and no claim
that the chosen layers are final: later restructuring steps may move prefixes or layers in a
reviewed change.

## Failure modes

- Adding a line to the baseline, moving a prefix to another area or changing a layer can admit a
  new wrong-way import. The test cannot tell this from a deliberate restructuring; such edits are
  visible in the diff and need review. The NEW_WRONG_WAY_EDGE message says not to baseline it.
- A platform outside the three named targets could select files whose imports are not counted.
  The product ships only those targets.
- A non-test file selected only by a custom build tag (any `//go:build` tag other than the GOOS,
  GOARCH and release tags of the three targets, such as `ignore` or an opt-in feature tag) is
  excluded from the graph, so its imports are neither checked nor counted.
- A malformed Go file fails the scan (the build would fail too); an import of a missing module
  directory fails it with the read error.

## Acceptance evidence and traceability

| Requirement | Evidence |
|---|---|
| IRT-V0-001 | `TestCheckRefusesManifestGaps`; `TestImportRatchet` (strict decode, all packages mapped) |
| IRT-V0-002 | `TestCheckAcceptsTheBaselinedGraph`, `TestCheckNamesNewWrongWayEdges` (peer-layer edge) |
| IRT-V0-003 | `TestScanModuleFollowsGoListRules` and `TestScanModuleRootPackage` (root package edges in both directions); under the panel's original area assignment the scanner's 78-edge set equals the panel's `go list -deps -json ./...` computation exactly, and the five-package reassignment removes exactly the eight listed edges (none added) |
| IRT-V0-004 | `TestCheckNamesNewWrongWayEdges`, `TestCheckRefusesStaleAndMalformedBaselines` |
| IRT-V0-005 | `TestImportRatchet` controls |
| IRT-V0-006 | `script/heavy.sh GOTOOLCHAIN=local go test -count=1 -v ./tools/import-ratchet/` (TestImportRatchet 0.71 s, 438 packages; 0.09 s, 298 packages and 44 edges after the V1-5233 re-key) |

Area names are the layers of decision 0594 item 8: `shared`, `analyzers`, `tests`, `docs` and
`release` (V1-5233). Until then they followed the panel's mapping (scripts of 2026-11-06):
`kernel`, `github`, `spi`, `analyzers`, `contextindex`, `doccorpus`, `appmap`, `affected`, `cem`,
`dogfood`, `tasks`, `release-ci`, `integrations`, `cli` and `entry`.

## Rollback

Delete `tools/import-ratchet/` and this spec's index rows. Nothing reads the manifest or baseline
outside that package, and no wire contract, store or CLI changes.
