# Corvint documentation

Start with the repository [README](../README.md) for installation, a first query, current release
status, support boundaries, and licensing. This directory separates current product and contributor
guidance from historical records.

## Current guidance

- [Product contract](PRODUCT.md) — product purpose, trust boundary, and supported direction.
- [Specification index](specs/README.md) — authoritative intent and delivery status.
- [Agent evidence routes](AGENT-ROUTES.md) — bounded routes from a task to original evidence.
- [Change Evidence Map](CHANGE-EVIDENCE-MAP.md) and [CEM in CI](CEM-CI.md) — change-evidence
  format and integration.
- [MCP server](MCP-SERVER.md), [integration ecosystem](EXTENSION-ECOSYSTEM.md), and
  [Beamfall integration](BEAMFALL-INTEGRATION.md) — integration contracts and current limits.
- [External evidence providers](EXTERNAL-EVIDENCE-PROVIDERS.md) — the provider record format,
  `context.external`, authority, and privacy boundary for a third-party evidence source.
- [Architecture](ARCHITECTURE.md), [development contract](SPEC-DRIVEN-DEVELOPMENT.md), and
  [dogfood contract](DOGFOOD.md) — contributor guidance.
- [Release notes](RELEASE-NOTES.md) and [public-alpha detail](RELEASE-NOTES-alpha.md) — current
  candidate status. Neither file is publication evidence.

## Repository layout

- `cmd/` contains executable entry points; `internal/` contains their Go implementation.
- `script/` runs build, release and maintenance checks; `tools/` contains development and
  evaluation programs. `conformance/` holds contract fixtures and checks, `interop/` independent
  protocol consumers, `testing/` retrieval goldens, and `tests/` the legacy-test migration map.
- `examples/` shows integrations; `extensions/` holds the VS Code extension, `integrations/`
  agent-host packages, and `assets/` shared visuals.
- `benchmarks/` preserves evaluation definitions and results. `experimental/` contains opt-in
  analyzer experiments.
- `docs/` contains current guides, specifications, and decision records. `.corvint/` holds tracked
  repository policy; `.agent-evidence/` carries the measurement pinned by the release-artifact check.

## Decision records and planning inputs

- [Decisions](decisions/README.md) preserve accepted, rejected, and superseded decisions.
- `plans/` holds the two planning inputs that current specifications and scripts still read.
- `agent-memory/` holds the six cross-session backlogs (bugs, fixes, tests, optimizations, ideas,
  questions) that the console's backlog pane lists; they start empty in the public tree.
- `BUILD-LOG.md` is the append-only evidence log for this repository; it starts empty in the public
  tree.

Decision records and specifications were written against internal working records (build logs,
reviews, evidence transcripts, plans, and backlog entries) that are not part of the public tree.
A backticked path under `docs/build-log/`, `docs/reviews/`, `docs/evidence/`, `docs/receipts/`,
`docs/plans/`, or `docs/agent-memory/` that does not resolve names one of those records; treat the statement as historical context, not as a live reference. Records can contain
old commands, paths, versions, and execution state. Follow them only when a current specification,
decision, or guide explicitly directs you there.
