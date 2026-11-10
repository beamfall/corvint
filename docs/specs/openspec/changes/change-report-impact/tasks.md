# Tasks: `change-report-impact`

Build tasks in order. Each names its requirement IDs and its proof.

- [ ] 1 File the ticket, reserve `CRI-V1` 001..009 and set `requirementRefs`. Proof: `corvint-tasks
      ticket show` lists the nine IDs.
- [ ] 2 First commit: create `change-report-v1.md`, its `docs/specs/README.md` entry and the
      regenerated index. Proof: `make spec-requirements` lists the nine IDs.
- [ ] 3 Report builder on the affected engine and the native flows impact answer (CRI-V1-001, 002,
      003), after change `flows-status-report` lands AFL-V1-007. Proof: the agreement tests against
      `affected` and `flows impact`.
- [ ] 4 Docs-check and Tasks readers (CRI-V1-004, 005). Proof: fixture tests with and without each
      input.
- [ ] 5 One body for CLI, MCP and Action; MCP row after change `mcp-tool-table` fixes the table
      (CRI-V1-006, 009). Proof: the parity and tool-list tests.
- [ ] 6 Remove the old engine and glue (CRI-V1-007). Proof: the removal diff and `go vet ./...`.
- [ ] 7 Preview label and release-gate absence (CRI-V1-008). Proof: the release-checks test.
- [ ] 8 Docs, build-log entry; complete through the Tasks gate. Proof: `corvint-tasks receipt
      audit`.

Earlier task list, kept for traceability:

- 1.1 Reserve `CRI-V1` 001..008; file the ticket; set `requirementRefs`.
- 1.2 First commit: create `change-report-v1.md`; README entry; index.
- 2.1 Report builder on the affected engine and flow readers.
- 2.2 Docs-check and Tasks readers; MCP and Action body.
- 2.3 Remove the old engine and glue.
- 3.1 Fixtures and tests; docs; build-log; Tasks gate.
