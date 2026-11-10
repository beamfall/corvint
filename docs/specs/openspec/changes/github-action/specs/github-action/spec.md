# Spec delta: `github-action` (change `github-action`)

## ADDED Requirements

The lane copies each statement into `docs/specs/github-action-v1.md` inside one `## Requirements`
section, as a numbered definition line beginning `(proposed 2026-11-09; <ticket>; delivery
experimental)`. "Blocks 1.0" and "preview" follow G16.

### Blocks 1.0: the action core

### Requirement: GHA-V1-001 One action plus a starter workflow, no account

Source: G16, G17; review fix F13. `beamfall/corvint-action` MUST provide one composite action plus
a starter workflow to copy (plan job, run job, result job), MUST need no account and no secret, and
MUST work in a job whose only permission is `contents: read`.

Earlier wording, withdrawn because a plan job, a matrix run job and a result job cannot be one
composite step: "one composite action usable with one `uses:` line". Whether the repository also
ships a reusable workflow is open (NEW-CI-2).

#### Scenario: Least-privilege job

- **GIVEN** a workflow whose job declares `permissions: contents: read` and no secrets
- **WHEN** the job uses the action on a pull request
- **THEN** the action plans and runs the selected tests without requesting any other permission

### Requirement: GHA-V1-002 Verified binary, pinned version

Source: G16, G15a; owner decision R14 (2026-11-09); review fix F6. Each action release MUST pin one
exact Corvint version and, for each binary it downloads, the SHA-256 of that binary's archive on
each supported platform (one digest per binary and platform) in its own source. The binaries are
`corvint`, every provider binary the selected runners still need until that provider is folded into
`corvint` after G23 (R14), and `corvint-docs` only when the optional docs drift step is enabled. The
action MUST download only from the published GitHub release of `beamfall/corvint`, MUST check each
archive against its pinned digest and against the release `SHA256SUMS`, MUST check the archive's
internal checksums before running anything from it, and MUST make no other network request of its
own after the download. Toolchain and dependency setup steps written by the adopter are outside the
action's code and outside this rule.

Earlier wording, superseded: "the SHA-256 of each supported platform archive". Releases ship one
archive per binary per target (`tools/release-binary/build.go`, `archiveName`, integ 9ec3a29d0).

#### Scenario: Tampered archive

- **WHEN** the downloaded archive's digest differs from the pinned digest
- **THEN** the job fails with a typed message, no binary is executed and no test runs

### Requirement: GHA-V1-003 Base resolution per event

Source: G16. The base MUST be the merge base of the base and head commits on `pull_request`,
`merge_group.base_sha` on `merge_group`, and `before` on `push`, deepening the clone as needed. When
the base cannot be resolved, the action MUST run every test and MUST print `BASE_UNRESOLVED`.

#### Scenario: First push of a new branch

- **WHEN** a `push` event carries an all-zero `before`
- **THEN** every test runs and the summary gives the reason `BASE_UNRESOLVED`

GHA-V1-017 replaces the `pull_request` clause of this requirement. The `push` clause stays as written
(owner decision R23 (2026-11-09): narrow on `pull_request` and on `push`, with the `push` diff taken
from `before`). Known limit: if the run for the previous push failed or was cancelled, its commits
are not in the next push's diff; the scheduled full run (GHA-V1-013) is the backstop. Whether to use
the last successful run's commit instead is open (NEW-CI-1).

### Requirement: GHA-V1-017 Pull request diff ends at the tested tree

Source: review fix F4, settled without the owner. On `pull_request`, `actions/checkout` tests the
merge commit `refs/pull/N/merge`. The action MUST diff `HEAD^1..HEAD` of that tested merge commit.
It MAY diff merge-base..head only when the job checks out the head SHA itself. The diff end MUST be
the commit whose tree the tests run on.

Failure mode: `HEAD` is not a merge commit when a merge checkout was expected, or `HEAD^1` is
missing from a shallow clone; the action deepens the clone once, and if it still cannot resolve the
base it runs every test with `BASE_UNRESOLVED` (GHA-V1-003).

Rollback: restore the merge-base..head clause of GHA-V1-003; selections then follow the head
commit's diff as before.

#### Scenario: Base branch moved after the pull request opened

- **GIVEN** a pull request whose base branch gained a commit that changes a tested file
- **WHEN** the action runs on the merge checkout
- **THEN** the diff is `HEAD^1..HEAD` of the merge commit and the plan selects for the tree under test

### Requirement: GHA-V1-018 One plan artifact with a digest

