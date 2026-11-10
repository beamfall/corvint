# Change proposal: 1.0 release process, calendar, demotions and release artifacts

Change name: `release-process`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-INT-012, PROD-INT-013, PROD-INT-014, PROD-INT-017 and the
platform and artifact clauses of PROD-INT-006 and PROD-INT-015 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/release-process-v1.md`, a new spec with prefix `REL-V1`
Requirement IDs this change adds: REL-V1-001 to REL-V1-015
Requirement IDs this change modifies: PRS-V1-004 (platforms)
Owner answers this change implements: G8, G9, G10, G10a, G15a, owner decision R2 (2026-11-09),
owner decision R3 (2026-11-09), owner decision R24 (2026-11-09), answer 16, and the decisions
made 2026-11-09 under R42: D6, D7, D8, D9, D12, D13, D14, D16, E5, M5, M7
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/release-process/spec.md`.

## Why

The plan states what 1.0 "done" means (G10), the freeze and stable dates (G10a) and the tag-started
release (G15a), but nothing yet makes these testable. Owner decision R2 (2026-11-09) moves
qualification before the freeze with a qualification candidate tag (milestone M5a, about
2026-12-08), so stable can still ship on 2027-01-05. Owner decision R3 (2026-11-09) re-scopes the
`v1-0` release object in the task store (milestone M0), because it still carries criteria and gates
the plan no longer requires. The review of the release and Tasks proposals (findings 1, 2, 3, 4, 14,
17 and 18, 2026-11-09) found the calendar, the demotion basis, the tag order, the soak count, the
benchmark host and the artifact list undefined.

## What exists today

Verified against integ 9ec3a29d0 on 2026-11-09:

- No workflow starts on a pushed tag. The only release-related workflow,
  `.github/workflows/release-gates.yml` (integ 9ec3a29d0), runs on a nightly schedule (cron line
  17-18) and on `workflow_dispatch` with a `sha` input; its matrix (lines 51-95) runs focused-docs,
  full-gate on ubuntu-24.04 and macos-15, interop-gate, selection-baseline, core-n1-replay and
  companion-release, and its comment says nothing there pins, commits or publishes.
- `tools/release-binary/binaries.json` (integ 9ec3a29d0) has schema `corvint.release-binaries/1`,
  five targets (lines 5-9: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64) and
  32 binaries, 17 of them `corvint-analyzer-*`.
- Decision 0433 (`docs/decisions/0433-rc2-stable-signing-and-support-policy-2026-09-29.md`, item 1,
  lines 12-17, integ 9ec3a29d0) selects "No signing" for 1.0.0, forbids any agent-held signing
  identity, and sets the support window in item 2.
- `PRS-V1-004` in `docs/specs/corvint-1.0-product-and-release-v1.md` (integ 9ec3a29d0) names only
  darwin/arm64 and linux/amd64.
- The `v1-0` release object "Corvint 1.0 stable" (revision 149, 377 tickets) carries 12 obsolete
  acceptance criteria and the required gates full-gate, interop-gate, companion-release,
  focused-docs and release-checklist; 183 `v1-0` tickets were open (3 P0, 164 P1, 16 P2) (release
  review finding 4, read with `corvint-tasks release show` on 2026-11-09; not re-counted here).
  `corvint-tasks release update` exists (`corvint-tasks release --help`, build
  `1.0.0-rc.3.dev+build.144`).

## What Changes

- Milestone M0: the `v1-0` release object is rewritten through `corvint-tasks release update`; each
  open ticket is mapped to a plan requirement or moved to `post-1-0` with a recorded reason.
- Milestone M5a: a qualification candidate tag about 2026-12-08, rebuilt only for blockers. Its
  pushed tag SHA seeds the held-out draw.
- One calendar rule: freeze 2026-12-15; stable no earlier than the latest of 2027-01-05, RC publish
  plus 21 days, and the end of the soak.
- Demotion verdicts recorded on a named freeze-candidate SHA and confirmed on the RC SHA.
- A tag workflow that builds three targets (linux/amd64, linux/arm64, darwin/arm64) with
  `SHA256SUMS`, a manifest, licences and a build attestation, then stops; the publish job waits for
  the owner's approval in a protected environment. M8 becomes "RC approval and publish".
- Decision 0433 is superseded for the build attestation only.
- `binaries.json` cut to three targets, listed per product.
- Removal batches finish by 2026-12-01.
- Defined soak and trial counts, and a named, reserved benchmark host.

## Capabilities

### New Capabilities

- `release-process`: release object scope, calendar, demotions, tag workflow, artifacts, gates.

### Modified Capabilities

- `corvint-1.0-product-and-release` (PRS-V1-004): three platforms instead of two.

## Impact

- New workflow file for the tag-started release; GitHub environment and tag protection settings
  are owner actions.
- `tools/release-binary/binaries.json` changes shape (fewer targets, binaries grouped by product).
- The task store's `v1-0` release object and the membership of every open `v1-0` ticket change.
- The acceptance record (change `acceptance-record`) holds the owner's signatures and the
  superseded text of decision 0433 item 1.
- Tasks policy v15 (change `tasks-policy-v15`) follows M0.

## Non-goals

- No notarization, no install script, no Homebrew tap in 1.0 (G15a).
- No native Windows artifact (answer 16).
- No publishing by anyone but the owner: every RC and stable publish needs the owner's explicit
  approval (G10, G15a); 1.0 acceptance, RC publish and stable promotion stay outside the delegated
  choices.
- No change to the support window or to the rule against an agent-held signing identity in
  decision 0433.
- No release of corvint-tasks from this repository; Tasks ships from its own repository.
