# Design: affected-plan-kinds

Status: proposed (2026-11-09). Requirement text lives in `specs/affected-plan-kinds/spec.md`.

## Decisions

1. One declaration file, `.corvint/flows.json`, versioned; the old file is read for one release
   (G6, G5).
2. Kind is a reporting label with a reported source; `unit` is the default (G13).
3. Only accepted, non-stale link records plus agreeing static reachability may exclude an E2E test
   (G2, G11).
4. Missing flow proof gives scope `UNKNOWN` and every E2E test, both at once (testing S3; APK-V0-009
   replaces APK-V0-005).
5. Reach is defined once on the static import graph (testing M11; APK-V0-010).
6. A flow selects every accepted linked test of any kind or runner (owner decision R12
   (2026-11-09); review T-B3). Links are proposed with a confidence score and accepted only by a
   human in 1.0; link records key tests by the canonical test ID, RNC-V1-012 (owner decision R11
   (2026-11-09)). The `test_id` member of APK-V0-004 is that canonical ID, not a Playwright-only
   file-plus-title key; a file-level selection matches every canonical ID in that file.
7. Tier vocabulary is `line | file | flow | full` plus `exact` (owner decision R16 (2026-11-09)).
   A flow-selected test carries tier `flow`; `flow` selections do not count as narrowed for the
   no-missed-test claim.
8. A flows edit (flow intent or link record) selects the tests linked to the changed flows and does
   not force a full run; only Corvint configuration files under `.corvint/` force a full run (owner
   decision R31 (2026-11-09)).
9. The inventory is the one committed inventory of change `test-inventory`, written only by
   `corvint inventory refresh`; absent or stale inventory runs all tests with reason
   `inventory-absent` or `inventory-stale` (owner decision R10 (2026-11-09)).

## Failure modes

- Declaration missing: today's plan, unchanged.
- Declaration malformed, over a bound, with an unknown profile version, or naming a directory
  outside the repository: typed failure, exit 2, no partial plan.
- Both `.corvint/flows.json` and `.corvint/e2e-flows.json` present: `E2E_FLOWS_INVALID`; every E2E
  test is selected and the plan names the reason.
- Link record stale (test content hash or routes changed), dangling (test or variation gone), or
  not accepted: the test is selected and counted `unassigned` with the reason.
- Link record names a canonical test ID that the inventory does not hold: the record is dangling
  and is handled as above.
- A proposed (not accepted) link carries a confidence score: it never excludes a test, whatever the
  score.
- Static reachability disagrees with an accepted link: the test is selected with reason
  `reachability-disagrees`.
- Reach cannot be resolved for a changed path (no owning unit in the graph, no adapter for the
  target's language, or a link target that no longer exists): every E2E test of the affected runner
  is selected with reason `reach-unknown` (APK-V0-010).
- Flow evidence or the inventory missing, stale, unreadable or over a bound: scope `UNKNOWN` and
  every E2E test selected with a typed reason (APK-V0-009); inventory absence and staleness use
  `inventory-absent` and `inventory-stale`.
- The full E2E list itself cannot be produced (inventory absent and no runner full command): the
  plan is `UNKNOWN` with reason `e2e-list-unavailable`, and `corvint test` runs the runner's
  declared full command (RNC-V1-013 in change `runner-contract`).
- The import graph exceeds its bound: handled by APK-V0-009 (`bound-exceeded`).
- Kind signals conflict: `kind` `UNKNOWN`, `kindSource` `conflict`; selection unchanged.

## Acceptance evidence

- A fixture repository with Go, Vitest and Playwright tests and `.corvint/flows.json`. One Go test
  per requirement, named for V1-5251, covering each scenario.
- Fixtures for each failure mode, asserted with its typed code, including both-files and an unknown
  profile version.
- APK-V0-009: one fixture test per failed-evidence reason asserting both the `UNKNOWN` scope and the
  full E2E selection.
- APK-V0-010: fixture tests for both scenarios and for an RSpec system spec with unresolved reach.
- APK-V0-008: a fixture flow linking a Go unit test, a Vitest test and a Playwright test; a change
  reaching the flow selects all three, each with tier `flow`.
- A plan for a fixture with no declaration compared byte for byte with a recorded plan.
- The import ratchet with the `internal/appflows -> internal/liveverify/affected*` baseline entries
  removed for the touched files.
- `corvint affected --base FULL_SHA` and focused tests; no repository-wide gate unless asked.
  NOT_RUN at the time of writing: every test of the current `oneplan` code.

## Rollback

Remove `.corvint/flows.json` from a repository to return it to the earlier plan. To withdraw the
change, revert its commits; the spec file and IDs are retired, not reused. The AFU-V1-1651 and 1655
text reverts with them.

- APK-V0-009: revert the commit; APK-V0-005's wording returns, and the rework lane must still pick
  "select all" until this is re-landed.
- APK-V0-010: revert the commit; reach then has no written definition, and the rework lane keeps the
  existing `reach` walk unchanged.
- APK-V0-008: revert the commit; flows then select E2E tests only, and non-E2E linked tests fall
  back to static selection.

## Open questions for the owner

- Should the deprecated `.corvint/e2e-flows.json` read be removed in 1.1 or in the first release
  after 1.0 that changes the flow profile?
