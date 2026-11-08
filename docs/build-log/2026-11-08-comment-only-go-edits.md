# Stop compile-closure propagation for comment-only Go edits

Date: 2026-11-08. Ticket: V1-5176. Requirements: AFP-V0-3320 through AFP-V0-3323
in `docs/specs/affected-plan-selection-v0.md`.
Base: `8a1935c528bc314057d15e7db7c92f7c36384e01` (the worktree's captured `origin/main`).

## Design and review

The Go adapter compares the same measured BASE and HEAD used by import-set analysis.
Both bounded reads must parse with `parser.ParseComments`; scanner tokens and literal
text must match, with comments and positions omitted but inserted semicolons retained.
Protected directives and cgo preambles retain raw bytes and token attachment. Raw gaps
preserve legacy build-tag blank lines; the prefix before a legacy build constraint also
preserves its position inside the accepted header. A parenthesized single `import "C"`
uses its declaration's doc comment, matching the installed Go 1.27.3 cgo implementation.
Missing, unreadable, oversized, added/deleted/renamed endpoints, parse/scanner failures,
changed code, changed directives/preambles and analysis failures supply no narrowing proof.
Comment-inspecting test packages also retain full propagation conservatively.

`WithGoChanges` attaches this proof to a copy of the graph after cache lookup. Plain library
callers and path-only probes have no proof and retain the existing behavior. CLI measured
plans, snapshots and Go-aware library helpers supply the comparison. The optional full and
compact JSON `plan.goChanges` field names each `COMMENT_ONLY` path; existing fields retain
their shapes. Comparison evidence is recomputed for each plan, never stored in the graph cache.

Selection separates dirty ownership from compilation seeds. The own package remains a
`DIRECT_SOURCE_CHANGE`; compile-dependent packages, binary runners, import-graph readers and
driver build edges no longer follow that path. Other dirty code paths still propagate,
including a sibling file whose import-set witness was initially attributed to the comment file.
All literal, glob, scoped, unbounded and driver byte-reader rules still see the complete dirty set.

Independent review found and closed two unsafe cases: legacy build blank-line replacement,
and direct ownership masking a testless package's self-read. The latter now independently
resolves literal/glob reads before carrying their test callers. Follow-up inspection of local
Go source added the legacy prefix and grouped-single-import cgo regressions. Review passed
after these repairs. The CLI fixture was also corrected to restore its source before the
clean-worktree-only `--path` probe.

Unproven `go list` observers can read package documentation (`.Doc`), as AFP-V0-9010 already
records. They therefore remain selected with an explicit `UNBOUNDED_READER` source-comment
cause, while proven import-only readers are excluded. This preserves byte-reader safety without
retaining an `IMPORT_GRAPH_READER` edge for an unchanged token stream.

## Paired measurement

Built `../v1-5176-before` from the captured `origin/main` and `../v1-5176-after` from the
finished source using `GOTOOLCHAIN=local go build -o <binary> ./cmd/corvint`.
Both ran against the separate, otherwise clean `../wt-5176-measure` worktree at the base above,
so implementation edits could not contaminate selection. Appended a blank line and
`// V1-5176 comment-only propagation measurement.` to `internal/tasks/journal/anchor.go`,
ran `<binary> affected --base $(git rev-parse HEAD) --format json`, and restored the file with
`git checkout -- internal/tasks/journal/anchor.go`. The measurement worktree is clean afterward.

| Metric | Before | After |
|---|---:|---:|
| Selected units | 290 | 279 |
| Selected Go packages | 80 | 69 |
| Go `testCount` total (test-file entries, not executed test functions) | 2,001 | 1,583 |
| Unknown entries | 67 | 67 |

| Witness kind | Before, all units | After, all units | Before, Go | After, Go |
|---|---:|---:|---:|---:|
| DIRECT_SOURCE_CHANGE | 1 | 1 | 1 | 1 |
| DEPENDENCY_PATH | 57 | 5 | 14 | 0 |
| BINARY_EXEC | 10 | 0 | 10 | 0 |
| IMPORT_GRAPH_READER | 14 | 0 | 14 | 0 |
| PATH_LITERAL_READER | 26 | 41 | 24 | 38 |
| PATH_GLOB_READER | 1 | 1 | 1 | 1 |
| DECLARED_READ_SCOPE | 15 | 17 | 9 | 11 |
| UNBOUNDED_READER | 73 | 121 | 7 | 18 |
| FRONTIER_SITE | 93 | 93 | 0 | 0 |

Every previously selected literal/glob/scoped/unbounded reader remains selected. Surviving
units can change witness when their compilation witness disappears, explaining the increased
reader counts. The five remaining dependency witnesses are driver-to-reader paths: three vet
targets through the Playwright fixture, the local-authority-deps job through static-reuse,
and the spec-toolchain-conformance job through its make target. None starts at the Go owner.
The after receipt reports exactly one classification, for `internal/tasks/journal/anchor.go`.

The task's supplied earlier count was 279 units / 69 Go packages / 1,901 tests. This run did
not reproduce that baseline; the paired numbers above use one explicitly pinned source tree.
Raw receipts remain outside tracked source in `../v1-5176-before.json` and
`../v1-5176-after.json`. SHA-256:

- Before binary: `1266d0ad52ec1f34f1a5e2b65af34980aa09ae5fec664f92311d7897b3d9fac0`.
- After binary: `16bd61d33527fddb6eb5f5aabce04f7f54835218c561caa3cfe0cab3f6e7ce3c`.
- Before receipt: `d3972310419f071cf192cb9ab99af8399b163371aea3f20eaa98b41f8362065c`.
- After receipt: `9afcf80990ee28825358070410dbe0114e7ddeb680156eba57c82f27905463e9`.

## Verification and limits

The requested command, run separately for each package, passed:

```sh
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'AFP_V0_332|AFP_V0_220' ./internal/liveverify/affected/
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'AFP_V0_332|AFP_V0_220' ./internal/liveverify/affected/golang/
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'AFP_V0_332|AFP_V0_220' ./internal/liveverify/affected/languages/
script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'AFP_V0_332|AFP_V0_220' ./cmd/corvint/
```

The wrapper ran affected and CLI tests on Darwin/arm64, and adapter and library-helper tests
on its configured Linux/amd64 host, with Go 1.27.3. An initial sandboxed Git fixture failed
because process-group inspection was prohibited; its permitted rerun passed. The initial
CLI path-probe fixture failure was repaired and the final rerun passed. `GOTOOLCHAIN=local
go vet` passed separately for all four packages. `gofmt -l` reported no changed Go file.
`make spec-requirements` regenerated the four requirement rows after moving the definitions
inside the spec's Requirements section as the generator requires.
`make changed-checks BASE=8a1935c528bc314057d15e7db7c92f7c36384e01` passed after mechanically
repinning one unchanged selection citation in `core-compatibility-freeze-v1.md`. Its unbounded
reader check retained a non-failing advisory about the existing `internal/tasks/cli` declaration;
that unrelated declaration was not changed.

Corvint context and pre-test affected planning were used; the installed Core build 133 reported
its stale-source degradation, so measurement used freshly built binaries. The official updater
found no matching published Core release and retained freshness UNKNOWN; no update was installed.
Tasks, publication, the exhaustive test gate and post-commit CEM/OCM bind/seal commits were not
run in this explicitly local, one-commit task. Native ticket completion is not claimed.

The plan remains advisory with 67 unknowns in the measurement. This is lexical equivalence,
not semantic equivalence of different code or proof that static reader discovery is complete.
Protected-comment attachment and comment-inspecting package checks deliberately err toward
full propagation. Rollback is to stop attaching `WithGoChanges`; no cache/store migration is needed.
