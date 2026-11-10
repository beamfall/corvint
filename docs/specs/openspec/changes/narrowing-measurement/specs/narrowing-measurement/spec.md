# Spec delta: narrowing-measurement

The lane copies each statement below into `narrowing-measurement-v1.md`, inside its
`## Requirements` section, as a numbered definition line that begins
`(proposed 2026-11-09; delivery not-started)`.

## ADDED Requirements

### Requirement: NMS-V1-001 Narrowing rate per runner

The measurement MUST report, for each runner, the number of replayed PRs, the number whose plan is
narrowed, and their ratio. A PR is narrowed for a runner when that runner's tier is `line` or
`file`; tier `flow`, tier `full` and state UNKNOWN do not count as narrowed (owner decision R16
(2026-11-09)). For Go, Vitest, Jest and pytest the runner MUST reach at least 60 percent to pass.
(G4)

#### Scenario: A runner below the bar

- **GIVEN** 100 replayed pytest PRs, 55 of them narrowed
- **WHEN** the measurement runs
- **THEN** the pytest status is FAIL with reason `narrowing-rate-below-bar` and the rate 55 percent

#### Scenario: Flow tier is not narrowed

- **GIVEN** a replayed PR whose Playwright selection has tier `flow`
- **WHEN** the measurement runs
- **THEN** that PR does not count as narrowed for Playwright

### Requirement: NMS-V1-002 Cost ratio from recorded per-test durations

For each narrowed PR the measurement MUST compute the selected tests' summed duration divided by the
full suite's summed duration, using per-test durations recorded from a full baseline run at the
same commit (change `per-test-durations`, owner decision R19 (2026-11-09)) and excluding build and
setup time. The median over replayed PRs MUST be at most 50 percent for Go, Vitest, Jest and pytest
to pass. A PR whose plan is not narrowed counts as 100 percent. The two bars are linked: at exactly
60 percent narrowed, a median of 50 percent or less needs more than 83 percent of the narrowed PRs
at 50 percent or less; the report MUST state this. (G4, review release-tasks 13)

#### Scenario: Durations recorded

- **GIVEN** a baseline run with per-test durations and a plan selecting 30 percent of that time
- **WHEN** the measurement runs
- **THEN** the PR's cost ratio is 0.30 and it enters the median

#### Scenario: Durations missing

- **GIVEN** a runner whose receipts carry only per-invocation time
- **WHEN** the measurement runs
- **THEN** the cost ratio is `NOT_MEASURED` and the runner cannot PASS

### Requirement: NMS-V1-003 Publish-only runners

Playwright, RSpec and Minitest MUST be measured and published with the same fields, with status
PUBLISH-ONLY and no threshold. (G4)

#### Scenario: Minitest numbers

- **GIVEN** a completed Minitest replay
- **WHEN** the report is written
- **THEN** its rate and cost ratio appear with status PUBLISH-ONLY

### Requirement: NMS-V1-004 Zero misses are evaluated first

A runner whose fault-injection verdict (FIH-V1) is missing or not zero-miss MUST have status FAIL,
and its narrowing numbers MUST NOT change that status. (G3, G4)

#### Scenario: Good narrowing, one miss

- **GIVEN** Jest narrows 80 percent of PRs and the fault-injection set records one unselected kill
- **WHEN** the report is written
- **THEN** Jest's status is FAIL with reason `confirmed-miss`

### Requirement: NMS-V1-005 Replay source, Go feasibility check and freeze before the RC

For each runner, the replay source MUST be the merged PRs of its pinned repository in the 12 months
before the manifest SHA, at least 100 PRs (owner decision R19 (2026-11-09), review release-tasks
13). The 200-PR Go replay of Corvint is a feasibility check of the thresholds only and is not a
runner verdict. The bar file (thresholds, per-runner PR lists, their SHA-256 hashes) MUST be
committed before the RC is measured, and the measurement MUST refuse a bar file whose hash differs
from the frozen one. (G4, G10a)

#### Scenario: Edited bar after freeze

- **GIVEN** a frozen bar file and a later edit to a threshold
- **WHEN** the RC measurement runs
- **THEN** it refuses with `bar-file-changed-after-freeze` and measures nothing

#### Scenario: Too few merged PRs

- **GIVEN** a pinned repository with 80 merged PRs in the 12 months before the manifest SHA
- **WHEN** the PR list is built
- **THEN** the runner's result is `INVALID` with reason `replay-source-too-small`

### Requirement: NMS-V1-006 The report states its sample and uncertainty

Each runner report MUST state the commit, repository, PR count, excluded PRs with reasons, the
excluded-test share, and a 95 percent Wilson interval for the narrowing rate (settled release-tasks
S6). Excluded tests (flaky or quarantined in the baseline) MUST be at most 5 percent of the full
suite, or the runner's result is `INVALID` (owner decision R19 (2026-11-09)). (G4)

#### Scenario: Small sample

