# MCP Server 2026-07-28 V0

Owner: Russell Lewis  
Date: 2026-08-23  
Intent status: proposed  
Delivery status: experimental  
Authoritative inputs: `docs/PRODUCT.md`, `docs/DOGFOOD.md`,
`docs/specs/agent-harness-integration-v0.md`, the
[MCP 2026-07-28 specification](https://modelcontextprotocol.io/specification/2026-07-28), and its
official JSON Schema pinned at commit
[`271ecc9accafdd9b83a3c869fa67c22953b2af80`](https://github.com/modelcontextprotocol/modelcontextprotocol/blob/271ecc9accafdd9b83a3c869fa67c22953b2af80/schema/2026-07-28/schema.json)
(sha256 `ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203`, matching the
`officialSchema.observedSha256` already recorded in `conformance/mcp-2026-07-28/cases.json`)

## Agent digest
- Claim: A local stdio MCP server exposes bounded read-only Corvint receipts, with 2026-07-28 default and explicit 2025-11-25 compatibility.
- Status: proposed/experimental
- Exists: `cmd/corvint-mcp`, `internal/mcp`, and independent compiled-process conformance vectors.
- Blocked on: official MCP conformance and promotion evidence remain `NOT_RUN`; official-schema execution is opt-in and passed on 2026-09-23 (V1-0191).
- Read next: User and measurable job; Explicit 2025-11-25 compatibility; Traceability.

## User and measurable job

A local coding agent can start one child `corvint-mcp` process for one repository and obtain the
same revision-bound, authority-preserving Corvint receipts available from the Go CLI, without source
bodies, repository mutation, a listener, or a second knowledge model. V0 is useful only if a
2026-07-28 client can discover it and exercise every advertised method through black-box stdio
conformance while malformed or adversarial input stays bounded.

MCP is a portable `FALLBACK` surface under the Agent Harness Integration contract. A passing MCP
connection does not make any native host tuple `FULL` and does not prove a Corvint workflow useful.

## Verified current state and decision

- The official 2026-07-28 protocol is stateless. It removed the legacy `initialize`/
  `notifications/initialized` handshake; every request instead carries the protocol version and
  client capabilities in `params._meta`, and every modern server implements `server/discover`.
- Corvint's Go kernel exposes bounded read-only query and impact primitives. Their receipts preserve
  repository identity, authority, freshness, exclusions, uncertainty, and abstention.
- No independently stable standalone evidence-resource or dashboard-snapshot bridge exists for this
  slice. V0 therefore does not advertise resources or those operations.
- Official Go SDK v1.7.0 at commit `bc72835f62eb94d0fb484439f886b6885b075f36` is rejected for V0.
  Its default surface includes historical protocol versions and the stateful legacy initialization
  model; its `encoding/json` path accepts duplicate object names; stdio message and nesting bounds
  are not configurable at the required boundary; and its dependency closure includes JWT, JSON
  Schema, Segment, OAuth 2, time, tools, assembly, `x/sys`, and `x/sync` packages. Wrapping those
  properties would replace the protocol core rather than reduce it. V0 uses a minimal dependency-
  free Go protocol core derived from inspection of the official schema. The conformance manifest
  pins that schema's URL and observed SHA-256
  `ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203`. The schema JSON is not
  vendored, so the default no-network suite skips official-schema execution. An operator who names
  a local copy with `CORVINT_MCP_OFFICIAL_SCHEMA` runs `TestServerTrafficMatchesOfficialSchema`,
  which refuses any other digest and validates the suite's requests and the live server's discovery,
  tool-list, tool-call and error responses against the schema's `$defs` (V1-0191).
- The candidate bridge invokes Corvint kernels in process and therefore performs no Corvint executable
  PATH lookup, shell execution, or user-controlled argv. Its inherited context-index, Go-kernel and
  planning-snapshot Git runners resolve `git` through one shared resolver. `corvint-mcp` pins that resolver to one
  absolute Git executable at start and refuses to start when Git does not resolve, so no later PATH
  change reaches a spawn. Direct argv, sanitized output, and process-group cleanup exist; the
  compiled-process case `TestGitPlantedOnPathAfterStartNeverRuns` observes the pin (V1-0191).

The repository unit and compiled-process black-box suites passed locally on 2026-08-23 with Go
1.27.0 using an isolated build cache. The official MCP conformance server runner remains `NOT_RUN`:
it requires a Streamable HTTP URL and does not expose a stdio-server target, while P0 deliberately
exposes no listener. The local result is implementation evidence, not official interoperability or
host-compatibility evidence.

### Isolated status reads (2026-09-06 repair)

Every MCP tool call enables the shared `internal/gitstatus` isolation boundary before repository
work. Status runs with an explicit admitted worktree and private copied Git metadata outside the
worktree and Git directory. Configuration is copied with bounded no-follow reads, parsed without
include expansion, and reused byte-for-byte; status never falls back to live configuration.
The temporary metadata is removed on return, error, or cancellation. Two cancellable status
slots bound its scratch allocation to two private copies independently of the 64 admitted
transport requests.

This experimental profile refuses configuration includes, executable clean/process filters,
external `core.attributesFile` settings, redirected worktrees, bare repositories, ambiguous
multiline config records, unsupported metadata paths/formats, split indexes and index gitlinks.
It also refuses unavailable or repository-contained temporary storage. Refusals use the existing
sanitized repository-unavailable tool failure; they do not silently change normalization or claim
that skipped submodule state was observed. Ordinary supported repositories, including immutable
object formats and standard worktree state, retain their existing receipt semantics. The MCP
startup restriction on linked-worktree `.git` markers is unchanged.

The same isolation is enabled by `docs draft` and `docs consume`. Other standalone CLI commands
retain their existing Git execution boundary and query budgets. Non-status Git reads and the
previously recorded executable-PATH limitation are not promoted by this repair. The new compiled
process cases cover filter nonexecution, root binding before and after admission, and rejection of
explicit null against the advertised integer impact limit. Official interoperability, schema
execution, and broader promotion gates remain open.

## Relationship to hosted MCP

[`deployment-neutral-index-platform-v0.md`](deployment-neutral-index-platform-v0.md) authorizes a
future authenticated HTTP MCP deployment profile over the same native-Go query kernel and immutable
index format. This V0 remains the local stdio profile: its no-listener, one-root, read-only,
accountless contract and current delivery evidence are unchanged. HTTP transport, authentication,
tenant isolation, opaque handles, consistency barriers, and provider synchronization require the
separate descendant profile and cannot be inferred from a passing stdio test. Remote source-content
exposure remains denied until that descendant freezes its own authorization, redaction, limit,
receipt, cache, and denial policy; the local source-content-free policy does not silently transfer.

## Requirements

- `MCPV0-001`: The executable MUST run as `corvint-mcp --root ABSOLUTE_ROOT`. It owns exactly one local
child process and one repository root. At startup, `ABSOLUTE_ROOT` MUST be absolute, clean, bounded,
free of control characters, and resolved through existing symlinks once to a canonical absolute
root. The supplied root is at most 4,096 UTF-8 bytes. Its `.git` marker MUST be a non-symlink
directory. A regular-file `.git` marker, including a linked worktree, is rejected in strict V0.
Requests cannot replace the root. The canonical root MUST resolve to the Git worktree observed by
Corvint before a tool can return repository evidence. A path escape, repository identity drift, or an
unavailable Git snapshot MUST fail closed for the Corvint operation. Clean and mixed worktrees are
both reportable states; neither may be silently converted to the other.

- `MCPV0-002`: P0 transport is stdio only. Each message is one UTF-8 JSON object followed by LF. The
JSON bytes before LF and the emitted JSON bytes before LF are each limited to 1,048,576 bytes. A
single optional CR immediately before LF is JSON whitespace and counts toward that limit.
The server MUST NOT open a socket, read stdin as anything other than MCP frames, write non-protocol
bytes to stdout, or accept JSON-RPC batches.

- `MCPV0-003`: The decoder MUST reject invalid UTF-8, invalid JSON, duplicate object names at any
nesting level, a nesting depth greater than 64, and an over-limit frame with one sanitized parse
error (`-32700`) and no request ID after the fully delimited frame is drained. A syntactically valid
non-object top-level value, including a JSON-RPC batch, is an invalid request (`-32600`). An
unterminated frame at EOF produces at most one parse error, drains already admitted responses as a
clean EOF does, and then the process exits. No error may
echo input, a root, path, argument, environment value, Git output, or Corvint output.

- `MCPV0-004`: A valid request has exactly JSON-RPC `2.0`, a string or integer `id`, a method, an object
`params`, and the method's closed parameter shape. Notifications have no `id`; request IDs are
opaque and responses preserve their JSON scalar exactly. String IDs are at most 128 UTF-8 bytes;
integer IDs are signed 64-bit values. An over-limit string ID or out-of-range integer is an invalid
request (`-32600`) whose response omits the rejected ID. A request whose ID equals a request still in
flight is an invalid request (`-32600`) whose response also omits the ID, so a client cannot correlate
the rejection with the live request. A request leaves flight when its response wins the output race
(`MCPV0-011`), before that frame is written, so a client may reuse the ID once it has read the
response. Invalid envelopes return `-32600`; unknown
or unadvertised methods return `-32601`; invalid parameters, missing per-request metadata, unknown
tool names, invalid tool arguments, and invalid cursors return `-32602`; unexpected internal
conditions return a sanitized `-32603`; a handler result that cannot be encoded is one such `-32603`
for its request, never a session failure. Tool-domain failures use a successful `tools/call` response
with `isError: true` so the model can inspect the safe Corvint receipt.

- `MCPV0-005`: The only supported protocol version is exactly `2026-07-28`. Every request, including
`server/discover`, MUST include:

```json
{
  "_meta": {
    "io.modelcontextprotocol/protocolVersion": "2026-07-28",
    "io.modelcontextprotocol/clientCapabilities": {}
  }
}
```

The protocol-version value is at most 64 UTF-8 bytes. Missing, malformed, or over-limit fields return
`-32602` without reflecting their value. A different within-bound version returns `-32022` with
sanitized data exactly shaped as `{"requested":"VALUE","supported":["2026-07-28"]}`. Each
successful result has `resultType: "complete"` and `_meta.io.modelcontextprotocol/serverInfo` with
fixed build-provided `name` and `version`. Client identity is untrusted display metadata and MUST NOT
affect authorization, root selection, evidence, limits, or behavior.

- `MCPV0-006`: `server/discover` MUST return only the implemented surface:

```json
{
  "resultType": "complete",
  "supportedVersions": ["2026-07-28"],
  "capabilities": {
    "tools": {"listChanged": false}
  },
  "ttlMs": 0,
  "cacheScope": "public",
  "_meta": {
    "io.modelcontextprotocol/serverInfo": {
      "description": "Local read-only Corvint context and repository evidence server.",
      "name": "corvint-mcp",
      "version": "BUILD_VERSION"
    }
  }
}
```

The server MUST omit instructions, prompts, resources, subscriptions, completions, sampling,
elicitation, tasks, experimental extensions, list-change notifications, and logging unless a later
accepted profile implements and tests them. It MUST return `-32601` for legacy `initialize`,
`ping`, `logging/setLevel`, `resources/subscribe`, and `resources/unsubscribe` requests; these are
not 2026-07-28 lifecycle operations. That answer precedes the per-request metadata check of
`MCPV0-005`, because a legacy client never sends that metadata. A `notifications/initialized` notification is unsupported and
ignored without a response, as required for notifications.

### Read-only Corvint tools

- `MCPV0-007`: `tools/list` is paginated even when one page currently contains all tools. Its closed
input is `{_meta, cursor?}`. The initial or absent cursor returns the complete stable lexicographic
tool list, `resultType: "complete"`, `ttlMs: 300000`, `cacheScope: "private"`, and no `nextCursor`.
Any non-empty cursor is invalid in V0 and returns `-32602`. Tool definitions MUST use JSON Schema
2020-12, set `additionalProperties: false`, and set annotations to
`readOnlyHint: true`, `destructiveHint: false`, `idempotentHint: true`, and
`openWorldHint: false`. Names are unique, case-sensitive, 1 through 128 characters, and use only
ASCII letters, digits, `_`, `-`, or `.`; the dot-separated names below follow the official grammar.

- `MCPV0-008`: V0 advertises exactly these tools, subject to the implementation schemas frozen below:

| Tool | Closed arguments | Result and authority |
|---|---|---|
| `corvint.query` | required ASCII `task` string | Corvint authority-start query receipt at its fixed native limit of one; unsupported intent is an explicit abstention, never a fabricated general query |
| `corvint.impact` | required non-empty `paths` array of repository-relative Go paths; optional bounded integer `limit` | Corvint impact receipt with proven, candidate, excluded, and unknown states preserved |
| `corvint.status` | no operation fields | Local server/repository/revision/worktree/capability status; no source body and no claim that the index is fresh unless observed |

Each `tools/call` has closed `{_meta, name, arguments}` parameters; `inputResponses` and
`requestState` are unsupported. The tool's `CallToolResult` has one text content block containing
the canonical compact receipt JSON wrapped in the same untrusted-data envelope and the same named
free-text field list the harness adapters apply to repository-authored free text
(`BEGIN CORVINT REPOSITORY DATA` / `END CORVINT REPOSITORY DATA`), built by `internal/repoenvelope`
under `AHI-004`: hidden characters become literal `\uXXXX` text, and a receipt carrying the envelope
terminator is refused as a tool error with code `corvint-envelope-terminator-collision` rather than
emitted. This is because repository-authored `title`, `summary`, and `evidence[].reason` fields reach
a model through this text block exactly as they do through the harness hook path. `structuredContent` carries the
identical parsed object unwrapped, for programmatic callers that do not read it as model input. The
closed bridge object is:

```text
schema: "corvint-mcp-bridge-result/0"
tool: "corvint.query" | "corvint.impact" | "corvint.status"
mutates: false
state: "READY" | "ABSTAINED"
epistemicClass: "OBSERVED" | "NOT_OBSERVED"
authorityClass: "REPOSITORY_EVIDENCE" | "GIT_REPOSITORY" | "NONE"
repository: null | {
  commitRevision, treeRevision, objectFormat, profileId,
  worktreeState: "CLEAN" | "MIXED", dirtyPathCount, dirtyPathsSha256
}
receipt: null | <unaltered native Corvint query/impact receipt>
abstention: {active: boolean, reason: string}
```

All digest and revision fields are lowercase hexadecimal with their native fixed widths; reason is
a closed sanitized reason code, not tool or repository text. `corvint.status` has a null `receipt` and
derives its state from the repository envelope. The bridge MUST NOT flatten away Corvint's receipt
profile, revision, worktree state, authority, inclusion reason, freshness, exclusions, gaps, or
abstention. A bounded evidentiary gap is `state: "ABSTAINED"`; invalid tool input remains `-32602`.
A safe operational tool failure instead sets `isError: true` and duplicates this closed object
across text and structured content:

```json
{
  "abstention": {"active": true, "reason": "OPERATION_FAILED"},
  "code": "SANITIZED_CODE",
  "mutates": false,
  "profile": "corvint-mcp-tool-error/0",
  "tool": "corvint.query"
}
```

`SANITIZED_CODE` is a closed implementation code such as `repository-unavailable`,
`repository-state-unstable`, or `internal-error`, never underlying Git, repository, or process text.
An unexpected protocol-server failure remains `-32603`. Results omit source and diff bodies but
MUST NOT omit a qualifying uncertainty field merely to fit the frame. If a complete safe receipt
cannot fit, the call returns an explicit bounded over-budget failure rather than truncated content.
One abstention deliberately retains a receipt: a native impact state of `OUT_OF_SCOPE` maps to
wrapper `ABSTAINED`/`NOT_OBSERVED`/`NONE` with reason `OUT_OF_SCOPE` while preserving the native
impact receipt so its exclusion evidence is not lost. Unsupported and output-budget abstentions have
a null receipt. The canonical bridge receipt is capped at 393,216 bytes and is also checked in its
duplicated text-plus-structured `tools/call` frame with an 8,192-byte transport reserve; either bound
can produce `OUTPUT_BUDGET_EXCEEDED`.

- `MCPV0-009`: Query `task` contains 1 through 2,000 printable ASCII characters and has no `limit`
argument; the native Go authority-start profile fixes its limit to one. Impact accepts 1 through
100 unique normalized repository-relative `.go` paths; a path is at most 1,024 Unicode scalar
values, uses `/`, contains no empty, `.` or `..` segment, is not absolute, and cannot escape the
bound root. `limit`, when present, is an integer from 1 through 50 and defaults to 10. Arguments are
validated before repository access.

- `MCPV0-010`: `corvint.status` reports only the exact repository binding already defined by the bridge
object: commit revision, tree revision, object format, repository profile, worktree state, and
bounded dirty-path count/digest. Server build identity already belongs to discovery result server
info. General Corvint index freshness, dashboard/evidence availability, and root text/digest are
`NOT_OBSERVED` in V0 and are documented omissions rather than invented status fields. Status does
not enumerate source paths, environment values, process arguments, credentials, or repository
contents.

### Cancellation, progress, logging, pagination, and roots

- `MCPV0-011`: The dispatcher MUST continue reading frames while a tool request runs. A valid
`notifications/cancelled` for an in-flight request cancels that request's context and its owned
descendant process group and produces no response to the notification. If cancellation wins the
serialized output race, the server emits no further response, result, or notification for the
original request; if a complete response already won, cancellation is a no-op. Unknown, completed,
and duplicate request IDs are no-ops. Cancellation never targets another request or a process the
server did not create. Request cancellation decides only whether a response or notification frame
starts; once a frame's bytes may have reached stdout, only connection teardown interrupts it, so a
cancelled request never leaves a torn line for the next frame to join. Clean EOF, and EOF after an
unterminated frame's single parse error, stops admission and drains already admitted responses; signal or transport failure cancels all work. EOF, SIGINT, and
SIGTERM wait for descendants and exit without leaving a Corvint or Git child alive; so does a transport
failure, including a write to a closed stdout, which exits with status 2 and never by `SIGPIPE`. Before `Serve`
returns, a stdio file descriptor whose `O_NONBLOCK` flag the transport changed has its original flag
restored, because inherited stdio shares its open file description with the parent process.

- `MCPV0-012`: If a request carries a `progressToken`, the server MAY emit
`notifications/progress` only for that request, with the exact opaque token, finite non-decreasing
`progress`, a finite `total` only when mechanically known, and sanitized fixed messages. It MUST NOT
invent progress or include repository/tool output. Absence of a token means no progress notification.
V0 does not require progress for operations too short to expose a truthful intermediate state.

- `MCPV0-013`: V0 does not advertise logging and emits no `notifications/message`. The 2026-07-28
per-request `io.modelcontextprotocol/logLevel` value may be syntactically validated but has no effect.
`logging/setLevel` is method-not-found. Fixed startup, invalid-argv, repository-unavailable, and
fatal transport diagnostics may go to stderr; they remain sanitized, contain no request or
repository data, and never become telemetry.

- `MCPV0-014`: V0 pagination is deterministic, stable within the process, and never used to conceal a
truncated receipt. List cursors are opaque bounded tokens tied to method, server build, and the exact
ordered registry; wrong-method, malformed, stale, or unknown cursors return `-32602`. Since the V0
tool registry fits one page, no cursor is currently issued. Tool results themselves are not paged.

- `MCPV0-015`: Repository authority comes only from `--root`. The server does not request
`roots/list`, does not infer authority from client-advertised roots, and does not advertise a server
roots capability (none exists). A client `roots` capability is ignored. Supporting client-provided
roots later requires an accepted multi-root authority profile; V0 never switches repositories in
response to request data.

### Trust boundary and failure policy

- `MCPV0-016`: Repository files, Git output, Corvint receipts, MCP metadata, request IDs, paths, task
text, cancellation reasons, and tool errors are untrusted. They MUST NOT become shell, format,
terminal-control, log, URI, or error-message syntax. V0 uses in-process Go packages where available.
Any necessary child command uses a pinned absolute executable and direct argv without a shell or a
second PATH lookup; bounded stdout/stderr are captured separately and never forwarded verbatim.

- `MCPV0-017`: All calls are read-only. The server MUST NOT write the repository, Git index, Corvint
trace, CEM/OCM, dashboard state, environment, configuration, or install location. It MUST NOT run
tests, update, install, commit, merge, fetch, contact a model, make a network request, start a
database, collect telemetry, or expose prompt templates, source bodies, secrets, write tools, or
approval bypasses.

- `MCPV0-018`: Per-call repository and revision identity is acquired once and bound to the Corvint
operation and result. If reciprocal Git identity/status probes disagree, the operation abstains with
a drift gap. A read may observe a mixed worktree only when the underlying Corvint receipt says so;
mutable bytes do not become revision-pinned evidence. `NOT_OBSERVED`, `UNKNOWN`, exclusions, and
unsupported states cross the MCP boundary unchanged in meaning.

- `MCPV0-019`: Frame allocation, tool arguments, Corvint output, stderr, concurrent requests, progress
state, and descendants are bounded. The fixed concurrency ceiling is 64 admitted requests; excess
calls receive one sanitized `-32603` without starting repository work. A slow or cancelled client
cannot make stdout writes grow without bound.
An output that would exceed 1,048,576 bytes is replaced by one bounded explicit over-budget result,
not truncated JSON.

- `MCPV0-020`: `corvint.query` and `corvint.impact` MAY accept an explicit
  `snapshot` object following AFP-V0-019's closed receipt and immutable bindings.
  Missing snapshot preserves the existing live profile; explicit null or malformed
  wire MUST fail with `invalid-arguments`, and stale/mismatched/incomplete bindings
  MUST fail with sanitized `repository-unavailable`, never retry without snapshot.
  Evidence MUST come from the exact committed tree and carry AFP-V0-019's diagnostic
  `snapshot` scope. The outer repository binding MUST still disclose observed dirty
  state; immutable receipt freshness MUST NOT claim that checkout bytes are clean.
  Query authority retains its existing task/limit admission and ranking, but omits
  mutable traces and history with explicit uncertainty. Impact keeps existing
  path/profile limits and exclusions. HEAD/tree MUST match again before publishing.
  No snapshot can certify tests, runtime behavior, current uncommitted source or
  host acceptance. The existing frame, argument and read-only bounds remain.
  Rollback removes optional snapshot admission without weakening legacy refusals.
  Evidence: `TestMCPExplicitSnapshotRetainsImmutableEvidenceInMixedWorktree` and
  `TestMCPSnapshotRejectsSymlinkAndGitlinkTrees` and AFP-V0-019's receipt rejection
  tests. External host qualification is NOT_RUN.

### Explicit 2025-11-25 compatibility

The owner-approved OpenCode repair adds a closed, opt-in transport profile. Requirements
`MCPV0-001..020` continue to govern the default modern profile; this section overrides only the
legacy selection, lifecycle and transport metadata described below. Shared bounds, read-only
receipts, cancellation, process cleanup, and all remaining promotion gates apply to both profiles.
The [official 2025-11-25 lifecycle](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle)
is the negotiation reference; the additional closed admission rules below are local profile bounds.

- `MCPV0-021`: All four existing MCP commands MUST accept at most one
  `--protocol-version VERSION` selector, where VERSION is exactly `2026-07-28` or `2025-11-25`.
  Omission selects the unchanged modern profile. Missing, duplicate, unknown selectors and a
  selector combined with `--version` MUST fail before repository startup. Existing required root
  admission remains; `corvint-corpus-mcp` still requires `--artifact` and remains experimental and
  outside the shipped companion set. Selection is explicit, never inferred from incoming frames.
- `MCPV0-022`: A legacy connection MUST initialize independently for each Serve invocation. It
  accepts one bounded `initialize` request with a nonempty protocolVersion of at most 64 UTF-8
  bytes, object capabilities and clientInfo with string name/version. Optional implementation
  metadata is data only; roots declarations cannot select roots or authorize callbacks. A valid
  different version offer receives the sole supported legacy version `2025-11-25`; this does not
  implement that offered version. Success returns only protocolVersion, tools capabilities,
  serverInfo and optional instructions. Admission MUST be ordered with input: initialization
  becomes visible only after its bounded success response is fully written, then an absent/empty
  params `notifications/initialized` advances to ready. Early, repeated and malformed initialized
  notifications are ignored without response. Duplicate initialization returns `-32600` without
  resetting the connection. Tools before ready return `-32600`; failed, cancelled or unencodable
  initialization never admits tools. Ping accepts absent/empty params before and after readiness.
  Legacy requests accept absent params and optional `_meta.progressToken`; malformed metadata and
  modern `io.modelcontextprotocol/` transport keys return `-32602`. `server/discover` and all other
  unimplemented methods return `-32601`. Existing strict JSON, duplicate-key, depth, frame, output,
  concurrency, ID reuse, progress, cancellation, EOF and termination bounds remain unchanged.
- `MCPV0-023`: Legacy calls MUST reuse each existing command's closed tool registry and native
  handlers. Only top-level modern resultType/cacheScope/ttlMs transport fields are removed; modern
  server-info decoration is skipped. Structured application receipts and enveloped text retain
  their evidence, authority, freshness, refusal and uncertainty semantics. Each handler receives
  independent copies of session declarations. Tests MUST exercise representative tool calls in
  every selected command, lifecycle refusal, cancellation and real descendant cleanup. A real
  OpenCode connection/list is discovery evidence only; actual host tools/call evidence is recorded
  separately and MUST NOT be inferred from direct protocol tests or promoted into formal FULL
  host authority. Official conformance and release gates remain independent.

An isolated OpenCode `mcp list` probe reproduced `initialize` method-not-found on the original
binary, then connected and discovered tools with the explicit profile. CLI version was 1.18.31;
initial captured clientInfo reported 1.17.18, while the later successful client reported 1.18.31.
These are separate observations, not normalized identities. This local synthetic transport fixture
is not a sealed workflow benchmark. A subsequent real OpenCode run executed `corvint.status` and returned the unchanged READY,
read-only repository receipt through the host tool lifecycle. Its provider was a deterministic
local fixture, with native network policy limiting outbound access to localhost; it does not
qualify model reasoning, complete cost, or the daily workflow. Persistent host configuration and
exact release-artifact qualification remain separate from these development probes.

Example local OpenCode server entry (absolute paths supplied by the operator):

```json
{
  "mcp": {
    "corvint": {
      "type": "local",
      "command": ["/absolute/path/corvint-mcp", "--root", "/absolute/repository", "--protocol-version", "2025-11-25"],
      "enabled": true
    }
  }
}
```

Rollback removes the selector to restore modern default behavior, or disables the server. No
listener, SDK dependency, proxy, daemon, new tool, repository mutation or authority is introduced.

### Bridge failure codes

The in-process bridge (`internal/mcp/bridge`) emits the kebab-case codes below (decision 0100).
Each row cites the first emitting site and states only the condition checked there.

| Code | First emitting site | At the cited site |
|---|---|---|
| `invalid-registry` | `internal/mcp/bridge/bridge.go:216` | `Registry.Call` is reached on a nil registry, or on one with an empty root, a nil root or Git identity, or a nil build, build-query, or probe operation; checked before cancellation and argument validation |
| `unsupported-tool` | `internal/mcp/bridge/bridge.go:235` | the tool name is not `ToolQuery`, `ToolImpact`, or `ToolStatus` |

## Acceptance matrix

The P0 profile is implemented only when all applicable rows pass on Go 1.27.0:

| Area | Required evidence |
|---|---|
| Schema and discovery | Official 2026-07-28 positive vectors; missing metadata; `-32022`; exact capabilities; every result has `resultType` and server info |
| Framing and JSON-RPC | LF/CRLF, EOF, invalid UTF-8/JSON, duplicate keys at every depth, depth 64/65, exactly-at/over 1 MiB, batches, IDs, notifications, error sanitization, recovery after a bad frame |
| Tools | Exact closed schemas and annotations; query/impact/status success, abstention, mixed worktree, unsupported query, invalid paths/limits, result budget, and no source bodies |
| Cancellation/processes | Cancel-before-start, during work, after completion, duplicate/unknown ID, EOF/SIGINT/SIGTERM, direct argv, pinned executable, and zero surviving descendants |
| Pagination/progress/logging/roots | deterministic tool list, bad cursor, emitted progress with exact token correlation on a truthful long-running operation, no unsolicited logs, no roots switch, and method-not-found for removed legacy methods |
| Security | planted secrets/control characters/path escapes/symlink swaps/output floods; repository and trace byte snapshots before/after; no listener or network attempt |
| Portability | `go test -race`, `go vet`, fuzzing of frame/depth/duplicate-key/schema/cursor decoders, and cross-builds for supported macOS/Linux architectures |
| External | official current black-box MCP conformance when it supports protocol `2026-07-28`; otherwise `NOT_RUN` with tool/version/reason retained |

Independent conformance MUST drive the compiled `corvint-mcp` process over stdio; in-package tests
alone do not close the wire contract. Promotion requires the exact official schema to be vendored or
fetched, verified to its pinned digest, and executed as test input, not merely named in a manifest or
reinterpreted from an older protocol release. The schema is fetched by the operator and executed by
the opt-in `TestServerTrafficMatchesOfficialSchema`; a default no-network run records it `NOT_RUN`.

## Non-goals and simpler baseline

V0 does not provide Streamable HTTP, a listener, authorization, remote repositories, multiple roots,
prompts, resources, source content, subscriptions, sampling, elicitation, tasks, frontier/why/live
features, standalone evidence, dashboard snapshots, lifecycle hooks, automatic context injection,
test execution, test-level results or their `LPCV-V0-047` projection (decision 0103), edits,
CEM/OCM writes, updates, installs, merges, telemetry, or a native host adapter. The simpler baseline is direct `corvint query` and `corvint impact`; MCP exists only to
remove repeated transport glue without changing evidence semantics.

## Rollout, rollback, and compatibility

1. Ship the binary as an experimental local stdio surface with protocol version pinned to
   `2026-07-28` and compatibility status `FALLBACK`.
2. Publish only black-box-tested client tuples. An MCP smoke test does not promote a host adapter.
3. Add an operation only after its native Corvint receipt, closed schema, authority, limits, and
   adversarial conformance vectors are stable. Capability discovery changes in the same release.
4. Streamable HTTP is a separate security and authorization profile; no HTTP code or dormant
   listener ships in P0.

Rollback removes or disables `corvint-mcp`; it has no repository, trace, database, or network state to
migrate. Protocol and conformance paths retain their applicable plain Apache-2.0 grant under
`LICENSING.md`; the rest of Corvint retains its repository license.

## Traceability

| Requirements | Implementation | Evidence |
|---|---|---|
| `MCPV0-001..006` | `cmd/corvint-mcp`, `internal/mcp/protocol`, `internal/mcp/server` | protocol/discovery/framing/version vectors; `TestDuplicateInFlightIDOmitsID`, `TestIDReusableOnceResponseIsRead`, `TestUnencodableResultIsRequestError`, and `TestLegacyLifecycleMethodsWithoutMetadataAreNotFound` observed; official-schema digest pinned; opt-in `TestServerTrafficMatchesOfficialSchema` passed on 2026-09-23 |
| `MCPV0-007..010` | `internal/mcp/bridge` | tool registry, closed-schema, receipt, budget, and abstention tests |
| `MCPV0-011..015` | `internal/mcp/server` | cancellation race and post-cancel health observed; `TestCancelledProgressNeverTearsFrame`, `TestUnterminatedEOFFlushesAdmittedResponse`, and Unix `TestServeRestoresInheritedDescriptorBlockingMode` observed; Unix-gated compiled-process INT/TERM fake-Git parent-and-child cleanup passed locally; Unix `TestClosedStdoutCancelsInFlightDescendantGroup` (closed stdout mid-call exits 2 and reaps the fake-Git group) observed; truthful emitted progress and non-Unix signal cleanup `NOT_OBSERVED`; pagination/logging/roots negative tests |
| `MCPV0-016..019` | all MCP implementation paths | local secret/mutation and Unix descendant-cleanup checks observed; start-time Git pinning observed by Unix `TestGitPlantedOnPathAfterStartNeverRuns` and `TestPinFixesExecutableAgainstLaterPathChanges`; fuzz, complete race, non-Unix cleanup, and cross-build evidence remain separate gates |
| `MCPV0-021..023` | all four MCP commands; `internal/mcp/protocol/legacy.go`; `internal/mcp/server/legacy.go` | `TestMCPV0021LegacyFlagAcceptsCapturedInitialize`, `TestMCPV0021ProtocolSelector`, `TestMCPV0022LegacyAdmissionAndReceipt`, `TestMCPV0022LegacyFailedInitializeCannotAdmitTools`, `TestMCPV0022LegacyMetadata`, `TestMCPV0022LegacyCancellationAndProgress`, `TestMCPV0022LegacyCancelledAfterCompletionIsIgnored`, `TestMCPV0022LegacyBlockedInitializeCancels`, both profiles of `TestServeToolsListAndCallRoundTripsDocsDraftAndConsume`, `TestCorpusMCPTransport`, `TestVectorsAndReadOnly` and `TestTerminationSignalsCancelInFlightDescendantGroup`; actual OpenCode discovery and status-call development probes; exact release-artifact qualification separate |
| all | `conformance/mcp-2026-07-28` | independent compiled-process vectors and retained `NOT_RUN` external result |

Current evidence: local Go 1.27.0 unit and compiled-process black-box suites `PASS` on 2026-08-23;
that pass covers strict framing, discovery, tool schemas/calls, a cancellation race with subsequent
health, logging/roots negatives, revision binding, mutation, sanitization, and a Unix-gated
compiled-process fixture in which INT and TERM each terminated an in-flight fake-Git process and its
background child with no surviving descendant. It does not prove a notification-cancellation-won
long-running call, an emitted progress notification, or equivalent cleanup on non-Unix platforms;
those are `NOT_OBSERVED`. Official MCP conformance is `NOT_RUN` for the retained reason above.
On 2026-09-23 the opt-in official-schema run passed against the pinned digest and the start-time Git
pin was observed (V1-0191). Fuzz, complete race, and cross-build promotion evidence are not closed by
that focused run.

## Unresolved decisions and promotion/kill criteria

There are no unresolved P0 wire decisions. Exact Git executable pinning was an implementation
promotion blocker; V1-0191 closed it by pinning at `corvint-mcp` start, and a build without that
pin is nonconformant and MUST remain experimental. A future resource or dashboard bridge needs its own
stable source-content-free resource identity and privacy review; a future HTTP profile needs explicit
authentication, origin, DNS-rebinding, session, CORS, and listener exposure decisions.

One source/secret leak, repository mutation, unauthorized network/listener, surviving descendant,
duplicate-key acceptance, path escape, false revision binding, swallowed `UNKNOWN`/`NOT_OBSERVED`,
or advertised-but-unimplemented capability blocks release. An official conformance failure remains
visible. If MCP requires a second evidence model or materially changes Corvint receipts, retain the
direct CLI baseline and remove this server.
