# MCP multi-root profile V0

Owner: Russell Lewis
Date: 2026-10-07
Requirement prefix: `MMR-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: AGENTS.md invariants 2, 3, 4, 7 and 8; `mcp-server-2026-07-28-v0.md`
`MCPV0-001`, `MCPV0-015`; native ticket `V1-0938`.

## Agent digest
- Claim: One corvint-mcp process serves up to 32 operator-declared repositories, each named by an alias that every repository-scoped tool call must select.
- Status: proposed (pending owner acceptance; V1-0938) / experimental
- Exists: `cmd/corvint-mcp/roots.go`, `cmd/corvint-mcp/main.go` (`parseArguments`, `toolHandler.call`), `bridge.Registry.SharesRoot`; tests in `cmd/corvint-mcp/roots_test.go` and `main_test.go`.
- Blocked on: owner acceptance; no qualified host run; `corvint-test-validity-mcp`, `corvint-docs-mcp` and `corvint-corpus-mcp` remain single-root.
- Read next: Requirements; Bounded state and measurements.

## Owner intent and scope

An agent working across several repositories needs one `corvint-mcp` child per repository today
(`MCPV0-001`), so a host with ten checkouts runs ten processes and lists ten copies of the tool
catalogue. This profile lets the operator declare several roots on one command line. Authority stays
with the operator: roots come only from argv, are validated and pinned at startup exactly as the
single root is, and no client capability or request field can add, remove or switch one
(`MCPV0-015` is unchanged). A single plain `--root ABSOLUTE_ROOT` remains the `MCPV0-001` server
byte for byte; the multi-root form is selected only by the aliased spelling below.

Roots are declared as `ALIAS=ABSOLUTE_ROOT` rather than derived from basenames, because linked
worktrees of one repository commonly share a basename and a derived name would change when a
directory is renamed. An explicit alias is stable, short in every schema, and never carries a path
into the tool catalogue.

## Requirements

- `MMR-V0-001`: (proposed, pending owner acceptance; V1-0938) `corvint-mcp` MUST accept
  `--root ALIAS=ABSOLUTE_ROOT`, repeated, as the multi-root form. One aliased root already selects
  multi-root mode. A value is aliased only when the text before its first `=` is a valid alias
  (`MMR-V0-002`); otherwise the whole value is a plain root. A plain root MUST appear alone: exactly
  one plain `--root` is the unchanged `MCPV0-001` server, whose `tools/list` and tool results are
  byte-identical to the server before this profile, and a plain root combined with any other root is
  refused. An aliased declaration with an empty root is refused. The form composes with
  `--protocol-version`, `--tool-profile` and `--error-profile` exactly as the single-root form does.
- `MMR-V0-002`: (proposed, pending owner acceptance; V1-0938) An alias MUST match
  `^[a-z][a-z0-9-]{0,31}$`, aliases MUST be unique, and at most 32 roots may be declared. Each root
  keeps the `MCPV0-001` bounds (absolute, clean, at most 4,096 UTF-8 bytes). A duplicate alias, more
  than 32 roots, or any argv shape outside `MMR-V0-001` MUST exit 2 with
  `corvint-mcp: invalid arguments` before any MCP byte is written.
- `MMR-V0-003`: (proposed, pending owner acceptance; V1-0938) Every declared root MUST be validated
  and pinned at startup by the same `MCPV0-001` rules and code as the single root, under the selected
  tool profile, before the server reads stdin. Any root that fails exits 2 with
  `corvint-mcp: repository unavailable`. The same directory declared under two aliases, under any
  spelling (symlink, case-folded path), compared by the pinned canonical root identity, MUST exit 2
  with `corvint-mcp: invalid arguments`. Distinct linked worktrees of one repository are distinct
  roots.
- `MMR-V0-004`: (proposed, pending owner acceptance; V1-0938) In multi-root mode `tools/list` MUST
  return the selected profile's tool set, each input schema equal to the single-root schema plus one
  property `repository` of `{"type":"string","enum":[ALIASES]}`, aliases in byte order. `repository`
  MUST be required for every tool except `corvint.status`, where it is optional. No root path appears
  in the catalogue. `cacheScope` and `ttlMs` are unchanged.
- `MMR-V0-005`: (proposed, pending owner acceptance; V1-0938) A tool call MUST reach only the
  repository its `repository` alias names; the selector is removed before the bridge's closed
  argument decode, so every other argument is validated exactly as in single-root mode. A missing
  selector (except on `corvint.status`), a non-string selector, an undeclared alias, a case variant,
  or a path in its place MUST be refused as JSON-RPC `-32602 Invalid params`, the same mapping the
  bridge's `invalid-arguments` code already has. Client `roots` capabilities, `_meta` members and any
  `root` or `roots` argument MUST NOT add, remove or switch a repository; the latter two remain
  closed-decode refusals, including on `corvint.status` without a selector.
- `MMR-V0-006`: (proposed, pending owner acceptance; V1-0938) `corvint.status` with no arguments
  MUST return every declared binding as the closed object
  `{"schema":"corvint-mcp-multi-root-status/0","tool":"corvint.status","mutates":false,"repositories":[...]}`,
  one `{"repository":ALIAS,"result":RESULT}` entry per alias in byte order. Each `RESULT` MUST equal
  the `structuredContent` a `corvint.status` call naming that alias returns at the same moment: the
  bridge result object, or the `corvint-mcp-tool-error/0` (or `/1`) object for that alias's failure.
  The text block is the canonical JSON inside the `MCPV0` untrusted-data envelope; `isError` is
  `false` because the listing itself succeeded. Cancellation is `-32603 Internal error`, as in
  single-root mode. Per-alias results carry no alias member; their closed `/0` shapes are unchanged
  and a client correlates them by request id.
- `MMR-V0-007`: (proposed, pending owner acceptance; V1-0938) Every result MUST bind to its own
  root's startup pin and current revision, re-verified per call as `MCPV0-001` requires. No result,
  abstention or cached value may be shared or mixed across roots: the bridge registries are
  independent, and the process-wide snapshot caches key on absolute path, size, modification time
  and content digest. A commit in one root changes only that root's binding. Interleaved calls across
  roots in one process MUST each return their own root's revision.
- `MMR-V0-008`: (proposed, pending owner acceptance; V1-0938) Per-root resident state MUST be only
  the startup identity pin (paths and `os.FileInfo` identities; no file descriptor, index, goroutine
  or timer). Index handles stay per call and are bounded by the server's 64-request concurrency
  ceiling; the only cross-call retention is the existing process-wide snapshot mapping cache of 4
  pack plus 4 sectioned entries, independent of the root count. An idle server performs no background
  work. The status-all listing probes roots sequentially.

## Bounded state and measurements

The ticket asked for lazy opening with at most K open roots closed in LRU order. The implementation
records K = 0 retained per-root index handles instead, because nothing per root is held open between
calls: each call already opens, builds and drops its own context index. What stays resident per root
is the startup identity pin, which is under 1 KiB and holds no descriptor. Closing a pin and reopening
it later would re-resolve the root and could silently accept a directory swapped while it was closed,
which `MCPV0-001` forbids, so the pins are eager and permanent. Memory is therefore bounded by
32 pins, 64 concurrent call working sets, and 8 retained snapshot mappings; descriptors are bounded
by the concurrent calls alone.

Measured on Darwin arm64 at the implementing commit, empty `.git` fixtures, one `tools/list` per
process (`wc -c` of the response line; maximum RSS from `/usr/bin/time -l`):

| Profile | Roots | One multi-root server: bytes / processes / max RSS | N single-root servers: bytes / processes / max RSS (sum) |
|---|---|---|---|
| default | 1 | 3,772 / 1 / 20.7 MB | 3,603 / 1 / 20.2 MB |
| default | 3 | 3,820 / 1 / 20.9 MB | 10,809 / 3 / 60.6 MB |
| default | 10 | 3,991 / 1 / 20.3 MB | 36,030 / 10 / 202.0 MB |
| task-review | 1 | 5,429 / 1 / 20.5 MB | 5,138 / 1 / 20.7 MB |
| task-review | 3 | 5,509 / 1 / 21.1 MB | 15,414 / 3 / 62.0 MB |
| task-review | 10 | 5,794 / 1 / 20.3 MB | 51,380 / 10 / 206.6 MB |

A status-all call over ten committed fixture repositories took 0.44 s wall time at 22.5 MB maximum
RSS, against 0.07 s and 21.2 MB for one single-root status call. These are single local
observations, not a benchmark gate.

## Non-goals and failure modes

Non-goals: client-provided roots, `roots/list`, or any other client-driven authority; adding or
removing roots while running; a different tool profile per root; cross-root query, impact or context
aggregation (only status lists every binding); Streamable HTTP or any listener; and changing any
`MCPV0` requirement, vector or single-root byte. Follow-up non-goal: `corvint-test-validity-mcp`,
`corvint-docs-mcp` and `corvint-corpus-mcp` each keep their own argument parser and registry and stay
single-root. The alias grammar, schema enum and routing in `cmd/corvint-mcp/roots.go` depend only on
a registry that lists tools and serves calls, so they can move to a shared package when one of those
servers needs this profile; that needs its own requirement rows.

Failure modes:
- Argv outside `MMR-V0-001` or `MMR-V0-002`, or one directory under two aliases: exit 2,
  `invalid arguments`, nothing on stdout.
- Any declared root unavailable at startup: exit 2, `repository unavailable`; no partial server.
- Missing or unknown alias on a call: `-32602 Invalid params`; no repository is touched.
- A root swapped, retargeted or replaced after startup: that root's calls abstain with
  `ROOT_IDENTITY_CHANGED` (`MCPV0-001`); other roots are unaffected, and status-all reports the
  abstention in that root's entry.
- One root's status failure: that entry carries the tool-error object; the other entries are intact.
- Limits: the status-all listing is sequential, so its latency grows with the root count; the
  flows-impact serialization lock is process-wide, so flows-impact calls serialize across roots.
  Evicted snapshot mappings are dropped from the cache without being unmapped (pre-existing,
  independent of this profile), so a long-lived process that rotates through more than four snapshots
  keeps the older mappings' address space until exit; this is recorded as a suspected defect, not
  claimed fixed here.

## Acceptance evidence and rollback

Acceptance evidence: the maintained tests in the traceability table pass; the existing
`cmd/corvint-mcp` golden `tools/list` files and the `conformance/mcp-2026-07-28` black-box suite pass
unchanged; the measurements above are recorded in
`docs/build-log/2026-10-07-mcp-multi-root.md`. Owner acceptance of this proposed intent is pending.

Rollback: revert the implementing commit. A plain single `--root` is unchanged by this profile, so
existing configurations are unaffected; a host using aliased roots returns to one server per root.
No persisted state, index format or wire field of the single-root profile changes.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `MMR-V0-001` | `cmd/corvint-mcp/main.go` `parseArguments`, `run`; `roots.go` `splitAlias` | `TestParseArgumentsIsClosed`, `TestMMRV0001SingleRootBytesUnchanged`, `TestAFUV1034FlowsToolProfile` (golden single-root bytes) |
| `MMR-V0-002` | `roots.go` `validAlias`, `maxRoots`, `maxAliasBytes`; `main.go` `parseArguments` | `TestParseArgumentsIsClosed`, `TestMMRV0003StartupRefusals` |
| `MMR-V0-003` | `roots.go` `openRepositories`; `bridge.Registry.SharesRoot` | `TestMMRV0003StartupRefusals` |
| `MMR-V0-004` | `roots.go` `multiRootTools`; `main.go` `Handle` | `TestMMRV0004RepositoryEnumOnEveryTool` |
| `MMR-V0-005` | `roots.go` `route`; `main.go` `call` | `TestMMRV0005AliasRoutingAndRefusal`, `TestMMRV0005ClientRootsNeverSwitchRepository` |
| `MMR-V0-006` | `roots.go` `statusAll`; `main.go` `toolErrorObject` | `TestMMRV0006StatusReportsEveryBinding` |
| `MMR-V0-007` | `bridge.Registry` per root; `contextindex` path-keyed caches | `TestMMRV0005AliasRoutingAndRefusal` (interleaved calls across three roots), `TestMMRV0006StatusReportsEveryBinding` (commit in one root) |
| `MMR-V0-008` | `bridge.Registry` pin-only state; `contextindex` `packCacheCapacity` | Measurements above; `TestMMRV0005AliasRoutingAndRefusal` (interleaved calls stay bound) |
