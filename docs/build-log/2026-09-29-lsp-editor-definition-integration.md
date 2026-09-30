# Experimental overlay editor definition integration

Owner direction asks for Go/gopls agent and optional VS Code/Neovim use. This candidate joins only
reviewed public component commits: snapshot add1fc8b plus storage repair 532e8ea5, stdio ad605e73,
and gopls client 36759d12. Public base is 75f06f9020a38af8a775a35479b4b16cc69b8e3c; integration
review delta starts at 17a428ef66dce6766cc167e5a10b5153a4c6c609, immediately after those cherry-picks.
No private lineage was imported. Technical intent remains proposed; this is an experiment.

The separate operator-configured command admits one canonical root and local binary. Definition
and full-text sync are advertised only after gopls negotiation. Exact retained lspsnapshot captures
join opaque per-session identities to synchronized gopls snapshots. One pending worker keeps the
reader available for cancellation; document mutation cancels and joins first. EOF aborts startup,
requests and owned process cleanup. Editor UTF-8/16/32 converts through validated boundaries to
UTF-16. Only open overlay targets are eligible; no Git, disk semantic or benefit claim follows.

## Observed verification

- `CORVINT_TEST_GOPLS=/tmp/corvint-lsp-integration/bin/gopls GOTOOLCHAIN=local go test -race -count=1 -timeout 30m ./internal/lspstdio ./cmd/corvint-lsp`: PASS (2.713s and 1.447s). Fake sessions cover cancellation, changes while pending, malformed incremental updates, multi-root rejection and EOF during hung initialization. Exact reopen capture identity and Unicode boundary tests pass.
- The same race run executes the live overlay fixture against explicitly supplied gopls v0.23.0, SHA256 `39431a5b273a5ac124a98521d1ec3e38af4bc5eec783e24a23344145eef24836`. Disk contains a different declaration; the unsaved overlay definition resolves after an emoji, with UTF-8 output character 13. This is integration evidence, not editor qualification.
- `GOTOOLCHAIN=local go vet ./internal/lspstdio ./cmd/corvint-lsp`: PASS.
- `GOTOOLCHAIN=local go test -race -count=1 -timeout 30m ./internal/specindex`: PASS (1.688s). Initial README claim mismatch failed, then was repaired and rerun.
- `make spec-requirements-check requirement-definitions-check traceability-tests-check`: PASS, with 3 existing planned traceability rows retained.
- Source diff whitespace check: PASS. Repository-wide gate NOT_RUN under the owner's scoped-work preference; focused checks do not imply equivalent coverage.

## Dogfood and remaining boundaries

Corvint context was used before edits; original receipt is `/tmp/lsp-integration-context.json`.
Affected advice was retained before focused tests and refreshed after staging in
`/tmp/lsp-integration-affected-final.json`. Advice conservatively selects broad units and repository
commands; their static presence does not override the owner's scoped-work instruction. Retrieval,
learning and external provider routes are inapplicable to this transport experiment.

Initial dogfood start was blocked by sandbox metadata writes; the authorized retry ran and retained
NOT_PRODUCED reasons: citation-plan-not-provided, missing-intent-scope, cem-status not-ready, and
outcome-input-not-provided. Full original output is `/tmp/lsp-integration-dogfood-start.log`.
The parent coordinator owns final CEM/OCM binding, independent integration review and publication.
No local completion or promotion is claimed by this candidate. Source component review receipts
are source-content evidence, not editor or product qualification. Actual-client runs, acceptance,
performance, usefulness, final dogfood seal and ticket completion remain outside this handoff.
Rollback stops the companion or omits the semantic flags; Core defaults remain unchanged.
