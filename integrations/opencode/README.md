# Corvint for OpenCode

Version 0.6.0 targets unmodified OpenCode 2.0.18. It supplies awaited task context, on-demand
query and exact source expansion, edit/evidence/verification observations, explicit outcomes, and
compaction recovery from current Git state. OpenCode 1.x requires the older 0.2.9 adapter.

Call `corvint_status` to check the installed package, host executable, Corvint executable, OS and
architecture against `../opencode-qualification.json`. Only an exact passing tuple reports
`integrationSupport: FULL`; changed or untested builds report `UNQUALIFIED`. Qualification is
maintainer test evidence, not execution attestation. Its scope is the stock native integration
under AHI-032, including the retained functional, latency and critical-evidence recall checks.

Execution authority is `NONE`. Core lifecycle receipts remain `FALLBACK`, and
`frontier-authority-unavailable` describes that authority boundary. Completion observations are
advisory; the plugin cannot enforce completion or keep the CLI alive for a continuation. No fork,
replacement OpenCode distribution or privileged authority service is installed.

The runner supports macOS and Linux process groups. Only tuples in the qualification report are
qualified; Windows remains unsupported until process-tree cleanup is implemented and tested.


## Context inspector

The native terminal sidebar shows **Corvint context** for the current session. Click it or run
`/corvint` to open the evidence panel. Run `/corvint Locate Add in add.go` to request context
without asking a model. The panel also accepts these keys:

| Key | Action |
| --- | --- |
| Up / Down | Select evidence in the list, scroll in the source/details pane |
| Enter / click a location | Open its pinned source through Core |
| Tab | Switch between evidence and source without losing selection |
| / | Find evidence by file, symbol, reason or authority |
| x | Clear the filter and return to the list |
| Page Up / Page Down | Move through evidence or scroll a page |
| Left / Right | Scroll long source lines horizontally |
| h | Enable host syntax highlighting (may download a language parser), or return to plain text |
| l | Return to the cited source line |
| g | Show gaps and limitations |
| i | Show the full inclusion reason, summary and immutable identities |
| r | Request context for an explicit task |
| f | Toggle full-screen; wide panels show evidence beside source |
| Escape | Close the panel |

Each location shows its inclusion reason, authority and confidence. The source reader preserves
Core's tree/blob verification. Control characters are escaped for display. Freshness describes the
last observation; this UI does not continuously check Git. Observed edits or compaction mark the
view stale until another context request. Missing/unsupported evidence stays visible. Source uses native line numbers and a cited-line marker. Plain rendering is immediate and requests
no syntax parser. Press h or click Enable syntax to opt into OpenCode highlighting for this panel;
the host may download missing language parsers. Source text is parsed locally.
Unrecognized languages stay readable as plain text. Long lines scroll horizontally to preserve
line alignment. At 96 or more panel columns the list stays beside the source; compact panels use
Evidence/Source/Gaps/Details views. Theme colors always have text labels. Selection survives
receipt reorder, while source is cleared when its receipt or observed state changes.

The view is bounded and held in memory, with no receipt or prompt journal. Its RPC runs through the
existing OpenCode server and inherits that server's client trust boundary. The terminal plugin
receives only context/evidence views and hashed-session update notices, not transcripts. Removing
or disabling the plugin disposes the view and its subscriptions. Desktop/web custom panels and the
proposed Impact/Proof views are outside this first slice.

`script/qualify-opencode-inspector.py --host /absolute/opencode --corvint /absolute/corvint
--output /absolute/evidence` exercises the actual 2.0.18 terminal in isolated temporary Git and XDG
locations, without a model call. It requires Python 3, Git, a POSIX PTY and permission to start the
host's temporary loopback server. Its UI witness is separate from full native integration
qualification; it never writes `opencode-qualification.json` or grants execution authority.

## Install, discover, upgrade, disable, uninstall

The package is not published to npm yet. OpenCode supplies the terminal renderer and Solid runtime. From a Corvint checkout,
add a `file://` URL for its `src` directory to OpenCode's `plugins` array:

```json
{
  "plugins": [
    { "package": "file:///absolute/path/to/corvint/integrations/opencode/src", "options": {} }
  ]
}
```

