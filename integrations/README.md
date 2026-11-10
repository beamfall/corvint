# Agent host packages

Corvint ships developer-preview skill packages for Claude Code and Codex. Each package only runs
the installed `corvint` command on `PATH`, for change-scoped answers: `corvint affected` (which tests
a change can affect), `corvint impact` (what a change or path touches), `corvint review` and
`corvint index --if-stale`. Neither package owns a second graph or changes Corvint evidence
semantics, and neither registers lifecycle hooks or an MCP server.

| Host | Package | Current status |
|---|---|---|
| Codex CLI/Desktop | [`codex/`](codex/) | `FALLBACK` |
| Claude Code | [`claude-code/`](claude-code/) | `FALLBACK` |

The hook adapters, the `corvint-harness-event/0` lifecycle profile, the MCP servers and the Gemini
CLI, OpenCode and Pi packages were removed by decision 0594. A package may be installed or removed
independently without migrating Corvint Core or repository evidence.

[`instruction-profiles.json`](instruction-profiles.json) records each host's instruction-file loading
rules for `corvint taskcontext` (`docs/specs/agent-instruction-doctor-v0.md`).
