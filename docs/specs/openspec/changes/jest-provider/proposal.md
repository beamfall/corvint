# Change proposal: Jest under the built-in runner contract

Change name: `jest-provider`
Ticket: none filed yet; to be filed in `.taskman/` before dispatch (not filed by this proposal).
Product requirement served: PROD-TEST-001 and PROD-TEST-004 in `docs/specs/PRODUCT-PLAN-1.0.md` for
the Jest runner, and PROD-TEST-020 for Jest run results.
Owning capability spec: a new spec `docs/specs/jest-runner-v1.md` (prefix `JST-V1`). It does not
exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: JST-V1-001 to JST-V1-010. `JST-` has no entry in
`docs/specs/REQUIREMENTS.tsv` and appears in no other folder under `docs/specs/openspec/changes/`
(checked 2026-11-09).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes.

## Needs first

- `runner-contract`: the compiled-in built-in runner contract (RNC-V1-001) that Jest is added under,
  and the default command rule (RNC-V1-013).
- `canonical-test-id`: Jest test IDs (CTI-V1-004).
- `test-inventory`: Jest discovery writes into the inventory.

Line evidence for Jest is in change `line-evidence` (LEV-V1-005); per-test durations are in change
`per-test-durations`.

## Why

Owner decision R7 (2026-11-09): "add PROD-TEST-020 append-only run result record; Jest gets its own
provider proposal." Owner decision R14 (2026-11-09): provider binaries are retired after their
built-in runner contract lands and are not shipped as separate 1.0 artifacts. G23 (owner answer):
one compiled-in contract, with the open item "Jest path through `corvint test` needs an end-to-end
check and a ticket." This change is that proposal: Jest is a built-in runner of `corvint`, with no
provider binary.

### What Jest support exists at integ 9ec3a29d0

- Detection: the TypeScript affected analysis recognises Jest from a `jest.config.*` file, a
  `jest` or `@jest/core` dependency and the `jest` key in `package.json`
  (`internal/liveverify/affected/typescript/typescript.go`, lines 60, 974, 2073, 2131 and 2147,
  integ 9ec3a29d0), and names Jest units `typescript:jest:<path>` (same file, lines 2031-2033).
- Running through `corvint test`: the provider path admits only Playwright and Vitest and refuses any
  other runner (`internal/sessiontest/declared.go`, lines 270-274, integ 9ec3a29d0), so a Jest
  project runs its declared full command and gets no per-test selection.
- The provider binary `corvint-js-test-provider` has Vitest and Playwright paths and no Jest path
  (`cmd/corvint-js-test-provider/main.go`, integ 9ec3a29d0), and is listed as a release product
  binary (`tools/release-binary/binaries.json`, line 24, integ 9ec3a29d0).
- A generic runner adapter builds a Jest command with `--json --outputFile=jest.json --runInBand`
  (`internal/testrunner/dynamic/build.go`, lines 73-74, integ 9ec3a29d0) and parses the report; its
  assertion record reads title, full name and status, not `ancestorTitles` or `duration`
  (`internal/testrunner/dynamic/parse.go`, lines 185-189, integ 9ec3a29d0). `corvint test` does not
  use this adapter.
- Line collection: the JavaScript line collector lists Jest among its runners and handles Jest's
  focused-file marking (`internal/jslinecov/collect.go`, lines 26 and 394-395, integ 9ec3a29d0).

No end-to-end check of a Jest project through `corvint test` was found at integ 9ec3a29d0.

## What changes

- Jest becomes a built-in runner of `corvint` under the runner contract, with no provider binary
  and no `providers.jest` entry (JST-V1-001).
- Discovery uses `jest --listTests` and the inventory's executing lister (JST-V1-002).
- Selection passes test files and an anchored name pattern; `exact` is true only when every selected
  name is unique in its file (JST-V1-003).
- Results are read from Jest's JSON report into canonical test IDs with outcome and duration; a
  selected test missing from the report is NOT_RUN and the run INCOMPLETE (JST-V1-004, JST-V1-005).
- Default command, instances per root, `full:jest` (JST-V1-006, JST-V1-007).
- Unsupported Jest versions or configurations run the full command with a reason (JST-V1-008).
- An end-to-end check on a Jest fixture through `corvint test`, `corvint affected`, `corvint watch`
  and MCP (JST-V1-009), and a release check that no Jest provider binary ships (JST-V1-010).

## Non-goals

- No `corvint-jest-test-provider` binary and no Jest subcommand in `corvint-js-test-provider`
  (owner decision R14).
- No line evidence collection here (change `line-evidence`).
- No Jest-specific flow links; Jest tests are kind `unit` or `integration` (G13).
- No support for Jest projects that run through a wrapper other than the declared command; those run
  the declared full command.

## Impact

- New spec `docs/specs/jest-runner-v1.md`; generated rows in `docs/specs/REQUIREMENTS.tsv`.
- `internal/sessiontest` (or its successor under the runner contract) gains a Jest adapter.
- Receipts gain Jest outcomes under canonical test IDs; legacy `typescript:jest:<path>` units are read
  at file granularity (CTI-V1-012).
- G4 bar: Jest must narrow at least 60% of PRs with median selected time at most 50%; this change
  supplies the selection and durations that the measurement needs.
- A ticket for the G23 Jest end-to-end check is filed before dispatch.
