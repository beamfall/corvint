# Proposed: accept the Corvint 1.0 product plan

- Date: 2026-11-09
- Status: proposed. Not accepted. This file has no decision number and no row in
  `docs/decisions/README.md`. Only the owner can accept it, in whole or in part. On acceptance it
  becomes `docs/decisions/NNNN-accept-product-plan-1-0-YYYY-MM-DD.md` with the next free number
  (change `acceptance-record`, ACR-V1-001).
- Authority it would record: owner answers 1 to 18, G1 to G27, R1 to R41 and R42 of 2026-11-09,
  recorded in the owner-answers file of that date, and owner decisions H1 to H12 of 2026-11-10,
  recorded in the same file. Owner decision R1 (2026-11-09): one decision
  record accepts the plan as amended by these answers, names the drafted wording, and states that
  in-flight work continues as experimental. Owner decision R42 (2026-11-09, final round) is a
  delegation: every still-open question takes the expert panel's recommendation as the decision.
  Publishing, stable promotion, credentials, repository creation and repository visibility stay
  with the owner. Each R42 item is a panel pick, not an owner-worded answer (section 3).
- Accepts: `docs/specs/PRODUCT-PLAN-1.0.md` as amended by those answers, at a commit SHA to be
  written here when the owner signs (open: the plan is still being edited; owner decision R35
  (2026-11-09) requires its proposal dependency table and anchored code citations first).
- Amends or supersedes: decision 0433 item 1 (in part), decision 0592 items 1 and 2 (in part),
  decision 0594 (Keeps line, item 3, item 4), decision 0385 (Core classification of `e2e-safe`),
  owner answers 3 and 11, the G9 MCP row, R26, the use-case receipt contract, and in `AGENTS.md`
  the "Dogfood Corvint" section and product invariants 3, 4, 5 and 7 (table in section 7). The
  owner's own later decisions amend R21's fixed held-out list (H3) and G14 and R41 (H12).
- Read with: the change `docs/specs/openspec/changes/acceptance-record/` (ACR-V1-001 to
  ACR-V1-012: record format, supersession table, acceptance log, owner signing key, the
  `AGENTS.md` invariant edits). The
  acceptance log `docs/specs/openspec/acceptance-log.tsv` holds only its header line at this
  commit; its rows are written by change `acceptance-record` (task 2.5), so every sentence below
  that reads the log applies only once those rows exist. Also read with
  `docs/decisions/PROPOSED-supersede-0594-keeps-line.md` and the change
  `docs/specs/openspec/changes/source-removal/`; that file and decision 0594 are not on this
  branch (section 7).

## Context

Owner answer 18 asked for one decision record that accepts the 1.0 plan and supersedes the pre-cut
1.0 contract, carrying over what is still valid. The owner then answered the expert-grill round
(G1 to G27) and the review round (R1 to R41). In the final round the owner delegated every question
still open to the expert panel's recommendation (R42, the owner's words: "can you just use the panel
reommendations for unanswered?"), keeping publishing, stable promotion, credentials, repository
creation and repository visibility. Owner decision G14 (2026-11-09) accepts the plan once; a later
proposal that only implements an accepted requirement is accepted automatically and logged. This
record is that single acceptance. On 2026-11-10 the owner answered the eleven items this record
carried open (H1 to H11, section 9) and amended G14 and R41 on the OpenSpec command (H12, section
1). Each answer below is one line; the owner-answers file holds the full text. That file is outside
this repository.

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
- H12 (2026-11-10), amending G14 ("no npm tool, no `openspec/` root, no claim of OpenSpec-CLI
  compatibility") and R41. The owner's words: "install openspec and ensure we are following its
  flows for this". Applied as:
  - `docs/specs/openspec/` is an OpenSpec root with `config.yaml` (schema `spec-driven`). The
    OpenSpec command (`@fission-ai/openspec`, pinned 1.17.0) is run from `docs/specs`. The
    repository root still holds no `openspec/`.
  - Propose, apply and archive use the command: `openspec new change`, `openspec validate`,
    `openspec status`, `openspec list`, `openspec archive <name> --skip-specs`.
  - One deliberate difference from stock OpenSpec, kept from G14 and not yet confirmed by the
    owner: the canonical specs stay the flat files `docs/specs/*.md` with stable requirement IDs,
    so apply copies each requirement into the owning spec and archive runs with `--skip-specs`.
  - Bar: `openspec validate --all` passes for all 45 change folders; strict-mode warnings are
    reported, not failing (OSC-V1-012, change `openspec-check`). The Go check OSC-V1-001 to
    OSC-V1-011 does not need the command, and the default product build gains no npm dependency
    (`AGENTS.md` invariant 7 is about the product binary and is unaffected).
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
- R9: Python file-level selection is a release exception (section 6).
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
- R21: held-out repositories fastapi, solidus and payload, each verified first. The fixed list is
  superseded by owner decision H3 (2026-11-10): one held-out repository per family is drawn from a
  sealed pool and never redrawn (section 9, H3); these names may be pool candidates.
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
  Owner decision H12 (2026-11-10) extends R41: the stages are run with the OpenSpec command
  (section 1, H12).

### 3. Final-round picks delegated under R42 (proposed)

Owner decision R42 (2026-11-09) delegates each question below to the expert panel's
recommendation. Every row is a panel pick, not an owner-worded answer, and is listed here so the
owner can amend or reject it one by one. "Lands in" names the change folder under
`docs/specs/openspec/changes/`, the requirement or the plan row that carries the pick; "no text"
means no plan line or change folder cites it and only the owner-answers file records it. A row
marked "fix" corrects a spec and decides nothing new. The acts R42 keeps with the owner are in
section 8; the items carried open at drafting, and the owner's answers of 2026-11-10 (H1 to
H11), are in section 9.

