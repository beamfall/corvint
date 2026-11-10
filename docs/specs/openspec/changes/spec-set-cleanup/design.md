# Design: `spec-set-cleanup`

## Decisions

### One workflow document (SSC-V1-001)

- `docs/SPEC-DRIVEN-DEVELOPMENT.md` is the only document that describes how a change is made. It
  names the three stages of owner decision R41 (2026-11-09) and points to `openspec-check` (OSC-V1)
  for the exact rules; it does not restate them:
  - propose: the change folder `docs/specs/openspec/changes/<name>/` (G14 layout) is committed
    before any build commit for the change;
  - apply: `tasks.md` is worked in order, and each ADDED or MODIFIED requirement is written into the
    canonical spec under `docs/specs/` in the same change as the code; a REMOVED requirement is
    marked retired there;
  - archive: when every task is ticked and every delta ID is in the canonical spec, the folder moves
    to `changes/archive/YYYY-MM-DD-<name>/` by a pure rename and is not edited again.
- Every other document that touches the workflow either points to that document in one sentence
  or is removed. "Redirect" below means: the document stays at its path, its body is replaced or
  headed by an `Archived:` line that names `docs/SPEC-DRIVEN-DEVELOPMENT.md`, and it leaves the
  active list of `docs/README.md`.

### Inventory of older or competing workflow documents

Line numbers are in integ 9ec3a29d0 unless the row says prodspecs.

| Document | What it says today | Verdict | Batch |
|---|---|---|---|
| `docs/SPEC-DRIVEN-DEVELOPMENT.md` | Change loop with CEM (line 208), OCM intent reader (line 141), lane closeout and OCM cap (lines 232 to 240), `.agent-evidence` (lines 34 to 35); no change folders | rewrite (outline below) | C1, residue in B6 |
| `AGENTS.md` | "Spec-driven development" (line 58) mentions CEMs (line 67); Verify runs `interop/cem01-go` (lines 110 to 111); grep excludes `.agent-evidence` and `.corvint/changes` (line 138) | rewrite (outline below) | C1, B6 |
| `docs/specs/openspec/README.md` (prodspecs) | Index of change folders, plus its own summary of the layout and stages | rewrite: keep the index and the acceptance paragraph; replace the stage summary with a pointer | C1 |
| `docs/decisions/PROPOSED-openspec-workflow.md` (prodspecs) | Unaccepted options paper for OpenSpec use | redirect: one status line "superseded by G14 and owner decision R41 (2026-11-09); see `docs/SPEC-DRIVEN-DEVELOPMENT.md`" | C1 |
| `docs/CHANGE-FLOW.md` | Order of the dogfood closing steps | remove | B6 |
| `docs/V1-WORKFLOWS.md` | "Six experimental change-evidence workflows" | remove | C1 |
| `docs/CHANGE-EVIDENCE-MAP.md` | CEM contract | redirect (archive) | B6 |
| `docs/DOGFOOD.md` | Already an 11-line retirement stub (decision 0594) | redirect: its pointer names `docs/SPEC-DRIVEN-DEVELOPMENT.md` | C1 |
| `docs/LANE-ORCHESTRATION.md` | Lane process (1636 lines), read by 21 scripts, the Makefile, `tools/flow-guide-check` and one `.claude` skill | redirect (stub) when B8b removes its readers | B8b |
| `docs/AGENT-ROUTES.md` | Lane row (line 13), `.agent-evidence` grep (line 21), pre-1.0 CEM route (line 24) | rewrite those rows; add a "make a change" row pointing to the workflow document | C1, B6, B8b |
| `docs/SELF-DEVELOPMENT.md` | Lane (line 13), CEM1 verify-candidate (line 30), CEM bind, check and seal (line 93) | rewrite those lines | B6, B8b |
| `docs/SPEC-TOOLCHAIN-INTEGRATION.md` | 2026-08-29 assessment of SpecKit and OpenSpec, never accepted | redirect (archive) | C1 |
| `docs/specs/SPEC-HEALTH-2026-11-09.md` | Health class proposal, the input to this change | redirect (archive) once every row is applied | C3 |
| `protocol/integrations/openspec/README.md` | Section "Reading OpenSpec specs as OCM intent" (line 22) with `corvint ocm prepare` | rewrite that section (Apache-2.0 path; terms kept) | B6 |
| `integrations/codex/plugins/corvint/skills/corvint/SKILL.md` | Line 50: "follow the repository's CEM/OCM policy" | rewrite the line to name the change-folder workflow | B6, or the `agent-packages` change if it lands first |
| `.claude/skills/corvint-lane-flow` | Lane-flow skill | remove (owned by `source-removal` B8b) | B8b |
| `docs/README.md` | Links CHANGE-EVIDENCE-MAP (line 14) and DOGFOOD (line 25) | rewrite as the active reading list plus an "Archived" section | C1 |
| `docs/agent-memory/*.md` | Six files that already redirect to the task store | keep | - |
| Process specs under `docs/specs/` (for example `daily-change-evidence-workflow-v0.md`) | Already retired or classed in `specs-verdicts.md` | per `specs-verdicts.md` | per row |

