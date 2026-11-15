# affected-change-status task 8 re-measure; V1-4362 AC2 and AC3 (V1-5191, V1-5194, V1-4362)

Lane ms1115. Base `babe97abb110feb735fef9098e0e91765c102e46`. This entry re-runs the task 8
measurements of `docs/specs/openspec/changes/affected-change-status/tasks.md` on a base that holds
walksel and affgate (PR 197, merge `072760041`). It also retains the V1-4362 replay. Task 8 stays
open: the pr-39 replay is still `FALLBACK`, for a reason the shell work cannot reach.

## Finding

walksel recorded pr-39 as `run, ci NARROW` (`docs/build-log/2026-11-11-walk-selection.md`). That
measurement used its code commit `e88626872`. That commit does not hold the non-code map selector
rule (`622c58513`, "Run everything for unmapped non-code paths in the selector"). Both landed in
the same merge, `072760041`. On the merged tree, the selector applies NCM-V1-006 ("Corvint
configuration runs everything") and the never-narrowed paths before any plan unknown is weighed.
pr-39 edits six `.corvint/` configuration files, the `Makefile` and two npm manifests. Its
`FALLBACK` is therefore required by an accepted requirement, not caused by a planner gap.

The selector's own lines on the pr-39 plan (`gate-affected-select PLAN github.com/Beamfall/corvint
CLONE`):

```text
noncode corvint-config .corvint/documentation-paths.json
noncode corvint-config .corvint/runtime-readers.json
noncode corvint-config .corvint/shell-foreign-callers.json
noncode corvint-config .corvint/test-binary-consumers.json
noncode read-scope-declaration .corvint/test-read-scope-literals.json
noncode read-scope-declaration .corvint/test-read-scopes.json
noncode corvint-config .corvint/unbounded-reader-exclusions.json
noncode corvint-config .corvint/unbounded-readers.json
noncode never-narrowed-path Makefile: make edit: make target "capability-catalog" runs a command or make goal that does not resolve: shell:unfollowed-command:command, shell:unfollowed-command:go run
noncode never-narrowed-path interop/spec-toolchains/npm/package-lock.json
noncode never-narrowed-path interop/spec-toolchains/npm/package.json
FALLBACK corvint-config .corvint/documentation-paths.json
```

## Measurements

Binaries were built `-trimpath -buildvcs=false` from the base tree (clean, HEAD = base). All runs
went through `script/heavy.sh` on this WSL2 host.

| Measurement | 2026-11-11 (base `4c3c20333`) | This run (base `babe97abb`) |
| --- | --- | --- |
| pr-39 | UNKNOWN, FALLBACK, ci FULL; 589 dirty, 546 selected, 133 unknown | UNKNOWN, FALLBACK, ci FULL; 589 dirty, 569 selected, 133 unknown; reason `corvint-config .corvint/documentation-paths.json` |
| ci-af9d23f80 | UNKNOWN, FALLBACK, ci FULL | UNKNOWN, FALLBACK, ci FULL; 587 dirty, 569 selected, 133 unknown; same reason; misses 0 over its 3 failing packages |
| `UNINDEXED_SOURCE_PATH` on pr-39 | present (`script/check-ai-trailers.sh`) | none; the deletion selects 7 DELETED_SCRIPT_OBSERVER units and 9 more through observing helpers |
| `selection-baseline` narrow merged / floor | 36/43, floor 33 | 17/43, floor 17 (re-recorded at 14 under the non-code map in `a2a79422f`, raised to 17 in `bedc4cc5b`); `OK floor narrow>=17 misses<=0` |
| misses / test-misses | 0 / 0 | 0 / 0 over 26 failing packages of 15 failed CI runs |
| FALLBACK rows | 2 | 39 of 58; no row moved against its recorded verdict |

The 39 `FALLBACK` reasons are all non-code map FULL rules. Each count names the row's first FULL
reason only. `never-narrowed-path` 21: `Makefile` 9, `.github/` workflows and forward rows 7, and
npm manifests and lock files 5. `corvint-config` 18: `.corvint/documentation-paths.json` 8,
`unbounded-readers.json` 5, `cem-lineage.tsv` 2, `shell-foreign-callers.json` 2 and
`gated-root-locators.json` 1.

pr-39 plan unknowns by kind, with the first three paths or details. The detail is printed where
the entry has no path.

| Count | Kind | First three |
| --- | --- | --- |
| 36 | UNOWNED_DIRTY_PATH | `.corvint/documentation-paths.json`, `.corvint/runtime-readers.json`, `.corvint/shell-foreign-callers.json` |
| 17 | typescript:bare-import-undeclared | `cmd/corvint/testdata/flows/acceptance/repo/e2e/checkout.spec.ts:1`, `.../orders-api.spec.ts:1`, `.../profile.spec.ts:1` |
| 17 | typescript:dynamic-import | `conformance/harness-event-v0/capture_opencode.mjs:4`, `conformance/interactive-alpha/roadmap-proof.mjs:89`, `extensions/vscode/src/testing.ts:7` |
| 12 | typescript:runtime-flags-unresolved | `extensions/vscode/test/configuration.test.ts:2`, `error-presentation.test.ts:3`, `executable-process.test.ts:5` |
| 10 | go:unobserved-module | `interop/cem01-go/cem.go`, `cem_test.go`, `patch.go` |
| 8 | typescript:executable-config-unresolved | `cmd/corvint/testdata/runs-identity-setup-project/repo/playwright.config.ts:7`, `conformance/interactive-alpha/fixture/playwright.config.js:5`, `.../vitest.config.js:5` |
| 6 | typescript:e2e-runtime-dependency | `conformance/interactive-alpha/fixture/e2e.spec.js:1`, `internal/doccorpus/testdata/behavior-events/live-1.61.1/tests/items.spec.cjs`, `internal/jstestprovider/testdata/route-stubs/live-1.61.1-residuals/tests/residuals.spec.cjs` |
| 6 | NO_SELECTABLE_TEST | `go:.../cmd/corvint-tasks`, `go:.../internal/gitancestry/ancestrytest`, `python:script.lane-transcript-metrics` |
| 3 | typescript:test-runner-unknown | `internal/appmap/testdata/fixture/e2e/scenarios/club-with-slots.ts`, `internal/doccorpus/testdata/legacy-suite/cypress/checkout.cy.js`, `.../jasmine/summary.spec.js` |
| 1 | UNSELECTED_BINARY_CONSUMER | `tools/web-flows/test/e2e.mjs` |
| 17 | one each: cfamily (2), dotnet (3), go build-constraint and nested-module, python (3), ruby, shell foreign callers (3), swift (3) | frontier sites, no path |

Commands (rc 0 each):

- pr-39 replay: `script/heavy.sh BIN/selection-baseline --root . --changes CHANGES --corvint
  BIN/corvint --selector BIN/gate-affected-select --out pr39.json`, where CHANGES holds only
  `pr-39` and `ci-af9d23f80` filtered from `benchmarks/selection-baseline/changes.json` and BIN
  and CHANGES sit under `$(git rev-parse --absolute-git-dir)/corvint/ms1115/` (64.8 s). The
  one-change floor line `FAIL narrowed merged changes 0 below the floor 17` is expected.
- pr-39 plan: in a private clone at `99e6e6c67`, `BIN/corvint affected --base 8daec98ce434...
  --accept-stale-binary` (14.3 s), then the selector line above.
- Full replay: `script/heavy.sh make selection-baseline-check SELECTION_BASELINE_OUT=full.json`
  (run=944.1 s). `BASE=babe97abb...` was not passed: with HEAD at that base, the input preflight
  prints `skip no selection-baseline input changed since babe97abb...` and replays nothing.
  Without `BASE`, it prints `run unknown read set: event base unavailable`. The run passed the
  tool's 600 s foreground limit, was moved to the background by the agent harness and was not
  started twice.

## V1-5191 and V1-5194 coverage on the base

- V1-5191 admission and refusals: `TestDeletedShellScriptAdmission_AFPV09992` (admitted deletion;
  present on disk, extensionless, sparse checkout, unobserved directory, second claimant),
  `TestDeletedScriptObservers_AFPV09994`, `TestDeletedScriptRelationHelpers_AFPV09993`
  (`internal/liveverify/affected`), `TestDeletedScriptObserversEndToEnd_AFPV09994`
  (`internal/liveverify/affected/shell`). All pass on the base.
- V1-5194: `TestUnboundedWalkerAcceptance_V15194` (`internal/liveverify/affected/shell`) runs the
  three failing inputs (`count_test.sh` on an added `script/new.sh`, `fixme_test.sh` on a modified
  `internal/x.go`, `md_test.sh` on an added `docs/new.md`), each selected. Its controls: an edit
  of `internal/x.go` does not select `count_test.sh`, and an edit of `docs/a.md` does not select
  `md_test.sh`. These are name-only walkers on edits (AFP-V0-9963). It passes on the base, and no
  new test was needed.

## V1-4362

- AC2: `TestGateselectBodyEditSkipsAnalyzerpython_AFPV02201`
  (`internal/liveverify/affected/golang`, AFP-V0-2201, AFP-V0-2202). The fixture mirrors the
  repository shape: gateselect -> sessiontest -> cmd/corvint, and analyzerpython listing
  `go list -deps ./cmd/corvint`. A measured body-only edit selects neither analyzerpython nor the
  unrelated unit. The unit that builds and runs cmd/corvint keeps BINARY_EXEC. An added import
  and a removed import each select analyzerpython as IMPORT_GRAPH_READER through
  gateselect -> cmd/corvint, and no comparison selects it.
- Live check on the base, using committed ranges in a private clone. A body-only edit to
  `internal/gateselect/select.go` does not select `internal/closureguard` (where the guard moved,
  V1-5071). It selects `internal/analyzerpython` only as UNBOUNDED_READER, because its
  `analyzer_test.go` reads `--show-toplevel`; that is not BINARY_EXEC. An added `unicode/utf8`
  import selects `internal/closureguard` as IMPORT_GRAPH_READER.
- AC3: the full replay above is the frozen zero-miss replay: misses 0 and test-misses 0 over 58
  changes.

## Proposal for task 8 (owner decision)

pr-39 cannot lose `FALLBACK` while NCM-V1-006 makes every `.corvint/` configuration edit run
everything and the never-narrowed list covers npm manifests. Restate the task 8 and V1-5191
criterion as "no `FALLBACK` whose reason is a plan unknown or a shell admission; a non-code map
FULL reason is the accepted outcome". Under that criterion, pr-39 and ci-af9d23f80 pass on this
base. Narrowing configuration edits per file, as V1-5465 did for the read-scope declarations,
would be a separate requirement change to NCM-V1-006 with its own soundness argument.

## Rollback

Delete this entry and the test file `internal/liveverify/affected/golang/gateselect_importers_test.go`.
No product code changed.
