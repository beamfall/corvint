# 2026-10-08: liveverify Git fixtures disable detached maintenance (V1-0662)

## Intent

PR473 head `abb23961` failed hosted shard 0 (run 36957953634, job 110685159451):
`TestDirtyNonUTF8PathIsDisclosedNotRefused` passed its assertions, then `TempDir` cleanup
failed with `.git/objects/pack` not empty. Its Git helper sets `GIT_CONFIG_GLOBAL=/dev/null`
and `GIT_CONFIG_NOSYSTEM=1`, so CI's global `maintenance.auto=false`/`gc.auto=0` never
applies, and each fixture `git commit` launches a detached `git maintenance run --auto`.

Both `observation_test.go` helpers already pass `-c maintenance.auto=false -c gc.auto=0` on
main (commit `c4f9604d`, build log 2026-10-04-cem10-pr443-ci-repairs). This change closes
the same gap in the sibling `internal/liveverify` fixtures and adds a guard, recorded as
proposed AFP-V0-039 in `docs/specs/affected-plan-v0.md`.

## Change

- `runGit` in `internal/liveverify/affected/golang/golang_test.go`, the `git` closure in
  `internal/liveverify/affected/typescript/mocha_qualification_test.go` and `runGit` in
  `internal/liveverify/pymutate/pymutate_test.go` now pass the two command-local settings.
  Config isolation, assertions and cleanup are unchanged.
- `internal/liveverify/affected/fixture_maintenance_test.go`: `unguardedFixture` lexes a Go
  source's string literals (comments do not count). A file that hides global config
  (`GIT_CONFIG_GLOBAL` or `HOME=`) and names an auto-maintenance subcommand (`am`,
  `cherry-pick`, `commit`, `fetch`, `merge`, `pull`, `rebase`, `revert`) must carry both
  `maintenance.auto=false` and `gc.auto=0`. `TestLiveVerifyGitFixturesDisableDetachedMaintenance`
  walks `internal/liveverify` and fails closed if it finds fewer than four isolating files.
  `TestUnguardedFixtureDetectsAMissingSafeguard` shows that the guard rejects a missing or
  partial safeguard, or one written only in a comment. The `affected` package is already a
  recorded unbounded reader (`.corvint/unbounded-readers.json`), so walking `..` leaves its
  selection unchanged.

## Evidence (darwin/arm64, Go 1.27.1, Git 2.54.0 Apple)

Counted with `GIT_TRACE2_EVENT=<dir>`: `child_start` events whose argv names `maintenance`.

| Fixture run | Before | After |
|---|---|---|
| `./internal/liveverify/affected/golang` (full package) | 15 launches, pass | 0 launches (45 git processes), pass |
| `./internal/liveverify/pymutate` with `CORVINT_TEST_EXTERNAL_PYTEST=1` (pytest 8.4.2) | 7 launches, pass | 0 launches (28 git processes), pass |
| observation tests (`TestPublishedObservation...`, `TestDirtyNonUTF8...`) | 3 launches with the pre-`c4f9604d` helper | 0 launches |

- Guard failing before: with the three helper edits stashed, the guard named all three files.
  With the pre-`c4f9604d` `observation_test.go` restored temporarily, it named that file too.
  With every edit in place, both guard tests pass.
- No `git maintenance` process from these runs remained afterwards (`pgrep`).
- `TestIncrementalSelectionMeetsTheLiveBudget` (`internal/liveverify/affected`) failed its 100 ms
  p95 budget at base `ecfff8e0` on this loaded shared host, before any edit. It is a timing
  budget, not a fixture, and is unrelated.

## Non-goals

- No product Git invocation changes. Product helpers such as `internal/liveverify/affected/dirty.go`
  and `internal/liveverify/mutate/export.go` keep their behavior.
- Test files outside `internal/liveverify` are not edited or guarded. A file-level scan with the
  same heuristic flags these 74 candidates. They hide global config, name a mutating subcommand,
  and contain no `maintenance.auto`/`gc.auto` literal anywhere. Each needs per-file
  confirmation, because some write `maintenance.auto` into repository config through a
  non-literal helper, or commit only under the host config:
