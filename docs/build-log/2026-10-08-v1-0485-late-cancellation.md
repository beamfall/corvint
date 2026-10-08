# V1-0485: late context cancellation can no longer return READY

## Intent

V1-0485 (P1 bug): a real `NewTaskReview` / `Registry.Call(ToolContext)` request cancelled while
`taskContextCompiler.lexicalRows` ran returned READY with a nil error. The fix must return
cancellation instead, retire the work the request started, keep uncancelled receipts and
abstentions byte-identical, and keep the default read-only behaviour.

## Findings

- Reproduced on `f33ea8ef` (origin/main) with a deterministic in-package seam: a stage hook that
  cancels the request context inside `lexicalRows` after every Git read had finished. `TaskContext`
  returned `state=READY, err=nil`. A context cancelled *before* the call also returned READY for
  the retrieval shape (no subject), because nothing but the history and recency Git readers ever
  observed the context.
- The bridge had the same gap one level up: `Registry.Call` checked the context only on entry. With
  the production `compileContext` wrapped to cancel after it returned its packet,
  `Call(ToolContext)` returned READY on the unfixed bridge.
- The recency reader was not joined when compile returned early on a history error (it exits on its
  own once the cancelled Git command ends, but nothing waited for it).

## Change

- `TCP-V0-064` (proposed): `taskContext` keeps the request context on the compiler and checks it at
  fixed compile boundaries (pair, mentioned, subject slots, after the history join, each lexical
  term, after the lexical walk inside `lexicalRows`, after the fill) and once more after the packet,
  spans and snapshot refusal. An ended context returns the existing `contextError` refusal and no
  packet. A deferred `join` waits for the history and recency readers on every path.
- `MCPV0-034` (proposed): `Registry.Call` re-checks the context after the operation returns and
  before the root-identity check, returning `cancelled` for every tool.

## Verification

- `TestTaskContextCancellationNeverReturnsAPacket` fails on the unfixed compile (`lexical` stage:
  READY) and `TestTaskContextPreCancelledRequestReturnsCancellation` fails on it (retrieval shape:
  packet returned); both pass after the change. Every stage, recency off and on, returns the
  cancellation refusal; measured cancel-to-return inside `TaskContext` was 3 µs to 1.8 ms on the
  fixture, and no history or recency goroutine survives the return. The uncancelled packet after
  the cancellations is byte-identical to the one before.
- `TestContextCancellationNeverReturnsReady` (bridge, real production `compileContext` on a cold
  repository) fails on the unfixed bridge for the late case and passes after; early and cold-loader
  cases return `cancelled`; the healthy receipt afterwards is byte-identical and the root digest is
  unchanged.
- `TestContextTimedCancellationRetiresWork` cancels real calls at 0 to 7/8 of a measured
  uncancelled call (about 45 to 52 ms): every cancelled call retired in at most 3.8 ms across three
  runs, no context-index goroutine survived, and the following healthy call succeeded.
- Focused packages and doc gates: see the lane report.

## Limits and NOT_RUN

- The retirement bound is per stage, not constant: a slot generator between two boundaries
  (symbol, reference, importer, sibling, graph placement) runs to completion. The worst case on a
  large repository is not measured (`NOT_RUN`).
- A cancellation that arrives after `Registry.Call`'s final check is a completed call; `MCPV0-011`
  decides at the server whether its frame is written.
- `make gate`, full `go test ./...`, the MCP compiled-process conformance suites and live LSP
  qualification: `NOT_RUN` (lane policy: focused tests only).

## Rollback

Revert the commit. Delete `stopped`, `join`, the compiler `ctx`/`cancelled` fields and the boundary
checks in `internal/contextindex/taskcontext.go`, the post-operation check in `Registry.Call`, both
new test files and the `TCP-V0-064` / `MCPV0-034` spec text. No state persists.
