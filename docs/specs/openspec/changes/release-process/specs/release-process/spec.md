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

The seed MUST be the commit SHA of the first pushed candidate tag. The draw MUST NOT be redrawn
on a rebuild; REL-V1-014 says what a rebuild repeats. (decided 2026-11-09 under R42, D16)

The draw is the seeded draw of PRR-V1-004: one held-out repository for each family (Go, JS/TS,
Python, Ruby), taken from that family's pool, which PRR-V1-011 seals and hashes before the first
candidate tag is pushed. No fixed list of held-out repositories applies. (owner decision H3
(2026-11-10))

#### Scenario: A blocker rebuild keeps the draw

- **GIVEN** a held-out draw seeded by the first pushed candidate tag
- **WHEN** a second candidate is built to fix a named blocker
- **THEN** the held-out repositories, the pool, the rule hash and the pass rules are unchanged

#### Scenario: The first candidate tag seeds the draw

- **GIVEN** four sealed family pools and no candidate tag yet pushed
- **WHEN** the first candidate tag is pushed
- **THEN** its commit SHA is recorded as the seed and one repository per family is drawn

#### Scenario: A rebuild without a blocker

- **GIVEN** a candidate tag already pushed
- **WHEN** a new candidate is proposed that names no open blocker ticket
- **THEN** the release record refuses it

### Requirement: REL-V1-003 Release calendar

The freeze MUST be 2026-12-15. Stable MUST NOT be promoted before the latest of four dates:
2027-01-05; the end of the 21-day Tasks RC trial; the Corvint RC publish date plus 21 days; and
the end of the 7-day soak. (G10a; owner decision R2 (2026-11-09); review finding 1; owner decision
H1 (2026-11-10))

The Tasks RC trial runs from 2026-12-08 to 2026-12-29 unless it restarts. It restarts as
TCF-V1-009 in change `tasks-conformance` says, on a store format change, and its end is then 21
days after the restart. A code change in the Corvint RC MUST restart the soak and the RC-publish
clock from the new RC publish, and MUST NOT restart the Tasks trial.

A late batch (B8b or B11, REL-V1-009) MUST restart a clock only if it changes a shipped binary,
meaning a binary listed in `tools/release-binary/binaries.json` (REL-V1-008). Such a batch counts
as a code change in the Corvint RC. A late batch that changes no shipped binary MUST NOT restart
any clock. (owner decision H1 (2026-11-10))

#### Scenario: A late RC publish

- **GIVEN** a Tasks trial that ends on 2026-12-29 and an RC published on 2026-12-20
- **WHEN** the stable date is computed
- **THEN** it is 2027-01-10, the RC publish date plus 21 days

#### Scenario: A fix during the soak

- **GIVEN** a code change lands in the RC on soak day 4
- **WHEN** the clocks are read
- **THEN** the soak and the RC-publish clock restart from the new RC publish
- **AND** the Tasks trial clock does not restart

#### Scenario: A restarted Tasks trial sets the date

- **GIVEN** the RC is published on 2026-12-16
- **AND** a store format change restarts the Tasks trial on 2026-12-18
- **WHEN** the stable date is computed
- **THEN** it is 2027-01-08, the end of the restarted trial

#### Scenario: A B11 batch that changes only `.github` files

- **GIVEN** a B11 batch whose diff changes only files under `.github`
- **WHEN** it is merged and the clocks are read
- **THEN** no clock restarts and the computed stable date is unchanged

### Requirement: REL-V1-004 Demotion verdicts at the freeze

The owner's demotion notes MUST rest on the fault-injection and measurement verdicts recorded on one
named freeze-candidate SHA, recorded before the freeze. The RC run MUST repeat those verdicts on the
RC SHA, and any verdict that regresses from the freeze-candidate verdict MUST be a blocking miss. No
demotion may be added after the freeze. (G10; review finding 2; plan checklist O20)

This holds for each of the seven blocking runners. A regression found on the RC MUST block the
RC until it is fixed, and MUST NOT be recorded as a demotion of that runner. (owner decision H2
(2026-11-10))

A verdict MAY be carried forward from an earlier candidate to the RC SHA only under the
REL-V1-014 rules. A carried-forward verdict MUST name the commit it was measured on. (decided
2026-11-09 under R42, D16)

#### Scenario: A regression after the freeze

- **GIVEN** a runner that was selective on the freeze-candidate SHA
- **WHEN** the RC run shows a miss for that runner
- **THEN** the RC is not approved and the record names the runner and the miss
- **AND** the runner is not recorded as DEMOTED

