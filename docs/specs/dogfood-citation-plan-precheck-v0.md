# Dogfood Citation Plan Precheck V0

Owner: Russell Lewis
Date: 2026-11-02
Requirement prefix: `DCW-V0`
Intent status: accepted under the owner's standing auto-accept (2026-10-11)
Delivery status: implemented
Authoritative inputs: V1-1724 owner lane request; `daily-change-evidence-workflow-v0.md`
(`DCW-V0-019`, `DCW-V0-029`, `DCW-V0-335`, `DCW-V0-380`, `DCW-V0-690`);
`cem-stable-v1.md` (`CEM-V1-114`).

## Agent digest
- Claim: A citation-plan precheck names every refused row and its CEM reason before binding.
- Status: accepted under the owner's standing auto-accept (2026-10-11) / implemented
- Exists: `dogfood inputs-check` and finish-lane prepare reuse `cem cite` against a temporary map.
- Blocked on: measured lane bind-attempt savings remain NOT_OBSERVED.
- Read next: Requirements; Acceptance and evidence; Compatibility and rollback.

## User job and verified baseline

A lane author needs the whole citation repair list before spending a bind attempt. At base
`b486d3978728731c6638b50fa2210dfbb6e365ea`, `precheckCitationPlan` in
`internal/dogfoodflow/change.go` already invokes the binder's `cem cite` for every cite row,
but prints only the first 32 refusals. Its mixed-code regression covers four failures.
The owning workflow spec has 251 indexed requirements at this base, so this companion
holds the additional obligation without growing that nearly full Requirements section.

## Requirements

- `DCW-V0-2130`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1724) For an admitted citation plan, `dogfood inputs-check` and finish-lane's prepare pass MUST name every row refused by the existing `cem cite` plan-row precheck, in input order, with its row number, hunk, evidence path, BASE span, reason code and CEM detail. This replaces the 32-named-row cap in `DCW-V0-335` and `DCW-V0-690`; the existing 256-plan-row limit bounds the list. `rows=N` MUST equal the refused-row count; valid rows MUST NOT be named. The precheck MUST reuse `cem cite` rather than implement another stability or authority rule, and MUST stop before binding when any row is refused. It MUST preserve HEAD and restore the prepared map and probe outputs under the existing preflight lifecycle.

## Non-goals, trust boundary and failure modes

No new verb, build, evidence authority, citation rule, wire format or bind behavior. The existing
`make dogfood-inputs-check BASE=FULL_SHA CITATIONS=plan.tsv INTENTS=intents.txt VERIFY=verify.txt`
entry prepares and validates without binding; direct `corvint dogfood inputs-check` uses the
caller's current tree binary. Admission, shape, map binding, uniformity and input requirements
remain governed by the owning workflow. Missing or stale evidence remains a refusal. Lists
are bounded by the admitted plan, at most 256 rows; the bind report's existing display cap is
unchanged. Probe operational failures retain the existing handling and are not reclassified
as evidence refusals. Passing precheck does not establish semantic support or passing tests.

## Acceptance and evidence

| Requirement | Implementation | Deterministic evidence |
|---|---|---|
| `DCW-V0-2130` | `internal/dogfoodflow/change.go` `precheckCitationPlan` | `TestDCWV02130InputsCheckNamesEveryRefusedRowAtPlanLimit`: 256 rows, 255 mixed authority/stability/missing-evidence refusals named exactly once, final valid row omitted, HEAD preserved, map/probes restored, no bind. |
| Existing mixed-row and prepare behavior | Same `cem cite` probe | `TestDCWV0690InputsCheckNamesEveryRefusedRowInOnePass`; `TestFinishLaneDCWV0335PrepareRefusesUnstableCiteSpanBeforeBind`. |

The focused tests exercise actual CEM preparation and citation, not a reimplementation of
its admissibility rules. Results and limits are retained in `../build-log/2026-11-02-lsf-w49.md`.
First-attempt lane savings remain NOT_OBSERVED; no frozen retrieval or learning behavior changes.

## Compatibility and rollback

Only precheck diagnostics expand past 32 rows; codes, exit status and state lifecycle are
unchanged. Revert the precheck display loop and companion amendment to restore the cap.
Keep the regressions to make that loss of whole-plan diagnostics visible. Future citation
rules remain owned by CEM and reach this check through the same probe call.

## Remaining decisions

No implementation decisions remain. Real lane time savings are outside this deterministic gate.
