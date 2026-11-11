<!-- SPDX-License-Identifier: Apache-2.0 -->
# Release Process V1

Owner: Russell Lewis
Date: 2026-11-10
Intent status: proposed
Delivery status: not-started
Authoritative inputs: change folder `docs/specs/openspec/changes/release-process/`;
`docs/specs/PRODUCT-PLAN-1.0.md` (PROD-INT-006, 012, 013, 014, 015 and 017; G8, G9, G10, G10a,
G12, G15a, G18); owner decisions R2, R3 and R24 (2026-11-09) and H1, H2 and H3 (2026-11-10);
answer 16; the 2026-11-09 decisions under R42 (D6, D7, D8, D9, D12, D13, D14, D16, E5, M5, M7);
decision 0433; PRS-V1-004 in `corvint-1.0-product-and-release-v1.md`.

## Agent digest
- Claim: The 1.0 release runs on one calendar, a tag-started build stopping before an owner-approved publish, three platforms and a release object re-scoped at M0.
- Status: proposed / not-started
- Exists: `.github/workflows/release-gates.yml` (nightly, no tag trigger), `tools/release-binary/binaries.json` with five targets, the `v1-0` release object with obsolete criteria, and `corvint-tasks release update`.
- Blocked on: owner acceptance of the change; M0 mapping of open `v1-0` tickets (REL-V1-001).
- Read next: Requirements; Failure modes; Acceptance and rollback.

## User and boundary

The plan states what 1.0 "done" means (G10), the freeze and stable dates (G10a) and the
tag-started release (G15a). This spec makes them testable: the M0 re-scope of the `v1-0` release
object, the M5a qualification candidate, one calendar rule, demotion verdicts fixed at the freeze,
the tag workflow and its owner-approved publish, the three-target artifact list, the removal
deadline, the soak and trial counts, the benchmark hosts, the release gates, the signed release
record, the rebuild evidence classes and the macOS release gate. PRS-V1-004 is amended in the same
change to three platforms.

Non-goals:

- no notarization, install script or Homebrew tap in 1.0 (G15a);
- no native Windows artifact (answer 16);
- no publishing by anyone but the owner: every RC and stable publish needs the owner's explicit
  approval;
- no change to the support window or to the rule against an agent-held signing identity in
  decision 0433;
- no release of corvint-tasks from this repository; Tasks ships from its own repository.

## Requirements

- `REL-V1-001`: Milestone M0 re-scopes the v1-0 release object. Before any removal lane starts,
  every open ticket at milestone `v1-0` MUST either carry a `requirementRefs` entry naming the
  `PROD` requirement it serves or be moved to `post-1-0` with the reason recorded in the ticket. The
  `v1-0` release object's acceptance criteria and required gates MUST be rewritten through
  `corvint-tasks release update` to the plan's 1.0 bar and the PROD-INT-017 gates. (owner decision
  R3 (2026-11-09); review finding 4)
- `REL-V1-002`: Milestone M5a qualification candidate. A qualification candidate tag MUST be pushed
  about 2026-12-08, before the freeze. It MUST be rebuilt only to fix a blocker, and each rebuild
  MUST name the blocker ticket. The commit SHA of the pushed candidate tag MUST seed the held-out
  repository draw. (owner decision R2 (2026-11-09); G12; review finding 3)
  The seed MUST be the commit SHA of the first pushed candidate tag. The draw MUST NOT be redrawn on
  a rebuild; REL-V1-014 says what a rebuild repeats. (decided 2026-11-09 under R42, D16)
  The draw is the seeded draw of PRR-V1-004: one held-out repository for each family (Go, JS/TS,
  Python, Ruby), taken from that family's pool, which PRR-V1-011 seals and hashes before the first
  candidate tag is pushed. No fixed list of held-out repositories applies. (owner decision H3
  (2026-11-10))
- `REL-V1-003`: Release calendar. The freeze MUST be 2026-12-15. Stable MUST NOT be promoted before
  the latest of four dates: 2027-01-05; the end of the 21-day Tasks RC trial; the Corvint RC publish
  date plus 21 days; and the end of the 7-day soak. (G10a; owner decision R2 (2026-11-09); review
  finding 1; owner decision H1 (2026-11-10))
  The Tasks RC trial runs from 2026-12-08 to 2026-12-29 unless it restarts. It restarts as
  TCF-V1-009 in change `tasks-conformance` says, on a store format change, and its end is then 21
  days after the restart. A code change in the Corvint RC MUST restart the soak and the RC-publish
  clock from the new RC publish, and MUST NOT restart the Tasks trial.
  A late batch (B8b or B11, REL-V1-009) MUST restart a clock only if it changes a shipped binary,
  meaning a binary listed in `tools/release-binary/binaries.json` (REL-V1-008). Such a batch counts
  as a code change in the Corvint RC. A late batch that changes no shipped binary MUST NOT restart
  any clock. (owner decision H1 (2026-11-10))
