# Change proposal: Tasks policy version 15 without CEM and OCM requirements

Change name: `tasks-policy-v15`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: milestone M0 in `docs/specs/PRODUCT-PLAN-1.0.md` (the cut removes the CEM
and OCM tooling that policy version 14 still requires)
Owning capability spec: `docs/specs/tasks-policy-v15.md`, a new spec with prefix `TPV-V1`
Requirement IDs this change adds: TPV-V1-001 to TPV-V1-008
Requirement IDs this change modifies: none
Owner answers this change implements: owner decision R3 (2026-11-09)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/tasks-policy/spec.md`.

## Why

Milestone M0 removes the CEM and OCM commands from `corvint`. The repository's Tasks policy still
requires a CEM and an OCM for every attempt and still declares two gates that run removed commands or
removed tests. After M0, a submit under policy version 14 would demand evidence that can no longer be
produced. Owner decision R3 (2026-11-09) drops those requirements in a new policy version.

## What exists today (verified against integ 9ec3a29d0 on 2026-11-09)

`.taskman/policy.json` line 1 (integ 9ec3a29d0) holds one JSON object:

- `policyVersion` "14", `profile` `taskman-policy/0`, `cemRequired` true, `ocmRequired` true.
- `reservations.sharedPaths`: `.corvint/change.cem.json`, `.corvint/changes/`,
  `docs/specs/INDEX.json`, `docs/specs/README.md`, `docs/specs/REQUIREMENTS.tsv`.
- Eleven gates: `cem-verdict` (required false; runs `corvint cem gate-verdict` with `cem01-go`),
  `companion-release`, `context-focused` (required false; `go test -run
  Context|Lexical|SlotWeight|AnalyzerSchemaInputs|TCPV0008 ./internal/contextindex ./cmd/corvint`),
  `context-vet` (required false; `go vet ./internal/contextindex ./cmd/corvint`), `focused-docs`
  (required true), `full-gate`, `host-lifecycle-focused`, `interop-gate`, `public-release`,
  `release-checklist`, `tasks-export-focused`.

## What Changes

- Policy version 15 sets no `cemRequired` and no `ocmRequired` (both removed or false) and drops the
  gates `cem-verdict` and `context-focused`. Every other member keeps its version 14 value.
- The store copy is updated first with `corvint-tasks policy update`; the tracked
  `.taskman/policy.json` follows in one carry PR with no other change.
- The update runs after the M0 removal lands (REL-V1-001), not before, so that no attempt is
  admitted without evidence while the CEM commands still exist.
- Before the update, no release object and no open ticket may reference a removed gate.
- The version 14 bytes and their SHA-256 are retained for rollback.

## Capabilities

### New Capabilities

- `tasks-policy`: the repository's Tasks policy contents for 1.0.

### Modified Capabilities

- None.

## Impact

- `.taskman/` store policy and `.taskman/policy.json`.
- In-flight attempts admitted under version 14 may be refused with `STALE_POLICY` and must be
  resubmitted.
- `AGENTS.md` and `docs/DOGFOOD.md` text that requires a CEM per change is handled by the change that
  removes the dogfood sections, not here.

## Non-goals

- No change to `taskman-policy/0` or to corvint-tasks code.
- No change to the remaining gates' commands (whether `context-vet` is retargeted is Open).
- No migration of existing receipts that cite `cem-verdict`.
