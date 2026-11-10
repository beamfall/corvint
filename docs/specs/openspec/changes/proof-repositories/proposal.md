# Change proposal: proof repositories, one pinned public repository per runner plus four held out

Change name: `proof-repositories`
Ticket: none filed yet (task 1 files it in `.taskman/`, milestone `v1-0`; it relates to V1-0019)
Product requirement served: PROD-TEST-002, PROD-TEST-011 and PROD-INT-008 in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/proof-repositories-v1.md`, a new spec with its own prefix
`PRR-V1`
Requirement IDs this change adds: PRR-V1-001 to PRR-V1-011
Owner answers this change implements: G1, G2, G3, G12, G12a, answer 17
(`owner-answers-2026-11-09.md`), and owner decisions R6, R19 and R21 (2026-11-09)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/proof-repositories/spec.md`).

## Why

The owner wants the 1.0 numbers measured on public repositories that other people recognise, and
checked on repositories that nobody tuned against (G12). One pinned public repository per non-Go
runner was already required (answer 17). G12 fixes the selection rule and the blocking conditions.
G12a names seven repositories and an alternate for each, all marked as not yet verified. Owner
decision R21 (2026-11-09) names the held-out repositories: fastapi, solidus and payload, each
verified against the licence and activity rules before freezing. The held-out Go repository is
helm/helm, alternate go-task/task (decided 2026-11-09 under R42, A14).

The verification table of the named repositories, the current V1-0019 record and the go-chi/chi
exclusion are in `design.md`.

## What Changes

- A selection rule file, measured against the named repositories, then committed and hashed before
  any run, with a stated green-rate formula.
- A manifest per repository (named or held-out) with every pin.
- The seven named repositories, verified against the rule, with swaps where a check failed.
- A frozen held-out list (fastapi/fastapi, solidusio/solidus, payloadcms/payload, helm/helm) and
  a replacement pool file of at least 5 candidates per held-out repository.
- On the pinned Playwright repository, 10 flows with at least 40 owner-accepted links, frozen and
  hashed into its manifest (decided 2026-11-09 under R42, A16).
- Seven public qualification mirrors `beamfall/corvint-qual-<runner>`, needed by 2026-11-27 and
  created by the owner; each URL enters its manifest after creation (decided 2026-11-09 under
  R42, N4).
- Held-out sets of 100 faults with the R19 pass bound.

## Capabilities

### New Capabilities

- `proof-repositories`: selection rule, named set, held-out list and pools, manifests and blocking
  rules.

### Modified Capabilities

- `untouched-repository-validation` (V1-0019 record): one untouched repository becomes four named
  held-out ones, and chi is excluded from every pool.

## Non-goals

- No flows on held-out repositories; their E2E tests run unnarrowed (G12).
- No vendored third-party source (G12).
- No runs on Unraid hosts (G1).
- No Cypress repository: Cypress is dropped from 1.0 and listed as post-1.0 (owner decision R6
  (2026-11-09)).
- No seeded draw at RC freeze; owner decision R21 (2026-11-09) fixes the held-out list instead.

## Impact

- New files under `benchmarks/proof-repositories/` (proposed path): `rule.json`, `named/*.json`,
  `heldout/list.json`, `heldout/pool-*.json`. Source is fetched, never vendored.
- Used by `narrowing-measurement` (NMS-V1) and `fault-injection-harness` (FIH-V1).
