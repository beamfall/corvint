# Change proposal: corvint-tasks lease, read and status corrections before 1.0

Change name: `tasks-hardening`
Ticket: none filed yet (task 1 files it in the corvint-tasks repository's `.taskman/` store and links
it from a `v1-0` ticket in this repository)
Product requirement served: PROD-TASK-002, PROD-TASK-003, PROD-TASK-005 and PROD-TASK-007 in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/tasks-hardening-v1.md` in the `beamfall/corvint-tasks`
repository, a new spec with prefix `TKF-V1`
Requirement IDs this change adds: TKF-V1-001 to TKF-V1-012
Requirement IDs this change modifies: none
Owner answers this change implements: owner decision R17 (2026-11-09; schema v1 and the
supported-command list, which these output changes must precede)
Related changes, referred to by name: `tasks-conformance` (schema v1, supported-command list,
conformance record), `tasks-integration` (Corvint-side coupling), `ticket-taxonomy` (ticket
categories, duplicate refusal, required fields and search; written separately)
Intent status: proposed (written 2026-11-10; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/tasks-hardening/spec.md`. The build lands in the `beamfall/corvint-tasks` repository; the
folder is recorded here because Corvint 1.0 waits on Tasks 1.0 (PROD-TASK-010).

## Why

A review of the Tasks product (`tricks/tasks.md`, verified in `tricks/tasks.verdicts.md`) found that
the read commands give wrong answers when a lease has expired, that the normal completion path loses
its lease between steps, and that several status counters do not mean what their names say. On the
live Corvint store 80% of completions are manual (2,039 manual against 521 verified), while the
health read reports `OK`. PROD-TASK-003 requires completion through submit, gate and complete;
PROD-TASK-005 requires correct answers to what is open, next and blocked. Both fail in practice today.

## What exists today

Verified 2026-11-10 against corvint-tasks PR #4 head 18c75e3 (`/private/tmp/claude-501/cvt-pr4`,
branch `claude/import-from-corvint`; Tasks `main` at ae47138 does not hold this code) and against the
installed `corvint-tasks 1.0.0-rc.3.dev+build.144` reading `/Users/russelllewis/projects/corvint`.
Paths are under `internal/tasks/`.

- `transaction/plan.go:492-507`: `choose` tests every reservation entry for collision and counts every
  entry toward `maxActiveAttempts` without checking expiry. `transaction/lease.go:986-996`:
  `entryOracle.LiveAttempt` reports any reservation entry as a live attempt. The claim path reaps
  expired leases first (`transaction/lease_claim.go:469-480`), so the preview and the claim disagree.
  Live: three leases expired about nine hours earlier still produced 968 `RESOURCE_COLLISION` and 23
  `LIMIT_EXCEEDED` deferrals in `plan preview --summary`, and `ticket blockers V1-5212` returned
  `ATTEMPT_LIVE` with `nextAction: wait-attempt`.
- `transaction/lease_gate.go:199-221`: `planSubmit` does not touch the lease; `planGateRun` (249)
  neither. Only `planRenew` (`transaction/lease.go:849-862`) extends it. `planHeartbeat`
  (`lease.go:999-1014`) records `lastHeartbeatAt` only. Once the lease expires, every command of
  that generation is fenced and the remedy is to claim, submit and run every gate again
  (`lease.go:787-801`).
- Verified completion is claim, submit, one `gate run` per required gate, then complete, each
  carrying `--attempt`, `--generation` and `--request-id`.
- `cli/receipt_health.go:92-98`: `completionHealth` is `OK` unless verified is zero. Live: 288 manual
  and 36 verified in the seven days ending 2026-11-09, state `OK`.
- `help` lists 170 implemented verbs; none answers "what next" without writing (`claim --next`
  writes). `roadmap` with no flags lists all 5,280 tickets; its first page is 48 ARCHIVED, 24
  COMPLETED and 28 OPEN, starting with V1-0001 ARCHIVED.
- `cli/cli.go:2325-2340` adds every closed ticket to `blocked` (live 4,283 with 1,029 OPEN).
  `cli/cli.go:2355` emits the UNKNOWN-eligibility count as `intentChecksPassed` (997).
  `lifecycle/lifecycle.go:116-118` derives lifecycle from the latest progress note with no age check
  (live: 92 IN_PROGRESS and 75 IN_REVIEW with three attempts, all expired).
- `ticket show V1-9999999` returns `codes ["MALFORMED"]`, the same code as a call with a missing
  argument.
- `ticket search --label agent-memory --status OPEN` returns 149,308 bytes for 100 items, with the
  same WHOLE_REPOSITORY advisory repeated 92 times.

## What Changes

- Plan and blocker reads treat an expired lease as reapable, not live, and say so (TKF-V1-001,
  TKF-V1-002).
- Submit, gate run and heartbeat extend the lease; one keep-alive clock for leased attempts
  (TKF-V1-003, TKF-V1-004).
- A `finish` verb runs submit, the missing required gates and complete for the caller's live attempt
  (TKF-V1-005).
- `receipt audit` reports the manual share, a `MOSTLY_MANUAL` state and the manual-only days
  (TKF-V1-006).
- A `next` read returns the top claimable tickets with the claim command; `roadmap` defaults to OPEN
  and HELD (TKF-V1-007, TKF-V1-008).
- `queue status` counters match their names; old progress notes show as stale (TKF-V1-009,
  TKF-V1-010).
- A `NOT_FOUND` result code (TKF-V1-011).
- A repeated advisory is emitted once; long inline ID lists become a count and a command
  (TKF-V1-012).

## Capabilities

### New Capabilities

- `tasks-hardening` (corvint-tasks repository): lease liveness in reads, lease extension, `finish`,
  `next`, completion health and status counter corrections.

### Modified Capabilities

- `tasks-conformance`: schema v1 and the supported-command list must include `finish`, `next`, the
  renamed counters and `NOT_FOUND`. No requirement text there changes.

## Impact

- corvint-tasks repository: `transaction/plan.go`, `transaction/lease.go`,
  `transaction/lease_gate.go`, `cli/receipt_health.go`, `cli/cli.go`, `lifecycle/lifecycle.go`, new
  `finish` and `next` commands, tests.
- Output shape changes for `queue status`, `roadmap`, `plan preview`, `ticket blockers` and
  `receipt audit`. They land before schema v1 is frozen by `tasks-conformance`.
- This repository: `AGENTS.md` and memory guidance that say "renew AND heartbeat" can drop to one
  verb after delivery.

## Overlap with `ticket-taxonomy`

`ticket-taxonomy` owns search, listing defaults, required fields and duplicate refusal. This change
does not cover: text-search relevance and word-boundary matching (review finding 11), the `ticket
list` and `ticket search` default status, limit and compact rows (findings 4 and 10, listing part),
or required scope fields (finding 8). TKF-V1-012 changes only how repeated advisories and inline ID
lists are emitted, for every read verb; if `ticket-taxonomy` sets a compact search row, TKF-V1-012
still applies to it.

## Non-goals

- No revival of an expired attempt; the fencing rule CAL-V0-010 is unchanged (review finding 2c);
  an expired attempt always needs a new claim in 1.0 (R42 (panel pick) C6).
- No optimistic or scope-less claim (finding 8) before 1.0; TTX-V1-011 makes scope required and
  `next` prints the scope (R42 (panel pick) C5).
- No change to `receipt audit` speed or read latency (findings 5 and 6); read timings are recorded
  per TCF-V1-008 and the budget is set at the qualification candidate (R42 (panel pick) C8).
- No split of `dispatch` into another binary (finding 14).
- No change to supervised (detached) attempts, which carry no lease.
