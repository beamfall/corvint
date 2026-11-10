# Doc verdicts: every entry directly under `docs/` (change `spec-set-cleanup`)

Status: proposed (written 2026-11-09; only the owner can accept it). Requirements: SSC-V1-001,
SSC-V1-002, SSC-V1-004, SSC-V1-006.

One row per entry directly under `docs/` in the prodspecs tree: 46 rows. `docs/specs/` (see
`specs-verdicts.md`), `docs/decisions/` and the individual `docs/build-log/` entries are excluded;
the `build-log/` directory itself has one row.

## Method

- Verdict: `keep` (stays active as is, or with edits another change already owns), `rewrite`
  (stays active and is rewritten to the 1.0 scope), `archive` (stays at its path with the archive
  marking of SSC-V1-002 and leaves the active list in `docs/README.md`), `delete` (removed from the
  tree; git history keeps it, SSC-V1-008), `move to <repo>`, or `undetermined` with what is missing.
- 1.0 scope (owner decision R33 and the three-product rule): in scope are testing, documentation,
  tasks, the CI Action and the agent packages. Out of scope are CEM, OCM, dogfood, context
  injection, LSP, the removed-language analyzers, the console and the model gateway.
- Batch: the `source-removal` batch (B0 to B11) or cleanup batch (C1 to C3, defined in
  `specs-verdicts.md`) that applies the row; a change name means that change already owns the edit.
  `-` means no batch changes the row.
- References: the number of files in integ 9ec3a29d0, outside `docs/build-log/`,
  `docs/decisions/` and the entry itself, that contain `docs/<entry>`, plus relative links
  `(<entry>` inside `docs/`, `README.md` and `AGENTS.md`; then the largest groups. The `.corvint`
  group includes `.corvint/changes/*.cem.json` (removed in B6) and
  `.corvint/documentation-paths.json`, which `make documentation-paths` regenerates from the
  top-level `docs/*.md` files (Makefile in integ 9ec3a29d0), so a deleted doc leaves it in the same
  commit.

## Counts

- keep: 20
- rewrite: 10
- archive: 9
- delete: 6
- move: 0
- undetermined: 1

## Table

