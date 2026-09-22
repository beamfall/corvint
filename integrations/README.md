# Agent harness integrations

Corvint ships developer-preview native packages for Codex, Claude Code, Gemini CLI, and OpenCode.
Every package translates host events into `corvint-harness-event/0`; none owns a second graph or
silently changes Corvint evidence semantics.

| Host | Package | Current status |
|---|---|---|
| Codex CLI/Desktop | [`codex/`](codex/) | `FALLBACK` |
| Claude Code | [`claude-code/`](claude-code/) | `FALLBACK` |
| Gemini CLI | [`gemini-cli/`](gemini-cli/) | `FALLBACK` |
| OpenCode | [`opencode/`](opencode/) | `FALLBACK` |

`FALLBACK` is deliberate: query and lifecycle adapters exist, but Corvint has no accepted closing
Frontier authority, exact-handle expansion command, or conformed MCP server yet. The exact tested
matrix and global gaps are in [`compatibility.json`](compatibility.json). A package may be installed
or removed independently without migrating Corvint Core or repository evidence.

## Where each adapter declares its compatibility

The matrix in [`compatibility.json`](compatibility.json) is the *published* support row per
`(host, surface, host version, adapter version, OS)`. Each adapter separately *ships* the same
facts to its own host, in that host's native manifest rather than a shared file:

| Host | Shipped declaration |
|---|---|
| Claude Code | [`claude-code/plugins/corvint/compatibility.json`](claude-code/plugins/corvint/compatibility.json) |
| Codex | [`codex/plugins/corvint/compatibility.json`](codex/plugins/corvint/compatibility.json) |
| Gemini CLI | [`gemini-cli/compatibility.json`](gemini-cli/compatibility.json) |
| OpenCode | [`opencode/package.json`](opencode/package.json), key `corvintIntegration` |

Four shapes, on purpose: an adapter carries no manifest its host does not already understand.
The `AHI-010` test in [`host-adapters.test.mjs`](host-adapters.test.mjs), run by
`TestHostAdapterJavaScriptHosts`, binds every published row's host, surface, adapter version and
status to the matching shipped declaration, so the two sides cannot drift apart unnoticed. It also
requires the adapter version to equal the package's manifest version (decision 0244).

The matrix names two different things "degradation", and they are not one vocabulary:

- **`receiptDegradationPolicy.recognised`** is the runtime wire vocabulary — codes Corvint Core emits
  inside a `corvint-harness-event/0` receipt. An adapter accepts any subset and refuses anything
  outside it. Adding a code here changes what adapters must tolerate at run time.
- **`globalDegradations`** is the product capability ledger — one entry per V7 item `ROADMAP.md`
  records as not delivered, which is the machine-readable form of the paragraph above. Nothing emits
  these; they describe gaps that hold for every host.

The same test asserts the two lists are disjoint. A code belonging to both would be unreadable: an
adapter could not tell whether it must accept it on the wire or must never see it there.
