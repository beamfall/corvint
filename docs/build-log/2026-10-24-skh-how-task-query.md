# p1-skh-b: site knowledge stage 6 — the how task query (V1-1903)

Date: 2026-10-24. Lane: local p1-skh-b (V1-1903, stacked on #1705).
Spec: `docs/specs/site-knowledge-howto-v0.md`. Platform for every result: linux/WSL2 (go1.27.3).

## Owner authority

`SKH-V0-060`..`064` are accepted under the owner's standing auto-accept (2026-10-11). The
refinements `SKH-V0-143`..`152` come from this lane's reserved block and are experimental.

## Decisions

- **New encoding `site-knowledge-how/0`.** One JSON answer: head (status, freshness, evaluated
  revision, match, sorted gaps, candidate reference, goal and start screens, oracle, ledger digest)
  and trimmable sections (setup, conditions, steps, path, candidates) under the AMSP-V0-009
  budget. It is a proposal with learned authority; nothing reads it as accepted intent.
- **Surface: `flows appmap how` plus a `how` view of `corvint.map_knowledge`.** A top-level
  `corvint how` needs the command registry, help, capability catalog and clean-command lists to
  change for one alias, so it is a follow-up ticket; `SKH-V0-060` reads PARTIAL for the alias
  only. The MCP view takes `goal` and `conditions` and reads no corpus and no ledger, like
  `flow_candidates`.
- **Resolution reuses stage 5 and the AMSP-V0 resolver.** Candidate flows from `inferFlows` are
  wrapped as plan flows with terms from source titles, step actions, start and end screen states
  and URLs and outcome text, and ranked by the existing `resolve`. A tie at the top coverage
  abstains with `ambiguous-goal`; no term covered abstains with `no-route`. Navigation composition
  (`AMSP-V0-006`) is not reused: the path is the primary test's own visits, and each hop lists the
  admitted edges that join its screens rather than a composed route.
- **Admitted edges keep provenance.** Static (stage 4), test-observed and flow-declared (map) and
  ledger `PROVEN` rows are listed side by side per hop, never merged. A ledger must be untrimmed,
  for the same app and map digest, and match its self-digest; otherwise the query refuses.
  `Links` and `SetupQuery` were split into `buildLinks`, `checkConditions` and `allSetup` so the
  query reuses their bodies; their output bytes are unchanged (package tests pass).
- **The oracle is copied, not synthesised.** It is the trailing assertions of the primary test's
  shared outcome. No shared outcome or a blocked candidate is `no-oracle`.

## Evidence

- `go test ./internal/appmap/` ok (43 s), `go test ./cmd/corvint-corpus-mcp/` ok,
  `go test -run 'SKHV0|Appmap|AppMap|Flows|Help|Usage|Capabilit|CAPV0|MCPV0' ./cmd/corvint/` ok,
  `go test ./internal/capcatalog/` ok; all linux/WSL2.
- `internal/capcatalog/catalog.json` regenerated (`make capability-catalog`): the new profile is
  listed.

## Rollback

Revert the stage 6 commit. Nothing persists the answer, so no stored state migrates.
