# Tasks: `how-rewrite`

Build tasks in order. Each names its requirement IDs and its proof.

1. File the ticket in `.taskman/` (milestone `v1-0`), reserve prefix `HOW-V1` and set
   `requirementRefs` to HOW-V1-001 to HOW-V1-012. Proof: `corvint-tasks ticket show` lists the
   twelve IDs.
2. First commit: create `docs/specs/how-v1.md`, its `docs/specs/README.md` row and the regenerated
   index (HOW-V1-001 to 012). Proof: `make spec-requirements` output lists the twelve IDs.
3. Record golden argument vectors and outputs at the base commit (HOW-V1-002, 003, 004, 005, 006).
   Proof: golden tests pass at the base commit and after the change.
4. Add the missing tests: older corvint-docs and `NOT_INSTALLED` (HOW-V1-007), help text (HOW-V1-008),
   the corvint-alone boundary (HOW-V1-010). Proof: the new tests pass; the smoke run with corvint
   alone reports `NOT_INSTALLED`.
5. Forwarding and import checks (HOW-V1-001, 009). Proof: the forwarding test and
   `TestImportRatchet` pass with the baseline unchanged.
6. MCP tool `corvint.docs_how` and its table row (HOW-V1-011; R42 B10), after change
   `mcp-tool-table` fixes the table. Proof: a tool-list test on both servers, and a test that the
   tool and `how --json` return the same profile for one input.
6a. Gap detail names the stale anchors and the alias (HOW-V1-012; R42 B1). Proof: fixture tests for
    a stale anchor and an aliased source; the `how` goldens recorded once with the new text.
7. Build-log entry; complete the ticket through the Tasks gate. Proof: `corvint-tasks receipt audit`
   passes.

Earlier task list, kept for traceability:

- [ ] 1.1 File the ticket, reserve prefix `HOW-V1` and set `requirementRefs` to the ten IDs.
- [ ] 1.2 First commit: create `how-v1.md`, its `docs/specs/README.md` row and the regenerated index.
- [ ] 2.1 Record golden argument vectors and outputs at the base commit.
- [ ] 2.2 Add the missing tests (older corvint-docs, unknown flow schema, help text).
- [ ] 3.1 Tests under "Acceptance evidence"; build-log entry; complete through the Tasks gate.
