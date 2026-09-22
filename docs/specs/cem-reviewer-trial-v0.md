# CEM Reviewer Trial V0

Owner: Russell Lewis
Date: 2026-09-03
Requirement prefix: `CRT-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: the V4 status record ("Run 30 Beamfall CEM lanes versus 30 controls. Require at
least 20% fewer missed-evidence findings, at least 70% citable material hunks, and under 5% incorrect
verifier hard failures"), `benchmarks/README.md` (partitions and the first-observation rule),
`docs/specs/confidently-wrong-trial-v0.md` (arms, dispatch, canonical output, reply grammar),
`docs/CHANGE-EVIDENCE-MAP.md` and `internal/cem` (the cem/0.1 workflow under test), `AGENTS.md`
invariants 2 and 4.

## Agent digest
- Claim: `tools/cem-trial` runs one agent over frozen Beamfall changes under `control` and `treatment` arms and scores withheld test-or-spec evidence per arm.
- Status: proposed/experimental
- Exists: `tools/cem-trial` (`select`, `run`, `score`), the five-change pilot manifest under `tools/cem-trial/testdata/pilot`, and `tools/cem-trial/testdata/fake-agent.sh`.
- Blocked on: the 30-pair held-out estimation run; the real-agent pilot is run and valid (2026-09-04).
- Read next: Requirements (selection, withheld patch, arms, metrics); Power; Non-goals; Stopping and publication.

## Intent and scope

The V4 status record names a numeric gate over 30 Beamfall CEM lanes against 30 controls, and nothing
in the tree runs it: `tools/cw-trial` measures confidently-wrong claims over retrieval and change
tasks, not evidence citation over a change under review, and `benchmarks/dogfood_measure.py` leaves
outcome metrics `NOT_OBSERVED`. This slice is that dispatcher plus its scorer.

Affected user: the repository owner reading V4's CEM gate. Measurable job: for one frozen set of
Beamfall changes, produce the mean fraction of the change's own withheld test-or-spec evidence the
agent failed to cite, per arm, with a paired 95% interval; the fraction of material hunks the
treatment arm's citations left supported; and the rate at which `cem verify` hard-failed a citation
an independent replay says was valid.

Definition encoded: a **missed-evidence finding** is a gold path the agent did not cite. Gold is the
set of test-or-spec files the change itself touched, withheld from the presented patch. The agent
sees the change's source hunks and the repository at the base revision; the evidence it is asked to
name is exactly what the change's author also wrote and the harness removed.

## Requirements

- `CRT-V0-001`: Unit and selection. One change is one non-merge Beamfall commit `C`, and its lane
  runs at `C^`. The selection rule is frozen verbatim in the manifest (`selection_rule`): non-merge
  commits since `--since`, oldest first, touching between 3 and 12 text files, with at least one
  modified non-test source file (`.go|.ts|.tsx|.swift|.py`) and at least one test-or-spec file
  (`_test.go`, `*.test.ts`, `*.test.tsx`, `*Tests.swift`, `_test.py`, `docs/specs/**`); the presented
  patch is the source-file hunks alone, at most 64 KiB, over which `cem begin` parses between 3 and
  20 hunks; a change is rejected when any source hunk contains a gold path literal; candidates are
  ordered oldest-first and selected at index `floor(i*N/n)`. The manifest records the population `N`,
  the rule string, every commit SHA, and each change's patch sha256; `run` recomputes those digests
  and refuses the manifest on mismatch, and refuses a manifest that lists one change id twice.
- `CRT-V0-002`: Pairing. Every change runs in both arms; the estimate is within-change. The arm order
  per change is a seeded shuffle over `--seed`, recorded on every lane as `arm_order`. Each arm runs
  in its own disjoint clone and its own agent session; no state crosses arms.
- `CRT-V0-003`: Withheld-patch construction. The presented patch `P(C)` contains only the source-file
  hunks of `C`. The test-or-spec hunks are withheld and become gold. Both arms receive byte-identical
  `P(C)`, its sha256, and a harness-generated numbered hunk list in the order `cem begin` reads them.
- `CRT-V0-004`: Ground truth. Primary gold for `C` is the set of test-or-spec paths `C` itself
  touched; secondary gold is the one-based line spans of those files' added lines, read from `C`'s own
  diff. Non-circularity: a change is rejected at selection when any source hunk contains a gold path
  literal. The known gameability is stem-convention guessing; the mitigation is a mechanical
  stem-baseline recorded per change at selection time, whose miss rate the report carries beside both
  arms so treatment is read as lift over that floor, never over zero.
- `CRT-V0-005`: Arms. `control` receives the prompt skeleton, `P(C)`, the numbered hunk list, and a
  read-only clone at `C^` with its history. `treatment` receives an identical prompt, model, effort,
  timeout, tool access, and clone, plus exactly three artefacts: the cem/0.1 map from
  `corvint cem begin --patch P(C)`, the `cem status` worklist, and `cem report`. Leak control: the
  map MUST be built with `cem begin` over the withheld bytes and never with
  `cem prepare --base C^ --target C`; the clone is scrubbed of `.corvint/` and holds no commit at or
  after `C` (a fetch of exactly `C^`, detached, never a shared clone of the source); the harness
  asserts `git cat-file -e C` fails in the clone and errors the change's lanes if it does not. The
  control prompt never names Corvint, CEM, or a map.
- `CRT-V0-006`: Reply grammar. Every reply MUST end with one fenced JSON block
  `{"citations":[{"hunk":"N","path":"...","lines":"S:E","relation":"specification|decision|test-claim|implementation|call-site|dependency|incident","confidence":"certain|likely|unsure"}],"unknown":["N"]}`.
  The scorer takes the last fenced block naming `"citations"`, or a reply that is itself one JSON
  object naming it. No such block is `ABSENT`; a block that does not parse into a citations array is
  `MALFORMED`. Both score as "cited nothing" and are counted, never dropped. A lane whose agent
  process exited non-zero produced no reply at all and is not one of these states: it is an errored
  lane under CRT-V0-010, dropped with its change, and the harness records the exit code and the
  bounded stderr tail as the lane's error. A reply cut at the reply bound (`reply_truncated`) with
  no citations block left is errored the same way, never `ABSENT`: the cut lost the observation.
- `CRT-V0-007`: Metrics. `miss(c,a) = |gold(c) \ citedPaths(c,a)| / |gold(c)|`. The primary estimate
  is `Δ = mean_c[miss(c,control) − miss(c,treatment)]`, the relative reduction is
  `R = Δ / mean_c[miss(c,control)]`, and the interval is a 95% BCa bootstrap over changes (10,000
  resamples, from a stream derived from the recorded seed). Secondary: the paired binary "missed
  anything" with an exact McNemar test. `citable` is `supported / (total − mechanical)` from the
  `cem status` counts the treatment lane's own citations produce, treatment only. The verifier
  hard-failure rate is the treatment lanes with at least one citation where `cem verify` reports
  `ok:false`; a hard failure is **incorrect** when an independent replay — resolve `base:path`, check
  the one-based range lies inside the file, extract and re-find the cited span, re-check the relation
  vocabulary — says the citation was valid. The replay is implemented against Git alone and never
  calls `internal/cem/verify`. Both `citable` and the verifier rate count only treatment lanes that
  did not error: a lane with an `error` or an observed non-zero `exit_code` (CRT-V0-006) is excluded
  whether or not its `cem` counts were recorded.
- `CRT-V0-008`: Power, pre-declared. 30 pairs is roughly 120 gold items per arm; with clustering
  (ICC 0.3, m=4, design effect 1.9) the effective n is about 63. At a control miss of 0.50 a 20%
  relative reduction is `Δ = 0.10` with SE ≈ 0.088 and a 95% CI of about (−0.07, 0.27); 80% power at
  α = 0.05 would need about 180 pairs. 30 pairs is therefore an **estimation run, not a test**: the
  gate "at least 20% fewer missed-evidence findings" is declared met only when the point estimate
  reaches 0.20 relative **and** the interval's lower bound is greater than 0, reporting
  "met: at least 20% fewer missed-evidence findings". Otherwise, an upper bound below 0 reports
  "not met: treatment missed more evidence than control"; a lower bound above 0 reports
  "below target but interval excludes 0". An interval covering both 0 and the target reports
  "consistent with 20% and with 0"; one covering 0 but excluding the target reports
  "consistent with 0 but not with 20%". Bounds are absolute miss-rate deltas: compare the target
  as `0.20 × mean control miss`, using the unrounded mean, with inclusive interval endpoints.
  No win is claimed for any non-met reading. Fewer than two scored pairs have no resampling
  variability, so `delta_ci95` is `NOT_OBSERVED` and the reading is
  "not estimable: fewer than two scored pairs", never met. The pair count is a manifest parameter
  (`--limit`) so the owner can widen it.
- `CRT-V0-009`: Pilot. The pilot uses its own seed and partition; its changes are excluded from the
  main population. Pilot results may tune prompts and timeouts, and are then development forever
  under `benchmarks/README.md`'s first-observation rule.
- `CRT-V0-010`: Stopping and publication. There is no interim analysis: the scorer runs once, after
  every lane has completed. The report is labelled first-run at
  `benchmarks/results/cem-reviewer-trial-first-run.json` with the partition `heldout`, the manifest
  sha256, the prompt-skeleton sha256, both prologues' text and sha256, the model id, the effort, the
  `corvint` version and sha256, and the seed. A run is **invalid** — discarded and re-declared, never
  re-scored — when the prompt digests differ between arms beyond the declared prologue, any lane's
  clone resolved `C`, more than three lanes errored (the scorer sets the report's `invalid` field
  from that count, reading each lane's recorded exit code so a report written by an earlier harness
  reaches the same verdict), the selection rule, seed, gold, or scorer changed after the first
  invocation, or any lane was re-run with a changed prompt. `--resume` and `--reuse`
  may only re-run lanes that never finished; a lane is reused only when its prompt sha256 is
  byte-identical, its wall time was observed, and it did not error under CRT-V0-006 (read from the
  recorded exit code too, so a prior report's failed lane without an `error` is re-run).

- `CRT-V0-011`: Invocation lifecycle (proposed). `runCommand` MUST share the
  `internal/procgroup` pre-reap supervisor with the other trial dispatcher. On Darwin and
  Linux, cancellation, deadline, normal leader exit and the runner's real INT/TERM path
  terminate the owned process group and bound cleanup and pipe draining to five seconds.
  Incomplete cleanup or exit observation is an invocation error. Normal and nonzero exit
  codes retain their existing shapes; timeout and cancellation remain errors. Empty/relative
  roots and PATH/root-relative executables normalize before dispatch; stdin is EOF and the
  environment is inherited.
  Capture keeps the first 8 MiB stdout and 64 KiB stderr while draining excess, with no
  overflow-induced short write, cancellation or exit-code change. This corrects the old
  CEM bounded writer's short-count defect when a write crossed its remaining capacity.
  The shared supervisor's default remains fail-on-overflow; only these invocations opt
  into truncation. Qualification covers the owned group only: changed process groups or
  sessions, other platforms, and other trial subprocess helpers remain outside this slice.
  It adds no hostile-execution containment claim and changes no gold, prompt, scorer,
  stopping condition or outcome qualification.

## Non-goals and simpler baseline

Human reviewers; a judge model; any agent CLI beyond `codex` and the `script` adapter; multi-turn
dialogue; measuring defect detection in general; declaring CEM adopted or product-validated; any write
to the Beamfall repository, which is read-only history.

Two ground-truth designs were considered and rejected. **Agent-authored gold spans** would let the
measured system define its own answer key, so a treatment win could not be separated from a treatment
gloss on the question. **Seeded evidence-gap mutants** would make the gold exact but would measure a
synthetic distribution of gaps rather than the one real changes produce; they may run as a
pre-registered secondary stratum on 5 changes, reported separately and never pooled with the primary.

The simpler baseline is the mechanical stem-convention guess, which needs no agent at all; CRT-V0-004
records it per change so the report shows what the agent added to it.

## Trust boundary, resource limits, failure modes

The tool trusts the manifest, the local Git history it is given, and the `corvint` and agent
executables named on the command line; it downloads nothing. Manifest 16 MiB; patch 64 KiB; artefact
256 KiB each; reply 1 MiB; agent stdout 8 MiB; `corvint` call 2 minutes; agent call `--timeout`
(default 5 minutes).

- `cem begin` rejects a patch: the change is excluded at selection, before the seed is fixed.
- Agent timeout or launch failure: the lane records `error`, and its **pair is dropped whole**, so the
  arms always cover the same changes.
- A gold path leaked into a source hunk: the change is excluded at selection.
- Stem-convention guessing inflates both arms: the report carries the stem baseline's own miss rate
  beside them, and treatment is read as lift over that floor.
- The treatment arm ignores the map: recorded, not corrected. The trial measures what the agent did
  with the artefacts, not whether it read them.
- Zero supported hunks — the observed prior, `benchmarks/results/cem-first-run-beamfall-pass-v0.1.json`
  records 0 supported against 4 unknown — leaves `citable` at 0 and the trial still publishes.

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CRT-V0-001 | `selectionRule`, `candidateCommits`, `inspectCommit`, `validated`, `beginHunks`, `evenlySpaced`, `readManifest` (`tools/cem-trial/select.go`, `main.go`) | `TestSelectionIsDeterministicAndRefusesGoldLeak`; pilot manifest `tools/cem-trial/testdata/pilot/tasks.json` (population 620, five changes) |
| CRT-V0-002 | `plan`, `shuffledArms`, `seededStream`, `laneClone` | `TestCloneCannotResolveTheChangeCommit`; pilot dry run (three of five changes ran treatment first) |
| CRT-V0-003 | `inspectCommit` (source-only diff), `hunkList`, `readPatch` digest check | `TestWithheldPatchExcludesEveryGoldPath` |
| CRT-V0-004 | `goldSpans`, `hunkRanges`, `stemBaseline`, `stemGuesses`, `leaksGold`, `hunkBody`, `stemMiss` (`gold.go`, `score.go`) | `TestSelectionIsDeterministicAndRefusesGoldLeak`; `TestWithheldPatchExcludesEveryGoldPath` (spans present) |
| CRT-V0-005 | `skeleton`, `treatmentPrologue`, `buildPrompt`, `buildArtefacts`, `cloneAtBase`, `resolvable`, `laneClone` | `TestBothArmsShareTheSamePromptDigestApartFromThePrologue`; `TestTreatmentUsesCemBeginNotPrepare`; `TestCloneCannotResolveTheChangeCommit` |
| CRT-V0-006 | `extractCitations`, `citationsBlock`, `extractUnknown`, `scoreLane` | `TestMissRateAndPairedBootstrapAreDeterministicUnderASeed`; `TestScoreRebuildsTheReportByteIdentically` |
| CRT-V0-007 | `scoreLane`, `summarize`, `bca`, `mcnemar`, `exactBinomial`, `citable`, `verifierRates`, `applyCitations`, `replayCitations`, `replayCitation` | `TestMissRateAndPairedBootstrapAreDeterministicUnderASeed`; `TestReplayCheckerDisagreesWithVerifyOnAKnownGoodCitation`; `TestCitableAndVerifierSkipNonZeroExitLanes`; 2026-09-04 pilot (`citable` 0.815 over 200 material hunks, verifier incorrect-hard-failure rate 0 over 25 citing lanes) |
| CRT-V0-008 | `verdicts`, `verdictAtLeast`, `verdictBelow`, `relativeGate`, `citableGate`, `hardFailureGate`; absolute interval target is `relativeGate × controlMiss` | `TestMissedEvidenceVerdictIntervalCompatibility` (`CRT-V0-008` subtests cover all five readings, absolute target scaling, and inclusive endpoints); `TestMissRateAndPairedBootstrapAreDeterministicUnderASeed`; `benchmarks/results/cem-reviewer-trial-pilot-2026-09-04.json`'s stored `missed_evidence` wording predates the 2026-09-04 amendment (control miss 0.90, interval [0, 0.06] now reads "consistent with 0 but not with 20%"); the stored report is history, not rescored |
| CRT-V0-009 | `--partition`/`--seed`/`--exclude` on `select`, `excludedCommits`, `objectName`, `withoutExcluded`, `inertExclusion`, `excludeError`, `poolError` (`tools/cem-trial/select.go`), `report.Pilot`; the manifest's `population` and the summary's `population N, excluded M` are the pool after the M drops | `tools/cem-trial/testdata/pilot/tasks.json` carries `partition: pilot`; `benchmarks/results/cem-reviewer-trial-pilot-2026-09-04.json` (seed `pilot-2026-09-03`, 5 pairs, 50 lanes, `gpt-5.6-sol` at medium); `TestExcludedPilotChangesAreDroppedAndCounted` (three qualifying changes, one-commit pilot, non-empty disjoint held-out set of population qualifying − excluded), `TestExcludeFileWithoutIdentifiersIsRefused`, `TestExcludeIdentifiersMustBeFullLowercaseHex`, `TestExcludeThatDropsNothingIsRefused`, `TestEmptyPoolIsRefusedAndAShortPoolWarns`, `TestExplicitlyEmptyExcludeListIsRefused` |
| CRT-V0-010 | `finalize`, `rescore`, `write`, `checkpointer`, `loadReuse`, `reuseSource.apply`, `laneFailed`, `report.Invalid` | `TestScoreRebuildsTheReportByteIdentically`; `TestErroredLaneDropsItsPair`; `TestNonZeroAgentExitErrorsTheLaneAndInvalidatesTheRun`; 2026-09-03 pilot rescored to `invalid` (31 errored lanes); 2026-09-04 re-run valid (`invalid: null`, 0 errored lanes) |
| CRT-V0-011 | `runCommand`; `internal/procgroup.Run`, `OverflowPolicy`, `Spec.StderrLimit` | `TestRunProcessLifecycleMatrix`; `TestRunProcessTruncateOverflowPolicy`; `TestRunProcessDefaultOverflowPolicyStillTerminates`; `TestRunProcessRejectsInvalidOverflowPolicyBeforeSpawn`; `TestTruncateCaptureConsumesCrossingWrite`; `TestRunCommandPreservesExitShapes`; `TestRunCommandNormalizesPathsAndEnvironment`; `TestRunCommandRefusesBeforeDispatch`; `TestRunCommandTruncatesOutput`; `TestRunCommandCancellationKillsDescendant`; `TestTrialMainSignalsCleanOwnedAgentGroup` |

## Rollout, rollback, compatibility

For the proposed lifecycle slice, rollback disables trial dispatch while reverting the
adapter/supervisor option change and its clauses together. Do not resume a frozen run under
a changed harness; retain its existing artifacts and first-observation restrictions.

The tool ships under `tools/`; no binary, adapter, or benchmark manifest calls it. Rollback is
deleting `tools/cem-trial` and this spec plus its README and index rows; nothing in `cmd/corvint`,
`internal/cem`, or `benchmarks/` changes. A discarded run leaves its report in place with
`"invalid": <reason>` rather than being re-scored.

Compatibility and drift: the harness copies `tools/cw-trial`'s dispatch, checkpoint, and canonical-
output shapes on purpose rather than importing them, because both are `package main`; if the codex
event names or the reuse rules change, both change in one commit. The `cem` command surface it drives
(`begin`, `cite`, `status`, `verify`, `report`, and the cem/0.1 `--patch` rule) is the one in
`internal/cem/workflow`; a lane clone must not carry `objects/info/alternates`, which `corvint`
refuses, so lanes are plain local clones.

## Unresolved decisions and promotion or kill

Which model and reasoning effort the judged run uses; whether the secondary mutant stratum runs at
all; whether `unknown` hunks the agent declares should score beside the citations rather than only
being recorded. Promote to `implemented` when a `heldout` set of at least 30 pairs, selected and
frozen before any agent output was read, has run once in both arms and its first report is preserved
unrepaired. V4's gate is then read from that report by the owner under CRT-V0-008's wording, never
from the pilot.
