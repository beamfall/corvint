# Tasks: `compatibility-promise`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order.

## Tasks

- [x] 1.1 File the ticket in `.taskman/` (milestone `v1-0`) and reserve prefix `CMP-V1` (001 to
      006). Requirements: all. Proof: `corvint-tasks ticket show`. Done: ticket V1-5434.
- [x] 1.2 First commit: create `docs/specs/compatibility-promise-v1.md` from the spec delta, add
      its row to `docs/specs/README.md`, regenerate the index and run
      `make spec-requirements-check`. Requirements: CMP-V1-001 to 006. Proof: check output.
- [x] 2.1 List the surfaces of CMP-V1-003 with the contract test that covers each, and read each
      stable format's label from the code. Stop and report to the owner if fewer than half have
      a test. Requirements: CMP-V1-002, CMP-V1-003. Proof: the list, with counts, in the
      build-log entry.
      Done: 7 of 22 surfaces have a contract test, so the lane stopped before 2.2
      (`docs/build-log/2026-11-11-compatibility-promise.md`). Resumed under the orchestrator's
      decision of 2026-11-11 (owner delegation of 2026-11-02): contract tests added for the
      in-repository surfaces, `not-promised` rows for the rest; 11 stable, 11 not-promised.
- [x] 2.2 Write the generator and generate `docs/COMPATIBILITY.md`. Requirements: CMP-V1-001,
      CMP-V1-002, CMP-V1-003, CMP-V1-005. Proof: a second run leaves the page byte-identical.
      Done: `tools/compatibility-table`, `TestCMPV1002RegenerationIsByteIdentical`.
- [x] 2.3 Write the stable and preview rules and the label rule above the table. Requirements:
      CMP-V1-004, CMP-V1-005, CMP-V1-006. Proof: the page text.
      Done: `docs/COMPATIBILITY.md` sections "Stable, preview and not-promised" and "Labels".
- [x] 2.4 Add the check and its tests to the documentation gates. Requirements: CMP-V1-002.
      Proof: tests for a missing contract test, a hand-edited page and an unlisted surface.
      Done: `make compatibility-check` in `DOC_GATE_TARGETS`; `TestCMPV1002MissingContractTest`,
      `TestCMPV1002HandEditedPage`, `TestCMPV1003UnlistedSurface`.
- [ ] 3.1 Build-log entry `docs/build-log/YYYY-MM-DD-compatibility-promise.md`; complete the
      ticket through the Tasks gate. Requirements: all. Proof: `corvint-tasks ticket show` and
      `receipt audit`.
