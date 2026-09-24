# Corvint MCP server

Status: experimental; protocol `2026-07-28`; local stdio only; compatibility `FALLBACK`.

`corvint-mcp` exposes bounded read-only Corvint receipts to a client that implements the
[MCP 2026-07-28 specification](https://modelcontextprotocol.io/specification/2026-07-28). It does
not expose repository source, prompts, writes, test execution, updates, merges, telemetry, HTTP, or
a network listener. The binding contract is
[`docs/specs/mcp-server-2026-07-28-v0.md`](specs/mcp-server-2026-07-28-v0.md).

## Build and run

The server requires Go 1.27.1 and one absolute repository root:

```console
$ GOTOOLCHAIN=local go build -o ./bin/corvint-mcp ./cmd/corvint-mcp
$ /absolute/path/to/corvint-mcp --root /absolute/path/to/repository
```

Configure an MCP client to own that child process. The portable shape is:

```json
{
  "command": "/absolute/path/to/corvint-mcp",
  "args": ["--root", "/absolute/path/to/repository"],
  "transport": "stdio"
}
```

Use absolute paths. Do not wrap the command in a shell. Do not configure a URL, port, legacy
`initialize` request, environment credential, or writable working directory as authority. One
process is bound to one root for its entire lifetime; request arguments cannot switch it. The root
is capped at 4,096 UTF-8 bytes, canonicalized through existing symlinks once, and must contain a
non-symlink `.git` directory. Strict V0 rejects a regular-file `.git` marker, including linked
worktrees.

Every request, including `server/discover`, must contain:

```json
"_meta": {
  "io.modelcontextprotocol/protocolVersion": "2026-07-28",
  "io.modelcontextprotocol/clientCapabilities": {}
}
```

The 2026-07-28 protocol is stateless. It has no `initialize`/
`notifications/initialized` lifecycle. A client supporting modern and legacy MCP should probe
`server/discover`; this server returns method-not-found for a legacy `initialize` request and
ignores a legacy `notifications/initialized` notification without responding.

## Advertised surface

`server/discover` advertises only `tools` with `listChanged: false`. `tools/list` returns:

| Tool | Arguments | Meaning |
|---|---|---|
| `corvint.query` | `{"task":"PRINTABLE ASCII TASK"}` | Authority-start context at the native fixed limit of one; unsupported intent abstains |
| `corvint.impact` | `{"paths":["relative/file.go"],"limit":10}` | Revision-bound Go impact evidence; `limit` is optional, 1–50 |
| `corvint.status` | `{}` | Exact commit/tree/object-format/profile/worktree/dirty-count-and-digest binding |

### Task-review profile (opt-in)

Starting the server with one extra closed selector adds two review tools (`MCPV0-024..026`,
decision 0374):

```console
$ /absolute/path/to/corvint-mcp --root /absolute/path/to/repository --tool-profile task-review
```

| Tool | Arguments | Meaning |
|---|---|---|
| `corvint.context` | `{"task":"TASK","subject":"relative/path","limit":20}` | The `corvint context` task-context packet at the bound revision; `subject` and `limit` (1–50) are optional |
| `corvint.cem.report` | `{"map":"relative/change.cem.json","expectedBase":"FULL_OID","target":"FULL_OID"}` | The `corvint cem report` reviewer report as a preview that writes nothing; optional `maxUnknown`/`maxMechanical` |

The only accepted value is `task-review`, given once, as two separate arguments. Any other value,
a repeated selector, `--tool-profile=task-review`, or the selector with `--version` exits 2 before
the repository is opened. Without the selector the server lists exactly the three tools above, and
a call to `corvint.context` or `corvint.cem.report` fails as an unknown tool (`-32602`). The
selector composes with `--protocol-version 2025-11-25`, in either order.

Discovery is root-independent and returns `cacheScope: "public"`, `ttlMs: 0`, and fixed server info:
name `corvint-mcp`, build version, and Corvint description
`Local read-only Corvint context and repository evidence server.` Tool listing is root-bound and
returns `cacheScope: "private"`, `ttlMs: 300000`, and no next cursor.

All input schemas are closed. Query text is 1–2,000 printable ASCII characters. Impact accepts
1–100 unique normalized repository-relative `.go` paths, each at most 1,024 Unicode scalar values.
Absolute paths, `.`/`..`/empty segments, non-Go targets, path escapes, extra fields, and invalid
limits are rejected before repository access. Context `task` is 1–32,000 bytes and not blank. The CEM
`map` is a repository-relative path of at most 512 bytes with no `.git` segment; a symlinked map or
ancestor is refused as `cem-map-unavailable`, so a repository file cannot redirect the read outside
the bound root. `expectedBase` and `target` must be full object IDs.

Tool names are unique, case-sensitive, 1–128 characters, and use only letters, digits, `_`, `-`,
or `.`. All `corvint.*` names, in both profiles, follow the official 2026-07-28 grammar.

Successful calls return the complete canonical `corvint-mcp-bridge-result/0` receipt both as one text
content block and as the identical parsed `structuredContent`. `READY` means the bounded operation
returned its native receipt; it does not mean Corvint proved the requested proposition.
`ABSTAINED`, `UNKNOWN`, `NOT_OBSERVED`, exclusions, mixed-worktree state, and unsupported intent are
results, not missing fields. Source and diff bodies are never returned.

An impact receipt with native state `OUT_OF_SCOPE` is retained while its bridge wrapper says
`ABSTAINED`/`NOT_OBSERVED` with reason `OUT_OF_SCOPE`; this preserves the exclusion evidence.
Unsupported and output-budget abstentions have a null native receipt.
The canonical bridge receipt is capped at 393,216 bytes and must also fit its duplicated
text-plus-structured MCP response with an 8,192-byte reserve; over-budget output abstains instead of
truncating JSON.

Resources, prompts, subscriptions, completion, sampling, elicitation, tasks, standalone evidence,
dashboard snapshots, frontier/why/live operations, logging, and list-change notifications are not
advertised. Calling one returns method-not-found. `io.modelcontextprotocol/logLevel` does not opt
this profile into logs.

## Limits and lifecycle

- One UTF-8 JSON object per LF-delimited input line; one JSON object per output line.
- Input and output JSON are each at most 1,048,576 bytes before LF; nesting depth is at most 64.
- String request IDs are at most 128 UTF-8 bytes; integer IDs are signed 64-bit. Larger IDs produce
  `-32600` without echoing the rejected ID. Protocol-version values are at most 64 UTF-8 bytes;
  larger values produce `-32602` without reflection.
- Duplicate keys, invalid UTF-8/JSON, over-limit input, and excess depth receive one sanitized
  parse error after the frame is drained. The next complete frame may proceed.
- A valid `notifications/cancelled` cancels only its matching in-flight request and, if it wins the
  output race, no further message for that request is emitted. Clean EOF stops admission and drains
  admitted responses; SIGINT, SIGTERM, and transport failure cancel owned work. Shutdown always
  waits for descendants.
- Progress is emitted only when the request supplied a progress token and a real intermediate state
  exists. No delivered tool currently exposes a truthful long-running progress point, so emitted
  progress execution is `NOT_OBSERVED`. The server sends no unsolicited logging or telemetry.
- Tool listing is deterministic and currently one page. A supplied non-empty or stale cursor is
  invalid; tool receipts are never paged or silently truncated.
- Client-advertised roots are ignored. Repository authority is exclusively the startup `--root`.

## Error interpretation

| Code/state | Meaning | Action |
|---|---|---|
| `-32700` | Invalid, duplicate-key, too-deep, or over-limit JSON frame | Fix framing; the server never echoes rejected input |
| `-32600` | Invalid JSON-RPC request envelope | Send JSON-RPC `2.0`, a string/integer ID, method, and object params |
| `-32601` | Unknown, removed legacy, or unadvertised request method | Re-run `server/discover`; do not assume an older MCP surface |
| `-32602` | Missing request metadata, invalid cursor, unknown tool, or invalid closed arguments | Correct the request; no repository operation ran |
| `-32022` | Unsupported protocol version | Retry only with one of the returned `supported` versions |
| `-32603` | Sanitized unexpected server failure | Treat the operation as unobserved; inspect local process health |
| `ABSTAINED` | Corvint could not safely produce the requested bounded receipt | Preserve the receipt's reason and gaps; widen through an explicit supported operation |

Errors never include request bodies, repository paths, task text, environment values, Git output,
Corvint output, or secrets. stdout is exclusively MCP. Fixed startup, invalid-argument,
repository-unavailable, and fatal transport diagnostics use sanitized stderr; per-request MCP logs
are never emitted.

## Privacy and authority

Corvint reads local Git and repository evidence under the configured root. It does not fetch or make
network requests. Receipts may contain repository-relative paths and content identities even though
source bodies are omitted; treat them as sensitive local metadata. Discovery and tool-list results
carry different cache hints: root-independent discovery is public and immediately stale; the fixed
tool registry is private with a five-minute TTL. Never infer correctness, exhaustive coverage, or a
clean worktree from a successful transport response.

`corvint.status` does not claim Corvint index freshness, dashboard/evidence availability, or an observed
root digest. Those fields are outside the delivered repository-binding status and remain
`NOT_OBSERVED`.

The server performs no Corvint executable PATH lookup because it calls the Go kernels in process.
The current candidate's inherited Git runners still resolve `git` through PATH per invocation;
post-start executable pinning is `NOT_OBSERVED`. This is a known release blocker, so the candidate
must remain experimental until the runners accept one absolute start-time-pinned Git executable and
the adversarial conformance case passes.

## Verification status

The required gate is a compiled-process black-box suite under `conformance/mcp-2026-07-28`, plus Go
1.27 unit, race, vet, fuzz, process-cleanup, security, and supported cross-build checks. The focused
unit and compiled-process suites passed locally on 2026-08-23 with Go 1.27.0 and an isolated build
cache. Official 2026-07-28 MCP conformance remains `NOT_RUN`: the official server runner requires a
Streamable HTTP URL and offers no stdio-server target, while this profile deliberately exposes no
listener. The manifest records the exact official schema URL and observed SHA-256
`ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203` but does not execute schema
validation; official-schema-driven conformance is also `NOT_RUN`. The local suite
observes a cancellation race and post-cancel health. Its Unix-gated compiled-process fixture also
passed locally for both INT and TERM: an in-flight fake-Git process and its background child exited
with no surviving descendant. A notification-cancellation-won long-running operation, emitted
progress, and equivalent non-Unix cleanup remain `NOT_OBSERVED`. The local pass is not an official
interoperability or host-compatibility claim.

Do not promote this surface from experimental or mark a host tuple `FULL` until the owning spec's
entire acceptance matrix passes. Disable or remove the child-process configuration to roll back;
the server creates no repository, trace, database, network, or migration state.
