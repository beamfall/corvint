# Tasks: `flows-status-report`

Build tasks in order. Each names its requirement IDs and its proof.

- [ ] 1 Reserve `AFL-V1` 001..008 and set `requirementRefs` on V1-5257. Proof: `corvint-tasks ticket
      show V1-5257` lists the eight IDs.
- [ ] 2 First commit: create `application-flow-status-v1.md` with the eight definition lines, its
      `docs/specs/README.md` entry and the regenerated index. Proof: `make spec-requirements` lists
      them.
- [ ] 3 Native `corvint flows status` in the tests layer; remove the `corvint-docs` copy
      (AFL-V1-001, 005, 006). Proof: Go tests and the corvint-alone smoke run.
- [ ] 4 Receipt reader for `corvint-test-run/0` with granularity marking (AFL-V1-002). Proof:
      fixture tests for no source, a receipt, an ingested report, a stale result and a file-level
      outcome.
- [ ] 5 Native `corvint flows impact`; remove the `corvint-docs` copy with a notice; keep
      `corvint.flows.impact` (AFL-V1-007). Proof: the two AFL-V1-007 scenarios and the APK-V0-007
      comparison test.
- [ ] 6 `--base` parity with `flows impact` (AFL-V1-004). Proof: the comparison test.
- [ ] 7 MCP rows for `corvint.flows.status` and `corvint.flows.impact` after change `mcp-tool-table`
      fixes the table (AFL-V1-003, 007; owner decision R30, 2026-11-09). Proof: a tool-list and
      parity test on `corvint mcp`.
- [ ] 7a Rename the three docs command families into one group with one-release aliases, before
      `user-docs` (AFL-V1-008; R42 B4). Proof: the alias and help-text tests.
- [ ] 8 Docs, build-log entry; complete V1-5257 through the Tasks gate. Proof: `corvint-tasks
      receipt audit` passes.

Earlier task list, kept for traceability:

- 1.1 Reserve `AFL-V1` 001..007; set `requirementRefs`.
- 1.2 First commit: create `application-flow-status-v1.md` with the seven definition lines; add
  the `docs/specs/README.md` entry; regenerate the index.
- 2.1 Native `corvint flows status` in the tests layer; remove the `corvint-docs` copy.
- 2.2 Receipt reader for `corvint-test-run/0`; granularity marking.
- 2.3 MCP parity.
- 2.4 Native `corvint flows impact` in the tests layer; remove the `corvint-docs` copy with a
  notice; keep `corvint.flows.impact` (AFL-V1-007).
- 3.1 Fixtures, tests and the corvint-alone smoke test; docs; build-log; Tasks gate.
