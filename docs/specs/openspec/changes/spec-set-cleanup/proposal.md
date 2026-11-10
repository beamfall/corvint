# Change proposal: one propose, apply and archive workflow, and a verdict for every old doc and spec

Change name: `spec-set-cleanup`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-INT-010 and PROD-INT-011 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/spec-set-cleanup-v1.md`, a new spec with prefix `SSC-V1`
Requirement IDs this change adds: SSC-V1-001 to SSC-V1-008 (no `SSC-` row exists in
`docs/specs/REQUIREMENTS.tsv` and no other change folder uses the prefix)
Requirement IDs this change modifies: none
Owner answers this change implements: G8, G14, G24, G25, G26, G27, owner decision R33 (2026-11-09),
owner decision R34 (2026-11-09), owner decision R37 (2026-11-09), owner decision R41 (2026-11-09)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/spec-set-cleanup/spec.md`. Two tables belong to it: `docs-verdicts.md` (46 entries under
`docs/`) and `specs-verdicts.md` (288 files under `docs/specs/`).

## Why

Owner decision R41 (2026-11-09): "again, make sure everything is using the flows of OpenSpec, and
that anything old that may confuse things is cleaned up". Every change uses the propose, apply and
archive stages under `docs/specs/openspec/changes/`; G14 is unchanged (no npm tool, no root
`openspec/`); `openspec-check` enforces the stages; and this change gives every old doc and spec a
verdict and leaves one workflow document.

Today a contributor who opens the repository finds several documents that each describe how a change
is made, most of them built around CEM, OCM, dogfood binding or lanes, which G8 and G27 remove. A
spec index that lists 288 files with no health class reads as if all of them were live. PROD-INT-010
requires every surviving spec to name the product requirement it serves or be marked retired or
outside 1.0; owner decision R37 (2026-11-09) archives out-of-scope specs in place with health
classes.

## What exists today

Verified against integ 9ec3a29d0 on 2026-11-09:

- `docs/SPEC-DRIVEN-DEVELOPMENT.md` (290 lines) is the workflow contract named by `AGENTS.md`. Its
  "Change loop" section (lines 185 to 262) cites `interop/cem01-go` (line 208), lane closeout and
  the OCM cap (lines 232 to 240); line 141 describes the OCM intent reader; lines 34 to 35 name
  `.agent-evidence`. It does not mention the OpenSpec change folders.
- `AGENTS.md` (164 lines): the "Spec-driven development" section (line 58) points to that document
  and mentions CEMs (line 67); the Verify section runs `go -C interop/cem01-go` (lines 110 to 111);
  line 138 excludes `.agent-evidence` and `.corvint/changes` from a grep; line 151 keeps
  `docs/agent-memory/` as a redirect.
- Other documents that describe a change workflow: `docs/CHANGE-FLOW.md` (dogfood closing steps),
  `docs/V1-WORKFLOWS.md` ("Six experimental change-evidence workflows"),
  `docs/CHANGE-EVIDENCE-MAP.md`, `docs/LANE-ORCHESTRATION.md` (1636 lines, read by 21 scripts, the
  Makefile, `tools/flow-guide-check` and `.claude/skills/corvint-lane-flow`), `docs/DOGFOOD.md`
  (already an 11-line retirement stub, decision 0594), the lane and CEM rows of
  `docs/AGENT-ROUTES.md` (lines 13, 21, 24) and `docs/SELF-DEVELOPMENT.md` (lines 13, 30, 93), the
  "Reading OpenSpec specs as OCM intent" section of `protocol/integrations/openspec/README.md`
  (line 22), and the CEM/OCM line of `integrations/codex/plugins/corvint/skills/corvint/SKILL.md`
  (line 50).
- In the prodspecs tree, `docs/specs/openspec/README.md` already describes the G14 layout and the
  R41 stages, and `docs/decisions/PROPOSED-openspec-workflow.md` is an unaccepted proposal that G14
  and R41 settle.
- `docs/specs/` holds 288 tracked files. `docs/specs/SPEC-HEALTH-2026-11-09.md` proposes KEEP 71,
  MERGE 19, RETIRE 19 and OUT-OF-1.0 59; the remaining specs already read `Delivery status:
  retired`. Retired specs use the header lines `Intent status: retired 2026-11-09 (decision 0594)`,
  `Delivery status: retired`, `Retirement base:` and `Retirement note:`.
- `script/gen-spec-requirements.sh` generates `REQUIREMENTS.tsv`, `INDEX.json` and the
  `docs/specs/README.md` table from spec headers, and reads `docs/specs/INTENT-STATUS-FROZEN.tsv`.
  `INDEX.json` lists every spec, retired or not. Its readers include
  `cmd/corvint/core_freeze_test.go`, `internal/docmaintain/maturity_test.go`,
  `.corvint/existence-readers.json` and the `internal/gateselect` tests.
