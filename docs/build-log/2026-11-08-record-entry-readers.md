# Added record entry readers (V1-4933 / V1-4544)

The owner requested addition-only narrowing for sealed CEM and build-log readers,
retaining directory enumerators and failing closed on unknown evidence. This
experimental slice implements AFP-V0-3420 through AFP-V0-3425. It does not claim
that every test selected for these additions has now been eliminated.

## Cause and change

AFP-V0-2743 narrowed the *location* to which a literal resolves. A literal naming
the record directory still matched every entry; it did not distinguish repository
enumeration from fixed sibling names, expected strings, or fixture repositories.
Declared directory scopes also admitted every new entry. The existing record
reader ratchet covered Go units but omitted shell tests.

The new `.corvint/record-entry-read-scopes.json` binds an audited exact-file or
directory scope to each unit's complete source/test bytes and named data inputs.
Only observed Git `A` entries outside a current scope lose literal, glob, or
declared-reader selection. Modified/deleted entries, untracked paths, snapshots,
path-only probes, malformed observations, unknown declarations and stale source
bindings retain the existing behavior. The gate selector re-observes the same
evidence. The existing ratchet now checks shell units and rejects stale bindings.

The source inventory and audit reasons are retained in the declaration. Two real
enumerators must remain selected:

- `internal/cem/corpusbinding/binding_test.go:68` calls `os.ReadDir(changes)`;
  its loop reads every `.cem.json` at line 76 and verifies its binding. Additional
  history fixture controls enumerate at
  `internal/cem/corpusbinding/history_fixture_controls_test.go:112` and `:174`.
- `internal/cem/wire/corpus_test.go:20` calls `os.ReadDir(dir)`; its loop reads
  every CEM at line 32. `firstSealed` also enumerates at line 111.

`cemchain` constructs isolated Git fixtures; Tasks CLI tests use fixture paths or
expected names; the audited shell tests create scratch repositories or compare
fixed text. Use-case conformance follows fixed ledger/receipt filenames, so those
data files also participate in its source binding. The documentation-paths
generator enumerates tracked files but filters to top-level `docs/*.md`, then
appends constant directory names: adding a build-log entry cannot change that list.

Rollback: remove the addition-only scope admission from the planner and gate
selector; existing conservative reader selection returns. Removing the declaration
alone also disables narrowing. The ratchet may be reverted independently.

## Verification

Go toolchain: `go1.27.3`. No full repository gate was requested or run. No Tasks
commands, publication, or subagents were used in this continuation.

- PASS: `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'TestAddedRecordEntry|RecordEntry|ReadScope' ./internal/liveverify/affected/`
  (fresh continuation run, 32.585 seconds).
- PASS: `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m -run 'TestAddedRecordEntry|TestRecordEntryShellReaderRatchet|AFPV274[5-8]' ./internal/gateselect/ ./tools/unbounded-readers/`
  (fresh run on the configured WSL runner).
- PASS retained from the preceding run, unchanged inputs:
  `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/test-read-scopes-check/`
  (59.322 seconds) and
  `GOTOOLCHAIN=local go vet ./internal/liveverify/affected/ ./cmd/corvint/`.
- PASS: `make spec-requirements` and `make spec-requirements-check` in the
  detached candidate worktree. The generated TSV exactly matches this worktree.
  Staging in the task worktree was refused by the filesystem before generation;
  the detached candidate contains the same staged/committed spec bytes.
- PASS: `GOTOOLCHAIN=local go build -o ../corvint-4933 ./cmd/corvint` and both
  measurement commands below.
- PASS: `make changed-checks BASE=447bdcbd5963683f5f2b4934a45b8f48fc6742a3`
  against the staged candidate and build log in the probe worktree. The existing
  stale unbounded-reader baseline entries are reported as advisory; no new
  unscoped record reader or stale addition-only binding fails the gate.

