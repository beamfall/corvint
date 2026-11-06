<!-- SPDX-License-Identifier: Apache-2.0 -->
# Slow test speedups, 2026-11-05

Lane `slowtests`, tickets V1-2740 and V1-4381, base
`28a3ff86a2c3d17662765da12cc9251c06466f5f`.
The owner narrowed these tickets to the seven named tests and safe scheduling in
`cmd/corvint`. This is test execution work: no new requirements, product behavior,
wire fields, refusal text, goldens, core-freeze output or assertions change.

## Decisions and preserved proofs

- V1-4381: `workBuildExecutable` and `transportBuild` use the caller's
  `CORVINT_GOCACHE`, then `GOCACHE`, retaining the previous temporary-directory
  fallback when neither exists. `heavy.sh` gives each command a private temporary
  root; overriding its persistent compiler cache forced cold compilation on every
  invocation. Go's content-keyed objects can be reused while each executable,
  build flag set, digest, repository and mutable fixture stays private. The native
  transport test retains every admission, distinct verifier build and citation
  operation (ALO-V0-023 through ALO-V0-026). Sealed closeout retains the actual
  commit, repin and refusal checks (LCP-V0-041, LCP-V0-042). The oversized-plan test
  still drives both 300-hunk stages, the rebind, check, seal and clean repository
  assertions (DCW-V0-446); its staged protocol is inherently sequential.
- IDX-SNAP-V0-7803: partition every eligible tracked regular file across at most
  eight parallel workers, bounded by GOMAXPROCS. Keep all adversarial inputs,
  both frozen regex oracles, the public extraction/prefilter equality and the
  minimum full-checkout assertion. No corpus is sampled or shortened. The
  baseline CPU profile attributes 60.92% cumulative samples to regex matching
  and 23.71% to regex backtracking, rather than a sleep or repeated build.
- CAP-V0-950: run independent help-command slices as parallel subtests. Each
  still compares exact bytes against the complete catalog and checks its 5 KiB
  bound; the invalid-name exit and diagnostic assertion stays intact.
- PRB-V0-024 and PRB-V0-970: build the two immutable harness executables once per
  package through sync.Once and remove them in TestMain after all tests. Both
  consumers only read the binaries. Each retains private fixture directories,
  budgets, samples and measurement state; timing tests remain serial. This
  removes two duplicate builds in a package run, rather than promising a faster
  standalone smoke test. Keep the real-load and intentionally contended runs,
  builder-identity rebuild and wrong-scale refusal.
- PUB-V0-013 and PUB-V0-054: leave the bundled reproducibility test unchanged.
  It already runs four independent components concurrently subject to its
  measured scratch bound. Its two separate cold caches per component and real
  toolchain/build-info verification are the proof. Sharing a binary/cache or
  reducing components would weaken it, and changing the product build pipeline
  is outside this charter.

Rollback is to revert these test scheduling/helper changes and traceability
notes. There is no product migration or format rollback.

## CLI scheduling audit

Add t.Parallel to 52 existing top-level tests after following their actual helper
and command dispatch paths: producer (7), version JSON (5), repository guidance
(17), impact freshness (6), two-root impact (7), tests CLI (6), and test plans (4).
They use private fixtures and command buffers; they do not set process environment,
change working directory, replace global hooks/streams or start process-wide CPU
profiling. Read-only registry initialization is sync.Once protected.

Retain serial execution for the reviewed process-global cases, including:

- `TestGPKV0084ImpactCapabilitiesOutsideGitRoot`, `TestPiToolContextExpansion`,
  `TestPWRV0042_RunsBaselineRelativeRoot`, `TestTCBV0016GateVerdictRunsBothConsumers`:
  t.Chdir changes the process directory.
- `TestCheckpointReadsNoIndexSnapshotAndRetainsNothing`, source-slice/session
  fixtures and other t.Setenv callers: process environment mutation. The
  conservative helper audit identifies 68 direct/transitive candidates here,
  and 11 direct t.Chdir cases.
- `TestSelfBinaryStaleIsReportedByAffectedAndTest` and its three related stale
  notice tests: stubSelfBuildInfo replaces a process-global function.
- `TestTaskmanFixtureWriteFailureReportsDiagnostic` and handoff-hook tests:
  process-global hook replacement.
- `TestPRBV0008EveryNamedCommandWritesProfiles`: process-wide CPU profiling.
- Helper subprocess tests, including `TestSnapshotRefreshWorkerHelper`: stream,
  process hook or exit manipulation belongs to a subprocess lifecycle.

The remaining serial inventory is retained outside the repository; unreviewed
serial tests are not asserted to require serialization. A conservative static
call graph flags runContext through the repro command's os.Chdir even when a
specific test dispatches only context/help; those candidates needed manual path
inspection, not automatic acceptance or rejection. No broad mechanical
parallelization was performed.

## Measurement method

All runs used this host, Go 1.27.3, race off/on as recorded below, and one package
per `script/heavy.sh` invocation. No lock directory override or exported commit
identity was used for tests. TMPDIR was the private mode-0700
`/home/russell/lanes/slowtests.tmp`. The seven tests were located with git grep.
Each measurement used:

