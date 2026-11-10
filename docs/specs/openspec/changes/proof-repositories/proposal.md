# Change proposal: proof repositories, one pinned public repository per runner plus four held out

Change name: `proof-repositories`
Ticket: none filed yet (task 1 files it in `.taskman/`, milestone `v1-0`; it relates to V1-0019)
Product requirement served: PROD-TEST-002, PROD-TEST-011 and PROD-INT-008 in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/proof-repositories-v1.md`, a new spec with its own prefix
`PRR-V1`
Requirement IDs this change adds: PRR-V1-001 to PRR-V1-011
Owner answers this change implements: G1, G2, G3, G12, G12a, answer 17
(`owner-answers-2026-11-09.md`), owner decisions R6, R19 and R21 (2026-11-09), and owner
decision H3 (2026-11-10)
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
decision R21 (2026-11-09) named a fixed held-out list; owner decision H3 (2026-11-10) replaced
it with a seeded draw that is never redrawn: one held-out repository per family (Go, JS/TS,
Python, Ruby) is drawn from a sealed, hashed pool. fastapi, solidus, payload, helm and go-task
(the names of R21 and A14) are pool candidates, not a fixed list.

The verification table of the named repositories, the current V1-0019 record and the go-chi/chi
exclusion are in `design.md`.

## What Changes

- A selection rule file, measured against the named repositories, then committed and hashed before
  any run, with a stated green-rate formula.
- A manifest per repository (named or held-out) with every pin.
- The seven named repositories, verified against the rule, with swaps where a check failed.
- Four sealed, hashed pool files, one per family, each with at least 5 candidates checked against
  the held-out rule and the untouched search, sealed before the first candidate tag is pushed.
- A deterministic draw of one held-out repository per family, seeded by the commit SHA of the first
  pushed candidate tag and never redrawn (owner decision H3 (2026-11-10)), then a drawn list with
  each repository's pin and check results.
- On the pinned Playwright repository, 10 flows with at least 40 owner-accepted links, frozen and
  hashed into its manifest (decided 2026-11-09 under R42, A16).
- Seven public qualification mirrors `beamfall/corvint-qual-<runner>`, needed by 2026-11-27 and
  created by the owner; each URL enters its manifest after creation (decided 2026-11-09 under
  R42, N4).
- Held-out sets of 100 faults with the R19 pass bound.

## Capabilities

### New Capabilities

- `proof-repositories`: selection rule, named set, held-out pools and draw, manifests and
  blocking rules.

### Modified Capabilities

- `untouched-repository-validation` (V1-0019 record): one untouched repository becomes four named
  held-out ones, and chi is excluded from every pool.

## Non-goals

- No flows on held-out repositories; their E2E tests run unnarrowed (G12).
- No vendored third-party source (G12).
- No runs on Unraid hosts (G1).
- No Cypress repository: Cypress is dropped from 1.0 and listed as post-1.0 (owner decision R6
  (2026-11-09)).
- No fixed held-out list and no redraw: owner decision H3 (2026-11-10) replaces the fixed list of
  R21 (2026-11-09) with a seeded draw that is never redrawn. A drawn repository that proves
  unusable is reported, not replaced.
- No choice of pool members and no dates here: the members of each pool are NOT_DECIDED.

## Schedule risk

The fault sets for the drawn repositories can only be built after the first candidate tag exists,
because the seed is that tag's commit SHA. They are built between the candidate tag and the freeze,
inside the M7 window of `fault-injection-harness` (FIH-V1-010, FIH-V1-012). This is a risk to the
schedule and the compute budget, not a decision. It also applies to the 50-PR replay on the drawn
repositories (A12).

## Impact

- New files under `benchmarks/proof-repositories/` (proposed path): `rule.json`, `named/*.json`,
  `heldout/pool-*.json` (sealed before the first candidate tag), `heldout/draw.json` and
  `heldout/list.json` (after the draw). Source is fetched, never vendored.
- Used by `narrowing-measurement` (NMS-V1) and `fault-injection-harness` (FIH-V1).
