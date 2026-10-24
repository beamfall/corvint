# skh-a: site knowledge stage 4 — static navigation edges beside observed ones

Date: 2026-10-24. Lane: cloud skh-a (V1-1901). Spec: `docs/specs/site-knowledge-howto-v0.md`.
Platform for every result below: linux/cloud. Dogfood targets: NOT_PRODUCED (cloud lane).

## Owner authority

New requirements `SKH-V0-110`..`115` come from the lane's reserved block `SKH-V0-100`..`139`.
They are accepted under the owner's standing auto-accept (2026-10-11).

## Decisions

- **A projection, not a map change (SKH-V0-008, 040, 110).** Static edges are printed by a new
  `flows appmap links` report next to the map's own edges. Each edge carries a closed provenance:
  `static`, `test-observed` or `flow-declared`. `application-map/0` and its digest are unchanged,
  so no existing map, ledger or impact report changes meaning. PROVEN edges stay in the ANL-V0
  ledger, and the report names that command in `ledger`. Writing static edges into the map file
  would have changed a shared wire contract for a stage that only reads it. This is recorded here
  as the `SKH-V0-008` reason for a new verb in the existing `flows appmap` family. No new command
  family or MCP tool is added: the corpus tool gains a `links` view.
- **No second capture path (SKH-V0-042).** Nothing is observed or run. A static edge only says a
  link exists in source. When the map also observed the same screen pair, the static record is
  marked `observed`, and both records are kept.
- **Source screens only from Rails conventions (SKH-V0-112).** The map does not bind templates or
  components to screens for ui-router or React states, and no `templateUrl` or `component` is
  kept. Guessing one from file names would invent certainty. So a non-Rails source is
  `source-not-bound`. A Rails view and a controller `def` bind through the compiled `GET` routes
  that `actions` and `branches` already use.
- **No dialect widened (SKH-V0-041).** There was no adopter repository in the cloud lane, so no
  need could be measured. Targets resolve only to screens the existing dialects produce. The
  measured-need clause is NOT MET (NEEDS-HOST).
- **Lexical by construction (SKH-V0-004, 043).** Attributes and calls are read by regular
  expressions over committed lines, and only literal values resolve. Interpolation, bindings, JSX
  expressions and non-literal arguments are `dynamic-target` at their line. Template languages the
  reader does not parse are named unknowns, never read.

## Evidence (linux/cloud)

- `TestSKHV0_110_115_StaticNavigationEdges` in `internal/appmap`: ok. It covers 21 static edges
  across ERB, Ruby, TSX and HTML; dynamic, relative, ambiguous and unbound cases; external links
  skipped; one HAML unknown; and the observed and flow-declared map edges listed separately.
- `TestSKHV0_013_PageObjectsAndStepsCLI` (`links` verb) in `cmd/corvint`, and
  `TestSKHV0013CorpusMCPMapKnowledge` (`links` view) in `cmd/corvint-corpus-mcp`: ok.
- Real-adopter static and observed edge counts: NOT_RUN. The first adopter is not reachable from
  the cloud session.

## Limits

- Line-based reading: a link split across lines, or one inside a block comment that does not
  start each line with `*`, may be missed or read.
- `/invoices/new` stays `ambiguous-template` against `/invoices/{}`. That is the existing AMAP-V0
  screen-index rule, and this stage does not change it.
