# Design: watch-flows-hardening

Status: proposed (2026-11-09). Requirement text lives in
`specs/watch-flows-hardening/spec.md`; the lane copies it into
`docs/specs/watch-flows-hardening-v1.md`.

## Decisions

1. A planning error is an answer, not a stop. On a per-save error watch writes one `plan` event
   with `scope: UNKNOWN`, the reason `plan-error`, the message, and the file and line when the
   error names them. The listed tests are the widest set watch can name: every package of the Go
   module, and for each other runner its full-suite command. Watch exits 2 only when it can no
   longer read events, the index or write output. This keeps PROD-TEST-007 (an answer per save)
   and PROD-TEST-002 (widen, never guess).
2. Warm state is keyed, not trusted. Each cached item records the inputs it was derived from
   (file content hash, HEAD commit, index state, runner config hash, intent tree hash). An item is
   reused only when all its inputs match. A HEAD move, checkout, stash, rebase or any input that
   watch cannot read drops the item and watch derives it again. Mtime alone never proves
   freshness.
3. A step or outcome is selected through its variations. The join builds, per flow, the set of
   variations that list each member. A test link whose `from` is a step or outcome belongs to every
   variation that lists that member. A hit selects a test link when it selects any of those
   variations. `flows impact` uses the same function, so the two answers cannot drift.
4. A hit with no variation widens within the flow. If a hit's member is in no variation, every test
   link of that flow is selected, with reason `member-without-variation`. This stops a missing
   variation list from silently dropping tests.
5. Old link shapes are reported, not upgraded. `flow-link-proposals` FLP-V1-006 records carry a
   `variation_id`. An existing step or outcome test link is never turned into such a record by a
   tool. It is listed under `e2e.link_not_variation`, and its test is treated as not linked, so the
   unassigned rule (FLP-V1-007) runs it. Decision 3 still selects it on a hit; decision 5 adds the
   always-run rule until a person moves the link to a variation.
6. Unanchored paths are named. A changed path that has an owner, is not a global input, and
   reaches no link target of any flow is listed under `e2e.unanchored` (count, and the first 50
   paths sorted). When the list is not empty the plan also selects every E2E test under the one
   reason `change-unanchored` (decided: R42 (panel pick) A1). APK-V0-004 and FLP-V1-007 state only
   when exclusion is allowed, so widening does not contradict them.
7. The deprecated reader stops narrowing. `.corvint/e2e-flows.json` keeps working for one release
   with its deprecation notice, but a consumer whose commands any changed path statically reaches
   is always selected. A reviewed link no longer explains the path away.
8. A snapshot plan says it is partial. When the tree is not the repository and the repository has a
   flows intent or a legacy flows file, the plan carries
   `e2e = {scope: UNKNOWN, unknown: [E2E_SNAPSHOT_UNJOINED]}` instead of omitting the field. Running
   the join on snapshots is a later choice; saying UNKNOWN is the cheap correct step.

## Failure modes

- A plan error with no file or line: the event carries the message only; the widened plan is still
  written.
- The widest Go set itself fails (no `go.mod`, `go list` fails): watch writes the `error` event on
  stdout and exits 2 with `watch-plan-unavailable`. It never goes silent.
- A variation lists a member that the intent does not define: the intent validator already refuses
  this; the join treats the flow as unbounded with reason `intent-invalid`.
- A cache input is unreadable (permission, racing write): the item is dropped and derived again.
- `e2e.unanchored` is longer than 50: the count is exact and the list is cut; the cut is marked
  `truncated: true`.
- The legacy reader and the flows.json join both apply: the union of their selections runs.

## Acceptance evidence

- Watch fixture test: save a `_test.go` with a syntax error, then a fixed one. Two `plan` events,
  the first `UNKNOWN` with `plan-error` and a file and line, the second normal; the process is still
  running.
- Cache test: run a plan, change HEAD with the working tree unchanged, plan again; the second plan
  re-derives (counted by a test hook) and matches a cold plan.
- Join fixture tests: a step-level test link selected through its variation; a hit on a member in
  no variation selecting every test link of the flow; a step-level link listed under
  `link_not_variation` and run `unassigned`; the same three answers from `flows impact`.
- Unanchored test: change an owned server file that no link targets; the plan lists it.
- Legacy reader test: a reviewed link of flow A, a changed path that statically reaches the
  consumer of flow B; the consumer is selected.
- Snapshot test: `snapshotReviewAffected` on a repo with flows gives `e2e.scope: UNKNOWN` with
  `E2E_SNAPSHOT_UNJOINED`.

NOT_RUN at the time of writing: everything.

## Rollback

Each requirement is a separate commit and can be reverted alone. Reverting WFF-V1-003 to 007
returns to the narrower selection, which is the state this change exists to remove; revert them
only with an owner note. Reverting WFF-V1-001 returns watch to exit-on-error. No stored data
changes; the new plan fields disappear on revert.

## Open questions for the owner

- O1 decided, R42 (panel pick) A1: an owned changed path that reaches no flow widens to every E2E
  test under the one reason `change-unanchored` (WFF-V1-006).
- O2 decided, R42 (panel pick) A2: the deprecated `.corvint/e2e-flows.json` reader narrows nothing
  during its last release (WFF-V1-007).