### Archive marking (SSC-V1-002)

- Specs. An archived spec stays at its path (RMV-V1-026). Its header carries:
  - `Health: RETIRE` or `Health: OUT-OF-1.0` (OSC-V1-008);
  - `Intent status: retired YYYY-MM-DD (<decision or owner answer>)` and `Delivery status: retired`,
    the vocabulary already used by decision 0594 retirements;
  - `Retirement base: <commit>` (the commit before the batch that removes its code, or the C1 base
    when the code is already gone) and `Retirement note:` naming the successor, if any;
  - the Agent digest status line changed to match.
  The body is not edited. Already-retired specs gain only the `Health:` line.
- Docs. An archived doc stays at its path with a first line after the title
  `Archived: YYYY-MM-DD (<owner answer>); see <active document>.` and is listed only under
  "Archived" in `docs/README.md`. A JSON file takes no header; its listing under "Archived" is its
  marking.
- Deleted docs leave the tree in the batch that removes their last reader; `make documentation-paths`
  is rerun in that commit so `.corvint/documentation-paths.json` stays current.

### Requirement history (SSC-V1-003)

- `REQUIREMENTS.tsv` keeps a row for every ID of an archived spec, with its retired status; the
  generator already reads every spec file, so in-place archiving keeps the rows.
- `openspec-check` refuses an ADDED ID that exists in `REQUIREMENTS.tsv` or another change folder
  (OSC-V1 ID rule), so archived IDs are never reused.
- The 37 Tasks specs leave the tree in B10 (RMV-V1-027), which would drop their rows. Proposed: B10
  records their prefixes in the generated README under "Moved to beamfall/corvint-tasks", and the
  ID rule of `openspec-check` treats those prefixes as taken. This adds a generated section, so it
  waits for the owner (open item).

### Index (SSC-V1-005)

- Proposed: `INDEX.json` and the `docs/specs/README.md` table list only specs whose Health is KEEP
  or MERGE (active); a new generated `docs/specs/INDEX-ARCHIVED.json` and a README "Archived"
  section list the rest with their retirement lines. This changes a generated format, so it waits
  for the owner (G14). The readers to check in the same change are
  `cmd/corvint/core_freeze_test.go`, `internal/docmaintain/maturity_test.go`,
  `.corvint/existence-readers.json` and the `internal/gateselect` tests (integ 9ec3a29d0).
- Until the owner answers, `INDEX.json` keeps its shape and gains the `health` field only
  (OSC-V1-008).

### Reference check (SSC-V1-004)

- Active documents: every `*.md` under `docs/` and `docs/specs/` that is not archived, plus
  `AGENTS.md`, `README.md` and `llms.txt`.
- Excluded readers: `docs/decisions/`, `docs/build-log/`, `docs/RELEASE-NOTES*.md`, archived
  documents, and `docs/specs/openspec/changes/archive/`.
- A link or backticked path in an active document that names an archived or deleted document fails,
  unless it sits under a heading named "Archived" or "History", or on a line that says the target is
  archived.
- The check is a Make target in `doc-gates`. It reports and exits 0 until C3, then blocks.

### Batch alignment (SSC-V1-006, SSC-V1-007)

- Rows whose code still exists are applied in the same merge commit as the `source-removal` batch
  that removes the code (B1 to B8b, B11). Rows whose code is already gone, and the workflow
  documents, go in cleanup-only batches:
  - C1: workflow documents, the `Health:` line on every spec, and specs and docs whose code is
    already gone;
  - C2: MERGE folds into their named target, then the source is archived;
  - C3: `SPEC-HEALTH-2026-11-09.md` is archived and the reference check becomes blocking.
- B9 is the lane and process spec sweep of `source-removal`; its 12 rows are named in
  `specs-verdicts.md`. B10 waits for `beamfall/corvint-tasks` PR #4.
