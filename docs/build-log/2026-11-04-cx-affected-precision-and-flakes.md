<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4544 / V1-4549: bounded documentation selection and CI failure attribution

Base: `6c33df33bc5670131482f5263b410cb980580657`. Implementation:
`abec7819d998769b5ffcb8275d7be015bec07f28`; fix round 1:
`461e91bd32c124a7c77b3762db2e31e00992335f`. Scoped local work; no publication, native Tasks writes or seal.
Independent review belongs to the orchestrator. No claim of repository-wide validation.

## Decisions and selection evidence (V1-4544)

TSP-V0-1061 and TSP-V0-1062 use the next free TSP ID on origin/main (981) plus 80.
Both are recorded under standing acceptance, not owner read-back.

Use the existing AFP-V0-772 project declaration for `docs/build-log/`, with a reason in
runtime-readers and a matching unbounded-reader exclusion. Do not special-case Markdown or
change the generic documentation contract. Literal, glob, enclosing, explicit-scope and declared
owner readers remain subject to their existing rules. The exclusion is an owner-maintained
statement about this repository's dated records. Root-locator-only tests build Go tools, scan
source, or operate on temporary/immutable fixtures; the selfcorpus test inventories only
`internal/doccompiler/draft.go`, not the documentation tree. A new runtime reader of the records
must declare that input or remove the exclusion. Removing the two entries restores the floor.

Keep the declared-consumer fan-out of a Go test edit. A read scope such as `internal/` permits
actual source reads, not just `go build`, so ignoring it for `_test.go` would be unsound. Every
DECLARED_READ_SCOPE witness now names its Go/shell declaration and matching entry, including
implicit own-subtree and changed-declaration cases. A changed `.corvint/test-read-scopes.json` is
named as itself even for a shell unit it selects (fix round 1; it was paired with the shell
declaration before). Full JSON, compact JSON and text carry the
explanation. No graph/cache format, read-scope grammar, import rule or confinement permission
changes. Rollback removes the optional witness and its projections.

`TestNewTestKeepsNamedDeclaredConsumers_TSPV01062` creates a genuinely new test file before
building the graph: only its package and an explicitly declared consumer are selected, never its
importer. A production edit still reaches the importer. `TestNewBuildLogKeepsDeclaredReaders_TSPV01061`
checks the real repository declarations, true named readers and undeclared-data fallback.
`TestDeclaredScopeExplanation_TSPV01062` checks both JSON projections and text.
`TestReadScopeCausePairsDeclarationAndEntry_TSPV01062` pins the declaration/entry pairing.

The round-0 replay selected the rescue-b paths over the lane's current source graph, so the
rescue-only `_test.go` was unindexed and took the fallback. Fix round 1 replaces it with a CLI
replay over the rescue-b tree itself, from `852b045e61484efc53a99fa1f320984b9bb3cd2c` to
`a435407595eb0cae03be4a9e7f49dd394f2659b5`. The changed-declaration arm is that base plus only
the lane's two declaration files (`0f0cb62fe`), with the rescue commit cherry-picked on top
(`40d1309dc`). An identity check confirms the same eight paths and target trees that differ only
by the two declarations. Both arms use one binary built from the fix-round head. Retained results:
`.agent-evidence/cx-affected-selection-replay.json` (`fixRound1`; round 0 kept as `round0`).

| Shape | Base declarations | Changed declarations | Interpretation |
|---|---:|---:|---|
| New build-log path | 35 | 11 | 24 UNBOUNDED_READER selections removed; true named readers retained |
| Indexed new test `tools/gate-ledger/shared_digest_test.go` | 37 | 37 | Indexed (DIRECT_TEST_CHANGE); 24 root-locator floor, 5 declared consumers |
| Complete rescue-b mixed delta | 106 | 106 | Same unit set; every removed unit is still reached by another path |

Why the union cannot shrink: all 24 units removed from the build-log path are still selected by
each of the other seven changed paths. Thirteen have a real witness on some other path (BINARY_EXEC
from `script/batch-precheck.sh`, IMPORT_GRAPH_READER, PATH_LITERAL_READER, DEPENDENCY_PATH or
DIRECT_TEST_CHANGE). The other eleven are reached by every spec, Go source, Go test and
shell-script path only through the root-locator UNBOUNDED_READER floor. They are exactly the
delta's eleven UNBOUNDED_READER units, now attributed to `docs/specs/gate-ledger-v0.md`.

V1-4544 AC3 is therefore PARTIAL. The union can drop at most those eleven floor-only units, and only
once none of the spec, the Go test, the shell scripts and the three gate-ledger Go files reaches
them through the floor. That needs the units' actual read scopes declared (or an owner-maintained
exclusion for those path classes): the fan-out in V1-4535, outside this ticket. Reduced isolated
contribution is demonstrated; reduced mixed-union count is not, and the delta stays UNKNOWN (two
unknowns). AC2 uses the declared-consumer alternative: the delta's twenty DECLARED_READ_SCOPE
witnesses name `test-read-scopes.json internal/`, `shell-read-scopes.json internal/` and
`script/`, or `docs/specs/`. No claim that all test edits select one package or that an
exclusion proves safe omission.

