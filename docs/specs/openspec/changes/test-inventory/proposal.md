# Change proposal: one committed test inventory for the seven blocking runners

Change name: `test-inventory`
Ticket: none filed yet; to be filed in `.taskman/` before dispatch (not filed by this proposal).
Product requirement served: PROD-TEST-004, PROD-TEST-005 and PROD-TEST-017 in
`docs/specs/PRODUCT-PLAN-1.0.md` (every selection starts from a known list of tests; an unknown list
runs everything).
Owning capability spec: a new spec `docs/specs/test-inventory-v1.md` (prefix `TIV-V1`). It does not
exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: TIV-V1-001 to TIV-V1-012. `TIV-` has no entry in
`docs/specs/REQUIREMENTS.tsv` and appears in no other folder under `docs/specs/openspec/changes/`
(checked 2026-11-09).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes.

## Needs first

- `canonical-test-id`: every inventory entry is keyed by the canonical test ID (CTI-V1-001).

This change is second in the build order fixed by owner decision R38 (2026-11-09).

## Why

Owner decision R10 (2026-11-09): one committed inventory under `.corvint/` for all seven runners,
written by an explicit `corvint inventory refresh`; a missing or stale inventory runs all tests with
reason `inventory-absent` or `inventory-stale`. Owner decision R15 (2026-11-09): several instances
per runner, keyed by root; sharding stays in the GitHub Action.

Review findings testing T-B5, T-M2, T-M3, T-M4 and T-M8 ask for one path and profile, a lister per
runner marked static or executing, binding to the tree, per-file staleness, one refresh command,
the full command when the inventory is absent, and instances keyed by runner, root and config.

### What exists at integ 9ec3a29d0

- Only Playwright has a listing step. `corvint affected discovery` requires `--playwright-config` and
  `--playwright-list` (a listing taken beforehand) and prints a `playwright-discovery/0` receipt to
  stdout, bound to HEAD, the config bytes and a source digest, and refuses drift
  (`cmd/corvint/affected.go`, lines 634-672 and 1058-1071, integ 9ec3a29d0). Nothing is written to
  the repository.
- `corvint test` reads declared commands from `.github/cishards/test-commands.json`
  (`internal/sessiontest/declared.go`, lines 26-29, integ 9ec3a29d0) and has no list of individual
  tests.
- No `inventory` verb exists: the top-level verb list in `cmd/corvint/main.go` (lines 644-646,
  integ 9ec3a29d0) has none.
- Change `runner-contract` names a listing command per runner (Go `go test -list`, Vitest
  `vitest list --json`, Jest `jest --listTests`, Playwright `--list --reporter=json`, pytest
  `--collect-only -q`, RSpec `--dry-run --format json`; Minitest has none).

## What changes

- One file, `.corvint/test-inventory.json`, profile `corvint-test-inventory/0`, committed
  (TIV-V1-001). The file name is proposed; see Open in design.md.
- One writing verb, `corvint inventory refresh`. `corvint affected`, `corvint test`, `corvint watch`
  and MCP only read the file. `corvint watch` refreshes nothing. The GitHub Action runs the refresh
  before planning (TIV-V1-002).
- Runner instances keyed by runner, root and config file; several instances of one runner are
  allowed; `CORVINT_SELECTION=full:<runner>` applies to every instance of that runner (TIV-V1-003).
- One lister per runner, each marked `static` or `executing` (TIV-V1-004).
- Each instance is bound to the commit, the config digest, the lister version and a blob digest per
  test file (TIV-V1-005).
- Absent, stale and invalid inventories run tests with a named reason; a changed test file makes only
  that file's tests `unassigned` (TIV-V1-006 to TIV-V1-009).
- Refresh refuses drift and bounds its output (TIV-V1-010, TIV-V1-011).
- Inventories are add-only in schema (TIV-V1-012).

## Non-goals

- No selection logic. How E2E selection reads the inventory is change `e2e-inventory-discovery`
  (AFU-V1-146 to AFU-V1-149); how other runners select is `runner-contract`.
- No running of tests by `affected`, `test`'s planning step, `watch` or MCP to build the list
  (product invariant 4).
- No cross-package analysis and no sharding outside the Action (owner decision R15).
- No runner beyond the seven blocking runners.
- No line evidence (`line-evidence`) and no run results (`run-evidence`).

## Impact

- New spec `docs/specs/test-inventory-v1.md`; generated rows in `docs/specs/REQUIREMENTS.tsv`.
- New verb `corvint inventory refresh` in `corvint` (tests layer, G5).
- A new committed file under `.corvint/`. Owner decision R31 (2026-11-09) says only `.corvint`
  config files force a full run; the inventory is derived data, so the lane states in the canonical
  spec whether a change to it alone forces a full run (see Open).
- `corvint affected discovery` stays a read command; its receipt format becomes the Playwright
  lister output.
- The GitHub Action (change `github-action`) gains a refresh step before planning.
- Changes `e2e-inventory-discovery`, `runner-contract`, `pytest-provider`, `ruby-providers`,
  `jest-provider` and `watch-seven-runners` read this file instead of their own lists.
