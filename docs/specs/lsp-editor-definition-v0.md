# LSP Editor Definition V0

- Owner: Russell Lewis
- Date: 2026-09-29
- Intent status: proposed
- Delivery status: experimental
- Authoritative inputs: owner-requested Go/gopls agent and optional editor direction; AGENTS.md; LSP snapshot overlay, gopls client session and stdio experiment contracts.

## Agent digest
- Claim: Explicitly configured editor sessions can request definitions between synchronized open Go overlays through a local gopls process.
- Status: proposed/experimental; no semantic promotion or client qualification claim.
- Exists: separate opt-in command mode, exact overlay joins, bounded request worker and Unicode conversion.
- Blocked on: independent real-client qualification; external targets remain unavailable.
- Read next: Requirements; Bounds and failures; Traceability.

## User job and baseline

The operator supplies `corvint-lsp --experimental --gopls /absolute/gopls --root /absolute/root`.
The editor declares that same single canonical root. This experimental path joins the existing
components without changing Core, installing tools, discovering binaries or writing editor config.
The baseline remains the editor's directly configured gopls. No benefit over that baseline is claimed. An operator may issue direct `textDocument/definition` development probes after initialization; standard editor navigation remains unavailable because `definitionProvider` is unadvertised until client qualification.

## Requirements

- `LED-V0-001`: The companion MUST enable semantic mode only with explicit operator executable and root arguments, and expose an explicit experimental definition-probe marker and full-text sync only after successful backend initialization. It MUST NOT advertise standard `definitionProvider` before an exact editor tuple qualifies. Multiple or mismatched roots MUST be rejected. An explicit operator workspace-drift guard MUST positively identify workspace-only observation and externalInputsPinned=false; legacy mode remains experimental.
- `LED-V0-002`: Open/change/close MUST retain exact captured overlay identity across synchronization. Currentness MUST check the actual capture; version or digest alone MUST NOT identify reopened documents. Unsupported incremental updates and invalid synchronization MUST terminate the session without claiming current evidence.
- `LED-V0-003`: Definition MUST use one bounded pending request, service cancellation while work is pending, cancel and join before document mutation, and recheck source and target snapshots before replying. Only synchronized open overlay targets are admitted; disk targets MUST return explicit unavailability. The opt-in guard MUST retain a session baseline of whole-root bytes/Git identity, compare before dispatch and after completion with one shared 200ms observation budget, and latch fixed stale after drift/failed observation. Save invalidation MUST cancel/join exactly once and emit one stale response, while ordinary overlay cancellation remains healthy.
- `LED-V0-004`: Editor UTF-8, UTF-16 and UTF-32 positions MUST convert through validated Unicode boundaries to gopls UTF-16 and back. Invalid, missing or out-of-range coordinates MUST be rejected rather than rounded.
- `LED-V0-005`: EOF, interrupt, cancellation and session expiry MUST close streams, cancel backend execution and join owned request/reader/process cleanup. Public errors MUST remain fixed text without backend paths or messages.

## Bounds and failures

The transport framing ceilings remain LSE. The session admits one root, 32 documents, 256 KiB per
document, 8 MiB total text, one 20-second definition request, and the gopls client's ten-minute
session bound. Full-text changes only; versions are nonnegative signed 32-bit integers. A second
pending request returns a fixed busy error. A change cancels the pending definition; the editor
may retry against the new version. EOF cancels outstanding startup and requests immediately; no
reply is promised after stream closure. Backend lifetime completion independently cancels owned I/O,
including a blocked editor write. Intentional shutdown disarms and joins that observer before
requesting backend shutdown, preserving the normal shutdown reply. Unknown notifications do not confer state or authority.

The configured executable is trusted local code. Root validation and offline environment are
admission bounds, not a filesystem/network sandbox. No dynamic roots, incremental edits, save
or rename tracking, references, hover, diagnostics, completion, edits, cross-root targets, disk
fallback, Git evidence, immutable semantic cache, client installation or default routing is provided.
Standard definition locations carry navigation only; they are not authenticated evidence packets.

## Traceability

| Requirements | Implementation | Evidence |
|---|---|---|
| LED-V0-001 | cmd/corvint-lsp, internal/lspstdio/semantic.go | TestSemanticOverlayDefinition; TestOptIn |
| LED-V0-002 | internal/lspstdio/semantic.go | TestCapturedIdentityReopen; TestSemanticCancelAndChange |
| LED-V0-003 | internal/lspstdio/semantic.go | TestSemanticOverlayDefinition; TestSemanticCancelAndChange |
| LED-V0-004 | internal/lspstdio/semantic.go | TestPositionConversions; TestSemanticOverlayDefinition |
| LED-V0-005 | internal/lspstdio/semantic.go | TestSemanticEOFDuringInitialize; TestSemanticCancelAndChange; TestSemanticBackendDeathUnblocksReply; TestSemanticShutdownReplyBeforeExit; TestInterrupt |

The live subtest requires explicit CORVINT_TEST_GOPLS and otherwise skips visibly. Tests assert that the standard definition capability is absent while the experimental probe marker is present; direct requests exercise the development path only. Fake sessions
prove joining behavior, not editor/provider interoperability. Client qualification, performance,
agent usefulness and promotion remain separate. Rollback removes the operator flags or stops the
companion; the original capability-free mode and Core remain available.

## Optional partial workspace drift guard

`--workspace-drift-guard` requires explicit experimental semantic admission. Successful initialize includes experimental.corvintWorkspaceDriftGuard `{profile:"whole-root-observation/0",scope:"workspace-only",externalInputsPinned:false}` and save includeText:false; standard definitionProvider remains absent. The whole canonical root includes ignored/untracked files, dot directories, vendor/embed data, contained modules/local replacements and go.work. Only exact Git administration is opaque; branch/commit/tree/root identity remains observed. Symlinks/nested repositories/special or unreadable/unstable entries refuse. Limits are100000 entries,256MiB total,16MiB/file,depth64,5s admission and shared 200ms pre/post observation. Benign writes/chmod can conservatively invalidate. Local filesystem syscalls retain OS overhead/hung-storage limitations; no atomic snapshot, unseen ABA or post-observation guarantee exists.

Observed disk drift or same-byte negotiated save latches stale until a new session, canceling/joining pending work and invalidating captures. A consumed result is never drained again. Unsaved full replacements with unchanged disk preserve healthy cancellation/retry. No save/edit/rename operation writes source. Legacy static-root behavior remains unqualified; this opt-in partial guard does not pin external SDK/cache/module inputs or satisfy complete provider closure.

The experimental guard admission refuses committed-tree and current-index gitlinks through bounded mode inventories (8MiB, 100000 records), including empty checkout directories. Later index-only staging is outside the provider-closure claim. Directory scans use context-checked chunks of at most 128 entries with a global allowance plus one overflow witness and directory identity brackets; they do not rely on WalkDir preallocation. The shared 200ms observation budget is unchanged.