- Target: C1, C2, C3 and B0 to B5 rows by 2026-12-01. Rows in B6 (gated on the companion decision),
  B7 (gated on the watch measurement), B8b (about 2026-12-08), B10 (PR #4) and B11 (by 2026-12-15)
  land with their batch; a gated batch is not folded into another (RMV-V1-001).

## AGENTS.md replacement outline

1. Title and one-line product statement (unchanged).
2. Delivery and context (unchanged).
3. Product invariants. 1 to 3, 5 and 6 unchanged. 4 amended in B6 if its ledgers go (G8 names
   invariants 4 and 8). 7 keeps its spec citations that stay active and drops archived ones. 8
   becomes: "Every change is documented before it is built, as a change folder that passes the
   propose, apply and archive stages; see `docs/SPEC-DRIVEN-DEVELOPMENT.md`."
4. Spec-driven development, five lines: the workflow document is the only process document; one
   change folder per change; the canonical spec is updated in the same change; requirement IDs are
   stable and never reused; one build-log entry per material decision or removal batch. No CEM
   sentence.
5. Repository etiquette (unchanged, including the standing publication approval).
6. Verify: the Go toolchain line and test, vet and Windows vet commands; the `interop/cem01-go`
   lines go in B6.
7. Agent routing: routes, spec digests, the task store and ticket labels; the `.agent-evidence`
   grep goes in B6; `docs/TICKET-PRIORITY.md` and the `docs/agent-memory/` redirect line stay.
8. Licensing paragraph (unchanged).

## SPEC-DRIVEN-DEVELOPMENT.md replacement outline

1. Authority (current lines 8 to 21): only the owner accepts intent; generated or inferred text
   proposes. Unchanged in substance.
2. Where things live: `PRODUCT-PLAN-1.0.md`; canonical specs under `docs/specs/`; change folders
   under `docs/specs/openspec/changes/`; generated `REQUIREMENTS.tsv`, `INDEX.json` and README
   table; tickets in `.taskman/`; build-log entries. Replaces "Four linked artifacts".
3. Status model: intent and delivery vocabulary, plus the `Health:` line. OCM wording removed.
4. The change workflow: propose, apply, archive, with what each stage produces and a pointer to the
   OSC-V1 rules and the `Change:` and `Requirement:` commit trailers. Replaces "Change loop"; the
   CEM, OCM, lane closeout and `.agent-evidence` steps are removed.
5. Acceptance: G14 auto-accept for a proposal that only implements an accepted requirement, logged
   by `acceptance-record` (ACR-V1); anything changing behaviour, a format or the release bar waits
   for the owner; unaccepted work stays `experimental`.
6. Capability spec shape, authoring template and Agent digest (current lines 129 to 169).
7. Requirements: stable IDs, never reused; archived IDs stay as history rows (SSC-V1-003).
8. Archiving specs and docs: the marking of SSC-V1-002 and the reference check of SSC-V1-004.
9. Verification: a pointer to the Verify section of `AGENTS.md`.
10. Backfill and generated documentation (current lines 263 to 276) and Proportionality (line 277).

Not carried: the OCM intent reader (line 141), `interop/cem01-go` (line 208), the lane closeout and
OCM cap (lines 232 to 240). The ID helper lines (213, 216, 221) stay only if their scripts survive
B8a and B8b. The "Proposed amendment: solo falsifiability" section (line 279) is an open item.

## Failure modes

- A spec is archived while its code is still in the tree: C1 newly archives a spec only when its own
  code is gone in integ 9ec3a29d0, meaning its declared paths are absent or are shared packages that
  stay for an active spec (the row in `specs-verdicts.md` names them). Specs already marked retired
  by decision 0594 gain only the `Health:` line. Every other archive row waits for its removal
  batch.
- An active document links an archived or deleted one: the reference check names the file and line.
- A deleted doc is still named in `.corvint/documentation-paths.json`: `documentation-paths-check`
  fails; the batch reruns `make documentation-paths`.
- The index split breaks a reader of `INDEX.json`: the readers named above are run in the same
  change; without the owner's answer the split is not made.
- An archived ID is proposed again: the `openspec-check` ID rule refuses it.
- A gated batch slips past 2026-12-01: its rows keep their current class and the table keeps the
  gate; no row moves to another batch.
- An undetermined row: it stays as it is (active, not archived) until the owner answers. Decided
  2026-11-09 under R42, E8: eleven of the twelve undetermined spec rows are classed in
  `specs-verdicts.md`; `test-attack-v0.md` waits for the fault-injection proposal. The `init`,
  `adopt` and self-observation ledger rows are low confidence: what `main.go` still writes was
  not traced.
- A kept workflow whose spec is archived: `greptile-gate-v0.md` stays active while
  `greptile-gate.yml` is kept (decided 2026-11-09 under R42, D11; RMV-V1-035).
- A new spec appears after the table was written: it needs a `Health:` line (OSC-V1-008); the table
  is a one-time input and the header line is the lasting record.

## Acceptance evidence

- The reference check passes in blocking mode at C3, and a search of active documents finds the
  stage names only in `docs/SPEC-DRIVEN-DEVELOPMENT.md` and in pointers to it.
- `openspec-check` health rule passes: every spec carries a `Health:` line and archived specs carry
  the retirement lines.
- `make spec-requirements-check` passes, and a comparison of `REQUIREMENTS.tsv` before C1 and after
  C3 shows no ID removed except those of B10, which appear in the "Moved" record.
- `make documentation-paths-check` passes after each batch.
- One build-log entry per batch (owner decision R34 (2026-11-09)), naming the batch, its rows and its
  parent commit.
- A revert drill: `git revert -m 1` of the C1 merge on a scratch branch restores the prior headers
  and `make spec-requirements-check` passes.

## Rollback

- Each batch is one merge commit; `git revert -m 1 <commit>` restores the previous headers, documents
  and generated files (RMV-V1-030). Nothing is removed from history: deleted docs and the B10 Tasks
  specs stay reachable at the parent commit named in the batch's build-log entry.
- The reference check is removed from `doc-gates` by deleting its target line; it stays runnable by
  hand.
- If the index split was made, reverting its commit restores the single `INDEX.json` shape.
