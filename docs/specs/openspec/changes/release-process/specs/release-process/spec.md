# Spec delta: `release-process` (change `release-process`)

## ADDED Requirements

### Requirement: REL-V1-001 Milestone M0 re-scopes the v1-0 release object

Before any removal lane starts, every open ticket at milestone `v1-0` MUST either carry a
`requirementRefs` entry naming the `PROD` requirement it serves or be moved to `post-1-0` with the
reason recorded in the ticket. The `v1-0` release object's acceptance criteria and required gates
MUST be rewritten through `corvint-tasks release update` to the plan's 1.0 bar and the PROD-INT-017
gates. (owner decision R3 (2026-11-09); review finding 4)

#### Scenario: An unmapped ticket

- **GIVEN** an open `v1-0` ticket with no `PROD` requirement reference and no move record
- **WHEN** the M0 audit runs
- **THEN** it lists the ticket and M0 is reported incomplete

#### Scenario: A moved ticket

- **GIVEN** a `v1-0` ticket that serves no plan requirement
- **WHEN** it is moved to `post-1-0`
- **THEN** the ticket records the move reason and its prior milestone

### Requirement: REL-V1-002 Milestone M5a qualification candidate

A qualification candidate tag MUST be pushed about 2026-12-08, before the freeze. It MUST be
rebuilt only to fix a blocker, and each rebuild MUST name the blocker ticket. The commit SHA of the
pushed candidate tag MUST seed the held-out repository draw. (owner decision R2 (2026-11-09); G12;
review finding 3)

#### Scenario: A rebuild without a blocker

- **GIVEN** a candidate tag already pushed
- **WHEN** a new candidate is proposed that names no open blocker ticket
- **THEN** the release record refuses it

### Requirement: REL-V1-003 Release calendar

The freeze MUST be 2026-12-15. Stable MUST NOT be promoted before the latest of 2027-01-05, RC
publish plus 21 days, and the end of the 7-day soak. Any code change in the RC MUST restart both the
soak and the Tasks trial clocks. (G10a; owner decision R2 (2026-11-09); review finding 1)

#### Scenario: A late RC publish

- **GIVEN** the RC is published on 2026-12-20
- **WHEN** the stable date is computed
- **THEN** it is 2027-01-10, the RC publish date plus 21 days

#### Scenario: A fix during the soak

- **GIVEN** a code change lands in the RC on soak day 4
- **WHEN** the clocks are read
- **THEN** both the soak and the trial restart from the new RC publish

### Requirement: REL-V1-004 Demotion verdicts at the freeze

The owner's demotion notes MUST rest on the fault-injection and measurement verdicts recorded on one
named freeze-candidate SHA, recorded before the freeze. The RC run MUST repeat those verdicts on the
RC SHA, and any verdict that regresses from the freeze-candidate verdict MUST be a blocking miss. No
demotion may be added after the freeze. (G10; review finding 2; plan checklist O20)

#### Scenario: A regression after the freeze

- **GIVEN** a runner that was selective on the freeze-candidate SHA
- **WHEN** the RC run shows a miss for that runner
- **THEN** the RC is not approved and the record names the runner and the miss

### Requirement: REL-V1-005 Tag-started build that stops before publishing

A pushed `v*` tag MUST start a workflow that runs the full suite and builds linux/amd64,
linux/arm64 and darwin/arm64 with `SHA256SUMS`, a manifest, the licence files and a build
attestation, and then stops. The build job MUST hold only `contents: read`, `id-token: write` and
`attestations: write`. Actions MUST be pinned by commit SHA and tags MUST be protected. (G15a;
owner decision R24 (2026-11-09); ci-agents review F11)

#### Scenario: A tag without approval

- **GIVEN** a pushed candidate tag
- **WHEN** the owner does not approve the publish job
- **THEN** the artifacts and attestation exist as workflow artifacts and nothing is published

#### Scenario: A failed suite

- **GIVEN** a pushed tag whose full suite fails
- **WHEN** the workflow runs
- **THEN** no attestation is produced and the candidate is not offered for approval

### Requirement: REL-V1-006 Attestation supersedes decision 0433 for attestation only

The release MUST carry a build attestation, superseding decision 0433 item 1 for the build
attestation and npm provenance only. The support window of decision 0433 item 2 and its rule that no
agent creates or uses a signing identity MUST stay in force. (settled without the owner; G15a;
G20; plan checklist O26)

#### Scenario: An agent signing identity

- **GIVEN** the tag workflow
- **WHEN** its configuration is reviewed
- **THEN** it uses only the GitHub OIDC attestation and no key or signing identity held by an agent

