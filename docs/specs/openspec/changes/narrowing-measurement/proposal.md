# Change proposal: measure and report the per-runner narrowing bar, freeze it, demote what misses it

Change name: `narrowing-measurement`
Ticket: none filed yet (task 1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-TEST-002, PROD-TEST-008, PROD-TEST-011, PROD-INT-007 and
PROD-INT-008 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/narrowing-measurement-v1.md`, a new spec with its own prefix
`NMS-V1`
Requirement IDs this change adds: NMS-V1-001 to NMS-V1-011
Owner answers this change implements: G1, G3, G4, G10, G10a, G16 (`owner-answers-2026-11-09.md`),
and owner decisions R2, R6, R8, R9, R13, R16, R19 and R20 (2026-11-09)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/narrowing-measurement/spec.md`).

## Why

The owner set a numeric narrowing bar for 1.0 (G4): Go, Vitest, Jest and pytest must each narrow
at least 60 percent of replayed PRs, and the median selected test time must be at most 50 percent
of the full suite. Playwright, RSpec and Minitest publish their numbers with no bar. Zero misses are
evaluated first (G3); a runner that fails the miss proof cannot pass on narrowing. The thresholds
are checked for feasibility on a 200-PR Go replay and frozen before the release candidate (RC) is
measured; runner verdicts are judged on the milestone M5a candidate SHA (owner decision R2
(2026-11-09)) and confirmed on the RC SHA. At the freeze on 2026-12-15 (G10a), a runner without a
pass ships running its full suite with a printed reason (G10), and a global or per-runner kill
switch `CORVINT_SELECTION=full` must exist and be tested in the RC.

What exists today, verified in `integ` at 9ec3a29d0:

- No code or document names `CORVINT_SELECTION` (repository-wide grep, zero hits). The kill switch
  is new.
- CI narrowing is switched by other repository variables, `CORVINT_PR_NARROWING` and
  `CORVINT_MQ_NARROWING`, read in `.github/workflows/ci.yml`, `ci-forward-bar.yml` and
  `go-macos.yml`. These govern Corvint's own CI only, not the product.
- Replay tools exist for Go only: `tools/tiers-replay/main.go:1-9` replays the last N first-parent
  merges for the AFP-V0-480 shadow tiers (AFP-V0-991); `tools/affected-zero-unknown-replay` and
  `tools/run-measure` measure plan states and run resource use.
- `corvint test` records elapsed time per invocation, not per test
  (`internal/sessiontest/run.go:85`, `Invocation.ElapsedMilliseconds`). The G4 cost ratio needs
  per-test durations; change `per-test-durations` adds them for every runner (owner decision R19
  (2026-11-09)).
- No report combines narrowing rate, cost ratio and the miss verdict per runner.

## What Changes

- A measurement command (proposed design: `tools/narrowing-measure`, a repository tool, not a
  product verb) replays a frozen PR list per runner, records the plan state of each PR and the
  per-test durations of a full baseline run at the same commit, and writes one report per runner.
  Narrowed means tier `line` or `file` (owner decision R16 (2026-11-09)).
- The replay source is the merged PRs of the pinned repository in the 12 months before the manifest
  SHA, at least 100; at most 5 percent of tests may be excluded (owner decision R19 (2026-11-09)).
- A frozen bar file holds the thresholds, the PR lists and their hashes. It is written after the
  200-PR Go feasibility replay and before the RC is measured.
- The release manifest lists, per runner, PASS, PUBLISH-ONLY or DEMOTED, with the reason printed.
- `CORVINT_SELECTION=full` or `full:<runner>[,<runner>]` forces a full run in `corvint affected`,
  `corvint test`, `corvint watch`, the GitHub action, agent hooks and MCP (owner decision R13
  (2026-11-09)). This change owns the grammar.
- The release report states the agreement figure (owner decision R20 (2026-11-09)).

## Capabilities

### New Capabilities

- `narrowing-measurement`: per-runner narrowing rate, cost ratio, frozen bar, demotion record,
  agreement figure and kill switch.

### Modified Capabilities

- `affected-plan`, `session-test-run`: honour the kill switch and print its reason.
- `ci-action` (owned by another proposal): reads the kill-switch repository variable (G16).

## Non-goals

- No change to how plans are computed. This change measures them.
- No bar for Playwright, RSpec or Minitest in 1.0 (G4).
- No Cypress: dropped from 1.0 and listed as post-1.0 (owner decision R6 (2026-11-09)).
- Over-selection on held-out repositories is reported only, never blocking (G12).
- No narrowing of the measured runs on Unraid hosts; runs are on Linux and macOS (G1).
- No Python line collector; pytest is measured at tier `file` as a recorded release exception
  (owner decision R9 (2026-11-09)).

## Impact

- New repository tool and report schema; reads the per-test duration member added by change
  `per-test-durations`.
- Release manifest gains a per-runner status table and the agreement figure.
- Depends on `fault-injection-harness` (the miss verdict), `proof-repositories` (the pinned
  repositories and their PR lists) and `per-test-durations`.
