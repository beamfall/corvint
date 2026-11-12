# Tasks: docs-install-path

Each task names its requirement IDs and the proof that closes it.

- [ ] 1 Add UPD-V0-920 to 922 to `docs/specs/operator-update-v0.md` and CMS-V0-930 to
   `docs/specs/core-bundled-apps-v0.md` from the spec delta, add the acceptance-log row and
   regenerate `REQUIREMENTS.tsv`. Requirements: all. Proof: `openspec validate docs-install-path`
   and `make spec-requirements` with no further diff.
- [ ] 2 Whole-set apply in `internal/update`: app archives bound by the release `SHA256SUMS`,
   layout and notice checks, version agreement, receipt members, apps switched before `corvint`.
   Requirements: UPD-V0-920. Proof: `TestUPDV0920ApplyInstallsTheWholeCoreSet` and
   `TestUPDV0920SetListsMatchTheReleaseCatalog`.
- [ ] 3 Whole-set rollback and the restore of switched members on error or cancellation.
   Requirements: UPD-V0-921. Proof: `TestUPDV0921RollbackAndInterruptedSetRestore`.
- [ ] 4 Missing app archive refusal, equal-build completion and the corvint-only platform.
   Requirements: UPD-V0-922. Proof: `TestUPDV0922MissingAppRefusesAndEqualBuildCompletesTheSet`.
- [ ] 5 The `not-installed` and `version-mismatch` refusals name the install command.
   Requirements: CMS-V0-930. Proof: `TestCMSV0930NotInstalledNamesTheInstallCommand` and the
   `cmd/corvint` docs shim test.
- [ ] 6 Update `docs/INSTALL.md`, the acceptance tables of both specs, the CMS-V0-002 traceability
   row and the release spec's non-goal line; run the focused tests and the documentation gates.
   Requirements: all. Proof: the commands and exit codes in the handback.