## Original CI evidence and per-test disposition (V1-4549)

Read issue [52](https://github.com/beamfall/corvint/issues/52), all ten records in its published
window, and two subsequent records. Downloaded original job logs directly with
`gh api repos/beamfall/corvint/actions/jobs/JOB/logs`: `gh run view --job JOB --log` returned
empty success for these logs. `.agent-evidence/cx-affected-ci-failures.json` retains artifact
records, target revisions, job URLs, SHA-256 digests and diagnostic excerpts. Complete downloaded
bytes and decoded logs remain in `/tmp/cx-affected-evidence/`. The cmd/corvint test from issue 52
is outside this charter and is not represented as fixed.

Most failures were race-detector contamination, not independently failing assertions. In
`41424244656` / job `125207342632`, `withReadPatience` wrote
`snapshot.DefaultPatience` while parallel CLI readers read it. In `41476603865` /
`125385291497`, the policy-update test made the same shared write. The base already carries
`2ff54c5b4` (per-command policy-preview patience) and `990f74e8a` (serialize the remaining
internal patience test). In `41467616941` / `125353161108`, `ShrinkReviewPacketHistory` raced
parallel packet reads; the base already carries `cb8fefdf0`, serializing that test. Reuse those
repairs, rather than serialize every collateral victim or weaken the race detector.

| CLI test | Original failure mode | Local disposition |
|---|---|---|
| TestCALV0181_TicketListStatusFilter | shared patience race | existing repair; race count=20 PASS |
| TestCALV0260_AdministrativeWritersSettleHeldPendingReceipt | policy-update shared patience write | existing per-call repair; race count=20 PASS |
| TestCALV0045_CLIConfiguredRetriesAndNoTreeHandoff | shared patience race | existing repair; race count=20 PASS |
| TestATRV0059_ClaimCarriesDrainNotice | detached claim SUPERVISOR_LOST, no kept result | original failure not reproduced; race count=20 PASS; no new claimed fix |
| TestATRV0310_DetachedChildrenAreJoinedBeforeTheLeakCheck | nested detached helper SUPERVISOR_LOST | original failure not reproduced; race count=20 PASS; no new claimed fix |
| TestCALV0141_PlanNodeBoundScalesPerEntry | shared patience race | existing repair; race count=20 PASS |
| TestCALV0214_CreateAndRefineWarnOnExactDirectoryKey | shared patience race | existing repair; race count=20 PASS |
| TestCALV0277_RenewOnlyHolderNeverReadsStale | shared patience race | existing repair; race count=20 PASS |
| TestCTSV0006_ReadVerbsUnderConcurrentWriter | shared patience writer | existing serialization; race count=20 PASS |
| TestDSGV0012_RollingUpgradeWithRealExec | shared patience race | existing repair; race count=20 PASS |
| TestERGV0017_ReviewPacketAcrossRounds | shared packet-history bounds race | existing serialization; race count=20 PASS |
| TestERGV0019_CapNamesOmittedRanges | shared packet-history bounds race | existing serialization; race count=20 PASS |
| TestERGV0026_CheckpointCommitBinding | packet-history race (later record also patience race) | existing repairs; race count=20 PASS |
| TestERGV0027_HistoryBoundNamesNextCursor | shared packet-history writer | existing serialization; race count=20 PASS |
| TestTCDV0169_BatchCloseClosesDeliveredOpenTickets | shared patience race | existing repair; race count=20 PASS |
| TestTCDV0185_GatedBatchCloseRunsRequiredGates | shared patience race | existing repair; race count=20 PASS |
| TestTOLV0139_ArchiveCarriesReceipts | shared patience race | existing repair; race count=20 PASS |

Distinct remaining fixtures repaired here:

- `TestCTSV0014_ImportMappedQueueLeasesAndReleases` (CTS-V0-014/015): original
  job `124575173368` refused execution cutover because its timestamp was one second before the
  release receipt. Initialize the lease helper after that release so its simulated clock cannot
  precede it. Race count=50 PASS, 54.832 s. No production timestamp rule changed.
- `TestCancellationRetiresDescendant` (PMR-V0-004): original job `124578728134` failed
  "no descendant observed" after 0.41 s, not its ten-second deadline. Reading the PID file between
  creation and write parsed empty bytes as zero and stopped waiting. Publish by atomic rename;
  the reader also waits for a valid positive PID. Race count=50 PASS, including the FIFO test,
  35.253 s. No production cleanup deadline changed.
- `TestSERVICE500_HelperInstallAcceptedOnLinuxRefusedElsewhere` (SERVICE500-005/007): original
  job `125068726618` matched `8080` anywhere in generated unit text, including random directory
  digits. Check quoted argv atoms instead, retaining the negative check for both `--port` and
  `8080`. Force `home8080` in the fixture path (independent of Go TempDir's truncated prefix).
  The pure unit-rendering qualification passed race count=50, 4.614 s. Round 0 called the install
  qualification externally blocked (a uid-65534 sandbox). Fix round 1 ran it on wsl-lanes (uid 1000,
  umask 0002). It failed at `2a2507748` with UNSUPPORTED_FILESYSTEM because the TempDir root kept
  mode 0775: the `home8080` suffix was restricted, but its parent was not. `newServiceHome` now
  chmods the TempDir root to 0700 before joining the suffix. The whole service package passes
  (10.4 s), and the five install/render tests pass `-race -count=20`, GOMAXPROCS=8 (7.6 s).
  Production authority checks are unchanged.

The 17-test CLI selection passed `-race -count=20`, GOMAXPROCS=8, on WSL2 in 399.778 s.
Natural host load and concurrent focused package runs supplied load; a retained observation reads
8.71 / 13.67 / 14.39. No artificial background workload was started. No tests were quarantined or
removed. The two detached failures remain unreproduced; a passing replay does not identify their
historical cause. Next weekly hosted report: NOT_OBSERVED (future external evidence), so V1-4549
is not represented as fully accepted.

## Verification and remaining limits

All Go tests used `script/heavy.sh`, one package per invocation, after
`corvint affected --base 6c33df33bc5670131482f5263b410cb980580657` selected the packages.
The implementation plan was UNKNOWN; tier-2 units, unknowns and omitted exhaustive coverage
remain visible in the retained plan. Go version: go1.27.3. Focused affected regressions, CLI
projection regressions, spec regeneration, declaration/read-scope checks, requirement definitions,
traceability, capacity/collision checks and `make windows-vet` passed. No exhaustive gate, model
reviewer, publication, release, Tasks mutation or seal was run, per charter. Fix round 1 reran
`corvint affected --base 2a2507748` (UNKNOWN, 141 units), the focused affected and CLI projection
tests, the service package and race runs above, the declaration/read-scope/requirement/traceability
checks, gofmt and `make windows-vet`, all passing. Retained under `fixRound1` in
`.agent-evidence/cx-affected-verification.json`. Local outcome stays `blocked`. Two items remain:
V1-4544 AC3's mixed-union reduction (V1-4535) and V1-4549's next weekly hosted report.

A first affected regression incorrectly expected lrfrepo to read build-log; the actual named
readers are cmd/corvint and flow-guide-check. Corrected the assertion from original plan evidence;
the final regression passes. This was a test expectation error, not a dropped production reader.

Friction with reproductions:

1. Plain `corvint-tasks ticket show V1-4544` refuses because this checkout has no journal/new
   ticket snapshot. The refusal supplies the read-only staged snapshot; retrying with
   CORVINT_TASKS_SNAPSHOT reads both tickets. No queue writes.
2. `make dogfood-prechange ... PATHS="internal/liveverify/affected ..."` records context/query
   but refuses impact `unsupported-impact-path-suffix`. Retry the same task at BASE with concrete
   .go paths; all three bound receipts then record successfully.
3. `script/heavy.sh ...` refuses `/home/russell/bench/rt/.locks` as unwritable. Use writable
   `/tmp/corvint-heavy-1000`; the configured host semaphore remains unavailable, so its cross-lane
   coordination is not claimed. Same policy, eight slots; no approval bypass.
4. The service-install test refuses UNSUPPORTED_FILESYSTEM on uid-65534 ancestors with the
   default TMPDIR and explicit TMPDIR=/tmp. Repro: `stat -c '%u %n' / /home /tmp`, then run the
   focused install test. Fix round 1: outside that sandbox, the test showed that the round-0 suffix
   change had left the TempDir root 0775. That is now fixed; the sandbox symptom had hidden it.
5. Git commit reports read-only `/home/russell/projects/corvint/.git/packed-refs.lock`, but the
   loose branch update and commit succeed. Do not alter the forbidden metadata path.
6. `gh run view RUN --job JOB --log` exits zero with empty output for the retained CI jobs;
   the direct job/logs API returns original bytes. Retain sizes and SHA-256, not an empty-success claim.
7. The original 17-test command exceeds the recorder's 512-character VERIFY-line bound.
   Retain that exact count=20 invocation separately and run a shorter equivalent-selection count=1
   spelling for the CEM verification list. The repeated-run evidence is not relabelled as that run.
8. `affected --path` correctly refuses the dirty worktree. The explicit `--worktree` path replay
   works and labels its evidence unpinned. No dirty work was discarded to make a read succeed.