### Requirement: REL-V1-007 Owner approval publishes, and M8 is approval and publish

Only the publish job MAY hold `contents: write`. It MUST run in a protected environment bound to
protected `v*` tags and only after the owner's explicit approval. Milestone M8 MUST be "RC approval
and publish": the owner's first signature (acceptance record) and the environment approval. The
soak clock MUST start at RC publish. Stable promotion MUST need the owner's second signature and a
second explicit approval. (G10; G15a; review finding 3)

#### Scenario: Approval by anyone else

- **GIVEN** a reviewer other than the owner on the protected environment
- **WHEN** the environment settings are checked before M8
- **THEN** the check reports that only the owner can approve, or M8 does not start

### Requirement: REL-V1-008 Release artifacts per product for three targets

`tools/release-binary/binaries.json` MUST list exactly the targets linux/amd64, linux/arm64 and
darwin/arm64, and MUST group binaries by product (testing, docs, shared update tool). A binary
retired by a removal batch MUST leave the list in that batch. corvint-tasks MUST ship the same three
targets from its own repository, and `corvint-update` MUST cover both components. (review finding
18; owner decision R14 (2026-11-09); answer 16)

#### Scenario: A windows target

- **GIVEN** a `binaries.json` that lists windows/amd64
- **WHEN** the release binaries check runs
- **THEN** it fails and names the target

### Requirement: REL-V1-009 Removals finish by 2026-12-01

Every M3 removal batch MUST be merged by 2026-12-01. A batch not merged by then MUST move to 1.0.x
and its removals MUST NOT land before the RC. (settled without the owner; plan checklist O24)

#### Scenario: A late batch

- **GIVEN** a removal batch still open on 2026-12-02
- **WHEN** the milestone record is updated
- **THEN** the batch is listed under 1.0.x and the binaries it removes stay in the 1.0 list

### Requirement: REL-V1-010 Soak and trial counts

The soak count MUST count PRs merged during the soak window in Corvint or Beamfall repositories,
read from the GitHub API. Held-out upstream PRs replayed on the RC MUST be reported as a separate
count. Each Tasks trial repository MUST record at least 50 ticket writes and 5 completions, read
from the store event log. (G10; review finding 14; plan checklist O21, O22)

#### Scenario: Too little trial activity

- **GIVEN** a trial repository with 40 ticket writes after 21 days
- **WHEN** the trial record is read at stable promotion
- **THEN** the trial is reported below its activity floor and stable is not promoted

### Requirement: REL-V1-011 Reserved benchmark hosts

Linux qualification timing MUST run on WSL2 on the Windows benchmark box and macOS timing on the
owner's Mac. Both MUST run no lanes from M5a to RC publish. (review finding 17; plan checklist O23)

#### Scenario: A lane on the benchmark host

- **GIVEN** a lane recorded on the benchmark box during the reserved window
- **WHEN** the qualification record is assembled
- **THEN** timing results from that window are NOT_VERIFIED and rerun

### Requirement: REL-V1-012 Release gates

The release gates MUST be the affected plan, exhaustive `go test` and `go vet` in both modules, and
one scripted qualification per product: the PROD-INT-004 "install alone" scenario plus that
product's primary verb on a pinned repository. The CEM, OCM and dogfood steps MUST NOT be release
gates. (G8; PROD-INT-017; plan checklist O29)

#### Scenario: A CEM gate in the release object

- **GIVEN** the rewritten `v1-0` release object
- **WHEN** its required gates are listed
- **THEN** none of them runs a `corvint cem` or dogfood command

### Requirement: REL-V1-013 Signed release record before publish

The RC publish and the stable promotion MUST each be preceded by the owner's signature in the
acceptance record (change `acceptance-record`), listing each requirement's evidence or demotion, the
NOT_RUN and NOT_OBSERVED items and the deferred P2 and P3 tickets. (G10; PROD-INT-012)

#### Scenario: No signature

- **GIVEN** an approved-looking environment run with no RC signature in the acceptance record
- **WHEN** the publish job starts
- **THEN** it refuses and names the missing signature

## MODIFIED Requirements

### Requirement: PRS-V1-004 Release platforms

Release platforms MUST be linux/amd64, linux/arm64 and darwin/arm64 native binaries. Windows is
supported only through WSL; native Windows is stated unsupported in 1.0. (Previously darwin/arm64
and linux/amd64 only; changed by answer 16, G15a and the settled three-platform decision.)

#### Scenario: linux/arm64

- **GIVEN** a release candidate
- **WHEN** its artifacts are listed
- **THEN** a linux/arm64 archive with its checksum and attestation is present
