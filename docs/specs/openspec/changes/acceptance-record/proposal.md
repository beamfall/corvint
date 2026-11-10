# Change proposal: one acceptance record for the 1.0 plan, a per-requirement acceptance log and signed release records

Change name: `acceptance-record`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: checklist O19 and the acceptance rule of `docs/specs/PRODUCT-PLAN-1.0.md`
(answer 18)
Owning capability spec: `docs/specs/acceptance-record-v1.md`, a new spec with prefix `ACR-V1`
Requirement IDs this change adds: ACR-V1-001 to ACR-V1-011
Requirement IDs this change modifies: none (SRG-V0-009 and SRG-V0-010 are extended by ACR-V1-007, not
changed)
Owner answers this change implements: answer 18, G10, G14, owner decision R1 (2026-11-09), owner
decision R9 (2026-11-09); owner decision R35 (2026-11-09) is a precondition
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/acceptance-record/spec.md`.

## Why

Answer 18 and owner decision R1 (2026-11-09) make acceptance of the 1.0 plan one decision record: it
accepts the plan as amended, names the requirement wording that was written rather than taken from
owner answers, and keeps in-flight work experimental. The plan supersedes parts of the pre-cut
contract (decision 0433, earlier answers, `AGENTS.md` dogfood sections) and nothing yet records which
text is replaced by what. G14 requires each requirement's acceptance to be logged so that a proposal
cannot declare itself accepted, and G10 requires the owner's signature before the RC publish and the
stable promotion. Owner decision R9 (2026-11-09) records Python file-level selection as a release
exception, which needs a place in that record.

## What exists today (verified against integ 9ec3a29d0 on 2026-11-09)

- `docs/SPEC-DRIVEN-DEVELOPMENT.md` lines 93 to 127 (integ 9ec3a29d0) define three acceptance modes
  per requirement (`owner`, `standing`, `unrecorded`) and the marker forms `(accepted, decision NNNN;
  V1-NNNN)` and `(accepted under the owner's standing auto-accept (YYYY-MM-DD); V1-NNNN)`.
- `Makefile` lines 401 to 407 (integ 9ec3a29d0) define `make spec-acceptance-audit`, which runs
  `script/spec-acceptance-audit.sh --acceptance $(ACCEPTANCE)` and reports `NOT_PROMOTABLE`
  (SRG-V0-010). It is a report, not a gate step.
- The latest decision record is `docs/decisions/0595-flows-and-how-move-to-docs-2026-11-09.md`.
- `docs/specs/openspec/` holds `README.md` and `changes/`; no acceptance log exists.
- No per-requirement acceptance entry exists for the PROD requirements or the checklist O-rows.

## What Changes

- One decision record accepts `docs/specs/PRODUCT-PLAN-1.0.md` as amended, by commit SHA. It names
  every requirement whose wording was written rather than quoted from an owner answer, states that
  in-flight work stays experimental until its own change is accepted, and carries a supersession
  table.
- The supersession table quotes each replaced text and names its replacement: decision 0433 item 1
  (attestation and npm provenance only), answer 3 (replaced by R8 and R9), answer 11, the G9 MCP row
  (replaced by R17), R26 (amended by R39 and R40), decision 0594 item 4, and the dogfood sections of
  `AGENTS.md`.
- The record lists Python file-level selection as a release exception (R9).
- An acceptance log `docs/specs/openspec/acceptance-log.tsv` holds one row per PROD requirement,
  checklist O-row and accepted change, with mode `owner`, `auto` or `pending-owner`, date, owner
  reference and rationale. The change `openspec-check` (OSC-V1-009) reads it.
- `make spec-acceptance-audit` also checks that log against the spec markers.
- Before the RC publish and before stable promotion, the owner signs a release record listing each
  requirement's evidence or demotion, NOT_RUN and NOT_OBSERVED items and deferred P2 and P3 tickets
  (G10, REL-V1-013).

## Capabilities

### New Capabilities

- `acceptance-record`: the plan acceptance decision, the acceptance log and the signed release
  records.

### Modified Capabilities

- `spec-requirements` (SRG-V0-009, SRG-V0-010): the audit reads the acceptance log too; existing
  requirement text is unchanged.

## Impact

- `docs/decisions/`: one new record; `docs/specs/openspec/acceptance-log.tsv`; the audit script and
  its tests; the release workflow reads the signed record (REL-V1-013).
- The change `openspec-check` depends on the log format.
- Owner decision R35 (2026-11-09) (the plan's dependency table and anchored citations) must be done
  before the record can name the plan SHA.

## Non-goals

- The record does not accept any change folder's intent; each change is accepted on its own row.
- No new acceptance mode beyond the three log modes and their mapping to the existing markers.
- How the owner's signature is made was open. Decided 2026-11-09 under R42, D12 and D13: a tag
  signed by the owner with a touch-required key listed in a committed allowed-signers file
  (ACR-V1-011). A separate machine account for agents is not part of this change; it is a 1.0.x
  ticket.
