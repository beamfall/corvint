# Tasks: literal-temp-root

Build the code tasks in order. Task 6 needs a Linux host that runs the Landlock wrapper. Each task
names its requirement IDs and its proof.

- [ ] 1 Set `requirementRefs` on V1-5201 to AFP-V0-107..114. Copy the spec delta into
   `affected-plan-selection-v0.md` and regenerate the index. Requirements: AFP-V0-107..114.
   Proof: `ticket show` read back and the spec diff.
- [ ] 2 Move the AFP-V0-828 temp-root function summary into a package shared by the planner and
   `test-read-scopes-check`, with the checker's tests unchanged. Requirements: AFP-V0-108. Proof:
   the checker's existing tests pass.
- [ ] 3 Add occurrence offsets, the temp-root literal proof, the escape screens and the
   all-occurrences rule in `pathTokens`. Requirements: AFP-V0-107, 108, 109, 110. Proof: unit
   tests for TempDir joins, MkdirTemp, helpers, reassignment, Symlink, exec without `Dir`, calls
   outside the package and production occurrences.
- [ ] 4 Add the bounded-climb joins, climbing literals, repository-root locals and the
   comparison-only exemption in `escapesPackage`. Requirements: AFP-V0-111, 112, 113. Proof: unit
   tests for resolving inside versus above the root, non-read consumers, composite-literal
   elements kept, and the AFP-V0-9450 link case.
- [ ] 5 Report every selection reason through `alsoSelectedBy`. Requirements: AFP-V0-114. Proof:
   a golden plan and a check that the first-witness field is unchanged.
- [ ] 6 Declare read scopes for `cmd/corvint`, `internal/contextindex` and
   `internal/liveverify/affected`, sized from Landlock-observed reads. Requirements: AFP-V0-023,
   AFP-V0-161 (existing). Proof: a full-CI Landlock run of each hub passes with its scope.
- [ ] 7 Re-measure the 30 hub pairs and the `qzx-probe-nothing.md` plan, and record before and
   after in a build-log entry. Requirements: AFP-V0-107..114. Proof: the build-log entry and the
   plan JSON files.
