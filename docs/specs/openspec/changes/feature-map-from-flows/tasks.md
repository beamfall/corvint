# Tasks: `feature-map-from-flows`

Build tasks in order. Each names its requirement IDs and its proof.

1. File the ticket, reserve `FMP-V1` 001..007 and set `requirementRefs`. Proof: `corvint-tasks
   ticket show` lists the seven IDs.
2. First commit: create `feature-map-v1.md`, its `docs/specs/README.md` entry and the regenerated
   index. Proof: `make spec-requirements` lists the seven IDs.
3. Map generator in `docs flows generate` (FMP-V1-001, 002, 004, 005, 006). Proof: fixture tests for
   two flows, an untested flow, a pending proposal, no flow file and an unknown profile.
4. Remove `feature`, `features`, `overview` and the heuristics; add the notices (FMP-V1-003). Proof:
   the notice test, the removal diff and `go vet ./...`.
5. MCP row, if offered, after change `mcp-tool-table` fixes the table (FMP-V1-007). Proof: a
   tool-list test on both servers.
6. Docs, build-log entry; complete through the Tasks gate. Proof: `corvint-tasks receipt audit`.

Earlier task list, kept for traceability:

- [ ] 1.1 Reserve `FMP-V1` 001..006; file the ticket; set `requirementRefs`.
- [ ] 1.2 First commit: create the spec; README entry; index.
- [ ] 2.1 Map generator in `corvint-docs`.
- [ ] 2.2 Remove `feature`, `features`, `overview` and the heuristics; add the notices.
- [ ] 3.1 Fixtures and tests; docs; build-log; Tasks gate.
