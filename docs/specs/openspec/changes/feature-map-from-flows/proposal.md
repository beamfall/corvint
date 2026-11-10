# Change proposal: the feature map is generated from `.corvint/flows.json` (preview)

Change name: `feature-map-from-flows`
Ticket: none filed yet (to be filed in `.taskman/` before dispatch).
Product requirement served: PROD-DOCS-013 (the feature map from flows), PROD-DOCS-004, PROD-DOCS-006
and PROD-DOCS-011 in `docs/specs/PRODUCT-PLAN-1.0.md` (PROD-DOCS-013 added 2026-11-09, review docs
F8)
Owning capability spec: a new spec `docs/specs/feature-map-v1.md` (prefix `FMP-V1`). It does not
exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: FMP-V1-001 to FMP-V1-007 (FMP-V1-007 added 2026-11-09 for owner
decision R30). `FMP-` has no entry in
`docs/specs/REQUIREMENTS.tsv` (checked 2026-11-09 on `claude/layers-integ` at `9ec3a29d0`).
Owner answers and decisions this change implements: G6, G25 items 4 and 6, owner answer 10, owner
decision R12 (2026-11-09) and owner decision R30 (2026-11-09)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started. Release label: preview for 1.0; it never blocks the release. G25
labels only items 1 and 2 as preview explicitly; this proposal applies the same label to item 4 as
the task directs.
Written before dispatch: yes.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/feature-map/spec.md`.

## Why

G25 item 4: `feature`, `features` and `overview` fold into flows; `corvint-docs` generates the
feature map from `.corvint/flows.json`; the regex heuristics are removed. G25 item 6: a model may
propose flow declarations, Corvint verifies them, and the owner accepts. A feature map inferred by
regex has no project authority (invariant 3) and duplicates what the flow file declares.

### Today's state, from the code at `9ec3a29d0`

- `feature` is a top-level command (`cmd/corvint/main.go:117-118`); `features`, `overview` and
  `review` are listed at `main.go:644-646`.
- `features`, `overview` and `review` share `cmd/corvint/repository_guidance.go` (751 lines;
  accepted at `:132`). Feature discovery uses two regexes, `guidanceMarkers` (`:281`) and
  `guidanceRegistrations` (`:282`); the receipt profile is `repository-guidance/0`, experimental,
  authority `inferred` (`:310`); a feature is added from inferred evidence at `:429`
  (`addGuidanceFeature` at `:415`).
- The code behind the single-feature `feature` command was not read for this proposal: NOT_VERIFIED.
- `.corvint/flows.json` (profile `corvint-flows/0`, `internal/appflows/flowplan.go:31-32`) declares
  flows; `corvint-docs docs flows generate --flows FILE` already groups declarations into the
  project's declared flows and reports unassigned declarations instead of inventing flows
  (`cmd/corvint-docs/docs_flows.go:160-170`).

## What Changes

- `corvint-docs` generates the feature map from `.corvint/flows.json`: one entry per flow, its
  variations, the routes or screens each declares, and the linked tests from accepted link records,
  each cited to the flow file at its commit.
- The map is an output of the existing `corvint-docs docs flows generate` command, not a new verb
  (settled without the owner, 2026-11-09: that command already groups declarations by flow).
- `corvint feature`, `corvint features` and `corvint overview` are removed. In 1.0.x each prints a
  fixed notice naming the `corvint-docs` command that replaces it and exits 2. The notices are
  removed in 1.1 (review docs F13).
- The map's output profile is named `corvint-feature-map/0` (review docs F13; FMP-V1-006).
- The regex heuristics and the `repository-guidance/0` profile are removed.
- Flow declarations proposed by a model enter the map only after a person accepts them (G25 item 6,
  PROD-DOCS-012). Link records are accepted by a person only (owner decision R12, 2026-11-09).
- If the map is offered over MCP, it is served only by `corvint-docs mcp`, with the name, input
  schema, output profile and error codes of its row in the table of change `mcp-tool-table`;
  `corvint mcp` does not serve it (owner decision R30, 2026-11-09; FMP-V1-007).

## Capabilities

### New Capabilities

- `feature-map` in `corvint-docs`.

### Modified Capabilities

- `feature`, `features`, `overview`: removed from `corvint`.

## Impact

- Removed: `cmd/corvint/repository_guidance.go` (its review part is removed by change
  `review-evidence-check`), the `feature` command's implementation, their tests, MCP tools if any
  (NOT_VERIFIED), and their entries in `docs/CAPABILITIES.md`.
- Added: a map generator in `corvint-docs` on the shared flow reader (docs layer imports shared only,
  `tools/import-ratchet/areas.json:4`).

## Non-goals

- Inferring features from code, names or history.
- Ranking features or files.
- Making the map a release gate.
