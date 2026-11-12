# Change proposals

This directory holds one folder per change under `changes/`. It is an index; the product contract
is `docs/specs/PRODUCT-PLAN-1.0.md`.

Acceptance state (V1-5296): the one acceptance record exists. Decision 0596 (owner decision H15,
2026-11-10) accepted the plan, and `acceptance-log.tsv` holds an `owner` row for each of its 77
PROD requirements and 67 checklist O-rows (owner decision R1 (2026-11-09); checklist O19). The log,
not this README, is the authority for each change folder, with one `change` row per folder
(ACR-V1-013). Under G14 a proposal that only implements an accepted plan requirement is
auto-accepted and logged `auto`; a proposal that changes behaviour, a file or wire format, or the
release bar waits for the owner. No folder has been classified under G14 yet, so one folder,
`acceptance-record`, is `auto` (under the standing auto-accept of 2026-10-11, V1-5272) and every
other folder is logged `pending-owner`: it waits for the owner, its proposal header says
`proposed`, and its work stays `experimental`. A lane never writes an `owner` row (ACR-V1-008).
`script/acceptance-log-audit.sh` names a folder with no row, a header or an index row that
disagrees with it (ACR-V1-014, ACR-V1-015). The plan file's own header still calls the plan a
proposal; that text is pinned by decision 0596 (`script/acceptance-plan-pin-check.sh`) and only
the owner can correct it.

## Layout and stages

- Layout (G14, amended by owner decision H12 (2026-11-10)): each change is a folder
  `changes/<name>/` with `proposal.md`, `design.md`, `tasks.md` and `specs/<capability>/spec.md`.
  The folder name is the change name.
- This directory is an OpenSpec root: `config.yaml` (schema `spec-driven`) sits beside `changes/`.
  The OpenSpec command (`@fission-ai/openspec`, pinned at 1.17.0) is run from `docs/specs`, with
  telemetry off (`OPENSPEC_TELEMETRY=0`). The repository root holds no `openspec/` directory. A
  change folder may hold the `.openspec.yaml` file that `openspec new change` writes.
- Stages (owner decision R41 (2026-11-09) and owner decision H12 (2026-11-10)): every change passes
  through propose, apply and archive.
  - propose: `openspec new change <name>` creates the folder; the four artifacts are written;
    `openspec validate <name>` passes and `openspec status --change <name>` shows every artifact
    done; the folder is committed before any code for the change;
  - apply: the lane ticks the `- [ ]` items of `tasks.md` in order (`openspec list` shows the
    count) and copies each requirement into the owning capability spec under `docs/specs/` in the
    same change as the code;
  - archive: when every task is ticked, `openspec archive <name> --skip-specs` moves the folder to
    `changes/archive/YYYY-MM-DD-<name>/`. An archived folder is not edited, and `--yes` is never
    used to pass over an unticked task.
- One difference from stock OpenSpec, kept from G14 and confirmed by owner decision H13
  (2026-11-10): the canonical specs are the flat files
  `docs/specs/*.md`, not `openspec/specs/<capability>/spec.md`. That is why archive runs with
  `--skip-specs`, and why strict validation notes that the target spec of a MODIFIED requirement
  does not exist.
- Bar: `openspec validate --all` passes for every change folder. Warnings from `--strict` are
  reported and do not fail. On 2026-11-10 strict mode gave 188 warnings in 41 of the 45 folders: 152
  for requirement text over 500 characters, 21 for text above the first section and 15 for text
  before the first requirement.
- `make openspec-check` (`tools/openspec-check`, part of `make doc-gates`) checks the layout, IDs,
  stages, the one root and the spec `Health:` lines (OSC-V1-001 to OSC-V1-011 in
  `docs/specs/openspec-check-v1.md`); `OPENSPEC_BASE=<rev>` adds the commit rules for commits that
  opt in to a change, and the findings listed in `tools/openspec-check/baseline.txt` are reported
  only. `make openspec-validate` runs the pinned
  command (OSC-V1-012) and reports NOT_RUN where it is not installed.
- Each proposal header names its change, ticket, the plan requirements it serves, the owning
  capability spec, the requirement IDs it adds, its intent status and its delivery status. IDs are
  reserved before the lane's first commit and are never reused.
- Tickets stay in `.taskman/`. A proposal names its ticket and holds no queue of its own.
- The spec generators read only `docs/specs/*.md`; a proposal does not appear in
  `REQUIREMENTS.tsv` or `INDEX.json`.
- `changes/spec-set-cleanup/` gives every older process document and spec a verdict
  (keep, rewrite, archive, delete or move) and leaves one workflow document for propose, apply and
  archive. Until it lands, `docs/decisions/PROPOSED-openspec-workflow.md` and
  `docs/SPEC-DRIVEN-DEVELOPMENT.md` stay as they are and do not override this layout.

