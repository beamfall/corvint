# Change proposal: a fault-injection harness that proves zero misses per runner

Change name: `fault-injection-harness`
Ticket: none filed yet (task 1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-TEST-002 (no miss), PROD-TEST-011 and PROD-INT-008 in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/fault-injection-harness-v1.md`, a new spec with its own prefix
`FIH-V1`, extending the method of `docs/specs/affected-edit-oracle-v0.md` (AEO-V0)
Requirement IDs this change adds: FIH-V1-001 to FIH-V1-015
Owner answers this change implements: G1, G2, G3, G12, G15b, G23, G26
(`owner-answers-2026-11-09.md`), and owner decisions R7, R9, R16, R19 and R21 (2026-11-09)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/fault-injection-harness/spec.md`).

## Why

The 1.0 claim is "no test that could fail is left out". The owner chose the proof (G3): plain replay
reports only selection size and over-selection; the miss proof is a separate fault-injection set per
blocking runner, frozen and hashed before execution. A fault is put into a line the PR changed, and
the selected run must execute the test that kills it and see that test fail. The release wording is
fixed: "0 misses in N pre-registered fault-injected cases per runner on pinned repos; 95 percent
upper bound on the per-change miss rate 3/N; not a guarantee." Owner decision R19 (2026-11-09) sets
N = 100.

What exists today, verified in `integ` at 9ec3a29d0:

- The nearest method is AEO-V0-004 (`docs/specs/affected-edit-oracle-v0.md:34`, accepted under the
  standing auto-accept, delivery experimental): an opt-in mutation replaces a pinned literal site,
  proves the command passes on the base, observes assertion failures on the mutant, and requires
  every failing test unit to be in that mutant's plan; a miss is `SAFETY_FAIL`. Build failures,
  timeouts and survivors are failed measurements. Code: `tools/affected-edit-oracle/main.go`
  (`compare` :124, `testObservations` :475, `runMutation` :538). It is Go-only, single-run, and its
  corpus is Corvint plus four Beamfall repositories (AEO-V0-006).
- `internal/liveverify/mutate/mutate.go:1-6` is the "test-kills-mutant falsifier" of the retired
  `falsifiable-packet-v0` spec. Its only importers are `internal/testrunner/golang/qualification.go:10`
  (`mutate.SelectionOperators()` at :52), `internal/liveverify/pymutate` and
  `internal/liveverify/rbmutate`. Nothing imports `internal/testrunner/golang`, so the Go operators
  are orphaned.
- `internal/liveverify/pymutate/pymutate.go:1-8` (FPK-V0-019, pytest) and
  `internal/liveverify/rbmutate/rbmutate.go:1-9` (FPK-V0-068..081, RSpec only) have no importers
  outside themselves. Neither covers Minitest.

Why the removed packages are not needed is recorded in `design.md` (G26).

## What Changes

- A repository tool (proposed: `tools/fault-harness`) generalising `tools/affected-edit-oracle` to
  seven runners, with a pre-registration file, a three-run baseline with quarantine, three-run kill
  confirmation, and a per-runner verdict.
- One runner adapter per runner on the single compiled-in runner contract (G23): run a named set of
  tests at a commit and return per-test outcomes.
- Case accounting with four counts; N = narrowed PRs with a confirmed kill, at least 100 after an
  over-sample of about 140 (owner decision R19 (2026-11-09)).
- Exactly 30 reverts per runner, an operator list per language, and non-code faults counted as a
  separate N.
- Held-out sets of 100 faults for fastapi, solidus and payload (owner decisions R19 and R21
  (2026-11-09)).
- A verdict file per runner, consumed by `narrowing-measurement` (NMS-V1-004, NMS-V1-007).

## Capabilities

### New Capabilities

- `fault-injection-harness`: pre-registered fault sets, kill confirmation, miss verdicts.

### Modified Capabilities

- `affected-edit-oracle`: its Go path becomes the first adapter of the harness (G1: the replay
  harness is built once, Go first). AEO-V0 requirements stay as written.

## Non-goals

- No product verb. This is release evidence, not a user feature.
- No test-strength scoring and no revival of `prove`, pymutate or rbmutate (G26).
- No removal of browser E2E tests by line evidence; E2E tests leave a plan only through accepted
  flow links plus static reachability (G2). The harness checks that rule, it does not loosen it.
- No runs on Unraid hosts (G1).
- No Cypress adapter: Cypress is dropped from 1.0 and listed as post-1.0 (owner decision R6
  (2026-11-09)).

## Impact

- New tool and verdict schema. Uses manifests from `proof-repositories` (PRR-V1). Non-code faults
  use the map of `non-code-map` (NCM-V1). Receipts carry per-test durations from change
  `per-test-durations`.
- Compute cost is large; the owner estimated about a week (G3), hence FIH-V1-010. The over-sample
  of about 140 PRs per runner raises it further.
