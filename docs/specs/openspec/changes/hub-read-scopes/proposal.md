# Change proposal: give the hub test packages a declared read scope

Change name: `hub-read-scopes`
Tickets: V1-5201 (P1, milestone `v1-0`); V1-5234 for the `cmd/corvint` stage
Product requirement served: PROD-TEST-001 and PROD-TEST-002 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/affected-plan-selection-v0.md` (prefix `AFP-V0`)
Requirement IDs this change adds: AFP-V0-107 to AFP-V0-110 (reserved for V1-5201 on 2026-11-10)
Intent status: proposed (written 2026-11-10; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No product code changes with this proposal.
Layout: `proposal.md`, `design.md`, `tasks.md`, `specs/hub-read-scopes/spec.md`.

## Why

A pull request that changes only documents and shell scripts still runs whole Go packages. On the
diff of pull request 178 (`AGENTS.md`, `Makefile`, three specs, two scripts) the planner selected
27 Go packages. Three hubs carry most of the cost: `cmd/corvint` (613 tests),
`internal/contextindex` (716) and `internal/liveverify/affected` (367).

### What the measurement found (at 2913fdc7e)

- Each hub is selected for two reasons at once. Its test files hold path literals that name the
  changed paths (rule (c), AFP-V0-2203), and the unit is an unbounded reader (rule (d)), listed in
  `.corvint/unbounded-readers.json`.
- A proof for literals joined to a temporary root frees none of the hubs. Lane `affprec2` built
  that proof (AFP-V0-9810, not landed) and measured the same docs-only probe: the same 11 hub
  packages stay selected, because their literals are fixture map keys written by helpers. A
  second count on this tree agrees: 0 of 30 (unit, path) pairs are freed.
- A declared read scope does free a hub in the planner. Probe: on `origin/main`, add
  `internal/contextindex` to `.corvint/test-read-scopes.json`, remove it from the unbounded set,
  then edit `AGENTS.md`. The plan no longer selects `internal/contextindex`. The scope is sound
  only when full CI confines the package to it (AFP-V0-161), so the probe proves the planner
  side only.
- The recorded reasons name some tests per hub that read the whole repository. The list is not
  complete; `internal/liveverify/affected` has more (see `tasks.md`, task 2):

| Hub | Tests that read the whole repository |
|---|---|
| `internal/contextindex` | `TestGoImportsDifferentialAgainstScanner` |
| `internal/liveverify/affected` | `TestIncrementalSelectionMeetsTheLiveBudget`, `TestSelectionOnTheLiveDirtyWorktree` |
| `cmd/corvint` | tests in `golist_test.go`, `cem_test.go`, `affected_audit_test.go`, `host_adapter_javascript_test.go` |

  The older cause, a read of the filesystem root, is recorded as resolved (DTL-V0-008).

## What Changes

1. Move the whole-repository tests of a hub into a small sibling test package. That package stays
   an unbounded reader. It is selected on every change, and it is small.
2. Declare a read scope for the hub, sized from the reads observed under the Landlock wrapper,
   and remove the hub from the unbounded set.
3. Keep a hub exactly as it is when its confined run does not pass. Claim no saving for it.
4. Record before and after selection for a fixed docs-only diff.

Stage 1 covers `internal/contextindex` and `internal/liveverify/affected`. Stage 2 covers
`cmd/corvint` and belongs with the package split of V1-5234; its feasibility is `NOT_OBSERVED`.

## Capabilities

### Modified Capabilities

- `affected-plan-selection`: no planner rule changes. The change adds requirements on which
  packages may hold whole-repository tests and on the evidence a hub scope needs.

## Non-goals

- No new literal proof and no change to rule (c) or rule (d). AFP-V0-9810 lands with its lane.
- No change to the Landlock wrapper or to how full CI enforces a scope.
- No weakening of fail-closed selection: an unproven hub stays selected on every change.
- No promise for `cmd/corvint-docs`, `conformance/release-artifact-v0` or the other unbounded
  units. Each needs its own measurement.

## Impact

- Files: `.corvint/test-read-scopes.json`, `.corvint/unbounded-readers.json`, the moved test
  files, `docs/specs/affected-plan-selection-v0.md`, one build-log entry.
- Expected effect of stage 1 on a docs-only change: 1083 fewer selected Go tests, less the moved
  tests. This is an inference from test counts, not a measured duration.
- Failure modes: a moved test needs unexported names (export a narrow test hook or keep the test
  and the hub unscoped); a scoped hub reads outside its scope (the confined run fails, so the
  scope is not declared).
- Rollback: restore the unbounded entry and delete the scope entry. The moved tests can stay.