Order and prerequisites: "Proposal dependency table" in the plan. Plan requirements that name each
proposal, and differences from the proposal headers: "Proposal traceability" in the plan
(checklist O31).

## Index

The Status column is the folder's acceptance-log row: "waiting for owner" for `pending-owner`
(the proposal's intent is proposed only), "auto-accepted" for `auto` and "accepted" for `owner`
(ACR-V1-015). No folder is planned without a `proposal.md` now. Status "archived" means every task
is ticked and the folder is under `changes/archive/`. The table has one row for each of the 50
folders under `changes/` and for each archived change. The PROD column gives the plan requirements
the proposal header serves.

| Change | Purpose | PROD requirements | Status |
|---|---|---|---|
| `acceptance-record` | The one decision record that accepts the plan as amended, including the Python file-level release exception (R1, R9) | checklist O19 | auto-accepted |
| `affected-change-status` | Base change status as a plan input, so name-listing shell readers stop selecting on edits (V1-5193, V1-5194) | PROD-TEST-001 | waiting for owner |
| `affected-plan-kinds` | Test kind and kind source in the `affected` plan, reporting only (G13) | PROD-TEST-003, 004 | waiting for owner |
| `agent-packages` | Claude Code and Codex packages (blocking) and OpenCode and Pi packages (preview) in `beamfall/corvint-agents` | PROD-TEST-009, PROD-DOCS-001, PROD-TASK-007; PROD-INT-023 to 027 | waiting for owner |
| `canonical-test-id` | One canonical test ID for every runner (RNC-V1-012, fresh IDs per R11) | PROD-TEST-006 | waiting for owner |
| `change-report-impact` | Impact section of the change report | PROD-TEST-003, 010; PROD-INT-001, 002 | waiting for owner |
| `compatibility-promise` | The 1.x compatibility promise: `docs/COMPATIBILITY.md` table of stable and preview surfaces (R42 D10, D10a) | plan gap (g); checklist O30, O64 | waiting for owner |
| `context-proven-only` | Cut the unproven context surfaces (task packet, features, overview, ranking, traces, context benches) and keep the proven ones; re-home the instruction doctor; reduce the path brief (decision 0597) | decision 0594 item 1; PROD-TASK-006 | archived 2026-11-11 (`changes/archive/2026-11-11-context-proven-only/`) |
| `deadcode-ratchet` | A pinned dead-function report with a shrinking baseline, then deletion of what it lists (V1-4801, V1-4827) | none named (owner decision R33, checklist O42) | waiting for owner |
| `docs-mcp-hardening` | Honest docs checks and usable docs MCP tools | PROD-DOCS-001, 003, 005, 007, 009 | waiting for owner |
| `e2e-inventory-discovery` | Discovery of browser E2E tests into the inventory | PROD-TEST-004 | waiting for owner |
| `fault-injection-harness` | Planted faults on the held-out repositories to test the no-missed-test claim (R19) | PROD-TEST-002, 011; PROD-INT-008 | waiting for owner |
| `feature-map-from-flows` | Feature map generated from `.corvint/flows.json` (preview) | PROD-DOCS-004, 006, 011 | waiting for owner |
| `flow-link-proposals` | Proposed links between tests and flows with a confidence score, accepted by a human only (R12) | PROD-DOCS-012; PROD-TEST-018 | waiting for owner |
| `flow-schema-version` | Schema version for the flow file | PROD-INT-016 | waiting for owner |
| `flows-status-report` | `corvint flows status`, joining run results to flows | PROD-TEST-010 | waiting for owner |
| `flows-unlinked-tests-report` | Report of E2E tests with no flow link; those tests always run | PROD-TEST-005 | waiting for owner |
| `github-action` | The GitHub Action in `beamfall/corvint-action` (R22 to R25) | PROD-TEST-012; PROD-INT-006 | waiting for owner |
| `how-rewrite` | Rewrite of `corvint how` | PROD-DOCS-007; PROD-INT-004 | waiting for owner |
| `jest-provider` | Jest runner provider, line-exact (R7, R8) | PROD-TEST-006, 008, 011 | waiting for owner |
| `line-evidence` | Line-level evidence behind line-exact selection (R8, R9) | PROD-TEST-008 | waiting for owner |
| `hub-read-scopes` | Declared read scopes for the hub test packages, with whole-repository tests moved out (V1-5201, V1-5234) | PROD-TEST-001, 002 | waiting for owner |
| `macos-required-shards` | Required macOS checks on the release candidate | PROD-INT-015 | waiting for owner |
| `mcp-tool-table` | One table of MCP tool names and schemas; one `corvint-docs mcp` server (R13, R30) | PROD-DOCS-001, 010; PROD-TEST-009 | waiting for owner |
| `narrowing-measurement` | Narrowing measurement, the tiered bar and the `CORVINT_SELECTION` kill switch (R13, R16) | PROD-TEST-002, 008, 011; PROD-INT-007, 008 | waiting for owner |
| `non-code-map` | Non-code files run everything by default; proven map entries narrow | PROD-TEST-002, 008, 012 | waiting for owner |
| `openspec-check` | Go check of the change folders and of the propose, apply and archive stages, and validation by the OpenSpec command (R41, H12) | PROD-INT-011, 010 | waiting for owner |
| `per-test-durations` | Per-test durations in every runner's receipt (R19) | PROD-TEST-016 | waiting for owner |
| `proof-repositories` | Pinned public repositories per runner; held-out repositories drawn by seed from sealed pools (R21, H3) | PROD-TEST-002, 011; PROD-INT-008 | waiting for owner |
| `pytest-provider` | pytest runner provider, file-level selection with a tier label (R8, R9) | PROD-TEST-006, 008, 011 | waiting for owner |
| `release-process` | Milestones M0 and M5a, tag-started release and owner approval (R2, R3) | PROD-INT-012, 013, 017 | waiting for owner |
| `review-evidence-check` | Review evidence check behind `corvint review` (advisory by default, R4) | PROD-TEST-010, 012; PROD-INT-001 | waiting for owner |
| `ruby-providers` | RSpec and Minitest runner providers, line-exact (R8) | PROD-TEST-006, 008, 011 | waiting for owner |
| `run-evidence` | Append-only record of test run results (R7) | PROD-TEST-020, 010 | waiting for owner |
| `runner-contract` | Built-in runner contract for the seven blocking runners | PROD-TEST-006, 011 | waiting for owner |
| `selection-hardening` | Close Python and JavaScript selection misses and give one verdict per plan | PROD-TEST-001, 002, 009, 014, 015, 016 | accepted (G14 auto, 2026-11-11; V1-5401); in delivery |
| `source-removal` | Removal of the code, commands and files cut from 1.0, in batches (R34, R36) | PROD-TEST-002; PROD-INT-009, 010 | waiting for owner |
| `spec-set-cleanup` | A verdict for every older spec and process document, archived, rewritten or removed with the removal batches; one workflow document; done by 2026-12-01 (R37, R41). Needs `openspec-check` and `source-removal` first | PROD-INT-010, 011 | waiting for owner |
| `tasks-conformance` | Conformance of the Tasks repository and the 21-day trial on `beamfall/core` (R17, R18); written in the Tasks repository | PROD-TASK-010 | waiting for owner |
| `tasks-hardening` | `corvint-tasks` lease, read and status corrections before 1.0; built in the Tasks repository | PROD-TASK-002, 003, 005, 007 | waiting for owner |
| `tasks-integration` | Integration between Corvint and Tasks by files and IDs | PROD-TASK-003, 004, 006 to 013; PROD-INT-004 | waiting for owner |
| `tasks-policy-v15` | Tasks queue policy v15 without `cemRequired`, `ocmRequired`, `cem-verdict` and `context-focused` (R3) | milestone M0 | waiting for owner |
| `tasks-store-cutover` | Cut the Corvint task store over to a fresh store of 1.0 tickets (R42, C11 to C17) | milestone M0 | waiting for owner |
| `test-inventory` | One committed inventory under `.corvint/` for all seven runners, written by `corvint inventory refresh` (R10) | PROD-TEST-017 | waiting for owner |
| `ticket-brief-context` | Ticket brief through `corvint context`, non-blocking (R28, R29) | PROD-TASK-006; PROD-INT-002 | waiting for owner |
| `ticket-taxonomy` | Ticket categories, identity key, required fields and search; built in the Tasks repository (R42, C18 to C21) | PROD-TASK-005, 006, 007; PROD-INT-005, 011 | waiting for owner |
| `traceability-gate-upgrade` | The traceability gate resolves `TestX/REQ-ID` citations against the test body and refuses the removal of a requirement's last test citation (V1-4803) | PROD-INT-005 | waiting for owner |
| `trusted-shell-selection` | CI shell-test selection and the doc-gates target list come from the base tree, so a PR cannot weaken its own checks (V1-5097) | none named (V1-5097 acceptance criteria) | waiting for owner |
| `user-docs` | User documents: first run, configuration, upgrade, errors, release notes and policy statements (product gaps) | PROD-INT-004, 006, 015, 017, 018 | waiting for owner |
| `watch-flows-hardening` | Watch survives planning errors; flow-linked E2E selection leaves no linked test out | PROD-TEST-002, 003, 004, 005, 007 | waiting for owner |
| `watch-seven-runners` | Watch mode for the seven blocking runners | PROD-TEST-007 | waiting for owner |
