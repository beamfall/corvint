# Design: e2e-inventory-discovery

Status: proposed (2026-11-09). Requirement text lives in `specs/e2e-inventory-discovery/spec.md`.

## Decisions

1. One committed inventory for all seven runners, written by `corvint inventory refresh` (owner
   decision R10 (2026-11-09)). This change reads the Playwright entries; change `test-inventory`
   owns the file. The Playwright lister is `executing` (it loads the project's config).
2. Fresh IDs AFU-V1-146 to 149; AFU-V1-140 to 145 are retired with their integ meanings recorded
   (owner decision R11 (2026-11-09); review T-M15). Mapping: old 140 and 144 become 146; old 141
   becomes 147; old 142 and 143 become 148; old 145 becomes 149.
3. Entries are keyed by the canonical test ID, RNC-V1-012 (owner decision R11 (2026-11-09)).
4. Staleness is per file for test files and whole-inventory for config, lockfile and profile
   (review T-M2). A changed or added test file does not make every E2E test run; its own tests run
   as `unassigned`. This is a narrower staleness than "any test file changed", and it keeps owner
   decision R10's rule that stale inventory runs all tests for the inputs that can change every
   test's identity.
5. Absent inventory runs the declared full command with reason `inventory-absent` (review T-M3;
   RNC-V1-013 supplies the command).
6. Write verb `corvint inventory refresh`; `corvint watch` refreshes nothing; the Action refreshes
   before planning (review T-M4; owner decision R10 (2026-11-09)).
7. A provider file adds tests and never removes them (settled without the owner, S5).

## Failure modes

- Inventory absent: the Playwright runner's declared full command runs with reason
  `inventory-absent`; if no full command is available, E2E scope is `UNKNOWN` (APK-V0-009,
  `e2e-list-unavailable`).
- Inventory stale for the whole runner (config, lockfile or inventory profile binding differs):
  widen to every E2E test with reason `inventory-stale`.
- Test file added or changed since the inventory was written: the tests in that file are selected
  and counted `unassigned`; tests in other files keep their entries.
- Test file deleted: its entries are dropped from selection; a flow link naming them is dangling
  (APK-V0-004 failure modes).
- Inventory malformed, over a bound, or with an unknown profile: typed refusal, exit 2.
- A listing naming a file absent from the tree: the inventory is stale.
- Provider file and inventory disagree: the union is selected and both differences are reported.

## Acceptance evidence

- A fixture with a Playwright config, three tests, accepted links for two, and a committed
  inventory. Go tests named for V1-5252 for each requirement and failure mode.
- A test that the plan read starts no child process for the fixture config.
- A test that a provider omitting a listed test does not shrink the selection (reverses the current
  `ReconcileProvider` behaviour).
- A test that editing one test file selects that file's tests as `unassigned` and leaves the rest
  narrowed, and that editing `playwright.config.ts` selects every E2E test with `inventory-stale`.
- NOT_RUN at the time of writing: every test of the current `oneplan` code.

## Rollback

Delete the inventory file: the plan runs the full E2E command (fail closed). To withdraw the change,
revert its commits; the IDs are retired, not reused. AFU-V1-140 to 145 stay retired after a revert.

## Open questions for the owner

None. (Answered: the refresh verb's home is `corvint` in the tests layer, as
`corvint inventory refresh`, owner decision R10 (2026-11-09).)
