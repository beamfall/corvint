# Tasks: `review-evidence-check`

## Tasks

Build tasks in order. Each names its requirement IDs and its proof.

1. Reserve `RVE-V1` 001 to 010; file the ticket; set `requirementRefs`. Proof: `corvint-tasks ticket
   show`.
2. First commit: create the spec, its README entry and the index. Proof: `make spec-requirements`.
3. Check builder: plan recompute, receipt reader (PROD-TEST-020 record when delivered), flows,
   inventory (RVE-V1-001 to 004). Proof: one fixture per finding kind.
4. Docs-check and ticket inputs; advisory docs findings; verdict and exit codes (RVE-V1-005 to 007).
   Proof: the testing-only `CLEAN` scenario and each `INCOMPLETE` cause.
5. Consumer contract: advisory by default with opt-in (RVE-V1-008, 009). Proof: the two Tasks gate
   scenarios, with changes `tasks-integration` and `github-action`.
6. MCP row after change `mcp-tool-table` (RVE-V1-010). Proof: the tool-list scenario.
7. Remove the guidance-based review; build-log; complete through the Tasks gate. Proof:
   `corvint-tasks receipt audit` passes.

Earlier task list, kept for traceability:


- [ ] 1.1 Reserve `RVE-V1` 001..009; file the ticket; set `requirementRefs`.
- [ ] 1.2 First commit: create the spec; README entry; index.
- [ ] 2.1 Check builder: plan recompute, receipt reader, flows, inventory.
- [ ] 2.2 Docs-check and ticket inputs; verdict and exit codes.
- [ ] 2.3 Remove the guidance-based review.
- [ ] 3.1 Fixtures and tests; docs; build-log; Tasks gate.