- `REL-V1-004`: Demotion verdicts at the freeze. The owner's demotion notes MUST rest on the
  fault-injection and measurement verdicts recorded on one named freeze-candidate SHA, recorded
  before the freeze. The RC run MUST repeat those verdicts on the RC SHA, and any verdict that
  regresses from the freeze-candidate verdict MUST be a blocking miss. No demotion may be added
  after the freeze. (G10; review finding 2; plan checklist O20)
  This holds for each of the seven blocking runners. A regression found on the RC MUST block the RC
  until it is fixed, and MUST NOT be recorded as a demotion of that runner. (owner decision H2
  (2026-11-10))
  A verdict MAY be carried forward from an earlier candidate to the RC SHA only under the REL-V1-014
  rules. A carried-forward verdict MUST name the commit it was measured on. (decided 2026-11-09
  under R42, D16)
- `REL-V1-005`: Tag-started build that stops before publishing. A pushed `v*` tag MUST start a
  workflow that runs the full suite and builds linux/amd64, linux/arm64 and darwin/arm64 with
  `SHA256SUMS`, a manifest, the licence files and a build attestation, and then stops. The build job
  MUST hold only `contents: read`, `id-token: write` and `attestations: write`. Actions MUST be
  pinned by commit SHA and tags MUST be protected. (G15a; owner decision R24 (2026-11-09); ci-agents
  review F11)
  GHA-V1-040 to GHA-V1-043 in change `github-action` own the release workflow's requirements. They
  stay there with their IDs unchanged; this requirement and REL-V1-007 cite them and do not replace
  them. (decided 2026-11-09 under R42, D6)
- `REL-V1-006`: Attestation supersedes decision 0433 for attestation only. The release MUST carry a
  build attestation, superseding decision 0433 item 1 for the build attestation and npm provenance
  only. The support window of decision 0433 item 2 and its rule that no agent creates or uses a
  signing identity MUST stay in force. (settled without the owner; G15a; G20; plan checklist O26)
- `REL-V1-007`: Owner approval publishes, and M8 is approval and publish. Only the publish job MAY
  hold `contents: write`. It MUST run in a protected environment bound to protected `v*` tags and
  only after the owner's explicit approval. Milestone M8 MUST be "RC approval and publish": the
  owner's first signature (acceptance record) and the environment approval. The soak clock MUST
  start at RC publish. Stable promotion MUST need the owner's second signature and a second explicit
  approval. (G10; G15a; review finding 3)
  The environment approval is a second step and MUST NOT be recorded as evidence of the owner's
  identity. The identity evidence is the signed acceptance tag of REL-V1-013 and ACR-V1-011. The
  release record MUST show the environment approval as given under the owner's account with identity
  `NOT_VERIFIED`. Whether a token can approve an environment deployment is NOT_VERIFIED. Giving the
  two environment approvals is the owner's act. (decided 2026-11-09 under R42, D12 and D13)
- `REL-V1-008`: Release artifacts per product for three targets.
  `tools/release-binary/binaries.json` MUST list exactly the targets linux/amd64, linux/arm64 and
  darwin/arm64, and MUST group binaries by product (testing, docs, shared update tool). A binary
  retired by a removal batch MUST leave the list in that batch. corvint-tasks MUST ship the same
  three targets from its own repository, and `corvint-update` MUST cover both components. (review
  finding 18; owner decision R14 (2026-11-09); answer 16)
- `REL-V1-009`: Removals finish by 2026-12-01. Every M3 removal batch MUST be merged by 2026-12-01.
  A batch not merged by then MUST move to 1.0.x and its removals MUST NOT land before the RC.
  (settled without the owner; plan checklist O24)
  Two batches are exceptions (decided 2026-11-09 under R42, E5). B8b MUST land by 2026-12-10 or move
  to 1.0.x; it removes only files with no reader in product code, CI or a gate target, and it MUST
  NOT remove `github-split-check`. B11 follows the G18 deadline of 2026-12-15 (GHA-V1-032).
  RMV-V1-001 in change `source-removal` names the same two exceptions.