| Pick | Decision in one line | Lands in |
|---|---|---|
| A1 | An owned changed path that reaches no flow runs every E2E test under one named reason | `watch-flows-hardening` |
| A2 (fix) | The deprecated `.corvint/e2e-flows.json` reader stops narrowing | `watch-flows-hardening` (WFF-V1-007) |
| A3 | Already-passed tests are reordered only, never skipped, in 1.0; skipping is a post-1.0 proposal | `run-evidence` (RVD-V1-011) |
| A4 | The `e2e-safe` profile and the appmap navigation verdict leave the public surface in 1.0; code deleted by 2026-12-01 or in the first 1.0.x | `source-removal` (RMV-V1-037); plan row O66; section 7 |
| A5 | Settled by G15b: `go.mod` and `go.sum` are never narrowed | no text |
| A6 (fix) | Jest `roots`, `testPathIgnorePatterns`, `moduleNameMapper` and Vitest `dir`, `root` join the dynamic-config list; literals are evaluated | `selection-hardening` (SLF-V1-005) |
| A7 | One-shot `affected` and `test` use the `watch` working-tree rule for line selection | `line-evidence` |
| A8 (fix) | The Go-only-edit and TypeScript-unknowns settling experiment runs now | `selection-hardening` (design, tasks) |
| A9 | Zero known misses on the seven pinned repositories' fault sets; never-seen repositories pass at up to 3 misses per 100, each published with its interval, filed P1 and fixed before the candidate | `fault-injection-harness`, `proof-repositories`; PROD-INT-008 |
| A10 | Both caps apply, on different sets: 2 percent flaky quarantine and 5 percent excluded tests | `fault-injection-harness`, `narrowing-measurement` |
| A11 | The `watch` 1-second bar is measured on the owner's Mac and on WSL2 on the benchmark box, on the seven pinned repositories, sizes published, Go also timed on Corvint, cores capped at 8 | `watch-seven-runners`; PROD-TEST-007 |
| A12 | The 50-change replay runs on each never-seen held-out repository, the 50 most recent merged PRs before each pin; under owner decision H3 (2026-11-10) these are the four drawn repositories, one per family, not the names fastapi, solidus and payload | `narrowing-measurement`, `proof-repositories` |
| A13 | Proof-repository swaps are made by rule and reported; only a Minitest alternate that is a second Rails component, or Playwright if freeCodeCamp fails, comes back to the owner | `proof-repositories` |
| A14 | Held-out Go repository `helm/helm`, alternate `go-task/task`; its test count is still to verify. Superseded by owner decision H3 (2026-11-10): both are Go pool candidates (PRR-V1-011), not a fixed pick | `proof-repositories` (PRR-V1-004), `fault-injection-harness`; PROD-INT-008 |
| A15 | WCH-V1-004 gains the G10 exit: an owner-recorded demotion before the freeze; Go, Vitest and Jest cannot be demoted on this bar | `watch-seven-runners` (WCH-V1-004) |
| A16 | Agents write flows on the freeCodeCamp pin; target 10 flows, 40 accepted links, 20 flow-reaching faults; the acceptances are the owner's act | `fault-injection-harness` (FIH-V1-002), `proof-repositories` (PRR-V1-003), `flow-link-proposals` |
| A17 | Deterministic reverted-fix rule, hashed before any run, first 30 by SHA-256 order | `fault-injection-harness` (FIH-V1-002) |
| A18 | Cost check in two steps (2026-11-16, 2026-11-27); budget 96 host-hours; over budget after the three measures returns to the owner | `fault-injection-harness` (FIH-V1-010), `proof-repositories` |
| B1 | DMF-V1-012 moves into `how-rewrite` as its one output change | `how-rewrite` (HOW-V1-012), `docs-mcp-hardening` |
| B2 | `corvint.docs_recommend_tests` is removed; descriptions point to the testing server | `docs-mcp-hardening` |
| B3 | The docs server verifies and reloads a rebuilt corpus, names the corpus digest in every answer and refuses if verification fails | `docs-mcp-hardening` |
| B4 | The three look-alike docs command families are renamed before 1.0 with one-release aliases, only inside the `flows-status-report` window | `flows-status-report`, `docs-mcp-hardening` |
| B5 | Strict byte binding stays for 1.0 with a clearer stale message; cited-files mode in 1.x | `docs-mcp-hardening` |
| B6 | All user-document items (a) to (k) ship as pages in the repository | `user-docs`; plan rows O30, O64 |
| B7 | Settled by G5: flow reports that join flows with tests are native in `corvint`; the superseding paperwork for decision 0595 is not written | plan row O38 |
| B8 (fix) | The new data-map file's first label stays `/0` | no text (the panel names NCM-V1-007 in `non-code-map`) |
| B9 | The 19 `docs_*` tools are re-described; a tool is pruned only on a proven strict-subset fixture test | `docs-mcp-hardening`, `mcp-tool-table` |
| B10 | The `corvint.docs_how` MCP tool lands with `how-rewrite` | `how-rewrite`, `mcp-tool-table` |
| C1 | `heartbeat` extends the lease | `tasks-hardening` (TKF-V1-004) |
| C2 | The `finish` verb ships in 1.0 | `tasks-hardening` (TKF-V1-005) |
| C3 | Defaults `maxManualShare` 0.5 and `progressStaleDays` 7 | `tasks-hardening` |
| C4 | The `roadmap` default shows open and held tickets by priority; `--all` restores the old output | `tasks-hardening` |
| C5 | Optimistic claim comes after 1.0; the fresh store requires scope | `tasks-hardening` |
| C6 | An expired attempt always needs a new claim in 1.0 | `tasks-hardening` |
| C7 | Settled by G9: the trial binary comes from Tasks main; PR #4 merges first | no text |
| C8 | Profile reads now, record timings, set the budget at the qualification candidate | `tasks-hardening`, `tasks-conformance` |
| C9 | Soak replays count and are reported separately, with at least 20 of the 30 PRs live; trial floor 50 writes and 5 completions per repository | `release-process` (REL-V1-010), `tasks-integration`; PROD-INT-012 notes |
| C10 | The change report matches tickets by commit IDs and by claim scope, each labelled | `change-report-impact`, `tasks-integration` |
| C11 | The fresh-store cutover runs at M0, after the plan and the ticket scheme are accepted | `tasks-store-cutover` (TSC-V1-001) |
| C12 | Seeded tickets keep their V1 IDs; the serial continues from 5263 | `tasks-store-cutover` (TSC-V1-010) |
| C13 | The panel's 18-ticket table stands, including its P1 rows; V1-4382 and V1-4819 are not seeded; the 49 lane-host tickets stay in the archive | `tasks-store-cutover` (TSC-V1-002) |
| C14 | Later tickets live in one generated read-only file in the repository | `tasks-store-cutover` (TSC-V1-017) |
| C15 | The 160 Tasks tickets park in the flat list at cutover; the `beamfall/corvint-tasks` store is initialised within three days of PR #4 merging; starting pool the 23 P1 `v1-0` tickets | `tasks-store-cutover` (TSC-V1-016) |
| C16 | The archive is kept for the whole 1.x line; moved live state until 30 days after stable; two verified off-Mac copies; no cloud upload | `tasks-store-cutover` |
| C17 | Nine held tickets follow the panel table | `tasks-store-cutover` |
| C18 | Identity key built into Tasks (B1 to B3) by 2026-11-27; schema v1 freezes 2026-12-01; interim `key:` label and gate script until then; B4 to B6 best effort | `ticket-taxonomy` (TTX-V1-016) |
| C19 | Bucket rule: `DUPLICATE_KEY` unless `distinctFrom` names every open holder; queue health counts keys with more than one open holder | `ticket-taxonomy` (TTX-V1-015) |
| C20 | A `delivery` ticket and every `v1-0` ticket must name a PROD requirement; `UNPLANNED` only on named `post-1-0` kinds, with a reason | `ticket-taxonomy` |
| C21 | Rig friction files in the Corvint store under component `rig`, milestone fixed `post-1-0` | `ticket-taxonomy` |
| C22 | PR #4 by 2026-11-13; hardening and B1 to B3 by 2026-11-27; Tasks RC and trial start 2026-12-08, end 2026-12-29; publishing the Tasks RC tag is the owner's act | `tasks-conformance`, `tasks-integration`; plan rows M5a, M9 |
| C23 | The orchestrating agent initialises the `beamfall/core` trial store by PR on 2026-12-08; seed reviewed by 2026-12-01; dry run by 2026-12-04; the merge stays the owner's | `tasks-conformance`, `tasks-integration` (TIN-V1-014) |
| D1 (fix) | An unresolvable base runs every test with reason `BASE_UNRESOLVED`; the force-push scenario is added | `github-action` (GHA-V1-003) |
| D2 | Agent qualification pass rule: deterministic steps and hook behaviour pass in every run; model-driven scenarios run three times per host and OS and pass at two of three; rule frozen before the first run | `agent-packages` |
| D3 | Settled by R39 and R40 | no text |
| D4 | One singular noun: `corvint agent doctor`, `agent hook`, `agent manifest`; no alias | `agent-packages` |
| D5 (fix) | Pi registers MCP if its client is confirmed usable, else the CLI-backed tool | `agent-packages` |
| D6 (fix) | GHA-V1-040 to GHA-V1-043 stay in `github-action`; `release-process` cites them | `github-action`, `release-process` (REL-V1-005); plan row O63; section 7 |
| D7 | The macOS exhaustive test and vet are a release gate on the freeze and release candidates; the per-merge macOS check stays advisory until flake tickets close and 20 consecutive main runs pass, and for 1.0 if that is not reached by 2026-12-15 | `release-process` (REL-V1-015); `macos-required-shards` (MRS-V1-002, MRS-V1-004) follows it by owner decision H11 (2026-11-10) |
| D8 | Benchmark hosts are WSL2 on the benchmark box and the owner's Mac, reserved only during declared measurement sessions; a result is NOT_VERIFIED if a lane ran | `release-process` (REL-V1-011); plan row O23 |
| D9 | Settled by G10 and G8; the REL-V1-012 wording becomes "untouched and pinned" | `release-process` (REL-V1-012); plan row O29 |
| D10 | A narrow 1.x compatibility promise covering only contract-tested surfaces; previews and caches excluded by name; one minor release of deprecation notice | `compatibility-promise` (CMP-V1-001 to CMP-V1-006), `user-docs` |
| D10a | `docs/COMPATIBILITY.md` table with a stable or preview column; no label renamed at 1.0 | `compatibility-promise` |
| D11 | `.github/testconfine/`, `greptile-gate.yml`, `merge-queue-cleanup.yml` and `cache-prune.yml` stay for 1.0; `greptile-gate-v0.md` stays active | `source-removal` (RMV-V1-035), `spec-set-cleanup` |
| D12 | Acceptance is the decision file plus a tag signed by the owner with a touch-required key listed in a committed allowed-signers file; fallback if the owner declines a key | `acceptance-record` (ACR-V1-011), `release-process` (REL-V1-007, REL-V1-013); section 8 |
| D13 | Four acts need the owner's key (plan, RC and stable acceptance tags; allowed-signers changes); all else is by convention, identity `NOT_VERIFIED` | `acceptance-record` (ACR-V1-011), `release-process` (REL-V1-013) |
| D14 | `greptile-gate` stays required on main through stable 1.0; it is not a release gate | `release-process` (REL-V1-012) |
| D15 | B11 is a `.github`-only PR; `ci-control-plane.yml` is trimmed and stays required; only `go-product` is replaced by the action's result job | `github-action` (GHA-V1-031), `source-removal` |
| D16 | The held-out draw is never redrawn; build, exhaustive tests and install smokes always rerun; measured classes are reused only by frozen path sets, fail-closed | `release-process` (REL-V1-002, REL-V1-004, REL-V1-014); the same rule as owner decision H3 (2026-11-10), section 9 |
| D17 | Agent hook session state outside the repository is permitted; invariant 4 says so and loses the `--learn-slot-weights` clause | `agent-packages` (AGT-V1-022); the invariant edit: `acceptance-record` (ACR-V1-012, owner decision H5 (2026-11-10)); plan row O35; section 7 |
| E1 | A frozen corpus nothing reads is removed with its last commit recorded in the batch build-log entry; evidence an active spec cites stays | `source-removal` |
| E2 | Settled by G27 and G18: decisions 0224 and 0352 are superseded in the change that lands B11 | plan row O34 |
| E3 | Invariant 3 drops "and learned task traces"; invariant 5 becomes "Corvint 1.0 learns nothing from use" with a rule for later learning | `acceptance-record` (ACR-V1-004; the edits: ACR-V1-012, owner decision H5 (2026-11-10)); section 7 |
| E4 | Settled by R25: no AGPL line on `origin/main` | no text |
| E5 | Settled by G18 and G27; B8b lands by 2026-12-10 or moves to 1.0.x and must not remove `github-split-check` | `release-process` (REL-V1-009), `source-removal` (RMV-V1-001); plan row M6 |
| E6 | Self-use is recorded as a progress note on the change's ticket; friction is filed as tickets; the `docs/DOGFOOD.md` stub stays until re-decided after B6 | `source-removal`, `spec-set-cleanup` |
| E7 | Settled by G25 item 5: no format change; the index is excluded from the compatibility promise | no text |
| E8 | The ten specs are classed per the panel table; `init` and `adopt` are 1.0 first-run verbs; the self-observation ledger stays, documented as local-only | `spec-set-cleanup`, `user-docs`; PROD-INT-017 notes |
| E9 | Settled by G27; the lane group leaves in B8b to a private operator repository; `heavy.sh` stays for 1.0 as unshipped tooling | `source-removal` (RMV-V1-025); plan row M4 |
| E10 (fix) | Proposal headers are the source; the plan's "Implemented by" column is generated; a check fails on mismatch | `openspec-check`; plan row O67 |
| F1 | The plan lands without `PROPOSED-supersede-0594-keeps-line.md`; that file follows with the big cut; this record names a commit containing both. Amended by owner decision H10 (2026-11-10): the record is signed without waiting for the removal branch, and the 0594 row takes effect when that branch lands | `source-removal` (proposal); section 7 |
| F2 (fix) | Bug-fix lanes start now only for findings that break an already-accepted requirement in an `active` spec, in code no removal batch touches, failing test first | no text |
| N1 | New flow-gap codes are additive under `application-flow-gaps/1`; readers ignore unknown codes | `flows-unlinked-tests-report`, `flow-link-proposals` |
| N2 | Flow links store no test kind; kind is read from the inventory by `test_id` | `flows-unlinked-tests-report`, `flow-link-proposals` |
| N3 | The action's adopter-gap split stands as written | `github-action` (design) |
| N4 | Seven public mirrors `beamfall/corvint-qual-<runner>` by 2026-11-27; creating them is the owner's act | `github-action` (GHA-V1-025), `proof-repositories` |
| N5 | No `SubagentStop` hook in 1.0 | `agent-packages` (AGT-V1-022) |
| N6 | No new skill; the migration note points dropped skills at their commands | `agent-packages` |
| N7, N8, N9, N10 | Settled by A12, A7, B2 and B1 | as those rows |
| N11 | Test milestones after B6b and B8a | `source-removal` |
| M1 | MCP `affected` on a stale binary returns select-all with reason `self-binary-stale`; the CLI keeps exit 2 | `mcp-tool-table` (MTT-V1-011) |
| M2, M3, M9, M10 | Four tickets (linecov record regeneration P1; M4 landing checklist P1; two P3 optimizations), filed at the fresh-store seed | no text |
| M4 | The V1-5183 oracle run is a line of the M4 checklist; V1-5216 AC4 keys are listed only after each failure is confirmed infrastructure | no text |
| M5 | The nightly `release-gates.yml` run holds exactly the release gates | `release-process` (REL-V1-012) |
| M6 | Use-case receipts are superseded and removed in B8 | `acceptance-record` (ACR-V1-004), `source-removal`; section 7 |
| M7 | The archive install smoke for the documentation product is a release gate | `release-process` (REL-V1-012) |
| M8 | Dropped: covered by R18 | no text |
| M11 | Shell provider status | `runner-contract` (RNC-V1-018) |
| M12 | The MCP tool table drops `test_validity` | `mcp-tool-table` (MTT-V1-005) |

