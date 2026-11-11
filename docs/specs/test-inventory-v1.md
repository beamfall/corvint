# Test inventory V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: proposed
Delivery status: not-started
Authoritative inputs: change folder `docs/specs/openspec/changes/test-inventory/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/test-inventory/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-004, PROD-TEST-005, PROD-TEST-017) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner decisions R10, R15 and R38).

## Agent digest
- Claim: One committed inventory lists every test of the seven blocking runners by canonical ID; a missing or doubtful list runs everything.
- Status: proposed / not-started. Change `test-inventory` is second in the R38 testing order, after `canonical-test-id`.
- Exists: the change folder; ticket V1-5406 delivers it.
- Blocked on: `canonical-test-id` (CTI-V1-001) for the entry key.
- Read next: Requirements; Surfaces; Traceability.

## User and boundary

An agent, a developer or the GitHub Action plans which tests a change runs. Before this change only
Playwright had a listing step (`corvint affected discovery`, a receipt printed to stdout), `corvint
test` read declared full commands with no list of individual tests, and no command wrote a list of
tests to the repository. A selection had no known list to start from.

This spec is the canonical home of requirements TIV-V1-001 to TIV-V1-012, copied from the change's
`specs/test-inventory/spec.md`. Design decisions, failure modes and rollback stay in the change's
`design.md`, which owns them; this file adds only the delivery record (Surfaces, Traceability).
Selection from the inventory belongs to `e2e-inventory-discovery` and `runner-contract`.

## Requirements

- `TIV-V1-001`: (proposed 2026-11-09; delivery not-started) One committed inventory

Corvint MUST keep one committed inventory file for all seven blocking runners at
`.corvint/test-inventory.json` with profile `corvint-test-inventory/0`. Every entry MUST be keyed by
the canonical test ID (CTI-V1-001) and name the file the test lives in.

**Scenario: repository with Go and Vitest**
- GIVEN a repository with one Go module and one Vitest project
- WHEN `corvint inventory refresh` completes
- THEN `.corvint/test-inventory.json` holds one instance record for each, with canonical IDs

- `TIV-V1-002`: (proposed 2026-11-09; delivery not-started) Only the refresh verb writes the inventory

`corvint inventory refresh` MUST be the only command that writes the inventory. `corvint affected`,
`corvint test`, `corvint watch` and the MCP tools MUST only read it and MUST NOT run a lister.
`corvint watch` MUST NOT refresh the inventory. The GitHub Action MUST run
`corvint inventory refresh` before it plans.

**Scenario: watch with a stale inventory**
- GIVEN a stale inventory
- WHEN `corvint watch` handles a save
- THEN it reports `inventory-stale`, runs the widened selection and leaves the file unchanged

- `TIV-V1-003`: (proposed 2026-11-09; delivery not-started) Runner instances keyed by runner, root and config

Each inventory record MUST describe one runner instance identified by runner, root directory and
config file. A repository MAY declare several instances of one runner. `CORVINT_SELECTION=full:<runner>`
MUST apply to every instance of that runner.

**Scenario: two Jest roots**
- GIVEN Jest instances at `apps/web` and `apps/admin`
- WHEN `CORVINT_SELECTION=full:jest` is set
- THEN both instances run their full commands

- `TIV-V1-004`: (proposed 2026-11-09; delivery not-started) One lister per runner, marked static or executing

Each runner MUST have one lister, and each instance record MUST state whether its lister was
`static` (reads source only) or `executing` (runs project code). Go, RSpec and Minitest listers are
static; Vitest, Jest, Playwright and pytest listers are executing. An executing lister MUST run only
inside `corvint inventory refresh`.

**Scenario: Playwright record**
- GIVEN a refreshed inventory for a Playwright project
- WHEN the record is read
- THEN it states lister kind `executing` and the lister command used

- `TIV-V1-005`: (proposed 2026-11-09; delivery not-started) Inventory bound to the tree

Each instance record MUST store the commit it was refreshed at, the config file digest, the lockfile
digest when one exists, the lister name and version, and a blob digest for every listed test file.

**Scenario: binding fields present**
- GIVEN a refreshed pytest instance
- WHEN the record is read
- THEN it carries the commit, config digest, lister version and one blob digest per test file

- `TIV-V1-006`: (proposed 2026-11-09; delivery not-started) Absent inventory runs the declared full command

When the inventory file is missing, or has no record for a declared runner instance, that instance
MUST run its declared full command and the plan MUST print reason `inventory-absent`.

**Scenario: no inventory committed**
- GIVEN no `.corvint/test-inventory.json`
- WHEN `corvint affected` plans any change
- THEN every declared runner instance is selected in full with reason `inventory-absent`

- `TIV-V1-007`: (proposed 2026-11-09; delivery not-started) Config, lockfile or lister change makes the instance stale

When the config file digest, the lockfile digest, the lister version or the inventory profile
differs from the record, every test of that instance MUST run and the plan MUST print reason
`inventory-stale`.

**Scenario: Vitest config edited**
- GIVEN `vitest.config.ts` changed since the last refresh
- WHEN `corvint affected` plans the change
- THEN every Vitest test of that instance is selected with reason `inventory-stale`

- `TIV-V1-008`: (proposed 2026-11-09; delivery not-started) A changed test file makes only its tests unassigned

When a listed test file's blob digest differs from the record, or a new test file appears under the
instance root, every test in that file MUST run and be counted `unassigned`. Other files keep their
entries.

**Scenario: new test added to one file**
- GIVEN a test added to `spec/user_spec.rb` after the refresh
- WHEN `corvint affected` plans the change
- THEN `spec/user_spec.rb` runs as `unassigned` and other RSpec files are planned from their entries

- `TIV-V1-009`: (proposed 2026-11-09; delivery not-started) Invalid inventory runs every test

When the inventory file cannot be read, fails schema validation or has an unknown profile, every test
of every declared runner MUST run and the plan MUST print reason `inventory-invalid`.

**Scenario: truncated file**
- GIVEN a truncated `.corvint/test-inventory.json`
- WHEN `corvint affected` plans any change
- THEN every declared runner runs in full with reason `inventory-invalid`

- `TIV-V1-010`: (proposed 2026-11-09; delivery not-started) Refresh refuses drift

`corvint inventory refresh` MUST record HEAD and the source digest before and after each lister and
MUST refuse with `inventory-refresh-drift`, leaving the previous file unchanged, if either moved.

**Scenario: file edited during refresh**
- GIVEN a test file is edited while the Jest lister runs
- WHEN refresh finishes the lister
- THEN it refuses with `inventory-refresh-drift` and the old inventory is kept

- `TIV-V1-011`: (proposed 2026-11-09; delivery not-started) Refresh bounds output and records lister failures

Refresh MUST refuse with `inventory-too-large` when the file would exceed its size cap. When one
lister fails, refresh MUST write the other instances, omit the failed instance and record the
failure, so readers treat that instance as absent.

**Scenario: pytest collection error**
- GIVEN `pytest --collect-only` exits with a collection error
- WHEN refresh completes
- THEN the pytest instance is omitted with the error recorded and the next plan runs pytest in
  full with `inventory-absent`

- `TIV-V1-012`: (proposed 2026-11-09; delivery not-started) Add-only schema

New fields MAY be added to `corvint-test-inventory/0`; existing fields MUST NOT change meaning. A
reader MUST ignore fields it does not know. A change of meaning requires a new profile, which old
readers treat as `inventory-invalid`.

**Scenario: newer writer adds a field**
- GIVEN an inventory with an extra field from a newer writer
- WHEN an older reader plans a change
- THEN it ignores the field and plans normally

## Surfaces

Not built yet; ticket V1-5406 records them here as each task lands.

## Traceability

| Requirement | Evidence |
|---|---|
| TIV-V1-001 to TIV-V1-012 | NOT_PRODUCED: not built yet |

## Non-goals

No selection logic (`e2e-inventory-discovery`, `runner-contract`); no running of tests by
`affected`, the planning step of `test`, `watch` or MCP to build the list (product invariant 4); no
cross-package analysis and no sharding outside the GitHub Action (owner decision R15); no runner
beyond the seven blocking runners; no line evidence (`line-evidence`) and no run results
(`run-evidence`).

## Rollback

Revert the change's commits and delete `.corvint/test-inventory.json`. Readers then see an absent
inventory and run each runner's full command with `inventory-absent`, the safe fallback. TIV-V1-001
to TIV-V1-012 stay listed as retired if the change is withdrawn and are never reused.