Fail-before proof: the identical
`TestRecordEntryShellReaderRatchet_AFPV3424` was run with the production changes
temporarily reverted. It reached its initialized fixture and failed because the
old ratchet admitted `new_test.sh` (`exit=0`, `ok=true`, no readers), instead of
rejecting the undeclared `docs/build-log/*.md` reader. Exit status was 1. Restoring
the implementation makes that test pass. The retained log is
`v14933-fail-before.log` in the task scratch directory.

## Before/after CLI measurement

The pinned task base is `447bdcbd5963683f5f2b4934a45b8f48fc6742a3`, the original
`origin/main`; the remote-tracking reference later advanced and was not used to
rebase this task. Both executables were built with `GOTOOLCHAIN=local go build`.
The baseline binary's VCS metadata is clean at that base; the candidate binary is
built from the preserved edits. Its embedded revision remains the base and emits
`self-binary-stale` against the detached candidate. This diagnostic does not change
selection; all 18 candidate files were byte-compared with the task worktree before
the probe was committed.

Both probes add identical bytes at exactly these paths:

```text
.corvint/changes/1111111111111111111111111111111111111111.cem.json
docs/build-log/2026-11-09-probe.md
```

The CEM is a copy of an existing entry, not a valid binding for its new basename;
the probe measures selection and does not assert that corpus verification passes.
Each executable ran `affected --base HEAD~1 --format json` in a detached worktree
after committing those two additions.

| Measurement | Base | Probe commit | `summary.line` |
| --- | --- | --- | --- |
| Before | `447bdcbd5963683f5f2b4934a45b8f48fc6742a3` | `244bf542e4d5508b649709080a9aa48be3ff5ee7` | `scope=UNKNOWN dirty=2 selected=240 excluded=659 unknown=5 LANGUAGE_FRONTIER=5` |
| After | `894abcf2dd2ef7ea1fde4606a4132b44aaa9443b` | `6aaa1a30eb848db8536360e9dfcbed9d94f724df` | `scope=UNKNOWN dirty=2 selected=230 excluded=668 unknown=5 LANGUAGE_FRONTIER=5` |

The literal/declared witnesses for the audited non-enumerators disappear. Overall
selection drops by ten units. This is a bounded gain, not complete elimination:
`internal/cemchain` remains an `UNBOUNDED_READER` because of its root-reaching
literal; `batch-precheck`, `dogfood-cache`, `gen-documentation-paths`, `lane-scripts`
and `requirement-id-collisions` shell tests remain through `DEPENDENCY_PATH`.
Those independent causes are deliberately unchanged by AFP-V0-3422. Five
`LANGUAGE_FRONTIER` unknowns remain. No unsound exclusion is inferred from them.

Raw receipts `v14933-before.json` and `v14933-after.json`, the fail-before log,
and continuation command logs are retained in the task scratch directory.
The complete selected-unit inventory follows; `both` means selected before and
after, while `before` means removed by the measured candidate.

## Remaining delivery limits

Initial staging attempts could not create the task worktree's Git `index.lock`
(`Operation not permitted`). The final staging and commit retry succeeded, and
the detached probe worktree was removed with `git worktree remove --force`.
The earlier `make dogfood-change` could not create its Git metadata directory;
that missing evidence is retained as `NOT_PRODUCED`. No seal or native ticket
completion is claimed. The continuation uses direct diff/source review;
independent subagents were explicitly disallowed.

Installed Core is `1.0.0-rc.3.dev`, build 137 at the pinned base. The official
update check returned freshness `UNKNOWN`: its smoke process could not prove
process-group cleanup in the sandbox. No update was installed and no freshness
claim is made. Tasks checks and automatic issue filing are omitted under the
owner's explicit prohibition on `corvint-tasks`.

## Complete selection inventory

