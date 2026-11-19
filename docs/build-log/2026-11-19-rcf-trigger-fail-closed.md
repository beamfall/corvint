# V1-5692 — Require current qualification for pagination and citation changes

RCF-V1-010 and RCF-V1-013 govern reuse of a passing qualification. At
`320a1c5a2cf66310233670c4ad222d8c1266d8c8`, a body-only edit to either
`internal/doccorpus/pagination.go` or `internal/doccorpus/compact_anchor.go` returned
`PASS ... reason=no-trigger-diff`. The former bounds the visible records and citations. The
latter expands compact provider anchors during corpus build. Both fixtures used the real,
byte-identical owner declaration at the qualified and candidate commits.

## Trigger boundary

`cmd/corvint-docs` dispatches the promoted corpus verbs and calls `docsserver.Serve` for MCP.
The server registers the corpus group from `internal/mcp/corpusbridge`. Use the repository-local
production import closure of that executable as a conservative boundary. It includes shared
input validation, Git and artifact reads, line anchors, rendering, and MCP transport. There are
no file-level exclusions within those packages. The separate `cmd/corvint-docs-mcp` registers
draft, consume and help only. It has no corpus registry. Its shared dependencies that also
compile into `corvint-docs` are covered.

The qualification owners match `*.go` in each package, including test files and new source
filenames. Related package directories share patterns to fit the existing 256-owner limit.
These patterns can select sibling packages as well. The initial declaration had 253 owners. The frozen
query-set directory keeps its owner. Source changes anywhere in `internal/doccorpus` or
`internal/mcp/corpusbridge` also select the existing ranking test. The gate now checks that closure before pin reuse.

## Verification

- `TestCurrencyPromotedSourceChange` first observes pin reuse with unchanged source, then
  requires the existing missing-configuration error for each isolated source edit. The two
  base reproducers and the CLI, MCP protocol and line-anchor cases pass in the whole
  `tools/rcf-currency` package.
- `go list -deps ./cmd/corvint-docs` yielded the same 91 repository packages on linux/amd64,
  linux/arm64 and darwin/arm64. The production parser and matcher covered all 867 tracked
  non-test Go files and a new filename in each of the 91 packages. Comparing matches on
  10,642 tracked paths preserved every base owner/test pairing. Coverage log SHA-256
  `d247fb5638ef0787f8248f166b19246afcefba76b2d217650edd6250369998d3`.
- `corvint affected --worktree --path internal/doccorpus/pagination.go` selected
  `cmd/corvint-docs` and `internal/mcp/corpusbridge` through the doccorpus owner. Its overall
  scope is `UNKNOWN` for Go build-constraint variants and two unselected binary consumers.
- The declaration compatibility and read-scope validator package tests passed, as did
  `TestRepositoryOwnerRowsNameExercisingTests_AFPV010329`. Live three-repository release
  qualification is `NOT_RUN` in this focused lane.

Reverting these owners reopens the stale-pin path. If a rollback is needed, require a fresh
RCF-V1-008 qualification and both RCF-V1-009 ranking runs before accepting another candidate.

## Self-review repair

A newly pinned import of an unowned package left later body edits eligible for reuse. The
release gate now runs offline `go list -deps -json` for `cmd/corvint-docs` on each target in
the committed release catalog. Both the catalog and `go.mod` require qualification owners.
Only main-module Go source and resolved embedded assets qualify as repository build inputs.
Standard-library packages come from the release toolchain. The gate also checks ownership
of Go files omitted by build constraints. Unknown package fields, native inputs, list errors
and uncovered files require fresh qualification. A deletion below a reachable package also
requires qualification because a removed wildcard embed can disappear from the candidate list.

The 255 owners cover the host schema, the trusted-navigation probe and the capability catalog's
embedded files. The complete DCS-V1-404 fixture directory selects both qualification and ranking.
In an isolated checkout of the actual source with a synthetic pin, a pin-only diff reused the
pin. Changing only the schema version, the `file-cancel` expected subject, or the body of a
newly pinned unowned dependency each produced the existing missing-configuration refusal.
`TestCurrencyProductionCoverage` exercises these edits in private Git fixtures, including
target-specific imports and removal of an embedded file. `TestCurrencyUnknownPackageField`
checks the default for a future Go input kind. The report and the pin format are unchanged.

## Independent review repair

A Go filename can name bytes outside its Git blob. Pin reuse now admits local source,
embeds, the module file and the release catalog only at owned candidate-tree paths with
mode 100644 or 100755. The gate resolves the unique docs product in version 1 of the
release catalog and requires its package to match the one the qualification test exercises.
A moved package needs a matching qualification runner before either evidence path can pass.

Go omits the automatic PGO profile from its package listing. In the command directory
the reuse proof admits owned regular Go files and the fixture directory `testdata/`.
All other entries require live qualification, including an unfamiliar file type or an
ignored profile. Resolved embeds inside the fixture directory still need their own owners.
This conservative directory rule preserves the release builder's PGO settings.

The repository owner regression checks the real command and transitive production inputs
on each catalog target using the checked-in declaration. Its confined read scope grants
the command, module metadata, catalog and dependency directories used by that check.
