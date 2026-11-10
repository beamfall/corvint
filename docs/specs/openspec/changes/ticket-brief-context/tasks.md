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

Earlier task list, kept for traceability:

- 1.1 Reserve `TBC-V1` 001..007; file the ticket; set `requirementRefs`.
- 1.2 First commit: create the spec; README entry; index.
- 2.1 Brief builder; MCP form.
- 2.2 Remove the ranking files and the ranked path.
- 3.1 Freeze the task set; run both arms; record the result; keep or remove.
- 3.2 Docs; build-log; Tasks gate.
