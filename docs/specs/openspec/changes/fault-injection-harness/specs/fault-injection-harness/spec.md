# Spec delta: fault-injection-harness

The lane copies each statement below into `fault-injection-harness-v1.md`, inside its
`## Requirements` section, as a numbered definition line that begins
`(proposed 2026-11-09; delivery not-started)`.

## ADDED Requirements

### Requirement: FIH-V1-001 Pre-registration before execution

For each blocking runner the harness MUST read a pre-registration file holding the sampling frame,
repository SHAs, the revert selection query, the operator list per language, seeds, the over-sample
size and exclusion rules, and MUST refuse to execute when its SHA-256 differs from the committed
hash. (G3)

#### Scenario: Changed seeds

- **GIVEN** a committed pre-registration hash
- **WHEN** a seed is edited and the harness starts
- **THEN** it refuses with `preregistration-changed` and runs nothing

### Requirement: FIH-V1-002 Composition of a fault set

Each blocking runner's code set MUST contain exactly 30 reverts of real bug fixes, chosen by the
revert selection query recorded in the pre-registration, and changed-line mutants at 3 to 5 per PR
for the remaining PRs, drawn from the operator list the pre-registration names for that language.
Non-code faults in configuration, fixtures, snapshots, lockfiles or generated files MUST be counted
as a separate set of at least 30 with their own N and their own 3/N bound, and MUST NOT be added
to the code-fault N. (G3, G15b, owner decision R19 (2026-11-09), review release-tasks 11)

#### Scenario: Too few non-code faults

- **GIVEN** a set with 110 code PRs and 12 non-code faults
- **WHEN** the set is sealed
- **THEN** sealing refuses with `non-code-faults-below-minimum`

#### Scenario: Two bounds

- **GIVEN** 104 code PRs and 32 non-code faults with confirmed kills and no misses
- **WHEN** the verdict is printed
- **THEN** it prints one sentence with N = 104 and a second with N = 32, each with its own bound

### Requirement: FIH-V1-003 Faults sit on changed lines

Every mutant MUST be placed on a line changed by its PR; a bug-fix revert MUST restore the pre-fix
lines. (G3)

#### Scenario: Unchanged line

- **GIVEN** an operator site outside the PR's changed lines
- **WHEN** mutants are generated
- **THEN** the site is skipped

### Requirement: FIH-V1-004 Baseline and quarantine

The full suite MUST run three times at the base. A test failing in any baseline run MUST be
quarantined and excluded from kill counting. If quarantined tests exceed 2 percent of the suite, the
runner MUST be disqualified. The verdict MUST also report, as a separate count, the quarantined
tests that a plan omitted for a PR touching their dependencies. (G3, review release-tasks 12)

#### Scenario: Flaky baseline

- **GIVEN** 3 percent of tests fail in at least one of three baseline runs
- **WHEN** the baseline completes
- **THEN** the runner verdict is `DISQUALIFIED`

#### Scenario: Quarantined test omitted

- **GIVEN** quarantined test Q whose dependency a sealed PR changes, and that PR's plan without Q
- **WHEN** the verdict is written
- **THEN** the count of omitted quarantined tests is 1 and names Q

### Requirement: FIH-V1-005 A kill is confirmed three times

A test MUST count as killing a fault only if it fails in three of three runs on the faulted tree
and passed in every baseline run. Faults that no test kills MUST NOT count. A PR counts once it has
at least one confirmed kill. (G3)

#### Scenario: Two of three

- **GIVEN** a test failing twice in three faulted runs
- **WHEN** kills are confirmed
- **THEN** it is not a kill

### Requirement: FIH-V1-006 A miss blocks

For each confirmed kill, the selected run (the plan Corvint produced for that PR, executed by the
runner) MUST execute the killing test and observe it fail. Otherwise the case is `MISS`, the runner
verdict is not zero-miss, and the miss MUST NOT be waived as a flake. (G3)

#### Scenario: Killing test not selected

- **GIVEN** a confirmed kill by test T and a plan without T
- **WHEN** the verdict is computed
- **THEN** the case is `MISS` and the release is blocked

### Requirement: FIH-V1-007 PRs are counted and the wording is fixed

The verdict MUST count PRs with a confirmed kill, not mutants, and MUST print the G3 release
sentence with N, the bound 3/N, and the 95 percent Wilson upper bound beside it (settled
release-tasks S6). (G3, review minor 20)

#### Scenario: 104 counted PRs

