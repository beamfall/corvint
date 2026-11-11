# Tasks: `flow-schema-version`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order.

## Tasks

- [x] 1.1 File the ticket and reserve prefix `FSV-V1` (001 to 007). Requirements: all. Proof:
      `corvint-tasks ticket show` with `requirementRefs` listing FSV-V1-001 to 007 and PROD-INT-016.
      V1-5410 filed; FSV-V1 reserved.
- [x] 1.2 First commit: create `docs/specs/flow-schema-version-v1.md` from the spec delta, its README
      row and the regenerated index. Requirements: FSV-V1-001 to 007. Proof:
      `make spec-requirements-check` passes.
- [x] 2.1 Version parsing and the supported list in `internal/appflows`. Requirements: FSV-V1-001,
      FSV-V1-002. Proof: unit tests for version 0, an unsupported version and malformed profiles;
      the member-list test fails on a fixture that adds a member under version 0.
- [ ] 2.2 Single reader and the source scan test. Requirements: FSV-V1-003. Proof: the scan test
      fails on a fixture package that opens `.corvint/flows.json` directly.
- [ ] 2.3 `FLOWS_SCHEMA_UNSUPPORTED` in the affected planner. Requirements: FSV-V1-004. Proof: a
      planner test where every E2E test is selected with the reason.
- [ ] 2.4 Refusal in `corvint flows` reports and in the documentation product's flow reader.
      Requirements: FSV-V1-005. Proof: command tests showing a non-zero exit and no partial output.
- [ ] 2.5 Writer and reader agreement. Requirements: FSV-V1-006. Proof: a round-trip test.
- [ ] 2.6 Release smoke job with `corvint` installed alone. Requirements: FSV-V1-007. Proof: the job
      passes on a candidate build and fails when the unsupported-version fixture plans fewer than all
      E2E tests.
- [ ] 3.1 Add `FLOWS_SCHEMA_UNSUPPORTED` to the error catalogue (change `user-docs`). Requirements:
      FSV-V1-004. Proof: the catalogue check passes.
- [ ] 3.2 Build-log entry; complete the ticket through the Tasks gate. Requirements: all. Proof:
      `corvint-tasks ticket show` and `receipt audit`.
