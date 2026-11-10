# Tasks: `flows-unlinked-tests-report`

Build tasks in order. Each names its requirement IDs and its proof.

1. Set `requirementRefs` on V1-5255 to AFU-V1-155 to 159; record 150 to 154 as retired. Proof:
   `corvint-tasks ticket show V1-5255` lists the new IDs.
2. First commit: add the five new definition lines, mark 150 to 154 retired with their integ
   meanings, regenerate the index. Proof: `make spec-requirements` lists 155..159 and keeps 150..154.
3. Native `corvint flows gaps` on the shared readers; remove the `corvint-docs` copy (AFU-V1-155,
   159). Proof: Go tests and the corvint-alone smoke run.
4. Unassigned, stale-link and dangling-link records; ratchet flag (AFU-V1-157, 158). Proof: the
   fixture tests for each case.
5. MCP row after change `mcp-tool-table` fixes the table (AFU-V1-156). Proof: the parity test.
6. Docs (`docs/FLOW-COVERAGE.md`, `docs/MCP-SERVER.md`, help), build-log; complete V1-5255 through
   the Tasks gate. Proof: `corvint-tasks receipt audit` passes.

Earlier task list, kept for traceability (task 1.1 is superseded by owner decision R11):

- [ ] 1.1 Keep the `AFU-V1` 150..154 reservation; set `requirementRefs`.
- [ ] 1.2 First commit: rewrite the five definition lines and regenerate the index.
- [ ] 2.1 Native `corvint flows gaps` on the shared readers; remove the `corvint-docs` copy.
- [ ] 2.2 Unassigned, stale-link and dangling-link records; ratchet flag.
- [ ] 2.3 MCP parity on `corvint mcp`.
- [ ] 3.1 Fixtures, tests and the corvint-alone smoke test.
- [ ] 3.2 Docs (`docs/FLOW-COVERAGE.md`, `docs/MCP-SERVER.md`, help); build-log; Tasks gate.
