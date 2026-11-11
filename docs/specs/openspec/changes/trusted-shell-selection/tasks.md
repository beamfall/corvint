# Tasks: `trusted-shell-selection`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order. Lane ghci2
(2026-11-11) wrote this change and no code; every task is open.

## Tasks

- [ ] 1.1 Owner reads `design.md`, answers its two open questions, and accepts or amends the
      intent. Requirements: CIN-V0-8724 to CIN-V0-8727. Proof: the owner's recorded answer.
- [ ] 1.2 Code pull request, first commit: copy the four requirements into
      `docs/specs/ci-narrowing-v0.md` (proposed until accepted) with traceability rows, regenerate
      `docs/specs/REQUIREMENTS.tsv`; add the new test file to `guardUnmappedInputs` in
      `tools/ci-forward-row/narrowing_reasons_test.go`. Requirements: all. Proof:
      `make spec-requirements` and `make requirement-ids-free PREFIX=CIN-V0 RANGE=8724..8727`.
- [ ] 2.1 `.github` pull request: rewrite the `shell-test-selected` step (event-base worktree build,
      stamped planner with `--accept-stale-binary`, union selection, FULL on removal or on a base
      that does not build, summary lines). Requirements: CIN-V0-8724, CIN-V0-8725. Proof: task
      2.3 test.
- [ ] 2.2 Same pull request: the `doc-gates` base-list step. Requirements: CIN-V0-8726. Proof: task
      2.3 test.
- [ ] 2.3 Same pull request: `.github/forward-rows/trusted-selection_test.py`, registered in
      `go-static`, with the fixture repository of `design.md` D5 and the control run against the
      `b55635262` step body. Requirements: CIN-V0-8727. Proof:
      `python3 -B .github/forward-rows/trusted-selection_test.py` passes, and fails on the old step.
- [ ] 2.4 Run `tools/ci-reuse-plan` (`TestCINV02320*`) and `tools/ci-forward-row` whole after both
      pull requests; the CIN-V0-2320 bindings that pin `make -s shell-test-affected` move to the new
      step text in the code pull request in the form that holds before and after. Requirements:
      CIN-V0-2320 (modified). Proof: package runs through `script/heavy.sh`.
- [ ] 3.1 One hosted pull-request run whose `shell-test-selected` summary names the event-base
      selector, and one that removes a step and shows the FULL warning. Requirements: CIN-V0-8724,
      CIN-V0-8725. Proof: run URLs in the build log.
- [ ] 3.2 Build-log entry; complete V1-5097 through the Tasks gate. Requirements: all. Proof:
      `corvint-tasks ticket show V1-5097` and `receipt audit`.
