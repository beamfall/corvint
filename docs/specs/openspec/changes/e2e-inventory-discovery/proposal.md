# Change proposal: E2E selection from the committed inventory

Change name: `e2e-inventory-discovery`
Ticket: V1-5252 (open, P1, milestone `v1-0`; the ticket stays in `.taskman/`)
Product requirement served: PROD-TEST-004 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/application-flow-understanding-v1.md` (prefix `AFU-V1`), inside
its single `## Requirements` section
Requirement IDs this change adds: AFU-V1-146 to AFU-V1-149 (fresh IDs, owner decision R11
(2026-11-09); checked free in integ `9ec3a29d0` `docs/specs/REQUIREMENTS.tsv` and in every folder
under `docs/specs/openspec/changes/` on 2026-11-09).
Requirement IDs this change retires: AFU-V1-140 to AFU-V1-145. Integ `9ec3a29d0` already carries
definition lines for them (`docs/specs/application-flow-understanding-v1.md`, lines 766-799). Owner
decision R11 (2026-11-09) forbids reusing them with a changed meaning, so they are retired through
the REMOVED delta and the new meanings take fresh IDs. AFU-V1-150 to AFU-V1-154 belong to change
`flows-unlinked-tests-report` and are not touched here.
Intent status: proposed (rewritten 2026-11-09; only the owner can accept it)
Delivery status: experimental code exists on `claude/layers-integ` (lane `oneplan`, head
`1f23ed3ed`, merged compile-only, tests and review not run). It predates G11 and implements the old
AFU-V1-145, which this rewrite reverses.
Written before dispatch: the first version was; this rewrite precedes the rework lane.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/e2e-inventory-discovery/spec.md`).

## Why

E2E selection needs a list of every E2E test. A hand-written provider file goes stale the day a test
is added, and a test missing from the list can never be selected. Owner answer 4 settles the shape:
a separate explicit step runs the lister and writes a pinned inventory; `corvint affected` only
reads it and widens to all E2E tests when it is stale. Product invariant 4 is kept.

Owner decision R10 (2026-11-09) generalises that step: there is ONE committed inventory under
`.corvint/` for all seven runners, written by an explicit `corvint inventory refresh`; a missing or
stale inventory runs all tests with reason `inventory-absent` or `inventory-stale`. The inventory
itself, its path, profile and per-runner listers belong to change `test-inventory`. This change
keeps only the E2E (Playwright) side: how E2E selection reads that inventory.

Verified today on `claude/layers-integ` at `9ec3a29d0`:

- `corvint affected discovery` turns a Playwright listing into a discovery receipt on stdout
  (`cmd/corvint/affected.go:384-386`, `:1062-1073`). Whether it reads a saved listing or launches
  the lister is NOT_VERIFIED from the code read for this proposal; the existing test feeds it a
  listing (`cmd/corvint/affected_playwright_discovery_test.go:40`).
- When `--provider FILE` is given beside the derived inventory, the provider's `test_selection` is
  the selection to run and derived files it omits are only recorded (`ReconcileProvider`,
  `internal/appflows/flowplan.go:564-590`). A test the provider omits can therefore be dropped.

Contradictions in the first version that this rewrite resolves:

1. Old AFU-V1-145 versus 141 and 142. 145 let a provider file that omits a discovered test decide
   the selection; 141 and 142 said an unlisted or unexplained test is always selected. The new rule
   (AFU-V1-149) lets a provider file add tests and never remove a discovered one (settled S5).
2. Old AFU-V1-142 said "a typed refusal or a full E2E selection". Owner answer 4 says stale means
   widen. The new rule (AFU-V1-148) widens for stale or absent inventories and refuses only
   malformed input.
3. Old AFU-V1-141 called the state `unlinked`; G11 names it `unassigned` and requires a count in
   every plan (AFU-V1-147).
4. Any change to any test file made the whole inventory stale (review T-M2). Under AFU-V1-148 only
   a config, lockfile or inventory-profile change makes the whole inventory stale; tests in added or
   changed test files run as `unassigned`.
5. The write step was `corvint affected discovery --write`, a read command with a write flag
   (review T-M4). The write verb is `corvint inventory refresh` (owner decision R10 (2026-11-09)).

## What Changes

- E2E selection reads the single committed inventory of change `test-inventory`, written only by
  `corvint inventory refresh`, with the Playwright lister marked `executing`. `corvint affected`
  and `corvint watch` only read it and execute no project code (owner answer 4, owner decision R10
  (2026-11-09)). `corvint watch` refreshes nothing; the GitHub Action runs `corvint inventory
  refresh` before planning (review T-M4).
- Each inventory entry is keyed by the canonical test ID (RNC-V1-012, change `canonical-test-id`),
  the same key link records use (G11, owner decision R11 (2026-11-09)).
- A discovered test that no accepted link ties to a flow is always selected and counted
  `unassigned` (G11).
- An absent inventory runs the Playwright runner's declared full command with reason
  `inventory-absent` (review T-M3). A config or lockfile change makes the inventory stale and widens
  to every E2E test with `inventory-stale`. Tests in added or changed test files run as
  `unassigned` (review T-M2).
- `--provider FILE` may add tests; it never removes a discovered test, and a disagreement is
  reported (settled S5).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `application-flow-understanding`: the E2E selection reads the committed inventory.
- `typescript-javascript-affected-adapter`: its discovery receipt becomes the Playwright lister's
  output for `corvint inventory refresh`; its own requirements do not change.

## Non-goals

- Running E2E tests.
- Launching the lister from `corvint affected` or `corvint watch` (invariant 4, owner answer 4).
- The inventory file format, its path, the refresh command and the listers of the other six runners
  (change `test-inventory`).
- Proposing or accepting links (`corvint-docs`, G11; change `flow-link-proposals`).
- The unlinked-tests report, its `no-e2e-proof` code and AFU-V1-150 to AFU-V1-154 (change
  `flows-unlinked-tests-report`).

## Impact

- Code: `cmd/corvint/affected.go`, `internal/appflows/selection.go`, `internal/appflows/flowplan.go`
  (`ReconcileProvider`), the discovery receipt reader.
- Declaration: the Playwright config path is a member of `.corvint/flows.json` (change
  `affected-plan-kinds`, G6). The inventory path is set by change `test-inventory`, not by
  `.corvint/flows.json`.
- Runners: Playwright in this change. pytest with pytest-playwright and RSpec system tests are kind
  `e2e` (G13); their inventory entries come from their own listers in change `test-inventory`, and
  unresolved reach selects every E2E test of that runner (APK-V0-010).
- Refresh command: `corvint inventory refresh` in `corvint` (tests layer), so a testing-only install
  can refresh it (G5, owner decision R10 (2026-11-09)). This answers the earlier open question about
  the verb's home. `corvint affected discovery` remains a read command that prints a receipt.
