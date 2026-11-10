# Change proposal: a living contract for `corvint how` under corvint-docs

Change name: `how-rewrite`
Ticket: none filed yet (task 1.1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-DOCS-007 and PROD-INT-004 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/how-v1.md`, a new spec with its own prefix `HOW-V1`. It replaces
the retired `site-knowledge-howto-v0.md` (retired by decision 0593) as the living contract for the
`how` verb. SKH-V0 IDs stay in place for traceability; they are not reused or renumbered.
Requirement IDs this change adds: HOW-V1-001 to HOW-V1-012 (HOW-V1-011 added 2026-11-09 for owner
decision R30; HOW-V1-012, formerly DMF-V1-012, for R42 (panel pick) B1)
Owner answers and decisions this change implements: decision 0595 (CMS-V0-912 to 916), G5
(`owner-answers-2026-11-09.md`), owner decision R30 (2026-11-09) and owner decision R31
(2026-11-09)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/how/spec.md`.

## Why

PROD-DOCS-007 promises `corvint how --map FILE --source e2e GOAL`. That promise has no surviving
requirement: the spec that defined it, `site-knowledge-howto-v0.md`, is retired, while the code still
cites its IDs. Decision 0595 moved the `flows` and `how` handlers into corvint-docs and left
`corvint flows` and `corvint how` as exec-only shims, keeping every argument vector, flag, output
schema, exit code and refusal. G5 then drew the boundary: flow reports are native in corvint, and
authoring, discovery, proposals and `how` stay in corvint-docs.

"Rewrite" here means a re-specification only: a new living spec that states today's behaviour as
numbered requirements, with one output change: HOW-V1-012 names the stale anchors and the alias in
the gap `detail` text (R42 (panel pick) B1).

## What exists today

Verified in `integ` at 9ec3a29d0 unless stated:

- Handler. `internal/command/flows/flows_appmap_how.go:11-21` (`HowHelp`) gives the usage
  `corvint-docs [--root PATH] how --map FILE --source [ALIAS:]DIR... [--repo ALIAS=ABSOLUTE_ROOT]...
  [--condition TEXT]... [--ledger FILE] [--corpus ARTIFACT] [--budget N | --full] GOAL`, and says
  `how` is an alias of `corvint-docs flows appmap how` with the same flags and the same
  `site-knowledge-how/0` bytes, that refusals exit 2, and that it writes nothing.
  `runFlowsAppmapHow` (`:26-58`) parses `--map`, `--corpus`, `--ledger`, `--budget`, `--full`,
  repeatable `--source`, repeatable `--repo` (bound by `bindSourceRepos`), repeatable `--condition`,
  and exactly one GOAL; its error text also names `--workspace FILE` and `--no-workspace`. The
  appmap help text in `internal/command/flows/flows_appmap.go` (integ 9ec3a29d0, around lines 48-53
  and 123-127) states their meaning: the source verbs take `--workspace FILE` (a
  `corvint-workspace/0` declaration binding aliases to roots) or read `--root/.corvint/workspace.json`
  when it exists, unless `--no-workspace`; a workspace never merges with `--repo`, and an invalid
  one refuses as `workspace-invalid`. `writeHow` (`:62`) is shared with
  `flows appmap plan --request GOAL --draft --source DIR`.
- Schema. `internal/appmap/site_knowledge_how.go:22` sets `HowSchema = "site-knowledge-how/0"`.
  States are COMPLETE, INCOMPLETE, STALE and ABSTAINED (SKH-V0-063, SKH-V0-144). Gap codes are
  `no-route`, `no-setup-for-precondition`, `no-oracle`, `no-page-object`, `ambiguous-goal` and
  `stale-evidence`. The goal limit is 256 bytes (`maxGoalBytes`). Output carries the label
  "proposal; learned authority, not accepted intent". Inputs are `application-map/0`,
  `application-edge-ledger/0` and the documentation corpus artifact.
- Wiring. `cmd/corvint-docs/flows.go:46-61` routes `how` to the appmap handler (SKH-V0-810).
  `runFlowsShim` in `cmd/corvint/flows_shim.go` (line 52 at integ `9ec3a29d0`) is the forwarding
  shim; its `test-evidence` branch (line 58 at the same SHA) keeps `flows test-evidence` native
  (CMS-V0-914). `internal/command/flows/flows_appmap.go:283-345` dispatches the appmap verbs.
- Decision 0595 exists in `integ` only; the `claude/product-specs` branch holds decisions up to
  0594. Its tests are `TestCMSV0912EveryFlowsSubcommandIsInTheCommandTree`,
  `TestCMSV0915FlowReaderIsSharedAndFlowCommandsAreDocs` and `TestImportRatchet`. It states that an
  older corvint-docs answers an unknown verb with an argument error; that is inferred and untested.

## What Changes

- A new spec `how-v1.md` with the requirements below, each citing the SKH-V0 or CMS-V0 IDs it
  carries forward.
- Golden tests that pin the current argument vectors, refusals and output bytes, where missing.
- No code change is expected beyond tests; any gap found becomes a ticket, not a silent fix.
- `how` is offered over MCP as one tool, `corvint.docs_how` (decided 2026-11-09 under R42, B10).
  It is served only by `corvint-docs mcp`, under the name, input schema,
  output profile and error codes fixed by change `mcp-tool-table` (owner decision R30, 2026-11-09).
  `corvint mcp` keeps the testing tools only and does not serve `how`. Integ 9ec3a29d0 has no `how`
  MCP tool today (`internal/capcatalog/mcp.go` lists none).

## Capabilities

### New Capabilities

- `how`: the goal-to-steps proposal verb, owned by corvint-docs.

### Modified Capabilities

- `docs-and-flows-split` (decision 0595): no change; this spec cites it.
- `site-knowledge-howto-v0`: stays retired; its IDs are referenced, not revived.

## Impact

- `cmd/corvint-docs`, `cmd/corvint/flows_shim.go`, `internal/command/flows`, `internal/appmap`
  tests only.
- One row in the MCP tool table owned by change `mcp-tool-table` (HOW-V1-011).

## Non-goals

- No new flag, schema member, state or gap code (HOW-V1-012 changes only the gap `detail` text).
- No move of `how` into the corvint binary; G5 keeps it in corvint-docs.
- No authority for `how` output: it remains a proposal, never accepted intent.
- No change to flow reports (gaps, status, impact, test-evidence), which G5 makes native in corvint
  under a separate change.
