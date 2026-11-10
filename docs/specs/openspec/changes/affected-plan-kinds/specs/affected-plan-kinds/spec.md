# Spec delta: affected-plan-kinds

The lane copies each statement below into `affected-plan-kinds-v0.md` as a numbered definition
line that begins `(proposed 2026-11-09; V1-5251; delivery experimental)`.

## ADDED Requirements

### Requirement: APK-V0-001 One plan for every test kind from one declaration

Source: G6, G5. When the repository has `.corvint/flows.json` with a known profile version,
`corvint affected --base FULL_SHA` MUST return unit, integration and E2E selections in one receipt
without any extra option. `.corvint/e2e-flows.json` MUST be read for one release with a deprecation
notice, and the presence of both files MUST fail closed with `E2E_FLOWS_INVALID`.

#### Scenario: A change reaches a flow

- **GIVEN** a repository with `.corvint/flows.json`, unit tests and Playwright tests
- **WHEN** a commit changes a source file reached by a flow with accepted links
- **AND** the user runs `corvint affected --base FULL_SHA` with no provider or profile option
- **THEN** one receipt lists the affected unit tests and the E2E tests of that flow

#### Scenario: No declaration

- **WHEN** the repository has neither file
- **THEN** the plan is byte-identical to the plan before this change

#### Scenario: Both files

- **WHEN** both `.corvint/flows.json` and `.corvint/e2e-flows.json` exist
- **THEN** every E2E test is selected and the plan carries `E2E_FLOWS_INVALID`

### Requirement: APK-V0-002 Every selection carries kind and kindSource

Source: G13. Every selection MUST carry `kind` from the closed set `unit | integration | e2e |
UNKNOWN` and `kindSource` from `runner | declaration | default | conflict`. `e2e` comes from the
runner when the test drives a real browser (Playwright; pytest with pytest-playwright fixtures;
RSpec `type: :system` or `js: true` with a browser driver, not Capybara `rack_test`). `integration`
comes from a project declaration (Go build tag or file-name suffix, pytest marker, Jest or Vitest
project name, or the `kinds` member of `.corvint/flows.json`). Every other test is `unit` with
`kindSource` `default`. Conflicting signals give `UNKNOWN` with `kindSource` `conflict`. Kind MUST
NOT change whether a test is selected.

#### Scenario: Undeclared test

- **WHEN** a selected Go test has no build tag, suffix or declaration
- **THEN** it carries `kind` `unit` and `kindSource` `default`

#### Scenario: Conflict

- **WHEN** a test is in a Vitest project named as integration and also matches a `kinds` rule
  declaring `unit`
- **THEN** it carries `kind` `UNKNOWN` and `kindSource` `conflict` and remains selected

### Requirement: APK-V0-003 E2E selections name flow, variation, path, reason and tier

Source: G6, G2, owner decision R16 (2026-11-09). Every E2E selection MUST name the flow ID and the
changed path that reached the flow, MUST carry the variation ID when the link record names one
(optional member), and MUST carry a one-line reason. A test reached through two flows appears once
and names both. Every selection, of any kind, MUST carry `tier` from `line | file | flow | full`
and the boolean `exact`; a selection made because a changed path reaches a flow carries tier
`flow`, and a `flow` selection MUST NOT be counted as narrowed for the no-missed-test claim.

#### Scenario: Two flows share a test

- **WHEN** a changed path reaches two flows that both link the same E2E test
- **THEN** the test appears once, naming both flows with the changed path and reason for each

#### Scenario: Flow tier

- **WHEN** a test is selected only because a changed path reaches its linked flow
- **THEN** the selection carries tier `flow` and is not counted in the narrowed share

### Requirement: APK-V0-004 Only accepted links plus reachability exclude an E2E test