Replace the absolute path with your checkout. `opencode.example.json` uses this local-source form;
a plain string entry works when no options are needed. OpenCode 2.0.18 loads a local entry only
from a directory, through its `server.js` or `index.js`: an entry naming `src/index.js` itself is
skipped with the warning "configured plugin path must be a directory", and one naming
`integrations/opencode` is dropped without a message. Configure the adapter once to avoid registering it
twice. Upgrade by updating the checkout. Disable or uninstall by removing the `plugins` entry;
remove the checkout only when it is no longer needed.

## Qualify this installation

From a clean, committed Corvint checkout, run the first-party command below. It requires Git,
Python 3.9 or later, Node.js 20 or later, Go 1.27.1, the real Corvint executable, and the actual
OpenCode 2.0.18 executable on macOS or Linux. If `opencode2` is a shell launcher, pass the
executable it launches; the command rejects shell launchers rather than recording the wrong image.

```sh
python3 script/qualify-opencode.py \
  --host /absolute/path/to/opencode \
  --corvint /absolute/path/to/corvint \
  --output /tmp/corvint-opencode-qualification
```

This command runs the focused adapter/cleanup tests and the real native campaign itself. It freezes
package, executable, collector and test inputs across both gates, then atomically writes
`integrations/opencode-qualification.json`, exactly where `corvint_status` reads it. It retains the
same accepted report and gate logs under `--output`, which must be outside the checkout. No manual
conversion or supplied PASS assertions are accepted. The lower-level
`script/qualify-opencode-native.py` produces native campaign evidence only and cannot qualify an
installation by itself.

A requalification saves the previous record with its new evidence and first sets the active record
to INCOMPLETE. Failure or interruption therefore leaves native support UNQUALIFIED. Only complete,
unchanged passing evidence replaces that record with PASS. The local record is ignored by Git;
cloning this source does not transfer qualification to another machine. Run the command there for
its exact tuple. If installing a copy of the package, copy it unchanged together with the record at
its sibling `opencode-qualification.json` path. Then restart OpenCode and call `corvint_status`.
Execution authority remains NONE, Frontier UNAVAILABLE, and legacy receipts FALLBACK.

## MCP servers

MCP servers are configured separately from the lifecycle plugin. Copy the entries from
`mcp.example.json` into the `mcp.servers` object in `opencode.json`, replacing executable and repository
paths. Each command needs its own absolute repository root. The strict MCP profile currently
requires a repository with a `.git` directory; linked Git worktrees are not admitted.

All Corvint MCP servers default to the latest published MCP protocol, `2026-07-28`.
OpenCode clients using the older `initialize` handshake need the explicit
`--protocol-version 2025-11-25` compatibility option shown in the example. Without it,
initialization returns `Method not found`. This selection changes transport negotiation only;
repository bounds, tool schemas and Corvint receipts stay the same. Modern clients should omit
the selector or select `2026-07-28` explicitly.

The example enables `corvint-mcp` and `corvint-test-validity-mcp`. Build them from this checkout
with `go build -o /absolute/bin/corvint-mcp ./cmd/corvint-mcp` and
`go build -o /absolute/bin/corvint-test-validity-mcp ./cmd/corvint-test-validity-mcp`, or use
release binaries containing the compatibility selector. Verify discovery with `opencode mcp list`;
a connected status alone does not verify a tool call or qualify native lifecycle support.
Development probes with OpenCode CLI 1.17.18 and 1.18.31 connected the core, test-validity and
docs servers built from this checkout and completed one call per tool through this selector; both
clients send `notifications/cancelled` after each completed call, which the server ignores. These
probes are transport evidence only, not a host-version support claim.
The optional docs and experimental corpus servers accept the same selector, but remain separately
configured companions with their existing prerequisites. Disabling a server uses `disabled: true`;
uninstalling it removes its `mcp.servers` entry.

See the [MCP transport contract](../../docs/specs/mcp-server-2026-07-28-v0.md) for the exact
supported profiles, recorded host probes and remaining qualification boundaries.

## Tool selection

The native plugin supplies general task context and explicit outcome observations. The core MCP
server supplies repository status, narrow workflow-authority queries and tracked-Go impact; the
separate test-validity server reads retained test evidence. Installing both surfaces makes these
complementary tools available without expanding either server's frozen registry.

Copy `skills/corvint/` into your project's `.opencode/skills/` (or your global
`~/.config/opencode/skills/`) to let OpenCode discover the `corvint` skill. It explains which tool
answers each question, how to interpret unsupported/missing evidence, and when to use the CLI.
The skill is loaded on demand through OpenCode's native skill tool. The optional documentation
servers are useful when working with their supported draft or corpus artifacts; they are not
prerequisites for ordinary code context and test evidence.

