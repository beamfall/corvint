# Tasks: jest-provider

Build in this order. Each task names its requirement IDs and the proof that closes it.

- [x] 1 Create `docs/specs/jest-runner-v1.md` with JST-V1-001 to JST-V1-010 from the spec delta as
   proposed, and regenerate `docs/specs/REQUIREMENTS.tsv`. File the G23 Jest ticket.
   Requirements: JST-V1-001 to JST-V1-010. Proof: the spec toolchain check passes; ticket ID cited.
   V1-5455 filed (the G23 Jest end-to-end ticket); `docs/specs/jest-runner-v1.md` created and
   `make spec-requirements-check` passes.
- [x] 2 Add Jest to the built-in runner set with its default command and instances.
   Requirements: JST-V1-001, JST-V1-006, JST-V1-007. Proof: unit tests for the default command,
   two roots and `full:jest`.
   `TestJSTV1001JestNeedsNoProvider`, `TestJSTV1006DefaultCommandFromTheInstanceRoot`,
   `TestJSTV1007TwoRootsRunInFull`. NOT_PRODUCED: reading `CORVINT_SELECTION=full:jest` on a
   surface waits on the NMS-V1-008 parser (runner-contract task 7); `Verdict.ForceFull` is tested.
- [x] 3 Add Jest discovery to the inventory.
   Requirements: JST-V1-002. Proof: inventory fixture listing Jest tests by canonical ID.
   `TestTIVV1004JestListerTwoRoots`, `TestJSTV1002FailedCollectionLeavesTheInstanceAbsent`.
- [x] 4 Add selection arguments and `exact`.
   Requirements: JST-V1-003. Proof: tests for escaping, anchoring and duplicate names.
   `TestJSTV1003NamePatternIsAnchoredAndEscaped`, `TestJSTV1003DuplicateNamesAreNotExact`.
- [x] 5 Add report reading with outcomes, durations and NOT_RUN.
   Requirements: JST-V1-004, JST-V1-005. Proof: report fixtures, including a missing test and an
   unparsable report.
   `TestJSTV1004ReportMapsNestedDescribesWithDurations`,
   `TestJSTV1005MissingResultsAndReportsAreIncomplete`. NOT_PRODUCED: keeping Jest's exit code
   and the receipt wait on the `corvint test` run path (runner-contract task 5).
- [x] 6 Add the unsupported-setup refusal.
   Requirements: JST-V1-008. Proof: fixtures for an old version and a custom reporter.
   `TestJSTV1008UnsupportedSetupsRunInFull` (Jest 26; `testResultsProcessor` as the custom
   reporter that rewrites the report), `TestJSTV1008SetupFromReader`. Minimum major version 27
   (Open 1).
- [ ] 7 Run the end-to-end fixture through the four consumers and add the release binary check.
   Requirements: JST-V1-009, JST-V1-010. Proof: the end-to-end result; the binary list check.
   Open. Release binary check delivered: `TestJSTV1010NoJestProviderBinary`. In-process two-root
   fixture delivered: `TestJSTV1009TwoRootFixtureChain`. NOT_PRODUCED: the run through the four
   consumers. `corvint test` has no narrowed contract path (runner-contract task 5), `corvint
   watch` refuses a non-Go index, and the host has no Jest install (live run NOT_RUN).
- [ ] 8 Record the trace and the delivery state in the spec.
   Requirements: all. Proof: the trace check maps every JST-V1 ID to a test or measured record.
   Recorded: Traceability and Surfaces in `docs/specs/jest-runner-v1.md`, and `make
   traceability-tests-check` passes. Left unticked only because task 7 is open and ticks go in
   order (OSC-V1-005).

Run path (V1-5443, 2026-11-13): `corvint test` now runs narrowed Jest selections through the
runner contract (RNC-V1-019): one invocation per owning instance, `npx jest --runTestsByPath`
by default, the receipt keeping Jest's exit status and one outcome and duration per canonical
test ID (JST-V1-001, 004, 005); `CORVINT_SELECTION=full:jest` runs every Jest instance in full
(JST-V1-007, RNC-V1-008); the end-to-end fixture covers `corvint affected` and `corvint test`
(RNC-V1-010). Blocker for ticking tasks 1 to 8 here: `docs/specs/jest-runner-v1.md` is not on
`origin/main` (`39050561`), so no JST-V1 ID is in `REQUIREMENTS.tsv` to trace; `corvint watch`
answers Go only and the MCP affected tool runs no tests, so JST-V1-009's four-consumer run stays
NOT_PRODUCED; live Jest is NOT_RUN (not installed on the lane host).
