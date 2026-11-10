# Change proposal: Corvint and corvint-tasks release coupling, file contract and integration points

Change name: `tasks-integration`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-TASK-003, PROD-TASK-004, PROD-TASK-006, PROD-TASK-007,
PROD-TASK-008, PROD-TASK-009, PROD-TASK-010 to PROD-TASK-013 and PROD-INT-004 in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/tasks-integration-v1.md`, a new spec in this repository with its
own prefix `TIN-V1`. It covers only the Corvint side of the contract and the release coupling; the
Tasks requirements themselves live in `beamfall/corvint-tasks`.
Requirement IDs this change adds: TIN-V1-001 to TIN-V1-014, with TIN-V1-005 retired (thirteen live
IDs; TIN-V1-012 replaced TIN-V1-005, TIN-V1-013 was added 2026-11-09 for owner decision R17, and
TIN-V1-014 was decided 2026-11-09 under R42, C23)
Owner answers this change implements: G6, G9, G25, G27, answers 11, 12 and 13
(`owner-answers-2026-11-09.md`), owner decision R4 (2026-11-09), owner decision R17 (2026-11-09)
and owner decision R18 (2026-11-09)
Related changes, referred to by name: `tasks-conformance` (conformance record format and criteria),
`tasks-policy-v15` (the queue policy that carries the I1 gate), `review-evidence-check` (owner of
the `corvint review` JSON)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/tasks-integration/spec.md`.

## Why

The owner coupled the releases (G9): Corvint 1.0 cannot ship until corvint-tasks 1.0 passes its
own conformance pack, with a 21-day takeover trial on the frozen RC binary. The two products stay
loosely coupled through files and identifiers, with no shared library (answer 13). G9 names two
integration points: I1, a ticket gate that runs `corvint affected` and `corvint test` and stores
the receipt ID; and I2, a ticket that cites a flow or variation ID. G25 adds the revived `review`
evidence check as an input to the Tasks completion gate. G27 orders the spec cleanup: Tasks specs
in this repository are deleted only after corvint-tasks PR #4 merges.

The product plan already states the coupling, pair and I1/I2 at product level (PROD-TASK-010 to
013). This change gives them testable Corvint-side requirements.

## What exists today

Verified on 2026-11-09:

- The installed Tasks binary reports version `1.0.0-rc.3.dev+build.144`, with formats
  `taskman-ticket/0`, `taskman-receipt/0`, `taskman-receipt/1` and `taskman-gate-result/0`
  (`corvint-tasks version`). There is no ticket schema v1 and no "receipt reference" format yet.
- `corvint-tasks gate run --attempt ID --generation G --request-id ID --gate GATE` exists
  (`corvint-tasks gate run --help`). Which commands a gate executes is set by queue policy; whether
  a policy gate can already run `corvint affected` and `corvint test` and keep a receipt ID is
  NOT_VERIFIED.
- `corvint test` receipts (`corvint-test-run/0`, `internal/sessiontest/manifest.go:28`) have no
  receipt-ID member (`internal/sessiontest/run.go:94-120` lists base, checks, digests, profile,
  result, revision, runner profile, scope, shard fields, source, summary and tool). I1 therefore
  needs a defined ID.
- Tasks already stores and checks receipt files by content: `corvint-tasks receipt put`, `receipt
  get` and `receipt audit --witness-receipts` (TOL-V0-134, TOL-V0-136, TOL-V0-138). No Corvint
  resolver is needed for I1.
- Tasks has no MCP server today. Owner decision R17 (2026-11-09) keeps it that way for 1.0: Tasks is
  CLI only, so the MCP parity item in TIN-V1-002 is dropped. Earlier wording: "MCP parity in
  TIN-V1-002 is new work."
- corvint-tasks PR #4 ("Import Corvint Tasks from Corvint 4663b4e2b (Corvint decision 0594)") was
  OPEN when checked with `gh pr view` on 2026-11-09.
- Tasks specs still in this repository are retired by decision 0594 (for example
  `docs/specs/tasks-criterion-binding-v0.md`). `panels/removal-r5.md` counts 36 retired Tasks specs;
  that count was not re-checked here (NOT_VERIFIED).
- `review` today is `snapshotReviewAffected` in `cmd/corvint/repository_guidance.go:681`, built on
  the affected engine with no CEM dependency. The G25 evidence-check design is not built.

## What Changes

- A release check that refuses to sign the Corvint RC without a passing Tasks conformance record.
- A shared fixture for ticket schema v1 and receipt reference v1, checked in both repositories' CI.
  Proposed profile names: `taskman-ticket/1` and `taskman-receipt-ref/1`. corvint-tasks owns and
  publishes both (owner decision R17); Corvint keeps a vendored copy pinned by SHA-256. Neither name
  exists yet: `corvint-tasks version` lists `taskman-ticket/0`, `taskman-receipt/0` and
  `taskman-receipt/1` only.
- Corvint calls only the commands on the supported-command list that corvint-tasks publishes (owner
  decision R17; TIN-V1-013).
- A defined `corvint test` receipt ID: the SHA-256 of the receipt file bytes, stored and checked
  with `corvint-tasks receipt put`, `receipt get` and `receipt audit --witness-receipts`. No
  `corvint receipt resolve` command is added (fix release-tasks 6; TIN-V1-012).
- A flow-citation resolver over `.corvint/flows.json` for I2.
- The completion gate's blocking input is `corvint affected` plus the stored `corvint test` receipt
  (TIN-V1-012). A `corvint review` report, whose JSON is owned by change `review-evidence-check`, is
  recorded as advisory evidence unless the queue policy opts in to blocking (owner decision R4).

## Capabilities

### New Capabilities

- `tasks-integration`: coupling check, file contract, I1, I2, review input, trial record.

### Modified Capabilities

- `session-test-run`: receipt ID (additive).
- `review` (owned by change `review-evidence-check`): a stable JSON output the gate may record.

## Impact

- Both repositories' CI gain the shared fixture. Release manifest records the paired Tasks version.
- No new import between the two codebases.

## Non-goals

- No Tasks tools in `corvint mcp`, and no Tasks MCP server in 1.0 (owner decision R17). Earlier
  wording, superseded: "Tasks serves its own MCP (answer 11)".
- No shared Go library between Corvint and Tasks (answer 13).
- No change to Tasks scope beyond its nine requirements PROD-TASK-001 to 009, CLI only (owner
  decision R17). Earlier wording: "its nine requirements plus MCP (G9)".
- `review` findings do not block the Corvint release in 1.0 (G25).