- `make documentation-paths` regenerates `.corvint/documentation-paths.json` from the top-level
  `docs/*.md` files, and `documentation-paths-check` is a doc gate (Makefile).

## What Changes

- One workflow (SSC-V1-001). `docs/SPEC-DRIVEN-DEVELOPMENT.md` is rewritten as the only document
  that describes how a change is made:
  - propose: the change folder is committed before any build commit for the change;
  - apply: `tasks.md` is worked in order, and the canonical spec under `docs/specs/` is updated in
    the same change as the code;
  - archive: the folder moves to `changes/archive/YYYY-MM-DD-<name>/` when its tasks are done and
    its delta is in the canonical spec.
  The exact stage rules are those of `openspec-check` (OSC-V1); this change writes the prose and
  does not restate the rules. Every other workflow document is rewritten, turned into a redirect, or
  removed, as listed in the inventory in `design.md`. Replacement outlines for `AGENTS.md` and
  `docs/SPEC-DRIVEN-DEVELOPMENT.md` are in `design.md`.
- Verdict tables. `docs-verdicts.md` gives each entry under `docs/` (excluding `docs/specs/`,
  `docs/decisions/` and individual build-log entries) a verdict: keep, rewrite, archive, delete or
  move, with a reason tied to the 1.0 scope and its references. `specs-verdicts.md` gives each of
  the 288 files under `docs/specs/` a class: active, archive in place, move to
  `beamfall/corvint-tasks`, or delete. Rows that cannot be settled say "undetermined" and what is
  missing.
- Archive marking (SSC-V1-002). An archived spec stays at its path and carries the existing
  retirement header lines plus the `Health:` line of OSC-V1-008; an archived doc stays at its path
  with an `Archived:` header line and moves to an "Archived" section of `docs/README.md`.
- Requirement history (SSC-V1-003). `REQUIREMENTS.tsv` keeps the IDs of archived specs as history
  rows; no archived ID is reused.
- Reference check (SSC-V1-004). A doc gate refuses an active document that links to an archived or
  deleted document as current guidance.
- Index (SSC-V1-005). The generated index and README table list only active specs by default;
  archived specs are listed separately. This changes a generated format, so it waits for the owner
  (G14).
- Order (SSC-V1-006, SSC-V1-007). Cleanup batches align with the `source-removal` batches: a spec is
  archived in the same merge commit that removes its code. Cleanup-only batches C1 to C3 carry the
  rows that remove no code. Tasks specs leave only after `beamfall/corvint-tasks` PR #4 merges
  (RMV-V1-027). Target: done by 2026-12-01, except rows whose removal batch is gated later.
- Rollback (SSC-V1-008). Every batch is reverted with `git revert`; nothing is removed from history.

## Capabilities

- New: `spec-set-cleanup` (SSC-V1), owned by `docs/specs/spec-set-cleanup-v1.md`.
- Read: `openspec-check` (OSC-V1) for the stage rules and the `Health:` line; `source-removal`
  (RMV-V1) for batch order, RMV-V1-026 (no spec file moved or deleted), RMV-V1-027 and RMV-V1-030.

## Impact

- `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `AGENTS.md`, `docs/README.md`, `docs/AGENT-ROUTES.md`,
  `docs/SELF-DEVELOPMENT.md`, `docs/PRODUCT.md` and `docs/specs/openspec/README.md` are rewritten.
- 164 specs are archived in place (those already retired gain only the `Health:` line) and 9 docs
  are archived; 6 docs are deleted, each in the batch that removes its last reader; 37 Tasks specs
  are deleted in B10; 12 spec rows and 2 doc rows stay undetermined until the owner answers.
- `script/gen-spec-requirements.sh` gains the Health projection (OSC-V1-008) and, if the owner
  accepts SSC-V1-005, an archived listing. Readers of `INDEX.json` named above must be checked in
  the same change.
- `.corvint/documentation-paths.json` is regenerated in every commit that deletes a top-level doc.
- `make doc-gates` gains the reference check, in report mode until C3.
- One build-log entry per batch (owner decision R34 (2026-11-09)).

## Non-goals

- No change to G14: no npm tool, no root `openspec/` directory, no `.openspec.yaml`, no claim of
  OpenSpec command compatibility.
- No restatement of the `openspec-check` rules and no second check of the stages.
- No edit of `docs/decisions/` entries or individual `docs/build-log/` entries; they are history.
- No move or deletion of a spec file other than the 37 Tasks specs in B10 (RMV-V1-026, RMV-V1-027).
- No code removal; `source-removal` owns that. This change only adds the doc and spec rows to each
  batch.
- No acceptance of intent; every verdict here is proposed until the owner accepts it.
