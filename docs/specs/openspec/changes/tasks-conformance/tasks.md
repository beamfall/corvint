# Tasks: `tasks-conformance`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order. Work happens in
the corvint-tasks repository unless a task says otherwise.

## Tasks

- [ ] 1.1 File the ticket in the corvint-tasks store, reserve prefix `TCF-V1` (001 to 010), and link
      it from a `v1-0` ticket in this repository. Requirements: all. Proof: `corvint-tasks ticket
      show` in both stores.
- [ ] 1.2 First commit in corvint-tasks: create `docs/specs/tasks-conformance-v1.md` from the spec
      delta. Requirements: TCF-V1-001 to 010. Proof: the Tasks repository's spec index check.
- [ ] 2.1 Schema files and the shared fixture. Requirements: TCF-V1-001. Proof: fixture tests for
      valid and invalid documents.
- [ ] 2.2 Supported-command list and the experimental marking in `help`. Requirements: TCF-V1-002,
      TCF-V1-003. Proof: a test that every listed command exists and `help` marks the others.
- [ ] 2.3 Requirement trace tests for PROD-TASK-001 to 009. Requirements: TCF-V1-004. Proof: trace
      table with test IDs.
- [ ] 2.4 Command schema checks. Requirements: TCF-V1-005. Proof: one output check per listed command.
- [ ] 2.5 Concurrency test with at least 8 processes. Requirements: TCF-V1-006. Proof: test log with
      zero double claims.
- [ ] 2.6 Kill -9 recovery test. Requirements: TCF-V1-007. Proof: ID and hash diff after injected
      kills.
- [ ] 2.7 Record producer and `taskman-conformance/1` format. Requirements: TCF-V1-008. Proof: a
      record produced on a release candidate commit.
- [ ] 2.8 Release workflow for three targets. Requirements: TCF-V1-010. Proof: candidate tag
      artifacts with checksums and attestation.
- [ ] 2.9 In this repository: vendor the fixture and command list with SHA-256 pins (TIN-V1-004,
      TIN-V1-013). Requirements: TCF-V1-001, TCF-V1-002. Proof: the pin check.
- [ ] 2.10 Trial start in Corvint and `beamfall/core` on the frozen RC binary, on 2026-12-08
      (C22), after the `beamfall/core` store is initialized (TIN-V1-014, C23). Requirements:
      TCF-V1-009. Proof: start record with ID and hash list.
- [ ] 3.1 Trial end records and the conformance record at the Tasks tag. Requirements: TCF-V1-008,
      TCF-V1-009. Proof: records read by the Corvint RC check.
- [ ] 3.2 Build-log entries in both repositories; complete the tickets through the Tasks gate.
      Requirements: all. Proof: `corvint-tasks ticket show` and `receipt audit`.
