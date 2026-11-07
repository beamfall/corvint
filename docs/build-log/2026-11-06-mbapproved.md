<!-- SPDX-License-Identifier: Apache-2.0 -->
# mbapproved: batch of approved, unmerged 11-04/05 lanes

The owner asked that every finished fix that never merged be merged. Lane mbapproved merges the
approved lanes below into `claude/mbapproved` from `origin/main`
`4052b8540b5fff818a819db37b55371ea5428c2c`, one `git merge --no-ff` per source, in the charter's
order. Main is newer than every source (517 to 695 commits), so each conflict keeps main's
surrounding code and re-applies the lane's behaviour, and parts main already holds by another route
are kept as main's and named here. Every merge keeps the lane's own commits, bind and seal in the
ancestry; one whole-range CEM binds the batch. Requirement IDs were checked against main and each
other: no lane ID collides, so the reserved 9980-9999 block was not used.

## Sources

| # | Source and head | Result | Conflicted paths (resolution) | Tickets |
|---|---|---|---|---|
| 1 | `claude/clicontract` `369139b92` | merged `58e27cfe1` | `cmd/corvint-pulse/main.go`, `cmd/corvint-release-gate/main.go` (modify/delete: main's V1-4747 AC3 retirement kept); `conformance/cli-errors-v0/contract_test.go`, `cmd/corvint/command_tree.go`, `internal/capcatalog/CLI-RATCHET.tsv`, `tools/capabilities-check/trees.go` (lane, minus the two retired binaries: Pulse contract row and `TestCLIProfile9301NativePulsePreamble` dropped, `internal/cliapp` preamble tests retarget `corvint-gateway`, `corvint-release-gate.json` tree removed); `Makefile` (both targets); `catalog.json`, `docs/CAPABILITIES.md`, `llms.txt` (regenerated); fixture identity tests, use-case receipts, citations and `docs/specs/README.md` (main, repinned or regenerated) | V1-2685, V1-3736, V1-3738 |
| 2 | `claude/fixtureident` `c316627cc` | merged `96579bb44` | `cmd/corvint/context_summary_test.go` (lane); `cmd/corvint/flows_test.go`, `flows_appmap_from_test.go` (main moved them to `internal/command/flows`, V1-4756); `internal/frontiernextrepo/repository_test.go` (main removed the Git fixture). The moved tests get `internal/command/flows/git_fixture_test.go`, and `cmd/corvint/flows_fixtures_test.go` uses the hermetic helpers | V1-4719 |
| 3 | `claude/floor` `96e89b950` | merged `63be610c9` | `.corvint/test-read-scopes.json`, `.corvint/unbounded-readers.json` (lane's release-artifact scope; main's `conformance/ocm-v0` unbounded entry kept; retired pulse scope not re-added); `affected-plan-selection-v0.md` (both lanes' notes, digest claim shortened for SRG-V0-040); `internal/companionrelease/tasks_archive_test.go` (both); generated spec files regenerated. Main's new `testGoSlices` callers pass the typed-reach executor | V1-0643, V1-2744 |
| 4 | `claude/azu-site` `ed5bf85d7` | **skipped** (merge resolved, tested, reset) | see below | V1-4686, V1-4691 (charter listed V1-4684, V1-4692; main's AZU trace credits those to lane azu-new) |
| 5 | `claude/reuse` `ee0a175ec` | merged `24309df9d` | `.github/workflows/ci.yml` and `tools/ci-reuse-plan/workflow_test.go` (artifact-integrity needs `[static-reuse, cadence-plan, job-plan]`, job-plan SKIP before main's DEFER clause, which `cadence_test.go` needs last); `cmd/corvint/affected.go`, `help.go` (main's `--ownership-audit` and the lane's `jobs`); `docs/CAPABILITIES.md`; generated and citation files | V1-1097, V1-1099 |
| 5c | worktree c2 `a2d6f67c1` | skipped | an earlier snapshot of reuse's tickets: all 18 paths are in reuse, whose later rounds revised every unmatched line | (V1-1097) |
| 6 | `claude/covlive` `4ec75e916` | merged `412b6dc12`, no conflict | none | V1-4369, V1-4370, V1-4371, V1-4375, V1-4376 |
| 6c | `claude/covcoll` `92a0ef066` | carried by 6 | an ancestor of covlive; its parts already on main merge unchanged | V1-4664 and the five above |
| 7 | `claude/bugfix` `c8dcf89d8` | merged `76f572f33` (the half main lacked) | Pi pins (main's 0.3.15); `internal/browsercov/hook.cjs`, `hook_fixture.cjs` (lane: closed-transport guard and regression); `integrations/pi-protected/dependencies.test.mjs` (main: same path fix plus bun skip); `integrations/pi/host-prerequisites.test.mjs` (both); `integrations/pi/README.md` (both); `.corvint/shell-foreign-callers.json` (main); `script/batch-precheck*.sh`, `Makefile ci-batch-cem-check`, `script/ci-batch-cem-check.sh`, ASD-V0-036 (main: batch-1105a dropped this mirror for ASD-V0-2472); two add/add build logs (main's batch versions); duplicate PWV-V0-8300..8302 definitions (lane's later text, once) | V1-4663, V1-4664, V1-4648, V1-2651, V1-4431, V1-3624, V1-0486, V1-4662, V1-4650 |
| 8 | `claude/sbgate` `c28f7da26` | merged `67e05d909` | `Makefile` `selection-baseline-check` (main's cadence DEFER exit, then the lane's skip decision) | V1-4724, V1-4329 |
| 9 | `claude/nightly-fix` `f80f64ad7` | merged `430dc9d2e` (partial) | main fixed most of run 41849536472 independently (CSD-V0-7513/7516/7517/7518, IDX-SNAP-V0-7560, ASD-V0-2474, the SRG range cell, the cem2proto retirement): those files and their amendments stay main's. Delivered: V1-4552 lifecycle launch, the V1-4551 alias regression, the V1-4553 bash32 PNG test | V1-4550, V1-4551, V1-4552, V1-4553 |
| 10a | `claude/placement` `1c3ebf4f8` | merged `13b00eaea` | `docs/specs/README.md` (regenerated) | V1-4355 |
| 10b | `claude/ranking` `48df9e832` | merged `77db30b17` (partial) | V1-1323 and V1-3758 parts superseded (below); fixture tests and citations (main) | V1-3710 (also V1-0205, V1-1670, V1-1672, V1-3775 verification-only) |

### Reviews that were not APPROVE

Placement: the review diffed `28a3ff86..HEAD` across the lane's merge of mainline `c5379b4cf`, so
both findings name mainline code, not the lane's change (`c5379b4cf..1c3ebf4f8` touches only
`tools/ci-test-slices`, CIN-V0-7620, help text and a build log). P2 (ci-reuse-plan admission) is
resolved on main by `0c8ae2da9` (CIN-V0-7600); P3's unset `EVENT_NAME` no longer exists in main's
`go-product` job. No fix commit was needed.

Ranking: the P2 finding (`.zsh`, `.ksh`, `.bats` missing from the conflict-marker gate) was fixed on
the lane by `2d0b2dfff`, after the review. Main has since delivered V1-1323 as the Go
`conflict-markers-check` (SRG-V0-8202, lane mixbugs), and V1-3758 as CEM-V1-8201's archive
locators, which require no private refs. The lane's script gate, its Make targets (which would
redefine main's target), DCI-V0-7902, batch-precheck trusted-binding parsing,
`refs/archive/cem-bases` lineage labels, CEM-V1-7901 and the unsealed shared CEM are not taken.

### azu-site, skipped

The merge resolved cleanly (main's precomputed graph tokens replace the lane's `readerTokens`;
the `FrontierDirection`/`PackageRunner` interfaces sit beside main's `AncestorBoundFrontiers`). On
current main, its AZU-V0-003 outbound closure and AZU-V0-008 package fallback break three main
requirements added after the lane's base. All three pass on main and on this batch without
azu-site:

- `TestPrecisionReplay_AFPV09003`: a `script/heavy.sh`-only change selects 259 language units against the
  bound below 86 (main: 80), because the closure selects 46 TypeScript frontier sites and walks
  from them.
- `TestRepositoryInboundCallers_AZU_V0_8901`: the wider reach makes 17 shell units bear
  `shell:foreign-invocation-unmodelled` for `internal/gateselect/allowlist.go` and `script/heavy.sh`.
- `TestRunnerFrontierLocality_AFPV02600`: `typescript:test-runner-unknown` entries become `NO_RUNNER`.

Closure-by-selection and the precision bound conflict by construction, so this needs an owner
decision and a rework of the lane on current main. Its tickets stay open.

## Batch-own commits

`973dc9bc9`, `16cd3c374`, `aa98b6af4`: AZU-V0-002 owners for the merged fixture trees
(typedreach testdata, `cmd/corvint-web-flows/help.json`, `.corvint/ci-job-inputs.json`, native
coverage and placement fixtures). `30960d338`: mechanical repin (26 citations, three use-case
receipts; the AUTOMATION `affected` exits citation re-anchored to `cmd/corvint/affected.go:823-889`,
whose `--base` exits are unchanged). `972ed47df`: doc gates the composition broke: a
ci-least-privilege exception for reuse's disabled `test-reuse-plan` job (`actions: read`, for
ci-control-plane review), main's `heavy.sh` foreign-caller reason (the lane's longer text exceeded
1024 bytes), and `script/go-archive-gate` in the release-artifact read scope. `aa98b6af4` also drops
the retired `corvint-release-gate` from a `tools/cli-discovery-gen` test. `a76ccb924`: a Bash 3.2
array guard in reuse's `heavy.sh` addition. `157a0c8d0`: floor's dry-run tests request
`--format json`, because main made a local dry run print text.

The bugfix lane's `.github/workflows/pi-host.yml` is included. Batch-1105a deferred it to a
separate control-plane change, which never landed, although main's Pi README says it did. This
batch already needs ci-control-plane review for reuse's `ci.yml` change.

## Verification

`corvint affected --base 4052b854...` returned 0: scope UNKNOWN, 546 selected, 110 unknown.
`corvint test --base ... --dry-run` returned 3: scope FULL, because covlive's
`internal/nativelinecov/testdata/live/swift/Package.swift` is an unindexed source path. As chartered,
the focused set ran instead, one package per `script/heavy.sh` command, and passed:
`internal/{cliapp,cliprofile,clierror,capcatalog,clitree,testreuse,jobinputs,nativelinecov,browsercov,linecov,sessiontest,companionrelease,gateselect,command/flows}`,
`internal/liveverify/affected` (whole) and its `golang`, `golang/typedreach`, `typescript`,
`dotnet`, `python`, `ruby` and `cfamily` packages, `conformance/cli-errors-v0`,
`tools/{capabilities-check,cli-discovery-gen,ci-reuse-plan,ci-test-slices,selection-baseline,typed-reach-measure,declaration-compat-check,test-read-scopes-check,linecov}`,
the `internal/contextindex` CRK-V0-7900 tests, the `interop/cem01-go` linked-worktree tests,
and the merged lanes' `cmd/corvint` tests with the conflict-touched ones (`-run`). Also passing:
the Pi node tests, `script/{capability-catalog,check-bash32,no-python-runtime-dependency}_test.sh`,
`go vet ./...`, interop vet and `make windows-vet`.

Pre-existing on `4052b8540` and left as found: `script/lane-scripts_test.sh` groups 27 and 55, and
`script/check-install-lifecycle_test.sh` step `mcp-unavailable-root`. Main's `corvint mcp` answers
`repository-unavailable`, not the `invalid-root` the script expects. The exhaustive gate was not run
(scoped issue work).

## Rollback

Revert any one merge commit with `git revert -m 1 <merge>`, together with the batch-own commit that
names it. The lanes' original commits, binds and seals remain in history.
