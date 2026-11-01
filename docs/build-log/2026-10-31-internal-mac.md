# 2026-10-31 internal-mac lane

Lane internal-mac took 16 internal-package tickets at origin/main
`8daec98ce4349c1583186692726d2c3a782ac647` on branch `claude/lane-internal-mac-1031`. Three sub-lanes
built disjoint subsets in their own worktrees (A: V1-1961, V1-1962, V1-2504, V1-1389; B: V1-2620,
V1-2732, V1-2302, V1-0805; C: V1-2612, V1-2350, V1-0418), and their branches were merged here. The lane
reserved requirement suffixes 940-949.

## Dispositions

| Ticket | Outcome | Retained limit |
|---|---|---|
| V1-3176 | Delivers | none |
| V1-1032 | Delivers (LPCV-V0-940, LPCV-V0-941) | none |
| V1-2292 | Delivers (recorded floor-member decision) | none |
| V1-0585 | Delivers (OCM-V0-940) | none |
| V1-0583 | ALREADY-DELIVERED (AC1) | AC2 BLOCKED on V1-0579; AC3 BLOCKED on orchestrator review and closeout |
| V1-1961 | Delivers (TOL-V0-940, TOL-V0-941); AC1 ALREADY-DELIVERED | integrated adopter-repository run NOT_RUN |
| V1-1962 | Delivers AC1 | AC2 NOT_PRODUCED: needs an adopter repository |
| V1-2504 | ALREADY-DELIVERED (ATR-V0-031, CAL-V0-077) | live caching-provider run NOT_RUN |
| V1-1389 | Delivers (CAL-V0-940); AC1 ALREADY-DELIVERED | AC3 BLOCKED: live dispatcher run and real reboot |
| V1-2620 | Delivers AC1 (CCE-V0-940) | AC2 BLOCKED: 5 hosted macos-15 cce-live runs after merge |
| V1-2732 | ALREADY-DELIVERED: no reproduction | none (20/20 passes on WSL meet the AC2 close condition) |
| V1-2302 | ALREADY-DELIVERED: no reproduction | none (-count=20 on WSL passed, 640 PASS) |
| V1-0805 | Delivers (PMR-V0-940..943) | independent review left to the orchestrator |
| V1-2612 | ALREADY-DELIVERED: not reproduced on the base | none (regression test added) |
| V1-2350 | ALREADY-DELIVERED by V1-2434 (DCP-V1-314..317) | none |
| V1-0418 | Delivers (DIRTY-CACHE-940..942) | none (the overlay is not faster than a rebuild; measured) |

## V1-3176: companionrelease subprocess failures name the signal

- Decision: `describeExit` reports the killing signal and the raw wait status when a build or
  packaging subprocess dies from a signal, instead of the bare `exit status -1`. Exit-code failures
  read as before.
- Evidence: `internal/companionrelease/proc_exit_test.go` kills a child with SIGKILL and checks the
  message names it. It needed a minimal environment because the inherited `GOFLAGS` produced a
  duplicate env entry.

## V1-1032: `/0` step-negation join binds spec and config digests only (LPCV-V0-940, LPCV-V0-941)

- Decision: a `/0` (`ExternalProfile`) receipt has no test repository identity, so the
  `LPCV-V0-068` join stays allowed, but each joined `/0` strength axis now carries the anchor
  `strength-unbound-inputs:test-support-files`. A reader can see that helper and fixture files are not
  bound. Refusing the join would have hidden real negation evidence. A `/1` join binds the whole
  repository and gets no anchor. `stepNegationJoinQualified` stays false.
- `LPCV-V0-941` records the `negate-provider-failed` refusal that the existing tests already assert.
- Evidence: `internal/testvaliditydoc`'s `TestStepNegationJoinNamesUnboundSupportFiles`. This test
  changes a helper under `/0`, then checks that the join is kept and the anchor is present.

## V1-2292: companionrelease and release-artifact-v0 accepted as unbounded-reader floor members

- Decision: these two packages are not split. Their bundle proofs need the real checkout, and the
  internals they read are unexported. Each entry in `.corvint/unbounded-readers.json` now carries a
  dated acceptance reason.
- Evidence: `make unbounded-readers-check` exit 0.

## V1-0585: OCM link replace-or-accumulate contract (OCM-V0-940)

- Decision: the default replace mode stays compatible when a re-link names the same single test
  path.
  - A re-link with a different path, which would erase claims at another path, is refused with
    `link-would-drop-claims`, and the message names those paths.
  - `corvint ocm link --accumulate` unions the hunk and claim IDs with the earlier link, sorted and
    checked against `maxReferences`. Every new claim still passes re-extraction and the exact-ID check.
  - To replace on purpose, `ocm mark` the obligation unknown first.
- Decision: `runOCMLinks` in the dogfood flow adds `--accumulate` to the second and later
  `DOGFOOD_OCM_LINKS` rows for the same obligation in a scope, so one plan can carry multi-file
  evidence.
- Structural linkage versus assertion coverage: an accumulated link proves that each named
  assertion exists and carries the exact requirement ID in an immutable blob. It does not prove that
  those assertions cover the whole requirement. That remains a review judgment.
- Evidence:
  - `internal/lrfrepo`'s `TestOCMV0940LinkAccumulatesClaimsFromTwoTestFiles` checks two files that
    both stay linked, the refusal leaving the map unchanged, a mismatched claim being refused, a
    valid verification and idempotence.
  - `internal/dogfoodflow`'s `TestOCMV0940LinkPlanAccumulatesRepeatedObligation`.
  - The existing OCM link tests also pass.

## V1-0583: delta DLT exact-ID anchors

- ALREADY-DELIVERED (AC1): on the base, every `DLT-V0-001`..`010` has exact-ID `t.Run` anchors on
  real assertions, as `git grep DLT-V0-0 -- internal/delta internal/cem/gitauth cmd/corvint/delta_test.go`
  shows. `DLT-V0-003` sits in `internal/cem/gitauth`, which is its owning package in the spec's
  traceability table: `bounded_blob_test.go` and `revision_fs_test.go`. The anchors arrived in the
  public snapshot `9477995d7`. No anchor was added or renamed.
