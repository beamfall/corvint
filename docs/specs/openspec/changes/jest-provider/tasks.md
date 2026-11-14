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
   `TestJSTV1007TwoRootsRunInFull`; `corvint test` reads `CORVINT_SELECTION=full:jest` through the
   one kill-switch parser and runs every instance in full: `TestJSTV1007FullJestRunsEveryInstance`
   (`cmd/corvint`, V1-5455, 2026-11-13).
- [x] 3 Add Jest discovery to the inventory.
   Requirements: JST-V1-002. Proof: inventory fixture listing Jest tests by canonical ID.
   `TestTIVV1004JestListerTwoRoots`, `TestJSTV1002FailedCollectionLeavesTheInstanceAbsent`.
- [x] 4 Add selection arguments and `exact`.
   Requirements: JST-V1-003. Proof: tests for escaping, anchoring and duplicate names.
   `TestJSTV1003NamePatternIsAnchoredAndEscaped`, `TestJSTV1003DuplicateNamesAreNotExact`. After the
   independent review a pattern is passed only when it is proven lossless (JST-V1-020,
   `TestJSTV1003RootDescribeBlockWidensToTheFile`), and an over-long one widens to the files
   (RNC-V1-016, `TestRNCV1016JestArgvLimitWidens`).
- [x] 5 Add report reading with outcomes, durations and NOT_RUN.
   Requirements: JST-V1-004, JST-V1-005. Proof: report fixtures, including a missing test and an
   unparsable report.
   `TestJSTV1004ReportMapsNestedDescribesWithDurations`,
   `TestJSTV1005MissingResultsAndReportsAreIncomplete`; the receipt keeps Jest's exit status and
   one outcome and duration per canonical test ID: `TestRNCV1019ContractRunRecordsPerTestResults`,
   `TestRNCV1019ContractRunFailsClosed` (`internal/sessiontest`),
   `TestJSTV1009TwoRootFixtureThroughTheConsumers` (`cmd/corvint`).
- [x] 6 Add the unsupported-setup refusal.
   Requirements: JST-V1-008. Proof: fixtures for an old version and a custom reporter.
   `TestJSTV1008UnsupportedSetupsRunInFull` (Jest 26; `testResultsProcessor` as the custom
   reporter that rewrites the report; a config that loads or spreads another module),
   `TestJSTV1008SetupFromReader`. Minimum major version 27 (Open 1). `corvint test` reads the setup
   before it narrows (`TestRNCV1019ContractUnitsPlanTheSelectedFiles`).
- [ ] 7 Run the end-to-end fixture through the four consumers and add the release binary check.
   Requirements: JST-V1-009, JST-V1-010. Proof: the end-to-end result; the binary list check.
   Open on one consumer. Release binary check delivered: `TestJSTV1010NoJestProviderBinary`. The
   two-root fixture runs through `corvint affected`, the MCP affected tool and `corvint test`, every
   receipt test matching the inventory: `TestJSTV1009TwoRootFixtureThroughTheConsumers`
   (`cmd/corvint`) and the in-process `TestJSTV1009TwoRootFixtureChain`. NOT_PRODUCED: the
   `corvint watch` run, which answers only Go selections until change `watch-seven-runners`
   delivers its multi-runner loop; the host has no Jest install (live run NOT_RUN).
- [ ] 8 Record the trace and the delivery state in the spec.
   Requirements: all. Proof: the trace check maps every JST-V1 ID to a test or measured record.
   Recorded: Traceability and Surfaces in `docs/specs/jest-runner-v1.md`, and `make
   traceability-tests-check` passes. Left unticked only because task 7 is open and ticks go in
   order (OSC-V1-005).

