# LSP Stdio Experiment V0

- Owner: Russell Lewis
- Date: 2026-09-29
- Intent status: proposed
- Delivery status: experimental
- Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`, official LSP 3.18 lifecycle and base protocol; owner requested optional editor companion under V1-0481.

## Agent digest
- Claim: An explicitly experimental optional native-Go stdio companion exercises bounded framing and lifecycle without semantic capabilities.
- Status: proposed/experimental
- Exists: standalone command and deterministic transcript tests.
- Blocked on: independent review, real-client qualification, snapshot/provider integration and accepted technical contract.
- Read next: Requirements; Bounds and non-goals; Traceability.

The capability-free mode described here remains the no-backend baseline. The separately opted-in
semantic mode is governed by [LSP Editor Definition V0](lsp-editor-definition-v0.md); its worker and
subprocess behavior are not covered by this capability-free contract.

## User job and baseline

An editor integrator can launch `corvint-lsp --experimental`, negotiate a session and shut it down safely. The simpler baseline is the editor's own language server. This experiment does not improve navigation or replace that server. The broader LQP proposal remains unaccepted; this narrow contract records prototype choices, not owner acceptance.

## Requirements

- `LSE-V0-001`: Startup MUST require explicit experimental opt-in and keep stdout exclusively framed JSON-RPC 2.0.
- `LSE-V0-002`: Framing MUST bound headers and body bytes, reject ambiguous lengths and truncated frames, and return JSON-RPC errors for malformed JSON and invalid requests.
- `LSE-V0-003`: The session MUST negotiate initialize/initialized/shutdown/exit ordering, reject requests before initialization and after shutdown, and leave unsupported methods unadvertised.
- `LSE-V0-004`: Initialization MUST retain declared local file roots without reading their contents and select an offered UTF encoding, defaulting to UTF-16. No semantic or text synchronization capability may be advertised.
- `LSE-V0-005`: Cancellation of already-completed or unknown requests MUST be harmless. EOF, context cancellation, exit and the session bound MUST close owned input/output. The prototype MUST spawn no subprocesses and execute no workspace commands or edits.

## Bounds and non-goals

Headers are at most 8 KiB, bodies at most 1 MiB, and a session at most 100,000 frames and 30 minutes. Roots are at most 64 unique absolute local file URIs. Invalid framing terminates the stream because resynchronization is unsafe. Invalid JSON receives ParseError; unsupported requests receive MethodNotFound. Responses from the client and unknown notifications are ignored: the prototype sends no requests. Diagnostics are fixed text with no input/path echo.

Serve owns both closers, including on interruption; callers must supply closers whose Close unblocks outstanding I/O. The command requires owned pipe/socket stdin/stdout, rejecting terminals and regular files before changing descriptor flags. This prevents mutating an inherited shell terminal file description. All supported requests complete synchronously with no asynchronous worker, so cancellation cannot abort an already-completed request. Future asynchronous semantic work needs a separate request registry and tests. There are no upstream children or private caches to retire. The command currently admits Unix pollable descriptors only; other platforms fail closed. Client processId monitoring, live overlays, dynamic workspace changes, semantic methods, writes, downloads, extensions, networking and promotion are out of scope. Parent-death liveness is bounded by EOF or the session deadline, not a qualified client lifecycle promise.

## Acceptance, rollback and drift

Run the focused Go race tests for internal/lspstdio and cmd/corvint-lsp. Tests prove transcript behavior, not real VS Code/Neovim support. Client qualification, Unicode document positions, provider/snapshot joins, latency and outcomes remain NOT_RUN. Rollback stops and disables this separate companion; Core CLI/MCP defaults are untouched. Any new method or integration invalidates this capability-free transcript baseline and requires new qualification.

## Traceability

| Requirements | Implementation | Deterministic evidence |
|---|---|---|
| LSE-V0-001 | cmd/corvint-lsp | TestOptIn |
| LSE-V0-002 | internal/lspstdio | TestFraming, TestInvalidMessages |
| LSE-V0-003, LSE-V0-004 | internal/lspstdio | TestLifecycle, TestInitializeValidation |
| LSE-V0-005 | internal/lspstdio, cmd/corvint-lsp | TestCancellationAndEOF, TestInterrupt |

## Protocol sources

The official [LSP 3.18 source](https://github.com/microsoft/language-server-protocol/blob/gh-pages/_specifications/lsp/3.18/specification.md) defines framing and JSON-RPC bounds. Its [initialize clause](https://github.com/microsoft/language-server-protocol/blob/gh-pages/_specifications/lsp/3.18/general/initialize.md) defines encoding negotiation and mandatory UTF-16 fallback. Context7 returned applicable 3.18 shutdown/exit/initialized sources but also a 3.19 processId excerpt; that mixed output is not treated as complete version verification. Direct 3.18 source was read for framing and negotiation. No client conformance claim follows.
