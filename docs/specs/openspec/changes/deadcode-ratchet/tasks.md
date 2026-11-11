# Tasks: deadcode-ratchet

Each task names its requirement IDs and its proof. Tasks 1 to 5 are the first commit (V1-4801).
Tasks 6 to 8 are the second commit (V1-4827).

- [ ] 1 Create `docs/specs/deadcode-ratchet-v0.md` from the spec delta as proposed definition
   lines, add its row to `docs/specs/README.md`, and regenerate `REQUIREMENTS.tsv`. Requirements:
   DCR-V0-001 to 008. Proof: spec diff, `make spec-requirements` with no further diff.
- [ ] 2 `make deadcode`: install the pinned analyzer for the host into a scratch directory, run
   it for the six configurations over every module package including dot directories, and remove
   the scratch directory on exit. Requirements: DCR-V0-001, DCR-V0-002. Proof: the target's
   output for one full run with elapsed time; `git diff --stat go.mod go.sum` is empty.
- [ ] 3 `tools/deadcode-ratchet`: identities, intersection, strict baseline decode and the
   verdict. Requirements: DCR-V0-003, DCR-V0-004, DCR-V0-005. Proof: the hermetic tests of task 4.
- [ ] 4 Hermetic tests on fixture JSON for the six cases the design lists. Requirements:
   DCR-V0-006. Proof: `script/heavy.sh env GOTOOLCHAIN=local go test -count=1
   ./tools/deadcode-ratchet/` passes and takes under one second.
- [ ] 5 Commit the measured baseline and the count assertion; show the ratchet failing on a
   scratch commit that adds one unreachable function. Requirements: DCR-V0-004, DCR-V0-005.
   Proof: exit codes of both `make deadcode` runs in the build-log entry.
- [ ] 6 Delete each listed function that no exclusion keeps; give every kept entry a reason.
   Requirements: DCR-V0-007. Proof: the removed-declaration check against the regenerated list.
- [ ] 7 Build and vet every configuration and custom build tag, vet `interop/cem01-go`, and run
   the tests of each package that lost a function. Requirements: DCR-V0-008. Proof: commands and
   exit codes in the handback.
- [ ] 8 Rerun `make deadcode`, update the baseline and the asserted count, run the doc gates and
   write `docs/build-log/2026-11-11-deadcode-ratchet.md`. Requirements: DCR-V0-005, DCR-V0-007.
   Proof: exit 0, the residual count, and each residual reason.