```sh
script/heavy.sh env GOTOOLCHAIN=local go test -json -count=1 -timeout 30m -run '^TestTransportAdaptedRecoveryNative$' ./cmd/corvint
script/heavy.sh env GOTOOLCHAIN=local go test -json -count=1 -timeout 30m -run '^TestTransportAdaptedRecoveryNative$' -race ./cmd/corvint
```

The non-race baseline scanner run additionally emitted a CPU profile and test
binary under the external evidence directory. Durations are the JSON timestamp
span from top-level run/continue to final pass, including parallel children and
excluding scheduler pause. Go's parent `Elapsed` omits parallel children and
would incorrectly report the changed scanner as 0.07 s. Command wall times,
compilation and semaphore waits are retained separately in the raw evidence.
These are single paired observations on a shared 32-CPU host, not quiet-host
medians or CI shard measurements. Recorded load ranges from 12 to 69; notably the
unchanged reproducibility race run was slower as host load reached 69.

| Test | Before s | After s | Race before s | Race after s |
|---|---:|---:|---:|---:|
| `TestTransportAdaptedRecoveryNative` | 65.00 | 24.53 | 55.83 | 45.80 |
| `TestCorvintBundledBinariesBuildTwiceIdentically` | 54.52 | 53.01 | 54.20 | 85.87 |
| `TestReferenceScannersMatchTheirRegexes` | 13.14 | 2.81 | 163.00 | 42.64 |
| `TestDogfoodFinishLaneDCWV0446StagesOversizedPlan` | 75.24 | 59.49 | NOT_RUN | 60.80 |
| `TestLocalCompletionSealedCloseoutRepin` | 33.06 | 12.81 | NOT_RUN | 26.84 |
| `TestCAPV0950HelpJSONCommandSlice` | 3.66 | 0.44 | NOT_RUN | 9.38 |
| `TestPRBV0024HarnessSmokeRun` | 14.76 | 15.65 | NOT_RUN | 79.69 |

The seven-test non-race sum is **259.381 → 168.745 s (34.9% lower)**.
The top-three race sum is **273.017 → 174.318 s (36.2% lower)**.
The 40% summed-time target is **NOT_MET** in these observations. No CI shard-time
improvement or quiet-host guarantee is claimed. Before-race runs for the other
four tests were NOT_RUN: the charter requires paired race measurements for at
least the top three; all seven changed-tree tests nevertheless ran with race.
The standalone harness observation is slightly slower; its reduction is in the
shared package setup, not fewer smoke measurements.

The 52-test CLI scheduling subset passed before and after with shuffle seed
1717: package execution **9.406 → 2.175 s (76.9% lower)**, command wall
12.751 → 4.994 s. The changed subset also passed with race in 6.463 s
(command wall 9.352 s), all 52 top-level tests executed in each run. No
whole-package before/after speed claim is made from this smaller subset.

## Verification, retained limits and friction

The pre-change context, query and impact were collected before editing; query
and impact receipts are bound to the stated base. Query and impact were READY,
with 1,872 explicit exclusions retained in the receipt rather than silently
broadened. Context digest
`16163178b39834d112875c7cc73f739148e1094972d68f976aa4459a4f57e83b`,
query digest
`0a9f2f0f1a0afd645cbe97f56b715f842c79ec7f25b1aaaa6917dc158097a133`,
impact digest
`b117ce5c3b9fc5434dbb4ebf1a83615b21d74adf069092f1a0b5eb9f211891de`.
This is actual local self-use evidence, not independent correctness or CI evidence.

`corvint affected --base 28a3ff86a2c3d17662765da12cc9251c06466f5f`
selected all three touched Go packages. The final-code plan has 303 selected
units and 3 LANGUAGE_FRONTIER unknowns (build-constraint variants, invalid shell
foreign-callers manifest, and unmodelled shell foreign invocation). Only the
three touched package suites and focused charter tests run in this lane; tier-2
read scopes and CI drivers are NOT_RUN here under the owner's bounded-check
instruction. The plan is an advisory selection, not proof that excluded units
are unaffected. TestMain causes perfbudget to be selected whole.

The traceability rows in the three existing specs were updated without changing
requirements. `make spec-requirements` passed with no generated changes;
`script/check-line-citations.sh --repin` found no unchanged citation content that
moved. `make -s spec-requirements-check line-citations-check` passed. Existing
spec-capacity warnings remain visible. The generator invokes its focused
obligation-bound test through heavy.sh; it passed.

The full package outcomes and final dogfood closure are retained in the lane
handback alongside the final commit. The entire contextindex package passed
(896 top-level passes, 3 existing skips; package 73.570 s, command wall 75.222 s). The whole CLI run executed all 1,520
top-level tests: 1,459 passed, 15 skipped under their existing conditions and
46 failed (package 309.066 s, command wall 316.152 s). No newly parallelized
scheduling candidate failed. The complete failure diagnostics remain in
`whole-cmd-failures.json` outside the repository; the sandbox path blockers
below prevent claiming whole-package qualification. No failed assertion was
removed or relaxed. Independent review, CI qualification,
landing, native ticket completion writes, repository-wide gates and publication
are NOT_RUN here by the lane contract. Darwin/Windows qualification is
NOT_PRODUCED by these Linux runs. The host
orchestrator owns review and integration. Tickets stay open until that work and
native completion are evidenced. No seal, push, PR or Tasks write is authorized
in this lane. No speed-specific accepted product intent or new requirement is
introduced; the CEM uses the explicit no-intent declaration and retains
intent-linkage NOT_ASSESSED, while the requirements above describe preserved
proofs.