Source: owner decision R22 (2026-11-09); review fixes F2, F5. The plan job MUST compute the plan once
and upload it as one artifact. The plan command MUST write schema-versioned JSON (proposed profile
`corvint-ci-plan/0`) holding at least: the base and head commits, the per-runner selections, the
matrix with its shards, the `full` flag, the reason codes, and the SHA-256 digest of the plan bytes.
Every run shard and the result job MUST read this artifact and check its digest. A reader MUST refuse
an unknown profile version or an unknown member with exit 2. The conformance test of GHA-V1-004
MUST bind to this schema.

#### Scenario: Unknown plan version

- **GIVEN** a plan artifact with profile `corvint-ci-plan/1`
- **WHEN** a shard built for `/0` reads it
- **THEN** it exits 2 naming the profile and runs no test

### Requirement: GHA-V1-004 The plan and matrix come from the binary

Source: G16, answer 6; owner decision R22 (2026-11-09); review fix F5. The action MUST obtain the
plan, the per-runner matrix and the shard split from one `corvint` read command (proposed name
`corvint ci plan`; the lane records the final name here) and MUST expose them as step outputs usable
with `fromJSON`: the plan artifact path, the plan digest, the matrix, a `full` flag and the reason
codes. The command's output MUST follow GHA-V1-018. The action's own code MUST contain no selection
rule.

#### Scenario: Same plan as local

- **WHEN** the action and `corvint affected --base SHA` run on the same commit pair and inputs
- **THEN** the action's selected tests equal the command's selections

### Requirement: GHA-V1-005 Run exactly the selected tests

