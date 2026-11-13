# V1-4827: the dead-function report regrew; delete it again

Experimental, under the owner's standing lane auto-accept; lane `struct1113`, base
`366408098f741126468695245f461cb4fa9dbf8b`, contract `docs/specs/deadcode-ratchet-v0.md`
(DCR-V0-004, DCR-V0-007, DCR-V0-008). Follows `docs/build-log/2026-11-11-deadcode-ratchet.md`.

## What the base reported

`make deadcode` exited 2 at the base with 80 findings against the 35-entry baseline:

- 78 `NEW_DEAD_FUNCTION`: 68 test helpers in `internal/contextindex` and 3 in
  `cmd/corvint/fixture_helpers_test.go` whose tests went when decision 0597 removed task-context
  packets; 5 forwarding wrappers that the layer splits of V1-5249 left with no caller
  (`jstestprovider.IsServerErrorStatus`, `IsSuccessStatus`, `rubytestprovider.Revision`,
  `testvaliditydoc.PlatformRefusal`, `StatFile`); and two that a spec cites,
  `jstestprovider.QualifiedApplicationRevision` (`docs/specs/run-verified-navigation-v0.md`) and
  `testsupport.PrivateWorkRunParent` (`docs/specs/work-queue-observation-v0.md`).
- 2 `STALE_BASELINE_ENTRY`: `unplannedread.RefusePacket` and `worksource.RefusalReason`, whose
  packages were deleted.

## What changed

- Deleted the 76 functions DCR-V0-007 allows, with their doc comments, and the imports they left
  unused, using a position-checked AST deleter that refuses a declaration whose name does not
  match the report. The removed-declaration check: the 76 `func` lines removed in `6be10d3f8`
  equal the 76 report entries exactly, and no type, variable or constant is removed. One comment
  that named a deleted helper (`recipeFixtureTask` in
  `internal/contextindex/ranking_regression_test.go`) no longer does; the constant stays, as
  DCR-V0-007 removes functions only.
- Kept the two spec-cited functions with their citing line as the reason. A bare-name match was
  not treated as a citation when the spec names another package's symbol
  (`wire.PlatformRefusal` in `docs/specs/corvint-tasks-store-init-v0.md` is
  `internal/tasks/wire`) or uses the word in prose (`Revision`).
- Dropped the two stale entries. The baseline holds 35 entries again, in the same kept groups;
  `wantBaselineEntries` stays 35 with a comment on the change.
- `script/requirement-citations-allowlist.tsv` loses the seven fixture IDs (`ALP-001`,
  `BIL-V0-001`, `NAV-007`, `RDM-001`, `RUN-V0-002`, `RUN-V0-003`, `SHR-002`) that only the deleted
  helpers held; `make requirement-definitions-check` named them stale. One spec line citation into
  `internal/contextindex/termtable_test.go` was repinned (both in `c62c52b12`).

## Still not zero

V1-4827 criterion 1 asks for 0 baseline functions. The 35 kept are 16 spec-cited, 2 called by a
cited function, 11 counted by the diagnostic-coverage ratchet, 4 under
`internal/liveverify/affected/**` (owned by another lane this round), 1 under `.github/**` and 1
fixture. Deleting the cited or ratchet-counted ones needs the citing specs, several of them
accepted, and `script/diagnostic-coverage.count` to change first, which is an owner decision per
item; the `liveverify/affected` and `.github` ones need their own changes. The ticket stays open.

## Commands (DCR-V0-008, scoped to the five packages that lost a function)

- `make deadcode`: exit 2 at the base (80 findings); exit 0 after (`deadcode: OK, the report
  equals the 35-entry baseline`).
- `go vet` of `internal/contextindex`, `cmd/corvint`, `internal/jstestprovider`,
  `internal/rubytestprovider` and `internal/testvaliditydoc` for linux, darwin and windows on
  amd64 and arm64 (`CGO_ENABLED=0`): exit 0 for all 30. `go vet -tags corvint_development`: exit 0
  for four; `internal/contextindex` fails with the same five `undefined` errors in
  `taskcontext_uncertainty_development_test.go` at the base (checked in a detached worktree),
  unchanged. No other custom build tag occurs in those packages. Exported names deleted: no Go
  file anywhere in the tree names them. `go -C interop/cem01-go vet ./...`: exit 0.
- `script/heavy.sh go test -count=1`: `internal/contextindex`, `internal/jstestprovider`,
  `internal/rubytestprovider`, `internal/testvaliditydoc`, `tools/deadcode-ratchet`: PASS.
  `cmd/corvint` with `-run '^(TestCoreVerbsEmitTheFrozenProfiles|TestCLIKit)$'`: PASS; the whole
  package was not run (lane rule: `-run` only), and its vet in six configurations compiled the
  edited test file.

## Rollback

Revert `c62c52b12` and `6be10d3f8`: the allowlist lines, the functions, imports and the two
stale entries return and `make deadcode` fails as at the base.
