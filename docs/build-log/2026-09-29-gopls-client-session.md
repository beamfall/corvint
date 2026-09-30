# Experimental gopls client session

Date: 2026-09-29. Work item: V1-0479 (remains open). Base: `75f06f9020a38af8a775a35479b4b16cc69b8e3c` from `origin/main`. This is an isolated candidate for independent review, not a qualification or completion claim.

## Decision and boundary

Add an unused `internal/goplsclient` package for a caller-owned optional session. Reuse `procgroup` instead of refactoring the existing terminating CLI provider. The first profile handles only UTF-16 definition requests between explicitly synchronized caller overlays. Per-document synchronization retains version, opaque identity and digest rather than a second authority store. Source/target URI and range admission plus caller freshness checks prevent an open buffer from being labelled committed. References and all provider diagnostics/edits remain unavailable.

The executable is explicitly configured, locally hashed and never downloaded. Private gopls/Go caches are removed after lifecycle join. Offline Go environment settings are not a network sandbox. The existing runner explicitly refuses `RequireDescendantCleanup`; its supported owned-process-group retirement and bounded descendant observation are used, with escaped-child containment left unclaimed.

## Evidence and retained failures

Pre-change `corvint query` returned one governing result, two ranked omissions and eight withheld test-path candidates; `impact internal/lspprovider/session.go` was retained. `affected --base` was captured before tests. Its generic exhaustive-gate advice does not supersede the owner's explicit focused-check policy; `make gate` is `NOT_RUN`.

The first dogfood-change attempt was blocked by sandbox Git-metadata writes; the authorized escalated attempt ran and reported `NOT_PRODUCED`/not-complete at the unchanged base (CEM `git-diff-failed`, missing intent scope/outcome inputs). The local workflow was enrolled against the existing external-provider intent before code changes. Final CEM, scoped OCM, keyed verification, reviewer report and seal remain pending at this candidate boundary; new GCS intent did not exist at the base.

Initial fake session tests failed because the runner deliberately refuses the stronger descendant-containment setting. Using its supported cleanup mode resolved the failure. Initial live initialization refused gopls JSON build-info; the repair extracts only the semver token and never emits arbitrary build metadata. An initial spec-index check rejected a README claim mismatch; the exact claim is now aligned. These are retained development failures, not discarded measurements.

Final scoped observations:

- `GOPLSCLIENT_LIVE_EXECUTABLE=/tmp/corvint-lsp-integration/bin/gopls GOTOOLCHAIN=local go test -race -count=1 -timeout 30m ./internal/goplsclient ./internal/specindex` passed (client 5.184 s; index 1.468 s; cache under `/tmp`).
- Local gopls v0.23.0, darwin/arm64, Go 1.27.1; configured executable SHA256 `39431a5b273a5ac124a98521d1ec3e38af4bc5eec783e24a23344145eef24836`. Two definitions followed unsaved open/change line shifts, while disk bytes remained unchanged. This is one synthetic dependency-free module witness, not a latency distribution.
- Hostile framing, UTF-16 surrogate boundaries, secret-bearing provider error, external URI, missing/invalid range, version/freshness rejection, request cancellation followed by healthy reuse, leader crash and session cancellation with descendant retirement passed.
- `go vet ./internal/goplsclient` passed. Spec requirements, requirement definitions, traceability tests, decision numbers and line-citation checks passed; the traceability checker retains three existing planned rows.
- Context7 resolved official LSP/gopls documentation; encoding/full-sync detail returned no-match, then the original LSP 3.18 initialize/didChange sources confirmed defaults and whole-document changes. The initial sandbox Context7 network attempt was retried with network authorization.

## Pending integration and rollback

No source here imports the new package, and default CLI/MCP behavior is unchanged. Independent source review, post-commit evidence binding and integration remain pending. V1-0167, V1-0168, V1-0478 and V1-0479 stay open. No editor qualification, held-out corpus, upstream-versus-combined outcome, memory/CPU ceiling, billed-token/cache telemetry or promotion was observed. UTF-8 clients need explicit captured-text conversion before any future integration.

Rollback cancels the owning context, awaits cleanup, and removes this package/spec/index registration. It needs no persistent-state migration. Private candidate receipts and the workflow resume key are held in `/tmp/gopls-session-checkpoint.json`; the parent coordinator owns subsequent review/binding.
