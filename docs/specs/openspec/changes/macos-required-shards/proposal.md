# Change proposal: macOS CI checks become required, macOS flakes block 1.0, native Windows leaves the bar

Change name: `macos-required-shards`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-INT-015 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/macos-required-shards-v1.md`, a new spec with prefix `MRS-V1`
Requirement IDs this change adds: MRS-V1-001 to MRS-V1-007
Requirement IDs this change modifies: none (PRS-V1-004 is modified by the change `release-process`)
Owner answers this change implements: answer 16, owner decisions R36 (2026-11-09) and H11
(2026-11-10)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/macos-required-shards/spec.md`.

## Why

Answer 16 makes Linux and macOS the supported platforms, with Windows through WSL only, and lists
two consequences: macOS CI shards become required checks, and hosted-macOS flake tickets become 1.0
blockers. Owner decision H11 (2026-11-10) settles the conflict with D7: the per-merge check is
advisory until the stated condition holds, and macOS is required at release. Today macOS results
never block a merge, so a darwin-only regression can reach `main` and the release. Owner decision
R36 (2026-11-09) also moves `make windows-vet` for removal lanes from every push to once per
removal batch.

## What exists today

Verified against integ 9ec3a29d0 on 2026-11-09:

- `.github/workflows/go-macos.yml` runs on `pull_request` only (lines 8-9); its header comment
  (lines 4-7) says it is advisory and never delays CI. `merge_group` appears only inside
  expressions (lines 39, 46-47, 140, 155), not as a trigger.
- Job `go-macos-shard` (line 173) has `continue-on-error: true`, shards 0 to 3, `max-parallel: 2`,
  `macos-15`, a 55-minute timeout and `LINECOV_PLAN_P95_BUDGET: "500ms"` (MCJ-V0-019). It is skipped
  when the cadence decision is DEFER (CIN-V0-4754) or the plan is verified docs-only. Shard selection
  uses `.github/macos/darwin_select.py` and `.github/macos/shard.py`.
- Job `go-macos` (line 336) is a stable aggregate name, also `continue-on-error`.
- `.github/workflows/release-gates.yml` runs the full gate on `macos-15` only nightly (cron, lines
  17-18) or by manual dispatch.
- AGENTS.md requires `make windows-vet` before each push that touches Go files (AFP-V0-054).

## What Changes

- The release requires an exhaustive macOS `go test` and `go vet` run in both modules on the
  freeze-candidate SHA and on the RC SHA.
- The per-merge `go-macos` aggregate stays advisory until the hosted-macOS flake tickets are closed
  and 20 consecutive `main` runs pass; it then becomes a required status check on `main`. If that
  has not happened by 2026-12-15 it stays advisory per merge for 1.0, and macOS is required at
  release only (D7, REL-V1-015; owner decision H11 (2026-11-10)). The workflow gains a `merge_group`
  trigger, loses `continue-on-error` on the shard and aggregate jobs, and is never skipped by the
  cadence DEFER decision; only a verified docs-only plan may skip, and the aggregate then reports
  success with the skip reason.
- Timing budgets on hosted macOS stay advisory.
- Every macOS flaky test is filed as a `v1-0` P1 ticket. There is no separate rule that holds the RC
  while such a ticket is open; the macOS release gate and the general open-P1 rule cover it
  (owner decision H11 (2026-11-10)).
- The documentation states native Windows is unsupported and points to WSL (text owned by the change
  `user-docs`); native Windows vet and archive work leaves the 1.0 bar.
- Removal lanes run `make windows-vet` once per removal batch, before that batch's push (R36).

## Capabilities

### New Capabilities

- `macos-required-shards`: macOS release gate, required check, flake policy and Windows scope.

### Modified Capabilities

- `go-macos` CI workflow (MCJ-V0 and CIN-V0 rows that make the job advisory or skippable are
  superseded for the aggregate check; their IDs are listed in task 1.2).

## Impact

- `.github/workflows/go-macos.yml`, `.github/workflows/release-gates.yml` and the repository
  ruleset for `main`. Changing the ruleset is an owner action, taken only if the stage 2 condition
  is met by 2026-12-15.
- Merge latency rises by the macOS run time on PRs that select darwin tests.
- AGENTS.md "Verify" section: the windows-vet sentence gains the removal-batch rule.
- The `.github/` edits go in their own PR, as the repository requires for `.github` changes.

## Non-goals

- No macOS timing budget becomes blocking on hosted runners (MCJ-V0-019; timing qualification runs
  on the reserved hosts of the change `release-process`).
- No native Windows support, archive or CI.
- No darwin/amd64 target.
- No change to which tests darwin selection picks.
