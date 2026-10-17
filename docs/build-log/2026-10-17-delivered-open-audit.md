# 2026-10-17: Audit of delivered but open tickets

## Method

A read-only audit of 75 tickets that had merged work but were still `OPEN` in the task store. It
ran against `origin/main` 55abb815c4c20a771363d38e315ddfddf14c6972, fetched 2026-10-17. For each
ticket it read the ticket's acceptance criteria from the `.taskman` export on that commit, then
checked each criterion against the merged PRs, the tests and the build-log entries they cite. No
`corvint-tasks` command ran and no repository, store or remote state changed. A `full-gate`
required gate that no lane ran is a closeout step, not an acceptance criterion, so it does not
change a verdict here.

Result: 43 `CONFIRMED`, 29 `PARTIAL` and 3 `NOT_DELIVERED`. A `PARTIAL` ticket has a split text for
its residual, kept with the orchestrator for filing. The owner's answers on the 46 blocked tickets
from the same day are in decision 0539.

## CONFIRMED (43)

| Ticket | Merge | Evidence |
|---|---|---|
| V1-0252 | 81c4b8515 (#201), c6574c7a7 (#675), 803037715 (#904) | `TestAFUV1028ScriptedAgentCompletesGoals`; navigation golden, effect class, grant and observer tests; reruns in 2026-10-14-batch1-flows-runners-closeout |
| V1-0253 | 484d03176 (#202), 803037715 (#904) | Docs render, drift, waiver and anchored-Markdown tests; `TestAFUV1036DocsReplaceConfined` |
| V1-0254 | 26d211e73 (#205), 803037715 (#904) | `TestAFUV1034FlowsToolProfile` and compiled conformance on both protocol versions; hash-bound official-schema log |
| V1-0255 | af2fe826d (#206), 803037715 (#904) | `TestAFUV1039AcceptanceFixture`; retained live companion, MCP and cleanup records with matching manifest hashes |
| V1-0249 | bad1cc5ad (#256), 82cde9bc7, 4f1cd96e2 (#981) | Adapter retry/timeout/failed-control fixtures; Playwright keep-every-attempt and run-evidence bound tests with mutations |
| V1-0251 | d0d0b1e49 (#192) | Fallback, undiscovered-test and byte-stability tests; frozen e2e-safe corpus report with unsafe=0 and a negative control |
| V1-0150 | 6d1f77281 (#905) | Hosted release-gates run 38654510601 (linux-core, amd64) passed, hostile matrix 27/30; transcribed in 2026-10-14-batch2-small-residuals |
| V1-1023 | 29cc7fd4e (#697), bcc9d6dc4 (#962) | `internal/testplan` and `test-plan consolidate/check`; 44-variation console inventory pinned by `TestConsoleQualificationGolden` |
| V1-0328 | ee12b23e0 (#980) | Bytes per re-index (155,933,247) and CPU medians retained in 2026-10-16-v1-0328-reindex-snapshot-measurement |
| V1-0414 | 43238ab10 (#983) | `TCP-V0-055..058` accepted; hidden-Unicode tests with mutation; `CEM-V1-270` `selfModifiedAuthority`; `docs/SECURITY.md` references |
| V1-0811 | 8803b7ddc (#870) | One forward-row artifact per control-plane run (runs 39033416546, 39018410108, 39018231490); focused CLEAN/FLAKY/MISS/INCOMPLETE tests |
| V1-1349 | ba269ccb5 (#934) | Out-of-hunk recurring-span tests fail at BASE and under mutation; `DCW-V0-063` accepted |
| V1-1420 | f8a963e38 (#928) | GOROOT re-hash cause instrumented; fixed fixtures pass at load 40-69; product bound pinned at 15 s; 3 mutations |
| V1-1421 | f8a963e38 (#928) | BASE fails 3/10 at load 61-63, fixed passes 10/10 at load 60-66; mutation reproduces the symptom |
| V1-1427 | 85ba6321a (#933) | `MCPV0-038` passes `repository-probe-timeout` through; `TestMCPV0038ProbeTimeoutHasDistinctCode` with mutation |
| V1-1428 | 85ba6321a (#933) | `GLTP-V0-054` `ErrAuthorityUnavailable`; `TestPostRunDeadlineIsUnavailableNotDrift` (4 cases) with mutation |
| V1-1429 | 85ba6321a (#933) | Post-run calls at most 5.9 s against the 15 s bound at load 48-59; no INCONCLUSIVE at loads 42-73 |
| V1-0161 | d867347d7 (#283) | daily-loop-v1 harness sets `EXPECTED_REFUSAL` per case and marks a mismatch non-informative (code reading only) |
| V1-0162 | d867347d7 (#283) | Harness refuses `preregistration-digest-mismatch` or `seal-missing`; `PCCO-V0-015` names the seal (code reading only) |
| V1-0181 | 3c1b894c3 (#239) | `docs/DOGFOOD.md` states OCM maps are local-only and names the rebuild and read-only status commands |
| V1-0289 | 3c1b894c3 (#240) | Plugin frontier and build-constraint variant scoping; regression tests in `internal/liveverify/affected` |
| V1-0292 | 94ca556f5 (#287) | Corroboration limited to names of at least 4 bytes; `TestTaskContextSwiftImporterNeedsMoreThanAnInitializerOrAMember` |
| V1-0352 | c21e7c19d | `TestCEMV1210LRFRefusesEveryMapBeforeAnyInput`; the 0.1/0.2 criterion is superseded by decision 0525 |
| V1-0372 | 39bcf1ba6 | Personalized PageRank exact-map and scan-once tests; graph-off parity by `TestContextGraphDefaultBytes` |
| V1-0376 | 84257dc41 | INSTALL anchors, `<version>` placeholder and four-server README table (status given in prose) |
| V1-0377 | 97c3a4bd0 (#755) | README, INSTALL and AUTOMATION name both ledgers and their write conditions |
| V1-1081 | 41ed08212 (#721) | Upstream-reachability tests; `COMMIT_NOT_INTEGRATED` registered. `CAL-V0-017` acceptance recorded by decision 0539 |
| V1-1118 | 71b2eb8fc (#747) | `CAL-V0-211` accepted (decision 0499); three exact-directory-key warning tests |
| V1-1127 | 97c3a4bd0 (#755) | Build log 2026-10-10-stage-slot-race names gate-a and GateRun at 30e306ec |
| V1-1341 | 1183ea443 | `DCW-V0-065` fix line names the `sh path/to/script.sh` escape; `TestDogfoodChangeNamesVerifySyntaxRemediation` |
| V1-1354 | 1183ea443 | One 16 MiB-bounded `flow.readGit`; `TestDCWV0064GitReadOverItsBoundRefusesOrIsUnbounded` |
| V1-1388 | 1183ea443 | `TestOCMV0022DogfoodFinishKeepsAggregateBoundRefusal` covers the three codes and the 512 B detail bound |
| V1-1509 | 88d740e29 (#987) | A scan of every `*.go` file finds no bidi or zero-width code points; AST literal hashes unchanged |
| V1-0102 | 8202c25ee | `EEP-TR-012..014` defined (decision 0351); capability negotiation and strict decode tests. Owner acceptance not found |
| V1-0155 | 97c3a4bd0 (#755) | `docs/DOGFOOD.md` supports a linked worktree for the loop, with the reason |
| V1-0176 | 97c3a4bd0 (#755) | DOGFOOD names `change.ocm.001.json`; `script/dogfood-change_test.sh` pins it |
| V1-0265 | 97c3a4bd0 (#755), deffd56e | Decision index has one row per decision; `check-decision-numbers.sh` fails on duplicate or missing numbers |
| V1-0267 | 97c3a4bd0 (#755) | Citations of `GPK-V0-046` and `GPK-V0-053` match the text; the citation-gate exclusion reason is recorded |
| V1-0362 | 5cd9419a6 | Sanitized Git environments in the named runners; each changed runner has a revert-failing test |
| V1-1115 | b524b5ed4 (#743) | `-count=200` passed under concurrent package runs |
| V1-1169 | 47dcf29bc | `AMAP-V0-078/079` accepted (decisions 0519, 0520); React route-array tests; computed arrays stay UNKNOWN |
| V1-0215 | 26d0b7e36 | `TestRelativePathsLinkedWorktree` (gitauth read path, not the CLI) |
| V1-0113 | 101be79c0 | `TestInitAdoptInterruptedRunLeavesNoStateAndRetriesCleanly` kills init and adopt at five delays |

## PARTIAL (29)

| Ticket | Merge | Met / not met |
|---|---|---|
| V1-0632 | 01f95dbe4 (#804) .. eb53998e0 (#836) | Per-slice evidence met; capsule and dashboard cem/1.0 positive cases, portable envelope and Tasks-to-verify chain not met |
| V1-1266 | 3fcc1d11c (#893), f81114713 (#944), c2bcf9147 (#961) | Held tier and release notes met; the 30-change held-share tally waits for V1-1449 |
| V1-1022 | 29cc7fd4e (#697) | TOL-V0 implemented and accepted; live Playwright 1.63 qualification `NOT_RUN` |
| V1-1024 | 29cc7fd4e (#697) | Step negation merged; `LPCV-V0-070` qualified-tuple matrix `NOT_RUN` |
| V1-1114 | 5b6b85441 (#954) | Fixture tests and permissions met; no live failed scheduled run observed yet |
| V1-0285 | 2814af65b (#901), 449c9d1a2 (#978) | Refusal replay 12/12; success replay 9/12 (cem, ocm, frontier skipped) |
| V1-0887 | a4aa63bbb (#936) | Semantics and `CAL-V0-224` met; the 50 ms heartbeat/renew target is not met |
| V1-0191 | d138a582d (#123), 38eff3609 (#128) | Schema run and task-review profile met; `mcp-server-unavailable` and the spec's proposed status remain |
| V1-0205 | 242fde633 (#126) | Constant named; re-bench on the merged binary and the frozen 7-task set missing |
| V1-1382 | 253d6341f (#943) | Release gates exist; a hosted failing and passing run is `NOT_RUN` |
| V1-0369 | d867347d7 (#283) | Cost gate code present; no committed fixtures or test |
| V1-0382 | d867347d7 (#283) | Seal checks present; daily-loop-v1 preregistration not committed or sealed |
| V1-0331 | 94ca556f5 (#287) | Symlinked-ancestor fix tested; no retained README quickstart run under macOS `/tmp` |
| V1-0436 | 53e7e3df2 (#926) | Synthetic benchmarks met; the Beamfall 2,894-item export profile is `NOT_RUN` |
| V1-0755 | 3ec6eeab7 | Pin refusals met; `CAL-V0-074/075` acceptance unrecorded; live Claude Code qualification `NOT_RUN` |
| V1-0756 | c539407e6 | Pin refusals met; `CAL-V0-076/077` acceptance unrecorded; live OpenCode qualification `NOT_RUN` |
| V1-0945 | ed3171a41 | Limits reviewed; root help 3,967 B against a ~1.5 KB target; `CCF-V1-009` proposed |
| V1-1302 | 55b0c0f09 (#914) | Reproduction and fix met; full `-p 1 ./cmd/corvint` runs hit the hang detector |
| V1-0023 | 778b70a55 | Summary/expand, digest, golden and rollback met; matched complete-task trial `NOT_OBSERVED` |
| V1-0752 | f9d881f47 (#550) | Ratchet and merge_group run met; the dependency-locates-root case still merges clean |
| V1-1121 | 2145af619 (#746) | Failed-report retention met; secret exclusion not evidenced |
| V1-1241 | d63c1c3a1 (#880) | Package costs regenerated from go1.27.2; `test-slices.json` not regenerated |
| V1-1397 | 1183ea443, bb6648f7e | Three fix lines met; distinct stale-binding code and INT/TERM trap missing |
| V1-1434 | c1e0a4a06 (#942) | Refuted 10/10 on darwin; the linux CI detached-browser failure is unresolved |
| V1-1465 | bd1bc85db (#965) | Sweep and pool fixed; TOLV0027 uses a fixed 20 s timeout; no low-load run |
| V1-0130 | 4088e22ad | Named omissions added; 71 of 203 linked specs absent from `PORTFOLIO-0.6.md` |
| V1-0303 | 0aac4a444 | Reword met; job status still synced by hand, not generated from the ledger |
| V1-0393 | 32bfc85a4 (#753) | The three named packages were fixed and darwin vet passes; cross-OS `go vet` fails today (below) |
| V1-0697 | 5d46b4b50 (#514), 618680492 | Service profile and drain met; every real platform witness `NOT_RUN`; no terminal CEM |

## NOT_DELIVERED (3)

| Ticket | Merge | Reason |
|---|---|---|
| V1-0478 | f227c72be (#457, labelled partial) | #457 rates AC1-3 partial and AC4 not met; held-out set `NOT_PRODUCED`, all tuples unqualified |
| V1-1321 | 59d492017 (#915) | Stopped: unioning `--scope` with touch paths contradicts accepted `CAL-V0-021` and needs an amendment |
| V1-0225 | #755 attempted | `GLOSSARY.md` is not scanned by the citation gate; its pinned MCP spec citation is stale |

## Weaker evidence

- V1-0161 and V1-0162 are `CONFIRMED` by reading the harness code only. No committed test exists,
  and daily-loop-v1 is unsealed.
- V1-0150 and V1-0811 rest on GitHub Actions run logs and artifacts, which expire (artifacts after
  14 days). V1-0811 has no count yet against the 20/5 bar.

## Cross-OS `go vet` fails today

Re-checked on this branch's base, `origin/main` 072a306f1 (host load about 60), with Go 1.27.2:

```sh
GOOS=<os> GOARCH=amd64 GOTOOLCHAIN=local go vet ./...
```

| GOOS | rc | First errors |
|---|---|---|
| freebsd | 1 | `internal/tasks/store/resource_exhausted_reap_test.go:30:23: undefined: inventoryClaimRepo` |
| openbsd | 1 | `internal/worktreeimpact/compiler.go:37:20: undefined: platformFileIdentity` (also `openTarget`, `identityFrom`); the tasks/store test error |
| netbsd | 1 | Same as openbsd |
| solaris | 1 | `internal/stepnegation/lock_unix.go:22:20: undefined: syscall.Flock`; `internal/contextindex/fifo_unix_test.go:21:20: undefined: syscall.Mkfifo`; the worktreeimpact and tasks/store errors; dependent packages `dashboard/roadmap`, `dashboard/source`, `localcompletion`, `playwrightruns`, `receiptbundle`, `tools/gate-ledger` and `tools/native-hook-observer` fail with them |
| darwin (host) | 0 | none |

No CI job vets these targets, so the debt is invisible. V1-1615 is the follow-up that makes it
visible. V1-0393 stays `PARTIAL` until then.

## Not done here

No ticket was completed, split or filed by this entry; the orchestrator does that in the task
store. `make gate` was `NOT_RUN`.
