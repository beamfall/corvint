# Change proposal: the Corvint GitHub Action and the tag-triggered release

Change name: `github-action`
Ticket: none filed yet. A ticket is filed in `.taskman/` before any lane starts; it holds the queue.
Product requirement served: PROD-TEST-012 ("Continuous integration MUST run the same plan as the
local command and run everything when the plan cannot be trusted") and PROD-INT-006 (verified
release artifacts) in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/github-action-v1.md` (new, prefix `GHA-V1`). The release
requirements GHA-V1-040 to GHA-V1-043 stay in this change for now; change `acceptance-record`
records whether they stay here or move to `docs/specs/public-release-v0.md` (owner decision R1
(2026-11-09): one acceptance record).
Requirement IDs this change adds: GHA-V1-001 to GHA-V1-018, GHA-V1-020 to GHA-V1-026,
GHA-V1-030 to GHA-V1-033, GHA-V1-040 to GHA-V1-043 (32 requirements; GHA-V1-018 added 2026-11-09
for owner decision R22). The prefix had no entry in
`docs/specs/REQUIREMENTS.tsv` on 2026-11-09.
Owner answers this comes from: answer 6, answer 16, G10, G15a, G15b, G16, G17, G18, G25 (item 2),
G27, and the owner instruction after G15 ("we want to make sure we have a world class and full
corvint integrated CI with github actions as well"); owner decisions R1, R2, R4, R7, R13, R14, R22,
R23, R24, R25 and R31 (2026-11-09)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/github-action/spec.md`.

## Why

Corvint's product sentence is "given a diff, name exactly the tests that must run and why". In CI
that is worth most, and today an adopter has no supported way to use it there. The only adopter path
is a script to copy, `script/adopter-ci-advisory.sh`, run as a `continue-on-error` job that never
blocks and never narrows (`docs/AUTOMATION.md:189-244`). Corvint's own CI narrows through bespoke
workflow scripts that no adopter can reuse. The owner chose a lean action that blocks 1.0 (G16), a
new public repository for it (G17), and a shadow run on Corvint's own CI before it replaces the
bespoke scripts (G18).

## What Changes

- A new public repository `beamfall/corvint-action` holds one composite action and the starter
  workflows (G17). Approval to create it is recorded in G17; it is created when this proposal is
  committed.
- The action downloads a checksum-verified `corvint` binary, resolves the base commit for
  `pull_request`, `merge_group` and `push`, asks the binary for a plan and a per-runner matrix with
  shards, runs the selected tests, and runs everything when unsure (G16).
- A repository variable `CORVINT_SELECTION` is the kill switch: `full`, or `full:<runner>[,<runner>]`;
  an unknown value exits 2 (G10, G16, owner decision R13 (2026-11-09)).
- The plan is computed once and stored as one artifact with a digest; shards run from it filtered by
  runner, and the result job checks every shard against it (owner decision R22 (2026-11-09)).
- Narrowing applies on `pull_request` and `push`; pull requests from forks always run everything
  (owner decision R23 (2026-11-09)).
- One always-running result job carries the required check; a job summary states what ran and why
  (G16).
- Preview at 1.0, specified here but not release-blocking (G16): shadow mode with a would-be-miss
  report, a PR comment built from `corvint review` (G25 item 2), a docs drift step, a flow-gaps
  step, a Tasks ticket-gate step, starter workflows per runner, and index cache reuse.
- Corvint's own CI runs the action in shadow next to the bespoke scripts, and replaces them after 30
  matching pull requests with no would-be miss, or after 1.0 if that is not reached by 2026-12-15
  (G18, G27).
- Corvint releases are published by a workflow started by a pushed version tag, with a publish job
  that runs only after owner approval (G15a). The action only consumes releases made this way.

## Capabilities

### New Capabilities

- `github-action`: the action, its trust rules, its outputs and its release
  (`docs/specs/github-action-v1.md`, new).

### Modified Capabilities

- `ci-narrowing` (`docs/specs/ci-narrowing-v0.md`, prefix `CIN-V0`): Corvint's own CI narrowing
  moves onto the action at cut-over; the spec gains a status line naming this change.
- `gate-ledger` (`docs/specs/gate-ledger-v0.md`) and `go-archive-gate` (`docs/specs/go-archive-gate-v0.md`):
  retired at cut-over (G27), with status lines and bodies kept as history.
- `adoption-setup-report` (`docs/specs/adoption-setup-report-v0.md`): ADS-V0-650, the copy-in
  advisory script, is replaced by the action at cut-over.
- `public-release` (`docs/specs/public-release-v0.md`): gains or links the tag-triggered release
  requirements.

## Impact

