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
Amendment of 2026-11-10: TBC-V1-009 to TBC-V1-014 add a path form of the brief that needs no
ticket, keep the requirement links of the sealed change records, and move a coverage check and a
pilot ahead of the build. They serve PROD-INT-032, which the owner accepted into 1.0 on 2026-11-10
("Accept into 1.0") and which has an `owner` row in `docs/specs/openspec/acceptance-log.tsv`.
PROD-INT-030 and TBC-V1-001 to TBC-V1-008 are unchanged. The coverage check runs first; the export
and the builder are written only if it meets the owner's bar, which is not set yet.
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

### Amendment of 2026-11-10: a path form for agents with no ticket

The owner asked on 2026-11-10 for a context command that is useful to every agent. The ticket form
above needs a Corvint Tasks ticket, so an agent in a repository with no ticket store gets nothing
from it. The amendment adds a second form with the same rules (declared links only, no ranking,
every item names its link, missing links stated):

`corvint context --path PATH [--path PATH ...]` prints six sections for the named paths at HEAD:
the instruction files in the path's directory chain, the matching `CODEOWNERS` rule, the tests tied
to the path by retained evidence and the affected plan, the flows through the path with their
linked tests, the requirements tied to the path, and the open tickets whose scope covers it
(TBC-V1-009 to TBC-V1-011). The ticket form is not changed; an agent with a ticket passes the
ticket's declared paths to the path form.

Evidence behind the amendment, recorded in
`docs/build-log/2026-11-10-context-replay-and-link-coverage.md`:

- Replay on 16 merged pull requests (39 changed source files): today's ranked `corvint context`
  listed 9 of the 39 files in its first eight results and a plain term grep listed 13. This agrees
  with the value audit of 2026-10-26: a ranked list of files to read does not help, because agents
  find the files unaided. The path form therefore answers a different question. It takes the path
  the agent has already found and lists what a search of the tree does not show: which tests cover
  it, which requirement governs it, which flow passes through it and who else is working on it.
- Requirement links: the 630 sealed `cem/1.0` records under `.corvint/changes` hold 17,481
  hunk-to-specification links and tie 442 of the 1,847 non-test Go files to a spec span. Change
  `source-removal` deletes these records with the ceremony (G8). The last 300 commits on
  `origin/main` carry no `Ticket:` trailer and 45 of their subjects cite a ticket ID, so commit
  history does not replace them. TBC-V1-012 keeps the links in one committed file and leaves G8 as
  accepted: no verb, no ceremony and no new record. The links are partial: 62% of them carry a
  requirement definition line and 27% carry no requirement ID, so the `requirements` section is
  never reported as complete.
- The measurement moves forward in two steps that claim nothing: a link-coverage check with no
  agent run before the builder is written (TBC-V1-013), and a pilot of the TBC-V1-006 design before
  the release candidate (TBC-V1-014). The 1.0.x keep-or-remove rule is unchanged.

A report keyed on a change range is not added: `corvint impact` (CRI-V1) is that report.

## Capabilities

### New Capabilities

- `ticket-brief`.
- Path brief (added 2026-11-10): the `--path` form of the same command, part of `ticket-brief`.

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

- Amendment of 2026-11-10, if the coverage check meets the bar: added, the path brief builder, the
  one-time link export and the committed `.corvint/requirement-links.json`. The export reads `.corvint/changes` at a
  named commit, so `source-removal` need not wait for it. Reading `CODEOWNERS` and nested
  instruction files is new code; the other four sections reuse the affected engine, the retained
  test evidence, `.corvint/flows.json` and `corvint-tasks`.

## Non-goals

- Search, ranking or "relevant files" guesses.
- Injecting the brief through hooks (G19).
- Making the brief a release gate, or claiming a measured benefit in 1.0 documentation (R28, R29).
- A brief keyed on a diff or a change range: that is `corvint impact` (CRI-V1).
- Reviving the `cem` verb, the ceremony or the writing of new change records (G8).
- Any section built from task text. The path form takes paths, never a description of the work.
