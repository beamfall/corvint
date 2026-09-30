# Gopls client session V0

Owner: Russell Lewis
Date: 2026-09-29
Requirement prefix: `GCS-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `AGENTS.md`, `docs/specs/external-evidence-provider-v0.md`; owner direction for V1-0479, Go/gopls agent enrichment and optional editor companion. This bounded prototype implements part of the proposed LQP-V0-007..009 adapter profile; it does not qualify or promote any exact platform profile.

## Agent digest
- Claim: An optional local gopls session observes definitions only between caller-owned synchronized overlays with explicit lifecycle and bounds.
- Status: proposed/experimental
- Exists: internal package, fake-server hostile tests, dependency-free Go/gopls live overlay witness.
- Blocked on: shared snapshot/editor integration, independent review, real client qualification and outcome gates.
- Read next: Requirements, profile bounds, acceptance evidence and rollback.

## User job and non-goals

An optional editor companion needs to ask a locally configured gopls about an unsaved Go document without misrepresenting that text as committed evidence. The caller supplies immutable captures and a freshness predicate. Its opaque identity must change across close/reopen and root resets even when URI, version and bytes match; a content digest alone is not a valid identity. This package stores only a synchronization stamp (version, opaque identity and content digest); it owns no Git authority or snapshot store.

This is a library experiment, not a CLI command, editor server or supported profile. It never activates through the current CLI provider. References, diagnostics, edits, formatting, completion, multi-root routing and unopened/disk-only targets are unavailable. V1-0167, V1-0168, V1-0478 and platform/client promotion remain open.

## Requirements

- `GCS-V0-001`: Startup MUST require an absolute explicitly configured executable and canonical root, hash the configured executable bytes, and negotiate gopls identity, a bounded safe version token, UTF-16 and definition/full-text synchronization capability. Unknown negotiation MUST refuse the profile. No installation or download is authorized. Launch MUST freeze the actual environment once, bind its canonical JSON digest, use that same environment/settings snapshot for process and initialize, and disclose that external filesystem inputs remain unbound.
- `GCS-V0-002`: The session MUST own its process group through `procgroup`, join cleanup on shutdown, startup failure and cancellation, remove its private cache, and expose whether owned-group retirement was proven. A cancelled request MUST send cancellation and ignore its late response while allowing later healthy requests. This is bounded observation, not containment of every escaping descendant.
- `GCS-V0-003`: Open/change/close MUST use caller text and monotonically increasing versions. Definitions MUST require exact synchronization stamps for both source and returned targets, validate caller freshness before and after the request, and return captured overlay identities. Closed or committed-only targets MUST be rejected; the adapter MUST NOT read file URI contents from disk.
- `GCS-V0-004`: Framing, documents, messages, requests, output, locations and lifetime MUST be bounded. URI/range validation MUST reject root escapes, noncanonical URIs, missing coordinates and UTF-16 surrogate splits. Raw provider messages, errors, diagnostics and build metadata MUST NOT escape as diagnostics.
- `GCS-V0-005`: Experimental evidence MUST retain one explicit local dependency-free Go overlay witness showing definitions move with unsaved open/change text while disk remains unchanged. Unrun editor, held-out, performance and outcome qualifications MUST remain visible.

## Profile bounds and failure modes

The initial observed tuple is darwin/arm64, Go 1.27.1, gopls v0.23.0, UTF-16, one dependency-free module, definition only. The executable SHA256 is retained by the live test. This identifies configured bytes at startup; it does not attest a hostile operator replacing the executable between hashing and execution. Only a release-style `vMAJOR.MINOR.PATCH` version is admitted, either directly or extracted from gopls JSON build metadata.

Limits: 1 MiB message body, 4 KiB headers, 256 KiB UTF-8 document, 32 open documents, 256 locations per result, 20-second request deadline, 10-minute session deadline, 32 MiB aggregate inbound and outbound bodies, 64 KiB stderr and a two-second process shutdown allowance. Each request is serialized with document synchronization. Callback execution must be bounded and concurrent-safe; callbacks are trusted internal code and cannot be forcibly interrupted. Process CPU and memory limits are not enforced by this library. Idle sessions expire at the session deadline.

Server requests are unsupported and terminate the session; notifications, including diagnostics, are discarded. Provider errors become fixed local errors. A blocked protocol write retires the session rather than leaving an unjoined writer. Request cancellation sends the standard cancellation notification, but cannot prove a server honored it; session bounds still apply. If the provider ignores cancellation, later requests may hit their own deadline. A caller must await `Done` after cancelling the Start context to observe cleanup.

The environment disables module proxy/checksum traffic, toolchain downloads and telemetry, uses a private gopls/Go build cache and does not inherit GOFLAGS or GOWORK. This is no OS network sandbox and confers no authority to execute edits or commands returned by gopls. Local server/toolchain execution is explicitly operator-selected. External workspace-loading/redaction qualification remains separate.

## Acceptance evidence

| Requirement | Witness | Evidence meaning |
|---|---|---|
| GCS-V0-001 | `internal/goplsclient/client_test.go`, `TestNegotiation` | Reject incompatible encoding/method/sync/server/version; live omitted encoding uses the protocol-defined UTF-16 default. |
| GCS-V0-002 | `TestSessionSyncAndCancellation`; `internal/goplsclient/lifecycle_unix_test.go`, `TestCrashAndSessionCancellationRetireDescendant` | Cancelled request followed by a successful request; crash and session cancel retire spawned group descendant. |
| GCS-V0-003 | `TestSessionSyncAndCancellation`, `TestStaleReturn` | Unsent/duplicate versions and closed/stale captures rejected. |
| GCS-V0-004 | `TestHostileResponses`, `TestFramingAndUTF16` | Provider secret, external URI, invalid range/framing and surrogate split rejected. |
| GCS-V0-005 | `TestLiveOverlay` | Opt-in executable only; open and changed definitions follow overlay line shifts while on-disk bytes are unchanged. |

The live test is `NOT_RUN` when `GOPLSCLIENT_LIVE_EXECUTABLE` is absent; a ordinary test pass does not imply live qualification. Set that variable to an absolute preinstalled gopls path and run the selected test explicitly. This adapter accepts only UTF-16. A future UTF-8 editor integration must convert positions using the captured text, not reinterpret character offsets. Real VS Code/Neovim, shared snapshot integration, agent projection, held-out corpus, latency distributions, CPU/RSS, billed tokens and task outcomes are `NOT_RUN` or `NOT_OBSERVED`. No formal supported or promoted tuple exists.

## Protocol sources

Context7 verified the official LSP repository and gopls settings sources. A no-match response for encoding/full-sync detail was retained and resolved using the original LSP 3.18 sources: [initialize](https://github.com/microsoft/language-server-protocol/blob/gh-pages/_specifications/lsp/3.18/general/initialize.md), [didChange](https://github.com/microsoft/language-server-protocol/blob/gh-pages/_specifications/lsp/3.18/textDocument/didChange.md), [shutdown](https://github.com/microsoft/language-server-protocol/blob/gh-pages/_includes/messages/3.18/shutdown.md), and [gopls settings](https://go.dev/gopls/settings). UTF-16 is the specified default when the server omits its encoding. A content-change event with only text supplies the whole document.

## Rollback

Stop the owning context and await `Done`, then remove this unused package and its spec/index rows. No default provider, CLI, MCP, persistent authority or stored evidence migration changes. Do not enable editor or agent integration until its separate tests, review and required gates are retained.

## Launch observation limitation

LaunchEnvironmentSHA256 is SHA256 of JSON encoding of the exact ordered environment array; duplicate variable names refuse. Only the digest and workspace-only/external-unbound flags are retained in the local profile, not raw HOME/PATH values in public diagnostics. Initialized fixed settings derive from the same frozen array. This identity does not pin SDK, inherited caches or dependencies. Guarded caches must be outside the observed root; all modes preserve owned cache cleanup.
