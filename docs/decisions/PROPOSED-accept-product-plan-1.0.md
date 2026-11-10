# Proposed: accept the Corvint 1.0 product plan

- Date: 2026-11-09
- Status: proposed. Not accepted. This file has no decision number and no row in
  `docs/decisions/README.md`. Only the owner can accept it, in whole or in part. On acceptance it
  becomes `docs/decisions/NNNN-accept-product-plan-1-0-YYYY-MM-DD.md` with the next free number
  (change `acceptance-record`, ACR-V1-001).
- Authority it would record: owner answers 1 to 18, G1 to G27 and R1 to R41 of 2026-11-09, recorded
  in the owner-answers file of that date. Owner decision R1 (2026-11-09): one decision record
  accepts the plan as amended by these answers, names the drafted wording, and states that
  in-flight work continues as experimental.
- Accepts: `docs/specs/PRODUCT-PLAN-1.0.md` as amended by those answers, at a commit SHA to be
  written here when the owner signs (open: the plan is still being edited; owner decision R35
  (2026-11-09) requires its proposal dependency table and anchored code citations first).
- Amends or supersedes: decision 0433 item 1 (in part), decision 0592 items 1 and 2 (in part),
  decision 0594 (Keeps line, item 3, item 4), owner answers 3 and 11, the G9 MCP row and R26 (table
  below).
- Read with: the change `docs/specs/openspec/changes/acceptance-record/` (ACR-V1-001 to
  ACR-V1-010: record format, supersession table, acceptance log
  `docs/specs/openspec/acceptance-log.tsv`), and
  `docs/decisions/PROPOSED-supersede-0594-keeps-line.md` with the change
  `docs/specs/openspec/changes/source-removal/`.

## Context

Owner answer 18 asked for one decision record that accepts the 1.0 plan and supersedes the pre-cut
1.0 contract, carrying over what is still valid. The owner then answered the expert-grill round
(G1 to G27) and the review round (R1 to R41). Owner decision G14 (2026-11-09) accepts the plan once;
a later proposal that only implements an accepted requirement is accepted automatically and logged.
This record is that single acceptance. Each answer below is one line; the owner-answers file holds
the full text.

## Decision (proposed)

### 1. Grill-round decisions accepted (G1 to G27)

- G1: Go, Vitest, Jest, Playwright, pytest, RSpec and Minitest block the release.
- G2: browser end-to-end tests are narrowed only by accepted flow links plus static reachability.
- G3: zero-miss proof by plain PR replay plus a frozen fault-injection set, 100 PRs per runner.
- G4: tiered narrowing bar, frozen first.
- G5: flow reports that join test evidence are native in `corvint`.
- G6: one flow file `.corvint/flows.json`; a flow selects, its variation is shown.
- G7: `corvint watch` is gated for all seven runners.
- G8: the CEM, OCM and dogfood ceremony is replaced fully.
- G9: Corvint 1.0 is coupled to corvint-tasks 1.0, with a 21-day trial.
- G10: 1.0 is done at the freeze date; anything not proven is demoted by a written owner note.
- G10a: freeze 2026-12-15; stable no earlier than 2027-01-05.
- G11: an end-to-end test linked to no flow always runs; links are proposed, then accepted.
- G12: proof repositories by a written rule plus named repositories, four held out.
- G12a: the named repository list, each checked against the selection rule first.
- G13: test kinds by driver plus declaration.
- G14: the owner accepts the plan once, by class; changes follow OpenSpec.
- G15a: a release starts from a tag and publishes only after owner approval.
- G15b: a changed non-code file selects every test that may read it unless a proven map entry
  narrows it.
- G16: the lean core of the GitHub Action blocks 1.0.
- G17: the Action lives in the new repository `beamfall/corvint-action`.
- G18: Corvint's own CI runs the Action alongside its scripts, then cuts over.
- G19: Claude Code and Codex block at the full bar; OpenCode and Pi ship as preview.
- G20: agent packages live in the new repository `beamfall/corvint-agents`, rebuilt, not restored.
- G21: agent proof by a pinned gate plus a weekly run against each agent's newest release.
- G22: the VS Code extension moves out and is rebuilt after 1.0.
- G23: one compiled-in runner contract; the out-of-process protocol is dropped.
- G24: partial cut of the test-quality features.
- G25: `impact`, `review` and `context` are revived as proposed; `feature`, `features` and
  `overview` fold into flows.
