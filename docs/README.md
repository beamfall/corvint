# Corvint documentation

Start with the repository [README](../README.md) for what Corvint does for a coding agent, a
sixty-second quickstart, measured results, support boundaries, and licensing. This directory
separates current product and contributor guidance from historical records.

## Current guidance

- [Product contract](PRODUCT.md) — product purpose, trust boundary, and supported direction.
- [Specification index](specs/README.md) — authoritative intent and delivery status.
- [Agent evidence routes](AGENT-ROUTES.md) — bounded routes from a task to original evidence.
- [Capability catalog](CAPABILITIES.md) — every command, binary and MCP tool: what it answers,
  the smallest call, maturity, refusals and owning spec.
- [Change Evidence Map](CHANGE-EVIDENCE-MAP.md) and the [CEM 1.0 stable contract](specs/cem-stable-v1.md)
  — change-evidence format and integration.
- [Triggered automation](AUTOMATION.md#session-hooks-frontier-authority-unavailable) — session-hook
  degradation guidance, including adopters without CEM/OCM artifacts.
- [MCP server](MCP-SERVER.md), [integration ecosystem](EXTENSION-ECOSYSTEM.md), and
  [Beamfall integration](BEAMFALL-INTEGRATION.md) — integration contracts and current limits.
- [Change-compiled E2E adoption](E2E-ADOPTION.md) — the minimum application-map manifest and what
  each `corvint flows appmap build` and `corvint-test e2e` step needs committed (experimental).
- [External evidence providers](EXTERNAL-EVIDENCE-PROVIDERS.md) — the provider record format,
  `context.external`, authority, and privacy boundary for a third-party evidence source.
- [Architecture](ARCHITECTURE.md), [development contract](SPEC-DRIVEN-DEVELOPMENT.md), and
  [dogfood contract](DOGFOOD.md) — contributor guidance.
- [Release notes](RELEASE-NOTES.md) and [public-alpha detail](RELEASE-NOTES-alpha.md) — release
  history and per-release qualification. Neither file is publication evidence.

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
- `agent-memory/` holds six historical redirect pages (bugs, fixes, tests, optimizations, ideas,
  questions); the console's Backlog history pane lists them, but they are not the live queue.
  Current work is tracked in the Corvint task store (`.taskman/`), read with `corvint-tasks queue
  status`, `roadmap`, or `ticket search`.
- `build-log/` holds one file per build-log entry (decision 0423). Entries from before 1.0, the
  closed `BUILD-LOG.md`, sealed CEMs, closed tickets and dated results live in the
  [`beamfall/corvint-history`](https://github.com/beamfall/corvint-history) archive at
  `1c4d8793cf71cbb01b8509b68a6cb1502a40a13e` (decision 0587).

Decision records and specifications were written against internal working records (build logs,
reviews, evidence transcripts, plans, and backlog entries) that are not part of the public tree.
A backticked path under `docs/build-log/`, `docs/reviews/`, `docs/evidence/`, `docs/receipts/`,
`docs/plans/`, or `docs/agent-memory/` that does not resolve names one of those records (the pre-1.0 ones resolve in `beamfall/corvint-history` at
`1c4d8793cf71cbb01b8509b68a6cb1502a40a13e`); treat the statement as historical context, not as a live reference. Records can contain
old commands, paths, versions, and execution state. Follow them only when a current specification,
decision, or guide explicitly directs you there.
