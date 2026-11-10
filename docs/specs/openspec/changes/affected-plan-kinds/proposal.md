# Change proposal: one affected plan for unit, integration and E2E tests

Change name: `affected-plan-kinds`
Ticket: V1-5251 (open, P1, milestone `v1-0`; the ticket stays in `.taskman/`)
Product requirement served: PROD-TEST-003 and PROD-TEST-004 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/affected-plan-kinds-v0.md`, a new companion to
`docs/specs/affected-plan-v0.md` with its own prefix `APK-V0`
Requirement IDs this change adds: APK-V0-001 to APK-V0-007 (reserved block; meanings rewritten on
2026-11-09 for owner answers G2, G5, G6, G11 and G13), plus APK-V0-008 (any-kind flow links, owner
decision R12 (2026-11-09)), APK-V0-009 and APK-V0-010 (added 2026-11-09 after review). APK-V0-005
is superseded by APK-V0-009 and retired, not reused, because integ `9ec3a29d0` already carries rows
for APK-V0-001 to 007.
Requirement IDs this change modifies: AFU-V1-1651 and AFU-V1-1655 (G6 renames their file)
Intent status: proposed (rewritten 2026-11-09; only the owner can accept it)
Delivery status: experimental code exists on `claude/layers-integ` (lane `oneplan`, head
`1f23ed3ed`, merged compile-only, tests and review not run). That code predates G6, G11, G13 and
the review-round decisions and must be reworked to this text before any promotion.
Written before dispatch: the first version was; this rewrite precedes the rework lane.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/affected-plan-kinds/spec.md`).

## Why

The owner's target is one answer to "which tests does this change require", covering unit,
integration and E2E tests, with E2E tests tied to flows and variations. Before lane `oneplan`,
`corvint affected` returned static units only, E2E selection needed
`--provider FILE --selection-profile e2e-safe`, and the flow join was a separate command.

What the integration branch holds today (verified on `claude/layers-integ` at `9ec3a29d0`):

- Two declaration files with two readers. `.corvint/flows.json`, profile `corvint-flows/0`, with
  members `flows`, `kinds` and `playwright` (`internal/appflows/flowplan.go:31-56`), and
  `.corvint/e2e-flows.json`, profile `corvint-e2e-flows/0`, with members `flows` and `consumers`
  (`internal/appflows/e2eflows.go:32-48`). Corvint's own checkout carries only
  `.corvint/e2e-flows.json`. G6 requires one file.
- A kind enum `unit | integration | e2e | UNKNOWN` (`internal/liveverify/affected/e2eplan.go:12-17`)
  taken only from path-prefix rules in `.corvint/flows.json`; any selection matching no rule is
  `UNKNOWN` (`internal/appflows/flowplan.go:146-162`). G13 instead makes `unit` the default and adds
  `kindSource`.
- An E2E reason `unlinked` (`e2eplan.go:33`). G11 names this state `unassigned`.
- Both readers are called from the tests-layer planner (`cmd/corvint/affected.go:1576-1580`).

Contradictions in the first version that this rewrite resolves:

1. APK-V0-001 versus APK-V0-002. 001 promised integration selections, but 002 allowed a kind only
   from a declaration or E2E inventory membership, and no integration declaration existed. Under
   G13 a kind comes from the runner, a project declaration, or the default, and the source is
   reported; a plan never needs a declaration to label a test.
2. Kind affecting selection. G13 makes kind a reporting label only. APK-V0-002 now says so.
3. The removal basis. The first version did not say what may exclude an E2E test. G2 and G11 make
   accepted link records plus static reachability the only basis (APK-V0-004).
4. E2E-only flow links. The first version let a flow link only Playwright tests. Owner decision R12
   (2026-11-09) and review finding T-B3 make a flow select every linked test of any kind or runner,
   keyed by the canonical test ID (APK-V0-008).

## What Changes

- One tracked file, `.corvint/flows.json`, with a versioned profile, declares the flows directory,
  the Playwright member, the kind declarations and the accepted test links (G6, G11). A reader
  refuses an unknown profile version (G5).
- `.corvint/e2e-flows.json` is still read for one release with a deprecation notice. Both files
  present is `E2E_FLOWS_INVALID` and the plan fails closed (G6).
- When the declaration exists, plain `corvint affected --base FULL_SHA` returns unit, integration
  and E2E selections in one receipt with no extra option.
- Every selection carries `kind` and `kindSource` (G13). Kind never changes selection.
- Every selection carries a `tier` from the one vocabulary `line | file | flow | full` plus the
  `exact` flag (owner decision R16 (2026-11-09)). A test selected because a changed path reaches a
  flow carries tier `flow`, which does not count as narrowed for the no-missed-test claim.
