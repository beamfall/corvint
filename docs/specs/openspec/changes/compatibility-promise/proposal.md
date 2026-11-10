# Change proposal: a narrow 1.x compatibility promise, written as one generated table

Change name: `compatibility-promise`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: product gap (g) and checklist rows O30 and O64 of
`docs/specs/PRODUCT-PLAN-1.0.md` (NEW-PROC-3). The plan has no PROD requirement for the promise
yet; the plan edit that adds one is listed for the owner and is not made by this change.
Owning capability spec: `docs/specs/compatibility-promise-v1.md`, a new spec with prefix `CMP-V1`
Requirement IDs this change adds: CMP-V1-001 to CMP-V1-006 (no `CMP-` row exists in
`docs/specs/REQUIREMENTS.tsv` and no other change folder uses the prefix, checked 2026-11-09)
Requirement IDs this change modifies: none (UDC-V1-007 in change `user-docs` drops its "not yet
decided" wording in that change, not here)
Owner answers this change implements: the decisions made 2026-11-09 under R42: D10 and D10a
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/compatibility-promise/spec.md`.

## Why

A 1.0 release with no stated promise is read as promising everything. Users, the action and the
agent packages need to know what a 1.x release can change. D10 chose a narrow promise that covers
only surfaces a contract test already protects. D10a chose its form: one table in
`docs/COMPATIBILITY.md` with a `stable` or `preview` column, and no label renamed at 1.0.

## What exists today

- `docs/COMPATIBILITY.md` does not hold a promise. Change `user-docs` (UDC-V1-007) reserves the
  page and says the statement is not yet decided.
- Several output formats carry a label that ends in `/0`. Which label each stable format carries
  today is UNVERIFIED beyond the label counts in the change folders.
- How many of the surfaces in CMP-V1-003 have a contract test today is NOT_VERIFIED.

## What Changes

- `docs/COMPATIBILITY.md` becomes one generated table: one row per surface, a stability column,
  and the contract test that protects each stable row.
- Stable surfaces change only by addition; a removal needs one minor release of notice and an
  alias.
- Preview surfaces are listed by name and can change in any release.
- No label is renamed at 1.0. A breaking change to a stable profile takes the next digit and the
  old profile is served for one minor release.
- A check fails when a stable row names no existing contract test or when the committed table
  differs from the generated one.

## Capabilities

### New Capabilities

- `compatibility-promise`: the table, its generator, its check and the rules for stable and
  preview surfaces.

### Modified Capabilities

- none. `user-docs` links to the page; its own change edits UDC-V1-007.

## Impact

- `docs/COMPATIBILITY.md`, one generator and one check with tests.
- Changes `github-action` and `agent-packages` name the action's major tag and the declared
  Corvint range; this change lists them as stable rows and does not define them.
- Release notes gain a line for every preview change.

## Non-goals

- No promise for MCP input and output schemas, Go packages, the index or any cache.
- No new contract test written only to widen the promise; a surface without a test is not stable
  in 1.0.
- No label rename and no wire change.
- No promise beyond 1.x.