Source: G2, G11, owner decisions R11 and R12 (2026-11-09). An E2E test MUST be left out of the plan
only when an accepted link record ties it to a flow the change does not reach and static
reachability (file and symbol level) agrees. A link record holds `test_id` (the canonical test ID
of RNC-V1-012, change `canonical-test-id`; it replaces the earlier file-path-plus-title-path key),
`variation_id`, `evidence`, `accepted_by`, `accepted_at` and the pinned commit. Only a record
accepted by a human counts; a proposed record and its confidence score MUST NOT exclude a test. A
record is stale when the cited test content hash or routes changed since the pinned commit. Tags in
test source and line evidence MUST NOT exclude a test.

#### Scenario: Stale link

- **WHEN** the test file of an accepted link changed after the pinned commit
- **THEN** the test is selected, counted `unassigned` and named with reason `stale-link`

#### Scenario: Line evidence says unreached

- **WHEN** line evidence says a change does not reach an E2E test that has no accepted link
- **THEN** the test is still selected

#### Scenario: Proposed link with high confidence

- **WHEN** an E2E test has only a proposed link with confidence 0.99 to a flow the change does not
  reach
- **THEN** the test is still selected and counted `unassigned`

### Requirement: APK-V0-006 Additive, deterministic, bounded and read-only

Source: G13 (reporting label only), invariant 4. The plan MUST keep every existing member unchanged,
MUST be byte-identical for identical inputs, MUST apply the stated read bounds, and MUST write
nothing to the repository.

#### Scenario: Older reader and repeated run

- **WHEN** a consumer written for the earlier plan reads a plan with the new members, and the
  command runs twice on the same inputs
- **THEN** the consumer reads the same units, scope and unknowns as before, both receipts are
  byte-identical, and `git status` shows no change

### Requirement: APK-V0-007 Agreement with the flows impact answer

Source: G5. The flow-selected set MUST contain every test that `corvint flows impact --base
FULL_SHA` names for the same inputs; both MUST run in `corvint` alone on the shared readers.

#### Scenario: Same inputs, two commands

- **WHEN** both commands run on the same base, head and declaration
- **THEN** every test key in the flows impact answer is a flow selection in the plan

Dependency: `corvint flows impact` still runs through the corvint-docs shim at integ `9ec3a29d0`.
Change `flows-status-report` makes it native (AFL-V1-007); APK-V0-007 cannot pass before that lands.

### Requirement: APK-V0-008 A flow selects every linked test of any kind or runner

Source: owner decision R12 (2026-11-09), review finding T-B3, owner decision R31 (2026-11-09).
When a changed path reaches a flow (APK-V0-010), or a commit edits that flow's intent or one of its
link records, the plan MUST select every test that an accepted link record ties to the flow,
whatever its kind (unit, integration, e2e) and whatever its runner. The link schema MUST accept the
canonical test ID of any of the seven runners. A flows edit MUST NOT by itself force a full run;
it selects the tests linked to the changed flows. Links are proposed with a confidence score and
accepted by a human only (change `flow-link-proposals`); this requirement reads accepted records.

#### Scenario: Mixed-kind links

- **GIVEN** flow F with accepted links to a Go unit test, a Vitest integration test and a
  Playwright test
- **WHEN** a commit changes a path that reaches F
- **THEN** all three tests are selected, each naming F and carrying tier `flow`

#### Scenario: Flow intent edited

- **WHEN** a commit changes only the intent file of flow F
- **THEN** the plan selects the tests linked to F, the scope is not `FULL`, and no other runner's
  tests are added on account of the edit

### Requirement: APK-V0-009 Missing flow proof means UNKNOWN and every E2E test

Source: owner brief ("unknown means run everything"), owner answer 4, G4 (settled without the
owner, testing S3), owner decision R10 (2026-11-09). When flow evidence or the E2E inventory is
missing, stale, unreadable, malformed or over a bound, the plan MUST report scope `UNKNOWN` AND
MUST select every E2E test of every affected runner, each with a typed reason naming the failed
evidence. An absent inventory carries `inventory-absent` and a stale one `inventory-stale`. The
builder MUST NOT choose one of the two instead of both. Unit and integration selections are
unchanged.

#### Scenario: Over the bound