- `REL-V1-010`: Soak and trial counts. The soak count MUST count PRs merged during the soak window
  in Corvint or Beamfall repositories, read from the GitHub API. Held-out upstream PRs replayed on
  the RC MUST be reported as a separate count. Each Tasks trial repository MUST record at least 50
  ticket writes and 5 completions, read from the store event log. (G10; review finding 14; plan
  checklist O21, O22)
  The soak MUST reach at least 30 PRs, counting the replayed held-out PRs, and at least 20 of the 30
  MUST be live Corvint or Beamfall PRs. (decided 2026-11-09 under R42, C9)
- `REL-V1-011`: Reserved benchmark hosts. Linux qualification timing MUST run on WSL2 on the Windows
  benchmark box and macOS timing on the owner's Mac. (review finding 17; plan checklist O23)
  Both hosts MUST be reserved only during declared measurement sessions, and MUST run no lane during
  a session. Each timing result MUST record the host, the kernel, the load and the concurrent
  processes. A result from a session in which a lane ran MUST be marked NOT_VERIFIED and rerun.
  Earlier wording, superseded by D8: "Both MUST run no lanes from M5a to RC publish." (decided
  2026-11-09 under R42, D8)
- `REL-V1-012`: Release gates. The release gates MUST be the affected plan, exhaustive `go test` and
  `go vet` in both modules, and one scripted qualification per product: the PROD-INT-004 "install
  alone" scenario plus that product's primary verb on a pinned repository. The CEM, OCM and dogfood
  steps MUST NOT be release gates. (G8; PROD-INT-017; plan checklist O29)
  The qualification repository MUST be untouched and pinned. There MUST be one qualification for
  each of the three products. The Tasks qualification MUST create a new store and then create,
  claim, submit, gate and complete a ticket and pass the receipt audit. (decided 2026-11-09 under
  R42, D9)
  Each per-product qualification MUST install from the candidate's built archive after verifying it
  against `SHA256SUMS`, and for documentation MUST run `corvint-docs`' primary verb and list its MCP
  tools on the pinned repository with no other Corvint binary on PATH. (final round, M7)
  The nightly `release-gates.yml` run MUST contain exactly the REL-V1-012 gates, and a candidate tag
  MUST NOT be pushed unless that run passed on the tagged commit's main ancestor within the previous
  24 hours. (final round, M5)
  `greptile-gate` is a merge check on main and is not a release gate. It stays a required status on
  main through stable 1.0, with its waiver. A waiver used more than three times between M5a and
  stable MUST be recorded in the release record as a known limit. The live ruleset is NOT_VERIFIED.
  (decided 2026-11-09 under R42, D14)
- `REL-V1-013`: Signed release record before publish. The RC publish and the stable promotion MUST
  each be preceded by the owner's signature in the acceptance record (change `acceptance-record`),
  listing each requirement's evidence or demotion, the NOT_RUN and NOT_OBSERVED items and the
  deferred P2 and P3 tickets. (G10; PROD-INT-012)
  The signature MUST be a tag signed by the owner with the key of ACR-V1-011, placed on the commit
  that holds the `docs/releases/<version>` record. That record MUST name the candidate tag's commit
  SHA and the `SHA256SUMS` digest. The publish job MUST verify the tag against the allowed-signers
  file as it stood at the previously accepted tag, or for the first release at the commit named in
  the acceptance decision file. It MUST NOT use the allowed-signers file in the released tree, and
  it MUST refuse on any mismatch. Signing the tags is the owner's act. If the owner declines a key,
  the fallback is an owner-merged decision file plus the protected-environment approval, and the
  record MUST state that the owner's identity is by convention and NOT_VERIFIED. (decided 2026-11-09
  under R42, D12 and D13)