Source: G16, PROD-TEST-006; owner decision R22 (2026-11-09); review fix F2. Each matrix entry MUST
run its runner's selected tests from the plan artifact of GHA-V1-018, through `corvint test
--placement measured --shard-plan PATH` plus a runner selector that this change adds to `corvint
test` (proposed `--runner NAME`), and MUST run no test outside the plan, except when GHA-V1-006
widens the run. A shard MUST NOT plan again on its own. A shard MUST refuse, and fail, when the
digest of the plan it reads differs from the digest published by the plan job.

Earlier wording, withdrawn: "through `corvint test --base SHA --shard I/N`". `corvint test` has no
runner filter today; its usage lists `--shard I/N`, `--placement ... [--shard-plan PATH]`,
`--manifest PATH`, `--receipt PATH` and `--accept-stale-binary`, and `--shard` splits across every
runner (`cmd/corvint/test.go`, `testHelp`, integ 9ec3a29d0).

#### Scenario: Two runners, three shards

- **WHEN** the plan selects Go and Vitest tests and asks for three Go shards
- **THEN** four run jobs start, each reads the same plan digest, and together they run exactly the
  selected tests

#### Scenario: Plan digest mismatch

- **GIVEN** a shard job that reads a plan file whose digest differs from the published digest
- **WHEN** the shard starts
- **THEN** it runs no test, fails with a typed message, and the result job fails

### Requirement: GHA-V1-006 Unsure means run everything

Source: G16, answer 2, G2. When the binary refuses, times out, or reports UNKNOWN or FULL for a
runner, that runner MUST run its full suite; when the plan as a whole fails, every runner MUST run
its full suite. Each widening MUST print a one-line reason.

#### Scenario: Plan command crashes

- **WHEN** the plan command exits with an error
- **THEN** every runner runs its full suite and the summary names the error code

### Requirement: GHA-V1-007 The non-code file rule cannot be weakened

Source: G15b. The action MUST apply the binary's rule for changed non-code files unchanged (full by
default; narrowed only by an accepted, fault-proven map entry; never narrowed for the G15b list) and
MUST offer no input that disables or relaxes it.

#### Scenario: Lockfile change

- **WHEN** a pull request changes only `go.sum`
- **THEN** every Go test runs

### Requirement: GHA-V1-008 Config and event trust

Source: G16; owner decision R23 (2026-11-09); owner decision R24 (2026-11-09); owner decision R31
(2026-11-09); review fix F7.

- A pull request from a fork MUST run every test, with reason `fork-pull-request` (R23). This keeps
  the current behaviour of Corvint's own CI, which narrows only same-repository pull requests
  (`.github/workflows/ci.yml` lines 535 and 586, `head.repo.full_name == github.repository`,
  integ 9ec3a29d0).
- A change to a Corvint configuration file under `.corvint/` (as defined by NCM-V1-006 in change
  `non-code-map`: every file there except the flow file and its link records, which select the tests
  linked to the changed flows, and the two private ledgers, which add nothing), to the narrowing
  map, to the provider declaration file, to the workflow file that calls the action, or to the pinned
  action version MUST run every test (R31).
- The action MUST refuse to run on `pull_request_target`, and the documentation MUST forbid checking
  out pull request code under `pull_request_target` (R24).
- The documentation MUST state the threat model plainly: on a pull request the workflow file and the
  action pin come from the pull request's merge ref, so these rules prevent accidental narrowing, not
  a hostile change. The controls against a hostile change are maintainer review of `.github/**` and
  `.corvint/**` (for example through CODEOWNERS), approval of workflows from forks, and branch
  protection.

Earlier wording, superseded by R23 and R31: "On a pull request from a fork the action MUST read
Corvint configuration from the base ref", with the scenario "Fork edits the narrowing map" (the map is
read from the base ref and every test runs). Fork pull requests now always run everything, so reading
configuration from the base ref is no longer needed.

#### Scenario: Fork pull request

- **GIVEN** a pull request from a fork that changes one Go file
- **WHEN** the action runs
- **THEN** every test runs and the summary gives the reason `fork-pull-request`

#### Scenario: Same-repository change to a `.corvint/` configuration file

- **GIVEN** a same-repository pull request that changes `.corvint/test-read-scopes.json`
- **WHEN** the action runs
- **THEN** every test runs and the summary names the path

#### Scenario: `pull_request_target`

- **WHEN** the action is started by a `pull_request_target` event
- **THEN** it exits non-zero with a typed message and runs nothing

### Requirement: GHA-V1-009 Kill switch

Source: G10, G16; owner decision R13 (2026-11-09). When `CORVINT_SELECTION` is `full`, every test
of every runner MUST run; when it is `full:<runner>[,<runner>]` (for example `full:go,jest`), those
runners MUST run fully. The binary MUST read the same variable, so the switch also works outside the
action. The summary MUST say that the switch is active and MUST print the raw value it received. An
unknown value MUST make the plan step exit 2 with a message naming the value, so the run fails in CI
(R13). The switch MUST be exercised in the release candidate.

The value grammar is owned by NMS-V1-008 (narrowing-measurement), and the action passes the variable
through unchanged. Behaviour for an unknown value: owner decision R13 (2026-11-09).

Earlier wording, superseded by R13: the per-runner form `go=full,jest=full`, and the scenario value
`jest=full`.

#### Scenario: Switch set for one runner

- **WHEN** the repository variable is `full:jest`
- **THEN** Jest runs fully, other runners follow the plan, and the summary names the switch

#### Scenario: Unknown value

- **WHEN** the repository variable is `go=full`
- **THEN** the plan step exits 2 naming the value, and the result job fails

### Requirement: GHA-V1-010 One always-running result job

Source: G16; owner decision R22 (2026-11-09); review fix F3. The starter workflows MUST include one
result job with a stable name that runs on every outcome. It MUST compare the expected shard list in
the plan artifact with the shard receipts it received, through `corvint test verify-shards
--manifest PATH RECEIPT...` (present in `cmd/corvint/test.go`, integ 9ec3a29d0), and MUST pass only
when the plan job succeeded and every expected shard left a passing receipt bound to the same plan
digest. It MUST fail when any expected shard failed, was cancelled, was skipped or did not start.
A plan that selects no test for any runner MUST skip the run job through `if:` (GitHub rejects an
empty matrix), and the result job MUST then pass only because the plan lists no expected shard,
never because a skipped job counts as success.

Earlier wording, withdrawn as self-contradictory: "fails when any shard ... did not start, and treats
a runner with no selected tests as passing".

#### Scenario: A shard is cancelled

- **WHEN** one run shard is cancelled
- **THEN** the result job fails and names the shard

#### Scenario: Empty plan

- **GIVEN** a plan that selects no test (for example a documentation-only change)
- **WHEN** the run job is skipped through `if:`
- **THEN** the result job passes and states that the plan lists no expected shard

#### Scenario: Plan job fails

- **WHEN** the plan job fails before writing the plan artifact
- **THEN** the run job does not start and the result job fails, naming the plan job

#### Scenario: Run job skipped while the plan lists shards

- **GIVEN** a plan that lists two expected shards
- **WHEN** the run job is skipped
- **THEN** the result job fails, naming both missing shards

### Requirement: GHA-V1-011 Job summary

Source: G16, G2. The action MUST write a job summary produced by the binary that states, per runner,
the number of selected and total tests, whether it narrowed or ran fully and why, every fallback
reason, the kill switch state, and the Corvint version and archive digest. Time saved MAY be shown
only from recorded per-test durations, labelled as an estimate, and otherwise as `NOT_OBSERVED`.
The summary MUST be bounded with a truncation marker and totals.

#### Scenario: No recorded durations

- **WHEN** no per-test durations are recorded
- **THEN** the summary shows time saved as `NOT_OBSERVED`

### Requirement: GHA-V1-012 Platforms

Source: answer 16, G15a, G16. The action MUST run on Linux (amd64, arm64) and macOS (arm64) runners
and MUST refuse a Windows runner with a typed message.

#### Scenario: Windows runner

- **WHEN** the job runs on `windows-latest`
- **THEN** the action exits non-zero with a message that Windows runners are unsupported in 1.0

### Requirement: GHA-V1-013 Full mode for periodic runs

Source: answer 6. The action MUST offer `mode: full`, which runs every test and records the plan the
binary would have produced for the same range. The documented setup MUST include a scheduled full
run on the default branch (proposed daily).

#### Scenario: Scheduled run

- **WHEN** the scheduled workflow runs in `mode: full`
- **THEN** every test runs and the artifact holds both the full results and the plan

### Requirement: GHA-V1-014 Pinned versions

Source: G15a, G17. The action MUST be tagged `v1` (moving major) and `v1.x.y` (fixed, never moved
or deleted). The documentation MUST recommend pinning by full commit SHA. Every action used inside
the composite action MUST be pinned by full commit SHA. A new Corvint version MUST arrive only
through a new action release.

#### Scenario: Release tag immutability

- **WHEN** `v1.2.0` has been published
- **THEN** it always resolves to the same commit, and `v1` moves only by a new release

### Requirement: GHA-V1-015 Licence

Source: G17; owner decision R25 (2026-11-09). `beamfall/corvint-action` MUST carry the Apache-2.0
licence, the same licence as `beamfall/corvint` as a whole (`LICENSING.md`, decision 0586, integ
9ec3a29d0).

Earlier wording, withdrawn because it was wrong: "the same licence as the `protocol/**` boundary in
`beamfall/corvint`" and "The owner fixes the choice when accepting". Since decision 0586 the whole
repository is Apache-2.0 with no path boundary.

#### Scenario: Licence file

- **WHEN** the repository is created
- **THEN** its `LICENSE` holds the Apache-2.0 text and the README states it

### Requirement: GHA-V1-016 Release of the action repository

Source: G15a, extended to the action repository by this proposal (owner to confirm). A pushed
`v1.x.y` tag MUST run the action's tests on Linux and macOS, then stop; moving `v1` and publishing
the GitHub release MUST run only after owner approval in a protected environment, in the only job
with write permission.

#### Scenario: Tag without approval

- **WHEN** a `v1.3.0` tag is pushed and nobody approves
- **THEN** tests run, no release is published and `v1` does not move

### Preview at 1.0 (specified, never release-blocking)

### Requirement: GHA-V1-020 Shadow mode and would-be-miss report

Source: G16 (preview for adopters), G18 (needed on Corvint's own CI before the freeze).
`mode: shadow` MUST run the full suite, compute the plan for the same range, and report the selected
count, the full count and every would-be miss: a test that failed in the full run and was not in
the plan. The report MUST be a machine-readable artifact and a summary section. The documentation
MUST state that shadow mode runs the full suite on every pull request, so its cost equals a full run
(for Corvint's own repository, `cmd/corvint` alone takes about 591 s, AGENTS.md "Verify").

#### Scenario: A would-be miss

- **WHEN** a test fails in the full run and the plan did not select it
- **THEN** the report lists it as a would-be miss with the plan's reason for leaving it out

### Requirement: GHA-V1-021 PR comment from the review evidence check

Source: G16 (preview), G25 item 2; owner decision R4 (2026-11-09); owner decision R23 (2026-11-09);
review fix F8. A separate workflow triggered by `workflow_run`, the only part holding
`pull-requests: write` (plus `actions: read`, needed to download the other run's artifact), MUST
post or update one comment per pull request from the `corvint review` evidence check, reading the
plan artifact as data and checking out no pull request code. The comment MUST contain: the plan per
runner (narrowed or full, with reason), changed lines whose selected tests did not run or failed,
changed flows with an untested variation, unlinked (`unassigned`) E2E tests, stale docs, and ticket
acceptance criteria without evidence. It MUST contain no source lines, no secrets and no environment
values, and MUST be bounded. In addition:

- The pull request MUST be resolved from the run's head SHA through the GitHub API, never taken from
  the artifact; `workflow_run.pull_requests` is always empty for a fork pull request
  (`.github/workflows/ci-control-plane.yml` lines 51-55, integ 9ec3a29d0).
- Artifact text MUST be treated as hostile: mentions and links MUST be escaped, and artifact data
  MUST NOT be interpolated with `${{ }}` into any `run:` step.
- The workflow MUST update only the one comment that carries its marker and was written by its own
  identity; it MUST NOT edit any other comment.
- The comment is advisory: it never sets a failing status (R4).

#### Scenario: Comment from a fork pull request

- **WHEN** a fork pull request completes its run
- **THEN** the comment workflow resolves the pull request from the head SHA and posts from the
  artifact without executing fork code

#### Scenario: Hostile artifact

- **GIVEN** a fork pull request whose artifact text holds `@org/team`, a link, a `${{ secrets.X }}`
  string and another comment's marker
- **WHEN** the comment workflow runs
- **THEN** the mention and link are escaped, the string is printed as text and never evaluated, and
  only the workflow's own marked comment is created or updated

### Requirement: GHA-V1-022 Docs drift step

Source: G16 (preview), answer 8; owner decision R4 (2026-11-09). An optional step MUST run the
`corvint-docs` drift check and report stale documents in the summary. The step is advisory by default
and MUST NOT fail the job; one documented opt-in input makes it blocking. When `corvint-docs` is not
installed the step reports `NOT_OBSERVED` and does not fail.

#### Scenario: Stale page

- **WHEN** a change makes a documented command stale
- **THEN** the summary lists the page and the cited source

### Requirement: GHA-V1-023 Flow gaps step

Source: G16 (preview), G11. An optional step MUST report `corvint flows gaps` and the count of
`unassigned` E2E tests in the summary.

#### Scenario: New unlinked E2E test

- **WHEN** a pull request adds an E2E test linked to no flow
- **THEN** the summary counts it as `unassigned` and the test runs

### Requirement: GHA-V1-024 Tasks ticket-gate step

Source: G16 (preview), G9 integration I1; owner decision R7 (2026-11-09). An optional step MUST emit
the `corvint test` receipt ID as a step output for a ticket gate, and MUST NOT write to any Tasks
store. The receipt is found at the default location fixed by PROD-TEST-020 (the append-only run
result record), not at a path chosen by the workflow.

#### Scenario: Receipt for a ticket

- **WHEN** the step runs for a pull request that names a ticket
- **THEN** the receipt ID is an output and no store is opened for writing

### Requirement: GHA-V1-025 Starter workflows per runner

Source: G16 (preview), G1. The action repository MUST ship one starter workflow for each of the seven
blocking runners (Go, Vitest, Jest, Playwright, pytest, RSpec, Minitest), each exercised in that
runner's qualification repository (G12a).

#### Scenario: pytest starter

- **WHEN** the pytest starter is copied into the pytest qualification repository
- **THEN** it plans, runs and reports with no edits other than the pinned versions

### Requirement: GHA-V1-026 Index cache reuse

Source: G16 (preview). The action MAY restore a Corvint index from the Actions cache keyed by Corvint
version and commit, and MUST use it only if it matches both and passes its integrity check; otherwise
it MUST rebuild. A cache MUST NOT change which tests are selected.

#### Scenario: Cache from another version

- **WHEN** the restored cache was written by a different Corvint version
- **THEN** it is discarded, the index is rebuilt and the plan is unchanged

### Corvint's own CI (G18, G27)

### Requirement: GHA-V1-030 Shadow next to the bespoke scripts

Source: G18; owner decision R2 (2026-11-09); review fixes F9, F10. Corvint's own CI MUST run the
action in shadow mode next to the bespoke jobs on every pull request. A pull request matches when the
action's result verdict equals the bespoke required verdict, the action's selected set contains every
test the bespoke path selected (or each difference is listed with a reason), and the shadow report
has no would-be miss. The list of matching pull request numbers MUST be recorded in a build-log entry.

The shadow can start only from the first release published through GHA-V1-041, because GHA-V1-043
requires the action's digests to come from such a release. Under owner decision R2 that is the M5a
qualification candidate (about 2026-12-08). On Corvint's own repository the shadow MUST pass
`--accept-stale-binary`: otherwise a released binary refuses with `self-binary-stale` whenever the
pull request's non-test Go source differs from the binary's revision (`binaryStaleAgainst` in
`cmd/corvint/self_binary.go`, integ 9ec3a29d0), every plan widens to full, and every pull request
matches trivially. Seeded faults or replayed failing commits as miss evidence, and whether this
shadow is the PROD-TEST-012 acceptance, are open (D18).

#### Scenario: Counting

- **WHEN** 30 pull requests merged after shadow start have matched, with no would-be miss on any
  pull request in that period
- **THEN** the cut-over criterion is met

### Requirement: GHA-V1-031 What the cut-over replaces

Source: G27, G18. At cut-over the action MUST replace `tools/gate-ledger`, `script/gate-receipt`,
`script/go-archive-gate` and `script/adopter-ci-advisory.sh` (G27), and the bespoke selection inputs
`.github/forward-rows/**`, `tools/ci-reuse-plan`, `tools/docs-ci-plan` and the variables
`CORVINT_PR_NARROWING`, `CORVINT_MQ_NARROWING` and `CORVINT_LINECOV_NARROWING` (G18). The second
list is an inventory to be confirmed by the lane before removal. The cut-over MUST be its own
`.github` pull request, and the required-check list MUST be checked in the same change.

#### Scenario: No orphan caller

- **WHEN** the cut-over lands
- **THEN** `git grep` finds no caller of a removed script or variable outside history files

### Requirement: GHA-V1-032 Deadline

Source: G18, G10a. If the criterion in GHA-V1-030 is not met by 2026-12-15, the bespoke scripts MUST
stay for 1.0 and the cut-over MUST be the first 1.0.x change.

#### Scenario: Not matched by the freeze

- **WHEN** only 22 pull requests have matched on 2026-12-15
- **THEN** 1.0 ships with the bespoke scripts and the shadow job continues

### Requirement: GHA-V1-033 Daily full run on Corvint's own main

Source: answer 6, G18. Corvint's own CI MUST run the action in `mode: full` on `main` at least daily
and report any would-be miss.

#### Scenario: Miss found by the daily run

- **WHEN** the daily full run finds a failing test the plan would have omitted
- **THEN** it is reported as a would-be miss and a ticket is filed by a person from the report

### Release publishing for `beamfall/corvint` (G15a)

### Requirement: GHA-V1-040 Tag starts the release build, which stops

Source: G15a; owner decision R24 (2026-11-09). A pushed, protected version tag MUST start a workflow
that runs the full Go test and vet commands in both modules, builds `linux/amd64`, `linux/arm64` and
`darwin/arm64` archives, writes `SHA256SUMS`, the release manifest and the licences, and produces a
build attestation, and then stops without publishing. The job that produces the attestation MUST hold
only `contents: read` and `id-token: write` (R24), plus `attestations: write` if GitHub requires it
(NOT_VERIFIED). Which job produces the attestation (the build job or the publish job after approval)
is open (D17).

#### Scenario: Tag pushed

- **WHEN** `v1.0.0-rc.1` is pushed
- **THEN** the build artifacts exist on the run and no release is published

### Requirement: GHA-V1-041 Owner approval publishes

Source: G15a, G10; owner decision R24 (2026-11-09). The publish job MUST run only after owner
approval in a protected GitHub environment and MUST be the only job with repository or release write
permission (`contents: write`). `id-token: write` used only for the attestation of GHA-V1-040 is not
counted as write permission here. Where GHA-V1-040 to GHA-V1-043 finally live (this spec or
`public-release-v0.md`) is recorded by change `acceptance-record`.

#### Scenario: Approval

- **WHEN** the owner approves the environment
- **THEN** the publish job creates the release from the stopped run's artifacts, unchanged

### Requirement: GHA-V1-042 Protected tags and pinned actions

Source: G15a. Version tags MUST be protected, every action in the release workflow MUST be pinned by
full commit SHA, and 1.0 MUST ship no notarization, install script or Homebrew tap.

#### Scenario: Unpinned action

- **WHEN** a release workflow step names an action by tag
- **THEN** the workflow check refuses the change

### Requirement: GHA-V1-043 The action consumes only these releases

Source: G15a, G16. The digests pinned in an action release MUST be copied from the `SHA256SUMS` of a
release published by GHA-V1-041.

#### Scenario: Pin update

- **WHEN** an action release pins Corvint 1.0.1
- **THEN** its digests equal the published 1.0.1 `SHA256SUMS` lines
