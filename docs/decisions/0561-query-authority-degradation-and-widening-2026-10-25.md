# Decision 0561 — `corvint query` degrades a missing root authority and widens beneath its confident results

Date: 2026-10-25. Status: accepted by the orchestrator lane under the owner's standing auto-accept
of lane requirements (2026-10-11); tickets V1-2716 and V1-2717 (from the V1-0415 ContextBench and
SWE-bench Pro negative results, beamfall/corvint#1832). It adds `GPK-V0-090` and `GPK-V0-091` to
`docs/specs/go-production-kernel-migration-v0.md`, `CEP-V0-007` to
`docs/specs/context-evolution-program-v0.md`, `AHI-210` to
`docs/specs/agent-harness-integration-v0.md`, register entry `DR-0045`, and retires the
`cli-parity-v0` refusal item `query-missing-authority-refusal`.

## Context

The full ContextBench rerun (N=1132 rows, 67 repositories) measured `corvint query` at the lowest
file recall of every arm: recall@20 0.239 against 0.360 for the `bm25:ident` baseline and 0.480 for
`corvint context`, with a median of 4 files per answer and 28 errors, 20 of them
`unsupported-query-authority: requires one uniquely highest-ranked root AGENTS.md`. The confident
tier is precise (0.249 at its own length) but short; the task-context slot ranking over the same
index names most of the gold files the confident tier misses. A row whose task matched two
project-operations terms over a repository with no root `AGENTS.md` returned an error instead of an
answer with stated uncertainty, which product invariant 2 does not permit when a bounded answer
exists.

## Decision

1. `GPK-V0-090`: the standalone `corvint query` receipt (and its byte-equal `batch` query
   operation, `SBQ-V0-003`) carries one additive member, `widening`: the task-context rows for the
   same task over the same pinned index that `results[]` does not already name, bounded by the
   requested limit, with governing rows excluded, a fixed disclosure that the rows are
   lower-confidence and carry no authority, and the packet budget yielded to `results[]` first. The
   ranking runs even when the confident rows fill the limit, so an eligible candidate is counted in
   `omitted_results` under state `QUOTA_FILLED` and `NO_CANDIDATES` means the window was examined
   and held nothing eligible. The
   confident `results[]`, `state` and abstention are unchanged. The harness user-prompt packet and
   `prove` are not widened.
2. `GPK-V0-091`: when the authority-start profile refuses only because the tree ranks no uniquely
   highest root `AGENTS.md`, the command adapter answers through the repository profile with the
   degradation `authority-start-unavailable` named in `degradations` and in
   `coverage.uncertainty`. The symlink and unsupported-content authority refusals are unchanged.
3. The frozen oracle predates both. Under decision 0051 the Go-only member is a registered
   divergence (`DR-0045`, typed `wideningDivergence` declaration on the eight affected cases),
   not an oracle edit; the refusal item `query-missing-authority-refusal`, whose certified
   `unsupported-query-authority` outcome is now unreachable for that fixture, is retired under this
   decision the way decision 0308 and decision 0525 retired items whose frozen expectation became
   unreachable. `retiredRefusalDecisions` pins it to `0561`; `TestProjectOperationsQueryDegradesWithoutRootAgentsFile`
   and `TestProjectOperationsQueryStillRefusesSymlinkAuthority` carry the behaviour.

## Consequences

- `coverage.packet_bytes` of a standalone query receipt grows by the member; a budgeted packet
  still fits its budget because widening rows are dropped from the tail first and the member is
  removed when no row fits.
- `cli-parity-v0` reports `known-divergences=39 unsupported-refusals=0 retired-refusals=3`; the
  `query` inventory row stays `PARTIAL`.
- Measured outcomes against the tickets' acceptance criteria are recorded in
  `docs/build-log/2026-10-25-v2716-v2717-query-recall-opencode-adoption.md`, including every
  remaining negative result.

## Rollback

Revert the lane change: `cmd/corvint/query_widening.go`, the `withQueryWidening` calls in
`cmd/corvint/main.go` and `cmd/corvint/batch.go`, `authorityDegradedQueryContext`,
`internal/contextindex/query_degradation.go`, the `wideningDivergence` declaration and the
`query-missing-authority-refusal` retirement in `conformance/cli-parity-v0`, the regenerated
core-freeze goldens, the two spec amendments and this decision. The frozen manifest expectations
are unchanged, so the previous bytes replay as before.
