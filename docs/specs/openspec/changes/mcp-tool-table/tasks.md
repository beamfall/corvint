# Tasks: `mcp-tool-table`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order.

## Tasks

- [ ] 1.1 File the ticket and reserve prefix `MTT-V1` (001 to 010). Requirements: all. Proof:
      `corvint-tasks ticket show` with `requirementRefs` listing MTT-V1-001 to 010.
- [ ] 1.2 First commit: create `docs/specs/mcp-tool-table-v1.md` from the spec delta, its README row
      and the regenerated index, naming the MMR-V0 IDs it supersedes. Requirements: MTT-V1-001 to
      010. Proof: `make spec-requirements-check` passes.
- [ ] 2.1 Table rows with schema, server, output profile, errors, availability and CLI equivalent.
      Requirements: MTT-V1-001, MTT-V1-006. Proof: table unit test (unique names, every column set).
- [ ] 2.2 Generated `docs/MCP-SERVER.md` and its staleness check. Requirements: MTT-V1-002. Proof:
      the check fails on a hand edit.
- [ ] 2.3 Drift test against registered tools on both servers. Requirements: MTT-V1-003. Proof: the
      test fails when a tool is added without a row.
- [ ] 2.4 `corvint-docs mcp` subcommand serving the docs tools. Requirements: MTT-V1-004. Proof:
      server tool-list test.
- [ ] 2.5 Remove the `docs` and `corpus` profiles from `corvint mcp` with a typed refusal; rename the
      flow tools. Requirements: MTT-V1-005, MTT-V1-007. Proof: tool-list test and refusal test.
- [ ] 2.6 `CORVINT_SELECTION` handling in the `affected` tool. Requirements: MTT-V1-008. Proof: an
      unknown value returns all tests with the reason; `full:go` selects all Go tests.
- [ ] 2.7 Parity tests for rows with a CLI equivalent. Requirements: MTT-V1-009. Proof: one parity
      test per row.
- [ ] 2.8 No tasks tools; retire MCP binaries from `binaries.json` in their removal batch.
      Requirements: MTT-V1-006, MTT-V1-010. Proof: tool-list test and the release binaries check.
- [ ] 3.1 Mark PROD-DOCS-001 DELIVERED in the plan evidence once 2.4 and 2.5 land. Requirements:
      MTT-V1-004, MTT-V1-005. Proof: plan row read-back.
- [ ] 3.2 Build-log entry; complete the ticket through the Tasks gate. Requirements: all. Proof:
      `corvint-tasks ticket show` and `receipt audit`.
