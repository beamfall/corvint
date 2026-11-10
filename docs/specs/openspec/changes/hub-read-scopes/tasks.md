# Tasks: hub-read-scopes

Tasks 3 and 6 need a Linux host that runs the Landlock wrapper. Each task names its requirement
IDs and its proof.

- [ ] 1 Set `requirementRefs` on V1-5201 to AFP-V0-107..110. Copy the spec delta into
   `affected-plan-selection-v0.md` and regenerate the index. Requirements: AFP-V0-107..110.
   Proof: `ticket show` read back and the spec diff.
   Status: spec copied and index regenerated; the `requirementRefs` write is left to the
   coordinator (the lane does not write the task store).
- [ ] 2 Stage 1: find every whole-repository test of `internal/contextindex` and
   `internal/liveverify/affected` by running each hub confined to its candidate scope. Known
   ones: `TestGoImportsDifferentialAgainstScanner`, `TestIncrementalSelectionMeetsTheLiveBudget`,
   `TestSelectionOnTheLiveDirtyWorktree`, `TestOwnershipAuditRatchet_AZU_V0_002` and
   `TestRepositoryScriptsAreBoundedSources_V14283`; the list is not complete. Move them all into
   sibling packages and list those packages in `.corvint/unbounded-readers.json` with their
   reason. Requirements: AFP-V0-107. Proof: the moved tests pass, and both hubs' remaining tests
   pass with no new skip.
   Status: done for `internal/liveverify/affected` (40 tests to `liverepo`, no new skip);
   `internal/contextindex` is blocked on unexported internals (see the build log).
- [ ] 3 Stage 1: observe each hub's reads under the Landlock wrapper, declare its read scope and
   remove it from the unbounded set. Requirements: AFP-V0-108, AFP-V0-109. Proof: the confined
   run of each hub passes; `tools/test-read-scopes-check` passes.
   Status: `internal/liveverify/affected` passed confined and declares `[]`;
   `internal/contextindex` stays unbounded (AFP-V0-109).
- [ ] 4 Measure the fixed docs-only diff (base `2913fdc7e`, pull request 178) before and after,
   and record both plans in a build-log entry. Requirements: AFP-V0-110. Proof: the entry and
   the two plan files.
   Status: measured on a fixed docs-only scratch diff instead; pull request 178 not measured.
- [ ] 5 Stage 2: list the `cmd/corvint` tests that read the whole repository, by test name, and
   record whether each can move. Requirements: AFP-V0-107. Proof: the list on V1-5234.
   Status: the blocking files are listed in the build log; the V1-5234 note is not written.
- [x] 6 Stage 2: when task 5 shows every such test can move, repeat tasks 2 to 4 for
   `cmd/corvint`. Otherwise record the blocker and leave the hub unbounded. Requirements:
   AFP-V0-107..110. Proof: the confined run, or the recorded blocker.