- E2E selection is per flow in 1.0 (frozen). Each E2E selection names the flow, the variation when
  known (optional member), the changed path and a one-line reason (G2, G6).
- A flow selects every accepted linked test of any kind and any runner, not only E2E tests (owner
  decision R12 (2026-11-09); APK-V0-008). Link records key tests by the canonical test ID
  (RNC-V1-012, delivered by change `canonical-test-id`). Links are proposed with a confidence score
  and accepted by a human only (change `flow-link-proposals`); this change reads accepted records.
- A commit that edits a flow intent or a link record selects the tests linked to the changed flows;
  it does not force a full run (owner decision R31 (2026-11-09)).
- An E2E test leaves the plan only through an accepted, non-stale link record whose static
  reachability agrees (G2, G11). Line evidence may add or rank, never remove.
- "Reach" is defined once, on the static import graph of `corvint affected` (APK-V0-010). When reach
  cannot be resolved, every E2E test of that runner is selected with reason `reach-unknown`.
- When flow evidence or the inventory fails, the plan is `UNKNOWN` and selects every E2E test; the
  builder has no choice between the two (APK-V0-009, settled without the owner, testing S3). The
  inventory is the single committed inventory of change `test-inventory`; its absence and staleness
  carry the reasons `inventory-absent` and `inventory-stale` (owner decision R10 (2026-11-09)).
- A repository with no declaration gets exactly the plan it got before this change.

## Capabilities

### New Capabilities

- `affected-plan-kinds`: one affected plan covering every test kind, with flow and variation names
  on E2E selections. Carried by the new spec `affected-plan-kinds-v0.md`.

### Modified Capabilities

- `affected-plan`: additive plan members only (`docs/specs/affected-plan-v0.md`).
- `application-flow-understanding`: AFU-V1-1651 and AFU-V1-1655 are rewritten for
  `.corvint/flows.json` (see the spec delta).

## Non-goals

- Writing the E2E inventory or any test inventory (changes `e2e-inventory-discovery` (V1-5252) and
  `test-inventory`; the write verb is `corvint inventory refresh`, owner decision R10 (2026-11-09)).
- Proposing or accepting links: `flows propose --links` and acceptance belong to `corvint-docs`
  (G5, G11) and change `flow-link-proposals`. This change reads accepted records only.
- Defining the canonical test ID grammar (change `canonical-test-id`, RNC-V1-012).
- The unlinked-tests report and its codes (change `flows-unlinked-tests-report`).
- Per-variation narrowing (additive after 1.0, G6).
- Kind changing selection, or kind guessed from a file name (G13).
- Running the selected tests (change `runner-contract`).

## Impact

- Code: `cmd/corvint/affected.go`, `internal/liveverify/affected`, `internal/appflows`.
- Layering (verified): `tools/import-ratchet/areas.json` puts `internal/appflows` in the `shared`
  layer (line 11) and `internal/liveverify` in `tests` (line 142); the rule (line 4) says shared
  imports no other layer. Six non-test files in `internal/appflows` import
  `internal/liveverify/affected` today (`e2eflows.go:19`, `impact.go:13`, `flowplan.go:18`,
  `repositories.go`, `selection.go`, `unlinked.go`), and the edge is grandfathered in
  `tools/import-ratchet/baseline.json:5-9` together with `doccorpus`, `jstestprovider`,
  `affected/languages` and `affected/typescript`. Required direction: the tests layer
  (`internal/liveverify/affected`, `cmd/corvint`) imports `internal/appflows`; `internal/appflows`
  keeps only the flow model, the declaration reader and the link-record reader (owner answer 14) and
  imports nothing outside `shared`. The join code that needs `affected.Graph` or `affected.Plan`
  (`BindE2EFlows`, `JoinFlowPlan`, `KindOf`) moves to the tests layer. This change must remove,
  not add to, the five baselined edges for the files it touches.
- Wire: the plan receipt gains members (`kind`, `kindSource`, `tier`, flow names). The lane states
  the version decision in the spec first (`docs/specs/core-compatibility-freeze-v1.md` may cover
  that wire).
- Callers: `corvint test`, the `corvint.affected` MCP tool and CI narrowing must ignore or carry the
  new members.
- Why a new spec: `affected-plan-v0.md` is at its obligation limit (256), so the capacity check asks
  for a companion spec with its own prefix.
- Dependencies by name: `canonical-test-id` (link key), `test-inventory` (inventory and its
  reasons), `flow-link-proposals` (how links are accepted), `flows-status-report` (native
  `corvint flows impact`, needed by APK-V0-007).
