# Flow Schema Version V1

Owner: Russell Lewis
Date: 2026-11-11
Requirement prefix: `FSV-V1`
Intent status: accepted automatically under G14 (decision 0596, 2026-11-10; V1-5410)
Delivery status: not-started
Authoritative inputs: change folder `docs/specs/openspec/changes/flow-schema-version/`
(`proposal.md`, `design.md`, `tasks.md` and `specs/flow-schema-version/spec.md`),
`docs/specs/PRODUCT-PLAN-1.0.md` (PROD-INT-016, PROD-DOCS-011, PROD-INT-004),
`docs/specs/affected-plan-kinds-v0.md` (APK-V0, the flows declaration and `plan.e2e`) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md`.

## Agent digest
- Claim: The flow file carries its schema version; one shared reader refuses unknown versions, so plans run every E2E test and flow reports print nothing partial.
- Status: accepted automatically under G14 (decision 0596, 2026-11-10; V1-5410) / not-started
- Exists: the change folder; the exact-profile reader `ReadFlowsDeclaration` in `internal/appflows/flowplan.go`.
- Blocked on: tasks 2.1 to 2.6 of the change.
- Read next: Requirements; Delivery and traceability; Rollback.

## User and boundary

A repository that commits `.corvint/flows.json` needs every Corvint reader of that file to agree on
what it means. Once `corvint` and `corvint-docs` ship and update separately (PROD-INT-004), a newer
flow file can meet an older reader; that reader must say so and fail closed rather than fail with a
generic parse error or read the file partially. This spec is the canonical home of FSV-V1-001 to
FSV-V1-007, copied from the change's spec delta. Decisions, failure modes and acceptance evidence
stay in the change's `design.md`, which owns them. Each delivery is recorded in one
`docs/build-log/YYYY-MM-DD-<slug>.md` entry.

Non-goals (from the change): no migration of `.corvint/e2e-flows.json` (change
`affected-plan-kinds`); no reading of older or newer versions by translation, since a reader
supports a closed list; no version field inside other `.corvint/` files; no change to how flows
select tests.

Trust boundary: the flow file is tracked repository input. A reader never trusts a version it was
not built for: an unsupported or malformed profile is a refusal, never a fallback to version 0.

## Requirements

Requirement IDs are stable. Each requirement keeps the change's text.

- `FSV-V1-001`: (accepted automatically under G14, decision 0596; V1-5410) The profile carries the schema version

`.corvint/flows.json` MUST carry its schema version in the `profile` member as `corvint-flows/N`,
where N is a whole number written without leading zeros. A file with profile `corvint-flows/0` MUST
stay valid as version 0. (G5; PROD-INT-016)

**Scenario: Today's file**
- GIVEN a flow file with profile `corvint-flows/0` and only today's members
- WHEN the reader runs
- THEN it accepts the file as version 0

- `FSV-V1-002`: (accepted automatically under G14, decision 0596; V1-5410) A closed list of supported versions; any member change increments N

Each reader build MUST hold one closed list of supported versions. Adding, removing or renaming a
member, or changing a member's meaning, MUST increment N. (G5)

**Scenario: A new member without a new version**
- GIVEN a change that adds a member to the flow file but keeps the same N
- WHEN the member-list test compares the reader's members for that N with the member list
  recorded for N when it was released
- THEN the test fails, names the added member and says to increment N

- `FSV-V1-003`: (accepted automatically under G14, decision 0596; V1-5410) One shared reader

Only `internal/appflows` MUST parse `.corvint/flows.json`. The testing product and the documentation
product MUST both read the file through it. (G5; PROD-INT-004)

**Scenario: A second parser**
- GIVEN a package outside `internal/appflows` that opens `.corvint/flows.json`
- WHEN the source scan test runs
- THEN it fails and names the package

- `FSV-V1-004`: (accepted automatically under G14, decision 0596; V1-5410) An unsupported version fails closed with a typed reason

When the profile has the `corvint-flows/` prefix and a well-formed N outside the supported list, the
reader MUST refuse with `FLOWS_SCHEMA_UNSUPPORTED`, naming the found version and the supported list.
The affected plan MUST then select every E2E test with that reason. Unit and integration selection
MUST be unchanged. (G5; PROD-INT-016)

**Scenario: A future schema**
- GIVEN `.corvint/flows.json` with profile `corvint-flows/7` and a reader that supports 0 and 1
- WHEN the user runs `corvint affected --base FULL_SHA`
- THEN every E2E test is selected with reason `FLOWS_SCHEMA_UNSUPPORTED`, and the reason names
  version 7 and the supported versions 0 and 1

**Scenario: A malformed profile**
- GIVEN a profile `flows/1`
- WHEN the reader runs
- THEN it refuses with `FLOWS_DECLARATION_INVALID`, not `FLOWS_SCHEMA_UNSUPPORTED`

- `FSV-V1-005`: (accepted automatically under G14, decision 0596; V1-5410) Flow reports refuse rather than print partial output

`corvint flows` reports and every documentation-product report that reads the flow file MUST exit
non-zero with the reader's reason and print no partial report when the reader refuses. (G5;
PROD-DOCS-011)

**Scenario: Gaps on a newer file**
- GIVEN a flow file with an unsupported version
- WHEN the user runs `corvint flows gaps`
- THEN the command exits non-zero, prints `FLOWS_SCHEMA_UNSUPPORTED` and lists no gaps

- `FSV-V1-006`: (accepted automatically under G14, decision 0596; V1-5410) The writer emits a version its own reader supports

Every command that writes `.corvint/flows.json` MUST write the highest version in its build's
supported list, and that build's reader MUST accept the written file. (G5)

**Scenario: Round trip**
- GIVEN a build of `corvint`
- WHEN a flow file is written and then read by the same build
- THEN the read succeeds with the written version

- `FSV-V1-007`: (accepted automatically under G14, decision 0596; V1-5410) Release smoke test with `corvint` installed alone

The release workflow MUST install only the `corvint` binary from the candidate archive on an otherwise
empty `PATH` and run `corvint affected` and the PROD-DOCS-011 flow reports on a pinned fixture, once
with a supported flow file and once with an unsupported version. The candidate MUST NOT be offered for
approval if either run differs from its expected output. (G5; PROD-INT-016; PROD-INT-004)

**Scenario: Smoke on the RC**
- GIVEN the RC archive for linux/amd64
- WHEN the smoke job runs
- THEN the supported run prints the expected plan and reports, and the unsupported run plans every
  E2E test with `FLOWS_SCHEMA_UNSUPPORTED`

## Delivery and traceability

| Requirement | Task | Implementation | Evidence |
|---|---|---|---|
| FSV-V1-001 | 2.1 | not yet built | not yet built |
| FSV-V1-002 | 2.1 | not yet built | not yet built |
| FSV-V1-003 | 2.2 | not yet built | not yet built |
| FSV-V1-004 | 2.3, 3.1 | not yet built | not yet built |
| FSV-V1-005 | 2.4 | not yet built | not yet built |
| FSV-V1-006 | 2.5 | not yet built | not yet built |
| FSV-V1-007 | 2.6 | not yet built | not yet built |

## Rollback

Revert the reader change: the exact-profile refusal returns and every profile other than
`corvint-flows/0` reads as `FLOWS_DECLARATION_INVALID`. Remove the smoke job from the release
workflow. `FLOWS_SCHEMA_UNSUPPORTED` stays reserved; it is never reused for another meaning.
