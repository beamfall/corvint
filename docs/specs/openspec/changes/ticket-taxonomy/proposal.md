# Change proposal: ticket categories, identity key, required fields and search

Change name: `ticket-taxonomy`
Ticket: none filed yet (task 1.1 files it in the corvint-tasks repository's `.taskman/` store and
links it from a `v1-0` ticket in this repository)
Product requirement served: PROD-TASK-005, PROD-TASK-006 and PROD-TASK-007, and PROD-INT-005 and
PROD-INT-011, in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/ticket-taxonomy-v1.md` in the `beamfall/corvint-tasks`
repository, a new spec with prefix `TTX-V1`
Requirement IDs this change adds: TTX-V1-001 to TTX-V1-018
Requirement IDs this change modifies: none
Owner answers this change implements: owner request of 2026-11-09 ("develop a way to be able to
categorize tickets better so that they can be quickly searched, duplicates can't exist and they
provide the exact information needed"); owner approval of 2026-11-09 for a fresh store seeded only
with surviving tickets
Related changes, referred to by name: `tasks-store-cutover` (waits for this change, TSC-V1-001, and
seeds in this shape, TSC-V1-009), `tasks-conformance` (schema v1 and the supported-command list,
which the new field and filters must be part of), `tasks-hardening`, `ticket-brief-context`
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/ticket-taxonomy/spec.md`. The store changes land in the `beamfall/corvint-tasks` repository.
The vocabulary file and the interim gate script land in this repository.

Prefix check, run 2026-11-09 in this checkout: `grep -cE "^TTX-" docs/specs/REQUIREMENTS.tsv`
printed 0, and `grep -rlE "\bTTX-" docs/` printed no file. `TTX-` has no entry in
`docs/specs/REQUIREMENTS.tsv` and no other change folder uses it.

## Why

Tickets are filed mostly by agents in a hurry. The open store shows the cost. Measured on
2026-11-09 over 1,029 open tickets: 81 free-text labels, 306 different title prefixes, 956 tickets
naming no requirement and none naming a `PROD-*` requirement, 927 tickets whose next action is
`set-effects`. A classification pass found 48 obsolete or duplicate tickets and 160 filed against
the wrong product. PROD-TASK-006 requires every ticket closed for 1.0 to name a requirement, and the
standing rule is that nothing is built before a committed spec.

## What exists today

Observed on 2026-11-09 in the source of corvint-tasks build `1.0.0-rc.3.dev+build.144` (paths under
`internal/tasks/`):

- Labels are free text, 1 to 64 bytes. The policy has no closed label list.
  (`wire/primitives.go:194-197`, `intent/policy.go:108-175`)
- A policy can require one field at create: `milestones.required`.
  (`mutation/apply.go:785-787`, `intent/policy.go:277-289`)
- `requirementRefs` accepts any identifier. Nothing checks that it exists.
  (`mutation/mutation.go:795-796`)
- Create refuses a duplicate ticket ID inside the write transaction.
  (`ticket/graph.go:204-214`, `mutation/apply.go:793-797`)
- Create runs a lexical duplicate check, but only for tickets labelled `agent-memory`, only against
  open tickets, and before the write. (`cli/similar.go:424-492`)
- Search has one `--label` filter and no filter on requirement, touch path or key.
  (`cli/cli.go:964`, `:1430-1458`)
- A ticket view never reports ELIGIBLE. Coverage other than QUALIFIED adds an advisory and the next
  action `set-effects`. (`ticket/view.go:300-304`, `:321-335`)

## What Changes

- Six facets, all closed and all required: component, product (computed from component), type,
  anchor (requirement IDs), runner or agent where it applies, and milestone.
- An identity key per ticket, built from the facets and one identifier taken from the evidence. A
  create whose key is already held is refused inside the write transaction. A second ticket under
  one key is allowed only when it names every open holder and says how it differs.
- A short template per type (bug, delivery, test-gap, optimization, decision, chore) with required
  lines, qualified effects with at least one touch path, and at least one acceptance criterion.
- Search filters for key, requirement and touch path, and more than one label filter.
- An interim gate script that applies the same rules before each create until the store enforces
  them. The same script is the gate for seeding the fresh store.

## Capabilities

### New Capabilities

- `ticket-taxonomy` (corvint-tasks repository): facet policy, identity key and collision refusal,
  per-type required fields, search filters.

### Modified Capabilities

- None in this repository. The vocabulary file and the gate script are new files.

## Impact

- corvint-tasks repository: ticket record (one new field, one new list), policy (one new block),
  create path, search filters, schema v1 and the supported-command list.
- This repository: `docs/specs/ticket-vocabulary.json` (proposed path) and the gate script.
- `tasks-store-cutover`: its mapping table has the columns this change defines.
- Agents and skills that file tickets: the create payload changes shape.

## Non-goals

- No decision on which tickets are seeded. That is `tasks-store-cutover`.
- No free-text labels, and no facet that is optional.
- No semantic or embedding search. Matching stays lexical and local.
- No tracker or parent tickets. A tracker is a search on the anchor.
- No change to claims, gates, completion or receipts.
