# Tasks: `how-rewrite`

Build tasks in order. Each names its requirement IDs and its proof.

1. File the ticket in `.taskman/` (milestone `v1-0`), reserve prefix `HOW-V1` and set
   `requirementRefs` to HOW-V1-001 to HOW-V1-011. Proof: `corvint-tasks ticket show` lists the eleven
   IDs.
2. First commit: create `docs/specs/how-v1.md`, its `docs/specs/README.md` row and the regenerated
   index (HOW-V1-001 to 011). Proof: `make spec-requirements` output lists the eleven IDs.
3. Record golden argument vectors and outputs at the base commit (HOW-V1-002, 003, 004, 005, 006).
   Proof: golden tests pass at the base commit and after the change.
4. Add the missing tests: older corvint-docs and `NOT_INSTALLED` (HOW-V1-007), help text (HOW-V1-008),
   the corvint-alone boundary (HOW-V1-010). Proof: the new tests pass; the smoke run with corvint
   alone reports `NOT_INSTALLED`.
5. Forwarding and import checks (HOW-V1-001, 009). Proof: the forwarding test and
   `TestImportRatchet` pass with the baseline unchanged.
6. MCP row (HOW-V1-011), after change `mcp-tool-table` fixes the table. Proof: a tool-list test on
   both servers.
7. Build-log entry; complete the ticket through the Tasks gate. Proof: `corvint-tasks receipt audit`
   passes.

Earlier task list, kept for traceability:

- [ ] 1.1 File the ticket, reserve prefix `HOW-V1` and set `requirementRefs` to the ten IDs.
- [ ] 1.2 First commit: create `how-v1.md`, its `docs/specs/README.md` row and the regenerated index.
- [ ] 2.1 Record golden argument vectors and outputs at the base commit.
- [ ] 2.2 Add the missing tests (older corvint-docs, unknown flow schema, help text).
- [ ] 3.1 Tests under "Acceptance evidence"; build-log entry; complete through the Tasks gate.
