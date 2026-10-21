# MCP context call releases its snapshot mapping (V1-1004)

Date: 2026-10-21. Base: `5cf75daf0` (`origin/main`). Requirement: `MCPV0-090`
(`docs/specs/mcp-server-2026-07-28-v0.md`), accepted under the owner's standing lane auto-accept
(2026-10-11).

## Finding

`callContext` never released the Index it read. Under `CORVINT_SNAPSHOT_FORMAT=pack` or
`sectioned`, each call's lease kept the mapping alive after the retention ring evicted it, so a
long-lived bridge pinned every mapping it had read until the process exited. The test reproduced
this: with the release removed, 1 mapping stays live after the ring is evicted. The default gob
format maps nothing, so the default path was unaffected. `compileContext` also dropped the first
Index without releasing it when a deferred read was refused and retried eagerly.

## Decision

The ticket allowed either an escape audit or a canonical-bytes copy. The lane chose the copy,
because an audit only holds until the packet builder next changes. Before the deferred release,
the packet is re-decoded from its canonical bytes, with `UseNumber` so number literals survive.
The copy must re-encode to the same bytes, or the call refuses with `internal-error`. A struct
value whose field order differed from the sorted map order would fail closed there instead of
changing the wire. The binding strings are cloned.

On the fixture, the uncopied packet did not fault after the unmap either, so the copy is a
defensive bound, not a reproduced crash. The release is what fixes the reproduced leak.

## Evidence

- `TestMCPV0090ContextCallReleasesItsSnapshotMapping` (`internal/contextindex`, external test package
  over the task-review bridge) ran 1 sequential and 24 concurrent context calls from a mapped pack:
  - the ring held 1 mapping, and 0 stayed live after eviction;
  - all 25 results re-encoded after the unmap to the first result's bytes;
  - it passed under `-race`.
- `go test ./internal/mcp/bridge/` passed (21.6 s), as did the context and semantic tests in
  `internal/lspstdio` and the review tests in `cmd/corvint-mcp`.

Rollback: remove the deferred `Release`, `detachedContext` and the retry release.
