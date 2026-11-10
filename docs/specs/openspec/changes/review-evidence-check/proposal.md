# Change proposal: `corvint review` as the deterministic pre-merge evidence check (preview)

Change name: `review-evidence-check`
Ticket: none filed yet (to be filed in `.taskman/` before dispatch).
Product requirement served: PROD-INT-029, PROD-TEST-010, PROD-TEST-012 and PROD-INT-001 in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owner decisions this change implements: owner decision R4 (2026-11-09) and owner decision R30
(2026-11-09); review docs F3
Owning capability spec: a new spec `docs/specs/review-evidence-check-v1.md` (prefix `RVE-V1`). It
does not exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: RVE-V1-001 to RVE-V1-010 (RVE-V1-010 added 2026-11-09 for owner
decision R30).
Prefix: `RVE-V1` replaces the first draft's `REC-V1` (settled without the owner, 2026-11-09).
`docs/specs/REQUIREMENTS.tsv` holds REC-V0-001..007 in `retrieval-eval-comparability-v0.md`
(retired by decision 0594), and `script/requirement-id-reservations.sh:19-20`, `:91` takes the
family stem up to `-V<n>`, so `REC-V1` would share the retired `REC` family. `RVE-` has no entry
(checked 2026-11-09 on `claude/layers-integ` at `9ec3a29d0`), and no `REC-V1` ID was ever committed,
so the rename costs nothing.
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started. Release label: preview for 1.0; it never blocks the release (G25).
Written before dispatch: yes. G25 requires a committed proposal before any build.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/review-evidence-check/spec.md`.

## Why

G25 item 2: `review` becomes the deterministic pre-merge evidence check: changed lines whose
selected tests did not run, changed flows with an untested variation, unlinked E2E tests, stale
docs, and ticket acceptance criteria without evidence. Consumers: the Action PR comment, the agent
end-of-turn report and the Tasks completion gate.

### Today's state, from the code at `9ec3a29d0`

- `review`, `features` and `overview` share one implementation, `cmd/corvint/repository_guidance.go`
  (751 lines; the three commands are accepted at `:132`). `review` takes `--base` and `--max-refs`
  (`:147`) and reads one repository (WSD-V0-146).
- `reviewGuidance` (`:506`) reports changed features inferred from the target tree, and
  `snapshotReviewAffected` (`:681-711`) runs `affected.Build` inside a `genesis.GuidanceSnapshot`.
  The receipt profile is `repository-guidance/0`, experimental, authority `inferred` (`:310`); its
  feature discovery uses regexes (`:281-282`, `:429`).
- `corvint test` writes a `corvint-test-run/0` receipt, by default
  `<git-dir>/corvint/test-run[.I].json` (`cmd/corvint/test.go:168`, `:417-425`, `:477-478`), with an
  outcome per selected unit and `NOT_RUN` for units that did not run (`internal/sessiontest/run.go:31`,
  `:801`).
- Tickets carry `acceptanceCriteria` (an array of free-text strings), `requirementRefs` and
  `requiredGates` (seen in `.taskman/tickets/V1-0019.json`). The criteria have no IDs, and no ticket
  field ties a criterion to evidence today.

## What Changes

`corvint review --base FULL_SHA [--ticket ID] [--receipt FILE ...] [--docs-check FILE]` recomputes
the plan for the range and checks the recorded evidence against it.

### Inputs

1. The base and head commits.
2. The affected plan for the range, recomputed by the affected engine.
3. `corvint-test-run/0` receipts: the default path, or each `--receipt FILE`. Only receipts whose
   recorded tree equals the head tree count.
4. `.corvint/flows.json`, its link records and the pinned E2E inventory.
5. Optionally `--docs-check FILE`: a saved `corvint-docs docs flows check` output for the head
   (`cmd/corvint-docs/docs_flows.go:160-170`); absent means the docs finding kind is `NOT_OBSERVED`.
6. Optionally `--ticket ID`: the ticket's acceptance criteria and the receipt IDs stored on it by a
   ticket gate (G9 item I1).

### Findings (the five kinds from G25)

- `selected-not-run`: a changed line whose selected tests have no passing outcome in a counted
  receipt (missing, `NOT_RUN`, or failed).
- `untested-variation`: a changed flow with a variation that has no linked test, or whose linked
  tests did not pass.
- `unassigned-e2e`: E2E tests in the inventory with no accepted link (G11).
- `stale-docs`: claims the docs check marks stale.
- `criterion-without-evidence`: an acceptance criterion of the given ticket with no evidence record.

### Output shape

Profile `corvint-review-check/0`: `{profile, base, head, verdict: CLEAN|FINDINGS|INCOMPLETE,
inputs: [{kind, state, path?, reason?}], findings: [{kind, subject, reason, evidence?}], totals,
truncated}`. Exit 0 for `CLEAN`, 1 for `FINDINGS`, 2 for refusal, 3 for `INCOMPLETE`.

### Consumers

`review` MUST NOT be required by any release-blocking gate or report (review docs F3). Blocking
consumers read `corvint affected` plus the `corvint-test-run/0` receipt directly; `review` is an
optional, advisory input behind an opt-in setting (owner decision R4, 2026-11-09).

- The GitHub Action PR comment: advisory (change `github-action`, GHA-V1-022).
- The Tasks completion gate: records the body as advisory evidence; it fails on `review` only when
  the queue policy opts in (changes `tasks-integration` TIN-V1-008 and `tasks-policy-v15`).
- The agent end-of-turn report does not read `review`; it uses `affected` plus receipts (change
  `agent-packages`, AGT-V1-013).

Earlier wording, superseded by R4 and F3: the agent end-of-turn report "prints the findings", and
"a ticket gate may require `verdict: CLEAN` for the ticket's range".

### Docs findings

When corvint-docs is absent or no `--docs-check` file is given, the docs input is `NOT_OBSERVED`
and does not block `CLEAN`. `stale-docs` findings are advisory by default and change no consumer's
result unless that consumer's opt-in setting is on (owner decision R4).

### MCP

If `review` is offered over MCP, its server, tool name, input schema, output profile and error codes
are those of its row in the table of change `mcp-tool-table` (owner decision R30, 2026-11-09).

## Capabilities

### New Capabilities

- `review-evidence-check`.

### Modified Capabilities

- `review` leaves `repository_guidance.go`; `features` and `overview` are handled by change
  `feature-map-from-flows`.

## Impact

- Removed: `reviewGuidance`, `snapshotReviewAffected`, `guidanceAffectedFallback`, the
  `repository-guidance/0` review output and their tests. The rest of `repository_guidance.go` is
  removed by `feature-map-from-flows`.
- Added: a check builder in the tests layer on the affected engine, the receipt reader and the shared
  flow readers.
- Unrelated and kept: `internal/appflows/review.go` holds link review state (AFU-V1-007 to 009,
  the "reviewed" basis), not the `review` verb. This change does not touch it (settled without the
  owner, 2026-11-09).

## Non-goals

- Code review opinions, style or risk scoring.
- Running tests or `corvint-docs` from inside `review`.
- Judging whether evidence satisfies a criterion's meaning; the check only reports whether evidence
  is recorded.
- Blocking the release: this is preview.