- **WHEN** the dirty path count exceeds the join bound
- **THEN** the plan scope is `UNKNOWN` and every inventory test is selected with reason
  `bound-exceeded`

#### Scenario: Unreadable link records

- **WHEN** the link records in `.corvint/flows.json` cannot be parsed
- **THEN** the plan scope is `UNKNOWN`, every E2E test is selected, and each carries reason
  `flow-evidence-unreadable`

#### Scenario: Inventory absent

- **WHEN** the committed inventory file of change `test-inventory` does not exist
- **THEN** the plan scope is `UNKNOWN` and every E2E test is selected with reason
  `inventory-absent`

### Requirement: APK-V0-010 Definition of reach

Source: G2, PROD-TEST-004 (testing M11). A changed path P MUST be said to reach a flow F when F has a
link target (a source file named by an anchor in F's intent, or a source path declared for one of
F's variations) whose owning unit in the static import graph used by `corvint affected` is P's
owning unit or imports it directly or transitively, at symbol level where the language adapter
records symbols and at file level otherwise. This is the reach that APK-V0-001, APK-V0-003,
APK-V0-004 and APK-V0-008 use, and the reach computed by the existing `reach` walk in
`internal/appflows` at integ `9ec3a29d0`. When reach cannot be resolved for P (P has no owning
unit, the target's language has no adapter, or a link target no longer exists), the plan MUST
select every E2E test of the affected runner with reason `reach-unknown`. A browser test outside
the Playwright runner (pytest with pytest-playwright fixtures, RSpec system specs, Rails system
tests) whose flow reach is not resolved MUST be handled the same way: its "group" is every E2E test
of that runner.

#### Scenario: Transitive import

- **GIVEN** flow F whose anchor names `app/routes/cart.ts`, which imports `lib/price.ts`
- **WHEN** a commit changes `lib/price.ts`
- **THEN** the change reaches F, and F's linked tests are selected

#### Scenario: Unresolved reach

- **GIVEN** a changed `.vue` file that has no owning unit in the graph
- **WHEN** the plan is computed
- **THEN** every E2E test of the Playwright runner is selected with reason `reach-unknown`

## MODIFIED Requirements

### Requirement: AFU-V1-1651 The flow declaration file

Source: G6. Text changes from `.corvint/e2e-flows.json` to `.corvint/flows.json` with a versioned
profile; `.corvint/e2e-flows.json` is read for one release with a deprecation notice; both present
is `E2E_FLOWS_INVALID`. The closed-object rule is kept.

#### Scenario: Deprecated file alone

- **WHEN** only `.corvint/e2e-flows.json` exists
- **THEN** it is read and the plan carries a deprecation notice naming `.corvint/flows.json`

### Requirement: AFU-V1-1655 Flow intents are committed

Source: G6. The repository commits flow intents under the directory that `.corvint/flows.json`
names, instead of the directory named by `.corvint/e2e-flows.json`.

#### Scenario: Directory from the new file

- **WHEN** `.corvint/flows.json` names `docs/flows`
- **THEN** flow intents are read from `docs/flows`

## REMOVED Requirements

### Requirement: APK-V0-005 Flow evidence fails closed (superseded by APK-V0-009; retired)

Retired 2026-11-09: the text below let the builder choose between selecting all and reporting
`UNKNOWN`, which conflicts with the owner rule that unknown means run everything. APK-V0-009
replaces it. The rework lane moves every test citation of APK-V0-005 to APK-V0-009 in the same
commit. The ID stays listed as retired and is never reused. Original text, kept for the record:

Source: G4, G13, owner answer 4. When flow evidence or the E2E inventory is missing, stale,
unreadable or over a bound, the plan MUST select every E2E test that evidence could have excluded,
with a typed reason, or report scope `UNKNOWN`. A browser test outside the Playwright runner MUST
select its whole group with a reason.

#### Scenario: Over the bound

- **WHEN** the dirty path count exceeds the join bound
- **THEN** every inventory test is selected with reason `bound-exceeded`