### 4. Drafted wording this record accepts

These requirement texts were written for the plan, not quoted from an owner answer (ACR-V1-002).
Accepting this record accepts their wording as it stands in the plan at the signed SHA:

- PROD-INT-003 (one capability table for every command and MCP tool) and PROD-INT-007 (1.0 wire
  formats and commands frozen), checklist row O60, owner decision R1 (2026-11-09).
- PROD-TASK-001 to PROD-TASK-006, including the source of PROD-TASK-006, checklist row O61, and
  PROD-TASK-007 to PROD-TASK-009, confirmed by owner decision R17 (2026-11-09).
- Every other plan requirement marked "(proposed 2026-11-09)" that the acceptance log lists with
  mode `owner` at signing. A requirement the log lists as `pending-owner` is not accepted by this
  record.

### 5. In-flight work stays experimental

Work in flight on `claude/layers-integ` and in open change folders continues as experimental
(owner decision R1 (2026-11-09); checklist row O16). Nothing may be described as delivered, and no
experimental work may be promoted or advertised, before this record is accepted and that work's own
change is accepted or auto-accepted under G14.

### 6. Release exception (owner decision R9 (2026-11-09))

pytest selects by file, not by line, in 1.0, and says so with its tier label (R8). This is a
recorded release exception to the line-exact bar. The Python line collector is a 1.0.x ticket. The
exception does not weaken the no-miss rule: a file-level selection still runs every test in each
selected file.