#### Scenario: A demotion proposed for a regressed runner

- **GIVEN** a blocking runner whose verdict regressed on the RC SHA
- **WHEN** a demotion of that runner is proposed in the release record
- **THEN** the record refuses the demotion and the miss stays blocking

### Requirement: REL-V1-005 Tag-started build that stops before publishing

A pushed `v*` tag MUST start a workflow that runs the full suite and builds linux/amd64,
linux/arm64 and darwin/arm64 with `SHA256SUMS`, a manifest, the licence files and a build
attestation, and then stops. The build job MUST hold only `contents: read`, `id-token: write` and
`attestations: write`. Actions MUST be pinned by commit SHA and tags MUST be protected. (G15a;
owner decision R24 (2026-11-09); ci-agents review F11)

GHA-V1-040 to GHA-V1-043 in change `github-action` own the release workflow's requirements.
They stay there with their IDs unchanged; this requirement and REL-V1-007 cite them and do not
replace them. (decided 2026-11-09 under R42, D6)

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

The environment approval is a second step and MUST NOT be recorded as evidence of the owner's
identity. The identity evidence is the signed acceptance tag of REL-V1-013 and ACR-V1-011. The
release record MUST show the environment approval as given under the owner's account with
identity `NOT_VERIFIED`. Whether a token can approve an environment deployment is NOT_VERIFIED.
Giving the two environment approvals is the owner's act. (decided 2026-11-09 under R42, D12 and
D13)

#### Scenario: Approval by anyone else

- **GIVEN** a reviewer other than the owner on the protected environment
- **WHEN** the environment settings are checked before M8
- **THEN** the check reports that only the owner can approve, or M8 does not start

#### Scenario: Environment approval without a signed tag

- **GIVEN** an environment approval given under the owner's account and no signed acceptance tag
- **WHEN** the publish job starts
- **THEN** it refuses, because the approval alone is not identity evidence

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

Two batches are exceptions (decided 2026-11-09 under R42, E5). B8b MUST land by 2026-12-10 or
move to 1.0.x; it removes only files with no reader in product code, CI or a gate target, and it
MUST NOT remove `github-split-check`. B11 follows the G18 deadline of 2026-12-15 (GHA-V1-032).
RMV-V1-001 in change `source-removal` names the same two exceptions.

#### Scenario: B8b after 2026-12-01

- **GIVEN** batch B8b merged on 2026-12-08
- **WHEN** the milestone record is updated
- **THEN** B8b is listed as delivered for 1.0 under the E5 exception

#### Scenario: A late batch

- **GIVEN** a removal batch still open on 2026-12-02
- **WHEN** the milestone record is updated
- **THEN** the batch is listed under 1.0.x and the binaries it removes stay in the 1.0 list

### Requirement: REL-V1-010 Soak and trial counts

The soak count MUST count PRs merged during the soak window in Corvint or Beamfall repositories,
read from the GitHub API. Held-out upstream PRs replayed on the RC MUST be reported as a separate
count. Each Tasks trial repository MUST record at least 50 ticket writes and 5 completions, read
from the store event log. (G10; review finding 14; plan checklist O21, O22)

The soak MUST reach at least 30 PRs, counting the replayed held-out PRs, and at least 20 of the
30 MUST be live Corvint or Beamfall PRs. (decided 2026-11-09 under R42, C9)

#### Scenario: Too little trial activity

- **GIVEN** a trial repository with 40 ticket writes after 21 days
- **WHEN** the trial record is read at stable promotion
- **THEN** the trial is reported below its activity floor and stable is not promoted

#### Scenario: Too few live PRs

- **GIVEN** a soak window with 15 live merged PRs and 15 replayed held-out PRs
- **WHEN** the soak record is read at stable promotion
- **THEN** the soak is reported below its live-PR floor and stable is not promoted

### Requirement: REL-V1-011 Reserved benchmark hosts

Linux qualification timing MUST run on WSL2 on the Windows benchmark box and macOS timing on the
owner's Mac. (review finding 17; plan checklist O23)

Both hosts MUST be reserved only during declared measurement sessions, and MUST run no lane
during a session. Each timing result MUST record the host, the kernel, the load and the
concurrent processes. A result from a session in which a lane ran MUST be marked NOT_VERIFIED
and rerun. Earlier wording, superseded by D8: "Both MUST run no lanes from M5a to RC publish."
(decided 2026-11-09 under R42, D8)

