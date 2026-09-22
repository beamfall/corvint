# Corvint for Gemini CLI

Status: **FALLBACK**. This package is documented against Gemini CLI 0.55.1. No Gemini CLI release has
yet passed the Corvint black-box conformance matrix. Corvint ships an experimental local stdio MCP
server (`corvint-mcp`), but this extension intentionally omits `mcpServers`; official-schema execution
and official MCP conformance remain `NOT_RUN`, and no accepted frontier-authority capability is
available. `compatibility.json` is the machine-readable claim.

## Native lifecycle

Prerequisites: `corvint` on `PATH` (see [Try it](../../README.md#try-it-in-a-minute)) and `node` on `PATH`, because
the hooks run `hooks/corvint-hook.mjs` with Node. Install asks you to trust the extension source folder.

```sh
gemini extensions validate /absolute/path/to/integrations/gemini-cli
gemini extensions install /absolute/path/to/integrations/gemini-cli
gemini extensions list
gemini extensions disable corvint --scope user
gemini extensions enable corvint --scope user
gemini extensions uninstall corvint
```

For development, use `gemini extensions link /absolute/path/to/integrations/gemini-cli`. Gemini CLI
copies normal installs, discovers them under its extension home, and applies management changes after
a restart. `/extensions list`, `/commands list`, `/skills list`, and `/hooks panel` expose the loaded
surfaces. Uninstall removes the copied package; Corvint repository data and receipts remain owned by
Corvint and are not extension state.

## Boundaries

Hooks invoke only `corvint --root CWD harness event ... --input - --budget-bytes 8000`, pass a strict
safe environment, cap stdin at 131072 bytes, and derive every Corvint subprocess
deadline from the host's declared 1000 ms kill (`AHI-017`): the kill less a 400 ms process reserve, the
10 ms kill grace, and the Node startup time already spent, so prompt queries get at most 590 ms and
non-query events at most 500 ms. A trimmed prompt over 2,000 Unicode
characters or 16,384 UTF-8 bytes is never truncated or sent to Corvint: its complete set of explicit
anchors is queried instead with a disclosure ahead of the context (`AHI-016`), and a prompt with no
anchors or too many degrades as `prompt-over-query-bound`. Hooks never send raw tool input, tool output, transcript paths, prompts-as-logs, or raw session
IDs. `AfterAgent` is an
advisory stop point: `stop_hook_active` and an integration sentinel suppress recursion, and the hook
never emits a deny decision or requests a retry. Missing permissions, missing Corvint, timeouts,
malformed output, and version skew produce a visible host-valid no-op.

The extension intentionally omits `mcpServers` and exact expansion. Those surfaces remain unavailable
until Corvint has accepted protocol contracts and conformance evidence for them.
