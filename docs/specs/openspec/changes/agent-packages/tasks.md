# Tasks: `agent-packages`

## Tasks

Build tasks in order. Each names its requirement IDs and its proof.

1. Owner accepts the superseding decision (AGT-V1-028); file the ticket in `.taskman/`; reserve
   AGT-V1-001 to 030. Proof: `corvint-tasks ticket show` lists the thirty IDs.
2. First commit: create `docs/specs/agent-packages-v1.md`; annotate PRS-V1-006 citations; regenerate
   the spec index. Proof: `make spec-requirements` lists AGT-V1-001 to 030.
3. Check every NOT_VERIFIED upstream item, including the Codex `Stop` block reason and
   `SessionStart` source `compact`, and record the result with URL and date (AGT-V1-013, 018).
   Proof: a retained note per item.
4. Binary: `corvint agent hook`, `corvint agent manifest`, `corvint agents doctor`, session state,
   `CORVINT_HOOKS`, the `Stop` decision with loop guard, the session-start diff, the unknown
   `CORVINT_SELECTION` rule, budgets and secret screen (AGT-V1-003, 012 to 022). Proof: unit tests
   per host and event, including the loop-guard and unknown-value scenarios.
5. Receipt flag after PROD-TEST-020 lands (AGT-V1-014). Proof: the "no receipt" scenario passes.
6. Create `beamfall/corvint-agents` under Apache-2.0: generator, three skills (tasks skill on the
   `corvint-tasks` CLI), four shims, MCP registration from change `mcp-tool-table`, drift check, npm
   publishing for OpenCode and Pi (AGT-V1-002, 004, 005, 007, 010, 011). Proof: drift check and npm
   pack audit.
7. Workflow lint for AGT-V1-029 (triggers, no hosted runner for logged-in jobs, no API key). Proof:
   the forbidden-trigger and hosted-runner scenarios fail the lint as required.
8. Qualification harness from `841a58cbd^`; fixture repository; pinned and weekly runs on the
   owner's machines with subscription logins (AGT-V1-023 to 025, 029). Proof: one evidence file per
   release candidate with the tuple including model ID.
9. Measure end-of-turn p50 and p95 on the seven pinned repositories and set the budget
   (AGT-V1-030). Proof: the retained latency record.
10. Retire `integrations/` and its consumers after the first qualified release (AGT-V1-027). Proof:
    the orphan-reference scenario.
11. Build-log entry; complete the ticket through the Tasks gate. Proof: `corvint-tasks receipt audit`
    passes.

Earlier task list, kept for traceability:


- [ ] 1.1 Owner accepts the superseding decision (AGT-V1-028); file the ticket; reserve AGT-V1-001
      to 030.
- [ ] 1.2 First commit: create `docs/specs/agent-packages-v1.md`; annotate PRS-V1-006 citations;
      regenerate the spec index.
- [ ] 1.3 Check every NOT_VERIFIED upstream item and record the result with URL and date.
- [ ] 2.1 Binary: `corvint agent hook`, `corvint agent manifest`, `corvint agents doctor`, session
      state, `CORVINT_HOOKS`, budgets and secret screen.
- [ ] 2.2 Create `beamfall/corvint-agents`: generator, three skills, four shims, drift check, npm
      publishing for OpenCode and Pi.
- [ ] 3.1 Qualification harness from `841a58cbd^`; fixture repository; pinned and weekly runs.
- [ ] 3.2 Qualify Claude Code and Codex on Linux and macOS; retain transcripts.
- [ ] 4.1 Retire `integrations/` and its consumers after the first qualified release.
- [ ] 4.2 Build-log entry; complete the ticket.
