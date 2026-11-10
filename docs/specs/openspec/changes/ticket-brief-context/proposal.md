# Change proposal: `corvint context` as a ticket starting brief from declared links (non-blocking)

Change name: `ticket-brief-context`
Ticket: none filed yet (to be filed in `.taskman/` before dispatch).
Product requirement served: PROD-INT-030, PROD-TASK-006 and PROD-INT-002 in
`docs/specs/PRODUCT-PLAN-1.0.md` (the join from a ticket's declared requirements and flows to the
work).
Owner decisions this change implements: owner decisions R28 and R29 (2026-11-09), owner decision R30
(2026-11-09), owner decisions R39 and R40 (2026-11-09); review docs F10
Owning capability spec: a new spec `docs/specs/ticket-brief-v1.md` (prefix `TBC-V1`). It does not
exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: TBC-V1-001 to TBC-V1-008 (TBC-V1-008 added 2026-11-09 for owner
decision R30). `TBC-` has no entry in
`docs/specs/REQUIREMENTS.tsv` (checked 2026-11-09 on `claude/layers-integ` at `9ec3a29d0`).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started. Release label: ships in 1.0 as redesigned and non-blocking, with no
claim of measured benefit; the keep-or-remove measurement in TBC-V1-006 runs in 1.0.x (owner
decisions R28 and R29, 2026-11-09). Earlier wording, superseded: "preview for 1.0 ... kept after 1.0
only if the measurement in TBC-V1-006 passes (G25 item 3)".
Written before dispatch: yes.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/ticket-brief/spec.md`.

## Why

G25 item 3: `context` becomes a ticket starting brief built from declared links only, with no
ranking. Owner decisions R28 and R29 (2026-11-09) ship it in 1.0 as redesigned and non-blocking,
with no claim of measured benefit, and move the measurement of agent tool calls on a fixed task set
to 1.0.x. (Earlier: "It is kept only if it measurably cuts agent tool calls on a fixed task set;
otherwise it is removed.") G19 adds that agents get an end-of-turn report and no context injection, so the brief
is something an agent or a person asks for, never something a hook inserts.

### Today's state, from the code at `9ec3a29d0`

- `corvint context` compiles a ranked task-context packet (`cmd/corvint/taskcontext.go:143-175`,
  277 lines). It reads the `contextindex` snapshot when one exists, otherwise builds an index over
  the committed tree.
- The ranking code is nine non-test files in `internal/contextindex` totalling 5,472 lines
  (`wc -l`): `taskcontext.go`, `taskcontext_anchors.go`, `taskcontext_frame.go`,
  `context_anchors.go`, `context_order.go`, `context_terms.go`, `context_fuzzy.go`,
  `instruction_loadset.go` and `slot_weights.go`. The panel's figure of about 4,260 lines in
  `cmd/corvint/taskcontext*.go` does not match this tree: only `cmd/corvint/taskcontext.go` exists
  there.
- `contextindex.BuildContext` (`internal/contextindex/index.go:328`) and `contextindex.QueryIntent`
  (`internal/contextindex/query.go:114`, used by `cmd/corvint-docs/shared.go:315`) are used outside
  the ranking path and are kept.
- Tickets carry `requirementRefs`, `acceptanceCriteria` and `effects.touchPaths` (seen in
  `.taskman/tickets/V1-0019.json`). A field citing a flow or variation ID (G9 item I2) is
  NOT_VERIFIED in the current ticket schema.

## What Changes

`corvint context --ticket ID` prints a brief assembled only from declared links:

1. The ticket's title, body digest and acceptance criteria, read with `corvint-tasks ticket show
   <ID> --json` (no shared library, answer 13).
2. Each `requirementRefs` ID with its spec file and definition line (from
   `docs/specs/REQUIREMENTS.tsv`).
3. The ticket's declared paths (`effects.touchPaths`, claim scope).
4. Each flow or variation the ticket cites (G9 item I2), with its linked tests from accepted link
   records.
5. The affected plan for the declared paths, as `corvint affected` would print it for a change to
   them.

There is no ranking, no term matching and no learned weighting. Every item states which link
produced it. The ranking code is removed whether or not the brief is kept (G25 "still removed").

## Capabilities

### New Capabilities

- `ticket-brief`.

### Modified Capabilities

- `context` command: replaced. The ranked packet and its MCP form are removed.

## Impact

- Removed: the nine ranking files above, the ranked path of `cmd/corvint/taskcontext.go`, and their
  tests. `corvint eval --learn-slot-weights` loses its purpose; AGENTS.md product invariant 4 names
  it (LTA-V0-009 to LTA-V0-012) and needs a matching amendment.
- Added: a brief builder reading tickets through `corvint-tasks ticket show --json` (read only),
  `REQUIREMENTS.tsv`, `.corvint/flows.json` and the affected engine.
- If the 1.0.x measurement fails, the brief builder is removed too and `context` leaves the command
  list in a 1.0.x release.
- The measurement runs use the owner's logged-in subscription CLIs on the owner's machines, with no
  API keys and no spend cap (owner decisions R39 and R40, 2026-11-09).
- If the brief is offered over MCP, its tool follows its row in the table of change `mcp-tool-table`
  (owner decision R30).

## Non-goals

- Search, ranking or "relevant files" guesses.
- Injecting the brief through hooks (G19).
- Making the brief a release gate, or claiming a measured benefit in 1.0 documentation (R28, R29).