- **GIVEN** 100 replayed PRs
- **WHEN** the report is written
- **THEN** the rate carries an interval of roughly plus or minus 10 points

#### Scenario: Too many excluded tests

- **GIVEN** a baseline in which 7 percent of tests are excluded
- **WHEN** the report is written
- **THEN** the runner's result is `INVALID` with reason `excluded-share-over-cap`

### Requirement: NMS-V1-007 Freeze and demote

A runner is selective when its FIH-V1 verdict is zero-miss and it is not DEMOTED (settled
release-tasks S5, review release-tasks 22). Verdicts MUST be judged on the milestone M5a
qualification candidate SHA (owner decision R2 (2026-11-09), around 2026-12-08); verdicts taken
after the freeze confirm them on the RC SHA. At the freeze (2026-12-15) every runner without PASS
or PUBLISH-ONLY with a zero-miss verdict MUST be recorded as DEMOTED in the release decision file
with an owner reason, and in 1.0 MUST run its full suite with a printed reason. Demotions MUST be
recorded before the freeze. If fewer than four of the seven runners are selective, the report MUST
state `release-slip` instead of a ship verdict. (G10, G10a)

#### Scenario: Three runners selective

- **GIVEN** three runners at PASS or PUBLISH-ONLY with zero misses and four demoted
- **WHEN** the release report is written
- **THEN** it states `release-slip` and names the four runners

#### Scenario: RC confirms M5a

- **GIVEN** a runner at PASS on the M5a candidate SHA
- **WHEN** its post-freeze verdict on the RC SHA records a miss
- **THEN** the runner is DEMOTED and the report names both SHAs

### Requirement: NMS-V1-008 Kill switch

Owner decision R13 (2026-11-09). `CORVINT_SELECTION` accepts exactly two forms: `full`, which
forces every runner's plan to FULL, and `full:<runner>[,<runner>]`, which forces each named runner
to FULL on every instance; the reason is `kill-switch`. The same grammar applies in
`corvint affected`, `corvint test`, `corvint watch`, the GitHub action, agent hooks and the MCP
tools. An unknown value or runner name MUST exit 2 with `kill-switch-value-unknown` on the CLI
surfaces and in the action, and MUST select all tests, naming the unknown value, in agent hooks and
MCP. The RC qualification MUST exercise both forms. (G10, G16)

This requirement is the only owner of the `CORVINT_SELECTION` grammar. The watch, runner-contract,
pytest, Ruby and GitHub action proposals cite it and do not define their own forms.

#### Scenario: Global switch

- **GIVEN** `CORVINT_SELECTION=full` in the environment
- **WHEN** `corvint affected --base FULL_SHA` runs
- **THEN** the plan state is FULL and the reason `kill-switch` is printed and recorded in the receipt

#### Scenario: Typo on the CLI

- **GIVEN** `CORVINT_SELECTION=ful`
- **WHEN** any selecting CLI command runs
- **THEN** it exits 2 with `kill-switch-value-unknown`

#### Scenario: Typo in an agent hook

- **GIVEN** `CORVINT_SELECTION=ful`
- **WHEN** an agent hook asks for the affected tests
- **THEN** all tests are selected and the answer names the unknown value

### Requirement: NMS-V1-009 Every fallback is printed and listed

Every full-suite fallback, whether from demotion, kill switch, refusal or UNKNOWN, MUST be printed
in the run output and listed in the release manifest with its reason. (G1, G2)

#### Scenario: Demoted runner in a user run

- **GIVEN** Minitest is DEMOTED in the release
- **WHEN** a user runs `corvint affected` in a Minitest repository
- **THEN** the output states that Minitest runs in full and why

### Requirement: NMS-V1-010 Replay corpora and their role

Plain replay over 200 Corvint, 100 Beamfall and 50 held-out PRs MUST report selection size and
over-selection only; it MUST NOT be presented as miss evidence. Non-Go runners MUST be replayed on
their pinned repository from `proof-repositories` (PRR-V1) with a frozen PR list built under
NMS-V1-005 (at least 100 PRs, owner decision R19 (2026-11-09)). (G3, G12)

#### Scenario: Replay report wording

- **GIVEN** a completed plain replay
- **WHEN** the report is written
- **THEN** it reports selection size and over-selection and states that misses are measured by the
  fault-injection set

### Requirement: NMS-V1-011 Agreement figure

Owner decision R20 (2026-11-09). The release report MUST state the agreement figure: the share of
PRs in the soak window whose narrowed-run pass/fail outcome equals the full-run pass/fail outcome
on the same commit, with the PR count and a 95 percent Wilson interval. A PR without both runs on
the same commit is excluded and counted.

#### Scenario: One disagreement

- **GIVEN** 120 soak-window PRs with both runs, 119 with equal outcomes
- **WHEN** the release report is written
- **THEN** it states 119 of 120 agree, with the interval, and lists the disagreeing PR
