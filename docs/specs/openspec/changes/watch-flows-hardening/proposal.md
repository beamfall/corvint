# Change proposal: watch survives planning errors, and flow-linked E2E selection leaves no linked test out

Change name: `watch-flows-hardening`
Ticket: none filed yet (to be filed in `.taskman/` before dispatch).
Product requirement served: PROD-TEST-002, PROD-TEST-003, PROD-TEST-004, PROD-TEST-005 and
PROD-TEST-007 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: a new spec `docs/specs/watch-flows-hardening-v1.md` (prefix `WFF-V1`). It
does not exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: WFF-V1-001 to WFF-V1-008. `WFF-` has no entry in
`docs/specs/REQUIREMENTS.tsv` and no use in any change folder (checked 2026-11-09 on
`claude/layers-integ` at `9ec3a29d0`).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/watch-flows-hardening/spec.md`).

## Why

A review of `corvint watch` and the flows.json E2E join found 15 problems; all hold in the code.
Most are already covered by `watch-seven-runners`, `affected-plan-kinds`, `flow-link-proposals`
and `non-code-map`, and the verdict file lists amendments for those. Six gaps have no owner. Four
of them let a linked E2E test drop out of a plan that still says BOUNDED, or stop watch without an
answer.

Today's state, from the code at `9ec3a29d0`:

- A planning error on one save ends watch. `internal/linecov/watch/loop.go:128-142` returns any
  diff, plan or parse error from the loop; only an event-source stop (`loop.go:111-125`) writes an
  `error` event to stdout. `cmd/corvint/watch.go:157-162` then exits 2 with `watch-failed` on the
  diagnostic stream. A half-typed `_test.go` gives `linecov: parse` (`internal/linecov/git.go:176`)
  and the user's watch is gone.
- Watch keeps no state between saves, so it has no invalidation rule. `watch-seven-runners`
  WCH-V1-007 adds warm state but does not say when that state is dropped.
- A flow intent may link a test from a step or an outcome (`internal/appflows/intent.go:344`). The
  join selects linked tests only where `from` equals a variation
  (`internal/appflows/flowplan.go:543-551`), yet it still marks the test file as linked
  (`flowplan.go:437-447`), so the unlinked-file rule does not run it either (`flowplan.go:328`).
  A hit on a member that no variation lists selects nothing (`flowplan.go:522-530`). `flows
  impact` has the same rule (`internal/appflows/export.go:96-104`). The result is BOUNDED
  (`flowplan.go:347`).
- An owned changed path that reaches no flow link target (for example a server handler that a
  browser test reaches over HTTP) leaves every linked test out without a word.
- The deprecated `.corvint/e2e-flows.json` reader omits a consumer when a reviewed link of any flow
  explains the changed path (`internal/appflows/e2eflows.go:156-160`, `307-320`), even though the
  graph shows the path reaches that consumer's commands (`e2eflows.go:285-302`).
- A snapshot plan joins no E2E member (`cmd/corvint/affected.go:1575-1581` runs the join only when
  the tree is the repository; `cmd/corvint/repository_guidance.go:708` passes a scratch tree), and
  the `e2e` field is `omitempty` (`internal/liveverify/affected/select.go:239`). The plan does not
  say the E2E part is missing.

## What Changes

- Watch answers a planning error with a widened plan and keeps running (WFF-V1-001).
- Every item of warm watch state names what drops it, and doubt drops it (WFF-V1-002).
- Step-level and outcome-level test links select through the variations that list the member; a
  hit with no variation selects every test link of the flow; existing step and outcome test links
  are reported and run `unassigned` until moved to a variation (WFF-V1-003 to 005).
- The plan names owned changed paths that reach no flow and then runs every E2E test under the
  reason `change-unanchored` (WFF-V1-006; R42 (panel pick) A1).
- The deprecated reader stops narrowing on static reach (WFF-V1-007).
- A snapshot plan in a repository with flows says its E2E part is UNKNOWN (WFF-V1-008).

## Capabilities

New: `watch-flows-hardening` (`docs/specs/watch-flows-hardening-v1.md`).
Modified: none by text. This change adds rules beside `watch-v1.md` (WCH-V1), the affected plan
kinds spec (APK-V0) and the flow link records (FLP-V1). Where a rule here and one there disagree,
the stricter one (the one that runs more tests) wins.

## Non-goals

- Speed. Save-to-answer time is WCH-V1-004; flow-side cost is an amendment there. Finding 12 (one
  reverse walk per plan): ticketed, not in this change (decided 2026-11-09 under R42, M10).
- Cancelling runs, per-test events, path-carrying event sources, answer fields such as
  `changed[]`: amendments to `watch-seven-runners`.
- Unowned paths and ownerless link targets: `affected-plan-kinds` APK-V0-010 and `non-code-map`.
- One linked test hiding unlinked tests in the same file: `flow-link-proposals` FLP-V1-007.
- Deriving flow anchors from `internal/appmap`, skipping tests that passed, removing the e2e-safe
  selector: after 1.0, each needing an owner decision.

## Impact

- Code: `internal/linecov/watch/loop.go`, `cmd/corvint/watch.go`,
  `internal/appflows/flowplan.go`, `internal/appflows/export.go`, `internal/appflows/e2eflows.go`,
  `cmd/corvint/affected.go`, `cmd/corvint/repository_guidance.go`.
- Plans get more E2E tests on repositories with step-level links and in the deprecated reader. No
  plan gets fewer.
- New plan fields: `e2e.unanchored`, `e2e.link_not_variation`, and the reason values `plan-error`
  and `E2E_SNAPSHOT_UNJOINED`. Consumers that ignore unknown fields are unaffected.