### 7. Superseded text

Quotes of files in this repository are copied from this branch at `d5bc186b2` (ACR-V1-004), and
each can be checked there. An earlier draft pinned `007bace5d`, which is not an ancestor of this
branch's head. A quote that spans a source line break is given with the break replaced by one
space. Three kinds of row cannot be audited from this repository at that commit:

- Owner answer 3, owner answer 11, the G9 MCP row and R26 are quoted from the owner-answers file
  of 2026-11-09, which is not tracked in this repository.
- Decision 0594 and `docs/decisions/PROPOSED-supersede-0594-keeps-line.md` are not on this branch.
  By owner decision H10 (2026-11-10) this record is signed without waiting for the removal
  branch; the 0594 row takes effect when that branch lands both files (R42, F1). Its quote is
  carried from the removal branch and is not checkable here.
- The "new wording" in the `AGENTS.md` rows is proposed text. No file holds it yet. Every edit of
  invariants 3, 4, 5, 7 and 8 is made by the change `acceptance-record` (ACR-V1-012, owner
  decision H5 (2026-11-10)). By owner decision H4 (2026-11-10) the invariant 7 rewording is
  accepted and invariant 8 is not reworded: the proposed rewording dropped "human-owned intent".
  A later rewording of invariant 8 must keep those words and go to the owner.

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
| `AGENTS.md`, section "Dogfood Corvint" | "Substantive Corvint work must follow `docs/DOGFOOD.md`" and "For substantive work, at change start run `make dogfood-change BASE=<sha>`." | G8: the ceremony is replaced fully. R42, E6: self-use is recorded as a progress note on the change's ticket and friction is filed as tickets; the `docs/DOGFOOD.md` stub stays until re-decided after B6. The edit is made by the change `spec-set-cleanup` | removed |
| Use-case receipt contract, `docs/specs/use-case-conformance-v0.md` (governing spec found by search; the B8 lane confirms it) | "Headline Corvint jobs remain UNPROVEN until each has complete content-addressed promotion evidence." | R42, M6: "The use-case receipt contract is superseded by the per-product end-to-end qualification of REL-V1-012; `conformance/use-cases-v0` is removed in B8 together with its two lints, and no 1.0 claim cites a use-case receipt." | superseded |
| Home of GHA-V1-040 to GHA-V1-043, `docs/specs/openspec/changes/github-action/proposal.md` | "records whether they stay here or move to `docs/specs/public-release-v0.md`" | R42, D6: the change `github-action` is their home and their IDs do not change; `release-process` cites them (REL-V1-005) | replaced |
| `AGENTS.md`, product invariant 3 | "Project-owned authority outranks syntax, history, and learned task traces." | R42, E3. New wording: "Project-owned authority outranks syntax and history." | amended |
| `AGENTS.md`, product invariant 4 | "nor to learning except through the operator-invoked `corvint eval --learn-slot-weights` step, whose output reaches ranking only after the frozen held-out gate admits it (`LTA-V0-009` to `LTA-V0-012`)" | R42, D17: the clause is dropped, and the invariant gains the sentence AGT-V1-022 needs: agent hook session state kept in the user cache directory is not repository or trace state | amended |
| `AGENTS.md`, product invariant 5 | "Learning is explicit, local, bounded, secret-screened, and evaluation-gated." | R42, E3. New wording: "Corvint 1.0 learns nothing from use. Any later learning must be explicit, local, bounded, secret-screened and evaluation-gated under its own decision." | replaced |
| `AGENTS.md`, product invariant 7 | "see `docs/specs/deployment-neutral-index-platform-v0.md`, `docs/specs/go-production-kernel-migration-v0.md` (hot-path language moves need exact parity and staged rollback), decision 0081 / `docs/specs/local-admin-console-v0.md`" | G22 and owner decision H4 (2026-11-10): the citations of active documents stay and those of archived documents are dropped; the edit is ACR-V1-012 (H5) | amended |
| Decision 0385, Classification, Core line (`AFU-V1-019..024`) | "the `e2e-safe` value of `corvint affected --selection-profile` (`AFU-V1-019..024`)" | R42, A4: the `e2e-safe` profile and the appmap navigation verdict leave the public surface in 1.0 (RMV-V1-037; plan row O66) | superseded |

