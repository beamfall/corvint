# 2026-11-12: Jest runner on the runner contract (V1-5455)

Change `jest-provider`: tasks 1 to 6 are delivered. Task 7 is open, and so is task 8: its record
is written, but ticks go in order (OSC-V1-005). Requirements JST-V1-001 to JST-V1-010 are in
`docs/specs/jest-runner-v1.md`. Each requirement's Delivery line there says what is built and what
waits. The change builds on `runner-contract` (V1-5416),
`canonical-test-id` (V1-5398) and `test-inventory` (V1-5406), and adds no executable (owner
decision R14).

## What changed

- `internal/testinventory`: the Jest lister also runs Jest's own collection,
  `--json --testNamePattern=^(?!)`. Its empty negative lookahead matches no full name, so Jest loads
  every file and reports each test skipped, and the lister names each test by canonical ID. The
  lister moves to version 2, so version-1 Jest records are stale and are listed again.
- `internal/testrunner/contract`: the Jest command adds one anchored `-t` pattern for selected
  tests. `Request.Listed` carries the inventory's IDs so that `exact` can be proven, and
  `Request.Jest` carries the setup that `JestSetup.Unsupported` judges. The Playwright filter
  escaper is now `regexpLiteral`, shared with Jest.
- Acceptance-log row `jest-provider` (kind `change`, mode `auto`): the change implements the
  accepted PROD-TEST-001, PROD-TEST-004 and PROD-TEST-020 (decision 0596, G14).

## Decisions

1. **Minimum Jest version (design Open 1): 27.** Jest 27 made jest-circus the default runner. Its
   `--json` assertions carry `ancestorTitles` and `duration`, and it skips a block's hooks when all
   of the block's tests are skipped, which the collection run needs. The owner may raise the
   minimum. Lowering it needs a measured older-Jest report.
2. **"Reporter that prevents the JSON report" is `testResultsProcessor`.** That option rewrites
   the aggregated results before `--json` writes them. Jest's `reporters` do not change the
   `--json` output, so they are not refused. A `testRunner` other than jest-circus or jest-jasmine2,
   a `runner` other than `jest-runner`, and a project named by path are refused.
3. **Case-insensitive duplicates.** Jest compiles `-t` with the `i` flag and matches it against
   the titles joined by spaces. So `exact` compares those joined names ignoring case, and
   `a b > c` collides with `a > b c`.
4. **A JavaScript config is read as text.** Values set through a preset, a spread or a computed
   object are not seen. The backstop is JST-V1-005: a report without a selected test leaves it
   NOT_RUN and the run incomplete.
5. **A nil setup is unsupported.** A caller that did not read the setup gets the full command, so
   leaving the setup out never narrows by default.

## Not produced

- The `corvint test` narrowed run, receipt and Jest exit code (runner-contract task 5).
- `CORVINT_SELECTION=full:jest` on any surface (NMS-V1-008, runner-contract task 7).
- The four-consumer end-to-end run (JST-V1-009). `corvint watch` refuses a non-Go index.
- A live Jest run: this host has no Jest install, so every Jest output in the tests is
  hand-written in the documented `--json` shape (NOT_VERIFIED against a real Jest).
