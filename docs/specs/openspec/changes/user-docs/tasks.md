# Tasks: `user-docs`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order. Tasks for
UDC-V1-001 to 007 start only after the owner's Q35 disposition is recorded.

## Tasks

- [ ] 1.1 File the ticket and reserve prefix `UDC-V1` (001 to 012). Requirements: all. Proof:
      `corvint-tasks ticket show` with `requirementRefs` listing UDC-V1-001 to 012.
- [ ] 1.2 First commit: create `docs/specs/user-docs-v1.md` from the spec delta, its README row and
      the regenerated index. Requirements: UDC-V1-001 to 012. Proof: `make spec-requirements-check`
      passes.
- [ ] 2.1 Settled statements: no telemetry, uninstall paragraph, Windows statement. Requirements:
      UDC-V1-008, UDC-V1-009, UDC-V1-011. Proof: diff read-back of `docs/INSTALL.md`.
- [ ] 2.2 `SECURITY.md` in each public repository, one PR per repository. Requirements: UDC-V1-010.
      Proof: `gh api` read of each file.
- [ ] 2.3 Configuration reference and its check. Requirements: UDC-V1-002. Proof: the check fails on
      a fixture constant with no row.
- [ ] 2.4 Error catalogue and its check. Requirements: UDC-V1-004. Proof: the check fails on a
      fixture reason with no row.
- [ ] 2.5 Removed-verb stubs and `docs/UPGRADING.md`. Requirements: UDC-V1-003. Proof: a command
      test per removed verb.
- [ ] 2.6 First-run pages with outputs from the smoke fixture. Requirements: UDC-V1-001. Proof: the
      staleness check.
- [ ] 2.7 `CHANGELOG.md` and the release-workflow refusal. Requirements: UDC-V1-005. Proof: a tag
      without a section is refused in a workflow test.
- [ ] 2.8 Location and compatibility pages per the owner's decision. Requirements: UDC-V1-006,
      UDC-V1-007. Proof: diff read-back.
- [ ] 2.9 Checks wired into `doc-gates`. Requirements: UDC-V1-012. Proof: `doc-gates` output.
- [ ] 3.1 Build-log entry; complete the ticket through the Tasks gate. Requirements: all. Proof:
      `corvint-tasks ticket show` and `receipt audit`.
