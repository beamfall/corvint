# Spec delta: `macos-required-shards` (change `macos-required-shards`)

## ADDED Requirements

### Requirement: MRS-V1-001 Exhaustive macOS run gates the release

The release workflow MUST run exhaustive `go test` and `go vet` in both modules on `macos-15` for the
freeze-candidate SHA and the RC SHA. A failure MUST block the candidate. (answer 16; PROD-INT-015)

#### Scenario: A darwin-only failure

- **GIVEN** an RC SHA where one test fails only on macOS
- **WHEN** the release workflow runs
- **THEN** the RC is not offered for approval and the record names the test

### Requirement: MRS-V1-002 The `go-macos` aggregate is a required check on `main`

Once every hosted-macOS flake ticket is closed, the `go-macos` aggregate MUST be a required status
check on `main`. The workflow MUST run on `pull_request` and `merge_group`, and neither the shard jobs
nor the aggregate MAY use `continue-on-error`. (answer 16; PROD-INT-015)

#### Scenario: A macOS failure in the merge queue

- **GIVEN** a PR in the merge queue whose darwin shard fails
- **WHEN** the queue evaluates required checks
- **THEN** the PR is removed from the queue

### Requirement: MRS-V1-003 Only a verified docs-only plan skips, and the check still reports

The cadence DEFER decision MUST NOT skip the macOS shards. A verified docs-only plan MAY skip them,
and the aggregate MUST then report success with the reason `docs-only`. Timing budgets on hosted
macOS MUST NOT fail the aggregate. (answer 16; MCJ-V0-019)

#### Scenario: A docs-only PR

- **GIVEN** a PR whose verified plan is docs-only
- **WHEN** the workflow runs
- **THEN** no shard runs and `go-macos` reports success with reason `docs-only`

#### Scenario: A timing miss

- **GIVEN** a shard whose plan p95 exceeds `LINECOV_PLAN_P95_BUDGET`
- **WHEN** the aggregate runs
- **THEN** it prints the miss and passes if every test passed

### Requirement: MRS-V1-004 macOS flakes are 1.0 blockers

A macOS test that fails and then passes on the same SHA MUST be filed as a ticket at milestone
`v1-0`, priority P1, with the run URL, test ID and both outcomes. The RC MUST NOT be approved while
such a ticket is open. (answer 16)

#### Scenario: A rerun hides a flake

- **GIVEN** a shard that failed and passed on rerun
- **WHEN** no flake ticket names that test
- **THEN** the rerun result is not accepted as the check result for release evidence

### Requirement: MRS-V1-005 Native Windows leaves the 1.0 bar

Native Windows MUST NOT be a 1.0 release target, and native Windows vet or archive work MUST NOT be
required for 1.0. The documentation MUST state native Windows is unsupported and point to WSL.
(answer 16; PROD-INT-015)

#### Scenario: A Windows user

- **GIVEN** the 1.0 install guide
- **WHEN** a reader looks for Windows
- **THEN** it says native Windows is unsupported and gives the WSL path

### Requirement: MRS-V1-006 `make windows-vet` once per removal batch

Removal lanes MUST run `make windows-vet` once per removal batch, before that batch's push, when the
batch touches a `.go`, `go.mod` or `go.sum` file. Other lanes keep the per-push rule. (owner decision
R36 (2026-11-09))

#### Scenario: A removal batch of three lanes

- **GIVEN** three removal lanes merged into one batch branch
- **WHEN** the batch is pushed
- **THEN** `make windows-vet` ran once on the batch head before the push

### Requirement: MRS-V1-007 macOS evidence in the acceptance record

The acceptance record for the RC MUST list the macOS exhaustive run on the RC SHA, the state of the
required check, and the closed flake tickets. (answer 16; PROD-INT-012)

#### Scenario: Missing macOS evidence

- **GIVEN** an RC acceptance record without the macOS run
- **WHEN** the owner reviews it
- **THEN** PROD-INT-015 is listed as NOT_RUN for the RC
