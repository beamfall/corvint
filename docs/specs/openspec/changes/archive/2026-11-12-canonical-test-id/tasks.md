# Tasks: canonical-test-id

Build in this order. Each task names its requirement IDs and the proof that closes it.

- [x] 1 Create `docs/specs/canonical-test-id-v1.md` with CTI-V1-001 to CTI-V1-012 copied from the
   spec delta as `(proposed 2026-11-09; delivery not-started)`, and regenerate
   `docs/specs/REQUIREMENTS.tsv`. Requirements: CTI-V1-001 to CTI-V1-012. Proof: the spec toolchain
   check passes; the new rows appear in the generated file.
- [x] 2 Add the ID package: format, parse, escape and file-level match.
   Requirements: CTI-V1-001, CTI-V1-002, CTI-V1-008. Proof: table-driven unit tests, including a
   round trip of every escape case.
- [x] 3 Add the per-runner name path mapping from each runner's machine report.
   Requirements: CTI-V1-003, CTI-V1-004, CTI-V1-005, CTI-V1-006, CTI-V1-007. Proof: one recorded
   report per runner under `testdata/`, mapped to the expected IDs.
- [x] 4 Add duplicate detection and the unknown-widening rule.
   Requirements: CTI-V1-010, CTI-V1-011. Proof: unit tests for two equal Jest titles, an unknown
   runner token and a malformed ID, each showing the widened selection and its reason.
- [x] 5 Add the `testId` field to `corvint-test-run/0` outcomes and read legacy keys.
   Requirements: CTI-V1-009, CTI-V1-012. Proof: a receipt test that reads an integ 9ec3a29d0
   receipt without `testId` and a new receipt with it; existing receipt tests stay green.
- [x] 6 Use the ID in the plan, watch output and the MCP affected tool.
   Requirements: CTI-V1-009. Proof: one fixture per runner where the plan, the receipt and watch
   print the same ID for the same test.
- [x] 7 Record the trace and the delivery state in the spec.
   Requirements: all. Proof: the trace check maps every CTI-V1 ID to a test.