`cmd/corvint-analyzer-go/production_spy_darwin_test.go`,
`cmd/corvint-corpus-mcp/mapplan_test.go`, `cmd/corvint-corpus-parity/main_test.go`,
`cmd/corvint-corpus-republish/main_test.go`, `cmd/corvint-go-test-provider/main_test.go`,
`cmd/corvint-postmerge-connect/main_test.go`, `cmd/corvint-postmerge-metrics/main_test.go`,
`cmd/corvint-work-queue/main_test.go`, `cmd/corvint/cem_test.go`, `cmd/corvint/delta_test.go`,
`cmd/corvint/flows_appmap_plan_test.go`, `cmd/corvint/flows_appmap_test.go`,
`cmd/corvint/flows_query_test.go`, `cmd/corvint/init_adopt_test.go`,
`cmd/corvint/record_fresh_test.go`, `conformance/go-live-test-v0/blackbox_posix_test.go`,
`conformance/release-artifact-v0/archive_gate_test.go`,
`conformance/release-artifact-v0/loose_gate_ignored_source_test.go`,
`conformance/release-artifact-v0/publication_status_test.go`, `internal/appmap/appmap_test.go`,
`internal/authoritystore/repository_test.go`, `internal/breakagemap/compile_test.go`,
`internal/cem/gitauth/canonical_create_test.go`, `internal/cem/gitauth/gitauth_test.go`,
`internal/cem/verify/candidate_test.go`, `internal/cem/verify/corpus_test.go`,
`internal/cem/verify/stable_repository_test.go`, `internal/cem/workflow/portable_test.go`,
`internal/cem/workflow/workflow_test.go`, `internal/cemcandidate/assemble_test.go`,
`internal/corpusindex/index_test.go`, `internal/corpusrepublish/build_test.go`,
`internal/corpusserve/http_test.go`, `internal/criterionexperiment/experiment_test.go`,
`internal/dashboard/repository/parse_test.go`, `internal/dashboard/roadmap/atm_test.go`,
`internal/delta/compile_test.go`, `internal/depsource/depsource_test.go`,
`internal/doccompiler/trusted_nav_test.go`, `internal/doccorpus/corpus_test.go`,
`internal/dogfoodflow/check_test.go`, `internal/flowdocs/flowdocs_test.go`,
`internal/frontierrepo/frontierrepo_test.go`, `internal/frontierrepo/tcq_conformance_test.go`,
`internal/genesis/repository_test.go`, `internal/gitnotes/gitnotes_test.go`,
`internal/intake/intake_test.go`, `internal/lrfrepo/ocm_prepare_test.go`,
`internal/postmergeconnector/conformance_test.go`, `internal/postmergehost/launcher_test.go`,
`internal/postmergemetrics/git_test.go`, `internal/postmergeworkflow/replay_test.go`,
`internal/receiptbundle/receiptbundle_test.go`, `internal/releasecandidate/readiness_test.go`,
`internal/releasecandidate/verify_test.go`, `internal/releasecandidate/version_test.go`,
`internal/stepverify/step_test.go`, `internal/taskman/capture_unix_test.go`,
`internal/tasks/cli/scope_test.go`, `internal/tasks/scopes/derive_test.go`,
`internal/tasks/store/guarded_inventory_test.go`, `internal/tasks/store/lease_gate_test.go`,
`internal/tasks/store/lease_test.go`, `internal/tasks/store/program_multirepo_test.go`,
`internal/tasks/store/store_test.go`, `internal/tasks/store/workflow_context_test.go`,
`internal/tasks/store/writer_hold_profile_test.go`, `internal/witness/witness_test.go`,
`interop/cem01-go/manifest_test.go`, `interop/cem01-go/stable_repository_test.go`,
`tools/corvint-pr-tests/clone_test.go`, `tools/corvint-pr-tests/main_test.go`,
`tools/docs-ci-plan/main_test.go`, `tools/unbounded-readers/main_test.go`

## Failure modes

- A new `internal/liveverify` fixture that isolates config and commits without the settings
  fails the guard.
- The guard is a file-level literal check. It can be satisfied by a file whose literals appear
  in a different helper from the one that commits. It cannot see a mutating subcommand assembled
  at run time.
- A walk that reaches no fixtures fails closed through the isolating-file floor.

## Rollback

Delete `internal/liveverify/affected/fixture_maintenance_test.go`, revert the three helper
edits and AFP-V0-039. The detached launches return under isolated config.

## NOT_RUN

- Hosted Linux Git 2.55 cleanup qualification. The race was never reproduced locally; that
  needs this branch's hosted CI.
- `TestMochaActualSelectionQualification` requires `CORVINT_MOCHA_NODE/CLI/EVIDENCE`, and no
  Mocha 12.0.3 CLI is installed locally. The helper edit was compiled and guarded, not executed.
- `make gate` and the full `./...` run are excluded by lane policy.