- G26: languages and analyzers beyond the seven runners are removed.
- G27: process tooling, the spec archive and the removal order follow the panel plan.

### 2. Review-round decisions accepted (R1 to R41)

- R1: one record accepts the plan as amended, names the drafted wording, keeps in-flight work
  experimental (this file).
- R2: qualify before the freeze; candidate tag about 2026-12-08; stable 2027-01-05.
- R3: milestone M0 re-scopes the v1-0 release object; Tasks policy v15 drops the CEM, OCM,
  cem-verdict and context-focused fields.
- R4: docs review is advisory by default in blocking consumers, with an opt-in to block.
- R5: a blocking Stop decision with a reason, at most one nudge per turn; SessionStart
  (source=compact) instead of PostCompact.
- R6: Cypress is dropped from 1.0 and listed post-1.0.
- R7: the seven-runner set is confirmed; PROD-TEST-020 adds an append-only run result record; Jest
  gets its own provider proposal.
- R8: line-exact selection for Go, RSpec, Minitest, Vitest and Jest; pytest selects by file with a
  tier label.
- R9: Python file-level selection is a release exception (section 5).
- R10: one committed test inventory under `.corvint/`, written by `corvint inventory refresh`;
  missing or stale runs all tests with a named reason.
- R11: canonical test ID (RNC-V1-012) with fresh requirement IDs.
- R12: flow links are proposed with a confidence score and accepted only by a person.
- R13: an unknown `CORVINT_SELECTION` value exits 2 in CLI and CI and runs all tests in hooks and
  MCP; grammar `full:<runner>` everywhere.
- R14: provider binaries retire after their compiled-in runner contract lands and fold into
  `corvint`.
- R15: several instances per runner, keyed by root directory; sharding in the Action only.
- R16: one tier vocabulary `line`, `file`, `flow`, `full` plus `exact`; `flow` is not counted as
  narrowed for the no-miss claim.
- R17: PROD-TASK-001 to PROD-TASK-009 confirmed; Tasks publishes schema v1 and a supported-command
  list; CLI only, no Tasks MCP server in 1.0.
- R18: the Tasks trial and conformance subject is `beamfall/core`.
- R19: 100 planted faults per held-out repository, at most 3 misses with a Wilson interval, at most
  5 percent of tests excluded, per-test durations in every receipt.
- R20: the PR agreement figure is the share of soak-window PRs whose narrowed and full pass/fail
  agree on the same commit.
- R21: held-out repositories fastapi, solidus and payload, each verified first.
- R22: one plan artifact with a digest; shards filter by runner; a final job checks every shard
  used it.
- R23: narrow on pull_request and push; fork PRs run everything.
- R24: actions pinned by SHA; contents read plus id-token write only for attestation; no PR
  checkout under pull_request_target.
- R25: `corvint-action` and `corvint-agents` are Apache-2.0.
- R26: agent qualification keys and spend cap (amended by R39 and R40).
- R27: one pinned model per agent host; model ID and agent version recorded.
- R28 and R29: `corvint context` ships redesigned and non-blocking, with no claim of measured
  benefit; the measurement runs in 1.0.x.
- R30: docs tools on one `corvint-docs mcp` server; `corvint mcp` keeps the testing tools; one
  tool table.
- R31: only Corvint configuration files under `.corvint/` force a full run; a flows edit runs the
  linked tests; doc-only edits and the two private ledgers add nothing.
- R32: a change no runner owns, or a removed or unavailable runner, runs all tests with a named
  reason; `UNOWNED_DIRTY_PATH` comes off the allowlist.
- R33: the VS Code extension moves to the public repository `beamfall/corvint-vscode`; orphaned
  code is deleted and tracked in a table.
- R34: one build-log entry per removal batch; the superseding decision stays short.
- R35: before acceptance, the plan gets one proposal dependency table and every code citation is
  anchored to a fixed commit.
- R36: `make windows-vet` runs once per removal batch, before that batch's push, for removal lanes
  only.
- R37: PRS-V1-018 (c) and (d) carry over; (a), (b) and (e) drop; out-of-scope specs are archived
  in place with spec health classes.
- R38: testing build order is canonical test ID, inventory, run results, line evidence, flow
  links.
- R39: no API keys and no spend cap; agent qualification uses the owner's subscription logins on
  the owner's machines, and its result is a release evidence file.