1. Ticket reads: the native journal is absent. Reproduce with
   `corvint-tasks ticket show V1-2740` (likewise V1-4381). Read-only recovery uses
   `CORVINT_TASKS_SNAPSHOT=/home/russell/lane-inputs/ticket-bodies-fr-scripts/queue-snapshot.json`
   with the same command. This snapshot is about 31 hours old; live journal and
   completion state are NOT_OBSERVED. No write workaround was used.
2. The installed binary predates this tree (built revision d436dff76de1);
   affected initially warned about 54 differing Go-source paths. Reproduce with
   the installed `corvint affected --base` command above. Subsequent selections
   use the lane-built current-tree binary from the initial dogfood build.
3. Regeneration reads the Git index. Running `make spec-requirements` or
   `script/check-line-citations.sh --repin` with the three edited specs unstaged
   refuses `unstaged-spec` (SRG-V0-954). Stage those specs first, then rerun; the
   staged generation and repin passed. No generated requirement IDs changed.
4. Timing extraction: summing only a parallel parent's JSON Elapsed loses child
   runtime. Reproduce by inspecting the changed scanner's parent pass (0.07 s)
   against its run/continue-to-pass timestamps (2.809 s). Use the timestamp span;
   corrected metrics and original logs are both retained.
5. Shared-host load reached 69 during the unchanged reproducibility race run.
   Its 54.195 → 85.872 s change is not an optimization result. Reproduce the
   commands and retain host load; a quiet-host repeat and CI result are external
   validation, not substituted here.
6. The optional repository-wide dogfood run ledger would write
   `/home/russell/projects/corvint/.git/corvint/dogfood-change-runs-all.tsv` outside
   the allowed Git metadata paths. Its parent is not writable in this sandbox;
   the wrapper's optional write is best-effort. Per-worktree dogfood evidence is
   retained under the allowed `worktrees/slowtests/corvint` directory. No Git
   permissions or hook configuration was changed.

Raw JSON logs, CPU profile, test binary, audit inventories and command drivers
stay under `/home/russell/lanes/slowtests-evidence/`. Only digests and supported
numbers are committed. The handback supplies the final external manifest digest
and whole-package outcomes. Files with `NOT_RUN` or `NOT_PRODUCED` outcomes remain
explicit rather than being inferred from a passing structural CEM check.

7. Whole CLI tests exposed sandbox path preconditions: `/home/russell/lanes`
   initially had mode 0775, while executable/scratch qualification rejects a
   non-sticky writable ancestor. Its account owner removed group-write
   (`chmod g-w /home/russell/lanes`, resulting mode 0755). A focused repeat still
   refuses because `/home` is owned by uid 65534 (`nobody`) in this sandbox,
   outside the operator/root ownership set. Reproduce with
   `TestWorkScratchRootFollowsOverride` and `stat -c '%U %a %n' /home
   /home/russell/lanes`. No assertion, ownership qualification or product code
   was weakened. The remaining ownership condition requires the host verifier.
8. Read-only sandbox placeholders include `/home/russell/lanes/.git` (mode
   0555, not a usable Git repository). The profile guard correctly refuses
   destinations beneath any Git marker, and non-repository CLI discovery also
   observes this marker. Reproduce the focused
   `TestAHI373MCPVerbOwnsItsArgumentsAndProfile`,
   `TestPRBV0008EveryNamedCommandWritesProfiles`,
   `TestCorpusWorkEvidenceIsNonAuthoritative` and
   `TestGPKV0084ImpactCapabilitiesOutsideGitRoot` tests using the required TMPDIR.
   The focused repeat failed unchanged after the permission repair. Removing or
   hiding the protected marker, changing TMPDIR or relaxing the guards is outside
   this lane's authorization. These failures leave whole-suite qualification
   BLOCKED for the host-side verifier; the local dogfood outcome is `blocked`.

The entire perfbudget package passed with race: 21 top-level tests,
package 105.504 s, command wall 107.699 s. This includes both shared
binary consumers and TestMain cleanup.

Measurement manifest: `measurement-manifest.json`, SHA-256
`81a64b8edcd474eddf105abedeb06d7ef9b11d1f24e0d4ac68ca6ec4fa02f3aa`, 74 artifacts.
Corrected timing data SHA-256
`4c08dde8508d75324b20ae0425baf0326bd7d07507a03d2be7630ad3ba268017`.
Scanner CPU profile SHA-256
`e5f841a8c25023b506af63cb427cd54fd18ee2c8777a0aa133ffcaa4f644fdf7`.
These immutable digests cover the retained original measurements, package
failures, candidate test diff and audit inventory; future closeout logs are
separate handback evidence.