#### Scenario: A lane on the benchmark host

- **GIVEN** a lane recorded on the benchmark box during a declared measurement session
- **WHEN** the qualification record is assembled
- **THEN** timing results from that session are NOT_VERIFIED and rerun

#### Scenario: A lane outside a session

- **GIVEN** a lane that ran on the benchmark box between two declared measurement sessions
- **WHEN** the qualification record is assembled
- **THEN** no timing result is affected

### Requirement: REL-V1-012 Release gates

The release gates MUST be the affected plan, exhaustive `go test` and `go vet` in both modules, and
one scripted qualification per product: the PROD-INT-004 "install alone" scenario plus that
product's primary verb on a pinned repository. The CEM, OCM and dogfood steps MUST NOT be release
gates. (G8; PROD-INT-017; plan checklist O29)

The qualification repository MUST be untouched and pinned. There MUST be one qualification for
each of the three products. The Tasks qualification MUST create a new store and then create,
claim, submit, gate and complete a ticket and pass the receipt audit. (decided 2026-11-09 under
R42, D9)

Each per-product qualification MUST install from the candidate's built archive after verifying it
against `SHA256SUMS`, and for documentation MUST run `corvint-docs`' primary verb and list its
MCP tools on the pinned repository with no other Corvint binary on PATH. (final round, M7)

The nightly `release-gates.yml` run MUST contain exactly the REL-V1-012 gates, and a candidate
tag MUST NOT be pushed unless that run passed on the tagged commit's main ancestor within the
previous 24 hours. (final round, M5)

`greptile-gate` is a merge check on main and is not a release gate. It stays a required status
on main through stable 1.0, with its waiver. A waiver used more than three times between M5a and
stable MUST be recorded in the release record as a known limit. The live ruleset is
NOT_VERIFIED. (decided 2026-11-09 under R42, D14)

#### Scenario: A CEM gate in the release object

- **GIVEN** the rewritten `v1-0` release object
- **WHEN** its required gates are listed
- **THEN** none of them runs a `corvint cem` or dogfood command

#### Scenario: Tasks qualification on a new store

- **GIVEN** the candidate's Tasks archive, verified against `SHA256SUMS`, and an untouched
  pinned repository
- **WHEN** the Tasks qualification runs
- **THEN** it creates a store and creates, claims, submits, gates and completes one ticket
- **AND** the receipt audit passes

#### Scenario: Documentation qualification alone on PATH

- **GIVEN** only `corvint-docs` from the verified archive on PATH
- **WHEN** the documentation qualification runs on the pinned repository
- **THEN** the primary verb succeeds and the MCP tool list is printed

#### Scenario: No nightly pass in the last 24 hours

- **GIVEN** a commit whose main ancestor has no passing `release-gates.yml` run in the previous
  24 hours
- **WHEN** a candidate tag is proposed on it
- **THEN** the tag is not pushed and the record names the missing run

#### Scenario: A greptile waiver during the release

- **GIVEN** a `greptile-gate` waiver on a main merge between M5a and stable
- **WHEN** the release gates are evaluated
- **THEN** the waiver does not fail a release gate and is counted for the known-limit rule

### Requirement: REL-V1-013 Signed release record before publish

The RC publish and the stable promotion MUST each be preceded by the owner's signature in the
acceptance record (change `acceptance-record`), listing each requirement's evidence or demotion, the
NOT_RUN and NOT_OBSERVED items and the deferred P2 and P3 tickets. (G10; PROD-INT-012)

The signature MUST be a tag signed by the owner with the key of ACR-V1-011, placed on the commit
that holds the `docs/releases/<version>` record. That record MUST name the candidate tag's commit
SHA and the `SHA256SUMS` digest. The publish job MUST verify the tag against the allowed-signers
file as it stood at the previously accepted tag, or for the first release at the commit named in
the acceptance decision file. It MUST NOT use the allowed-signers file in the released tree, and
it MUST refuse on any mismatch. Signing the tags is the owner's act. If the owner declines a
key, the fallback is an owner-merged decision file plus the protected-environment approval, and
the record MUST state that the owner's identity is by convention and NOT_VERIFIED. (decided
2026-11-09 under R42, D12 and D13)

#### Scenario: No signature

- **GIVEN** an approved-looking environment run with no RC signature in the acceptance record
- **WHEN** the publish job starts
- **THEN** it refuses and names the missing signature

