# Tasks: `compatibility-promise`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order.

## Tasks

- [ ] 1.1 File the ticket in `.taskman/` (milestone `v1-0`) and reserve prefix `CMP-V1` (001 to
      006). Requirements: all. Proof: `corvint-tasks ticket show`.
- [ ] 1.2 First commit: create `docs/specs/compatibility-promise-v1.md` from the spec delta, add
      its row to `docs/specs/README.md`, regenerate the index and run
      `make spec-requirements-check`. Requirements: CMP-V1-001 to 006. Proof: check output.
- [ ] 2.1 List the surfaces of CMP-V1-003 with the contract test that covers each, and read each
      stable format's label from the code. Stop and report to the owner if fewer than half have
      a test. Requirements: CMP-V1-002, CMP-V1-003. Proof: the list, with counts, in the
      build-log entry.
- [ ] 2.2 Write the generator and generate `docs/COMPATIBILITY.md`. Requirements: CMP-V1-001,
      CMP-V1-002, CMP-V1-003, CMP-V1-005. Proof: a second run leaves the page byte-identical.
- [ ] 2.3 Write the stable and preview rules and the label rule above the table. Requirements:
      CMP-V1-004, CMP-V1-005, CMP-V1-006. Proof: the page text.
- [ ] 2.4 Add the check and its tests to the documentation gates. Requirements: CMP-V1-002.
      Proof: tests for a missing contract test, a hand-edited page and an unlisted surface.
- [ ] 3.1 Build-log entry `docs/build-log/YYYY-MM-DD-compatibility-promise.md`; complete the
      ticket through the Tasks gate. Requirements: all. Proof: `corvint-tasks ticket show` and
      `receipt audit`.