| Entry | Verdict | Batch | Reason (1.0 scope) | References |
|---|---|---|---|---|
| `AGENT-ROUTES.md` | rewrite | C1, B6, B8b | Routes stay; the lane row (line 13), the `.agent-evidence` grep (line 21) and the pre-1.0 CEM route (line 24) point at out-of-scope work. | 38: .corvint 18, benchmarks 4, docs/specs 4, script 3 |
| `APPLICATION-MAP.md` | keep | - | Documentation product (`corvint flows appmap`). | 12: .corvint 3, internal 3, cmd 2 |
| `ARCHITECTURE.md` | rewrite | B6 | 13 CEM mentions; must describe the testing, documentation and tasks products. | 31: benchmarks 15, docs/specs 6, tools 4 |
| `AUTOMATION.md` | rewrite | `github-action` | CI Action is in scope; the `github-action` change rewrites the adopter recipe. | 110: .corvint 95, docs/specs 4, .taskman 3 |
| `BEAMFALL-INTEGRATION.md` | archive | C1 | Consumer contract for ranking, receipts and traces (context); no 1.0 product names it and `beamfall-shadow-dogfood-v1` is RETIRE. | 4: conformance 2, .corvint 1, docs/README.md 1 |
| `BENCHMARK-HOST.md` | keep | - | Quiet-host setup for the measured testing targets (`narrowing-measurement`, watch). | 6: .corvint 4, docs/AGENT-ROUTES.md 1 |
| `CAPABILITIES.md` | rewrite | B2, B5, B6 | Lists out-of-scope capabilities; each removal batch already edits it, and the final text lists only 1.0 capabilities. | 127: .corvint 77, docs/specs 17, tools 9 |
| `CHANGE-EVIDENCE-MAP.md` | archive | B6 | CEM, out of scope; archived in the batch that removes CEM. | 35: docs/specs 12, .taskman 6, benchmarks 4 |
| `CHANGE-FLOW.md` | delete | B6 | Competing workflow document: fixes the order of the dogfood closing steps (dogfood, out of scope). Its readers are CEM files B6 removes, `daily-change-evidence-workflow-v0` (retired) and `documentation-paths.json`. | 5: .corvint 4, docs/specs 1 |
| `DOCUMENTATION-CORPUS.md` | keep | - | Documentation product (DCP-V1). | 14: .corvint 6, docs/specs 4, cmd 2 |
| `DOGFOOD.md` | keep | - | Already an 11-line retirement stub (decision 0594) that redirects; 303 references, most in files B6 removes. Deleting it is re-decided after B6 (decided 2026-11-09 under R42, E6: the stub stays until then; self-use is recorded as a progress note on the change's ticket, or in the PR body when there is no ticket, and friction is filed as tickets). | 303: .corvint 178, docs/specs 46, .taskman 30 |
| `EXTENSION-ECOSYSTEM.md` | archive | B4 | B4 adds its status line; the `agent-packages` change supersedes lines 123-125. | 11: .taskman 3, benchmarks 3, conformance 2 |
| `EXTERNAL-EVIDENCE-PROVIDERS.md` | delete | B4 | B4 removes it with `examples/evidence-provider`. | 6: benchmarks 2, .corvint 1, .taskman 1 |
| `FLOW-COVERAGE.md` | keep | - | Documentation and testing product (flows). | 8: docs/specs 5, .corvint 1 |
| `FLOW-DOCUMENTATION.md` | keep | - | Documentation product (flows). | 6: docs/specs 2, .corvint 1, cmd 1 |
| `GLOSSARY.md` | rewrite | B6 | Keeps the 1.0 terms; drops the CEM, `lrf-0` and `tcq-0` entries. | 22: .corvint 17, docs/specs 3 |
| `INSTALL.md` | keep | `user-docs` | Install is in scope; the `user-docs` change rewrites the uninstall paragraph. | 23: benchmarks 5, .corvint 4, docs/specs 4 |
| `LANE-ORCHESTRATION.md` | rewrite | B8b | Becomes a stub when B8b removes the lane scripts; read by 21 scripts, the Makefile, `tools/flow-guide-check` and one `.claude` skill until then. | 115: .corvint 76, script 21, docs/specs 9 |
| `LEGACY-ADOPTION.md` | keep | - | Brownfield documentation recovery through `init`/`adopt`. Decided 2026-11-09 under R42, E8: `init` and `adopt` are 1.0 first-run verbs with a product requirement, so the document stays (same decision as `genesis-backfill.md`; low confidence). | 6: conformance 2, docs/specs 2, llms.txt 1 |
| `MCP-SERVER.md` | keep | `mcp-tool-table` | Agent packages use it; the `mcp-tool-table` change owns its table. | 19: .corvint 6, benchmarks 3, docs/specs 3 |
| `PORTFOLIO-0.6.md` | archive | C1 | 0.6 reading map; not 1.0 guidance. | 6: .corvint 2, ROADMAP.md 1, benchmarks 1 |
| `PRODUCT.md` | rewrite | C1 | Lines 5 and 34 call Corvint a "change-evidence verifier"; must state the three 1.0 products. | 73: docs/specs 32, benchmarks 16, conformance 7 |
| `README.md` | rewrite | C1 | Links CHANGE-EVIDENCE-MAP (line 14) and DOGFOOD (line 25); becomes the active list plus an "Archived" section (SSC-V1-002). | 37: benchmarks 22, .corvint 3, docs/specs 3 |
| `RELEASE-NOTES-alpha.md` | keep | - | Release history; not guidance. | 13: .corvint 4, benchmarks 3, docs/specs 2 |
| `RELEASE-NOTES.md` | keep | - | Release history; `user-docs` adds `CHANGELOG.md` for 1.0. | 35: docs/specs 8, .corvint 7, .taskman 5 |
| `RELEASE-RUNBOOK.md` | keep | `release-process` | Release path is in scope. | 16: .corvint 4, benchmarks 4, docs/specs 3 |
| `SECURITY.md` | keep | - | Applies to every product. | 4: .corvint 1, .taskman 1 |
| `SELF-DEVELOPMENT.md` | rewrite | B6, B8b | Line 13 (lane), line 30 (CEM1 verify-candidate) and line 93 (CEM bind, check and seal) are out of scope. | 26: benchmarks 5, docs/specs 5, .taskman 4 |
| `SHELL-TEST-REGISTRATION.md` | keep | - | Testing product gate. | 35: .corvint 25, docs/specs 3, script 3 |
| `SPEC-DRIVEN-DEVELOPMENT.md` | rewrite | C1 | Becomes the single workflow document (SSC-V1-001); outline in `design.md`. | 171: .corvint 68, docs/specs 51, conformance 15 |
| `SPEC-TOOLCHAIN-INTEGRATION.md` | archive | C1 | 2026-08-29 assessment, never accepted; G14 settles the OpenSpec question. Most readers are CEM files B6 removes. | 56: .corvint 48, docs/specs 3 |
| `TECHNICAL-BRAIN.md` | archive | C1 | Proposed context contract (context injection, out of scope); its dogfood spec is retired. | 20: docs/specs 10, conformance 7 |
| `TEST-CLAIM-QUALIFICATION.md` | archive | B2 | B2 (G24) removes the TCQ internals it documents. | 5: conformance 2, .corvint 1 |
| `TEST-READ-SCOPES.md` | keep | - | Testing product gate. | 8: .corvint 6 |
| `TICKET-PRIORITY.md` | keep | - | This repository's queue uses the rubric (AGENTS.md line 150 in integ 9ec3a29d0). | 9: .agent-evidence 4, .corvint 2, docs/specs 2 |
| `V1-WORKFLOWS.md` | delete | C1 | Competing workflow document ("six experimental change-evidence workflows"); its only readers are `documentation-paths.json`, `CAPABILITIES.md` and `SELF-DEVELOPMENT.md`, edited in the same commit. | 3: .corvint 1, docs/CAPABILITIES.md 1, docs/SELF-DEVELOPMENT.md 1 |
| `agent-memory/` | keep | - | Six files that already redirect to the task store; read by the Makefile, `cmd/corvint` tests, `internal/liveverify/mutate/export.go`, `internal/secretscreen/secretscreen.go` and `script/agent-preflight_test.sh`. | 70: benchmarks 26, docs/specs 13, conformance 6 |
| `build-log/` | keep | - | One entry per removal batch (owner decision R34) and per material decision. | 843: .corvint 450, .taskman 181, docs/specs 118 |
| `cem-0.1.schema.json` | delete | B6 | CEM; no live code reader in integ 9ec3a29d0 (only pinned result snapshots and `protocol/cem-1.0/ALGORITHMS.md`). | 11: conformance 6, docs/specs 2, tools 2 |
| `cem-0.2.schema.json` | delete | B6 | Same as `cem-0.1.schema.json`. | 11: conformance 6, tools 2 |
| `experiments/` | keep | - | Dated result files cited as evidence (including the 2026-11-04 linecov shadow runs for testing); history, not guidance. | 13: .corvint 7, docs/specs 5 |
| `extraction-manifest.json` | keep | - | `PROVENANCE.md` reads it. | 5: conformance 2, docs/specs 2, PROVENANCE.md 1 |
| `lrf-0.schema.json` | archive | C1 | Ranking schema; `lexical-relevance-floor-v0` is retired with 0 code hits. JSON takes no header, so the archive marking is its listing in the README "Archived" section; delete is re-decided with B7. | 11: conformance 6, docs/specs 2, tools 2 |
| `plans/` | archive | C1 | Two 2026-09 plans; the first already says the task store supersedes it. | 80: benchmarks 27, conformance 13, docs/specs 11 |
| `tcq-0.schema.json` | undetermined | B2 | No Go file in integ 9ec3a29d0 names `tcq-0`; B2 removes the TCQ internals. Missing: B2's check that no kept command still writes a `tcq-0` result (then delete; otherwise archive). | 10: conformance 6, tools 2 |
| `worklist.json` | delete | B1 | Read only by `internal/worklistadapter`, which B1 removes. | 6: conformance 2, .corvint 1 |