## Lifecycle options

Corvint must be available as the `corvint` executable. Plugin `options` may set `corvintBinary`,
`hostVersion`, `automaticTimeoutMs`, `queryTimeoutMs`, and `enableBetaContext`; equivalent explicit
environment settings are `CORVINT_BIN`, `CORVINT_OPENCODE_HOST_VERSION`,
`CORVINT_OPENCODE_TIMEOUT_MS`, `CORVINT_OPENCODE_QUERY_TIMEOUT_MS`, and
`CORVINT_OPENCODE_BETA_CONTEXT=1`. Values are never added to lifecycle payloads. The child process
receives only a small non-secret environment allowlist.

Explicit `corvintBinary`, `hostVersion`, `automaticTimeoutMs`, and `queryTimeoutMs` options take
precedence over the ambient `CORVINT_BIN` and `CORVINT_OPENCODE_*` variables. Without a `hostVersion`
option the version OpenCode reports to the plugin is used, then `CORVINT_OPENCODE_HOST_VERSION`.

Automatic events and explicit context queries default to 2,000 ms. Automatic overrides accept
integers from 25 to 2,000 ms; query overrides accept 25 to 10,000 ms. Invalid values use the default.
These are complete-command deadlines, separate from latency targets. A timeout warning includes
the applied deadline; it does not diagnose the underlying cause. `FALLBACK` also appears on successful
receipts when authoritative frontier evidence is unavailable, so inspect `ok` and the named code.

The `session.created`, `session.execution.succeeded`, `session.execution.failed`, `session.idle`
and `session.deleted` events and the tool `execute.after` hook are translated to
`corvint harness event`. Raw session IDs are hashed; raw prompts, transcripts, tool arguments, tool
output, and environment maps are never sent. A completed built-in `write`, `edit` or `patch` call
reports its target files as a session-scoped `file-change`; the adapter serializes those
subprocesses and coalesces duplicate paths for one session so an edit burst cannot overlap Corvint
invocations. An `unsupported-impact-path-suffix` or `unsupported-impact-repository` refusal from that
best-effort event is recorded at info level instead of as a warning. Opened in a directory outside
any Git repository, the plugin registers no hooks or tools and runs no Corvint command (decision
0378). OpenCode 2 exposes plugin tools to the model through its code-mode `execute` tool; that outer
call is not reported, its inner tool calls are. The
prompt hook and `corvint_context` tool query the current bounded task. A task over the
2,000-character/16,384-byte bound is served by its disclosed anchor query (`AHI-016`) or refused as
`prompt-over-query-bound`, never truncated. Only the `metadata.corvint` namespace can contribute evidence handles or verification observations.
`corvint_record_outcome` records a caller-reported `passed`, `failed`, or `blocked` outcome only when
the caller also supplies a bounded task, changed paths, and closed-schema verification observations.
The task is hashed locally; only `taskSha256` enters the non-persistent session-end receipt.

The optional `enableBetaContext` setting repeats cached startup context. Default prompt context
and compaction recovery use awaited native hooks; all injected additions are bounded and framed.

## Automatic context and exact expansion

The prompt hook awaits Corvint and appends at most 8000 UTF-8 bytes, including framing and any
anchor-query disclosure, to the same unchanged prompt. Repeated events/messages are deduplicated;
deleted or evicted sessions cancel pending work. OpenCode may retain the augmented prompt in its
normal conversation history. The adapter writes no prompt files and reads no transcript.

`corvint_context` returns `expansionHandles` encoded from the evidence's pinned Git tree/blob/path.
Pass a handle to `corvint_expand`; replace its `all` range with an explicit `START-END` when a full
file exceeds the 6000-byte source cap. The core verifies identity and returns exact selected bytes;
invalid, stale, oversized or unsafe output is refused. Selection does not read changed worktree bytes.

After `session.compaction.ended`, the next context hook awaits fresh `startSource: compact` context.
Failure keeps recovery pending and never restores an old packet. Recovery covers current dirty
paths; it does not reconstruct previous conversation decisions. `enableBetaContext` additionally
repeats startup context on model requests; ordinary task context and compaction recovery are enabled
by default.
