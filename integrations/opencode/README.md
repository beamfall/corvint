# Corvint for OpenCode (developer preview)

Support is `FALLBACK`. No OpenCode release has passed Corvint safe-frontier and continuation
conformance. The adapter was written against the official `@opencode-ai/plugin` 1.18.21 API; this is
an API reference, not a tested host-version claim. This preview runs only on macOS and Linux: it
uses a detached POSIX process group so timeout and interruption can kill and reap Corvint descendants.
Windows is `UNSUPPORTED` until equivalent process-tree cleanup is implemented and tested.

## Install, discover, upgrade, disable, uninstall

The package is not published to npm yet. From a Corvint checkout, run `npm install` in
`integrations/opencode`, then use a local file URL in OpenCode's native `plugin` array:

```json
{
  "plugin": ["file:///absolute/path/to/corvint/integrations/opencode/src/index.js"]
}
```

Replace the absolute path with your checkout. `opencode.example.json` uses this installable
local-source form. Alternatively, put a loader in `.opencode/plugins/corvint.js` that exports
`{ CorvintPlugin }` from that same absolute source path. Use one discovery method to avoid
registering the adapter twice. Upgrade by updating the checkout and reinstalling its dependencies.
Disable or uninstall by removing the plugin entry or loader; remove the checkout only when it is
no longer needed.

## MCP servers

MCP servers are configured separately from the lifecycle plugin. Copy the entries from
`mcp.example.json` into the `mcp` object in `opencode.json`, replacing executable and repository
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
configured companions with their existing prerequisites. Disabling a server uses `enabled: false`;
uninstalling it removes its `mcp` entry.

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

Corvint must be available as the `corvint` executable. Package options may set `corvintBinary`,
`hostVersion`, `automaticTimeoutMs`, `queryTimeoutMs`, and `enableBetaContext`; equivalent explicit
environment settings are `CORVINT_BIN`, `CORVINT_OPENCODE_HOST_VERSION`,
`CORVINT_OPENCODE_TIMEOUT_MS`, `CORVINT_OPENCODE_QUERY_TIMEOUT_MS`, and
`CORVINT_OPENCODE_BETA_CONTEXT=1`. Values are never added to lifecycle payloads. The child process
receives only a small non-secret environment allowlist.

Explicit `corvintBinary`, `hostVersion`, `automaticTimeoutMs`, and `queryTimeoutMs` options take
precedence over the ambient `CORVINT_BIN` and `CORVINT_OPENCODE_*` variables.

Automatic events and explicit context queries default to 2,000 ms. Automatic overrides accept
integers from 25 to 2,000 ms; query overrides accept 25 to 10,000 ms. Invalid values use the default.
These are complete-command deadlines, separate from latency targets. A timeout warning includes
the applied deadline; it does not diagnose the underlying cause. `FALLBACK` also appears on successful
receipts when authoritative frontier evidence is unavailable, so inspect `ok` and the named code.

Stable `session.created`, `session.idle`, `session.deleted`, and `file.edited` events plus
`tool.execute.after` are translated to `corvint harness event`. Raw session IDs are hashed; raw
prompts, transcripts, tool arguments, tool output, and environment maps are never sent. The
adapter serializes `file.edited` subprocesses and coalesces duplicate paths for one session so an
editor burst cannot overlap Corvint invocations. An `unsupported-impact-path-suffix` refusal from
that best-effort event keeps its structured code in OpenCode's log instead of becoming a terminal
warning. The
`corvint_context` tool is the only prompt-bearing path and requires an explicit task. A task over the
2,000-character/16,384-byte bound is served by its disclosed anchor query (`AHI-016`) or refused as
`prompt-over-query-bound`, never truncated. Only the `metadata.corvint` namespace can contribute evidence handles or verification observations.
`corvint_record_outcome` records a caller-reported `passed`, `failed`, or `blocked` outcome only when
the caller also supplies a bounded task, changed paths, and closed-schema verification observations.
The task is hashed locally; only `taskSha256` enters the non-persistent session-end receipt.

The optional beta context hook is isolated in `src/beta-hooks.js`, disabled by default, bounded,
receipt-linked, and injects only cached session-start context. Failures are reported with an
`[corvint/opencode]` warning; a successful receipt's degradations and the expected session-eviction
and stop-recursion guards go to OpenCode's log through `client.app.log` instead. Neither blocks
unrelated OpenCode work. Corvint currently
has no accepted authoritative stop decision: `session.idle` performs one guarded frontier check,
does not continue or stop the host, and suppresses duplicate/recursive invocations.
