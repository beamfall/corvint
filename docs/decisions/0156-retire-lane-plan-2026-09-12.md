# Decision 0156 — Retire lane-plan; retain the accepted WQO closure

Date: 2026-09-12. Status: accepted, delegated owner call. Authority: the coordinator's
explicit delegation to choose either integration into WQO or retirement from the measured evidence.

## Decision and discriminating evidence

Choose **B**, the command rollback in the proposed Lane plan V0 spec. Reject its proposed intent;
remove its interception, help, command inventory entry, CLI implementation and tests, and
`internal/laneplan`. Remove the proposed spec and its README/INDEX/requirement rows. No replacement
alias or parser for roadmap prose is introduced. The removed `LPL-V0-001..016` remain historical
IDs, not accepted obligations. No error-code allowlist entries are restored: their laneplan
emitters are also removed. `work observe` and `work propose-wave` retain their existing wires.

The two actual implementations at `dd6c58dcab4d09461d35fcc43f5c3e06dc5438ba` were run against
**the same index built from a committed scratch Go repository**, before removal. Module:
`example.com/project`; files: `go.mod`, `a/a.go` (package a), `b/b.go` (package b, blank-import a),
`c/c.go` (package c, blank-import b), `d/d.go` (package d, blank-import c). Each blank import is
module-qualified. Ticket A declares exactly `{a/a.go}`; D declares exactly `{d/d.go}`.
`contextindex.Build` supplies both `laneplan.Compile` and `workqueue.IndexCollisionSource.Closure`.

| Actual implementation | A closure | D closure | Observed result |
|---|---|---|---|
| laneplan | `a/a.go`, `b/b.go`, `c/c.go`, `d/d.go` | `d/d.go` | `COLLIDES`, shared `d/d.go`; admits A, holds D |
| WQO | `a/a.go` | `d/d.go` | both closures complete; no shared path |

`GOTOOLCHAIN=local go run ./.scratch-laneplan` exited 0. Full fixture bytes, method, source
revision, actual extracted import map and outputs are retained in
the comparison receipt (an internal evidence record, not part of the public tree). The disposable runner
and fixture were removed; this observation is not a permanent test or a qualification claim.

There are **two distinct differences**, not just traversal depth. The real Go index stores
module import strings; WQO's closure tests import keys against tracked repository paths, so those
Go edges contribute no neighbours. This is a separately filed Go resolution gap, not evidence of
independence. Even with path-resolved edges `b/b.go -> a/a.go`, `c/c.go -> b/b.go`,
`d/d.go -> c/c.go`, accepted WQO-V0-042 gives A=`{a/a.go,b/b.go}` and
D=`{c/c.go,d/d.go}`: still no overlap. `TestIndexCollisionSourceDirectNeighboursOnly` locks that
contract boundary with explicit indexed-path edges and derived groups. It does not claim to test
the native Go resolver or silently bless its gap.

## Why B rather than A

Decision 0046 accepts direct import/importer expansion and maximum-wave selection (up to the
specified budget), not a proof of all possible write effects. Laneplan's unaccepted contract
instead requires a reverse-package fixpoint, excluded-Go abstentions even outside the reached
closure, and greedy maximal admission. The measured Go fixture already changes the collision
answer; replacing the closure also changes unknowns, group membership and selected waves.
Laneplan lacks WQO's forward-neighbour rule, non-Go indexed-path behavior and direct path bound.
It cannot be substituted as a drop-in engine while claiming the accepted behavior is preserved.

The prior laneplan spec also records a Corvint lane held by excluded testdata packages. That is
historical evidence at the pinned source above, not a new measurement here. Neither that hold nor
the comparison proves a useful throughput or safety advantage sufficient to change accepted WQO
intent. Retirement removes the competing unsupported claim with the smallest contract change.
Resolving Go module imports for WQO needs its own focused change; changing closure depth or the
wave optimizer needs separate intent and evidence.

## Contract, verification, and rollback

WQO-V0-042 clarifies that complete indexed-path closure is not transitive or semantic independence;
its expansion and wire remain unchanged. WQO-V0-045 requires retirement from invocation, help and
inventory, with ordinary `invalid-arguments` refusals, exit 2, no stdout or stdin reads. SOL-V0-007
removes the deleted command from its self-observation inventory. No WQO schema or identity changes.
`TestLanePlanRetired` failed before removal because the command was advertised; it covers the
retired surface, including both help forms and root options. Existing WQO package and conformance
tests retain the wire, groups, incomplete-closure and maximum-wave checks.

Rollback requires an owner decision reopening the rejected proposal. Restore the removed command,
package, tests and spec from the pinned revision, then reconcile its intent before advertising it.
Do not route WQO through it implicitly. No persisted laneplan data or WQO wire migration is needed.
