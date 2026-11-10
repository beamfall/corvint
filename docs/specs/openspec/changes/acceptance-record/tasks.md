# Tasks: `acceptance-record`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order.

## Tasks

- [ ] 1.1 File the ticket in `.taskman/` (milestone `v1-0`) and reserve prefix `ACR-V1` (001 to 010).
      Requirements: all. Proof: `corvint-tasks ticket show`.
- [ ] 1.2 First commit: create `docs/specs/acceptance-record-v1.md` from the spec delta, add its row
      to `docs/specs/README.md`, regenerate the index and run `make spec-requirements-check`.
      Requirements: ACR-V1-001 to 010. Proof: check output.
- [ ] 2.1 Confirm the R35 precondition (dependency table and anchored citations) is merged.
      Requirements: ACR-V1-001. Proof: the merge commit.
- [ ] 2.2 Draft the decision record with the plan SHA, drafted wording list and in-flight statement.
      Requirements: ACR-V1-001, ACR-V1-002, ACR-V1-003. Proof: the draft file.
- [ ] 2.3 Write the supersession table with quotes at named commits. Requirements: ACR-V1-004. Proof:
      the table and the stale-quote audit test.
- [ ] 2.4 Add the R9 release exception. Requirements: ACR-V1-005. Proof: the record section.
- [ ] 2.5 Create `docs/specs/openspec/acceptance-log.tsv` with a row per PROD requirement and O-row.
      Requirements: ACR-V1-006. Proof: row count equals the plan's PROD and O-row count.
- [ ] 2.6 Extend `script/spec-acceptance-audit.sh` with the log check and tests. Requirements:
      ACR-V1-007, ACR-V1-008. Proof: tests for a missing row and a mode mismatch.
- [ ] 2.7 Define the release record format and the workflow read. Requirements: ACR-V1-009,
      ACR-V1-010. Proof: a workflow run that refuses an unsigned record on a test tag.
- [ ] 3.1 Owner signs the decision record (owner action). Requirements: ACR-V1-001. Proof: the signed
      record.
- [ ] 3.2 Build-log entry `docs/build-log/YYYY-MM-DD-acceptance-record.md`; complete the ticket
      through the Tasks gate. Requirements: all. Proof: `corvint-tasks ticket show` and `receipt
      audit`.