- R40: Codex qualification uses the logged-in Codex CLI.
- R41: every change uses the OpenSpec propose, apply and archive stages under
  `docs/specs/openspec/changes/`; the change `spec-set-cleanup` gives every old document a verdict.

### 3. Drafted wording this record accepts

These requirement texts were written for the plan, not quoted from an owner answer (ACR-V1-002).
Accepting this record accepts their wording as it stands in the plan at the signed SHA:

- PROD-INT-003 (one capability table for every command and MCP tool) and PROD-INT-007 (1.0 wire
  formats and commands frozen), checklist row O60, owner decision R1 (2026-11-09).
- PROD-TASK-001 to PROD-TASK-006, including the source of PROD-TASK-006, checklist row O61, and
  PROD-TASK-007 to PROD-TASK-009, confirmed by owner decision R17 (2026-11-09).
- Every other plan requirement marked "(proposed 2026-11-09)" that the acceptance log lists with
  mode `owner` at signing. A requirement the log lists as `pending-owner` is not accepted by this
  record.

### 4. In-flight work stays experimental

Work in flight on `claude/layers-integ` and in open change folders continues as experimental
(owner decision R1 (2026-11-09); checklist row O16). Nothing may be described as delivered, and no
experimental work may be promoted or advertised, before this record is accepted and that work's own
change is accepted or auto-accepted under G14.

### 5. Release exception (owner decision R9 (2026-11-09))

pytest selects by file, not by line, in 1.0, and says so with its tier label (R8). This is a
recorded release exception to the line-exact bar. The Python line collector is a 1.0.x ticket. The
exception does not weaken the no-miss rule: a file-level selection still runs every test in each
selected file.

### 6. Superseded text

Quotes are copied from this branch at `007bace5d` (ACR-V1-004).

| Superseded source | Quoted text | Replacement | Effect |
|---|---|---|---|
| Owner answer 3 (line-level narrowing) | "Go, JS and Playwright must be line-level or the release blocks." | G2: browser end-to-end tests (Playwright) narrow by flows plus reachability only; line-exact set by R8, pytest at file level by R9 | amended |
| Decision 0433 item 1 | "the owner selects **No signing**" | G15a: build attestation on release artifacts; G20 and checklist row O26: npm trusted publishing with provenance for the agent packages. Support window and the rest of 0433 stay | amended, attestation and npm provenance only |
| Decision 0592 item 1, RSP-V0-025 | "A runner's pieces are provided in one of two forms" | G23: compiled-in only (RSP-V0-025 MODIFIED in the change `source-removal`) | amended |
| Decision 0592 item 1, RSP-V0-026 (and RSP-V0-027 to RSP-V0-032, RSP-V0-047) | "`corvint-runner/1` is UTF-8 JSON over the plugin's standard input and output" | G23: the out-of-process protocol is dropped for 1.0 (REMOVED in the change `source-removal`) | removed |
| Decision 0594 Keeps line, item 3 and item 4 | "Keeps: the `corvint` top-level verbs adopt, affected, cem (item 4), context, ..." | `docs/decisions/PROPOSED-supersede-0594-keeps-line.md` (G8, G22 to G27, R14, R28/R29, R32, R33) | superseded through that decision |
| Owner answer 11 | "Tasks serves its own MCP; `corvint mcp` has no tasks tools." | R17: Tasks is CLI only; no Tasks MCP server in 1.0 | superseded |
| G9 conformance pack, MCP row | "MCP parity with CLI" | R17: no Tasks MCP server in 1.0; the parity row leaves the pack | removed |
| R26 | "owner provides API keys as repository secrets with a HARD monthly cap" | R39 and R40: subscription logins and the logged-in Codex CLI on the owner's machines | amended |

The pre-cut 1.0 contract rows (PRS-V1-001 to PRS-V1-018) are resolved as the plan's "Conflicts with
accepted text" table says, with PRS-V1-018 (c) and (d) carried over by R37.

## Rollback

Before acceptance: delete this file; nothing else depends on it. After acceptance: a decision
record is never deleted (ACR-V1-001); a new decision record supersedes this one, names what it
restores, and the acceptance log rows it changes go back to `pending-owner`.

## What the owner decides

1. Accept, amend or reject sections 1 to 6.
2. The plan commit SHA this record names.
