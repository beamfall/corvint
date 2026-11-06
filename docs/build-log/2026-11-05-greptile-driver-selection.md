# PR #98 conservative driver selection repairs

Date: 2026-11-05. Owner-directed scope: the four Greptile P1 findings on batch
PR #98, with constituent BASE `22c1f51b162e169ebd8420db0cc8d1fa0c03fb20`.
Owning intent: AZU-V0-001 and the owner-requested refinements AZU-V0-9700 through
AZU-V0-9703 in `docs/specs/affected-zero-unknown-v0.md`. No wider AZU qualification
or publication is claimed.

Missing wrapper operands now retain a computed invocation before either slice
can exhaust the word list. Package-free supported Go commands import their current
directory, including after flags and before test-binary arguments; the frozen path
encoding conservatively imports every owner for the root or an unknown directory.
Make lookups with multiple stored assignments retain a computed invocation because
the parser does not retain assignment operators. Unsupported flow-style step nodes
retain their job as a computed invocation, selecting that job and its dependents.
No JSON shape, error code, committed golden or frozen Core output changed.

Independent review of `e31e125c39994c64623d4d75d0f4716e476f4545` found that collapsing
identical make values still loses an edge for `PKG = ./left; PKG += ./left` and
`go test $(PKG)_suffix`. The correction and its regression are committed at
`702aaab0bf481214f01935fc9e5051e300cbcb3d`; the independent follow-up confirmed it
resolved with no remaining findings. A computed frontier is asserted directly in
the graph, in addition to checking selected targets and dependents: the first
witness may already be a valid dependency edge.

All four table regressions pass in the focused run (40 cases):
`TestDriverMissingWrapperOperand_AZU_V0_9700`,
`TestDriverImplicitGoPackage_AZU_V0_9701`,
`TestDriverAlternativeMakeValues_AZU_V0_9702`, and
`TestDriverFlowSteps_AZU_V0_9703`. The initial fixture run correctly reported
`NO_SELECTABLE_TEST` until the changed fixture packages had tests.

Verification of the final code revision (commands through `script/heavy.sh`,
one Go package per test command, `TMPDIR=/home/russell/lanes/b1105a-greptile.tmp`):

- Focused four-test command: rc 0.
- `HEAVY_WHOLE_REASON='greptile fix' script/heavy.sh go test -count=1 -timeout 30m ./internal/liveverify/affected`: rc 1: `TestIncrementalSelectionMeetsTheLiveBudget` (best enforced p95 212.485068 ms / probe p95 41.174734 ms, ratio 5.16 > 5.00) and `TestOwnershipAuditRatchet_AZU_V0_002` (the pre-existing native coverage README). All four regression tables pass in this run as well. The live-dirty-worktree budget test passed on this final run; the incremental-budget miss remains unresolved, with host contention observed but no isolated baseline measured.
- `script/heavy.sh go test -v -count=1 -timeout 30m -run 'TestAffected|TestCoreVerbsEmitTheFrozenProfiles' ./cmd/corvint`: rc 1 only at `TestAffectedRejectsNonRepositoryAndExtraArguments`. The required scratch directory is inside the parent `/home/russell/lanes` Git repository. The frozen test passes, and the remaining affected tests pass. Environment-only ceiling and scratch-barrier attempts did not fix this fixture assumption and were removed; no assertion was changed.
- `script/heavy.sh make windows-vet`: rc 0, toolchain `go1.27.3`.
- `make spec-requirements`, `script/check-line-citations.sh --repin`, and `make line-citations-check`: rc 0. The repinner moved no citations. `gofmt -l` is empty and `git diff --check` passes.

The earlier whole-package run retained two performance-budget failures under host
load (`TestIncrementalSelectionMeetsTheLiveBudget` and
`TestSelectionOnTheLiveDirtyWorktree`) and the ownership-ratchet failure for
`internal/nativelinecov/README.md`. The README blob already exists at BASE as
`59799db116fa8d364dff14d72f9f8697797f3d54`; its ownership declarations, ratchet and
baseline were unchanged by this repair. The BASE-built binary also lists that
README as unowned (1695 total unowned paths, against the maintained 1694-path
baseline). These failures were not repaired by widening this change's scope.

Pre-change `dogfood-change` and the bound context/query/impact recorder ran at BASE.
The query was READY without abstention, with four ranked results omitted and 16
test-path candidates withheld; the context packet omitted paired CLI tests, so the
original spec, implementation and tests were read directly. The post-edit BASE
binary's affected plan was UNKNOWN: 243 selected, 515 excluded, with build-constraint
and shell frontiers plus two unselected web-flow binary consumers. Those limits
remain unknown. Retrieval/learning changes, the full AZU-V0-009 campaign, live
external-provider qualification, all-platform runtime qualification and `make gate`
are outside this four-finding repair. No Tasks writes, push, PR creation or seal
is authorized for this constituent. The local outcome records failed verification;
structural CEM checking must not be read as a green test gate.

Host friction: the installed binary was stale, so the repository's BASE-built
private binary supplied the affected plan; commits succeeded despite a denied
`.git/packed-refs.lock` write; host admission waited for load and slots and warned
about low `/tmp` space. The review CLI could not initialize its state in a read-only
path, so a separate read-only reviewer supplied the independent review above.
No permission boundary or test assertion was relaxed. Logs, the reviewed citation
plan and complete command receipts are retained in the private scratch directory
and summarized in `/home/russell/lanes/b1105a-greptile.handback.md`. Tokens, cache
savings and comparative cost remain NOT_OBSERVED.

Rollback: revert the repair commits; no migration or wire change is needed. The
original omission/panic behavior returns, so this rollback is not a safety claim.