The pre-cut 1.0 contract rows (PRS-V1-001 to PRS-V1-018) are resolved as the plan's "Conflicts with
accepted text" table says, with PRS-V1-018 (c) and (d) carried over by R37.

### 8. Owner acts

R42 keeps publishing, stable promotion, credentials, repository creation and repository visibility
with the owner. This table lists each act the plan or a change waits on. "NOT_DECIDED" means no
owner answer, panel pick or requirement records the value; this record does not supply one.

| Act | Due | Milestone that waits | Fallback if late |
|---|---|---|---|
| Create the touch-required signing key, confirm its fingerprint, and sign the plan acceptance tag (R42, D12 and D13; ACR-V1-011) | NOT_DECIDED (before M0) | M0: the fresh-store cutover and the ticket re-scope run only after the plan and the ticket scheme are signed (R42, C11; TSC-V1-001) | If the owner declines a key: an owner-merged decision file plus a protected-environment approval, with identity by convention and `NOT_VERIFIED` (ACR-V1-011, REL-V1-013). If the act is only late: NOT_DECIDED |
| Sign the RC and stable acceptance tags and give the two environment approvals (REL-V1-007, REL-V1-013) | At M8 and at M10 | M8, M10 | The same decline fallback; late: NOT_DECIDED |
| Create the seven public mirrors `beamfall/corvint-qual-<runner>` (R42, N4; GHA-V1-025) | 2026-11-27 | The starter acceptance of `github-action`, which stays `NOT_RUN` and names the missing mirror | NOT_DECIDED |
| Review and sign the flow links for about 10 flows on the freeCodeCamp pin (R42, A16; PRR-V1-003, FIH-V1-002) | 2026-12-04 | Sealing the Playwright fault set, before M5a and M7 | M5a slips until the links are signed; Playwright is not demoted (owner decision H6 (2026-11-10)) |
| Publish the Tasks RC tag (R42, C22) | 2026-12-08 | M5a and the start of the M9 trial | NOT_DECIDED; `tasks-conformance` names 2026-12-15 as the latest trial start that still meets a 2027-01-05 stable |
| Merge the trial-store PR in `beamfall/core` (R42, C23; TIN-V1-014) | 2026-12-08 | The M9 trial | The trial cannot start and a blocker is filed (`tasks-conformance` design); beyond that NOT_DECIDED |
| Make the one-line global-instruction edit once the `beamfall/corvint-tasks` store exists (R42, C15; TSC-V1-016 initialises the store within three days of PR #4 merging) | NOT_DECIDED | None named | NOT_DECIDED |
| Every later publish and each stable promotion (R42, C22; REL-V1-007) | At M8 and at M10 | M8, M10 | NOT_DECIDED |
| npm publisher account and trusted-publisher setup for the OpenCode and Pi packages (AGT-V1-007; no npm scope is named in the plan or a change) | NOT_DECIDED | The first `corvint-agents` release of the preview packages | NOT_DECIDED |
| Create the private operator repository that B8b moves the lane group into (R42, E9; RMV-V1-025) | 2026-12-10 | B8b, after M4 | B8b moves to the first 1.0.x (R42, E5; REL-V1-009) |
| Bind the protected release environment to protected `v*` tags (REL-V1-007; GHA-V1-041, GHA-V1-042) and make the ruleset required-check changes that need owner rights (RMV-V1-002) | Before M8; with each `.github` batch | M2, M8 | NOT_DECIDED |
| Create `beamfall/corvint-action` (G17), `beamfall/corvint-agents` (G20) and `beamfall/corvint-vscode` (R33), and set their visibility | NOT_DECIDED | `corvint-action`: the shadow run and the M5 cut-over. `corvint-agents`: the M7 agent qualification. `corvint-vscode`: no 1.0 milestone | `corvint-action`: the CI scripts stay for 1.0 if the action has not matched them by 2026-12-15, and the cut-over is the first 1.0.x change (G18). The other two: NOT_DECIDED |
| Conditional acts: record a runner demotion before the freeze (R42, A15); decide a fault-run budget overrun (R42, A18). No replacement held-out repository is named: the drawn repositories are never redrawn (owner decision H3 (2026-11-10); REL-V1-014) | When the condition arises | The freeze (M6); the fault runs (M7) | NOT_DECIDED |

### 9. Carried open, settled 2026-11-10

At drafting, the items below had no owner answer or panel pick. On 2026-11-10 the owner answered
each (H1 to H11). Each row gives the answer and where it is applied. Two points the closing review
raised were never in this table because a pick already decides them: the soak's live-PR floor
(R42, C9; now in REL-V1-010) and the plan checklist rows O23, O29, O33, O34, O38 and O51 (marked
settled, each by the pick it names).

| Decision | Owner's answer | Where it is applied |
|---|---|---|
| H1. Stable clock | Stable is not published before the latest of: (a) 2027-01-05; (b) the end of the 21-day Tasks RC trial, 2026-12-08 to 2026-12-29, unless it restarts under TCF-V1-009 on a store format change; (c) the Corvint RC publish plus 21 days; (d) the end of the 7-day soak. A Corvint RC code change restarts the soak and the RC-publish clock; the Tasks trial restarts only under TCF-V1-009. A late batch (B8b, B11) restarts a clock only if it changes a shipped binary | REL-V1-003 and the `release-process` design; TCF-V1-009; plan "The 1.0 bar", PROD-INT-012, PROD-INT-013 and milestone row M9 |
| H2. Regression on the RC | A regression found on the RC blocks, for each of the seven blocking runners; it never becomes a demotion of that runner | REL-V1-004; NMS-V1-007; plan PROD-INT-008 |
| H3. Held-out repositories | One held-out repository per family (Go, JS/TS, Python, Ruby), drawn from a sealed, hashed pool of at least five candidates per family, checked against the held-out rule and the untouched search, sealed before the first candidate tag is pushed. The seed is the commit SHA of that tag; the draw is deterministic and never redrawn. fastapi, solidus, payload, helm and go-task stop being a fixed list and may be pool candidates. Fault sets for the drawn repositories are built between the candidate tag and the freeze | PRR-V1-004, PRR-V1-011; REL-V1-002, REL-V1-014; `narrowing-measurement`; section 2 (R21), section 3 (A12, A14, D16), section 8; plan PROD-INT-008, PROD-TEST-015, checklist rows 11 and O13, milestone row M5a |
| H4. Invariants 7 and 8 | The invariant 7 rewording is accepted: citations of active documents stay, archived ones are dropped. The invariant 8 rewording is rejected because it drops "human-owned intent"; invariant 8 is not reworded, and a later rewording keeps those words and goes to the owner | ACR-V1-012; section 7; plan G8 removal row and "Corrections" item 3 |
| H5. Who edits the `AGENTS.md` invariants | Every edit of invariants 3, 4, 5, 7 and 8 is made by the change `acceptance-record` | ACR-V1-012; section 3 (D17, E3); section 7; plan checklist row O35 |
| H6. Flow links late | If the flow links miss 2026-12-04, M5a slips until they are signed; Playwright is not demoted | Section 8, flow-links row |
| H7. M4 and M5a order | M4 lands before the M5a tag | Plan milestone rows M4 and M5a |
| H8. `impact --base` default | With `--base` omitted, `impact` uses the merge base of HEAD with the default branch, reports the base it chose, and refuses when it cannot determine one. The recommendation the owner took was marked low confidence by its author | CRI-V1-001 and the `change-report-impact` design; plan PROD-INT-028 |
| H9. Status `external-spec-not-verified` | An accepted status of the openspec check: reported, not failing | OSC-V1-005 |
| H10. Signing and the removal branch | The record is signed without waiting for the removal branch; the row superseding decision 0594's Keeps line takes effect when that branch lands | Section 3 (F1); section 7 |
| H11. macOS checks | MRS-V1-002 and MRS-V1-004 follow D7 and REL-V1-015. The per-merge macOS check stays advisory until the flake tickets are closed and 20 consecutive main runs pass, then it is made required; if that has not happened by 2026-12-15 it stays advisory per merge for 1.0. macOS is required at release. MRS-V1-004 keeps filing a P1 ticket per flake; its own rule against RC approval while such a ticket is open gives way to the release gate and the general open-P1 rule | MRS-V1-002, MRS-V1-004; section 3 (D7); plan dependency row `macos-required-shards` and PROD-INT-015 |

## Rollback

Before acceptance: delete this file; nothing else depends on it. After acceptance: a decision
record is never deleted (ACR-V1-001); a new decision record supersedes this one, names what it
restores, and the acceptance log rows it changes go back to `pending-owner`.

## What the owner decides

1. Accept, amend or reject sections 1 to 7, including each delegated pick of section 3.
2. The plan commit SHA this record names.
3. The acts of section 8, and a value for each `NOT_DECIDED` cell the owner wants fixed.
4. Confirm the reading in section 1 (H12) that the canonical specs stay as the flat
   `docs/specs/*.md` files, so `openspec archive` runs with `--skip-specs`. Section 9 lists the
   owner's answers of 2026-11-10 (H1 to H11); it leaves nothing open.
