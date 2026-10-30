# LSP Wrapped Backends V0

- Owner: Russell Lewis
- Date: 2026-10-25
- Intent status: proposed
- Delivery status: experimental
- Authoritative inputs: owner-filed GitHub beamfall/corvint#1788 (ticket V1-2687); LSP editor definition and editor context contracts; gopls client session contract; AGENTS.md product invariants 2, 3 and 7.

## Agent digest
- Claim: corvint-lsp can wrap operator-named TS/JS and Ruby language servers routed by file suffix, labelling their results as server observations, not authority.
- Status: proposed/experimental; fake-server tests only, with no real typescript-language-server or ruby-lsp qualification.
- Exists: `--tsserver`, `--ruby-lsp` and generic `--server SUFFIXES=ABSOLUTE_ARGV` flags; a closed suffix table; one process per backend; named refusals; and `corvint/context` for routed files under the workspace-drift guard.
- Blocked on: an approved real-server qualification run, server-to-client request support and independent client interoperability.
- Read next: Requirements; Failure modes; Traceability and rollback.

## User job and baseline

An operator editing a mixed Go, TypeScript and Ruby repository wants the same companion session to
answer definition probes and `corvint/context` requests for every open file. Before this change
`corvint-lsp` wrapped only gopls, so TS/JS and Ruby documents had no semantic backend and could not
be named in `corvint/context`. The baseline is running each language server in a separate editor
client, where nothing separates the server's answers from Corvint's own evidence.

This spec extends `docs/specs/lsp-editor-definition-v0.md` (LED-V0) and
`docs/specs/lsp-editor-context-v0.md` (LEC-V0). The gopls lifecycle and failure behaviour is
unchanged. gopls definition rows gain the same additive `corvintObservation` label, because they
are also a wrapped server's observations.

## Requirements

- `LWB-V0-001`: The companion MUST wrap a non-Go server only when the operator names it on the
  command line by absolute executable path: `--tsserver ABSOLUTE_BINARY` (run as `BINARY --stdio`),
  `--ruby-lsp ABSOLUTE_BINARY` (run as `BINARY`), or the repeatable generic
  `--server SUFFIXES=ABSOLUTE_ARGV`, where SUFFIXES is a comma-separated list of one family's
  suffixes and ABSOLUTE_ARGV is a whitespace-separated argv whose first word is absolute. Every
  wrapped flag MUST require `--experimental` and `--root`. Corvint MUST NOT search PATH, download,
  install or choose a server on its own.
- `LWB-V0-002`: Routing MUST use the exact, case-sensitive extension of a `file:` URI against the
  closed table: `.ts .mts .cts` (typescript), `.tsx` (typescriptreact), `.js .mjs .cjs`
  (javascript), `.jsx` (javascriptreact), all in the typescript family; and `.rb .rake` (ruby).
  `.go` keeps routing to gopls when `--gopls` is set. A `didOpen` MUST carry the routed languageId.
  Synchronization notifications for a document that no configured backend routes MUST be ignored,
  not forwarded and not treated as a session error.
- `LWB-V0-003`: The companion MUST run at most one process per backend family (gopls, typescript,
  ruby), and a suffix MUST NOT be claimed by two backends. Each process MUST be started once at
  `initialize`, own its process group, and be retired at shutdown or session end.
- `LWB-V0-004`: A wrapped server MUST use a generic negotiation profile: the server-chosen
  position encoding, full or incremental text sync accepted only as full-text sync from Corvint,
  no `initializationOptions`, no gopls cache, and a frozen environment of `LANG=C`, `LC_ALL=C` and
  only `PATH`, `HOME` and `TMPDIR` from the companion. The session MUST record the server's
  reported name and version and the executable's SHA-256 as observations.
- `LWB-V0-005`: Every definition location returned by a wrapped server (gopls included) MUST carry
  `corvintObservation` with `observer` set to the backend name, `class`
  `wrapped-server-observation` and `authority` `NONE`. The `initialize` result MUST list each
  backend in `capabilities.experimental.corvintBackends` with its name, suffixes, server name,
  server version, executable SHA-256, observation class and `authority: NONE`. Wrapped results
  MUST NOT enter Core evidence, ranking, authority or learning. Core's own non-Go evidence in
  `corvint/context` MUST keep its existing labels (for example `authority: syntax`) unchanged.
- `LWB-V0-006`: A configured server executable that does not exist MUST be refused with the named
  reason `SERVER_BINARY_MISSING`: at startup as `corvint-lsp: refused: SERVER_BINARY_MISSING` with
  exit status 2, and at `initialize` (if it disappears after startup) as JSON-RPC error `-32001`
  with `data.reason` `SERVER_BINARY_MISSING`. Any other start or negotiation failure at
  `initialize` MUST reply `-32001` with `data.reason` `BACKEND_UNAVAILABLE`. Every backend already
  started MUST be retired in both cases.
- `LWB-V0-007`: A flag that names a suffix outside the closed table MUST be refused at startup as
  `corvint-lsp: refused: UNSUPPORTED_SUFFIX` with exit status 2. A definition or `corvint/context`
  request for a document whose suffix no configured backend routes MUST reply `-32602` with
  `data.reason` `UNSUPPORTED_SUFFIX`.
