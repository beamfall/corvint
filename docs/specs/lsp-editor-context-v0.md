# LSP Editor Context V0

- Owner: Russell Lewis
- Date: 2026-09-29
- Intent status: proposed
- Delivery status: experimental
- Authoritative inputs: owner-requested Go/gopls agents and VS Code/Neovim direction; accepted LSP quality platform technical contract; task-context packet and MCP bridge contracts.

## Agent digest
- Claim: Explicit editor context requests preserve Core task-review evidence separately from a current open-overlay observation.
- Status: proposed/experimental; no supported tuple or performance qualification.
- Exists: fixed native context worker, namespaced request, bounded protocol and point-in-time freshness observations.
- Blocked on: independent review, real-client qualification and accepted promotion floors.
- Read next: Requirements; Wire and observation limits; Traceability and rollback.

## User job and baseline

In the optional operator-configured Go semantic session, an editor asks for governing requirements,
related tests and other existing task-context evidence for an open document. The baseline is calling
the task-review MCP bridge separately. This projection uses exactly that operation; unsaved text is
observed separately and never changes its Git facts. No wider Corvint tool dispatch is provided.

## Requirements

- `LEC-V0-001`: Only explicit `corvint/context` MUST be added. Initialization MUST expose `experimental.corvintContext` with exactly `method: "corvint/context"` and `schema: "corvint-editor-context/0"` after the native lifecycle proof. Standard capability advertisement MUST remain unchanged.
- `LEC-V0-002`: Params MUST be the closed object `{textDocument:{uri},task,limit}`; recursively reject unknown, duplicate, null or missing members. URI MUST be canonical, within the operator root, open and at most 4096 bytes; task MUST be nonblank and at most 32000 bytes; limit MUST be an integer 1..50; request ID MUST be at most 4096 bytes. No client command, executable, root or tool selector is admitted.
- `LEC-V0-003`: Success MUST contain schema `corvint-editor-context/0`, intact validated `Result.Object()` as `core`, `overlayObservation`, and fixed `inclusionReason: "requested-open-document-subject"`. Valid binding-free ABSTAINED results MUST remain successful. Serialized LSP bodies MUST be at most 1 MiB; overflow MUST refuse rather than truncate evidence.
- `LEC-V0-004`: Overlay observation MUST carry a fresh crypto-random 128-bit session ID as 32 lowercase hex characters, decimal-string capture ID in 1..18446744073709551615 without leading zeroes, URI, document version, `digestAlgorithm: "sha256"` and lowercase 64-hex digest. Capture IDs MUST monotonically increase, differ after identical reopen, and refuse overflow. They confer no Git identity or authority.
- `LEC-V0-005`: Admission and post-Core observations MUST compare root directory identity, commit, tree, dirty-path digest/count/state and symbolic HEAD. Each repository probe MUST be bracketed by matching symbolic HEAD reads and root identities. Any returned Core binding MUST agree with the observations. Exact overlay capture and request context MUST be checked before reply. Unknown observations MUST fail closed; unseen ABA and changes after the final observation are outside the claim.
- `LEC-V0-006`: The existing single pending request MUST be reused. Cancellation, edits, close, EOF and backend death MUST cancel and join owned work. Parent cancellation/deadline MUST suppress even a successful late Core result. A healthy subsequent request MUST remain possible after request cancellation.
- `LEC-V0-007`: Core MUST run only in a same-executable fixed one-shot native worker using `NewTaskReview` and `ToolContext`, never the LSP bridge or another gopls. Its executable's SHA-256 identity MUST match the session-start capture before and after execution. Input MUST be bounded at 256 KiB (including JSON escaping), stdout at 1 MiB and stderr at 4096 bytes; no raw child error may be published.
- `LEC-V0-008`: Worker ownership MUST be enabled once at startup before Git resolution, Core or launch-capable goroutines, after checking Darwin/Linux group-leader identity. Both Core Git launchers MUST inherit that group, cancel only their own unreaped leader and skip private-group reaping/cleanup. Default Core behavior MUST remain unchanged. Unsupported worker platforms MUST refuse before repository work.
- `LEC-V0-009`: Parent request deadline MUST be 20 seconds; runner operation timeout MUST be 20 seconds and configured shutdown budget 2 seconds. Startup, observer/drain overhead and OS scheduling prevent a strict 22-second aggregate guarantee. Success MUST require runner success, joined Wait, drained pipes, owned-group cleanup, its normal known qualification and an absent observed descendant inventory without failures. Unknown cleanup MUST refuse. `RequireDescendantCleanup` MUST remain false; observation is not hostile escape containment.
- `LEC-V0-010`: Errors MUST be fixed/redacted: invalid params -32602; busy -32000; cancel/deadline -32800 with `CANCELLED`/`DEADLINE`; stale -32801 with `CONTENT_CHANGED`/`REPOSITORY_CHANGED`; Core/bridge/overflow failures -32001 with `CORE_UNAVAILABLE`. Error data MUST contain only the fixed reason. Root/task/branch/child-error bytes MUST NOT appear in errors.

