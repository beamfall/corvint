# Tasks: `tasks-policy-v15`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order.

## Tasks

- [ ] 1.1 File the ticket in `.taskman/` (milestone `v1-0`) and reserve prefix `TPV-V1` (001 to 008).
      Requirements: all. Proof: `corvint-tasks ticket show`.
- [x] 1.2 First commit: create `docs/specs/tasks-policy-v15.md` from the spec delta, add its row to
      `docs/specs/README.md`, regenerate the index and run `make spec-requirements-check`.
      Requirements: TPV-V1-001 to 008. Proof: check output.
- [ ] 2.1 Wait for the M0 removal to land on `origin/main`. Requirements: TPV-V1-004. Proof: the
      merge commit of the removal.
- [ ] 2.2 Run the reference check over release objects and open tickets. Requirements: TPV-V1-005.
      Proof: empty result retained in the build-log entry.
- [ ] 2.3 Retain the version 14 bytes and SHA-256. Requirements: TPV-V1-007. Proof: the hash in the
      build-log entry.
- [ ] 2.4 Write version 15 and its JSON diff against version 14. Requirements: TPV-V1-001, TPV-V1-002.
      Proof: the diff.
- [ ] 2.5 Run `corvint-tasks policy update` on the store. Requirements: TPV-V1-003. Proof: the store
      reports version 15.
- [ ] 2.6 Open the carry PR with only `.taskman/policy.json`. Requirements: TPV-V1-003. Proof: PR diff
      and byte match with the store.
- [ ] 2.7 Submit and complete one attempt under version 15 without a CEM. Requirements: TPV-V1-006,
      TPV-V1-008. Proof: the receipt.
- [ ] 3.1 Build-log entry `docs/build-log/YYYY-MM-DD-tasks-policy-v15.md` with the evidence; complete
      the ticket through the Tasks gate. Requirements: all. Proof: `corvint-tasks ticket show` and
      `receipt audit`.
