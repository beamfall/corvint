# Spike answers: V1-1272/V1-1325 symlinked root guides, V1-1534, V1-1705, V1-1706, V1-1844

Decisions delegated to the orchestrator (decision 0548). No code was changed for these spikes.

## V1-1272 and V1-1325: an AGENTS.md that is a symlink to a canonical CLAUDE.md

- **Query refusal (V1-1325).** `internal/contextindex/query.go:116` refuses an authority-start query
  with `unsupported-query-authority` unless there is exactly one top-ranked root `AGENTS.md`.
- **Codex governance (V1-1272).** A tracked symlink is a Git blob (mode 120000) whose content is the
  target path. The index does not treat it as an authority document, so the Codex governance context
  reports `AGENTS.md` as unavailable.

Both observations are the same capability gap, and both fail closed.

**Proposed recommendation:**

- **1.0:** document that tracked symlinked guides are not authority documents. Consumers keep the
  canonical file, as the ticket's own fallback does, and the refusal message should name the symlink
  so the cause is actionable.
- **Post-1.0 idea:** resolve a tracked symlink whose target is a tracked regular file in the same
  tree at the same revision. That resolution is pure pinned Git content, so it does not weaken
  containment.

Never follow links that leave the tree.

## V1-1534: xcode-xctest zero tests and build errors are reported as INVALID_NATIVE_REPORT

**Proposed recommendation:**

- A destination-less summary with a build-failure signal maps to the `BUILD` problem kind.
- One with a zero-test signal maps to `NO_TESTS`.
- `INVALID_NATIVE_REPORT` is kept for genuine disagreement between destination reports.

This is a follow-up build. It needs fixture xcresult summaries captured from
`/private/tmp/cem10-build/platform-2026-10-17/runs/xcode-xctest/` and requires Xcode, which this
lane did not run.

## V1-1705: Spec Kit presence from the tree inventory or from admitted sources

**Proposed recommendation:** keep admitted sources. If a project excludes `.specify/`, that is a
project-owned authority decision (invariant 3), and taking presence from the raw tree would override
it. Document in STA-V0-007 that presence follows the admitted source set.

## V1-1706: versioning the Spec Kit and OpenSpec integration packages

**Proposed recommendation:**

- Each host package's version bumps whenever any file the package ships changes.
- `requires.tools` declares the minimum Corvint version whose CLI surface the package uses.
- Release notes list package version changes.

Follow-up: a check that fails when a file under `protocol/integrations/<host>/` changes without a
matching version change in that package's manifest.

## V1-1844: warning and salvage fields on `finished`, or a per-worker `stopped` event

**Proposed recommendation:** accept the ATR-V0-127 mapping. Keep the fields on the worker's
`finished` event. The dispatcher's `stopped` event is program-level and names no worker, so a
per-worker `stopped` event would duplicate `finished` and add nothing new.