#### Scenario: Allowed-signers changed in the released tree

- **GIVEN** a candidate whose tree adds a second key to the allowed-signers file
- **AND** an acceptance tag signed with that second key
- **WHEN** the publish job verifies the tag
- **THEN** it uses the file as at the previously accepted tag and refuses

### Requirement: REL-V1-014 Rebuild evidence rules

When a candidate is rebuilt to fix a blocker, evidence MUST be treated in three classes.
(decided 2026-11-09 under R42, D16)

Never redone: the held-out draw. One held-out repository for each family (Go, JS/TS, Python,
Ruby) is drawn from that family's sealed, hashed pool (PRR-V1-004, PRR-V1-011). Its seed is the
SHA of the first pushed candidate tag, and the pools, the rule hash and the pass rules stay
frozen. The draw MUST NOT be redrawn, on a rebuild or for any other reason. A held-out repository
in which the blocker was found MUST be rerun, labelled `refit` and reported separately; it MUST
NOT be replaced by a second draw. (owner decision H3 (2026-11-10))

Always redone on the rebuilt candidate: the tag workflow (full suite, build, checksums and
attestation); the exhaustive Linux and macOS test and vet of both modules; each product's
install-alone qualification from the rebuilt archive; and the scripted agent package steps.

Redone only when the diff touches the class's declared input paths: per-runner fault injection
(R19), the narrowing measurement, the timing measurements, the model-driven agent scenarios and
the action shadow match count. Otherwise the result MAY be carried forward with a record naming
the measured commit and the diff.

The input path set of each class MUST be written and frozen before M5a. A changed path that
belongs to no class MUST cause every class to be redone. The reuse check MUST be a plain path-set
comparison and MUST NOT use Corvint's own test selection.

Before RC publish a rebuild does not affect the soak and trial clocks; after publication
REL-V1-003 applies unchanged. Every carried-forward result MUST be listed in the signed release
record. Every class MUST be rerun without condition on the final candidate before signing when
there were more than two rebuilds or when any carried result is older than 14 days.

#### Scenario: A rebuild outside a class's paths

- **GIVEN** a rebuild whose diff touches no declared input path of the timing measurements
- **WHEN** the qualification record is assembled
- **THEN** the timing results are carried forward with the measured commit and the diff named
- **AND** the tag workflow and the install-alone qualifications are redone

#### Scenario: A changed path in no class

- **GIVEN** a rebuild whose diff changes a path that no class declares
- **WHEN** the reuse check runs
- **THEN** every class is redone

#### Scenario: A third rebuild

- **GIVEN** a final candidate that is the third rebuild
- **WHEN** the release record is prepared for signing
- **THEN** every class has been rerun on that candidate and nothing is carried forward

#### Scenario: Blocker found in a held-out repository

- **GIVEN** a blocker found in one held-out repository
- **WHEN** that repository is rerun on the rebuilt candidate
- **THEN** its result is labelled `refit` and reported apart from the held-out results
- **AND** no repository is drawn in its place

### Requirement: REL-V1-015 macOS release gate and advisory merge check

The exhaustive macOS `go test` and `go vet` MUST be a release gate on the freeze-candidate
commit and on the release-candidate commit. The per-merge macOS check MUST stay advisory until
its flake tickets are closed and 20 consecutive main runs have passed; it then becomes required.
If that has not happened by 2026-12-15, the per-merge check MUST stay advisory for 1.0. (decided
2026-11-09 under R42, D7)

#### Scenario: macOS fails on the release candidate

- **GIVEN** a release-candidate commit whose exhaustive macOS `go test` fails
- **WHEN** the release gates are evaluated
- **THEN** the candidate is not offered for approval

#### Scenario: Flakes still open at the freeze

- **GIVEN** an open macOS flake ticket on 2026-12-15
- **WHEN** the required-status list for 1.0 is fixed
- **THEN** the per-merge macOS check is advisory and the release gate still applies

## MODIFIED Requirements

### Requirement: PRS-V1-004 Release platforms

Release platforms MUST be linux/amd64, linux/arm64 and darwin/arm64 native binaries. Windows is
supported only through WSL; native Windows is stated unsupported in 1.0. (Previously darwin/arm64
and linux/amd64 only; changed by answer 16, G15a and the settled three-platform decision.)

#### Scenario: linux/arm64

- **GIVEN** a release candidate
- **WHEN** its artifacts are listed
- **THEN** a linux/arm64 archive with its checksum and attestation is present