- `LWB-V0-008`: When a wrapped server exits, or a forward or request transport to it fails, the
  backend MUST be retired and latch as crashed: a pending definition and every later definition on its suffixes MUST reply `-32001`
  with `data.reason` `SERVER_CRASHED`, its notifications MUST be dropped, and the session and every
  other backend MUST continue. Shutdown MUST still fail when a crashed backend's process-group
cleanup is unproven. A gopls failure keeps the LED-V0 behaviour of ending the session.
- `LWB-V0-009`: `corvint/context` MUST accept an open, routed TS/JS or Ruby document and return
  Core task-review evidence for the session's bound commit revision, with `--workspace-drift-guard`
  set. Its Core result MUST be byte-equal to the direct task-review bridge call at that revision,
  MUST NOT include wrapped-server output, and the request MUST NOT modify the worktree. Under the
  guard, the workspace and Git state MUST be checked against the session baseline before dispatch
  and before reply. Observed drift, including a moved HEAD, MUST reply `-32801` with
  `data.reason` `REPOSITORY_CHANGED` and invalidate the captures.
- `LWB-V0-010`: `corvint-lsp --help` and the usage line MUST list `--tsserver`, `--ruby-lsp` and
  `--server` with their argument forms. Every refusal MUST print a fixed reason token, never a
  backend path, argument, or server message.

## Non-goals

- Real typescript-language-server, tsserver or ruby-lsp qualification, version pinning or tuple
  support claims. Downloads are not approved for this slice.
- Advertising standard `definitionProvider` or any capability beyond the LED-V0 experimental probe.
- Hover, references, completion, diagnostics, rename or any other LSP method for wrapped servers.
- Server-to-client requests (`workspace/configuration`, `client/registerCapability` and similar).
- Additional families, case-insensitive routing or content sniffing.
- Changing Core evidence, ranking or authority for TS/JS or Ruby.

## Failure modes

| Failure | Behaviour | Reason token |
|---|---|---|
| Flag names an unknown suffix | startup exit 2 | `UNSUPPORTED_SUFFIX` |
| Request on an unrouted suffix | `-32602` reply, session continues | `UNSUPPORTED_SUFFIX` |
| Executable missing at startup | startup exit 2 | `SERVER_BINARY_MISSING` |
| Executable missing at `initialize` | `-32001` reply; started backends retired | `SERVER_BINARY_MISSING` |
| Negotiation refused or start failed | `-32001` reply; started backends retired | `BACKEND_UNAVAILABLE` |
| Wrapped server exits or a forward fails | backend latched; `-32001` on its requests | `SERVER_CRASHED` |
| Workspace or HEAD drift before a guarded context reply | `-32801` reply; captures invalidated | `REPOSITORY_CHANGED` |
| Relative path, two backends per family, duplicate suffix, malformed `--server` | usage, exit 2 | none (configuration) |

## Wire limits and known limitations

- A wrapped server that sends a server-to-client request is retired and latches `SERVER_CRASHED`.
- The existing ten-minute session bound on a backend process also latches `SERVER_CRASHED`.
- ruby-lsp writes `.ruby-lsp/` under the root, which a whole-root drift guard observes as drift.
- `--server` argv words cannot contain whitespace; only `PATH`, `HOME` and `TMPDIR` pass through.
- Definition targets must be synchronized open overlays, as in LED-V0; other targets are
  unavailable rather than invented.

## Traceability and rollback

| Requirements | Implementation | Evidence |
|---|---|---|
| LWB-V0-001, 002, 003, 005 | internal/lspstdio/backends.go; semantic.go; cmd/corvint-lsp | TestWrappedSuffixRouting; TestBackendConfiguration |
| LWB-V0-004 | internal/goplsclient/client.go | TestWrappedNegotiationAndLanguage; TestWrappedNegotiationRefusals |
| LWB-V0-006 | internal/goplsclient/client.go; internal/lspstdio/semantic.go; cmd/corvint-lsp | TestMissingExecutable; TestWrappedMissingBinaryAtInitialize; TestBackendRefusals |
| LWB-V0-007 | internal/lspstdio/backends.go; semantic.go; cmd/corvint-lsp | TestWrappedUnsupportedSuffix; TestBackendRefusals |
| LWB-V0-008 | internal/lspstdio/semantic.go | TestWrappedServerCrash |
| LWB-V0-009 | internal/lspstdio/semantic.go; context worker | TestWrappedContextAtBoundRevision |
| LWB-V0-010 | cmd/corvint-lsp/main.go | TestBackendRefusals |

All evidence uses in-repo fake LSP servers (the test binary re-executed through a shell wrapper).
None of it is real-server, editor-client or performance qualification.

Rollback: omit `--tsserver`, `--ruby-lsp` and `--server`, and the companion runs exactly the gopls
session of LED-V0 and LEC-V0. Reverting the change removes the flags, the suffix table and the
`corvintBackends` capability. There is no persistent state or migration.
