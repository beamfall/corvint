# Change proposal: Go check of the OpenSpec change folders and their propose, apply and archive stages

Change name: `openspec-check`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-INT-011 and PROD-INT-010 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/openspec-check-v1.md`, a new spec with prefix `OSC-V1`
Requirement IDs this change adds: OSC-V1-001 to OSC-V1-012
Requirement IDs this change modifies: PRS-V1-018 (clauses (c) and (d) carried over, (a), (b) and (e)
dropped)
Owner answers this change implements: G14, owner decision R37 (2026-11-09), owner decision R41
(2026-11-09), owner decision H9 (2026-11-10), owner decision H12 (2026-11-10), owner decision H13
(2026-11-10)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/openspec-check/spec.md`.

## Why

G14 adopts the OpenSpec change-folder layout under `docs/specs/openspec/changes/`, "checked by a Go
check in CI", with no root `openspec/` directory. G14 also said "no npm tool" and "no claim of
OpenSpec CLI compatibility"; owner decision H12 (2026-11-10) replaces those two clauses: the
OpenSpec command is installed and its propose, apply and archive flow is followed. Owner decision
R41 (2026-11-09) requires every change to use the propose, apply and
archive stages, and requires the check to enforce those stages, not only the folder layout. Owner
decision R37 (2026-11-09) carries over two of the five spec-driven development improvements of
PRS-V1-018: (c) generated spec header rows and (d) a recorded acceptance mode per requirement with
an audit command; and it archives out-of-scope specs in place with spec health classes.

Without a check, the layout and the "documented before built" rule (PROD-INT-011) depend on review
alone, and a finished change folder can stay under `changes/` indefinitely, where it reads as
pending work.

## What exists today

Verified against integ 9ec3a29d0 on 2026-11-09:

- No Go check reads `docs/specs/openspec/changes/`. `script/gen-spec-requirements.sh` reads only
  files directly under `docs/specs/`, so change-folder deltas never reach `REQUIREMENTS.tsv`.
- PRS-V1-018 (c) is largely delivered: `make spec-requirements` generates `REQUIREMENTS.tsv`,
  `INDEX.json` and the `docs/specs/README.md` table from spec headers, and
  `spec-requirements-check` fails when they are stale (SRG-V0-012 in
  `docs/specs/spec-requirement-index-generator-v0.md`; `docs/SPEC-DRIVEN-DEVELOPMENT.md` lines
  243-258; integ 9ec3a29d0).
- PRS-V1-018 (d) is largely delivered: `REQUIREMENTS.tsv` carries the columns `acceptance` and
  `acceptedBy` with the modes `owner`, `standing` and `unrecorded` (SRG-V0-009), and
  `make spec-acceptance-audit` (Makefile lines 401-407, `script/spec-acceptance-audit.sh`) lists
  the requirements of one mode and reports a validated spec that still holds a standing requirement
  as NOT_PROMOTABLE (SRG-V0-010). It is a report, not a gate.
- `interop/spec-toolchains/versions.json` (lines 6-11, integ 9ec3a29d0) pins the npm package
  `@fission-ai/openspec` 1.16.0 as a toolchain Corvint recognises in other repositories. It is not
  used to check this repository.
- Since 2026-11-10 `docs/specs/openspec/config.yaml` (schema `spec-driven`) makes
  `docs/specs/openspec/` an OpenSpec root, and `openspec validate --all` run from `docs/specs` with
  version 1.17.0 passes for all 45 change folders. No Make target or CI job runs it yet.
- The existing change folders were written before this check. Some `tasks.md` items name no
  requirement ID (for example `tasks-integration/tasks.md` task 2.1 in the prodspecs tree).
- `docs/specs/SPEC-HEALTH-2026-11-09.md` (prodspecs tree) proposes a class for each of 168 specs:
  KEEP 71, MERGE 19, RETIRE 19, OUT-OF-1.0 59. No spec header carries a class today.

## What Changes

- A Go check (`tools/openspec-check`) run in CI and in the focused documentation gate.
- Layout rules for each active change folder (G14).
- Stage rules over a commit range:
  - propose: the change folder exists in the parent commit of any code commit for that change;
  - apply: `tasks.md` items are ticked in order, and the canonical spec carries each delta in the
    same change as the code;
  - archive: a finished change moves to `docs/specs/openspec/changes/archive/YYYY-MM-DD-<name>/`,
    and an archived folder is never edited.
- Boundary rules: exactly one OpenSpec root, `docs/specs/openspec/`, with its `config.yaml`; no
  `openspec/` directory and no `.openspec.yaml` at the repository root; no second set of canonical
  specs under `docs/specs/openspec/specs/`.
- A Make target `openspec-validate` that runs `openspec validate --all` at one pinned version
  (owner decision H12 (2026-11-10)). Its CI wiring is a separate `.github` pull request.
- The status `external-spec-not-verified` for a requirement owned by another repository is a
  reported status that does not fail the check (owner decision H9 (2026-11-10)).
- A spec health class row in each canonical spec header, checked against a closed set.
- PRS-V1-018 is modified: (c) and (d) carry over and are traced to SRG-V0-009, 010 and 012; (a),
  (b) and (e) are dropped.

## Capabilities

### New Capabilities

- `openspec-check`: layout, stage, boundary and health-class checks, and validation by the
  OpenSpec command.

### Modified Capabilities

- `corvint-1.0-product-and-release` (PRS-V1-018).
- `spec-requirement-index-generator`: the health class becomes a generated header row.

## Impact

- `make doc-gates` and the `focused-docs` gate gain one check.
- Developers who write or archive a change need the OpenSpec command (`@fission-ai/openspec`, Node
  20.19 or later) at the pinned version. The product build and the shipped binaries do not.
- Existing change folders must be brought into the layout before the check becomes blocking.
- CI checkouts used by the check need enough history to see the commit that added each folder.
- The change `spec-set-cleanup` gives every old document and spec a verdict and leaves one workflow
  document; it writes the health classes and does the archiving. This change only checks the result.
- The auto-accept log format and its audit command belong to the change `acceptance-record`; this
  check reads that log.

## Non-goals

- No root `openspec/` directory (G14). No npm dependency in the product build, and no shipped
  binary that calls the OpenSpec command (AGENTS.md invariant 7 is unaffected).
- The Go check does not call the OpenSpec command; the two run side by side.
- No native spec merge (owner decision H13 (2026-11-10): "Keep flat specs"): `openspec archive`
  is always run with `--skip-specs`, and no `docs/specs/openspec/specs/` tree is created.
- Strict-mode warnings of the OpenSpec command (requirement text over 500 characters, text above
  the first section) do not fail anything in this change.
- The delta does not replace the canonical spec; repository spec files and requirement IDs stay the
  source of truth (G14).
- No change to `interop/spec-toolchains/versions.json`, which describes other repositories.
- No semantic review of requirement text; the check reads structure, IDs and history only.