- **GIVEN** 104 narrowed PRs with confirmed kills and no misses
- **WHEN** the verdict is printed
- **THEN** it reads "0 misses in 104 pre-registered fault-injected cases per runner on pinned repos;
  95 percent upper bound on the per-change miss rate 3/104; not a guarantee."

### Requirement: FIH-V1-008 One adapter per runner on the compiled-in contract

The harness MUST drive Go, Vitest, Jest, Playwright, pytest, RSpec and Minitest through one
compiled-in runner contract that runs a named test set at a commit and returns a per-test outcome.
(G1, G23, owner decision R7 (2026-11-09))

#### Scenario: Unsupported runner

- **GIVEN** a pre-registration naming a runner without an adapter
- **WHEN** the harness starts
- **THEN** it refuses with `runner-adapter-missing`

### Requirement: FIH-V1-009 Pinned and reproducible

Every case MUST run against the PRR-V1 manifest pins (toolchain, lockfile, browser build, repository
SHA, image digest) on Linux or macOS. (G1)

#### Scenario: Different browser build

- **GIVEN** a Playwright manifest pinning one browser build
- **WHEN** the host offers another
- **THEN** the case refuses with `pin-mismatch`

### Requirement: FIH-V1-010 One PR per runner first

Before the full set, the harness MUST run one pre-registered PR per runner and report elapsed time,
so the owner can confirm the compute budget. (G3)

#### Scenario: Cost probe

- **GIVEN** seven sealed sets
- **WHEN** the probe runs
- **THEN** it reports one completed PR and its elapsed time per runner

### Requirement: FIH-V1-011 Go first, built once

The Go adapter MUST be delivered first by generalising `tools/affected-edit-oracle`, and later
adapters MUST reuse the same harness core. (G1)

#### Scenario: Go parity

- **GIVEN** an AEO-V0 mutation case
- **WHEN** run through the harness Go adapter
- **THEN** the miss verdict matches `affected-edit-oracle` on the same inputs

### Requirement: FIH-V1-012 Held-out fault sets

Owner decision R19 (2026-11-09). Each held-out repository named under PRR-V1-004 (fastapi, solidus
and payload, owner decision R21 (2026-11-09)) MUST receive N = 100 faults by this method, frozen
and hashed before execution. Its verdict passes with at most 3 misses in N, reported with the 95
percent Wilson interval (settled release-tasks S6); excluded tests MUST be at most 5 percent of the
suite, or the verdict is `INVALID`. Every receipt MUST carry per-test durations (change
`per-test-durations`). (G12)

#### Scenario: Held-out freeze

- **GIVEN** a held-out repository from the frozen list
- **WHEN** its faults are generated
- **THEN** the hash is committed before any selected run

#### Scenario: Four misses

- **GIVEN** a held-out set of 100 counted faults with 4 misses
- **WHEN** the verdict is computed
- **THEN** the held-out verdict is FAIL and states the Wilson interval

### Requirement: FIH-V1-013 No dependency on removed packages

The harness MUST NOT import `internal/liveverify/pymutate` or `internal/liveverify/rbmutate`. It MAY
reuse `internal/liveverify/mutate` only after the G26 ticket re-owns that package under this spec.
(G26)

#### Scenario: Import check

- **GIVEN** the harness source
- **WHEN** imports are checked
- **THEN** neither removed package appears

### Requirement: FIH-V1-014 Case accounting and the size of N

Owner decision R19 (2026-11-09), review release-tasks 10. The verdict MUST report four counts:
cases sealed, cases dropped because no test killed the fault, cases whose plan was FULL or UNKNOWN,
and cases whose plan was narrowed with a confirmed kill. N is the last count. The pre-registration
MUST over-sample, sealing about 140 PRs per blocking runner, so that N can reach 100. If N is below
100, the runner's verdict MUST be `INVALID`.

#### Scenario: Too few narrowed kills

- **GIVEN** 140 sealed PRs, of which 28 have no kill and 21 have FULL plans
- **WHEN** the verdict is computed
- **THEN** N is 91, the four counts are printed and the verdict is `INVALID`

### Requirement: FIH-V1-015 Line-level proven

Review release-tasks 21. A runner MAY be called line-level proven only when its FIH-V1 verdict is
zero-miss and every counted case's plan was computed at line granularity (tier `line`, owner
decision R16 (2026-11-09)). A runner proven at tier `file`, such as pytest under owner decision R9
(2026-11-09), MUST be reported as file-level.

#### Scenario: File-tier pytest

- **GIVEN** a zero-miss pytest verdict whose plans all carry tier `file`
- **WHEN** the release report is written
- **THEN** pytest is reported as file-level, not line-level proven