| Unit | Selected |
| --- | --- |
| `driver:ci:.github/workflows/cache-prune.yml#cache-prune` | both |
| `driver:ci:.github/workflows/cem-release-gate.yml#cem-release-gate` | both |
| `driver:ci:.github/workflows/cem-release-gate.yml#cem-release-readiness` | both |
| `driver:ci:.github/workflows/cem-release-gate.yml#cem-release-record` | both |
| `driver:ci:.github/workflows/ci-control-plane.yml#control-plane` | both |
| `driver:ci:.github/workflows/ci-control-plane.yml#merge-group` | both |
| `driver:ci:.github/workflows/ci-route-health.yml#routes` | both |
| `driver:ci:.github/workflows/ci.yml#artifact-integrity` | both |
| `driver:ci:.github/workflows/ci.yml#artifact-integrity-record` | both |
| `driver:ci:.github/workflows/ci.yml#cce-live` | both |
| `driver:ci:.github/workflows/ci.yml#ci-narrowed-tree-record` | both |
| `driver:ci:.github/workflows/ci.yml#ci-shard-cost-drift` | both |
| `driver:ci:.github/workflows/ci.yml#ci-tested-tree-record` | both |
| `driver:ci:.github/workflows/ci.yml#doc-gates` | both |
| `driver:ci:.github/workflows/ci.yml#doc-gates-record` | both |
| `driver:ci:.github/workflows/ci.yml#docs-plan` | both |
| `driver:ci:.github/workflows/ci.yml#go-interop` | both |
| `driver:ci:.github/workflows/ci.yml#go-live` | both |
| `driver:ci:.github/workflows/ci.yml#go-product` | both |
| `driver:ci:.github/workflows/ci.yml#go-product-shard` | both |
| `driver:ci:.github/workflows/ci.yml#go-static` | both |
| `driver:ci:.github/workflows/ci.yml#host-adapters` | both |
| `driver:ci:.github/workflows/ci.yml#local-authority-deps` | both |
| `driver:ci:.github/workflows/ci.yml#shard-plan` | both |
| `driver:ci:.github/workflows/ci.yml#shell-test-selected` | both |
| `driver:ci:.github/workflows/ci.yml#static-reuse` | both |
| `driver:ci:.github/workflows/ci.yml#web-flows` | both |
| `driver:ci:.github/workflows/ci.yml#web-flows-plan` | both |
| `driver:ci:.github/workflows/go-macos.yml#docs-plan` | both |
| `driver:ci:.github/workflows/go-macos.yml#go-macos` | both |
| `driver:ci:.github/workflows/go-macos.yml#go-macos-shard` | both |
| `driver:ci:.github/workflows/greptile-gate.yml#gate` | both |
| `driver:ci:.github/workflows/greptile-gate.yml#resolve` | both |
| `driver:ci:.github/workflows/linecov-index.yml#collect` | both |
| `driver:ci:.github/workflows/linecov-index.yml#merge` | both |
| `driver:ci:.github/workflows/perf-budget.yml#perf-budget` | both |
| `driver:ci:.github/workflows/pr-tests-qualification.yml#freeze` | both |
| `driver:ci:.github/workflows/pr-tests-qualification.yml#qualify` | both |
| `driver:ci:.github/workflows/pr-tests-qualification.yml#row` | both |
| `driver:ci:.github/workflows/spec-toolchain-conformance.yml#spec-toolchain-conformance` | both |
| `driver:make:Makefile#ai-coauthor-trailers-check` | both |
| `driver:make:Makefile#analyzer-schema-check` | both |
| `driver:make:Makefile#analyzer-schema-test` | both |
| `driver:make:Makefile#bash32-check` | both |
| `driver:make:Makefile#batch-assemble` | both |
| `driver:make:Makefile#batch-precheck` | both |
| `driver:make:Makefile#batch-precheck-test` | both |
| `driver:make:Makefile#batch-push-check` | both |
| `driver:make:Makefile#batch-regen` | both |
| `driver:make:Makefile#batch-review-packet` | both |
| `driver:make:Makefile#binary-blobs-check` | both |
| `driver:make:Makefile#build` | both |
| `driver:make:Makefile#capabilities-check` | both |
| `driver:make:Makefile#capability-catalog` | both |
| `driver:make:Makefile#capability-catalog-check` | both |
| `driver:make:Makefile#cce-live` | both |
| `driver:make:Makefile#cem-base-retention-check` | both |
| `driver:make:Makefile#cem-base-retention-test` | both |
| `driver:make:Makefile#cem-release-gate` | both |
| `driver:make:Makefile#cem-release-suites` | both |
| `driver:make:Makefile#cem-stable-regen` | both |
| `driver:make:Makefile#cem-stable-regen-any-of` | both |
| `driver:make:Makefile#cem-stable-regen-capture` | both |
| `driver:make:Makefile#cem-stable-regen-check` | both |
| `driver:make:Makefile#cem-stable-regen-replace` | both |
| `driver:make:Makefile#cem-stable-regen-widen` | both |
| `driver:make:Makefile#changed-checks` | both |
| `driver:make:Makefile#ci-least-privilege-check` | both |
| `driver:make:Makefile#codex-review-test` | both |
| `driver:make:Makefile#companion-release-gate` | both |
| `driver:make:Makefile#conflict-markers-check` | both |
| `driver:make:Makefile#core-n1-replay` | both |
| `driver:make:Makefile#cross-vet` | both |
| `driver:make:Makefile#decision-numbers-check` | both |
| `driver:make:Makefile#declaration-compat-check` | both |
| `driver:make:Makefile#diagnostic-coverage-check` | both |
| `driver:make:Makefile#doc-gates` | both |
| `driver:make:Makefile#documentation-paths` | both |
| `driver:make:Makefile#documentation-paths-check` | both |
| `driver:make:Makefile#documentation-paths-test` | both |
| `driver:make:Makefile#dogfood-batch-check` | both |
| `driver:make:Makefile#dogfood-bin-reuse-test` | before |
| `driver:make:Makefile#dogfood-bind-range` | both |
| `driver:make:Makefile#dogfood-bind-range-test` | both |
| `driver:make:Makefile#dogfood-cache-test` | both |
| `driver:make:Makefile#dogfood-change` | both |
| `driver:make:Makefile#dogfood-change-runs` | both |
| `driver:make:Makefile#dogfood-change-test` | both |
| `driver:make:Makefile#dogfood-check` | both |
| `driver:make:Makefile#dogfood-closeout` | both |
| `driver:make:Makefile#dogfood-closeout-precheck` | both |
| `driver:make:Makefile#dogfood-closeout-repin-test` | both |
| `driver:make:Makefile#dogfood-finish` | both |
| `driver:make:Makefile#dogfood-inputs-check` | both |
| `driver:make:Makefile#dogfood-prechange` | both |
| `driver:make:Makefile#dogfood-retire-traces` | both |
| `driver:make:Makefile#dogfood-seal` | both |
| `driver:make:Makefile#dr-numbers-check` | both |
| `driver:make:Makefile#eol-policy-check` | both |
| `driver:make:Makefile#error-code-ownership-check` | both |
| `driver:make:Makefile#fixed-tmp-paths-check` | both |
| `driver:make:Makefile#flow-guide-check` | both |
| `driver:make:Makefile#gate` | both |
| `driver:make:Makefile#gate-affected` | both |
| `driver:make:Makefile#gate-receipt-clear` | both |
| `driver:make:Makefile#go-archive-gate` | both |
| `driver:make:Makefile#go-cache-trim` | both |
| `driver:make:Makefile#go-format-check` | both |
| `driver:make:Makefile#go-test` | both |
| `driver:make:Makefile#go-version` | both |
| `driver:make:Makefile#go-version-pins-check` | both |
| `driver:make:Makefile#go-vet` | both |
| `driver:make:Makefile#goos-filenames-check` | both |
| `driver:make:Makefile#help` | both |
| `driver:make:Makefile#hlq-local` | both |
| `driver:make:Makefile#host-adapter-node-test` | both |
| `driver:make:Makefile#host-adapter-test` | both |
| `driver:make:Makefile#host-package-version-bump` | both |
| `driver:make:Makefile#host-package-version-verify` | both |
| `driver:make:Makefile#host-package-versions-check` | both |
| `driver:make:Makefile#hostile-regressions-check` | both |
| `driver:make:Makefile#import-direction-check` | both |
| `driver:make:Makefile#install-lifecycle-test` | both |
| `driver:make:Makefile#install-local` | both |
| `driver:make:Makefile#instruction-conflicts` | both |
| `driver:make:Makefile#interop-gate` | both |
| `driver:make:Makefile#lane-dispatch-check` | both |
| `driver:make:Makefile#lane-screen-test` | both |
| `driver:make:Makefile#lane-scripts-test` | both |
| `driver:make:Makefile#ledger/go-test` | both |
| `driver:make:Makefile#legal-file-pins-check` | both |
| `driver:make:Makefile#license-headers-check` | both |
| `driver:make:Makefile#license-headers-test` | both |
| `driver:make:Makefile#line-citations-check` | both |
| `driver:make:Makefile#line-citations-selects` | both |
| `driver:make:Makefile#live-test` | both |
| `driver:make:Makefile#long-lines-check` | both |
| `driver:make:Makefile#mcp-tools-list-goldens` | both |
| `driver:make:Makefile#merge-drivers` | both |
| `driver:make:Makefile#mkfifo-build-tags-check` | both |
| `driver:make:Makefile#perf-budget` | both |
| `driver:make:Makefile#pi-protected-build` | both |
| `driver:make:Makefile#pi-protected-test` | both |
| `driver:make:Makefile#plugin-test` | both |
| `driver:make:Makefile#public-release-check` | both |
| `driver:make:Makefile#py-signal-lint` | both |
| `driver:make:Makefile#repin` | both |
| `driver:make:Makefile#requirement-definitions-check` | both |
| `driver:make:Makefile#requirement-id-collisions` | both |
| `driver:make:Makefile#requirement-ids-free` | both |
| `driver:make:Makefile#requirement-ids-next` | both |
| `driver:make:Makefile#requirement-ids-release` | both |
| `driver:make:Makefile#requirement-ids-reserve` | both |
| `driver:make:Makefile#requirement-next-id` | both |
| `driver:make:Makefile#review-head-check-test` | both |
| `driver:make:Makefile#runtime-environment-test` | both |
| `driver:make:Makefile#scratch-sweep` | both |
| `driver:make:Makefile#selection-baseline-check` | both |
| `driver:make:Makefile#selection-contract-check` | both |
| `driver:make:Makefile#shell-foreign-callers-check` | both |
| `driver:make:Makefile#shell-test` | both |
| `driver:make:Makefile#shell-test-affected` | both |
| `driver:make:Makefile#shell-test-register` | both |
| `driver:make:Makefile#shell-test-registration-check` | both |
| `driver:make:Makefile#spec-acceptance-audit` | both |
| `driver:make:Makefile#spec-batch-capacity-check` | both |
| `driver:make:Makefile#spec-capacity-check` | both |
| `driver:make:Makefile#spec-claims-check` | both |
| `driver:make:Makefile#spec-readme-check` | both |
| `driver:make:Makefile#spec-requirements` | both |
| `driver:make:Makefile#spec-requirements-check` | both |
| `driver:make:Makefile#spec-requirements-resolve` | both |
| `driver:make:Makefile#spec-requirements-test` | both |
| `driver:make:Makefile#spec-status-check` | both |
| `driver:make:Makefile#spec-toolchain-conformance` | both |
| `driver:make:Makefile#sql-native-ratchets` | both |
| `driver:make:Makefile#stray-backups-check` | both |
| `driver:make:Makefile#tasks-build` | both |
| `driver:make:Makefile#tasks-profiles-check` | both |
| `driver:make:Makefile#tasks-reconcile` | both |
| `driver:make:Makefile#tasks-test` | both |
| `driver:make:Makefile#test-commands-check` | both |
| `driver:make:Makefile#test-exec-writes-check` | both |
| `driver:make:Makefile#test-read-scopes-check` | both |
| `driver:make:Makefile#test-script-refs-check` | both |
| `driver:make:Makefile#test-shard` | both |
| `driver:make:Makefile#traceability-tests-check` | both |
| `driver:make:Makefile#tracked-ignored-check` | both |
| `driver:make:Makefile#unbounded-readers-check` | both |
| `driver:make:Makefile#use-case-receipts-check` | both |
| `driver:make:Makefile#walk-skipdir-check` | both |
| `driver:make:Makefile#windows-main-check` | both |
| `driver:make:Makefile#windows-vet` | both |
| `go:github.com/Beamfall/corvint/cmd/corvint` | both |
| `go:github.com/Beamfall/corvint/conformance/dashboard-snapshot-v0` | both |
| `go:github.com/Beamfall/corvint/conformance/release-artifact-v0` | both |
| `go:github.com/Beamfall/corvint/conformance/use-cases-v0` | before |
| `go:github.com/Beamfall/corvint/internal/analyzerhtmlcss` | both |
| `go:github.com/Beamfall/corvint/internal/analyzerpython` | both |
| `go:github.com/Beamfall/corvint/internal/cem/corpusbinding` | both |
| `go:github.com/Beamfall/corvint/internal/cem/wire` | both |
| `go:github.com/Beamfall/corvint/internal/cemchain` | both |
| `go:github.com/Beamfall/corvint/internal/companionrelease/currentproof` | both |
| `go:github.com/Beamfall/corvint/internal/console` | both |
| `go:github.com/Beamfall/corvint/internal/contextindex` | both |
| `go:github.com/Beamfall/corvint/internal/dashboard/repository` | both |
| `go:github.com/Beamfall/corvint/internal/doccorpus/selfcorpus` | both |
| `go:github.com/Beamfall/corvint/internal/liveverify/affected` | both |
| `go:github.com/Beamfall/corvint/internal/liveverify/affected/golang` | both |
| `go:github.com/Beamfall/corvint/internal/opencodequalification` | both |
| `go:github.com/Beamfall/corvint/internal/postmergehost` | both |
| `go:github.com/Beamfall/corvint/internal/postmergeworkflow` | both |
| `go:github.com/Beamfall/corvint/internal/tasks` | both |
| `go:github.com/Beamfall/corvint/internal/tasks/cli` | before |
| `go:github.com/Beamfall/corvint/tools/capabilities-check` | both |
| `go:github.com/Beamfall/corvint/tools/ci-reuse-plan` | both |
| `go:github.com/Beamfall/corvint/tools/test-exec-writes-check` | both |
| `go:github.com/Beamfall/corvint/tools/test-read-scopes-check` | both |
| `shell:script/analyzer-schema_test.sh` | both |
| `shell:script/batch-precheck_test.sh` | both |
| `shell:script/check-cem-base-retained_test.sh` | both |
| `shell:script/check-license-headers_test.sh` | both |
| `shell:script/check-line-citations_test.sh` | before |
| `shell:script/codex-review_test.sh` | both |
| `shell:script/dogfood-bin-reuse_test.sh` | before |
| `shell:script/dogfood-cache_test.sh` | both |
| `shell:script/dogfood-closeout-repin_test.sh` | both |
| `shell:script/gen-documentation-paths_test.sh` | both |
| `shell:script/gen-spec-requirements_test.sh` | before |
| `shell:script/lane-host-tools_test.sh` | before |
| `shell:script/lane-scripts_test.sh` | both |
| `shell:script/merge-drivers_test.sh` | before |
| `shell:script/no-python-runtime-dependency_test.sh` | before |
| `shell:script/repin-use-case-receipts_test.sh` | before |
| `shell:script/requirement-id-collisions_test.sh` | both |
| `shell:script/requirement-id-reservations_test.sh` | both |
| `shell:script/review-head-check_test.sh` | both |
| `shell:script/runtime-environment_test.sh` | both |
| `typescript:playwright:conformance/interactive-alpha/fixture/e2e.spec.js` | both |
| `typescript:playwright:tools/web-flows/test/fixture/flows.spec.ts` | both |

Receipt SHA-256:

```text
0a46806d9ed19300f07ad128b888009aa01d43c5cd33a2134a9be75111587177  v14933-before.json
2ea118610a9eefadb64b623f00dc24688bb065799af719518804117750c0e114  v14933-after.json
bc27fd6a01c2672abd941c12809398b5b510843074b0f9031613e7ab74d173c2  v14933-fail-before.log
```
