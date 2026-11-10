# Tasks: hub-read-scopes

Tasks 3 and 6 need a Linux host that runs the Landlock wrapper. Each task names its requirement
IDs and its proof.

- [ ] 1 Set `requirementRefs` on V1-5201 to AFP-V0-107..110. Copy the spec delta into
   `affected-plan-selection-v0.md` and regenerate the index. Requirements: AFP-V0-107..110.
   Proof: `ticket show` read back and the spec diff.
- [ ] 2 Stage 1: move `TestGoImportsDifferentialAgainstScanner` and the two live-repository tests
   of `internal/liveverify/affected` into sibling packages, and list those packages in
   `.corvint/unbounded-readers.json` with their reason. Requirements: AFP-V0-107. Proof: the moved
   tests pass, and both hubs' remaining tests pass.
- [ ] 3 Stage 1: observe each hub's reads under the Landlock wrapper, declare its read scope and
   remove it from the unbounded set. Requirements: AFP-V0-108, AFP-V0-109. Proof: the confined
   run of each hub passes; `tools/test-read-scopes-check` passes.
- [ ] 4 Measure the fixed docs-only diff (base `2913fdc7e`, pull request 178) before and after,
   and record both plans in a build-log entry. Requirements: AFP-V0-110. Proof: the entry and
   the two plan files.
- [ ] 5 Stage 2: list the `cmd/corvint` tests that read the whole repository, by test name, and
   record whether each can move. Requirements: AFP-V0-107. Proof: the list on V1-5234.
- [ ] 6 Stage 2: when task 5 shows every such test can move, repeat tasks 2 to 4 for
   `cmd/corvint`. Otherwise record the blocker and leave the hub unbounded. Requirements:
   AFP-V0-107..110. Proof: the confined run, or the recorded blocker.
