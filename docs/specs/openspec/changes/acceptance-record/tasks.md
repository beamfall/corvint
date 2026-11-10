# Tasks: `acceptance-record`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order.

## Tasks

- [x] 1.1 File the ticket in `.taskman/` (milestone `v1-0`) and reserve prefix `ACR-V1` (001 to
      012). Requirements: all. Proof: `corvint-tasks ticket show`.
- [ ] 1.2 First commit: create `docs/specs/acceptance-record-v1.md` from the spec delta, add its row
      to `docs/specs/README.md`, regenerate the index and run `make spec-requirements-check`.
      Requirements: ACR-V1-001 to 012. Proof: check output.
- [ ] 1.3 Regenerate `docs/specs/REQUIREMENTS.tsv` with `make spec-requirements` on the integ base
      before the record is drafted, and again after every later spec edit in this change. The
      prodspecs copy stopped at AFU-V1-131 and lacked AFU-V1-1651 to 1655, which exist at integ
      9ec3a29d0 (review testing T-m10). Requirements: ACR-V1-006. Proof: `git diff --exit-code
      docs/specs/REQUIREMENTS.tsv` is clean after the run, and the file lists AFU-V1-1651.
- [ ] 2.1 Confirm the R35 precondition (dependency table and anchored citations) is merged.
      Requirements: ACR-V1-001. Proof: the merge commit.
- [x] 2.2 Draft the decision record with the plan SHA, drafted wording list and in-flight statement.
      Requirements: ACR-V1-001, ACR-V1-002, ACR-V1-003. Proof: the draft file.
- [x] 2.3 Write the supersession table with quotes at named commits. Requirements: ACR-V1-004. Proof:
      the table and the stale-quote audit test.
- [x] 2.4 Add the R9 release exception. Requirements: ACR-V1-005. Proof: the record section.
- [x] 2.5 Create `docs/specs/openspec/acceptance-log.tsv` with a row per PROD requirement and O-row.
      Requirements: ACR-V1-006. Proof: row count equals the plan's PROD and O-row count.
- [ ] 2.6 Extend `script/spec-acceptance-audit.sh` with the log check and tests. Requirements:
      ACR-V1-007, ACR-V1-008. Proof: tests for a missing row and a mode mismatch.
- [ ] 2.7 Define the release record format and the workflow read. Requirements: ACR-V1-009,
      ACR-V1-010. Proof: a workflow run that refuses an unsigned record on a test tag.
- [ ] 2.8 Add the allowed-signers file path and the tag verification against the earlier file to
      the release workflow read; add the M6, D6 and invariant rows to the supersession table.
      Requirements: ACR-V1-004, ACR-V1-011. Proof: a test tag signed by an unlisted key is
      refused.
- [x] 3.0 Owner generates the touch-required key on the owner's own device, checks the
      fingerprint by eye and commits the allowed-signers file, or records that no key will be
      used (owner action; no agent creates, holds or uses the key). Requirements: ACR-V1-011.
      Proof: the allowed-signers commit, or the recorded fallback.
- [ ] 3.1 Owner signs the decision record with a signed tag on the acceptance decision commit
      (owner action). Requirements: ACR-V1-001, ACR-V1-011. Proof: the signed record and tag.
- [x] 3.2 After the tag of 3.1, edit `AGENTS.md` product invariants 3, 4, 5 and 7 to the wording of
      ACR-V1-012 and leave invariant 8 unchanged (the invariants carry no other editor; change
      `spec-set-cleanup` task 2.2 leaves them alone). Add the audit checks for the phrase
      "human-owned intent" in invariant 8 and for a foreign invariant edit. Requirements:
      ACR-V1-012. Proof: diff read-back with invariant 8 byte-identical, and audit tests for a
      dropped phrase and a foreign edit.
- [ ] 3.3 Build-log entry `docs/build-log/YYYY-MM-DD-acceptance-record.md`; complete the ticket
      through the Tasks gate. Requirements: all. Proof: `corvint-tasks ticket show` and `receipt
      audit`.
