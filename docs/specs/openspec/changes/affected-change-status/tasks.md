# Tasks: affected-change-status

Each task names its requirement IDs and its proof.

- [x] 1 Copy the spec delta into `affected-plan-selection-v0.md` as proposed definition lines;
   amend the AFP-V0-9133 and AFP-V0-9134 text where the delta says so; regenerate
   `REQUIREMENTS.tsv`. Requirements: AFP-V0-9960 to 9964. Proof: spec diff, `make
   spec-requirements` with no further diff.
- [ ] 2 Change status from the base tree in `corvint affected`, passed to the planner; absent
   status is `unknown`. Requirements: AFP-V0-9960. Proof: a table test over added, deleted,
   renamed, type-changed, modified, untracked and no-base inputs.
- [ ] 3 Name-listing reads in the shell plugin for the closed command list. Requirements:
   AFP-V0-9961. Proof: one test row per admitted form and per refused form (`ls -l`, `find
   -newer`, `find -exec`, `du`, recursive `grep`, a computed operand, a helper).
- [ ] 4 Selection by status for name-listing reads. Requirements: AFP-V0-9962. Proof: for `ls
   script | wc -l`, selected on an added and on a deleted file, not selected on an edit, selected
   on `unknown` and on `type-changed`; a unit with a content read of the same directory is
   selected on an edit.
- [ ] 5 The same for name-only unbounded walkers. Requirements: AFP-V0-9963. Proof: `find .
   -name '*.md'` not selected on an edit and selected on an addition; `grep -rn .` selected on
   an edit; a unit with both is selected on an edit.
- [ ] 6 Cache field and version. Requirements: AFP-V0-9964. Proof: a round-trip test, and a test
   that an older-version cache is rebuilt.
- [ ] 7 Mutation evidence for `listingCanChange` and for the closed list. Proof: each named
   mutant killed by a named test, listed in the build log.
- [ ] 8 Measurements: `selection-baseline` (misses 0, narrow count not below the floor) and the
   pr-39 replay (no `FALLBACK`). Proof: the table in a build-log entry, with the commands.