- BLOCKED (AC2): docs-only and four-class acceptance stay PARTIAL until V1-0579 has proof or an owner
  disposition.
- BLOCKED (AC3): an independent review of the regenerated OCM and report set, plus strict
  completion, seal and gate, belong to the orchestrator's review and closeout, not to this lane.

## Friction

1. When load is above its threshold, `script/heavy.sh` waits up to 120 s by default and then runs
   anyway, so the wait is pure delay.
   - Repro: Mac load around 150, any `script/heavy.sh go test` without `HEAVY_LOAD_WAIT`.
2. Corvint hooks hit `dogfood-event-deadline` under host load.
3. The lane charter named area packages that do not exist (`cce`, `freshness`, `obligations`, `ocm`,
   `taskcontext`). Dispatch lives under `internal/tasks`.
4. The spec Intent-status header is frozen at 1000 characters, so acceptance text must go in the
   requirement note. The refusal message explains this.
   - Repro: append an acceptance to line 5 of `docs/specs/ocm-v0-dogfood.md`, then
     `make spec-requirements`.
5. `heavy.sh` offloads packages with darwin-only files to WSL even when `HEAVY_LOCAL_REASON` is set,
   and `HEAVY_REMOTE=0` is needed. The orchestrator then set `HEAVY_REMOTE_HOST=wsl-lanes` in
   `~/.config/corvint/heavy.env` and offload became the default, so a darwin-only flake (V1-2302)
   is still slow to run locally under lane load.
   - Repro: `script/heavy.sh go test ./internal/lrfrepo/`.
6. A hand-written OCM claim selector for a `t.Run` case fails `claim-selector-out-of-range` because
   selectors are normalized. Tests must derive selectors from `enumerateClaims` (related to V1-0275).

## Sub-lane A

Worktree /Users/russelllewis/projects/corvint-lanes/internal-mac-a, branch claude/lane-internal-mac-1031-a, base 8daec98ce4349c1583186692726d2c3a782ac647.
Tickets: V1-1961, V1-1962, V1-2504, V1-1389.

### Claimed requirement IDs
- TOL-V0-940, TOL-V0-941 (V1-1961)
- ATR-V0-940 (V1-2504, reserved)
- CAL-V0-940, CAL-V0-941 (V1-1389, reserved)