- New repository `beamfall/corvint-action`: `action.yml`, a small shell entry point that only moves
  data between the binary and GitHub, `starters/<runner>.yml` for the seven blocking runners, its
  own test workflow and release workflow, `LICENSE`.
- `beamfall/corvint` binary: new read commands that emit the plan, the matrix, the job summary and
  the shadow report in GitHub-ready form. Proposed names: `corvint ci plan`, `corvint ci summary`,
  `corvint ci shadow-report`, plus a runner selector for `corvint test` (proposed `--runner NAME`). The
names are proposals; the lane records the final names in the spec. The plan output schema is fixed
by GHA-V1-018.
  The selection logic stays in the binary, as for the agent packages (G20); the action holds none.
- `beamfall/corvint` `.github/`: a release workflow (new), the shadow jobs, then the cut-over. Each
  `.github` change goes in its own pull request, because `github-split-check` (`ASD-V0-706`)
  refuses a range that mixes `.github` with code.
- Removed at cut-over (G27): `tools/gate-ledger/**`, `script/gate-receipt` and its test,
  `script/go-archive-gate` and its three tests, `script/adopter-ci-advisory.sh` and its test, the
  Makefile targets that call them, and the bespoke selection inputs listed under GHA-V1-031.
- `docs/AUTOMATION.md` "Adopter CI recipe" is rewritten around the action.
- `tools/release-binary/binaries.json` lists five platforms today, including `darwin/amd64` and
  `windows/amd64` (`tools/release-binary/binaries.json:5-9`); G15a names three. See "Contradictions".

## Verified at this head and assumed

Read in the integration checkout (`scratchpad/integ`, head `9ec3a29d0`) on 2026-11-09:

- No `action.yml` and no `workflow_call` workflow exist; the adopter path is the copy-in script
  (`script/adopter-ci-advisory.sh:1-15`, `docs/AUTOMATION.md:189-244`, `continue-on-error: true`
  at `docs/AUTOMATION.md:213`).
- `corvint test` already takes `--shard I/N` and `--shard-plan PATH` (`cmd/corvint/test.go:33-34`,
  usage at `cmd/corvint/test.go:270`).
- Corvint's own CI narrowing is gated by repository variables `CORVINT_PR_NARROWING` and
  `CORVINT_MQ_NARROWING` (`.github/workflows/ci.yml:528-535`, `.github/workflows/ci-forward-bar.yml:15`).
  Selection inputs are bespoke: `.github/forward-rows/*.sh|jq`, `tools/ci-reuse-plan`,
  `tools/docs-ci-plan`.
- No product code writes `GITHUB_STEP_SUMMARY`, annotations, a check run or a PR comment (a search
  of `cmd/` and `internal/` for `GITHUB_STEP_SUMMARY` finds no non-test hit).
- `CORVINT_SELECTION` does not exist anywhere in `cmd/`, `internal/` or `.github/` today.
- The G27 scripts exist: `tools/gate-ledger/main.go`, `script/gate-receipt`, `script/go-archive-gate`,
  `script/adopter-ci-advisory.sh`. Callers: `Makefile:135-155` (gate, ledger/STEP),
  `Makefile:212-231` (go-archive-gate), `Makefile:783-785` (gate-receipt-clear),
  `.github/workflows/ci.yml:2897` (`make go-archive-gate`).
- No workflow publishes a release. `.github/workflows/release-gates.yml` is owner-dispatched or
  scheduled, has `permissions: contents: read` and states that it publishes nothing
  (`.github/workflows/release-gates.yml:9,16-36`).

Assumed, NOT_VERIFIED:

- That a composite action cannot read the `vars` context directly. The starter workflows therefore
  pass `CORVINT_SELECTION: ${{ vars.CORVINT_SELECTION }}` as an environment variable. The lane checks
  the GitHub documentation before relying on either form.
- That a build attestation needs `attestations: write` in addition to `id-token: write` on the job
  that makes it. Owner decision R24 (2026-11-09) allows `id-token: write` for attestation only;
  GHA-V1-040 records the rest as NOT_VERIFIED.
- Whether `vars` reach workflows run for fork pull requests. Under R23 forks run everything, so a
  missing kill switch there cannot narrow.
- The docs drift command name in `corvint-docs` (no drift command is wired into any workflow today).

## Non-goals

- GitLab, Bitbucket or any other CI; Windows runners; a hosted dashboard; JavaScript or Docker
  action forms (G16, out).
- Any account, token, secret, OIDC exchange or upload to a Corvint service (G16; invariant 7).
- Writing to a Corvint Tasks store from CI.
- Notarization, an install script or a Homebrew tap in 1.0 (G15a).
- Ranking or reordering tests. The action runs the plan; it does not choose.
- Making macOS hosted-runner shards required in Corvint's own CI. That is a separate consequence of
  the owner answers and is tracked outside this change.
