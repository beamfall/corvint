# LSP Snapshot Overlay V0

Owner: Russell Lewis
Date: 2026-09-29
Intent status: proposed technical contract; owner accepted agent and optional editor direction
Delivery status: experimental
Authoritative inputs: `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`, owner V1-0480 request; proposed LQP-V0-003/004/006/014 platform requirements.

## Agent digest
- Claim: Experimental in-memory snapshots separate syntactic Git identity from bounded session overlays.
- Status: proposed technical contract; owner accepted agent and optional editor direction/experimental
- Exists: isolated native-Go package with immutable captures, full-text edits, stale checks and bounded live storage.
- Blocked on: independent review, final evidence binding, actual Git validation, evidence joins, transport integration and client qualification.
- Read next: Requirements; Limits and failure modes; Acceptance evidence.

## User job and current state

The owner requested high-quality LSP evidence for agents and optional editors, initially Go/gopls,
VS Code and Neovim. V1-0480 needs a common snapshot foundation to keep unsaved content distinct
from immutable Git facts. This experiment implements only that foundation. At base
`75f06f9020a38af8a775a35479b4b16cc69b8e3c`, `internal/lspevidence/context.go` provided optional
Go semantic evidence but no shared overlay store. The simpler baseline is an upstream editor server
plus existing committed-only Corvint reads. No current consumer is changed by this package.

## Requirements

- `LSO-V0-001`: Git identity MUST contain repository, lexical root URI, object format, commit, tree, blob and relative path; validation MUST be syntactic only and MUST NOT claim object existence or verified content.
- `LSO-V0-002`: Client text MUST be stored as immutable full-text captures with session, lexical URI, version and SHA-256 digest. Changes MUST strictly increase version; rejected changes MUST preserve admitted state.
- `LSO-V0-003`: Close/reopen and explicit reset MUST invalidate old captures even when version and content repeat. Session instances and root epochs MUST isolate observations. Callers MUST signal branch, root, save and rename changes; this package MUST NOT infer events from disk.
- `LSO-V0-004`: URI admission MUST require one lexical local file URI spelling contained below the declared root, rejecting remote authorities, query/fragment, traversal and encoded aliases. Lexical admission MUST NOT claim symlink or case identity resolution.
- `LSO-V0-005`: Sessions MUST enforce explicit positive document-count, per-document-byte and total-live-content-byte ceilings before mutation and own retained strings independently of caller backing allocations. Reads MUST NOT write, execute processes, learn or convert overlays into Git evidence.

## Limits and failure modes

`Bounds` are explicit caller-proposed experimental values, not accepted deployment ceilings.
URI strings are limited to 4096 bytes, session IDs to 128 bytes and Git repository/path strings to
4096 bytes. UTF-8 client content and nonnegative int64 versions are required. This initial subset
rejects negative protocol versions; a transport must surface that limitation rather than claim full
LSP synchronization support. Only full text replacement is supported. Errors distinguish invalid
identity, state, stale order and resource exhaustion without echoing paths or content.

A mutex serializes store mutation. Capture values retained by the caller are outside the live-store
budget; callers own their lifetime and aggregate request budgets. Generation/epoch exhaustion fails
closed. Root and URI comparisons are lexical and case-sensitive; filesystem aliases and symlinks
remain unresolved. Git identities are untrusted input; external verification must establish actual
repository/object membership before an evidence engine uses them. `IsCurrent` is a point-in-time
check; consumers must serialize their final publication with event handling to avoid a check/use race.

Save/rename integration may conservatively reset all live documents and reopen them from client text.
There is no filesystem watcher, persistence, semantic fact join, ranking, adapter, protocol handling,
editor integration or new Core behavior. Caller-held immutable GitIdentity values survive resets;
that does not establish that they describe the current checkout. No editor/agent parity or profile
promotion is claimed, and LQP-V0-005/006/014 remain open beyond this bounded foundation.

## Acceptance evidence

| Requirement | Implementation | Test witness |
|---|---|---|
| LSO-V0-001 | `internal/lspsnapshot/snapshot.go` | `TestGitIdentity` |
| LSO-V0-002 | same | `TestVersionsAndCapture`, `TestConcurrentVersions` |
| LSO-V0-003 | same | `TestInvalidationAndIsolation`, `TestEqualSessionCoordinates` |
| LSO-V0-004 | same | `TestURIAdmission` |
| LSO-V0-005 | same | `TestBounds`, `TestCallerBackingOwnership` |

Run `go test -race -count=1 -timeout 30m ./internal/lspsnapshot` and `go vet ./internal/lspsnapshot`.
Retained local observations are in `docs/build-log/2026-09-29-lsp-snapshot-overlay.md`.
Client interoperability, resource distributions and independent outcome qualification are NOT_RUN.

## Rollback and drift

Delete this unreferenced package and its proposed spec to roll back; no persistent state needs migration.
Any integrated consumer or changed identity/limit contract requires additional specification and review.
The native ticket remains open until its broader acceptance evidence and completion write succeed.
