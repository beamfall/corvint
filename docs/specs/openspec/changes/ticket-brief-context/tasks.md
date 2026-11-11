# Tasks: `ticket-brief-context`

## Tasks

Build tasks in order. Each names its requirement IDs and its proof.

- [ ] 1 Reserve `TBC-V1` 001 to 008; file the ticket; set `requirementRefs`. Proof: `corvint-tasks
      ticket show`.
- [ ] 2 First commit: create the spec, its README entry and the index. Proof: `make
      spec-requirements`.
- [ ] 3 Brief builder reading `corvint-tasks ticket show --json` (TBC-V1-001 to 004, 007). Proof:
      fixture tickets for each item and each missing-link case.
- [ ] 4 Remove the ranking files and the ranked path (TBC-V1-005). Proof: the build-after-removal
      scenario.
- [ ] 5 MCP row after change `mcp-tool-table` (TBC-V1-008). Proof: the tool-list scenario.
- [ ] 6 Freeze and commit the task list and hash before the 1.0 release candidate; record the
      measurement as NOT_RUN at the candidate (TBC-V1-006). Proof: the committed list and the
      NOT_RUN record.
- [ ] 7 Docs stating non-blocking and no benefit claim; build-log; Tasks gate. Proof: `corvint-tasks
      receipt audit` passes.
- [ ] 8 In 1.0.x: run both arms on the owner's machines, record the result, keep or remove
      (TBC-V1-006).

Tasks of the amendment of 2026-11-10 (the owner accepted PROD-INT-032 into 1.0 on 2026-11-10).
Task 9 runs first and needs only the frozen task list of task 6; it needs neither the export nor
the builder. Tasks 10 to 12 run after task 3 and only if task 9 meets the bar the owner sets.

- [ ] 9 Link-coverage check of the `tests` and `requirements` sections on the frozen task set, by a
      scratch script over existing commands and the sealed records, with no agent run
      (TBC-V1-013). Proof: the build-log record and the owner's bar.
- [ ] 10 One-time export of the sealed requirement links to `.corvint/requirement-links.json`
      (TBC-V1-012). Proof: the two-run byte-identity scenario; input commit and output hash in the
      build log.
- [ ] 11 Path brief builder with six sections; the ticket form is not changed (TBC-V1-009 to
      TBC-V1-011). Proof: fixture repositories for each section and each absent source, one with
      no Corvint files.
- [ ] 12 Verify whether a completed ticket's commits can be read from its `progress` and
      `completion` fields; file a Tasks ticket if they cannot. Proof: `corvint-tasks ticket show`
      on a completed fixture ticket.
- [ ] 13 Pilot on the first 8 tasks with arms A, B and C before the release candidate, or record
      NOT_RUN (TBC-V1-014). Proof: the pilot record.

Earlier task list, kept for traceability:

- 1.1 Reserve `TBC-V1` 001..007; file the ticket; set `requirementRefs`.
- 1.2 First commit: create the spec; README entry; index.
- 2.1 Brief builder; MCP form.
- 2.2 Remove the ranking files and the ranked path.
- 3.1 Freeze the task set; run both arms; record the result; keep or remove.
- 3.2 Docs; build-log; Tasks gate.