### V1-1961 — Delivers (spec 255d2ccc0, impl 2b00d9e55)
- AC1 (dogfood-check names unlinked requirement IDs): ALREADY-DELIVERED. internal/dogfoodflow/ocm_coverage.go `unlinkedNote` names each unlinked requirement ID (bound 32, OCM-V0-025); in 9477995d7 ("Corvint 1.0"), extended by 4a6e8b214 (V1-3092). Evidence: TestOCMV0025CheckNamesUnlinkedRequirementIDs PASS (heavy.sh, linux/amd64).
- AC2 decision (TOL-V0-940): `preflight --plan PATH --witnesses` reuses planWitnesses/reviewCandidate, so ledgerWitnesses, the UNASSIGNED `considered` member and STALE_REVIEW match `ticket obligations plan --witnesses`. STALE_REVIEW is a preflight finding on the plan path (refuses), with a warning. --witnesses without --plan is usage; --witnesses with --cache is usage (a reviewed witness is judged at HEAD/candidate branch, which the cache key does not bind). The key gains a `witnesses true` line only when set, so existing cache keys are unchanged. Without the flag, output is byte-identical.
- AC3 decision (TOL-V0-941): new store.TreePathsAtCommitUpTo stops reading `git ls-tree` and kills Git at the first kept path past the bound; the capability read uses it (bound 1,024). TreePathsAtCommit semantics are unchanged (shared streamTreeAtCommit).
- Measured on WSL (linux/amd64, heavy.sh, 5 runs each; this replaces the Mac numbers per the coordinator's 10-31 charter change). Store listing under big/, keep 1,024:

  | Paths | Unbounded (old capability loop) | Bounded |
  |---|---|---|
  | 1k | 1.25 ms | 1.22 ms |
  | 100k | 6.36 ms | 1.31 ms |
  | 1M | 53.9 ms | 1.34 ms |

  End-to-end CLI capability read at 1M paths, after the change: `--path big` 14.0 ms, `--path .` 11.6 ms, `--contains` 290 ms (SEARCH_BOUND_EXCEEDED abstention; blob search cost, not the listing). The before/after CLI comparison was taken only on the Mac under load 129-175 (163 -> 58 ms, `--path big`), and is superseded by the store-level WSL before/after.
- Integrated evidence NOT_RUN: a preflight --witnesses run on an adopter ledger with a test repository (needs an adopter repo).
- Tests (heavy.sh, linux/amd64), all PASS:
  - internal/tasks/cli: TestTOLV0940_PreflightPlanWitnessesMatchesPlan, TestTOLV0940_PreflightWitnessesUsage, TestTOLV0104..0108, all Preflight*/TOLV0025-0027/TOLV004x/TOLV005x (45 tests), TestCAPV0046HelpJSONIsTheTree, TestCAPV0042EveryTreeNodeAnswersHelp.
  - internal/tasks/store: TestTOLV0941_BoundedListingStopsAtTheBound plus the Preflight* store tests.
  - internal/dogfoodflow: TestOCMV0025CheckNamesUnlinkedRequirementIDs.
  - gofmt -l internal/tasks: clean.

### Friction
- F1: the help text and internal/capcatalog/trees/corvint-tasks.json drift silently until TestCAPV0046 runs. Any help edit needs a manual `make capability-catalog` (or `go run ./cmd/corvint-tasks help --json > ...`). Repro: edit a usage string in internal/tasks/cli/command_help.go, then run TestCAPV0046HelpJSONIsTheTree; it fails "stale". Expected: a pre-commit hint or a regeneration step in spec-requirements.
- F2: heavy.sh offloads ./internal/tasks/cli to wsl-lanes, which skips the 2 darwin-only files (doctor_probe_darwin.go). That is fine for these tests, but the note prints on every run; there is no way to say "darwin-only files are irrelevant to this -run" except HEAVY_REMOTE=0.
- F3: wire.Encode returns []byte, while test helpers mostly compare strings. One build round was lost on it. Minor.

### V1-1962 — Delivers (spec trace 366fafe95, fixtures 84e3b62e9)
- AC1: each failure mode now has a fixture that asserts its abstention code (internal/tasks/cli/obligation_abstention_test.go). No product code changed.
  - TestTOLV0101_SubjectAbsentUnreadableMismatchAtReview: SUBJECT_MISMATCH_AT_REVIEW, SUBJECT_UNREADABLE_AT_REVIEW (blob over 1 MiB) and SUBJECT_ABSENT_AT_REVIEW, each in both the boundary and the worklist reviewedSubject.
  - TestTOLV0108_CapabilityBlobOverBoundAndUnlistableBaseline: an over-bound blob gives SEARCH_BOUND_EXCEEDED, and is reported truncated beside a readable match. An unlistable baseline tree gives ENVIRONMENT_BASELINE_UNRESOLVED with baseline.commit null; it is made two ways, with the tree object deleted and with a path over 64 KiB. The test asserts the commit itself still resolves, so the abstention comes from the listing.
- Decision: a commit's content is fixed by its id, so these codes are reachable only through a damaged or foreign-rewritten object store. The fixtures overwrite a loose object file with another object's bytes. Git cat-file serves the substituted content under the original id without verifying it (checked on Mac git 2.54 and on the WSL runner). Replace refs are no route, because gitEnvironment sets GIT_NO_REPLACE_OBJECTS=1.
- AC2: NOT_PRODUCED, recorded in the spec traceability row. A live deployment-ref capability run and a non-browser adopter plan both need an adopter repository: one that declares an environment baseline a deployment moves, or a ledger planned with non-browser witnesses. Neither exists in reach.
- Tests (heavy.sh, linux/amd64): the 2 new tests plus TestTOLV0101_ReviewedSubjectAtReviewedCommit and TestTOLV0108_CapabilityBoundsAndReadOnly, all PASS.

### V1-2504 — ALREADY-DELIVERED (no commit)
- Ticket premise ("OpenCodeLineUsage returns input and output only") does not hold at base: internal/tasks/supervisor/opencode.go openCodeStepCounters sums tokens.input + cache.read + cache.write into input and output + reasoning into output; dispatch/usage.go (case "opencode") calls supervisor.OpenCodeLineUsage for both the CAL-V0-157 total and the ATR-V0-031 context estimate. Present in base commit 9477995d7 ("Corvint 1.0").
- Spec already states it: ATR-V0-031 "the estimate is that step's input (including cache read and write) plus its output (including reasoning)". No amendment needed; ATR-V0-940 reservation released unused.
- AC1 (does tokens.input include cache reads?): decided without a live run. OpenCode's session usage normalises the AI SDK's inputTokens net of cached input (it subtracts cache read/write before reporting tokens.input and reports cache.read/cache.write separately), so summing is the correct total, not a double count. This rests on OpenCode source as recalled, not re-read against 2.1.x; a live cached-provider run was NOT_RUN. It is moot for safety: if a future OpenCode version made tokens.input gross, the sum would over-count, which errs toward earlier hand-off (conservative), never toward a missed stop.
- AC2 path: "otherwise record the evidence and close" — fixture tests already exist with cache counters:
  - TestCALV0077_OpenCodeUsageObservedOrUnknown (supervisor): input 10+4+5+20+6, output 2+3+1; a missing cache member is UNKNOWN.
  - TestCALV0157_UsageVocabulariesAndStates (dispatch): "opencode stopped" 12/4 = 2 x (input 5 + cache.read 1) / 2 x output 2.
  - TestATRV0031_ContextEstimateFromRecognisedLinesOnly (dispatch): "opencode step" estimate 8 = 5 + 1 + 2; "latest step replaces" 503.
  - All PASS (heavy.sh, linux/amd64, at 84e3b62e9).

### V1-1389 — Delivers AC2 event (spec 18259ad26, impl 0be54f66c); AC1 + AC2 attestation ALREADY-DELIVERED; AC3 BLOCKED
- AC1, pressure re-sample: ALREADY-DELIVERED as the spec's alternative ("or the spec records why it is unnecessary"). The "V1-1389 rationale" paragraph in docs/specs/corvint-tasks-agent-leases-v0.md (CAL-V0-225..229 section, base 8daec98ce) explains why. A recovery launches nothing. A restart keeps the recorded pressure level and is UNKNOWN until its own first sample (CAL-V0-068). So a sample taken by the recovery would be staler than the one the start takes anyway.
- AC2, attestation as a journal entry: ALREADY-DELIVERED. The same rationale records why a state-directory record suffices: no Tasks authority, the program lock fences, and a journal write would make recovery depend on a writable store.
- AC2, observable event: DELIVERED as CAL-V0-940. Decision: recover-reader does not write events.jsonl or state.json. The event sequence is owned by the dispatcher ledger, and the ledger may be drained or handed over and not ours to rewrite. Instead:
  - Before removing the marker, recover-reader hard-links the record into reader-recoveries/unannounced/NAME (fsynced; an existing link is kept, so a replay links nothing twice).
  - The next Open, under the program lock and after the quarantine check, emits `reader-recovered` (kind OPERATOR, detail kind/run/requestId/recordedAt) in name order, at most 64 per start.
  - Each link is removed only after its event line is appended (at least once). An unreadable linked record gets an `alert` and stays linked.
  - Old records have no link and are never announced. The help text names the event.
- AC3: BLOCKED (external event).
  - A "real dispatcher run on this host" exercising HOLD, refused start, recover-reader, then start needs a live dispatcher program with a real store and agent host. Its hermetic equivalent already runs as TestCALV0053_DispatchReaderCLICrashRetainsQuarantine plus TestCALV0226_DispatchRecoverReaderOperator (PASS).
  - Linux boot identity: TestCALV0227_DispatchRecoverReaderByReboot ran on WSL linux/amd64 and PASSED, not skipped. It used the real /etc/machine-id and /proc/sys/kernel/random/boot_id through ReaderHostIdentity, so the Linux identity read and the auto-recovery path executed on Linux with a simulated earlier boot marker.
  - An actual reboot-driven recovery needs a host reboot: BLOCKED.
- Tests (heavy.sh, linux/amd64), all PASS:
  - internal/tasks/dispatch: TestCALV0940_OperatorRecoveryAnnouncedAtNextStart and TestCALV0940_UnreadableLinkedRecordAlertsAndStays (both new), plus the -run set TestCALV0226|0227|0229|0053|0058|0131.
  - internal/tasks/cli: TestCALV0226_DispatchRecoverReaderOperator (extended: exactly one OPERATOR event after the start), TestCALV0228_DispatchHelpNamesRecovery (extended: names reader-recovered), TestCALV0227_DispatchRecoverReaderByReboot, TestCALV0226_DispatchRecoverReaderLiveEvidence, TestCALV0053_DispatchReaderCLI* (4), TestCALV0131_* (5), TestCAPV0046 and TestCAPV0042.
  - The whole dispatch package was not run. Corvint selection is FULL (whole: BINARY_EXEC via capcatalog/command_help.go), so focused tests only; batch CI covers the rest.
- Finding (for filing, out of scope): standaloneStateCommand (internal/tasks/dispatch/reader.go ~472, program work-state-reader) applies the HOST_REBOOT proof without emitting any event. No dispatcher start ever announces it. The spec non-goal now names this.

### Friction (cont.)
- F4: `make spec-requirements` refuses an edit to a spec's frozen Status/Delivery-status header (SRG-V0-980). The message is clear, but the rule is not in the spec template's visible text. Repro: append a requirement mention to line 6 of docs/specs/corvint-tasks-agent-leases-v0.md, stage it, then run make spec-requirements; it exits 255. Expected: none (working as designed); one lost round.
- F5: `corvint test --dry-run` selects the whole repository for any capcatalog/command_help.go touch (BINARY_EXEC via the help tree). A help-text sentence therefore forces FULL selection.

## Sub-lane B

Worktree /Users/russelllewis/projects/corvint-lanes/internal-mac-b, branch claude/lane-internal-mac-1031-b, base origin/main 8daec98ce4349c1583186692726d2c3a782ac647.
Tickets: V1-2620, V1-2732, V1-2302, V1-0805.

### Claimed requirement IDs
- CCE-V0-940 (V1-2620)
- PMR-V0-940..PMR-V0-943 (V1-0805; reserved, final list below)

### V1-2620 — cce-live INCONCLUSIVE when one control attempt is lost to infrastructure

Delivers AC1; AC2 BLOCKED.

- Prior state on base: CCE-V0-116 (earlier V1-2620 lane) records each lost attempt's cause and PWP-V3-010
  fixed the `qualified-encoding-refused` cause of recurrences 2 and 3. Nothing retried a lost control,
  so any single hosted-runner loss still read `INCONCLUSIVE (control-not-killed:...)`.
- Decision: retry in the shared prove path (`proveScenario` -> `rerunLostControls`, internal/changee2e/prove.go),
  not in the live tests, so every live test (TestCCEV0009LivePreserves, TestCCEV0053LiveRecordsCoverage, ...)
  and real `e2e prove` users get it. Bound: one rerun, of only the lost controls, through a fresh BBF plan
  and workspace (`runControls` gained an `only` control-ID filter). Teardown-attributed losses
  (CCE-V0-087, a located test-tree failure) and killed/survived/invalid/not-run controls never rerun.
  The first run's losses move to the additive `retried_lost_attempts` field; the verdict note is unchanged
  (a second loss still reads `control-not-killed:<id>`), so CCE-V0-087/116 contracts are untouched.
  Rejected: changing the note to a distinct `control-lost-to-infrastructure` reason (would amend 087/116
  and every consumer of the note) and skipping in the live tests (would hide product regressions).
- Spec: CCE-V0-940 in docs/specs/change-compiled-e2e-v0.md (commit 36a37014d), traceability row added.
- Test: `TestCCEV0940LostControlRerunOnce` (6 cases: lost then killed -> DISCRIMINATES; lost twice ->
  INCONCLUSIVE control-not-killed with both losses; lost then survived -> CONTROL_SURVIVED; no loss -> no
  rerun; teardown-attributed -> no rerun; survivor beside loss -> only the loss reruns). Mutation check:
  with the rerun call removed, 4 of 6 subtests FAIL; restored, all PASS.
  `proofLosses` (live failure printer) also prints first-run losses.
- Runs: focused `-run TestCCEV0940|TestCCEV0116|TestCCEV0087|Offline` PASS on Mac (23.7 s);
  whole package PASS (heavy.sh routed it to PC linux/amd64 go1.27.3, 42.2 s). Live tests skip here
  (no qualified Playwright tuple).
- BLOCKED AC2: "cce-live passes 5 consecutive runs on hosted macos-15, or the residual rate is recorded"
  needs hosted CI runs after merge; NOT_RUN.

### V1-2732 — suspected flake, internal/changee2e (WSL)

Outcome: NOT REPRODUCED; closes on the 20-run evidence (AC2). No code change.
- `script/heavy.sh env GOTOOLCHAIN=local go test -count=10 -v -timeout 580s ./internal/changee2e/`, twice, at
  ea2ea534a (base 8daec98 + the V1-2620 rerun change; changee2e is the package under test). heavy.sh ran both on
  PC WSL linux/amd64 go1.27.3, which is the ticket's own host, so no substitute host was needed:
  batch 1 ok 444.9 s (20:44 EDT), batch 2 ok 522.8 s (21:02-21:11 EDT; WSL load average after the run
  58.6 / 33.9 / 30.8). 20/20 package runs passed, 0 `--- FAIL`, 1090 `--- PASS` lines in batch 1 (60 SKIP: live tests
  without a Playwright tuple). Full -v output: scratchpad b-2732-run1.txt and b-2732-run3.txt.
- An extra attempt (b-2732-run2.txt) ran locally on the Mac (heavy.sh did not offload because the command
  was wrapped in `sh -c`), Mac load 30-120: about 2.5 iterations passed (267 PASS) before my own 580 s
  `-timeout` fired in TestCCEV0019RemovedAtHeadUnknown (1 s in): this was the budget, not a test failure.
- Inference (not proved): a whole-package go test with `-timeout` near the run time under host load
  reads as `FAIL` with only a panic header; the original run printed FAIL after 24.2 s, which is
  shorter than any timeout, so this is not that cause. The original cause stays unknown.

### V1-2302 — suspected flake, internal/postmergeworkflow

Outcome: NOT REPRODUCED; closes with the measurement (AC). No code change.
- WSL: `go test -count=20 -v -timeout 580s ./internal/postmergeworkflow/` via heavy.sh -> PC linux/amd64
  go1.27.3, ok 377.1 s (21:11-21:18 EDT), 640 PASS, 0 FAIL, 0 SKIP; WSL load average afterwards 20.5 / 37.8 / 35.1.
  Single run baseline: 20.5 s.
- Mac (darwin-only path native_open_darwin.go; HEAVY_REMOTE=0): `-count=8 -v -timeout 570s`, darwin/arm64
  go1.27.3 (homebrew), Mac load 12 at start, 78 at end: 5 full iterations plus part of a 6th passed (201 PASS,
  0 test FAIL) before my own 570 s timeout fired in TestNativeAdmissionDoesNotEchoInvalidIdentity (7 s in).
  Under load single tests reached 30 s (TestCLIReplay 30.1 s, TestNativeRequiredNestedAdmission 21.9 s); a full
  run took about 90 s against 20 s on WSL.
- Output: scratchpad b-2302-wsl.txt, b-2302-mac.txt.


### V1-0805 — post-merge workflow: author and draft replay, historical set, qualification campaign

Delivers AC1-AC3; independent review of AC1 is left to the orchestrator (no codex in sub-lanes).
Commits: 9018ce85c (spec PMR-V0-940..943 + regenerated REQUIREMENTS.tsv), 2e2393ca1 (author/draft
replay), 589c1f2f3 (historical set + campaign), 5adaedc43 (user guide).

- Decision (AC1): the harness, not the adapter, owns the author and draft stages. An adapter returns
  `authored` content (closed `Authored{kind, files}`; at most one docs and one tests entry, 1-64 unique
  files, <= 1 MiB each). Docs files must be the adapter's own documentation targets; tests files must be
  test paths (_test.go, .test.mjs, .test.js, _test.sh, testdata/) and need reported test gaps; anything
  else is `authored-scope-invalid`. With the host policy's new `draft_url` (else `draft-url-unpinned`)
  the harness commits each entry in `scratch/authored` (`git init`, objects/info/alternates -> the
  product's common objects dir; read-tree merge + hash-object + update-index + write-tree + commit-tree,
  parent = merge, fixed identity corvint-replay, date = merged_at, so both runs author identical SHAs)
  and passes connector drafts on that private root to Build / ValidatePlan / Record. The ledger records
  `upsert-draft-change` requests; nothing pushes, opens or writes a product ref/object/index/worktree.
  Adapter-supplied connector drafts still block (`draft-scope-validation-unavailable`, PMR-V0-005).
  `replayed_stages` lists `<kind>-author` and `draft-requests` (observed with digests); they leave
  `deferred_stages`; scope/validation stay deferred. New limits: authored-content-scope-validation-not-verified,
  draft-requests-recorded-not-performed, alternate-object-mtime-may-be-freshened. workflow_qualification
  stays NOT_OBSERVED. Rejected: letting the adapter return draft revisions (it cannot be bound to the
  product without a write) and writing a product ref (violates the read-only replay).
- Decision (AC2): historical set = 7 real commit-level changes on origin/main (all ancestors of 8daec98ce),
  internal/postmergeworkflow/testdata/historical/manifest.json: 4 changes with a later same-ticket
  follow-up (V1-2415, V1-2513, V1-3383, V1-1269) and 3 controls with none (V1-2741, PRS-V1-840, V1-2559).
  Labels are basis `generated` with an evidence digest of (id, base, change, follow-up, provenance);
  label definitions, provenance text and uncertainty (V1-3383 follow-up may be same-PR; V1-1269 defect
  has no line anchor) are in the manifest. No human-verified labels were claimed.
- AC3 campaign: `TestHistoricalReplayCampaign` replays each entry through the real Replay path against a
  `git clone --shared --no-checkout` of the checkout (the developer worktree is never the product root)
  with the changed-path reference adapter (historicalAdapter: source dirs = flows; source dir without a
  changed test = gap; authors the follow-up's test files as the tests draft). Missing commits -> NOT_RUN
  per entry; shallow checkout or no Git -> t.Skip NOT_RUN; never MATCH. Outcomes are asserted equal to
  the manifest's `recorded` block.
  Result (WSL linux/amd64 go1.27.3, 2026-10-31): 7 of 7 ran, 0 NOT_RUN, 0 BLOCKED.
  | entry | status | mismatch fields | draft requests |
  |---|---|---|---|
  | V1-2415 | MATCH | - | 1 (tests draft from 310eeed15) |
  | V1-2513 | MISMATCH | affected_flows, followup, test_gaps | 0 |
  | V1-3383 | MISMATCH | followup, test_gaps | 0 |
  | V1-2741 (control) | MATCH | - | 0 |
  | PRS-V1-840 (control) | MATCH | - | 0 |
  | V1-2559 (control) | MATCH | - | 0 |
  | V1-1269 | MISMATCH | affected_flows, followup | 0 |
  Follow-up recall 1/4, controls 3/3. Mismatch causes: the rule ignores non-code files (V1-2513 JSON
  config needed an integrations test; V1-1269 docs dirs), a test-only change cannot predict the test that
  runs it (V1-3383), and a same-directory test change hides a later defect fix (V1-1269). MATCH qualifies
  only the harness and this rule, not the product workflow.
- Tests (all via script/heavy.sh -> PC WSL linux/amd64 go1.27.3):
  - focused `-run 'TestReplay|TestCLI|TestClosed|TestPins|TestHuman'` PASS 14.9 s (incl.
    TestReplayAuthorAndDraftStages 2.3 s, TestReplayAuthoredBlocks 1.3 s, 6 BLOCKED cases).
  - `-run 'TestHistoricalReplayCampaign|TestReplayAuthor' -v` PASS 11.0 s (campaign 7.5 s).
  - mutation: historicalAdapter gap rule `!tested[dir]` -> `true`: campaign FAILS on 5 of 7 entries; restored.
  - whole package `./internal/postmergeworkflow/` (HEAVY_WHOLE_REASON) ok 30.7 s at 589c1f2f3.
  - darwin build: `go vet ./internal/postmergeworkflow/` on the Mac clean (native_open_darwin.go is not
    built on WSL).
- Not done here: independent review (orchestrator), dogfood/CEM, doc-gates (lane owner after merge).

### Friction (with repro)

1. heavy.sh refuses a whole-package `go test` without HEAVY_WHOLE_REASON even for one small owning
   package; the reason has to be invented from `corvint test --dry-run`. Repro: `script/heavy.sh env
   GOTOOLCHAIN=local go test -count=1 ./internal/postmergeworkflow/` -> exit 2.
2. Wrapping the command in `sh -c '...'` silently defeats the WSL offload (runs on the Mac). Repro:
   `script/heavy.sh sh -c 'go test -count=10 ./internal/changee2e/'` -> runs locally, no offload line.
3. Installed corvint reports DEGRADED `self-binary-stale` (build dbc17137) in this worktree.
4. `corvint test --base 8daec98ce --dry-run` after a spec-only commit selected 99 units (wide), so it
   gave no useful narrowing for a one-package change.
5. heavy.sh offload skips darwin-only files (internal/postmergeworkflow/native_open_darwin.go): a
   darwin-only flake (V1-2302) needs HEAVY_REMOTE=0 plus HEAVY_LOCAL_REASON, and the Mac run is slow
   under lane load (about 90 s per package run vs 20 s on WSL).
6. docs/specs/README.md: the requirement range is regenerated by `make spec-requirements` but the summary
   column is hand-written, so it is easy to leave stale after adding requirements.
7. postmergeworkflow / postmergeconnector identifiers forbid "/" (`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`),
   so directory-shaped flow and gap IDs must be re-encoded (":"); no shared helper exists.

## Sub-lane C

Branch claude/lane-internal-mac-1031-c, base 8daec98ce4349c1583186692726d2c3a782ac647.
Claimed requirement IDs: DIRTY-CACHE-940, DIRTY-CACHE-941, DIRTY-CACHE-942 (V1-0418; grepped free on origin/main and internal-mac* worktrees 2026-10-31)

### V1-2612 — NOT REPRODUCED (misdiagnosis), regression test added

- Original receipt /Users/russelllewis/projects/VanillaTwisted/.agent-evidence/obedience-audit-2026-10-11/corvint-context.json:
  `context.revision` = `context.freshness.revision` = c7e2140df8ea18a5708a8780ca055522f634e794,
  `learning.history_tip` = a9e90f93b418cbc5599da1342c572d66003b3688.
- In VanillaTwisted: `git cat-file -t c7e2140df8ea...` = `tree`; `git rev-parse a9e90f93^{tree}` = c7e2140df8ea18a5708a8780ca055522f634e794.
  So c7e2140 is the tree of live HEAD a9e90f93, not a stale captured base. The packet's `revision` is the
  HEAD tree id by contract (`Index.Revision = identity.treeRevision`, internal/contextindex/index.go;
  receipt.go writes it to `revision` and `freshness.revision`); `history_tip` is the HEAD commit.
- Reproduction on current main: `go build -o $S/corvint ./cmd/corvint` at 8daec98ce (Corvint 1.0.0-rc.3.dev,
  commit 8daec98ce4349c1583186692726d2c3a782ac647). Scratch repo: commit A, `corvint query --task "alpha docs"`
  (revision = A^{tree} 3a58c5b, fresh), then commit B, dirty README.md, rerun: revision = freshness.revision =
  B^{tree} 5c7391b, state mixed-worktree, history_tip = B 9a70cf8. `corvint context --task` likewise reports B^{tree}.
  No stale base.
- Regression test: cmd/corvint/query_repository_test.go TestRepositoryQueryMixedWorktreeBindsLiveHeadAfterHeadMoves
  (query at HEAD, commit to move HEAD, dirty a file, query again: revision == HEAD^{tree}, history_tip == HEAD, mixed-worktree). PASS.
- Friction/idea (not filed by me, no Tasks writes): the packet field named `revision` carries a tree id beside a
  commit id in `history_tip`; a reader (an agent auditing VT) misread it as a stale commit. Repro: any `corvint query`
  receipt; `git cat-file -t <revision>` prints `tree`. Suggest a `tree`/`commit` label in a later packet schema.

### V1-2350 — ALREADY-DELIVERED (by V1-2434 / GH #1554, DCP-V1-314..317), measured

- AC1 (skip checkout observation and pin verification while the observed checkout is unchanged, drift
  detection kept): delivered on base 8daec98ce by the corpus MCP checkout stamp. DCP-V1-314 (lstat stamp of
  worktree + repository dir, racy-window and unavailable cases never match), DCP-V1-315 (equal stamp reuses the
  freshness overlay without observing any checkout), DCP-V1-316 (pin verification skipped while stamp + pin set
  equal; amendment V1-2542/2544 keys on the ordered pin set too). Code: internal/mcp/corpusbridge/stamp.go,
  corpusbridge.go (artifactFor pinReuseKey, freshness live.stamp). Drift tests: TestDCPV1314StampChangesWithCheckoutState,
  TestDCPV1314StampUnavailableOutsideCoveredState, TestDCPV1314StampRefusesRacyEntries, TestDCPV1315WarmCallsSkipGitWhileStampUnchanged,
  TestDCPV1143LazyLoadAndMemo, TestDCPV1280AdditionalRepositoryFreshnessReused (one-file change recomputes, names the repo).
  All PASS on this branch (see the WSL run below).
- AC2 measured before/after on the 8,001-input fixture (multiRepositoryFixture [4000,3000,1000] = 4,000 primary + 3,000 + 1,000
  additional + 1 provider input), with a scratch in-package test (TestZZV12350Measure, not committed), 40 warm calls each
  (docs_get/docs_search alternating). Run on WSL per the coordinator's charter change (script/heavy.sh offload to wsl-lanes,
  PC linux/amd64 go1.27.3, slot 3/8, at 2be87cd5b; Mac numbers from the overloaded host were discarded). The Linux run exercises
  stamp_linux.go/stamp_unix.go; stamp_darwin.go differs only in the Stat_t field accessor.
  | mode | freshness_ms median/p95 | verify_ms median/p95 | total_ms median/p95 (max) |
  |---|---|---|---|
  | before: stamp forced unavailable (stampRacyWindow=1<<62; observes + verifies every call, the pre-V1-2434 path) round 1 | 21 / 23 | 5 / 6 | 54 / 63 (63) |
  | before, round 2 | 21 / 24 | 5 / 6 | 51 / 62 (62) |
  | after: stamp available (stampRacyWindow=0) round 1 | 0 / 0 | 19 / 21 | 34 / 39 (46) |
  | after, round 2 | 0 / 0 | 18 / 20 | 33 / 38 (44) |
  Warm p95 total 38-39 ms after, 62-63 ms before, both below 218 ms. verify_ms after is the stamp capture only (DCP-V1-316 reports
  stamp + any verification); on Linux the stamp lstat walk (~19 ms) costs more than pin verification (~5 ms), so the saving is the
  removed checkout observation (~21 ms freshness). First (cold) call ~1.9-2.0 s in every mode.
  The DCP-V1-317 17,001-input fixture (TestDCPV1281AdditionalRepositoryWarmCallsAtScale) on WSL: warm total_ms sorted
  [54 61 62 65 67 68 73 74 76 76 80 84 85 89 90 96 114 120 144 190], p95 190 ms (bound 1 s). PASS.
  Tests on WSL: go test -count=1 -run 'TestDCPV1315|TestDCPV1314|TestDCPV1281|TestDCPV1280|TestDCPV1143LazyLoadAndMemo'
  ./internal/mcp/corpusbridge: 12 tests PASS, 16.1 s.
- No code change. No timing assertion at 218 ms added: the spec bound is 1 s and a tighter wall-clock bound flakes on loaded hosts.

### V1-0418 — Delivers (spec 9f3b6f59a, implementation a0a441a52)

Spec: docs/specs/revision-cache-dirty-worktree-v0.md DIRTY-CACHE-940 (opt-in `--worktree-overlay` on `corvint query` and
`corvint context`; HEAD snapshot on a hit else a full in-memory HEAD build; re-derive only tracked dirty paths through
forbiddenPath + admitted-suffix + no-follow bounded regular-file read; equal-to-HEAD content keeps the HEAD source; deleted /
unreadable / irregular / oversize / generated / LFS dropped with a named exclusion; untracked never read; nothing written),
DIRTY-CACHE-941 (worktree Git blob id on overlaid rows, `revision`/`commit` stay HEAD, query freshness stays mixed-worktree with
`scope=git+worktree-overlay` and `overlay_path_count`, context coverage uncertainty line names the overlay count), DIRTY-CACHE-942
(overlaid context rows/spans: authority `worktree-overlay`, trust tool-output, confidence low, except reserved rows the authority
screen already downgraded; query results: `advisory: true`, `worktree_overlay: true`, evidence authority worktree-overlay /
confidence low, never critical, one coverage uncertainty line). DIRTY-CACHE-010 amended to name the opt-in exception.
Accepted under the owner's standing auto-accept (2026-10-11).

AC mapping:
- AC1 (reuse HEAD snapshot, re-derive only dirty paths, dirty rows marked by worktree blob id): contextindex.OverlayWorktree
  attaches the HEAD index as the IDX-SNAP-V0-037 reuse prior, so every unchanged source keeps its per-source facts and term
  postings (measured reused = sources - dirty, below); overlaid evidence cites `git hash-object` of the worktree bytes.
  Tests TestOverlayWorktreeRederivesOnlyDirtyTrackedPaths, TestOverlayWorktreeDropsDeletedAndUnreadablePaths,
  TestOverlayWorktreeWritesNothing, TestQueryWorktreeOverlayCLI, TestContextWorktreeOverlayCLI.
- AC2 (latency 1/10/100 dirty vs full rebuild, retained): table below.
- AC3 (dirty rows never carry authoritative confidence from committed authority): trust table maps worktree-overlay to
  tool-output (tainted, so it satisfies no governance/critical/basis selector); a dirty AGENTS.md keeps its self-modified
  downgrade. TestWorktreeOverlayRowsAreNeverAuthority, TestQueryWorktreeOverlayCLI.

Decisions (mine, recorded):
- Opt-in flag, not default: the flag is the operator's consent to read uncommitted bytes; default packets are byte-identical
  (no new fields), so batch, MCP (incl. internal/mcp/corpusbridge), hooks and every frozen receipt are unchanged.
- Distinct freshness scope `git+worktree-overlay` (not DIRTY-CACHE-012's `git+working-tree`), explicit `worktree-overlay`
  authority mapped to tool-output in the closed trust table.
- Untracked paths are never overlaid (secret/identity exposure; no HEAD prior). Deleted or unreadable dirty paths are dropped,
  never served stale from HEAD.
- The view shares the base's mapping lease and bodies; cmd does not release early because a receipt may alias mapped bytes
  until emitted (same as every other cmd reader).
- Query READY/abstain state is computed before marking; only advisory flags, evidence labels, critical filtering and the
  uncertainty line change. `context --worktree-overlay --expand` is refused (expand reads Git objects by handle).
- On a snapshot miss the overlay builds HEAD with contextindex.Build (query) / BuildContextObserved (context) and prints the
  usual miss notice; without priorFacts the view falls back to a full compile.
- Whole-tree derived tables (compileTables, symbol windows, ident graph, documents/profile) are recompiled over the view, as in
  IDX-SNAP-V0-037's re-index; only per-source facts and postings are reused. This is why the overlay is not faster (below).

Measured latency (AC2), WSL per the charter change: script/heavy.sh offload to wsl-lanes, PC linux/amd64 go1.27.3,
GOMAXPROCS=32, at 9f3b6f59a+wip (the a0a441a52 tree). Scratch in-package test (TestScratchOverlayMeasure, deleted, not
committed): `git clone --shared` of this repository (tree d7bb2152, 9,022 sources), BuildForSnapshot + WriteSnapshot, then for
N in 1/10/100 dirty tracked internal/**/*.go files (stride over 2,156 candidates, appended a func): full = contextindex.Build of
the dirty clone; overlay = LoadContextSnapshot (hit) + OverlayWorktree; best of 3 each. Two runs:
| dirty | overlaid | reused sources | full rebuild ms | snapshot load ms | overlay ms | load+overlay ms |
|---|---|---|---|---|---|---|
| 1 (run 1) | 1 | 9,021 | 1,367 | 551 | 853 | 1,405 |
| 10 (run 1) | 10 | 9,012 | 1,553 | 543 | 997 | 1,541 |
| 100 (run 1) | 100 | 8,922 | 1,552 | 543 | 924 | 1,467 |
| 1 (run 2) | 1 | 9,021 | 1,803 | 663 | 1,150 | 1,814 |
| 10 (run 2) | 10 | 9,012 | 1,467 | 544 | 912 | 1,456 |
| 100 (run 2) | 100 | 8,922 | 1,219 | 398 | 969 | 1,367 |
Overlay compile phases (run 2, ms): priorFacts+attach 28-66, vocabulary (reused) 45-48, compileTables 413-479, symbol windows
306-415, ident graph 139-205.
Reading: per-source work is reused (reused = 9,022 - N; cost flat from 1 to 100 dirty), but load+overlay is at parity with a
full in-memory rebuild on this 32-core host (~1.4-1.5 s each), because the whole-tree tables above (~0.9 s) are recompiled.
The overlay's value delivered here is correctness (dirty bytes become cited, low-confidence evidence; a full rebuild reads
HEAD and never sees them), not speed. Follow-up for the orchestrator to file (I make no Tasks writes): optimizations — patch
symbol windows / ident graph / compileTables incrementally for overlay views (postings are keyed by global symbol position,
so this needs a remap), target overlay << snapshot load.

Tests run (all via script/heavy.sh on wsl-lanes, linux/amd64 go1.27.3):
- go test -run 'TestOverlayWorktree|TestWorktreeOverlay' ./internal/contextindex: PASS (4 tests).
- go test ./internal/contextindex whole (HEAVY_WHOLE_REASON: `corvint test --base 8daec98ce --dry-run --summary` selected it
  whole, NARROWED 231 units, go whole=229): PASS 57.4 s after the analyzer-schema repin (first run failed only
  TestAnalyzerSchemaInputs, expected input drift).
- go test -v -run 'TestWorktreeOverlayFlagParses|TestQueryWorktreeOverlayCLI|TestContextWorktreeOverlayCLI|TestParseTaskContextInvocation|TestRepositoryQueryMixedWorktree' ./cmd/corvint: 6 PASS.
- go test -run 'Query|TaskContext|Context|Help|WorktreeOverlay|Parse' ./cmd/corvint: first run FAIL TestMCPV0100/0105 (catalog
  drift from the new help text) -> `make capability-catalog` -> rerun with '^TestCAPV0|Help|Query|TaskContext|Context|WorktreeOverlay|Parse':
  PASS ./cmd/corvint 44.8 s, ./internal/capcatalog, ./tools/capabilities-check, ./internal/mcp/bridge.
- Local (cheap, not go test): go build/vet ./cmd/corvint ./internal/contextindex clean; gofmt -l clean;
  `go run ./conformance/use-cases-v0` valid:true after `script/analyzer-schema.sh bump --base 8daec98ce` (kept schema
  corvint-analyzer/150 since output digest unchanged; repinned 3 use-case receipts for the main.go edit).
- Not run (lane owner runs once after merge): dogfood-change/closeout, make doc-gates, windows-vet, exhaustive gate.

Friction (repros):
1. `make spec-requirements` refuses new text in a spec's frozen header "Intent status" / digest "Status" (SRG-V0-980), with a
   message naming the requirement but not saying where acceptance must go instead. Repro: add "DIRTY-CACHE-940" to the
   Intent status line of docs/specs/revision-cache-dirty-worktree-v0.md, stage, `make spec-requirements`.
2. A bold `**accepted under ...**` note leaves REQUIREMENTS.tsv acceptance "unrecorded"; only the leading parenthetical
   `(accepted under the owner's standing auto-accept (2026-10-11); V1-xxxx)` on the definition line records `standing`.
   Undocumented in the refusal. Repro: write the note in bold, `make spec-requirements`, grep the ID in REQUIREMENTS.tsv.
3. heavy.sh refuses a whole-package run without HEAVY_WHOLE_REASON even when `corvint test --dry-run` selects every go unit
   whole (229/229 here); the operator must copy the selection verdict by hand. Repro: edit internal/contextindex,
   `script/heavy.sh go test ./internal/contextindex`.
4. `corvint test --dry-run` on PATH reports DEGRADED self-binary-stale (built from dbc1713779fc; 3 Go paths differ at HEAD).
   Repro: `corvint test --base 8daec98ce... --dry-run --summary` in this worktree.
5. Adding one help line to `query`/`context` fails TestMCPV0100/0105 in cmd/corvint until `make capability-catalog`
   regenerates internal/capcatalog/catalog.json; the failure message ("differs from corvint help --json") does not name
   the regeneration target. Repro: add a flag line to queryHelpFormat, run those tests.
6. Any internal/contextindex non-test edit requires `script/analyzer-schema.sh bump` (input digest pin), found only by the
   whole-package run, not by focused -run tests. Repro: edit internal/contextindex/trust.go, run TestAnalyzerSchemaInputs.
