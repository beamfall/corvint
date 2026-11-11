# 2026-11-11: non-code map (V1-5424)

Change `non-code-map`, tasks 1 to 5. Requirements NCM-V1-001 to NCM-V1-009 in
`docs/specs/non-code-map-v1.md`. Accepted automatically under G14 (decision 0596): owner answers
G2, G3, G15b and G16, owner decisions R31 and R32; the acceptance-log row `non-code-map` (kind
`change`, mode `auto`) records it.

## What changed

- `internal/noncodemap` (tests layer; one prefix line in `tools/import-ratchet/areas.json`):
  the strict reader of `.corvint/data-map.json` (`corvint-data-map/0`), entry status (`proposed`,
  `ignored`, `stale`, `unproven`, `proven`), the never-narrow list, the per-path decision with a
  named reason, and the proposal from recorded reads.
- The AFP-V0-012 selector (`internal/gateselect`, read by `corvint test` and
  `tools/gate-affected-select`) applies the decision before attribution. It prints one
  `noncode <reason> <path>` line per decided path and one `data-map <key> <status>: <detail>` line
  per covering entry, and falls back with `FALLBACK <reason> <path>` when any path needs a full
  run. `UNOWNED_DIRTY_PATH` is off the AFP-V0-982 allowlist (R32).
- `corvint affected --propose-data-map RECORDED` writes a proposal to stdout and no file, with its
  help text, catalog entry and `docs/CAPABILITIES.md` sentence.
- `.corvint/test-read-scopes.json`: `internal/perfbudget` builds `./cmd/corvint`, so it declares
  `internal/noncodemap/`.

## Decisions

1. OWNER-delegated orchestrator call on the design's open question 1 (2026-11-11):
   `.corvint/changes/`, `.corvint/deferred-intents/`, the committed inventory file and the
   `unplanned-reads.enabled` marker stay configuration for 1.0 (full run, `corvint-config`). Open
   question 2 (folding the read declarations into the map) is out of scope and stays with the
   owner.
2. Where the decision is enforced: the selector, not the affected plan. The plan keeps its
   `UNOWNED_DIRTY_PATH` entries and scope `UNKNOWN`. Adding a member to `affected-plan/1` would
   change frozen Core output, which the lane charter forbids.
3. What a non-code path is: a path the plan reports as `UNOWNED_DIRTY_PATH`, plus the `.corvint/`
   and never-narrow overrides applied to every changed path. A non-code path whose every possible
   reader the planner proves selected (`goReadData`, AFP-V0-600), or that an existing reviewed
   declaration classifies, keeps its attribution. That is "every test that might read it". It
   leaves Corvint's existing read declarations unchanged, which is a non-goal of the proposal.
   `non-code-unmapped` names an unowned path a Go package directory encloses or a map entry
   covers; `UNOWNED_DIRTY_PATH` names one no runner's tree holds.
4. The qualified selector re-observes with the Go plugin only, so it reports another language's
   source (a Playwright spec) as unowned. Ownership is the full plan's, so it sets aside an unowned
   entry the receipt does not also report.
5. go:embed inputs: the selector's index records that a package embeds, not which files, so every
   non-Go file below an embedding package counts. This can only widen. Example: this change edits
   `internal/capcatalog/catalog.json`, so `corvint test` on this branch plans FULL.
6. A missing map is empty, so every non-code path runs everything (the design's rollback). An
   invalid map is refused: `FALLBACK data-map-invalid: ...` in the selector, exit 2 in
   `--propose-data-map`.

## NCM-V1-004 and task 6

No FIH-V1 resolver exists until fault-injection-harness V1-5405 lands. `Env.FaultCase` is nil, so
every accepted entry with a proof is `unproven`, with detail `fault case <proof> is unknown:
FIH-V1 is not linked (V1-5405)`, and no entry can narrow. This is the intended fail-closed state.
The narrowing side is decided and unit-tested with a stub resolver (`proven-entry`). The selector
does not yet select a proven entry's tests: that is task 6's wiring. Task 6 stays unticked.

## Superseded narrowings

The spec's Supersession section lists them: AFP-V0-050, AFP-V0-152, AFP-V0-2744, AFP-V0-2746,
AFP-V0-3423, ADN-V0-011, TSP-V0-608, AZU-V0-005, AZU-V0-9920, AFP-V0-9012, CIN-V0-860 (V1-3480),
AFP-V0-9013, AFP-V0-9310, AFP-V0-980, AZU-V0-9900, AFP-V0-2377 and the TSP-V0-006 assembly
case. Each test that asserted the narrowing now asserts the named full run. Where `packageBounds`
still applies the rule (c) attribution, the test keeps covering it through an attribution-only
helper (`attributeFrom(..., nonCode=false)`). No golden was re-recorded.

## Task 7 (not ticked)

Missing: a committed fixture repository for Go, one JS runner, pytest and one Ruby runner, each
with an unmapped fixture, a proven entry, an unproven entry, a stale entry and a never-narrowed
path. A proven entry cannot exist before task 6. The Go scenarios exist only as in-test fixtures
in `internal/gateselect/noncode_test.go`, and no non-Go runner fixture was built.

## Verification

Focused tests, all passing on the final tree: `internal/noncodemap`, `internal/gateselect`,
`internal/sessiontest`, `internal/liveverify/affected/liverepo`, `tools/gate-affected-select`,
`tools/selection-baseline`, `tools/shell-test-select`, `tools/ci-forward-row`,
`tools/affected-zero-unknown-replay`, `tools/linecov`, `tools/ci-reuse-plan`, `internal/linecov`,
`internal/releasecandidate`, the capability-catalog packages, and filtered runs of
`internal/liveverify/affected`, `internal/liveverify/affected/golang` and `cmd/corvint`. The
handback lists the commands. `openspec validate`: NOT_RUN (the OpenSpec CLI is not installed).
Independent review: NOT_RUN in this lane; the orchestrator runs it.
