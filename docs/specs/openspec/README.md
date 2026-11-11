# Change proposals

This directory holds one folder per change under `changes/`. It is an index; the product contract
is `docs/specs/PRODUCT-PLAN-1.0.md`, which is itself a proposal.

Nothing here is accepted. Every folder is proposed until the one acceptance record exists (owner
decision R1 (2026-11-09); checklist O19 in the plan). A proposal that only implements an accepted
plan requirement is then auto-accepted and logged; a proposal that changes behaviour, a file or
wire format, or the release bar waits for the owner (G14). Unaccepted work stays `experimental`.

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

Status "proposed" means the folder has a `proposal.md` and its intent is proposed only. Status
"planned" means the plan names the change and no `proposal.md` is written yet; no folder is planned
now. The table has one row for each of the 48 folders under `changes/`. The PROD column gives the
plan requirements the proposal header serves.

| Change | Purpose | PROD requirements | Status |
|---|---|---|---|
| `acceptance-record` | The one decision record that accepts the plan as amended, including the Python file-level release exception (R1, R9) | checklist O19 | proposed |
| `affected-plan-kinds` | Test kind and kind source in the `affected` plan, reporting only (G13) | PROD-TEST-003, 004 | proposed |
| `agent-packages` | Claude Code and Codex packages (blocking) and OpenCode and Pi packages (preview) in `beamfall/corvint-agents` | PROD-TEST-009, PROD-DOCS-001, PROD-TASK-007; PROD-INT-023 to 027 | proposed |
| `canonical-test-id` | One canonical test ID for every runner (RNC-V1-012, fresh IDs per R11) | PROD-TEST-006 | proposed |
| `change-report-impact` | Impact section of the change report | PROD-TEST-003, 010; PROD-INT-001, 002 | proposed |
| `compatibility-promise` | The 1.x compatibility promise: `docs/COMPATIBILITY.md` table of stable and preview surfaces (R42 D10, D10a) | plan gap (g); checklist O30, O64 | proposed |
| `docs-mcp-hardening` | Honest docs checks and usable docs MCP tools | PROD-DOCS-001, 003, 005, 007, 009 | proposed |
| `e2e-inventory-discovery` | Discovery of browser E2E tests into the inventory | PROD-TEST-004 | proposed |
| `fault-injection-harness` | Planted faults on the held-out repositories to test the no-missed-test claim (R19) | PROD-TEST-002, 011; PROD-INT-008 | proposed |
| `feature-map-from-flows` | Feature map generated from `.corvint/flows.json` (preview) | PROD-DOCS-004, 006, 011 | proposed |
| `flow-link-proposals` | Proposed links between tests and flows with a confidence score, accepted by a human only (R12) | PROD-DOCS-012; PROD-TEST-018 | proposed |
| `flow-schema-version` | Schema version for the flow file | PROD-INT-016 | proposed |
| `flows-status-report` | `corvint flows status`, joining run results to flows | PROD-TEST-010 | proposed |
| `flows-unlinked-tests-report` | Report of E2E tests with no flow link; those tests always run | PROD-TEST-005 | proposed |
| `github-action` | The GitHub Action in `beamfall/corvint-action` (R22 to R25) | PROD-TEST-012; PROD-INT-006 | proposed |
| `how-rewrite` | Rewrite of `corvint how` | PROD-DOCS-007; PROD-INT-004 | proposed |
| `jest-provider` | Jest runner provider, line-exact (R7, R8) | PROD-TEST-006, 008, 011 | proposed |
| `line-evidence` | Line-level evidence behind line-exact selection (R8, R9) | PROD-TEST-008 | proposed |
| `hub-read-scopes` | Declared read scopes for the hub test packages, with whole-repository tests moved out (V1-5201, V1-5234) | PROD-TEST-001, 002 | proposed |
| `macos-required-shards` | Required macOS checks on the release candidate | PROD-INT-015 | proposed |
| `mcp-tool-table` | One table of MCP tool names and schemas; one `corvint-docs mcp` server (R13, R30) | PROD-DOCS-001, 010; PROD-TEST-009 | proposed |
| `narrowing-measurement` | Narrowing measurement, the tiered bar and the `CORVINT_SELECTION` kill switch (R13, R16) | PROD-TEST-002, 008, 011; PROD-INT-007, 008 | proposed |
| `non-code-map` | Non-code files run everything by default; proven map entries narrow | PROD-TEST-002, 008, 012 | proposed |
| `openspec-check` | Go check of the change folders and of the propose, apply and archive stages, and validation by the OpenSpec command (R41, H12) | PROD-INT-011, 010 | proposed |
| `per-test-durations` | Per-test durations in every runner's receipt (R19) | PROD-TEST-016 | proposed |
| `proof-repositories` | Pinned public repositories per runner; held-out repositories drawn by seed from sealed pools (R21, H3) | PROD-TEST-002, 011; PROD-INT-008 | proposed |
| `pytest-provider` | pytest runner provider, file-level selection with a tier label (R8, R9) | PROD-TEST-006, 008, 011 | proposed |
| `release-process` | Milestones M0 and M5a, tag-started release and owner approval (R2, R3) | PROD-INT-012, 013, 017 | proposed |
| `review-evidence-check` | Review evidence check behind `corvint review` (advisory by default, R4) | PROD-TEST-010, 012; PROD-INT-001 | proposed |
| `ruby-providers` | RSpec and Minitest runner providers, line-exact (R8) | PROD-TEST-006, 008, 011 | proposed |
| `run-evidence` | Append-only record of test run results (R7) | PROD-TEST-020, 010 | proposed |
| `runner-contract` | Built-in runner contract for the seven blocking runners | PROD-TEST-006, 011 | proposed |
| `selection-hardening` | Close Python and JavaScript selection misses and give one verdict per plan | PROD-TEST-001, 002, 009, 014, 015, 016 | accepted (G14 auto, 2026-11-11; V1-5401); in delivery |
| `source-removal` | Removal of the code, commands and files cut from 1.0, in batches (R34, R36) | PROD-TEST-002; PROD-INT-009, 010 | proposed |
| `spec-set-cleanup` | A verdict for every older spec and process document, archived, rewritten or removed with the removal batches; one workflow document; done by 2026-12-01 (R37, R41). Needs `openspec-check` and `source-removal` first | PROD-INT-010, 011 | proposed |
| `tasks-conformance` | Conformance of the Tasks repository and the 21-day trial on `beamfall/core` (R17, R18); written in the Tasks repository | PROD-TASK-010 | proposed |
| `tasks-hardening` | `corvint-tasks` lease, read and status corrections before 1.0; built in the Tasks repository | PROD-TASK-002, 003, 005, 007 | proposed |
| `tasks-integration` | Integration between Corvint and Tasks by files and IDs | PROD-TASK-003, 004, 006 to 013; PROD-INT-004 | proposed |
| `tasks-policy-v15` | Tasks queue policy v15 without `cemRequired`, `ocmRequired`, `cem-verdict` and `context-focused` (R3) | milestone M0 | proposed |
| `tasks-store-cutover` | Cut the Corvint task store over to a fresh store of 1.0 tickets (R42, C11 to C17) | milestone M0 | proposed |
| `test-inventory` | One committed inventory under `.corvint/` for all seven runners, written by `corvint inventory refresh` (R10) | PROD-TEST-017 | proposed |
| `ticket-brief-context` | Ticket brief through `corvint context`, non-blocking (R28, R29) | PROD-TASK-006; PROD-INT-002 | proposed |
| `ticket-taxonomy` | Ticket categories, identity key, required fields and search; built in the Tasks repository (R42, C18 to C21) | PROD-TASK-005, 006, 007; PROD-INT-005, 011 | proposed |
| `traceability-gate-upgrade` | The traceability gate resolves `TestX/REQ-ID` citations against the test body and refuses the removal of a requirement's last test citation (V1-4803) | PROD-INT-005 | proposed |
| `trusted-shell-selection` | CI shell-test selection and the doc-gates target list come from the base tree, so a PR cannot weaken its own checks (V1-5097) | none named (V1-5097 acceptance criteria) | proposed |
| `user-docs` | User documents: first run, configuration, upgrade, errors, release notes and policy statements (product gaps) | PROD-INT-004, 006, 015, 017, 018 | proposed |
| `watch-flows-hardening` | Watch survives planning errors; flow-linked E2E selection leaves no linked test out | PROD-TEST-002, 003, 004, 005, 007 | proposed |
| `watch-seven-runners` | Watch mode for the seven blocking runners | PROD-TEST-007 | proposed |
