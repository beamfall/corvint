# MCP Server 2026-07-28 V0

Owner: Russell Lewis  
Date: 2026-08-23  
Intent status: accepted (companion profile; owner's 1.0 feature-cut delegation, 2026-10-20; V1-0191)  
Delivery status: implemented (stdio companion profile, official-schema receipt 2026-10-20)  
Authoritative inputs: `docs/PRODUCT.md`, `docs/DOGFOOD.md`,
`docs/specs/agent-harness-integration-v0.md`, the
[MCP 2026-07-28 specification](https://modelcontextprotocol.io/specification/2026-07-28), and its
official JSON Schema pinned at commit
[`271ecc9accafdd9b83a3c869fa67c22953b2af80`](https://github.com/modelcontextprotocol/modelcontextprotocol/blob/271ecc9accafdd9b83a3c869fa67c22953b2af80/schema/2026-07-28/schema.json)
(sha256 `ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203`, matching the
`officialSchema.observedSha256` already recorded in `conformance/mcp-2026-07-28/cases.json`)

## Agent digest
- Claim: A local stdio MCP server exposes bounded read-only Corvint receipts, with 2026-07-28 default and explicit 2025-11-25 compatibility.
- Status: accepted (companion profile; owner's 1.0 feature-cut delegation, 2026-10-20; V1-0191)/implemented (stdio companion profile, official-schema receipt 2026-10-20)
- Exists: `cmd/corvint-mcp` (entry point), `internal/mcp/cli`, `internal/mcp`, and independent compiled-process conformance vectors.
- Blocked on: nothing for the stdio companion profile; the official-schema receipt passed at `a7870cac3` on 2026-10-20 and `mcp-server-unavailable` is retired (see Companion promotion evidence). The official Streamable HTTP conformance runner has no stdio target and stays `NOT_RUN`; official-schema execution stays opt-in. The default V0 tool list is four: the three original tools plus `corvint.help`, which `MCPV0-100` adds to every profile and which amends `MCPV0-008` and decision 0103's three-tool rule (2026-10-23, gh#1588); `corvint.context` and `corvint.cem.report` are advertised only by the opt-in task-review descendant profile (`MCPV0-024..026`, decision 0374). The five read-only `corvint.flows.*` tools are advertised only under `--tool-profile flows` (`MCPV0-026` as amended by `AFU-V1-034` and `AFU-V1-049`). Tool errors stay `corvint-mcp-tool-error/0` unless the opt-in reason-class profile selects `/1` (`MCPV0-027..028`, decision 0383).
- Read next: User and measurable job; Explicit 2025-11-25 compatibility; Task-review descendant profile; Flows tool profile; Reason-class tool-error profile; Follow-ref worktree binding (`MCPV0-039..041`, opt-in `--follow-ref`); Traceability.

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

- `MCPV0-035`: (accepted, decision 0512) Cancellation of an active
  `corvint.flows.impact` MUST stop its impact graph walk under AFU-V1-059 and return the
  existing `cancelled` failure without a ready receipt. A regression MUST cancel after the
  actual language walk begins and prove the call returns, under the race detector.

- `MCPV0-001`: The executable MUST run as `corvint-mcp --root ABSOLUTE_ROOT`. It owns exactly one local
child process and one repository root. At startup, `ABSOLUTE_ROOT` MUST be absolute, clean, bounded,
free of control characters, and resolved through existing symlinks once to a canonical absolute
root. The supplied root is at most 4,096 UTF-8 bytes. Its `.git` marker MUST be a non-symlink
 directory or a bounded regular gitfile with reciprocal linked-worktree metadata. The owner repair
 scope of 2026-09-28 (#334) admits linked roots through the existing CEM worktree validator.
 The root, marker, per-worktree directory and common directory identities remain pinned;
 pointer retargeting or directory replacement before or during a call MUST abstain.
Requests cannot replace the root. The canonical root MUST resolve to the Git worktree observed by
Corvint before a tool can return repository evidence. A path escape, repository identity drift, or an
unavailable Git snapshot MUST fail closed for the Corvint operation. Clean and mixed worktrees are
both reportable states; neither may be silently converted to the other. This single-root form is
unchanged by the accepted (decision 0441) descendant profile `mcp-multi-root-v0.md` (`MMR-V0`), which alone defines
operator-declared `--root ALIAS=ABSOLUTE_ROOT` roots.

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
accepted profile implements and tests them. It MUST return `-32601` for legacy `initialize` (since
`MCPV0-820` only on a server without `NameLegacyFlag`), `ping`, `logging/setLevel`, `resources/subscribe`, and `resources/unsubscribe` requests; these are
not 2026-07-28 lifecycle operations. A server that sets the opt-in `NameLegacyFlag` instead answers
a legacy `initialize` with `-32602`, naming its version and `--protocol-version 2025-11-25`
(`DCP-V1-652`); `corvint-corpus-mcp` sets it, and since `MCPV0-820` so do `corvint-mcp`,
`corvint mcp`, `corvint-docs-mcp` and `corvint-test-validity-mcp`. That answer precedes the per-request metadata check of
`MCPV0-005`, because a legacy client never sends that metadata. A `notifications/initialized` notification is unsupported and
ignored without a response, as required for notifications.

### Read-only Corvint tools

- `MCPV0-007`: `tools/list` is paginated even when one page currently contains all tools. Its closed
input is `{_meta, cursor?}`. The initial or absent cursor returns the complete stable lexicographic
tool list, `resultType: "complete"`, `ttlMs: 300000`, `cacheScope: "private"`, and no `nextCursor`.
Any non-empty cursor is invalid in V0 and returns `-32602`. Tool definitions MUST use JSON Schema
2020-12, set `additionalProperties: false`, and set annotations to
`readOnlyHint: true`, `destructiveHint: false`, `idempotentHint: true`, and
`openWorldHint: false` (amended by `MCPV0-114`: under `--follow-ref` a ref-bound tool sets
`readOnlyHint: false`). Names are unique, case-sensitive, 1 through 128 characters, and use only
ASCII letters, digits, `_`, `-`, or `.`; the dot-separated names below follow the official grammar.

- `MCPV0-008`: V0 advertises exactly these four tools, subject to the implementation schemas frozen
below. Amendment (2026-10-23, `MCPV0-100`, gh#1588): this requirement said "exactly these three
tools" until the owner, through the orchestrator, directed that `corvint.help` ship on every profile
before rc.3. That supersedes decision 0103's rule that a new MCP tool needs a descendant profile
identity, for this one binary-only, repository-free tool only; every other new tool still needs a
descendant profile. Decision 0103 freezes the rest of this default list; the opt-in task-review descendant profile
(`MCPV0-026`) adds the two tools of `MCPV0-024` and `MCPV0-025` only when selected, and the flows
profile (`AFU-V1-034`, `AFU-V1-049`) adds the five `corvint.flows.*` tools instead:

| Tool | Closed arguments | Result and authority |
|---|---|---|
| `corvint.query` | required ASCII `task` string | Corvint authority-start query receipt at its fixed native limit of one; unsupported intent is an explicit abstention, never a fabricated general query |
| `corvint.impact` | required non-empty `paths` array of repository-relative Go paths; optional bounded integer `limit`; optional integer `budgetBytes` (`MCPV0-110`) | Corvint impact receipt with proven, candidate, excluded, and unknown states preserved |
| `corvint.status` | no operation fields | Local server/repository/revision/worktree/capability status; no source body and no claim that the index is fresh unless observed |
| `corvint.help` | optional `topic`: exactly `catalog` (default) or `guide` | `corvint-mcp-help/0` receipt whose `text` is byte-for-byte `corvint help --json` or `corvint help guide` from the running binary (`MCPV0-100`); no repository read and a null `repository` |

Each `tools/call` has closed `{_meta, name, arguments}` parameters; `inputResponses` and
`requestState` are unsupported. The tool's `CallToolResult` has one text content block containing
the canonical compact receipt JSON wrapped in the same untrusted-data envelope and the same named
free-text field list the harness adapters apply to repository-authored free text
(`BEGIN CORVINT REPOSITORY DATA` / `END CORVINT REPOSITORY DATA`), built by `internal/repoenvelope`
under `AHI-004`: hidden characters become literal `\uXXXX` text, and a receipt carrying the envelope
terminator is refused as a tool error with code `corvint-envelope-terminator-collision` rather than
emitted. This is because repository-authored `title`, `summary`, and `evidence[].reason` fields reach
a model through this text block exactly as they do through the harness hook path. `structuredContent` carries the
identical parsed object unwrapped, for programmatic callers that do not read it as model input; when
the receipt carries a `results` list, the text block instead carries the `MCPV0-033` summary of that
object. The closed bridge object is:

```text
schema: "corvint-mcp-bridge-result/0"
tool: "corvint.query" | "corvint.impact" | "corvint.status" | "corvint.help"
      (task-review profile only: | "corvint.context" | "corvint.cem.report")
      (flows profile only: | "corvint.flows.map" | "corvint.flows.gaps"
                           | "corvint.flows.impact" | "corvint.flows.navigate")
mutates: false
state: "READY" | "ABSTAINED"
epistemicClass: "OBSERVED" | "NOT_OBSERVED"
authorityClass: "REPOSITORY_EVIDENCE" | "GIT_REPOSITORY" | "NONE"
                | "CORVINT_BINARY" (corvint.help only)
repository: null | {
  commitRevision, treeRevision, objectFormat, profileId,
  worktreeState: "CLEAN" | "MIXED"
              | "NOT_OBSERVED" (corvint.graph.imports only; dirtyPathCount 0),
  dirtyPathCount, dirtyPathsSha256
}
receipt: null | <unaltered native Corvint query/impact receipt, context packet, or CEM report preview>
         | <corvint.help: {schema: "corvint-mcp-help/0", topic, command, byteCount, sha256, text}>
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
Only the opt-in `MCPV0-027` selector replaces this object with `corvint-mcp-tool-error/1`, which adds
a closed `reasonClass` (`MCPV0-028`). An unexpected protocol-server failure remains `-32603`. Results omit source and diff bodies but
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
response to request data. The accepted (decision 0441) `mcp-multi-root-v0.md` profile (`MMR-V0-005`) keeps this
rule: its roots are operator-declared only, and client roots or request data never select one.

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
- `MCPV0-820`: (experimental under the standing lane auto-accept; V1-2507, V1-2513; amends
  `MCPV0-006` for `initialize` only) Started without `--protocol-version`, `corvint-mcp`,
  `corvint mcp`, `corvint-docs-mcp` and `corvint-test-validity-mcp` MUST set the shared server's
  `NameLegacyFlag`, so a 2025-11-25 `initialize` gets the `DCP-V1-652` answer: `-32602`, a message
  naming the version the server speaks and `--protocol-version 2025-11-25`, and `data`
  `{supported, flag, requested?}`, instead of a bare `-32601 Method not found`. The server keeps
  serving after the answer. `ping`, `logging/setLevel` and the resource subscription methods keep
  the `MCPV0-006` `-32601`. Selection stays explicit (`MCPV0-021`): the server never switches to
  the legacy profile from an incoming frame. Non-goals: negotiating 2025-11-25 by default, a
  change to the explicit legacy profile or to any tool. Failure mode: a client that branches on
  `-32601` for `initialize` now sees `-32602`, the code a 2025-11-25 client expects for an
  unsupported version. Falsifier: a default-profile `initialize` on any of the four commands that
  answers `-32601` or omits the flag. Evidence: `TestMCPV0820DefaultProfileNamesLegacyFlag`
  (`internal/mcp/cli`), `TestMCPV0820DocsAndTestValidityNameLegacyFlag` (`cmd/corvint-docs-mcp`,
  `cmd/corvint-test-validity-mcp`). Rollback: drop `NameLegacyFlag` from the three server
  configurations.

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

### Task-review descendant profile (`corvint-mcp-2026-07-28-conformance/1`)

Decision 0374 adds this opt-in descendant profile under decision 0103's rule that new MCP tools need
a descendant profile identity rather than a wider V0 list. Without the selector, `corvint-mcp` is
exactly the V0 profile of `MCPV0-008` (three tools until the `MCPV0-100` amendment added
`corvint.help`, four since), and conformance profile `/0` keeps its meaning.
With the selector, the two tools below are added under the unchanged `MCPV0-007` descriptor rules,
`MCPV0-008` envelope, terminator-collision refusal, bridge object and 393,216-byte budget, and
`MCPV0-016..019` read-only, secret and process rules. The profile's black-box vectors are
`conformance/mcp-2026-07-28/cases-task-review.json`.

- `MCPV0-026`: `corvint-mcp` MUST accept at most one `--tool-profile` selector, following the
  closed-argv rule of `MCPV0-021`, as amended by `AFU-V1-034`. The value is exactly `task-review`, `task-review-lsp` or
  `flows`; the additive `NGI-V0-305` amendment (V1-3090, 2026-10-28) also admits
  `import-graph`, which adds only `corvint.graph.imports` to the default tools and leaves
  every existing profile's tool list unchanged. A missing, empty, differently cased, unknown or duplicate value, two selectors even with
  different values, the `--tool-profile=VALUE` spelling, and the selector combined
  with `--version` MUST fail before repository startup with exit 2 and no stdout. Omission selects
  the default list (four tools since `MCPV0-100`), and the `AFU-V1-034` amendment leaves the default and task-review `tools/list`
  bytes unchanged. The selector composes with `--protocol-version 2025-11-25`: the
  legacy profile reuses whichever registry was selected (`MCPV0-023`). A `tools/call` naming a tool
  the selected profile does not advertise fails as `unsupported-tool`, which the server maps to
  `-32602` exactly like any unknown tool. Selection is explicit and never inferred from frames.
  The other three MCP commands do not accept the selector.

| Tool | Closed arguments | Result and authority |
|---|---|---|
| `corvint.context` | required non-blank `task` string; optional repository-relative `subject`; optional bounded integer `limit`; optional integer `summaryBytes` (`MCPV0-111`) | the `corvint context` task-context packet at the bound revision: files to read, each with its admitting relation, gaps and abstention preserved |
| `corvint.cem.report` | required repository-relative `map` path; required full `expectedBase` and `target` object IDs; optional non-negative `maxUnknown` and `maxMechanical` ceilings | the `corvint cem report` reviewer report and verification counts, rendered as a preview that publishes nothing |

- `MCPV0-024`: `corvint.context` MUST compile the same packet as `corvint context` in process, with no
  executable spawn beyond the pinned Git runner: it reads an existing context snapshot or builds an
  observed index in memory, reads admitted slot weights, and never writes a snapshot, trace, ledger or
  weight file. The optional gopls attachment is not run. `task` is 1 through 32,000 UTF-8 bytes and
  not blank after Go whitespace trimming; the schema advertises 8,000 code points (bytes divided by
  `utf8.UTFMax`) and a pattern requiring one character outside ECMA `\s` and U+0085, so every
  schema-valid task is runtime-valid. `subject`, when present, follows the `MCPV0-009` path rule
  without the `.go` suffix; `limit` is an integer from 1 through 50 and defaults to 20. The task is
  not echoed into the result. An untracked subject is `ABSTAINED` with reason
  `NOT_TRACKED_AT_REVISION`, revision drift during the build is `REPOSITORY_STATE_UNSTABLE`, and an
  unqualified platform is `UNSUPPORTED_PLATFORM`; the repository binding is the snapshot the packet was
  compiled from.
- `MCPV0-025`: `corvint.cem.report` MUST render the `corvint cem report` Markdown and the stable
  verification for one `cem/1.0` map (`MCPV0-036`; the `cem/0.2` and `cem/0.3` reading first named
  here is retired by `CEM-V1-210`), as a preview that never
  publishes `.git/corvint/cem-review.md` or any other file. The `map` argument is a closed
  repository-relative path: at most 512 bytes (128 advertised code points), no control byte, not
  absolute, no backslash, no empty, `.` or `..` segment, and no `.git` segment in any case. A
  symlinked map or symlinked ancestor, and a missing map, fail as `cem-map-unavailable` without
  echoing the path. The reason is that the map is repository-authored input: following a link would
  let a tracked file make the server read outside, or inside Git metadata of, the root it was bound
  to at start, and the result would then carry bytes the operator never scoped. `expectedBase` and
  `target` are full lowercase 40- or 64-hex object IDs; a symbolic revision is `-32602`. A retired
  `cem/0.1`, `cem/0.2` or `cem/0.3` map fails as `cem-map-invalid` before any other judgment
  (`CEM-V1-210`); a malformed map is `cem-map-invalid`; a `maxUnknown` ceiling on a `cem/1.0` map is
  `cem-map-stable-refused` (`MCPV0-037`); a CEM Git read that ran out of its own bound
  (`GitTimeout`, `GitDiffTimeout`) on a live request is `repository-probe-timeout` (`MCPV0-038`), a
  CEM Git read that exhausted its operation budget or output bound (`GitBudgetExceeded`,
  `GitOutputExceeded`) is `corpus-capacity-exceeded`, the capacity code `DCP-V1-158` names, because
  the repository is present and only a bound was reached (amended 2026-10-20, V1-1992), and
  every other Git read failure is `repository-unavailable` (amended 2026-10-16, decision 0533,
  V1-1432). The repository probe brackets the
  read, and a changed probe is `REPOSITORY_STATE_UNSTABLE`. Report Markdown carries repository-authored
  hunk paths, so it travels only inside the `MCPV0-008` envelope, and a report containing the
  terminator is refused with `corvint-envelope-terminator-collision`. The CEM Git runner is pinned to
  the same start-time executable as the other kernels.
- `MCPV0-036`: (V1-0632 S2) `corvint.cem.report` MUST accept one optional `artifacts`
  argument under the same closed repository-relative path rule as `map`, resolved under the root
  exactly as the caller named it at start and passed to the CEM read as its artifact root. A
  `cem/1.0` map is admitted against that caller-named root, never the symlink-resolved root the
  retired 0.x reads used, byte for byte, so a symlinked `--root` is refused as `cem verify-stable --repository` refuses it. The server accepts only an absolute, clean `--root`; every other spelling is refused at start as `invalid-root`. A retired 0.x
  map fails as `cem-map-invalid` whether or not `artifacts` is given (`CEM-V1-210`). A `cem/1.0` map yields a READY receipt whose
  `stableVerification` is the stable verifier's complete result for the same map, revisions and
  artifact root, unchanged, with the `cem/1.0` review Markdown and `recordSetSha256`; it never carries
  or synthesizes the 0.x `verification`, `counts`, `policyIssues` or `worklist`, and it still publishes
  nothing.
- `MCPV0-037`: (V1-0632 S2) A `cem/1.0` map refused before its stable verifier runs (a
  policy ceiling, `--patch`, or another 0.x-only input) MUST fail as `cem-map-stable-refused`, not
  `cem-map-invalid`. An unknown spec such as `cem/9` and a retired 0.x map fail as `cem-map-invalid`
  (`CEM-V1-210`).

### Flows tool profile (amendment by `AFU-V1-034`, `AFU-V1-035`)

`--tool-profile flows` advertises the V0 tools of `MCPV0-008` (including `corvint.help`) plus `corvint.flows.map`, `corvint.flows.gaps`,
`corvint.flows.impact` and `corvint.flows.navigate`, with the `MCPV0-007` read-only annotations. The
requirements, bounds and refusals are the `AFU-V1` ones in
`docs/specs/application-flow-understanding-v1.md`; this section records only the MCP surface. Each
tool calls the same `internal/appflows` library as its `corvint flows` verb, reads the intents at
`HEAD` between two repository probes (it abstains with `REPOSITORY_STATE_UNSTABLE` if `HEAD`, the tree
or the dirty-path set differs between them), and returns the verb's JSON document as the bridge `receipt`, whose `revision` MUST equal the bound commit. Every
tool takes a required repository-relative `flows` directory. `map` and `gaps` take optional
`evidence` files; `map` also takes one of `path` or `testKey`, which excludes `evidence`; `impact`
takes a required full object-ID `base`; `navigate` takes optional `evidence` and `traffic` files and an
optional `goal` of at most 64 characters, with `maxEffect` only beside a goal. Input paths are
repository-relative, at most 100 per array, and never under `.git`; each component of an input file's
parent directory is checked to be a real directory, not a symlink, before the file is opened, and a
swap after that check is not detected. Only one `corvint.flows.impact` call runs at a time. Invalid arguments are `-32602`; a refusal by the flows verb is the closed code
`flows-refused`, which never carries the verb's message. Flow intents carry repository-authored text,
so a flows result omits `structuredContent` and returns the bridge object only as the `MCPV0-008`
enveloped text (`AFU-V1-035`). No conformance manifest or black-box vectors exist for this profile
yet.

### Reason-class tool-error profile (`corvint-mcp-2026-07-28-conformance/2`)

Decision 0383 lets an agent without a terminal learn why a repository was refused without opening
the closed tool-error object to Git, repository or process text. Without the selector, every tool
error is exactly the `corvint-mcp-tool-error/0` object above and conformance profile `/0` keeps its
meaning. The profile's black-box vectors are `conformance/mcp-2026-07-28/cases-reason-class.json`.

- `MCPV0-027`: `corvint-mcp` MUST accept at most one `--error-profile reason-class` selector,
  following the closed-argv rules of `MCPV0-021` and `MCPV0-026`. The value is exactly
  `reason-class`; a missing, differently cased, unknown or duplicate value, the
  `--error-profile=VALUE` spelling, and the selector combined with `--version` MUST fail before
  repository startup with exit 2 and no stdout. The selector composes with `--tool-profile` and
  `--protocol-version 2025-11-25`. With it, every tool error MUST be the `/0` object with `profile`
  `corvint-mcp-tool-error/1` and one added required `reasonClass`; `code` and every other member are
  unchanged. The other three MCP commands do not accept the selector.
- `MCPV0-028`: `reasonClass` MUST be one of the closed values `git-filter`, `config-include`,
  `attributes-file`, `ref-storage`, `worktree-config`, `config-malformed`, `submodule`,
  `split-index`, `gitdir-pointer`, `metadata-unreadable`, `metadata-limit`, `metadata-directory`,
  `metadata-drift`, `scratch-dir`, `root-unresolved` or `unclassified`. A value MUST come only from a
  typed class attached where `internal/gitstatus` builds the refusal, or from identity with that
  package's metadata-drift sentinel (`metadata-drift`), never from parsing message text. Any error without that class, including every non-status failure, MUST be `unclassified`,
  and a client MUST read an unknown value as `unclassified`. No free text and no filter driver name
  crosses into the object. Adding a value needs an amendment to decision 0383.

### Bridge failure codes

The in-process bridge (`internal/mcp/bridge`) emits the kebab-case codes below (decision 0100).
Each row cites the first emitting site and states only the condition checked there.

- `MCPV0-038`: A tool call whose Go repository probe exceeded its own bound while the request
  context was still live MUST fail with `code` `repository-probe-timeout`, not
  `repository-unavailable`: the repository may be present and healthy, and only the bounded probe
  ran out of time. The value comes only from the typed gokernel code, never from message text. A
  probe stopped by the caller's cancellation or deadline stays `cancelled`, and every other
  repository refusal keeps `repository-unavailable`. The same code covers a CEM report's Git read
  that exceeded its own bound (`cemcode.GitTimeout`, `cemcode.GitDiffTimeout`) and a context
  index refusal whose typed cause is its own Git deadline; caller cancellation is checked first
  and still yields `cancelled` (amended 2026-10-16, decision 0533, V1-1432 sites 2 and 2b). Under `corvint-mcp-tool-error/1` the error
  carries `reasonClass` `unclassified`, because a probe overrun has no `MCPV0-028` status class, so
  decision 0383's closed set is unchanged. (Added 2026-10-15 for V1-1427 under the owner approval
  "approve V1-1427, V1-1428 and V1-1429 amendments", which accepts this wire change.)

| Code | First emitting site | At the cited site |
|---|---|---|
| `invalid-registry` | `internal/mcp/bridge/bridge.go:449@b6e5e0f0` | `Registry.Call` is reached on a nil registry, or on one with an empty root, a nil root or Git identity, or a nil build, build-query, probe, context, or CEM report operation; checked before cancellation and argument validation |
| `unsupported-tool` | `internal/mcp/bridge/bridge.go:458@6e5d7ee2` | the tool name is not advertised by the selected profile: `ToolHelp`, `ToolQuery`, `ToolImpact`, or `ToolStatus`, plus `ToolContext` and `ToolCEMReport` under `task-review`, the `ToolFlows*` tools under `flows`, or `ToolGraphImports` under `import-graph` (`NGI-V0-305`, `MCPV0-026`) |
| `cem-map-unavailable` | `internal/mcp/bridge/bridge.go:787@f5156052` | the CEM read reports the map missing, unreadable, or reached through a symlink |
| `cem-map-unsupported` | `internal/mcp/bridge/bridge.go:787@f5156052` | an invalid-arguments refusal that is not a `cem/1.0` stable refusal; since `CEM-V1-210` no retired 0.x map reaches it (a 0.x map is `cem-map-invalid`), so the report preview has no observed route to it |
| `cem-map-stable-refused` | `internal/mcp/bridge/bridge.go:809@5462a4a2` | a `cem/1.0` map was refused before its stable verifier ran (`MCPV0-037`) |
| `cem-map-invalid` | `internal/mcp/bridge/bridge.go:788@b2fd0644` | the map fails CEM strict decoding or field validation |
| `repository-probe-timeout` | `internal/mcp/bridge/bridge.go:1187@8c7cfeb6` | the failure carries gokernel's `repository-probe-timeout` code: the repository probe, a CEM Git read or the context index's Git read outlasted its own bound while the request context was live (`MCPV0-038`, decision 0533) |
| `corpus-capacity-exceeded` | `internal/mcp/bridge/bridge.go:795@c8f9bb58` | a CEM Git read exhausted its operation budget or output bound (`MCPV0-025`, V1-1992) |
| `flows-refused` | `internal/mcp/bridge/flows.go:267@0ffc3b6a` | a flows tool's intent load or verb returned an error other than cancellation (`AFU-V1-034`) |

### Explicit Go LSP descendant (V1-0476)

- `MCPV0-029`: The opt-in `--tool-profile task-review-lsp` descendant exposes the task-review
  tools and adds only the optional `lsp` string enum (`off`, `gopls`, default `off`) to
  `corvint.context`. All existing profile descriptors and behavior stay unchanged and reject
  any presence of this field, even null or off. The descendant also rejects null, unknown or
  non-string values. Environment variables never enable MCP enrichment. Startup authorizes and
  pins the absolute gopls PATH match (or retains absence); each call separately requests it.
  Callers cannot supply executables, argv or roots. This is the narrowly scoped exception to
  MCPV0-024's Git-only execution rule, not an expansion of the default profile.
- `MCPV0-030`: An explicitly requested gopls call uses TCP-V0-052's shared evidence and drift
  bracket, the EEP-V0-024..027 process bounds, the pinned/bounded Git runner and existing output
  budget and untrusted envelope. Root replacement, cancellation or repository drift cannot
  emit evidence bound to the old snapshot. Provider absence/failure remains a visible unavailable
  row. The installed server and operator Go toolchain configuration are trusted local execution;
  they are not sandboxed. Private provider-cache and ordinary Go build-cache writes are disclosed,
  while repository/index/trace writes, automatic install, downloads and persistent servers remain
  prohibited. Gopls telemetry is disabled in the child environment. Required qualification is
  TCP-V0-053, plus real cancellation/descendant cleanup and executable-replacement regressions.

- `MCPV0-031`: (V1-0944; no GitHub issue) Each impact result carries at most four
  `evidence` rows and at most four `references`. A result that had more carries
  `evidence_omitted` or `references_omitted` with the count of rows removed, so no cut is silent;
  a result under both caps carries neither key. The cap applies inside the impact receipt itself,
  so CLI impact, MCP `corvint.impact`, and every consumer that embeds the impact packet (for
  example `FPK-V0-010`) see the same rows. Builder-level `MAX_EVIDENCE` truncation before this
  cap is unchanged. Amendment (2026-10-24, `MCPV0-112`, V1-1003): it is now counted separately as
  `evidence_truncated`; `evidence_omitted` still counts only this cap's cut.
- `MCPV0-032`: (V1-0944; no GitHub issue) Path impact accepts `--budget-bytes N`
  (inline or as the next argument, once, within the `query` packet bounds) and compiles the
  receipt under that byte budget with the same `BUDGETED` / `CRITICAL_EVIDENCE_OVERFLOW` states
  and omission disclosures as `query`. Range (`--base`) and `--working-tree-untracked` impact
  keep refusing it as `unsupported-impact-option`, and `--provider` with `--budget-bytes` is an
  argument error because the external section is not budgeted. `prove` refuses `--budget-bytes`
  because it embeds the unbudgeted impact packet (`FPK-V0-010`). This amends the
  `GPK-V0` "only `--limit`" impact-option clause for path impact only.
- `MCPV0-033`: (V1-0944; no GitHub issue) When a successful tool result carries
  `structuredContent` and its receipt has a `results` list, the text block carries the framed
  canonical summary profile `corvint-mcp-text-summary/0`: the full bridge object with each
  `receipt.results` row projected to its `kind`, `id` and `score` keys and a top-level
  `textProfile` naming the profile. `structuredContent` keeps the full object. The terminator
  refusal of `MCPV0-008` is still decided on the full object. Results without a `results` list,
  tool errors and flows results keep the full framed text. This deliberately departs from the MCP
  SHOULD that a structured result also returns its serialized JSON as text; a text-only client
  sees the ranked rows but not their evidence. A client that validates the text block MUST also
  admit the full framed object, which servers predating this requirement emit.
- `MCPV0-034`: (accepted 2026-10-08 (decision 0470; V1-0485; no GitHub issue)) `Registry.Call` checks the request context
  again after the tool operation returns and before the root-identity check: a call whose context
  ended at any point before that check returns the `cancelled` failure and no result, never a
  READY receipt or an abstention built before the cancellation was observed. `invalid-arguments`
  and an operation's own `cancelled` keep their precedence. An uncancelled call keeps its result,
  abstention and evidence binding byte for byte. This closes the in-process bridge path (used
  directly by the LSP context adapter); `MCPV0-011`'s server-side output race is unchanged, and a
  cancellation observed only after the check is a completed call under that rule. The compile
  side of `corvint.context` stops at its own stage boundaries (`TCP-V0-064`).

### Follow-ref worktree binding (V1-1603)

- `MCPV0-039`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1603)
  `corvint-mcp --root ROOT --follow-ref REF` MUST bind the server to the commit REF names on a
  dedicated worktree: ROOT is one plain root (never with an aliased root or `--version`), a linked
  worktree (`.git` is a file) whose HEAD is detached, and REF (at most 256 bytes, no leading `-`,
  whitespace or NUL; given once) resolves to a commit at startup. Otherwise startup exits 2 with
  `corvint-mcp: follow-ref-not-dedicated`, `corvint-mcp: follow-ref-missing` or
  `corvint-mcp: invalid arguments` (amended by `MMR-V0-011`: an argument refusal on this path
  names its reason after a colon; amended by `MMR-V0-010`: with aliased roots,
  `--follow-ref ALIAS=REF` binds the named root alone). The binding is pinned to the tool registry's canonical root
  and its root, `.git` marker and Git directory identities; every Git command runs in that root
  with the Core sanitized Git environment, so an inherited `GIT_DIR`, `GIT_WORK_TREE`,
  `GIT_INDEX_FILE` or object override cannot redirect it. Each `tools/call` holds one lock across
  the move and the answer, so a concurrent call cannot move the worktree under an answer. Under that
  lock the server resolves REF and, on every call, checks the root identity (`ROOT_CHANGED`), a
  detached HEAD and a clean tree; when the worktree HEAD differs it runs
  `git checkout --detach --quiet --no-overwrite-ignore COMMIT` (hooks disabled, 2-minute bound), so
  an ignored local file is never overwritten, and then refreshes the index snapshot in process as
  `corvint index --if-stale` does, so the answer that follows describes the moved commit (`state`
  `REFRESHED`). An index refresh that fails yields `INDEX_FAILED`; a
  checkout that fails yields `CHECKOUT_FAILED`. Without `--follow-ref` nothing changes. This is an
  explicit operator-selected writer of the dedicated worktree and its index snapshot only, like
  `index`; the default server stays read-only.
- `MCPV0-040`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1603) A
  worktree with any tracked change or untracked file (`DIRTY`), or with a branch checked out
  (`NOT_DETACHED`), MUST never be modified: the answer describes the current head and the
  follow-ref block sets `stale` true. The checks run even when HEAD already equals the commit, and
  such a worktree's index snapshot is not written.
- `MCPV0-041`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1603) Every
  `tools/call` response under `--follow-ref`, result or tool error, MUST append one server-authored
  text block `corvint-mcp-follow-ref/0` (canonical JSON, outside the repository-data envelope) with
  `ref`, the resolved `commit`, the worktree `head`, `state` (`CURRENT`, `REFRESHED`, `DIRTY`,
  `NOT_DETACHED`, `ROOT_CHANGED`, `CHECKOUT_FAILED`, `INDEX_FAILED` or `MISSING`), `stale` and `mutates`. A REF that
  no longer resolves MUST answer with the closed tool error `code` `follow-ref-missing` (under
  `corvint-mcp-tool-error/1`, `reasonClass` `unclassified`), with `state` `MISSING`, an empty
  `commit` and the worktree unchanged. Failure modes: a ref moved between the block's resolution and a
  later Git read inside the tool is answered by the tool's own revision binding; a slow index
  build delays the answer within the request's own bounds. Falsifier: an answer after a ref move
  that still describes the old commit on a clean worktree; a dirty worktree modified; a response
  without the block. Exception (2026-10-23, `MCPV0-100`): a `corvint.help` call never binds the
  ref and carries no block, because it reads only the binary. Rollback: remove `--follow-ref` and
  `internal/mcp/cli/follow_ref.go`; the default server and its bytes never changed.
- `MCPV0-042`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1999) A
  `tools/call` result that cannot fit one MCP message on any server built on
  `internal/mcp/server` (both the `2026-07-28` and the legacy `2025-11-25` path) MUST be answered
  as a successful `tools/call` response with `isError: true` whose one text block is
  `output-bound-exceeded: tool result of <bytes> bytes exceeds the <bound>-byte MCP message bound`,
  never a generic `-32603`. The text carries only the measured size and the bound, no tool output.
  A tool that bounds its own result first (the corpus bridge, `DCP-V1-200..202`) is unchanged; any
  other oversized or unencodable result stays the sanitized `-32603` of `MCPV0-004`. Falsifier: an
  over-bound tool result answered `-32603`, or a refusal frame that itself exceeds the bound.
  Rollback: restore the `-32603` branch in `writeResultContext`.
- `MCPV0-090`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1004) A
  `corvint.context` call MUST release the Index it read before it returns, so a long-lived bridge
  never pins a mapped pack or sectioned snapshot after the retention ring evicts it
  (`IDX-SNAP-V0-028`). Before the release, the packet is copied onto the heap through its canonical
  bytes, with numbers kept as their literals, and the copy MUST encode to the same bytes; a copy
  that differs refuses with `internal-error` rather than change the wire result. The bridge
  binding strings are cloned too. A deferred read that is refused and retried through the eager
  loader releases the first Index. Falsifier: a live mapping after every context call returned and
  the ring was evicted; a result whose bytes change after the mapping is unmapped; a fault under
  concurrent context calls. Rollback: remove the deferred `Release` and `detachedContext` from
  `callContext`; the mapping is then pinned until process exit, as before.
- `MCPV0-100`: (owner directive through the orchestrator, 2026-10-23; gh#1588; amends `MCPV0-008`
  and decision 0103) Every `corvint-mcp` profile (default, `task-review`, `task-review-lsp` and
  `flows`, under either protocol version) MUST advertise one more read-only tool, `corvint.help`,
  under the `MCPV0-007` descriptor rules. Its closed arguments are `{}` or `{"topic": "catalog" |
  "guide"}`; omission means `catalog` (amended by `MCPV0-953`: omission means `guide`, and
  `command` names one catalog entry). Any other member, a `null`, non-string or other topic value,
  or non-object arguments are `invalid-arguments`. The result is the closed bridge object with
  `state: "READY"`, `epistemicClass: "OBSERVED"`, `authorityClass: "CORVINT_BINARY"`, a null
  `repository`, and the receipt `{schema: "corvint-mcp-help/0", topic, command, byteCount, sha256,
  text}`, where `text` is exactly the bytes `corvint help --json` (`catalog`, the embedded
  `internal/capcatalog` snapshot that `CAP-V0-019` holds equal to that stdout) or `corvint help guide`
  (`guide`, `CAP-V0-016`) print from the same build, `command` names that CLI form, and `byteCount`
  and `sha256` describe `text`. Both texts come from `internal/capcatalog` in the binary: the call
  reads no repository, index, Git or trace state, needs no index, and is deterministic for a build.
  Under `--follow-ref` the call skips ref binding entirely: no `corvint-mcp-follow-ref/0` block, no
  checkout or reindex, and the same answer when the ref is current, moved or deleted. The result
  validator admits only the closed six-member receipt whose `topic` is known, whose `command` is
  that topic's CLI form, whose `text` is that topic's embedded text, whose `byteCount` is an
  integer equal to its length and whose `sha256` is the embedded digest; any other shape is an
  internal error rather than a forged receipt.
  The result stays under the 393,216-byte bridge budget and the framed `tools/call` check; if a
  future catalog does not fit, the call returns `ABSTAINED` with `OUTPUT_BUDGET_EXCEEDED` and a null
  receipt rather than truncated text. Cancellation is checked first, so a cancelled call fails
  `cancelled` even when its text would exceed the budget. The text block carries the receipt inside the `MCPV0-008`
  envelope like every other tool, so a text that contained the terminator would be refused as
  `corvint-envelope-terminator-collision`. A call cancelled before it returns fails `cancelled`. In
  the multi-root profile the tool takes the required `repository` alias like every tool except
  `corvint.status`; it still reads nothing from that repository. Measured at delivery: the catalog
  text is 105,424 bytes and the guide 5,804 bytes; after the review fixes added `requiredFlags`
  and `nestedHelpTopics` they are 116,793 and 6,085 bytes. Non-goals: no per-command filter or search
  argument, no other MCP server (`corvint-docs-mcp`, `corvint-corpus-mcp`,
  `corvint-test-validity-mcp`) gains the tool (amended 2026-10-23 by `MCPV0-101..105`, which add
  it to those three servers), no MCP resource or prompt, no change to the
  existing tools' bytes, and no wider exception to decision 0103 for any other tool. Failure modes:
  a catalog that drifts from the CLI (caught by the parity tests below and `make capability-catalog-check`);
  a catalog that outgrows the budget (bounded abstention); a client that hard-codes three tools
  (the `tools/list` goldens and conformance lists changed in the same change, and adding a tool is
  additive for MCP clients). Falsifier: a profile without the tool, a receipt whose text differs
  from the CLI's stdout by one byte, a repository read during the call, or an accepted argument
  outside the closed set. Rollback: remove the `corvint.help` descriptor, `callHelp` and its
  validator, restore the three-tool goldens and conformance lists, and restore this amendment's
  "three tools" wording in `MCPV0-008`; no stored state depends on the tool.
- `MCPV0-101`: (owner request, 2026-10-23; extends `MCPV0-100`; amends `SDD-V0-006`)
  `corvint-docs-mcp` MUST advertise `corvint.help` with exactly the `MCPV0-100` descriptor,
  beside `corvint.docs_consume` and `corvint.docs_draft`, so its `tools/list` holds three tools,
  under either protocol version. A call MUST answer as `MCPV0-104` describes. Falsifier: a
  `tools/list` without the tool, or a descriptor or result that differs from the shared one.
- `MCPV0-102`: (owner request, 2026-10-23; extends `MCPV0-100`) `corvint-corpus-mcp` MUST
  advertise `corvint.help` with exactly the `MCPV0-100` descriptor in every argv configuration,
  beside the tools the artifact's capabilities and the `--map`, `--consolidation` and
  `--legacy-cases` selectors list, under either protocol version. A call MUST answer as
  `MCPV0-104` describes. It is not a corpus read, so the `DCP-V1-145` call deadline and the
  `corvint/timing` result `_meta` do not apply to it; an `--outcome-log` records it like any other
  listed tool. Falsifier: a configuration without the tool, or a help result carrying corpus
  timing or trust-envelope members.
- `MCPV0-103`: (owner request, 2026-10-23; extends `MCPV0-100`; amends `MTV-V0-001`)
  `corvint-test-validity-mcp` MUST advertise `corvint.help` with exactly the `MCPV0-100`
  descriptor beside `corvint.test_validity`, so its `tools/list` holds two tools, under either
  protocol version. A call MUST answer as `MCPV0-104` describes. The help tool presents no
  test-level result, so `LPCV-V0-047` places nothing on it. Falsifier: a `tools/list` without the
  tool, or a descriptor or result that differs from the shared one.
- `MCPV0-104`: (owner request, 2026-10-23) The three servers of `MCPV0-101..103` MUST answer
  `corvint.help` through one shared path, `bridge.HelpToolResult`, which applies
  `Registry.Call`'s argument bounds (at most 4 KiB of valid UTF-8) and the `MCPV0-100` closed
  arguments, receipt, result validator, 393,216-byte budget and cancellation order. It MUST return
  the same `CallToolResult` that `corvint-mcp` emits for that call: the bridge object as
  `structuredContent`, and its canonical JSON inside the `MCPV0-008` envelope as the one text
  block. The transport's `_meta` and `resultType` are the same as on every other result. Invalid
  arguments are JSON-RPC `-32602`, and a cancelled call is a JSON-RPC internal error, as each server
  maps its own tools' failures. The call reads no repository, index, Git, corpus artifact,
  application map or trace state, and skips each server's repository identity check, so it answers
  the same after the root is removed. Each server still needs its usual `--root` (and
  `--artifact`) to start. Falsifier: a wire result that differs from `bridge.HelpToolResult`'s,
  a copied descriptor or receipt builder, or a help answer that depends on repository state.
- `MCPV0-105`: (owner request, 2026-10-23) The receipt `text` that `MCPV0-104` returns MUST be,
  byte for byte, the stdout of `corvint help --json` (topic `catalog` or none) or
  `corvint help guide` (topic `guide`) from the same build. Falsifier: one differing byte.
  `MCPV0-106` to `MCPV0-109` are reserved for this slice and unassigned.
- `MCPV0-110`: (proposed; owner decision 2 of 2026-10-24, V1-1003; extends `MCPV0-032`)
  `corvint.impact` MUST accept an optional integer `budgetBytes` with the CLI path-impact
  `--budget-bytes` semantics: given, the receipt is the one `contextindex.EvalImpactSyntax`
  compiles for the same index, paths, limit and budget, with the same `request.budget_bytes`,
  `BUDGETED` / `CRITICAL_EVIDENCE_OVERFLOW` states and omission disclosures. Its range is the
  `query` packet bounds (`MinPacketBytes` 1216 to `MaxPacketBytes` 1000000, the CLI's
  "budget_bytes must be between" refusal); a value outside it, explicit `null`, a fraction or a
  non-integer is `invalid-arguments` before any repository read. Omitted, the unbudgeted receipt
  is unchanged byte for byte. The CLI-only refusals of `MCPV0-032` (`--base`,
  `--working-tree-untracked`, `--provider`) have no MCP argument to refuse. Failure modes: a budget
  that cannot hold the critical rows is the receipt's own `CRITICAL_EVIDENCE_OVERFLOW`, and a
  result above the bridge budget is still `OUTPUT_BUDGET_EXCEEDED`. Falsifier: a budgeted MCP
  receipt that differs from `EvalImpactSyntax`'s, or an omitted budget that changes the wire.
- `MCPV0-111`: (proposed; owner decision 2 of 2026-10-24, V1-1003; extends `MCPV0-024` with
  `ESV-V0-008`) `corvint.context` MUST accept an optional integer `summaryBytes` from 1024 to 65536
  with the `--summary-bytes` semantics: given, the result receipt is the
  `experimental-evidence-summary/0` projection (`internal/contextsummary.Summarize`, shared with
  `corvint context --summary`) of the canonical receipt plus LF, so `packet_sha256` and
  `packet_bytes` name the bytes the same call without `summaryBytes` returns. Every top-level
  member except `results` is kept, so the `MCPV0-024` binding (`tool`, `revision`) still holds.
  Only `continuation` differs from the CLI: it names the MCP call instead of the CLI rerun; `expand`
  stays the CLI command, because MCP has no expand tool. There is no separate context packet budget
  (the decision amends V1-1003 AC1 to this projection). Out-of-range, `null`, fractional or
  non-integer values are `invalid-arguments`. A packet whose critical rows do not fit, or that the
  summary cannot read, abstains as `SUMMARY_BUDGET` or `SUMMARY_SHAPE` with the binding. Falsifier:
  a summary that differs from `Summarize` over the unsummarized receipt plus LF, or a summary that
  drops a critical row.
- `MCPV0-113`: (experimental under the standing lane auto-accept; V1-2516; clarifies `MCPV0-001`
  and `MMR-V0-001`) `corvint-mcp --help` or `-h` as the only argument MUST print usage to stdout
  and exit 0 without starting a repository. A `--root` whose path is not absolute, plain or
  aliased, MUST still be refused before any repository work and MUST say so on stderr as
  `corvint-mcp: repository unavailable: --root must be an absolute path`, exit 2; it is never
  resolved against the launch directory. Non-goals: resolving relative roots; `--help` for the
  docs, corpus or test-validity servers. Failure mode: `--help` beside any other argument stays
  `invalid arguments`. Amended by `MCPV0-950`: a non-absolute plain single root (including an
  empty or unexpanded `${CLAUDE_PROJECT_DIR}` value) now starts the server unavailable with the
  same reason text instead of exiting 2; an aliased one still exits 2.
- `MCPV0-950`: (experimental under the standing lane auto-accept; V1-3700; amends `MCPV0-001`,
  `MCPV0-113` and `AHI-820`) At startup each declared `--root` MUST be resolved to the Git
  worktree toplevel it lies in: the root itself when it holds the `.git` marker, else its nearest
  physical ancestor that does (`git rev-parse --show-toplevel` without the environment), and that
  toplevel is then bound under every `MCPV0-001` check. When a root cannot be resolved, stderr MUST
  name one fixed reason (`--root must be an absolute path`, `--root must be a clean path of at
  most 4,096 bytes without control characters`, `--root does not exist`, `--root is not a
  directory`, `--root is inside a .git directory, not a worktree`, `--root is not inside a Git
  worktree`, or a refused toplevel `.git` marker) after `corvint-mcp: repository unavailable: `,
  never the path or a system error. A single plain root that neither follows a ref nor binds a
  corpus artifact then starts UNAVAILABLE instead of exiting: `initialize`, `server/discover` and
  `tools/list` answer as for a usable root with the same tool list (profile and groups), and
  `corvint.help` still answers from the binary (`MCPV0-100`), but every other listed tool's
  `tools/call` MUST return an `isError` result with code `invalid-root` (the group tools under
  their own tool-error profiles), so no repository evidence is ever returned; an unlisted tool
  stays invalid params. Aliased roots, `--follow-ref` and `--corpus-artifact` still exit 2 with the
  named reason. The release install lifecycle (`script/check-install-lifecycle.sh`, step
  `mcp-unavailable-root`) runs the handshake against a non-repository root. Non-goals: resolving
  relative roots; a root that becomes usable later (restart the server); partial multi-root
  service. Falsifier: a subdirectory root that exits, an unusable root that exits 2 or answers a
  repository call without `isError`, or a reason line that carries the path. Rollback: restore the
  exit-2 branch in `run` and drop `rootbind.Resolve` and `internal/mcp/cli/unavailable.go`.
- `MCPV0-951`: (experimental under the standing lane auto-accept; V1-3702; amends `MCPV0-001`
  and `AHI-820`) `corvint mcp` MUST report the core binary's own build identity as
  `serverInfo.version` and in `corvint mcp --version`: its `corvint --version` line without the
  leading `Corvint ` (`1.0.0-rc.3 (build N, commit SHA)`, `PUB-V0-053`). The standalone
  `corvint-mcp` and every in-process caller keep `0.1.0-experimental`, so the profile goldens and
  companion smoke are unchanged. A launcher MAY declare its plugin with `CORVINT_PLUGIN_VERSION`
  (`HOST/VERSION`) and `CORVINT_PLUGIN_TOOLS` (a comma list of 1 to 64 `corvint.*` tool names), read
  once at startup; the Claude Code plugin's `.mcp.json` declares both, with the version equal to
  its `plugin.json` and the tools equal to what its args list. With a declaration, every
  `corvint.status` result (success or `isError`) MUST carry `_meta` member
  `io.corvint/pluginCompatibility` (profile `corvint-plugin-compatibility/0`: `plugin`, `server`,
  `expectedTools`, sorted `missingTools`, `compatible` true exactly when every expected tool is
  listed) and one added text block naming the result; a declaration outside the bounds is
  `compatible:false`, `reason:"malformed-declaration"`, and is never partly used. The status
  receipt and `structuredContent` never change, and without a declaration the result bytes are
  unchanged. Non-goals: version ranges, refusing to serve a skewed plugin, a CLI flag (an older
  binary would refuse it and close the connection). Falsifier: a stamped `corvint mcp` that reports
  `0.1.0-experimental`, a missing expected tool reported compatible, or an undeclared launch whose
  status bytes change. Rollback: pass the empty version from `cmd/corvint/mcp.go`, drop
  `internal/mcp/cli/plugin.go` and the `.mcp.json` `env`.
- `MCPV0-952`: (experimental under the standing lane auto-accept; V1-3703) Every `-32602
  Invalid params` that `corvint-mcp`, `corvint-docs-mcp`, `corvint-test-validity-mcp` or
  `corvint-corpus-mcp` answers MUST carry `data` with a closed `reason` and the offending `field`
  (a dotted path such as `arguments.paths[1]` or `_meta.io.modelcontextprotocol/protocolVersion`),
  plus `allowed` (enum, const, tool or param names) or `rule` (expected type, `minimum N`-style
  bound, or the pattern) when the listed schema states them. Reasons: `unknown-param`,
  `missing-param`, `invalid-param`, `invalid-meta-key`, `invalid-cursor`, `missing-tool-name`,
  `unknown-tool`, `unknown-argument`, `missing-argument`, `invalid-type`, `invalid-enum`,
  `invalid-const`, `pattern-mismatch`, `out-of-range`, `duplicate-item`, and `invalid-argument`
  (a rule the advertised schema does not state, field `arguments`). A tools/call diagnosis reads
  only the request and the tools/list schemas after the existing decision, so it never changes
  which calls are refused, and the unified and dedicated servers answer the same data
  (`MMR-V0-013`). The message stays `Invalid params`; the server forwards only this closed data
  from a handler and drops a field longer than 1,024 bytes. Non-goals: data on other error codes,
  diagnosing every runtime-only rule. Falsifier: a -32602 without `data.reason` and `data.field`,
  or a diagnosis that differs between the unified and a dedicated server. Rollback: drop
  `toolgroup.WithInvalidParamsData` and `InvalidParamsAt`, restoring bare errors.
- `MCPV0-953`: (experimental under the standing lane auto-accept; V1-3701; amends `MCPV0-100`,
  `MCPV0-007` and `MCPV0-101..103`) `corvint.help`'s closed arguments become `{}`, `{"topic":
  "guide" | "catalog"}`, or `{"command": NAME}` with an optional `"topic": "command"`, where NAME is
  in the descriptor's enum of catalogued commands. Omission now means `guide`; `command` returns the
  bytes of `corvint help --json NAME` (`CAP-V0-950`, under 5 KB) with receipt `command` `corvint help
  --json NAME`; `catalog` stays the full catalog and must be asked for. A `null` member, a `command`
  beside `guide` or `catalog`, or `topic: "command"` without one is `invalid-arguments`. The default
  and a command answer each stay under 32 KB on the wire. Every tool these four servers list whose
  result carries `structuredContent` MUST declare `outputSchema` `{"type":"object","properties":
  {"tool":{"type":"string"},"mutates":{"const":false}}}`, which every bridge, group, corpus and
  tool-error object meets, so a client may read structuredContent instead of the text block that
  repeats it; the flows tools (`AFU-V1-035`) declare none. Non-goals: paging the full catalog,
  per-tool output schemas, dropping the text block (hosts that read only text still need it).
  Falsifier: a default help answer of 32 KB or more, a slice that differs from the CLI's, or a
  structuredContent tool without `outputSchema`. Rollback: restore the catalog default, drop the
  command topic and `bridge.WithOutputSchemas`, and repin the goldens.
- `MCPV0-954`: (experimental under the standing lane auto-accept; V1-3704; amends `MCPV0-820` and
  `MCPV0-007`) Under `NameLegacyFlag`, an `initialize` whose `protocolVersion` is the version the
  server speaks (`2026-07-28`) MUST be answered `-32602` with a message saying that version has no
  initialize handshake and to send `server/discover` with `_meta` on each request, and `data`
  `{requested, supported, method: "server/discover"}` without the restart flag; every other
  `initialize` keeps the `DCP-V1-652` answer, whose message now adds `(server/discover, no
  initialize)` after the version. Every advertised input schema MUST carry `$schema` only at its
  root, and every path `pattern` MUST be lookaround-free so ECMA and RE2 validators agree: each is
  generated by `bridge.PathRule` as segments joined by single slashes, accepting exactly what the
  lookahead pattern it replaced accepted (ECMA's `.` now spelled as excluding LF and CR). Non-goals:
  a `format` keyword, other keywords RE2 validators lack. Falsifier: a 2026-07-28 initialize told to
  restart with `--protocol-version 2025-11-25`, a nested `$schema`, a pattern Go's `regexp` cannot
  compile, or a path the old and new rules disagree on. Rollback: restore the literal patterns and
  `objectSchema` in `snapshotSchema`, and drop the modern branch of `legacyInitializeRefusal`.
- `MCPV0-114`: (experimental under the standing lane auto-accept; V1-3030; amends `MCPV0-039`)
  Under `--follow-ref`, `tools/list` MUST advertise every tool a call binds to the ref with
  `readOnlyHint` false and its description followed by one fixed sentence stating that each call
  first moves the dedicated worktree to the ref's commit (`git checkout --detach`) and reindexes
  it. `destructiveHint` stays false because the move never modifies a dirty or branch-bound
  worktree nor overwrites an ignored file (`MCPV0-040`), and `idempotentHint` stays true because a
  repeated call against an unchanged ref moves nothing. `corvint.help` never binds the ref
  (`MCPV0-100`) and keeps its read-only annotation and description. Without `--follow-ref`, every
  profile's `tools/list` bytes are unchanged (the four profile goldens). Falsifier: a ref-bound
  tool advertised `readOnlyHint` true under `--follow-ref`, or any change to the plain list.
  Rollback: drop `followRefTools` from the `tools/list` handler.
- `MCPV0-112`: (proposed; owner decision 2 of 2026-10-24, V1-1003; amends `MCPV0-031`) An impact
  result whose builder dropped rows at the per-result `maxEvidence` ceiling (10) MUST carry
  `evidence_truncated` with the count dropped: same-package reference lines, reference lines of a
  test-convention row, marker tests, document references, ledger records' ADR, marker and scenario
  rows, registry-dialect callers, Ruby autoload referrer lines, range path spans and range marker
  rows, and rows refused when two changed paths merge into one full result. `evidence_omitted`
  keeps counting only the `MCPV0-031` display cut, so the two never mix. When any emitted result
  carries the key, `coverage.uncertainty` MUST name the total in its own line, "N evidence rows in M
  results omitted by the per-result evidence ceiling of 10", which is not the packet budget's or the
  result limit's text, counted over the results the packet budget keeps. A result under the ceiling carries no key, so the wire of an untruncated
  receipt is unchanged. Failure mode: a count can only be an exact sum of dropped rows; a builder
  that cannot count reports none rather than a guess. Falsifier: a ceiling cut with no
  `evidence_truncated`, a count folded into `evidence_omitted`, or the disclosure missing.
  The `$ref` deduplication of `tools/list` schemas proposed in V1-1003 AC3 is rejected by the same
  decision; the four `tools/list` goldens grow by the two new properties only.
- `MCPV0-095`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2219) A `MIXED`
  bridge `repository` binding MUST stay exactly the seven members of the closed
  `corvint-mcp-bridge-result/0` object above: it names no dirty path, only `dirtyPathCount` and
  `dirtyPathsSha256`, unlike the bounded path list of `corvint affected` (`AFP-V0-356`, whose
  non-goals already exclude this binding). Why not: (1) the binding is the closed `/0` wire, and
  `internal/lspstdio` serialises the same `bridge.RepositoryBinding`, so a path member changes two
  wires and every closed-shape consumer; (2) it is an identity, not a diagnosis: the count and
  digest let a caller see that the dirty set changed between calls, and they are repeated in every
  result, including `ABSTAINED` ones and `corvint.status`, so names there would put repository-authored
  path text into model input on every call; (3) `corvint.status` and `corvint.cem.report` bind from
  `gokernel.ProbeRepositoryContext`, which keeps only the count and digest, so names would need a
  second worktree read that can disagree with the bound snapshot. A caller that needs the names
  reads them where they are diagnostic: the native receipt, which the bridge passes through
  unaltered, so any MIXED path naming a receipt carries reaches MCP callers unchanged; `corvint
  affected` (`AFP-V0-356`); or `git status`. Falsifier: a MIXED binding that carries a path, a
  path fragment or any member beyond the seven. Rollback: delete this requirement; a path member
  would then need its own wire revision.

## Acceptance matrix

The P0 profile is implemented only when all applicable rows pass on Go 1.27.0:

| Area | Required evidence |
|---|---|
| Schema and discovery | Official 2026-07-28 positive vectors; missing metadata; `-32022`; exact capabilities; every result has `resultType` and server info |
| Framing and JSON-RPC | LF/CRLF, EOF, invalid UTF-8/JSON, duplicate keys at every depth, depth 64/65, exactly-at/over 1 MiB, batches, IDs, notifications, error sanitization, recovery after a bad frame |
| Tools | Exact closed schemas and annotations; query/impact/status success, abstention, mixed worktree, unsupported query, invalid paths/limits, result budget, and no source bodies; context and CEM report success, `-32602` rejection, map escape and symlink refusal, no writes, terminator refusal, and result budget under the task-review selector; under the flows selector, the five flows tools advertised read-only; byte goldens of every profile's `tools/list` (V1-2021), receipts equal to the CLI verbs' documents, and repository text only inside the envelope; the default list is exactly four tools (`corvint.help` since `MCPV0-100`) and the selector is closed; `corvint.help` answers on every profile with the CLI's catalog and guide bytes |
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

V0 does not provide Streamable HTTP, a listener, authorization, remote repositories, multiple roots
(the separate proposed `mcp-multi-root-v0.md` profile),
prompts, resources, source content, subscriptions, sampling, elicitation, tasks, frontier/why/live
features, standalone evidence, dashboard snapshots, lifecycle hooks, automatic context injection,
test execution, test-level results or their `LPCV-V0-047` projection (decision 0103), edits,
CEM/OCM writes (the opt-in `MCPV0-025` report is a preview that publishes nothing), CEM bind or check, updates (the opt-in `--follow-ref` binding, `MCPV0-039`, moves only its dedicated detached worktree and index snapshot), installs, merges, telemetry, or a native host adapter. The reason-class profile does not change default `corvint-mcp --root ROOT` bytes, profile `corvint-mcp-tool-error/0` or the `/0` vectors, and does not carry reason text. The simpler baseline is direct `corvint query` and `corvint impact`; MCP exists only to
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
migrate. Rolling back the task-review profile (`MCPV0-024..026`, decision 0374) alone removes the
`--tool-profile` selector, the two tool descriptors and bridge cases, and the `/1` conformance
manifest and tests; the default tool output never changed with them, and no stored state depends on them. Rolling back `corvint.help` (`MCPV0-100`) is described in that requirement. Rolling back the reason-class profile (`MCPV0-027..028`, decision 0383) removes the
`--error-profile` selector, the `/1` object, the class plumbing and the `/2` manifest and tests; the
default tool-error bytes never changed. Rolling back the flows profile (`AFU-V1-034..035`) removes the
`flows` selector value, `internal/mcp/bridge/flows.go`, its bridge cases and tests; the default and
task-review tool lists never changed. Rolling back the post-operation cancellation check
(`MCPV0-034`) deletes the `ctx.Err()` check after the dispatch switch in `Registry.Call` and
`internal/mcp/bridge/context_cancel_test.go`; a late-cancelled call then returns its completed
result again, and no state depends on it. Protocol and conformance paths retain their applicable plain Apache-2.0 grant under
`LICENSING.md`; the rest of Corvint retains its repository license.

## Traceability

| Requirements | Implementation | Evidence |
|---|---|---|
| `MCPV0-001..006` | `internal/mcp/cli`, `internal/mcp/protocol`, `internal/mcp/server` | protocol/discovery/framing/version vectors; `TestDuplicateInFlightIDOmitsID`, `TestIDReusableOnceResponseIsRead`, `TestUnencodableResultIsRequestError`, and `TestLegacyLifecycleMethodsWithoutMetadataAreNotFound` observed; official-schema digest pinned; opt-in `TestServerTrafficMatchesOfficialSchema` passed on 2026-09-23 |
| `MCPV0-820` | `internal/mcp/cli`, `cmd/corvint-docs-mcp`, `cmd/corvint-test-validity-mcp` (`NameLegacyFlag`) | `TestMCPV0820DefaultProfileNamesLegacyFlag`, `TestMCPV0820DocsAndTestValidityNameLegacyFlag`; live Claude Code `claude mcp list` receipt in `docs/build-log/2026-10-31-linux-w2-bugs.md` |
| `MCPV0-007..010` | `internal/mcp/bridge` | tool registry, closed-schema, receipt, budget, and abstention tests |
| `MCPV0-011..015` | `internal/mcp/server` | cancellation race and post-cancel health observed; `TestCancelledProgressNeverTearsFrame`, `TestUnterminatedEOFFlushesAdmittedResponse`, and Unix `TestServeRestoresInheritedDescriptorBlockingMode` observed; Unix-gated compiled-process INT/TERM fake-Git parent-and-child cleanup passed locally; Unix `TestClosedStdoutCancelsInFlightDescendantGroup` (closed stdout mid-call exits 2 and reaps the fake-Git group) observed; truthful emitted progress and non-Unix signal cleanup `NOT_OBSERVED`; pagination/logging/roots negative tests |
| `MCPV0-016..019` | all MCP implementation paths | local secret/mutation and Unix descendant-cleanup checks observed; start-time Git pinning observed by Unix `TestGitPlantedOnPathAfterStartNeverRuns` and `TestPinFixesExecutableAgainstLaterPathChanges`; no promisor fetch (V1-0349) observed by `TestMCPReadsRefuseAMissingPromisorObjectWithoutFetching` (every read tool over a blob:none sparse clone, also through a Git that drops `GIT_NO_LAZY_FETCH`) `TestMCPFlowsCoverageRefusesAMissingPromisorObjectWithoutFetching` (`corvint.flows.coverage`, with a full blob:none clone as the serving control) and `TestMCPCEMReportRefusesAMissingCitedBlobWithAValidMapWithoutFetching` (V1-1039: `corvint.cem.report` with a present map and a missing cited base blob, with an unfiltered clone as the serving control); fuzz, complete race, non-Unix cleanup, and cross-build evidence remain separate gates |
| `MCPV0-021..023` | all four MCP commands; `internal/mcp/protocol/legacy.go`; `internal/mcp/server/legacy.go` | `TestMCPV0021LegacyFlagAcceptsCapturedInitialize`, `TestMCPV0021ProtocolSelector`, `TestMCPV0022LegacyAdmissionAndReceipt`, `TestMCPV0022LegacyFailedInitializeCannotAdmitTools`, `TestMCPV0022LegacyMetadata`, `TestMCPV0022LegacyCancellationAndProgress`, `TestMCPV0022LegacyCancelledAfterCompletionIsIgnored`, `TestMCPV0022LegacyBlockedInitializeCancels`, both profiles of `TestServeToolsListAndCallRoundTripsDocsDraftAndConsume`, `TestCorpusMCPTransport`, `TestVectorsAndReadOnly` and `TestTerminationSignalsCancelInFlightDescendantGroup`; actual OpenCode discovery and status-call development probes; exact release-artifact qualification separate |
| `MCPV0-024`, `MCPV0-025` | `internal/mcp/bridge` (`NewTaskReview`), `internal/cem/workflow` (`report-preview`), `internal/cem/gitrun` (`PinBinary`) | `TestContextToolReturnsBoundPacketWithoutWrites`, `TestCEMReportToolPreviewsCLIReportWithoutPublishing`, `TestCEMReportToolRefusesMapsOutsideTheRoot`, `TestNewToolsAbstainOverTheBridgeBudget`, `TestCEMReportToolAbstainsWhenTheCheckoutMoves`, `TestMCPCEMReportRefusesAMissingCitedBlobWithAValidMapWithoutFetching` (a missing cited blob behind a valid `cem/1.0` map is a READY receipt whose stable result is `UNSUPPORTED` `repository-object-unavailable`, with no fetch, V1-1039, `CEM-V1-210`); compiled-process `TestContextAndCEMReportAreBoundReadOnlyAndFramed`, `TestContextAndCEMReportRefuseInvalidArgumentsAndEscapes`, `TestCEMReportRefusesTerminatorAndAbstainsOverBudget`, `TestTaskReviewCEMReportNeverRunsPlantedGit`, `TestTaskReviewToolsRefuseExecutableConfigAndWorktreeRedirects` and `TestTaskReviewTrafficMatchesOfficialSchema`, all under the task-review selector (decision 0374); official-schema exchanges passed on 2026-09-23 |
| `MCPV0-026` | `internal/mcp/cli` (`extractToolProfile`), `internal/mcp/bridge` (`Registry.advertises`) | `TestMCPV0026ToolProfileSelectorIsClosed`, `TestToolsExposeOnlyDeliveredClosedReadSurface`; compiled-process profile `/1` `TestTaskReviewCaseInventoryIsClosed`, `TestTaskReviewSelectorIsClosed`, `TestTaskReviewDefaultProfileUnchanged`, `TestTaskReviewToolCatalogue`, `TestTaskReviewLegacyProtocol`; profile `/0` `TestToolCatalogueAndResourceOmission` lists exactly four tools since `MCPV0-100`; flows value (`AFU-V1-034`): `TestAFUV1034FlowsToolProfile` (golden default and task-review bytes, closed selector, read-only flows list) and `TestAFUV1034FlowsToolsMatchCLIVerbs` |
| `MCPV0-027`, `MCPV0-028` | `internal/mcp/cli` (`extractErrorProfile`, `toolFailure`), `internal/mcp/bridge` (`reasonClass`), `internal/gitstatus` (`RefusalClass`), `internal/gokernel`, `internal/contextindex` | `TestMCPV0027ErrorProfileSelectorIsClosed`, `TestMCPV0028ReasonClassToolError`, `TestEveryRefusalSiteCarriesAClosedClass`, `TestRefusalClassComesFromTypedRefusalOnly`, `TestStatusRefusesUnsupportedMetadataBeforeLiveStatus`; compiled-process profile `/2` `TestReasonClassCaseInventoryIsClosed`, `TestReasonClassSelectorIsClosed`, `TestReasonClassToolErrorOverRefusedRepositories`, `TestReasonClassUnclassifiedToolError` (decision 0383) |
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
On 2026-10-20 at `2b0cfce94` the compiled-process suite passed again (35 PASS, 0 FAIL) with the
three official-schema tests skipped for want of a local schema copy, and `internal/mcp/cli` and
`internal/mcp/bridge` passed. Later that day at `a7870cac3` the three official-schema tests passed
against the pinned digest (V1-0191; see Companion promotion evidence). Fuzz, complete race,
cross-build, a notification-cancellation-won call, an emitted progress notification and non-Unix
cleanup remain outside that receipt.

| `MCPV0-029` | `NewTaskReviewLSP`, `callContext`, `extractToolProfile` | `TestContextLSPProfileIsExplicit`, `TestContextLSPToolProfileSelector` |
| `MCPV0-030` | `lspevidence.Attach`, `lspprovider.rootCommit`, `callContext` | `TestContextLSPPinAndDrift`, `TestContextLSPCancellationRetiresDescendants`; `script/qualify-lsp.py` |
| `MCPV0-031` | `contextindex.capImpactRows`; VS Code `decodeCorvintReceipt` | `TestMCPV0031ImpactCapsRowsWithVisibleOmissions`; extension test `strict CLI decoder keeps capped impact omissions visible` |
| `MCPV0-032` | `cmd/corvint` `parseImpactArgumentsForPlatform`, `impactBudget`, `parseProveInvocation` | `TestMCPV0032PathImpactHonoursBudgetBytes`, `TestMCPV0032ProveRefusesBudgetBytes`; refusal cases in `TestImpactRejectsUnsupportedInputsWithoutReadingStdin` and `TestConvertedRefusalDiagnostics` |
| `MCPV0-033` | `bridge.Result.TextJSON`, `internal/mcp/cli` `structuredResult` | `TestMCPV0033TextSummaryProjectsRowsAndKeepsEnvelopeFields`, `TestToolCallWrapsRepositoryFreeTextInUntrustedDataEnvelope`, `TestToolCallEnvelopeEscapesHiddenCharactersAndRefusesTerminator`, conformance `assertToolReceipt` |
| `MCPV0-035` | `affected.Build`, context-bound `affected.Source`, `callFlowsImpact` | `TestAFUV1059FlowsImpactCancelledMidWalk` |
| `MCPV0-036`, `MCPV0-037` | `callCEMReport`, `validCEMPath`, `cemFailure`; `internal/cem/workflow` (`readStable`, `stableReport`) | `TestCEMReportToolAdvertisesArtifacts`, `TestCEMReportToolResolvesArtifactsUnderTheRoot`, `TestCEMReportToolNamesStableRefusals`, `TestCEMReportToolPreviewsStableMap`, `TestCEMReportToolRefusesMapsOutsideTheRoot` (retired 0.x maps, `CEM-V1-210`), `TestCEMReportToolAdmitsTheCallerNamedRoot`; `internal/mcp/cli/testdata/tools-list-task-review.golden.jsonl` |
| `MCPV0-038` | `normalizeFailure` | `internal/mcp/bridge/probe_timeout_test.go`'s `TestMCPV0038ProbeTimeoutHasDistinctCode` injects a probe overrun with `gokernel.WithProbeBound` at one nanosecond on a live request context and asserts `repository-probe-timeout` with no typed class; restoring the `repository-unavailable` mapping fails it; `internal/mcp/bridge/git_timeout_test.go`'s `TestMCPV0038CEMGitTimeoutHasDistinctCode` and `TestMCPV0038IndexDeadlineHasDistinctCode` pin the CEM Git timeouts and the index Git deadline to `repository-probe-timeout`, a plain Git failure to `repository-unavailable`, and caller cancellation or deadline to `cancelled` (decision 0533) |
| `MCPV0-039` | `extractFollowRef`, `newFollowRef`, `followRef.run`, `followRef.bound`, `followRef.sync`, `refreshIndex`, `Registry.SameRoot` | `TestMCPV0039InheritedGitEnvironmentCannotRedirectMove`, `TestMCPV0039FollowRefPinnedToCanonicalRootIdentity`, `TestMCPV0039AnswerHoldsTheFollowLock`: inherited Git variables, a retargeted root path, a replaced root (`ROOT_CHANGED`) and a concurrent call never move another checkout or move under an answer; `TestMCPV0039FollowRefRefreshesTreeAndIndexBeforeAnswer`: a moved branch is checked out on the detached linked worktree and its index snapshot is fresh before the next status answer, which reports the moved commit; startup refusals for a main checkout, an attached worktree, a missing ref, an aliased root, a duplicate or option-like ref and `--version` |
| `MCPV0-040` | `followRef.sync` dirty and attached guards | `TestMCPV0040DirtyTreeNeverModifiedAndFlaggedStale`: untracked and tracked edits survive, head unchanged, `DIRTY` and `stale`; a checked-out branch is `NOT_DETACHED`; an injected index failure is `INDEX_FAILED`; `TestMCPV0040ChecksRunWhenHeadAlreadyCurrent`: dirty and attached at the current head are stale with no index write; `TestMCPV0040MoveNeverOverwritesIgnoredFiles`: an ignored file the new commit tracks survives as `CHECKOUT_FAILED` |
| `MCPV0-041` | `withFollowRef`, `followState.block`, `toolFailure` | `TestMCPV0041MissingRefIsTypedErrorAndCommitEchoed`: the commit is echoed; a deleted ref is tool error `follow-ref-missing` with `reasonClass` `unclassified`, `state` `MISSING` and the worktree unchanged; the `corvint.help` exception: `TestMCPV0100HelpToolSkipsFollowRefBinding` |
| `MCPV0-090` | `callContext` (deferred `Index.Release`), `detachedContext`, `compileContext` retry release | `internal/contextindex/bridge_release_test.go`'s `TestMCPV0090ContextCallReleasesItsSnapshotMapping`: one task-review bridge answers 1 sequential and 24 concurrent (6x4) `corvint.context` calls from a mapped pack; the ring holds exactly 1 mapping, 0 stay live after it is evicted, and every result encodes to the first result's bytes after the unmap; fails with 1 live mapping when the release is removed; passes under `-race` |
| `MCPV0-095` | `RepositoryBinding`, `snapshotIndex`, `bindingFromRepository` (no path member) | `internal/mcp/bridge/worktree_test.go`'s `TestMCPV0095MixedBindingIsCountAndDigestOnly`: `corvint.status` and `corvint.query` over a worktree with one untracked file bind `MIXED` with count 1, exactly the seven members, and no byte of the dirty path's name |
| `MCPV0-100` | `internal/mcp/bridge` (`ToolHelp`, `callHelp`, `validHelpReceipt`, `allTools`), `internal/capcatalog` | `internal/mcp/bridge/help_test.go`'s `TestMCPV0100HelpToolReturnsTheEmbeddedCatalogAndGuide` (all four profiles: advertised, exact catalog and guide text with matching `byteCount` and `sha256`, under half the budget and inside the frame, deterministic, closed-argument refusals, cancellation, and an empty `.git` that would fail any repository read); `internal/mcp/cli/help_test.go`'s `TestMCPV0100HelpToolAnswersOnEveryProfile` (compiled `run` per profile: `structuredContent` and the enveloped text block both carry the embedded bytes); `cmd/corvint/help_catalog_test.go`'s `TestMCPV0100HelpToolMatchesTheCLI` (bridge receipt text equals `corvint help --json` and `corvint help guide` stdout byte for byte); the same test's receipt-validator tampers (text, recomputed hash, missing or other-topic command, cross-topic text, wrong or float `byteCount`, extra member, missing schema); `TestMCPV0100HelpCancellationPrecedesTheBudget` (oversized text with a cancelled context fails `cancelled`, live context abstains `OUTPUT_BUDGET_EXCEEDED`); `internal/mcp/cli/help_test.go`'s `TestMCPV0100HelpToolSkipsFollowRefBinding` (compiled `run --follow-ref` carries no follow-ref block; handler calls with the ref current, moved and deleted answer the embedded catalog without refreshing or moving the lane HEAD); repinned `internal/mcp/cli/testdata/tools-list-*.golden.jsonl` and conformance tool lists |
| `MCPV0-101` | `cmd/corvint-docs-mcp` (`Handle`, `call`, `helpResult`) | `cmd/corvint-docs-mcp/help_test.go`'s `TestMCPV0101HelpToolAnswersOnTheWire` (both protocol versions: shared descriptor listed, wire result equal to `bridge.HelpToolResult`, embedded bytes in `structuredContent` and the enveloped text, unknown topic `-32602`) and `TestMCPV0101HelpToolReadsNoRepositoryState` (a valid `corvint.docs_draft` succeeds before the root is removed and refuses with `repository-unavailable` after); `cmd/corvint-docs-mcp/main_test.go`'s three-tool `tools/list` |
| `MCPV0-102` | `cmd/corvint-corpus-mcp` (`listedTools`, `call`, `helpResult`) | `cmd/corvint-corpus-mcp/help_test.go`'s `TestMCPV0102HelpToolAnswersOnTheWire` (the same checks; no corpus timing `_meta`) and `TestMCPV0102HelpToolReadsNoRepositoryState` (root and artifact removed; a valid `corvint.docs_info` succeeds before and refuses with `corpus-input-unavailable` after) |
| `MCPV0-103` | `cmd/corvint-test-validity-mcp` (`Handle`, `call`, `helpResult`) | `cmd/corvint-test-validity-mcp/help_test.go`'s `TestMCPV0103HelpToolAnswersOnTheWire` and `TestMCPV0103HelpToolReadsNoRepositoryState` (a valid `corvint.test_validity` succeeds before the root is removed and refuses with `repository-unavailable` after); `conformance/mcp-test-validity-v0` (`cases.json` tools, `TestToolCatalogueIsExactlyOneReadOnlyTool`); `internal/companionrelease` workflow smoke tool list |
| `MCPV0-104` | `internal/mcp/bridge` (`HelpDescriptor`, `HelpToolResult`) | `internal/mcp/bridge/help_test.go`'s `TestMCPV0104HelpToolResultIsTheRegistryAnswer` (equal to `Registry.Call`'s object and framed text, same descriptor, oversized, non-UTF-8 and closed-argument refusals, cancellation); the three wire tests above |
| `MCPV0-105` | `internal/mcp/bridge` (`HelpToolResult`), `internal/capcatalog` | `cmd/corvint/help_catalog_test.go`'s `TestMCPV0105CompanionHelpMatchesTheCLI` (receipt text equals `corvint help --json` and `corvint help guide` stdout) |
| `MCPV0-110` | `internal/mcp/bridge` (`impactInput.BudgetBytes`, `optionalBytes`, `callImpact`) | `internal/mcp/bridge/budget_args_test.go`'s `TestMCPV0110ImpactBudgetBytesIsCLIPathImpactBudget` (budgeted receipt equals `EvalImpactSyntax`'s bytes, omitted budget equals `ImpactSyntax`'s, six invalid values refused); repinned `internal/mcp/cli/testdata/tools-list-*.golden.jsonl` |
| `MCPV0-111` | `internal/contextsummary` (`Summarize`, `SafePath`), `internal/mcp/bridge` (`contextInput.SummaryBytes`, `summarizedContext`), `cmd/corvint` (`summarizeContextPacket`) | `TestMCPV0111ContextSummaryBytesIsCLISummary` (summary equals `Summarize` over the canonical receipt plus LF, digest and MCP continuation, binding kept, five invalid values refused, `SUMMARY_SHAPE` reason); CLI bytes unchanged under the existing `cmd/corvint` context-summary tests |
| `MCPV0-113` | `internal/mcp/cli` (`run`, `usage`) | `usage_test.go`'s `TestMCPV0113HelpAndRelativeRoot`; `roots_test.go` relative aliased root |
| `MCPV0-950` | `internal/mcp/rootbind` (`Resolve`), `internal/mcp/bridge` (`Profile.Tools`), `internal/mcp/cli` (`run`, `startUnavailable`, `answerUnavailable`), `script/check-install-lifecycle.sh` | `internal/mcp/cli/unavailable_test.go`'s `TestMCPV0950SubdirectoryRootBindsToplevel`, `TestMCPV0950UnusableRootServesListAndRefusesCalls` (eight unusable roots: same `tools/list`, every tool `invalid-root` but help, unknown tool `-32602`, exact reason line), `TestMCPV0950UnusableRootStillExitsWhereARepositoryIsRequired`, `TestMCPV0950ProfileToolsMatchRegistries`; `script/check-install-lifecycle_test.sh` step `mcp-unavailable-root` |
| `MCPV0-951` | `cmd/corvint` (`mcpServerVersion`), `internal/mcp/cli` (`Serve`, `runVersioned`, `declaredPlugin`, `notePluginCompatibility`), `integrations/claude-code/plugins/corvint/.mcp.json` | `internal/mcp/cli/plugin_test.go`'s `TestMCPV0951ServerInfoCarriesTheBuildVersion`, `TestMCPV0951DeclaredPluginIsBounded`, `TestMCPV0951StatusReportsPluginCompatibility`; `cmd/corvint/mcp_test.go`'s `TestMCPV0951MCPVerbReportsTheBuildVersion`; `TestAHI820ClaudePluginDeclaresTestValidityAndDocsGroups` and `TestAHI820ClaudePluginArgsServeTestValidityAndDocs` (declared version and tools); `AHI-198` in `integrations/host-adapters.test.mjs` |
| `MCPV0-952` | `internal/mcp/protocol` (`InvalidParamsData`, `InvalidParamsAt`), `internal/mcp/server` (`sanitize`), `internal/mcp/toolgroup` (`ListParamsRefusal`, `WithInvalidParamsData`, `diagnose`), the `tools/list` and `tools/call` handlers of `internal/mcp/cli` and the three dedicated servers | `internal/mcp/cli/invalid_params_test.go`'s `TestMCPV0952InvalidParamsCarryReasonAndField` (unknown tool, bad enum, pattern violation and ten more) and `TestMCPV0952LegacyInitializeInvalidParamsCarryField`; `TestMMRV0013ParityWithDedicatedServers` (same data from both servers) |
| `MCPV0-953` | `internal/mcp/bridge` (`callHelp`, `helpTopicText`, `helpCommandNames`, `validHelpReceipt`, `helpDescriptor`, `WithOutputSchemas`), the `tools/list` surfaces of `internal/mcp/cli` and the three dedicated servers | `internal/mcp/bridge/help_test.go`'s `TestMCPV0100HelpToolReturnsTheEmbeddedCatalogAndGuide` (guide default, command slices, refusals, command-topic tamper); `internal/mcp/cli/help_test.go`'s `TestMCPV0100HelpToolAnswersOnEveryProfile` (default and slice under 32 KB on the wire) and `TestMCPV0953ToolsDeclareOutputSchema`; `cmd/corvint/help_catalog_test.go`'s `TestMCPV0100HelpToolMatchesTheCLI` and `TestMCPV0105CompanionHelpMatchesTheCLI` (slice equals `corvint help --json impact`); the three `MCPV0-101..103` wire tests; repinned `internal/mcp/cli/testdata/tools-list-*.golden.jsonl` |
| `MCPV0-954` | `internal/mcp/server` (`legacyInitializeRefusal`), `internal/mcp/bridge` (`PathRule.Pattern`, `SubjectPathRule`, `impactPathRule`, `nestedObjectSchema`, `snapshotSchema`, the flows path schema), `internal/mcp/docsbridge` and `internal/mcp/testvaliditybridge` path schemas | `internal/mcp/server/server_test.go`'s `TestMCPV0954InitializeRefusalDistinguishesTheClient`; `internal/mcp/bridge/patterns_test.go`'s `TestMCPV0954PathPatternsAreRE2AndEquivalent` (every string of up to five characters over a 14-character adversarial alphabet, plus `.go` and `.tsx` suffixes, against the replaced rules) and `TestMCPV0954SchemasHaveOneRootSchemaKeywordAndRE2Patterns`; repinned `internal/mcp/cli/testdata/tools-list-*.golden.jsonl`; `conformance/mcp-2026-07-28` and `conformance/mcp-test-validity-v0` pass unchanged |
| `MCPV0-114` | `internal/mcp/cli` (`followRefTools`, `toolHandler.Handle`) | `follow_ref_tools_test.go`'s `TestMCPV0114FollowRefToolsNotReadOnly`: both modes' `tools/list` compared; the profile goldens keep the plain list |
| `MCPV0-112` | `contextindex` (`noteEvidenceTruncated`, `evidenceTruncatedDisclosure`, `mergeEvidence`, `recordResultWithSortedPaths`, `registryCaller`, `rubyAutoload.referrers`, range impact) | `internal/contextindex/impact_caps_test.go`'s `TestMCPV0112ImpactCountsEvidenceCeilingSeparately` (15 reference lines: 10 kept, `evidence_truncated` 5, `evidence_omitted` 6, disclosure line, untruncated result without the key, merge count) |
| `MCPV0-034` | `Registry.Call` (post-operation `ctx.Err()` check) | `TestContextCancellationNeverReturnsReady` (early, cold-loader and late cases; healthy receipt and root digest unchanged), `TestContextTimedCancellationRetiresWork` (measured cancel-to-return, no surviving context-index goroutine), `TestCancelledCallFailsClosed` |

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

### Companion promotion evidence (V1-0191)

The intent is accepted for the companion profile under the owner's 1.0 feature-cut delegation
(2026-10-20); requirements without their own acceptance marker inherit it. `corvint-mcp` stays the
required 1.0 companion of the 1.0 release classification table, never a Core surface, and every
adapter row keeps `FALLBACK` (`Relationship to hosted MCP`, rollout item 2). Delivery moved from
experimental to implemented for the stdio companion profile once all of the evidence below was
retained for one code tree (`a7870cac3` changes only documentation over `2b0cfce94`):

1. The compiled-process suite `conformance/mcp-2026-07-28` passes (`/0`, `/1`, `/2` and the flows
   cases). Observed on 2026-10-20 at `2b0cfce94`: 35 top-level tests pass, 0 fail.
2. With `CORVINT_MCP_OFFICIAL_SCHEMA` naming a copy whose SHA-256 is
   `ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203`,
   `TestServerTrafficMatchesOfficialSchema`, `TestTaskReviewTrafficMatchesOfficialSchema` and
   `TestAFUV1034FlowsOfficialSchema` pass rather than skip. Observed on 2026-10-20 at `a7870cac3`
   against an owner-approved copy outside the repository (the schema is not vendored): all three
   pass with no skip, and the whole package passes 38 tests with 0 skips
   (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-mcp-companion-qualification.md`).
3. The task-review tools' read-only evidence (`MCPV0-024`, `MCPV0-025`) passes in process and over
   the compiled process; observed on 2026-10-20 (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-mcp-companion-qualification.md`).

`mcp-server-unavailable` left `globalDegradations` in `integrations/compatibility.json` and both
adapter manifests in the change that cites the item-2 receipt, together with
`integrations/README.md` and `ROADMAP.md`. That receipt covers the stdio server only, so no host
tuple changed status with it: every row stays `FALLBACK`, the Claude Code manifest keeps
`surfaces.mcp` `UNAVAILABLE` because it ships no MCP declaration, and the adapter versions moved
only because their shipped content changed (`AHI-020`). The official Streamable HTTP conformance runner has no stdio target
and stays `NOT_RUN`; it is not a prerequisite for the companion profile.