- `REL-V1-014`: Rebuild evidence rules. When a candidate is rebuilt to fix a blocker, evidence MUST
  be treated in three classes. (decided 2026-11-09 under R42, D16)
  Never redone: the held-out draw. One held-out repository for each family (Go, JS/TS, Python, Ruby)
  is drawn from that family's sealed, hashed pool (PRR-V1-004, PRR-V1-011). Its seed is the SHA of
  the first pushed candidate tag, and the pools, the rule hash and the pass rules stay frozen. The
  draw MUST NOT be redrawn, on a rebuild or for any other reason. A held-out repository in which the
  blocker was found MUST be rerun, labelled `refit` and reported separately; it MUST NOT be replaced
  by a second draw. (owner decision H3 (2026-11-10))
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
- `REL-V1-015`: macOS release gate and advisory merge check. The exhaustive macOS `go test` and `go
  vet` MUST be a release gate on the freeze-candidate commit and on the release-candidate commit.
  The per-merge macOS check MUST stay advisory until its flake tickets are closed and 20 consecutive
  main runs have passed; it then becomes required. If that has not happened by 2026-12-15, the
  per-merge check MUST stay advisory for 1.0. (decided 2026-11-09 under R42, D7)

## Failure modes

| Failure | Behavior |
|---|---|
| A tag is pushed without owner approval | Artifacts and the attestation are retained as workflow artifacts; nothing is published (REL-V1-005, REL-V1-007). |
| The full suite fails in the tag workflow | The build stops before the attestation step; the candidate is not offered for approval (REL-V1-005). |
| A `v1-0` ticket has neither a requirement reference nor a move record after M0 | The M0 audit lists it and M0 is not complete (REL-V1-001). |
| A verdict regresses between the freeze-candidate SHA and the RC SHA | Blocking miss for that runner; never recorded as a demotion (REL-V1-004). |
| A late batch changes a shipped binary after RC publish | The soak and the RC-publish clock restart; a batch that changes no shipped binary restarts nothing (REL-V1-003). |
| A held-out repository yields a blocker | It is rerun and labelled `refit`; the draw is not repeated (REL-V1-014). |
| A removal batch is not merged on 2026-12-01 | It moves to 1.0.x; its binaries stay in the 1.0 list (REL-V1-009). |
| The benchmark host runs a lane during a declared measurement session | That session's timing results are NOT_VERIFIED and rerun (REL-V1-011). |
| A rebuild changes a path that no evidence class declares | Every class is redone (REL-V1-014). |
| The acceptance tag does not verify against the earlier allowed-signers file | The publish job refuses (REL-V1-013). |
| The attestation step cannot obtain an OIDC token | The build fails closed; no unattested artifact is offered (REL-V1-005, REL-V1-006). |
| The GitHub count of soak PRs is unavailable | The count is NOT_OBSERVED and stable is not promoted (REL-V1-010). |

## Acceptance scenarios

| Requirement | Acceptance scenarios (change delta) |
|---|---|
| REL-V1-001 | An unmapped ticket; A moved ticket |
| REL-V1-002 | A blocker rebuild keeps the draw; The first candidate tag seeds the draw; A rebuild without a blocker |
| REL-V1-003 | A late RC publish; A fix during the soak; A restarted Tasks trial sets the date; A B11 batch that changes only `.github` files |
| REL-V1-004 | A regression after the freeze; A demotion proposed for a regressed runner |
| REL-V1-005 | A tag without approval; A failed suite |
| REL-V1-006 | An agent signing identity |
| REL-V1-007 | Approval by anyone else; Environment approval without a signed tag |
| REL-V1-008 | A windows target |
| REL-V1-009 | B8b after 2026-12-01; A late batch |
| REL-V1-010 | Too little trial activity; Too few live PRs |
| REL-V1-011 | A lane on the benchmark host; A lane outside a session |
| REL-V1-012 | A CEM gate in the release object; Tasks qualification on a new store; Documentation qualification alone on PATH; No nightly pass in the last 24 hours; A greptile waiver during the release |
| REL-V1-013 | No signature; Allowed-signers changed in the released tree |
| REL-V1-014 | A rebuild outside a class's paths; A changed path in no class; A third rebuild; Blocker found in a held-out repository |
| REL-V1-015 | macOS fails on the release candidate; Flakes still open at the freeze |

## Acceptance and rollback

Acceptance evidence: the M0 audit output and `corvint-tasks release show` of the rewritten release
object; the M5a candidate tag, its workflow run, `SHA256SUMS`, manifest and attestation for three
targets; the freeze-candidate and RC verdict records side by side; a test run of the tag workflow
showing the publish job waiting for approval and holding the only write permission;
`binaries.json` with three targets and product groups; the soak and trial count records.

Rollback: M0 ticket moves are reversed ticket by ticket, and the prior release object revision is
restored by another `release update`; the tag workflow is deleted in a `.github`-only PR and tags
are never deleted; `binaries.json` is reverted; the PRS-V1-004 amendment is reverted with the
workflow.
