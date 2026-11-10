# Design: `release-process`

## Decisions

- Calendar (owner decision R2, 2026-11-09; G10a; review finding 1). M5a tags a qualification
  candidate about 2026-12-08. The freeze is 2026-12-15. Stable is promoted on the latest of three
  dates: 2027-01-05, RC publish plus 21 days (Tasks trial), and the end of the 7-day soak. Any code
  change in the RC restarts both clocks (PROD-INT-013).
- Tag first (review finding 3). The candidate tag is pushed first. Its commit SHA is the seed of
  the held-out repository draw (G12), so the draw cannot be known before the candidate exists. The
  tag workflow runs the full suite and builds, then stops. M8 is "RC approval and publish": the
  owner signs and approves the protected publish job. The soak clock starts at publish, not at tag.
- Rebuild only for blockers (owner decision R2). A rebuilt candidate cites the blocker ticket that
  caused it. Proposed: the held-out draw is not redrawn on a blocker rebuild (listed as open in the
  hand-back; it is not an owner decision).
- Demotion basis (review finding 2; plan checklist O20). Before the freeze the owner records
  demotions from the fault-injection (FIH) and measurement (NMS) verdicts recorded on one named
  freeze-candidate SHA. The RC run repeats them on the RC SHA; any verdict that regresses from the
  freeze-candidate verdict is a blocking miss. "Selective" means zero-miss and not demoted (settled
  without the owner).
- Release object (owner decision R3; review finding 4). Each open `v1-0` ticket gets one of: a
  `requirementRefs` entry naming the `PROD` requirement it serves, or a move to `post-1-0` with a
  reason recorded in the ticket. The release object's acceptance criteria are replaced by the plan's
  1.0 bar, and its required gates by the PROD-INT-017 gates. Which existing gate IDs survive
  (full-gate, interop-gate, companion-release, focused-docs, release-checklist) is decided in task
  2.2 against PROD-INT-017 and listed in the hand-back as open.
- Tag workflow permissions (G15a; owner decision R24; ci-agents review F11). The build job has
  `contents: read`, `id-token: write` and `attestations: write`, and no `contents: write`. Only the
  publish job has `contents: write`, and it runs in a protected environment bound to protected `v*`
  tags. All actions are pinned by commit SHA. No job checks out pull-request code under
  `pull_request_target`. GitHub release immutability is NOT_VERIFIED and is not claimed.
- Attestation (settled without the owner; plan checklist O26). Decision 0433 item 1 is superseded
  for the build attestation and npm provenance only. Its support window (item 2) and its rule that
  no agent may create or use a signing identity stay. The superseded text is quoted in the
  acceptance record.
- Platforms (settled without the owner; G15a; answer 16). linux/amd64, linux/arm64, darwin/arm64.
  PRS-V1-004 is modified to match.
- Artifacts (review finding 18). `binaries.json` keeps only the surviving products' binaries for the
  three targets, grouped by product. Binaries retired by the removals (analyzers, provider binaries
  under owner decision R14, 2026-11-09) leave the list in the removal batch that deletes them. Tasks
  ships the same three targets from its own repository; `corvint-update` covers both components.
- Removals (settled without the owner; plan checklist O24). M3 removal batches finish by 2026-12-01;
  a batch not merged by then moves to 1.0.x.
- Soak and trial counts (review finding 14; plan checklist O21, O22). A real PR is one merged during
  the soak window in a Corvint or Beamfall repository, counted from the GitHub API. Held-out upstream
  PRs replayed on the RC are a separate count. The Tasks trial needs at least 50 ticket writes and
  5 completions per trial repository, read from the store event log.
- Benchmark host (review finding 17; plan checklist O23). Linux runs on WSL2 on the Windows
  benchmark box; macOS runs on the owner's Mac. Both are reserved with no lanes from M5a to RC
  publish.
- Release gates (G8; PROD-INT-017; plan checklist O29). The affected plan, exhaustive `go test` and
  `go vet` in both modules, and one scripted qualification per product: the PROD-INT-004 "install
  alone" scenario plus the product's primary verb on a pinned repository.

## Failure modes

- A tag is pushed without owner approval: artifacts and the attestation are built and retained as
  workflow artifacts; nothing is published.
- The full suite fails in the tag workflow: the build stops before the attestation step; the
  candidate is not eligible for approval.
- A ticket in `v1-0` has neither a requirement reference nor a move record after M0: the M0 audit
  (task 2.1) lists it and M0 is not complete.
- A verdict regresses between the freeze-candidate SHA and the RC SHA: blocking miss; the RC is not
  approved until fixed (which restarts the clocks) or the owner records a demotion before the freeze.
  A demotion cannot be added after the freeze.
- A removal batch is not merged on 2026-12-01: it moves to 1.0.x; the binaries it would have
  removed stay in the 1.0 artifact list.
- The benchmark host runs a lane between M5a and RC publish: the qualification timing results from
  that window are marked NOT_VERIFIED and rerun.
- The attestation step cannot obtain an OIDC token: the build fails closed; no unattested artifact
  is offered for approval.
- The GitHub count of soak PRs is unavailable: the count is NOT_OBSERVED and stable is not promoted.

## Acceptance evidence

- The M0 audit output: every formerly open `v1-0` ticket with its requirement reference or move
  record; `corvint-tasks release show` of the rewritten release object.
- The M5a candidate tag, its workflow run, its `SHA256SUMS`, manifest and attestation for three
  targets.
- The freeze-candidate verdict record and the RC verdict record, side by side.
- A test run of the tag workflow on a non-`v*` test tag in a fork or a dry-run mode showing the
  publish job waiting for approval and holding the only write permission.
- `binaries.json` with three targets and product groups; `make release-binaries-check` output.
- The soak and trial count records at stable promotion.

## Rollback

- M0: each ticket move is a store mutation with its prior milestone recorded; moves are reversed
  ticket by ticket. The release object keeps its revision history (`release update` creates a new
  revision); the prior revision's criteria and gates are restored by another `release update`.
- Tag workflow: delete the workflow file in a `.github`-only PR; tags are never deleted (G10 rollback
  order: kill switch, 1.0.1 forward fix, withdraw).
- `binaries.json`: revert the commit; the removed targets return.
- PRS-V1-004: revert the spec edit in the same revert as the workflow.
