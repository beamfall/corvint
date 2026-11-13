# Tasks: pytest-provider

Build after changes `runner-contract`, `canonical-test-id`, `test-inventory` and `run-evidence`
(owner decision R38 (2026-11-09) build order). Each task names its requirement IDs and its proof.

- [ ] 1 File the ticket, reserve prefix `PYR-V1` and set `requirementRefs` to the ten IDs. Proof:
   `ticket show` read back.
- [ ] 2 First commit: create `pytest-provider-v1.md`, its `docs/specs/README.md` row and the
   regenerated index. Requirements: PYR-V1-001..010. Proof: spec diff.
- [ ] 3 pytest lister for `corvint inventory refresh`, one instance per rootdir. Requirements:
   PYR-V1-001. Proof: fixture inventory test.
- [ ] 4 Wire the compiled-in pytest runner into `corvint test` at tier `file`; per-test outcomes and
   durations keyed by canonical ID. Requirements: PYR-V1-002, 003, 004. Proof: mixed-plan and
   failing-test fixtures.
- [ ] 5 Fail-closed paths including xdist and plugin deselect. Requirements: PYR-V1-006. Proof: one
   fixture per path.
- [ ] 6 Kind labels; environment record; advisory parity test. Requirements: PYR-V1-005, 007, 008.
   Proof: fixture tests.
- [ ] 7 Kill switch. Requirements: PYR-V1-009. Proof: per-runner switch test.
- [ ] 8 Harness adapter for FIH-V1. Requirements: PYR-V1-010. Proof: harness call test.
- [ ] 9 Tests under "Acceptance evidence"; build-log entry; complete through the Tasks gate. Proof:
   retained receipts, `receipt audit`.

Run path (V1-5443, 2026-11-13): `corvint test` now runs the test files the plan lists for each
selected `python:<module>` unit through the pytest runner contract, one invocation per rootdir
instance, with one outcome and duration per canonical test ID in the receipt (PYR-V1-002 to 004,
RNC-V1-019), and `CORVINT_SELECTION=full:pytest` runs every pytest instance in full (PYR-V1-009,
RNC-V1-008). Blocker for ticking tasks here: `docs/specs/pytest-provider-v1.md` is not on
`origin/main` (`39050561`), so no PYR-V1 ID is in `REQUIREMENTS.tsv` to trace; the harness adapter
(PYR-V1-010 in this folder), xdist and plugin-deselect refusals and kind labels are not built; live
pytest is NOT_RUN (not installed on the lane host).
