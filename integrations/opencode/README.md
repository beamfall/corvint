# Corvint for OpenCode (developer preview)

Support is `FALLBACK`. No OpenCode release has passed Corvint safe-frontier and continuation
conformance. Version 0.3.0 targets the OpenCode 2 plugin API (`@opencode/plugin` 2.0.18) and no
longer loads in OpenCode 1.x; use adapter 0.2.9 there. A development smoke run with OpenCode 2.0.18
loaded the plugin and exercised its events, tools and edit hooks; that is not a tested host-version
claim. This preview runs only on macOS and Linux: it
uses a detached POSIX process group so timeout and interruption can kill and reap Corvint descendants.
Windows is `UNSUPPORTED` until equivalent process-tree cleanup is implemented and tested.

## Install, discover, upgrade, disable, uninstall

The package is not published to npm yet and has no runtime dependencies. From a Corvint checkout,
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
`corvint_context` tool is the only prompt-bearing path and requires an explicit task. A task over the
2,000-character/16,384-byte bound is served by its disclosed anchor query (`AHI-016`) or refused as
`prompt-over-query-bound`, never truncated. Only the `metadata.corvint` namespace can contribute evidence handles or verification observations.
`corvint_record_outcome` records a caller-reported `passed`, `failed`, or `blocked` outcome only when
the caller also supplies a bounded task, changed paths, and closed-schema verification observations.
The task is hashed locally; only `taskSha256` enters the non-persistent session-end receipt.

The optional beta context hook is isolated in `src/beta-hooks.js`, disabled by default, bounded,
receipt-linked, and adds only the cached session-start context to each model request through the
session `context` hook; it never runs Corvint itself. Failures are reported with an
`[corvint/opencode]` `console.warn` line; a successful receipt's degradations and the expected
session-eviction and stop-recursion guards use `console.info`. OpenCode 2 has no plugin log API, so
both reach the OpenCode server log. Neither blocks unrelated OpenCode work. Corvint currently has no
accepted authoritative stop decision: the end of a session execution performs one guarded frontier
check, does not continue or stop the host, and suppresses duplicate/recursive invocations.
