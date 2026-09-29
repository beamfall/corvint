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
The baseline remains the editor's directly configured gopls. No benefit over that baseline is claimed.

## Requirements

- `LED-V0-001`: The companion MUST enable semantic mode only with explicit operator executable and root arguments, and advertise definition/full-text sync only after successful backend initialization. Multiple or mismatched roots MUST be rejected.
- `LED-V0-002`: Open/change/close MUST retain exact captured overlay identity across synchronization. Currentness MUST check the actual capture; version or digest alone MUST NOT identify reopened documents. Unsupported incremental updates and invalid synchronization MUST terminate the session without claiming current evidence.
- `LED-V0-003`: Definition MUST use one bounded pending request, service cancellation while work is pending, cancel and join before document mutation, and recheck source and target snapshots before replying. Only synchronized open overlay targets are admitted; disk targets MUST return explicit unavailability.
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

The live subtest requires explicit CORVINT_TEST_GOPLS and otherwise skips visibly. Fake sessions
prove joining behavior, not editor/provider interoperability. Client qualification, performance,
agent usefulness and promotion remain separate. Rollback removes the operator flags or stops the
companion; the original capability-free mode and Core remain available.
