# Spec delta: test-inventory

Capability spec: `docs/specs/test-inventory-v1.md` (new). This delta is a proposal only; the
canonical spec is updated in the same change (G14).

## ADDED Requirements

### Requirement: TIV-V1-001 One committed inventory
Source: owner decision R10 (2026-11-09); review T-B5.
Status: proposed 2026-11-09; delivery not-started.
Corvint MUST keep one committed inventory file for all seven blocking runners at
`.corvint/test-inventory.json` with profile `corvint-test-inventory/0`. Every entry MUST be keyed by
the canonical test ID (CTI-V1-001) and name the file the test lives in.

#### Scenario: repository with Go and Vitest
- **GIVEN** a repository with one Go module and one Vitest project
- **WHEN** `corvint inventory refresh` completes
- **THEN** `.corvint/test-inventory.json` holds one instance record for each, with canonical IDs

### Requirement: TIV-V1-002 Only the refresh verb writes the inventory
Source: owner decision R10 (2026-11-09); review T-M4.
Status: proposed 2026-11-09; delivery not-started.
`corvint inventory refresh` MUST be the only command that writes the inventory. `corvint affected`,
`corvint test`, `corvint watch` and the MCP tools MUST only read it and MUST NOT run a lister.
`corvint watch` MUST NOT refresh the inventory. The GitHub Action MUST run
`corvint inventory refresh` before it plans.

#### Scenario: watch with a stale inventory
- **GIVEN** a stale inventory
- **WHEN** `corvint watch` handles a save
- **THEN** it reports `inventory-stale`, runs the widened selection and leaves the file unchanged

### Requirement: TIV-V1-003 Runner instances keyed by runner, root and config
Source: owner decision R15 (2026-11-09); review T-M8.
Status: proposed 2026-11-09; delivery not-started.
Each inventory record MUST describe one runner instance identified by runner, root directory and
config file. A repository MAY declare several instances of one runner. `CORVINT_SELECTION=full:<runner>`
MUST apply to every instance of that runner.

#### Scenario: two Jest roots
- **GIVEN** Jest instances at `apps/web` and `apps/admin`
- **WHEN** `CORVINT_SELECTION=full:jest` is set
- **THEN** both instances run their full commands

### Requirement: TIV-V1-004 One lister per runner, marked static or executing
Source: review T-B5.
Status: proposed 2026-11-09; delivery not-started.
Each runner MUST have one lister, and each instance record MUST state whether its lister was
`static` (reads source only) or `executing` (runs project code). Go, RSpec and Minitest listers are
static; Vitest, Jest, Playwright and pytest listers are executing. An executing lister MUST run only
inside `corvint inventory refresh`.

#### Scenario: Playwright record
- **GIVEN** a refreshed inventory for a Playwright project
- **WHEN** the record is read
- **THEN** it states lister kind `executing` and the lister command used

### Requirement: TIV-V1-005 Inventory bound to the tree
Source: review T-B5.
Status: proposed 2026-11-09; delivery not-started.
Each instance record MUST store the commit it was refreshed at, the config file digest, the lockfile
digest when one exists, the lister name and version, and a blob digest for every listed test file.

#### Scenario: binding fields present
- **GIVEN** a refreshed pytest instance
- **WHEN** the record is read
- **THEN** it carries the commit, config digest, lister version and one blob digest per test file

### Requirement: TIV-V1-006 Absent inventory runs the declared full command
Source: owner decision R10 (2026-11-09); review T-M3.
Status: proposed 2026-11-09; delivery not-started.
When the inventory file is missing, or has no record for a declared runner instance, that instance
MUST run its declared full command and the plan MUST print reason `inventory-absent`.

#### Scenario: no inventory committed
- **GIVEN** no `.corvint/test-inventory.json`
- **WHEN** `corvint affected` plans any change
- **THEN** every declared runner instance is selected in full with reason `inventory-absent`

### Requirement: TIV-V1-007 Config, lockfile or lister change makes the instance stale
Source: owner decision R10 (2026-11-09); review T-M2.
Status: proposed 2026-11-09; delivery not-started.
When the config file digest, the lockfile digest, the lister version or the inventory profile
differs from the record, every test of that instance MUST run and the plan MUST print reason
`inventory-stale`.

#### Scenario: Vitest config edited
- **GIVEN** `vitest.config.ts` changed since the last refresh
- **WHEN** `corvint affected` plans the change
- **THEN** every Vitest test of that instance is selected with reason `inventory-stale`

### Requirement: TIV-V1-008 A changed test file makes only its tests unassigned
Source: review T-M2.
Status: proposed 2026-11-09; delivery not-started.
When a listed test file's blob digest differs from the record, or a new test file appears under the
instance root, every test in that file MUST run and be counted `unassigned`. Other files keep their
entries.

#### Scenario: new test added to one file
- **GIVEN** a test added to `spec/user_spec.rb` after the refresh
- **WHEN** `corvint affected` plans the change
- **THEN** `spec/user_spec.rb` runs as `unassigned` and other RSpec files are planned from their entries

### Requirement: TIV-V1-009 Invalid inventory runs every test
Source: every unknown runs with a named reason (owner decisions R10 and R32, 2026-11-09).
Status: proposed 2026-11-09; delivery not-started.
When the inventory file cannot be read, fails schema validation or has an unknown profile, every test
of every declared runner MUST run and the plan MUST print reason `inventory-invalid`.

#### Scenario: truncated file
- **GIVEN** a truncated `.corvint/test-inventory.json`
- **WHEN** `corvint affected` plans any change
- **THEN** every declared runner runs in full with reason `inventory-invalid`

### Requirement: TIV-V1-010 Refresh refuses drift
Source: review T-B5; same rule as the existing discovery receipt (TJAA-V0-019).
Status: proposed 2026-11-09; delivery not-started.
`corvint inventory refresh` MUST record HEAD and the source digest before and after each lister and
MUST refuse with `inventory-refresh-drift`, leaving the previous file unchanged, if either moved.

#### Scenario: file edited during refresh
- **GIVEN** a test file is edited while the Jest lister runs
- **WHEN** refresh finishes the lister
- **THEN** it refuses with `inventory-refresh-drift` and the old inventory is kept

### Requirement: TIV-V1-011 Refresh bounds output and records lister failures
Source: review T-B5.
Status: proposed 2026-11-09; delivery not-started.
Refresh MUST refuse with `inventory-too-large` when the file would exceed its size cap. When one
lister fails, refresh MUST write the other instances, omit the failed instance and record the
failure, so readers treat that instance as absent.

#### Scenario: pytest collection error
- **GIVEN** `pytest --collect-only` exits with a collection error
- **WHEN** refresh completes
- **THEN** the pytest instance is omitted with the error recorded and the next plan runs pytest in
  full with `inventory-absent`

### Requirement: TIV-V1-012 Add-only schema
Source: settled D25 (add-only rule for receipts and inventories).
Status: proposed 2026-11-09; delivery not-started.
New fields MAY be added to `corvint-test-inventory/0`; existing fields MUST NOT change meaning. A
reader MUST ignore fields it does not know. A change of meaning requires a new profile, which old
readers treat as `inventory-invalid`.

#### Scenario: newer writer adds a field
- **GIVEN** an inventory with an extra field from a newer writer
- **WHEN** an older reader plans a change
- **THEN** it ignores the field and plans normally

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
