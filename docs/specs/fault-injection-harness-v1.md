# Fault Injection Harness V1

Owner: Russell Lewis
Date: 2026-11-11
Requirement prefix: `FIH-V1`
Intent status: accepted automatically under G14 (decision 0596, 2026-11-10; V1-5405)
Delivery status: experimental; partially delivered
Authoritative inputs: change folder `docs/specs/openspec/changes/fault-injection-harness/`
(`proposal.md`, `design.md`, `tasks.md` and `specs/fault-injection-harness/spec.md`),
`docs/specs/PRODUCT-PLAN-1.0.md` (PROD-TEST-002, PROD-TEST-011, PROD-INT-008),
`docs/specs/affected-edit-oracle-v0.md` (AEO-V0) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md`.

## Agent digest
- Claim: Pre-registered, hashed fault sets per runner prove that a selected run executes every test that kills a fault on a changed line, with a fixed miss verdict.
- Status: accepted automatically under G14 (decision 0596, 2026-11-10; V1-5405) / experimental; partially delivered
- Exists: `tools/fault-harness` with its README: the harness core, the Go adapter, the AEO parity command, the pre-registration, the reverted-fix rule file, the seal, the baseline with quarantine, the `run` command and the verdict writer (tasks 3, 4, 5 and 7).
- Blocked on: the six other adapters, the manifest pins and the cost check (task 6); sealed sets and verdicts at the RC SHA (task 8); the held-out sets after the draw (task 9).
- Read next: Requirements; Delivery and traceability; Rollback.

## User and boundary

The release owner needs the 1.0 claim "no test that could fail is left out" proven per blocking
runner, not asserted. A fault is put into a line a PR changed; the selected run (the plan Corvint
produced for that PR, executed by the runner) must execute the test that kills it and see that test
fail. This spec is the canonical home of FIH-V1-001 to FIH-V1-015, copied from the change's spec
delta. Decisions, failure modes and open questions stay in the change's `design.md`, which owns
them. Each delivery is recorded in one `docs/build-log/YYYY-MM-DD-<slug>.md` entry.

Non-goals (from the change): no product verb, since this is release evidence and not a user
feature; no test-strength scoring and no revival of `prove`, pymutate or rbmutate (G26); no removal
of browser E2E tests by line evidence (G2); no runs on Unraid hosts (G1); no Cypress adapter (owner
decision R6 (2026-11-09)).

Trust boundary: the pre-registration, rule file, fault set and runner commands are trusted
operator input, executed with process-group containment, not a sandbox (as AEO-V0). The harness
fails closed: a timeout, a crash, a build failure, an unresolved fault or a surviving fault is a
failed measurement, never a pass, and a miss is never waived as a flake.

## Requirements

Requirement IDs are stable. Each requirement keeps the change's text; a closing `Delivery:` line
says what this tree delivers.

- `FIH-V1-001`: (accepted automatically under G14, decision 0596; V1-5405) Pre-registration before execution

For each blocking runner the harness MUST read a pre-registration file holding the sampling frame,
repository SHAs, the revert selection query, the operator list per language, seeds, the over-sample
size and exclusion rules, and MUST refuse to execute when its SHA-256 differs from the committed
hash. The file MUST also hold the SHA-256 of the reverted-fix rule file of FIH-V1-002 (decided
2026-11-09 under R42, A17). A drawn held-out repository (FIH-V1-012) is not known before the draw
of PRR-V1-004, so its pre-registration MUST be written and its hash committed after the draw and
before any fault is generated for it; the harness MUST refuse a held-out pre-registration earlier.
(G3, owner decision H3 (2026-11-10))

**Scenario: Changed seeds**
- GIVEN a committed pre-registration hash
- WHEN a seed is edited and the harness starts
- THEN it refuses with `preregistration-changed` and runs nothing

**Scenario: Held-out pre-registration before the draw**
- GIVEN no committed draw record under PRR-V1-004
- WHEN a held-out pre-registration or fault set is requested
- THEN it refuses with `held-out-not-drawn`

Delivery: delivered. `fault-harness check`, `select-reverts`, `generate-go` and `seal` read the
pre-registration (`fault-harness-preregistration/1`: runner, language, repository SHA and pin date,
sampling frame, the reverted-fix rule's path and SHA-256, the operator list per language, seeds,
over-sample, exclusion budget and rules) and refuse with `preregistration-changed` when its SHA-256
differs from the committed hash file, before anything runs; a missing hash file is
`preregistration-unhashed`. A held-out pre-registration is refused with `held-out-not-drawn` unless
it pins a draw record whose SHA-256 matches; the PRR-V1-004 draw record does not exist yet, so every
real held-out request is refused. Test: `TestPreregistrationHash_FIH_V1_001`.

- `FIH-V1-002`: (accepted automatically under G14, decision 0596; V1-5405) Composition of a fault set

Each blocking runner's code set MUST contain 30 reverts of real bug fixes, chosen by the one
deterministic rule below, and changed-line mutants at 3 to 5 per PR
for the remaining PRs, drawn from the operator list the pre-registration names for that language.
Non-code faults in configuration, fixtures, snapshots, lockfiles or generated files MUST be counted
as a separate set of at least 30 with their own N and their own 3/N bound, and MUST NOT be added
to the code-fault N. (G3, G15b, owner decision R19 (2026-11-09), review release-tasks 11)

The reverted-fix rule (decided 2026-11-09 under R42, A17). The rule MUST be one deterministic rule
file, committed and hashed before any Corvint run on the pin. Its frame is the PRs merged in the 12
months before the pinned commit (24 months if fewer than 30 qualify) that are fixes (a `fix` title
prefix, a bug label, or a closing link to a bug issue), that change runner-owned source and at
least one test, whose source-only revert applies and builds at the pin, and whose revert fails at
least one test three times out of three on the full suite. The frame MUST be ordered by SHA-256 of
(rule-file hash, PR number) and the first 30 taken, with at most two per source file. If fewer than
30 qualify, the shortfall MUST be filled with changed-line mutants and reported. Earlier wording,
superseded by A17: "exactly 30 reverts of real bug fixes, chosen by the revert selection query
recorded in the pre-registration".

Flow-reaching faults (decided 2026-11-09 under R42, A16). The Playwright set MUST hold at least 20
faults in code reached by a linked flow. The flows are those of the pinned freeCodeCamp repository
(10 flows with at least 40 owner-accepted links), frozen and hashed into its manifest before this
set is sealed (PRR-V1-003).

**Scenario: Same rule, same thirty**
- GIVEN one rule-file hash and one frame of qualifying PRs
- WHEN the reverts are selected twice
- THEN both selections name the same 30 PRs in the same order

**Scenario: Fewer than thirty qualifying fixes**
- GIVEN a frame of 24 qualifying PRs after the 24-month extension
- WHEN the set is sealed
- THEN it holds 24 reverts, fills the shortfall of 6 with changed-line mutants and reports the
  shortfall

**Scenario: Too few flow-reaching faults**
- GIVEN a Playwright set with 12 faults in code reached by a linked flow
- WHEN the set is sealed
- THEN sealing refuses and reports 12 against the minimum of 20

**Scenario: Too few non-code faults**
- GIVEN a set with 110 code PRs and 12 non-code faults
- WHEN the set is sealed
- THEN sealing refuses with `non-code-faults-below-minimum`

**Scenario: Two bounds**
- GIVEN 104 code PRs and 32 non-code faults with confirmed kills and no misses
- WHEN the verdict is printed
- THEN it prints one sentence with N = 104 and a second with N = 32, each with its own bound

Delivery: partly delivered. The A17 rule is the committed file
`tools/fault-harness/revert-rule.json` with its hash `revert-rule.json.sha256`; `loadRule` refuses
another SHA-256 (`revert-rule-changed`) or other numbers (`revert-rule-invalid`). `selectReverts`
applies it to observed PR facts (12-month frame, 24 when fewer than 30 qualify, SHA-256 order of
rule hash and PR number, first 30, at most two per source file, shortfall reported). `seal` refuses
a set whose reverts differ from the selection, whose shortfall is not filled by changed-line mutant
PRs, whose mutant PRs hold other than 3 to 5 mutants or an operator outside the pre-registered list,
with fewer than 30 non-code faults (`non-code-faults-below-minimum`), or, for Playwright, fewer than
20 flow-reaching faults. `generate-go` places Go mutants. The verdict keeps code and non-code faults
in separate sections, each with its own N, its own 3/N bound and its own sentence, and `verdict`
applies the non-code minimum again: it refuses a set sealed with fewer than 30 non-code cases
(`non-code-faults-below-minimum`). Observing the PR
facts on a pinned repository, the Playwright flow manifest and the non-code fault generators are
tasks 6 and 8 and not yet built. Tests: `TestRevertRule_FIH_V1_002`, `TestSeal_FIH_V1_002_014`,
`TestVerdictCountsAndSentence_FIH_V1_007_014`, `TestVerdictBoundToSeal_FIH_V1_002_007_014`.

- `FIH-V1-003`: (accepted automatically under G14, decision 0596; V1-5405) Faults sit on changed lines

Every mutant MUST be placed on a line changed by its PR; a bug-fix revert MUST restore the pre-fix
lines. (G3)

**Scenario: Unchanged line**
- GIVEN an operator site outside the PR's changed lines
- WHEN mutants are generated
- THEN the site is skipped

Delivery: delivered for Go. `goSites` places a mutant only on a line of the PR's `git diff -U0` new
side and skips every other site; `applyFault` refuses a fault whose bytes or line differ from its
record; a revert reverse-applies the fix's source-only diff and is proved to restore the pre-fix
lines by the fix applying forward again. Tests: `TestChangedLineSites_FIH_V1_003`,
`TestFaultsOnChangedLinesInGit_FIH_V1_003`.

- `FIH-V1-004`: (accepted automatically under G14, decision 0596; V1-5405) Baseline and quarantine

The full suite MUST run three times at the base. A test failing in any baseline run MUST be
quarantined and excluded from kill counting. If quarantined tests exceed 2 percent of the suite, the
runner MUST be disqualified. The verdict MUST also report, as a separate count, the quarantined
tests that a plan omitted for a PR touching their dependencies. Both caps apply to one baseline, on
different sets: quarantined (flaky) tests above 2 percent disqualify the runner's miss proof, and
all excluded tests (quarantined plus unrunnable in the pinned image) above 5 percent make the
measurement `INVALID` (R42 (panel pick) A10). (G3, review release-tasks 12)

**Scenario: Flaky baseline**
- GIVEN 3 percent of tests fail in at least one of three baseline runs
- WHEN the baseline completes
- THEN the runner verdict is `DISQUALIFIED`

**Scenario: Quarantined test omitted**
- GIVEN quarantined test Q whose dependency a sealed PR changes, and that PR's plan without Q
- WHEN the verdict is written
- THEN the count of omitted quarantined tests is 1 and names Q

Delivery: delivered. `fault-harness run` runs the whole suite three times at each case head (once
per head) and `evaluateBaseline` quarantines every test that did not pass in all three runs (a
failure, a missing result, or a pass beside a skip), counts tests skipped in every run or declared
with `--unrunnable` as unrunnable, and sets `disqualified` above 2 percent quarantined and `invalid`
above 5 percent excluded, in exact integer arithmetic; the verdict is then `DISQUALIFIED` or
`INVALID`. A quarantined test never passes every baseline run, so it never counts as a kill. The
plan and the dependent units (for Go, from `go list -test -deps`) are recorded for every fault, and
the verdict counts and names the quarantined tests a plan omitted for a PR that changed their
dependencies. Tests: `TestBaselineQuarantine_FIH_V1_004`, `TestRunSealedSet_FIH_V1_004_005_006`,
`TestVerdictFailClosed_FIH_V1_004_014`.

- `FIH-V1-005`: (accepted automatically under G14, decision 0596; V1-5405) A kill is confirmed three times

A test MUST count as killing a fault only if it fails in three of three runs on the faulted tree
and passed in every baseline run. Faults that no test kills MUST NOT count. A PR counts once it has
at least one confirmed kill. (G3)

**Scenario: Two of three**
- GIVEN a test failing twice in three faulted runs
- WHEN kills are confirmed
- THEN it is not a kill

Delivery: delivered. `confirmKills` counts a test only when it fails in three of three faulted runs
and passed in all three baseline runs; a faulted tree that does not build or crashes (including the
runner's own test-timeout panic) is no kill and joins the dropped count. A faulted run stopped by
cancellation or by the harness's own wall bound is not fault-induced: the fault is `ERROR`, the case
a failed measurement and the verdict `INVALID`, and a clone checkout stopped the same way is a
failed measurement, not `pin-unavailable`. Tests: `TestKillConfirmationAndMiss_FIH_V1_005_006`,
`TestGoOutcomesFailClosed_FIH_V1_005`, `TestFaultedRunStopped_FIH_V1_005_014`.

- `FIH-V1-006`: (accepted automatically under G14, decision 0596; V1-5405) A miss blocks

For each confirmed kill, the selected run (the plan Corvint produced for that PR, executed by the
runner) MUST execute the killing test and observe it fail. Otherwise the case is `MISS`, the runner
verdict is not zero-miss, and the miss MUST NOT be waived as a flake. (G3)

**Scenario: Killing test not selected**
- GIVEN a confirmed kill by test T and a plan without T
- WHEN the verdict is computed
- THEN the case is `MISS` and the release is blocked

Delivery: delivered in the harness core. For a NARROWED plan the selected run executes the plan's
own units with their test filters, and every confirmed kill it does not execute and see fail is a
miss; a selected-run failure after a confirmed kill is a failed measurement (case `ERROR`). The
release block itself is the verdict of FIH-V1-007. Test:
`TestKillConfirmationAndMiss_FIH_V1_005_006`.

- `FIH-V1-007`: (accepted automatically under G14, decision 0596; V1-5405) PRs are counted and the wording is fixed

The verdict MUST count PRs with a confirmed kill, not mutants, and MUST print the G3 release
sentence with N, the bound 3/N, and the 95 percent Wilson upper bound beside it (settled
release-tasks S6). (G3, review minor 20)

**Scenario: 104 counted PRs**
- GIVEN 104 narrowed PRs with confirmed kills and no misses
- WHEN the verdict is printed
- THEN it reads "0 misses in 104 pre-registered fault-injected cases per runner on pinned repos;
  95 percent upper bound on the per-change miss rate 3/104; not a guarantee."

Delivery: delivered as the verdict writer; the seven verdict files at the RC SHA are task 8 and not
yet produced. `fault-harness verdict` counts PRs with a confirmed kill under a narrowed plan, not
mutants, and for a zero-miss verdict prints the G3 sentence exactly with N and 3/N and the 95
percent Wilson upper bound beside it; any miss makes the verdict `MISS` and lists it per fault, the
Wilson bound counts missed cases (`missedCases`), like N, and no release sentence is printed for a
verdict other than `ZERO_MISS`. `verdict` requires the sealed set and its seal record (`--set`,
`--seal`) and refuses results that are not their measurement: another fault-set hash
(`fault-set-changed`), seal counts that differ from the set (`fault-set-invalid`), or a case list
that is not the sealed one, each sealed case exactly once with its kind, PR and head, no unsealed
case, and every counted or dropped case holding its sealed faults in order with a status that
agrees with them (`results-not-sealed`). Tests: `TestVerdictCountsAndSentence_FIH_V1_007_014`,
`TestVerdictCommand_FIH_V1_007`, `TestVerdictBoundToSeal_FIH_V1_002_007_014`.

- `FIH-V1-008`: (accepted automatically under G14, decision 0596; V1-5405) One adapter per runner on the compiled-in contract

The harness MUST drive Go, Vitest, Jest, Playwright, pytest, RSpec and Minitest through one
compiled-in runner contract that runs a named test set at a commit and returns a per-test outcome.
(G1, G23, owner decision R7 (2026-11-09))

**Scenario: Unsupported runner**
- GIVEN a pre-registration naming a runner without an adapter
- WHEN the harness starts
- THEN it refuses with `runner-adapter-missing`

Delivery: partly delivered: the compiled-in contract (`Adapter`: run a named test set in a tree,
return per-test outcomes) and the `runner-adapter-missing` refusal for any runner without an
adapter. Only the Go adapter exists; the other six adapters are task 6 and not yet built. Test:
`TestAdapterContract_FIH_V1_008`.

- `FIH-V1-009`: (accepted automatically under G14, decision 0596; V1-5405) Pinned and reproducible

Every case MUST run against the PRR-V1 manifest pins (toolchain, lockfile, browser build, repository
SHA, image digest) on Linux or macOS. (G1)

**Scenario: Different browser build**
- GIVEN a Playwright manifest pinning one browser build
- WHEN the host offers another
- THEN the case refuses with `pin-mismatch`

Delivery: not yet built (task 6): the PRR-V1 manifest pins and the `pin-mismatch` refusal. The
harness checks out each case at its full commit SHA and excludes a case whose pin cannot be checked
out as `pin-unavailable`.

- `FIH-V1-010`: (accepted automatically under G14, decision 0596; V1-5405) Cost check in two steps, then one PR per runner first

The cost check MUST run in two steps (decided 2026-11-09 under R42, A18). Step 1, by 2026-11-16:
time one full-suite run of each of the seven pinned repositories on the benchmark box and publish
the projected total. The four drawn held-out repositories are unknown until the first candidate tag
(PRR-V1-004), so the held-out part of the projection MUST be reported as `NOT_OBSERVED` and MUST be
re-run once the draw exists; how it is estimated before then is NOT_DECIDED. Step 2, by 2026-11-27,
once the Go adapter exists (FIH-V1-011): before the full set, the harness MUST run one
pre-registered fault-carrying PR per runner and report elapsed time. The budget is 96 host-hours
inside the M7 window (2026-12-08 to 2026-12-15). If the projection is over budget, these measures
MUST be applied in this order: one baseline per repository pin; kill confirmation reruns only the
failing tests; held-out sets run in parallel on the second reference host. If the projection still
exceeds the budget after the three measures, the choice between a smaller N and a later freeze
returns to the owner. Earlier wording, superseded by A18: "Before the full set, the harness MUST run
one pre-registered PR per runner and report elapsed time, so the owner can confirm the compute
budget." (G3)

**Scenario: Step 1 projection**
- GIVEN the seven pinned repositories and no draw yet
- WHEN step 1 runs on the benchmark box
- THEN the report holds one full-suite time per pinned repository, marks the held-out part
  `NOT_OBSERVED`, and shows the projected total beside the 96 host-hour budget

**Scenario: Projection after the draw**
- GIVEN a committed draw record
- WHEN the projection is re-run
- THEN it holds one full-suite time per drawn repository, and the total is compared with the
  96 host-hour budget

**Scenario: Cost probe**
- GIVEN seven sealed sets
- WHEN the probe runs
- THEN it reports one completed PR and its elapsed time per runner

**Scenario: Still over budget**
- GIVEN a projection above 96 host-hours after the three measures
- WHEN the cost report is written
- THEN it states the excess and that the choice between a smaller N and a later freeze is the
  owner's, and the harness changes neither

Delivery: not yet built (task 6): neither cost-check step has run, and the held-out part of the
projection is NOT_OBSERVED until the draw.

- `FIH-V1-011`: (accepted automatically under G14, decision 0596; V1-5405) Go first, built once

The Go adapter MUST be delivered first by generalising `tools/affected-edit-oracle`, and later
adapters MUST reuse the same harness core. (G1)

**Scenario: Go parity**
- GIVEN an AEO-V0 mutation case
- WHEN run through the harness Go adapter
- THEN the miss verdict matches `affected-edit-oracle` on the same inputs

Delivery: delivered for Go. `tools/fault-harness` generalises `tools/affected-edit-oracle` (its
process, Git and receipt plumbing is copied, the oracle is unchanged) and `fault-harness parity`
runs an AEO-V0 mutation case through the harness core and Go adapter.
`TestGoParityWithOracle_FIH_V1_011` builds the oracle and compares both miss verdicts and missing
units on one synthetic repository for three planners. The parity run on the checked-in AEO corpus is
not run here.

- `FIH-V1-012`: (accepted automatically under G14, decision 0596; V1-5405) Held-out fault sets

Owner decision R19 (2026-11-09). Each held-out repository drawn under PRR-V1-004 (one per family,
a seeded draw that is never redrawn: owner decision H3 (2026-11-10)) MUST receive N = 100 faults by
this method, frozen and hashed before execution. The sets can only be built after the first
candidate tag exists, because that tag seeds the draw; they MUST be built between the candidate tag
and the freeze (a schedule risk, not a decision). Its verdict passes with at most 3 misses in N,
reported with the 95 percent Wilson interval (settled release-tasks S6); excluded tests MUST be at
most 5 percent of the suite, or the verdict is `INVALID`. Each held-out miss MUST be published as
measured with its interval, filed as a P1 ticket and fixed by widening before the release
candidate. The per-runner fault sets of the seven pinned repositories MUST have zero known misses
(G3). Every receipt MUST carry per-test durations (change `per-test-durations`). A drawn repository
that proves unusable MUST be reported by PRR-V1-004 and MUST NOT be replaced. (G12; R42 (panel
pick) A9)

**Scenario: Held-out freeze**
- GIVEN a held-out repository from the drawn list
- WHEN its faults are generated
- THEN the hash is committed before any selected run

**Scenario: Four misses**
- GIVEN a held-out set of 100 counted faults with 4 misses
- WHEN the verdict is computed
- THEN the held-out verdict is FAIL and states the Wilson interval

**Scenario: One held-out miss**
- GIVEN a held-out set of 100 counted faults with 1 miss
- WHEN the verdict is computed
- THEN the verdict passes, publishes the miss with its interval, and names the P1 ticket that
  must be fixed by widening before the release candidate

Delivery: not yet built (task 9): it needs the PRR-V1-004 draw, which needs the first candidate tag.
Only the `held-out-not-drawn` refusal of FIH-V1-001 exists.

- `FIH-V1-013`: (accepted automatically under G14, decision 0596; V1-5405) No dependency on removed packages

The harness MUST NOT import `internal/liveverify/pymutate` or `internal/liveverify/rbmutate`. It MAY
reuse `internal/liveverify/mutate` only after the G26 ticket re-owns that package under this spec.
(G26)

**Scenario: Import check**
- GIVEN the harness source
- WHEN imports are checked
- THEN neither removed package appears

Delivery: delivered. The harness imports neither removed package, nor `internal/liveverify/mutate`,
which no G26 ticket has re-owned; it defines its own operators. Test: `TestImports_FIH_V1_013`.

- `FIH-V1-014`: (accepted automatically under G14, decision 0596; V1-5405) Case accounting and the size of N

Owner decision R19 (2026-11-09), review release-tasks 10. The verdict MUST report four counts:
cases sealed, cases dropped because no test killed the fault, cases whose plan was FULL or UNKNOWN,
and cases whose plan was narrowed with a confirmed kill. N is the last count. The pre-registration
MUST over-sample, sealing about 140 PRs per blocking runner, so that N can reach 100. If N is below
100, the runner's verdict MUST be `INVALID`. The cost check of FIH-V1-010 MUST NOT lower N or the
over-sample; a smaller N is the owner's choice (decided 2026-11-09 under R42, A18).

**Scenario: Too few narrowed kills**
- GIVEN 140 sealed PRs, of which 28 have no kill and 21 have FULL plans
- WHEN the verdict is computed
- THEN N is 91, the four counts are printed and the verdict is `INVALID`

Delivery: delivered in the tool. The pre-registration must over-sample at least 100 code PRs and
`seal` refuses a set with fewer code PRs than the pre-registered over-sample
(`over-sample-not-met`). The verdict prints the four counts (sealed, dropped with no kill, plan FULL
or UNKNOWN, narrowed with a confirmed kill) and is `INVALID` when N is below 100; excluded and
failed-measurement cases are reported beside them, and exclusions over the pre-registered budget or
any failed measurement also make it `INVALID`. The counts are taken over the sealed case list only:
`verdict` refuses results with a sealed case missing, repeated or replaced. Nothing in the tool
lowers N or the over-sample. Tests: `TestSeal_FIH_V1_002_014`,
`TestVerdictCountsAndSentence_FIH_V1_007_014`, `TestVerdictFailClosed_FIH_V1_004_014`,
`TestVerdictBoundToSeal_FIH_V1_002_007_014`, `TestFaultedRunStopped_FIH_V1_005_014`.

- `FIH-V1-015`: (accepted automatically under G14, decision 0596; V1-5405) Line-level proven

Review release-tasks 21. A runner MAY be called line-level proven only when its FIH-V1 verdict is
zero-miss and every counted case's plan was computed at line granularity (tier `line`, owner
decision R16 (2026-11-09)). A runner proven at tier `file`, such as pytest under owner decision R9
(2026-11-09), MUST be reported as file-level.

**Scenario: File-tier pytest**
- GIVEN a zero-miss pytest verdict whose plans all carry tier `file`
- WHEN the release report is written
- THEN pytest is reported as file-level, not line-level proven

Delivery: delivered as the verdict's `granularity` label: `line-level proven` only for a zero-miss
verdict whose every counted plan records tier `line`, `file-level` when the counted plans record
only `line` or `file` with at least one `file`, and `not proven` otherwise, including a plan that
records no tier. The planner receipt does not yet carry the R16 tier, so a real run reads `not
proven` until it does. Test: `TestVerdictGranularity_FIH_V1_015`.

## Delivery and traceability

| Requirement | Task | Implementation | Evidence |
|---|---|---|---|
| FIH-V1-001 | 4 | `tools/fault-harness/prereg.go`, `cmds.go` | `TestPreregistrationHash_FIH_V1_001` |
| FIH-V1-002 | 4, 7 | `tools/fault-harness/prereg.go`, `seal.go`, `verdict.go`, `revert-rule.json` | `TestRevertRule_FIH_V1_002`, `TestSeal_FIH_V1_002_014`, `TestVerdictCountsAndSentence_FIH_V1_007_014`, `TestVerdictBoundToSeal_FIH_V1_002_007_014` |
| FIH-V1-003 | 3 | `tools/fault-harness/faults.go` | `TestChangedLineSites_FIH_V1_003`, `TestFaultsOnChangedLinesInGit_FIH_V1_003` |
| FIH-V1-004 | 5 | `tools/fault-harness/baseline.go`, `run.go`, `verdict.go` | `TestBaselineQuarantine_FIH_V1_004`, `TestRunSealedSet_FIH_V1_004_005_006`, `TestVerdictFailClosed_FIH_V1_004_014` |
| FIH-V1-005 | 3 | `tools/fault-harness/run.go`, `adapter.go`, `exec.go` | `TestKillConfirmationAndMiss_FIH_V1_005_006`, `TestGoOutcomesFailClosed_FIH_V1_005`, `TestFaultedRunStopped_FIH_V1_005_014` |
| FIH-V1-006 | 3 | `tools/fault-harness/run.go` | `TestKillConfirmationAndMiss_FIH_V1_005_006`, `TestGoParityWithOracle_FIH_V1_011` |
| FIH-V1-007 | 7 | `tools/fault-harness/verdict.go`, `cmds.go`, `seal.go` | `TestVerdictCountsAndSentence_FIH_V1_007_014`, `TestVerdictCommand_FIH_V1_007`, `TestVerdictBoundToSeal_FIH_V1_002_007_014` |
| FIH-V1-008 | 6 | `tools/fault-harness/adapter.go` (Go adapter only) | `TestAdapterContract_FIH_V1_008`; six adapters not yet built |
| FIH-V1-009 | 6 | not yet built | not yet built |
| FIH-V1-010 | 6 | not yet built | not yet built |
| FIH-V1-011 | 3 | `tools/fault-harness/aeo.go`, `main.go` | `TestGoParityWithOracle_FIH_V1_011` |
| FIH-V1-012 | 9 | not yet built | not yet built |
| FIH-V1-013 | 3 | `tools/fault-harness` | `TestImports_FIH_V1_013` |
| FIH-V1-014 | 4, 7 | `tools/fault-harness/seal.go`, `verdict.go`, `run.go` | `TestSeal_FIH_V1_002_014`, `TestVerdictCountsAndSentence_FIH_V1_007_014`, `TestVerdictFailClosed_FIH_V1_004_014`, `TestVerdictBoundToSeal_FIH_V1_002_007_014`, `TestFaultedRunStopped_FIH_V1_005_014` |
| FIH-V1-015 | 7 | `tools/fault-harness/verdict.go` | `TestVerdictGranularity_FIH_V1_015` |

Release evidence (seven sealed pre-registrations, the cost-check reports, seven verdicts at the RC
SHA and four held-out verdicts) is tasks 6, 8 and 9 of the change and is not produced here.

## Rollback

Revert the change's commits; the tool and verdict schema go and the IDs are retired. The release
then has no miss proof and cannot claim PROD-TEST-002; runners fall back to full runs with a
printed reason under NMS-V1-007.
