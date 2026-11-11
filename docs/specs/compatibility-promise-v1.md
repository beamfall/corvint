<!-- SPDX-License-Identifier: Apache-2.0 -->
# Compatibility Promise V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted (decision 0596, 2026-11-10)
Delivery status: experimental
Authoritative inputs: change folder `docs/specs/openspec/changes/compatibility-promise/` (the
proposal, design and spec delta this file was created from); owner decisions D10 and D10a
(2026-11-09, under R42), recorded in decision 0596; `docs/specs/PRODUCT-PLAN-1.0.md` product gap
(g) and checklist rows O30 and O64; ticket V1-5434.

## Agent digest
- Claim: One generated table in docs/COMPATIBILITY.md marks each 1.x surface stable or preview, and only a surface a named contract test covers is stable.
- Status: accepted (decision 0596, 2026-11-10) / experimental
- Exists: this spec; the surface list and the generator and check come with tasks 2.1 to 2.4 of the change.
- Blocked on: the surface count of task 2.1 (CMP-V1-003 stop rule) and the generator and check.
- Read next: Requirements; Failure modes; Rollback.

## User and boundary

A 1.0 release with no stated promise is read as promising everything. Users, the GitHub Action
and the agent packages need to know what a 1.x release can change. Owner decision D10 chose a
narrow promise that covers only surfaces a contract test already protects; D10a chose its form,
one table in `docs/COMPATIBILITY.md` with a `stable` or `preview` column, and no label renamed at
1.0.

Non-goals:

- no promise for MCP input and output schemas, Go packages, the index or any cache;
- no new contract test written only to widen the promise: a surface without a test is not stable
  in 1.0;
- no label rename and no wire change;
- no promise beyond 1.x;
- no second release-note check: a preview change without a release note is caught by the release
  record check of change `release-process`.

## Requirements

- `CMP-V1-001`: (accepted, decision 0596; V1-5434) `docs/COMPATIBILITY.md` MUST hold the 1.x
  compatibility promise as one table with one row per surface and a stability column whose value
  is `stable` or `preview`. A surface that has no row MUST be treated as `preview`. (decided
  2026-11-09 under R42, D10 and D10a)
- `CMP-V1-002`: (accepted, decision 0596; V1-5434) A row MUST be `stable` only when a contract test
  covers the surface, and the row MUST name that test. The stable rows MUST be generated from the
  existing contract tests; a row MUST NOT be written by hand. A check MUST fail, naming the row,
  when a stable row names a contract test that does not exist, and regenerating the table with the
  contract tests unchanged MUST leave `docs/COMPATIBILITY.md` byte-identical. (decided 2026-11-09
  under R42, D10)
- `CMP-V1-003`: (accepted, decision 0596; V1-5434) The stable set MUST be exactly these surfaces,
  each subject to CMP-V1-002: (1) the supported-command tables of `corvint`, `corvint-docs` and
  `corvint-tasks`: verbs, flags and exit codes; (2) the documented `--json` fields of these
  formats: flow file, plan, test-run receipt, run evidence, ticket schema v1 and receipt reference
  v1; (3) the documented files under `.corvint/` and in the Tasks store; (4) the action's inputs
  and outputs, the action's major tag, and the Corvint version range that each agent package
  declares; (5) the MCP tool names in the tool table. The label of each format in item 2 MUST be
  read from the code before its row is written. A listed surface with no contract test gets no
  `stable` row, and the change's build-log entry names the gap. If fewer than half of these
  surfaces have a contract test, the lane MUST stop and report that to the owner before it writes
  the table. (decided 2026-11-09 under R42, D10 and D10a)
- `CMP-V1-004`: (accepted, decision 0596; V1-5434) A `stable` surface MUST change only by addition
  within 1.x: a 1.x release that adds a field to a stable `--json` format keeps every documented
  field's name, type and meaning. Removing or renaming a stable verb, flag, field, file, input,
  output or tool name MUST be announced one minor release before it happens, and the old name
  MUST keep working as an alias, with a deprecation notice, during that release. (decided
  2026-11-09 under R42, D10 and D10a)
- `CMP-V1-005`: (accepted, decision 0596; V1-5434) `docs/COMPATIBILITY.md` MUST list these as
  `preview`, by name: `corvint-feature-map/0`, `corvint-data-map/0`, `corvint-change-report/0`,
  `corvint-review-check/0`, `corvint-flow-link-proposals/0`, `corvint-harness-event/0`, the MCP
  input and output schemas, the Go packages, the index and every cache. A `preview` surface MAY
  change in any release, and the release notes MUST name the change. An MCP tool name can be
  `stable` while its input schema is `preview`. (decided 2026-11-09 under R42, D10 and D10a)
- `CMP-V1-006`: (accepted, decision 0596; V1-5434) No profile label MUST be renamed at 1.0. The
  digit in a label is part of the name and MUST NOT be read as a stability mark; the stability
  column of CMP-V1-001 carries that meaning. A breaking change to a stable profile MUST take the
  next digit, and the old profile MUST be served for one minor release. (decided 2026-11-09 under
  R42, D10a)

## Failure modes

| Failure | Behavior |
|---|---|
| A stable row names a test that was deleted | The check fails and names the row (CMP-V1-002). |
| A hand edit to `docs/COMPATIBILITY.md` | The check fails on the difference from the generated table (CMP-V1-002). |
| Fewer than half of the listed surfaces have a contract test | The lane stops and reports to the owner before it writes the table (CMP-V1-003). |
| A `/0` label under a stable row is read as unstable | The page states, above the table, that the digit is part of the name and that the column carries the stability (CMP-V1-006). |
| A stable format's label differs from what the change folder assumed | The label is read from the code and recorded in the build-log entry (CMP-V1-003). |
| A preview surface changes without a release note | The release record check of change `release-process` catches it; this spec adds no second check (CMP-V1-005). |

## Acceptance scenarios

| Requirement | Acceptance scenarios (change delta) |
|---|---|
| CMP-V1-001 | A surface without a row |
| CMP-V1-002 | A stable row without a test; Regenerating the table |
| CMP-V1-003 | A listed surface has no contract test |
| CMP-V1-004 | Removing a stable flag; Adding a field |
| CMP-V1-005 | A cache format changes; An MCP schema changes |
| CMP-V1-006 | A stable profile whose label ends in `/0`; A breaking change to a stable profile |

## Acceptance and rollback

Acceptance evidence: `docs/COMPATIBILITY.md` generated, with every stable row naming an existing
contract test and a second generation byte-identical; the check's tests for a missing contract
test, a hand-edited page and an unlisted surface; the count of CMP-V1-003 surfaces with and without
a contract test, and the label read from the code for each stable format, in the build-log entry
`docs/build-log/2026-11-11-compatibility-promise.md`.

Rollback: the page, generator and check are additive. Reverting the change's commits returns
`docs/COMPATIBILITY.md` to the placeholder of UDC-V1-007 (or to no page). A promise that has
shipped in a release is not withdrawn by a revert; narrowing it needs an owner decision and a
release note.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| CMP-V1-001 to CMP-V1-006 | not yet built (tasks 2.1 to 2.4) | NOT_PRODUCED |