## Wire and observation limits

Success shape:

```json
{"schema":"corvint-editor-context/0","core":{},"overlayObservation":{"sessionID":"0123456789abcdef0123456789abcdef","captureID":"1","uri":"file:///repo/a.go","version":1,"digestAlgorithm":"sha256","digest":"<64 lowercase hex>"},"inclusionReason":"requested-open-document-subject"}
```

The illustrative `core` placeholder stands for the complete existing validated bridge object,
including its uncertainty, abstention, authority and receipt. Core CLEAN/MIXED labels are compared
using the existing bridge interpretation of the repository probe's clean/dirty state. Comparison
does not rewrite the Core object. Separate-call parity is tested only on frozen fixture inputs.

Normal EOF after the worker's single JSON stdin message is expected. External editor EOF cancels
the parent runner. No response is promised after editor streams close. Edits cancel and join the
pending request before changing the capture; a caller can retry. The final observation is a sampled
point before publication, not an atomic filesystem snapshot or continuous branch monitor.

The trusted local operator selects the root, binary and installed Git. The worker policy is an
internal startup API, not an environment/config toggle. The reachable Core launchers are
contextindex Git, gokernel Git and gitstatus's macOS xcrun resolver (which inherits the group).
No production loader path in this operation starts Go, gopls, a shell, tests or a learning operation.
The executable digest checks detect endpoint drift; they are not exec-fd isolation against hostile
replacement between checks. PID observation cannot contain deliberately detaching hostile code.

## Traceability and rollback

| Requirements | Implementation | Evidence |
|---|---|---|
| LEC-V0-001–004 | internal/lspstdio/context.go; semantic.go | TestContextClosedParams; TestContextResponseErrorsAndBounds; TestCapturedIdentityReopen |
| LEC-V0-005 | internal/lspstdio/context.go | TestContextFreshnessAfterCore; TestNativeContextWorker |
| LEC-V0-006–009 | cmd/corvint-lsp; internal/lspstdio/context_worker.go; internal/gitstatus/worker.go; Core process helpers | TestNativeContextWorker; TestWorkerIdentityDrift; TestWorkerGitProcessGroup; retained real CPU-phase and editor EOF transcripts |
| LEC-V0-010 | internal/lspstdio/context.go | TestContextResponseErrorsAndBounds; TestContextClosedParams |

The build-log entry retains native production-loader, CPU-phase, group identity, EOF, retry and
selected test evidence with their limitations. This is experimental development evidence, not
world-class quality, general read-only proof, accepted profile qualification, held-out outcomes or
client interoperability. The underlying in-process Core cancellation issue remains open (V1-0485).
Rollback disables the companion or removes this experimental method/marker and internal worker
policy; existing Core and the definition experiment remain available, with no persistent migration.
