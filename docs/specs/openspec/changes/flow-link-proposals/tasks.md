# Tasks: flow-link-proposals

Build in this order. Each task names its requirement IDs and the proof that closes it.

1. Create `docs/specs/flow-link-proposals-v1.md` with FLP-V1-001 to FLP-V1-011 from the spec delta
   as proposed, and regenerate `docs/specs/REQUIREMENTS.tsv`.
   Requirements: FLP-V1-001 to FLP-V1-011. Proof: the spec toolchain check passes.
2. Add the link record to the flow file reader and writer (on the G6 schema).
   Requirements: FLP-V1-006. Proof: round-trip tests of a link record with every field.
3. Add `flows propose --links` and `--all`, writing the private proposals file.
   Requirements: FLP-V1-001, FLP-V1-002. Proof: fixture run shows proposals with cited evidence and
   no file under `.corvint/` changed.
4. Add the versioned confidence formula.
   Requirements: FLP-V1-003. Proof: determinism test and one test per evidence input.
5. Add `flows accept --flow ID --by NAME` with its refusals.
   Requirements: FLP-V1-004, FLP-V1-005. Proof: CLI tests for accept, no terminal, absent and stale
   proposals; a test that no MCP tool or hook path reaches accept.
6. Make selection read accepted links only and keep unassigned tests running.
   Requirements: FLP-V1-007, FLP-V1-008, FLP-V1-011. Proof: plan before and after acceptance; a
   source tag alone does not narrow; a pytest-playwright link selects the whole group.
7. Add stale, disagreeing and dangling link handling and reporting.
   Requirements: FLP-V1-009, FLP-V1-010. Proof: one fixture per case with its reason.
8. Record the trace, the Corvint self-use proposals and acceptances, and the delivery state.
   Requirements: all. Proof: the trace check maps every FLP-V1 ID to a test; the self-use record.
