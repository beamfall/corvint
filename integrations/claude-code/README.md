# Corvint for Claude Code

This local marketplace contains the `corvint@corvint` plugin, which works in the Claude Code terminal
and in the Code tab of the Claude desktop app. It provides skills that run the installed `corvint`
command for change-scoped answers. It registers no hooks and no MCP server (both were removed by
decision 0594), and its support level is `FALLBACK`.

Prerequisite: `corvint` is on `PATH` (see [Try it](../../README.md#try-it-in-sixty-seconds)), and the repository is
trusted in Claude Code.
The plugin does not install dependencies, start a daemon, read transcripts, fetch from the network,
or inherit secrets into the Corvint child process.

Native lifecycle, run from the root of a Corvint checkout:

```text
claude plugin marketplace add ./integrations/claude-code
claude plugin install corvint@corvint --scope user
claude plugin list
claude plugin details corvint@corvint
claude plugin disable corvint@corvint --scope user
claude plugin enable corvint@corvint --scope user
claude plugin uninstall corvint@corvint --scope user
claude plugin marketplace remove corvint
```

Commands (skills). Each runs one existing verb and reports its uncertainty. Only `/corvint:index`
writes state, and only the derived snapshot:

| Command | Runs | Started by |
|---|---|---|
| `/corvint:affected [BASE]` | `corvint affected` | user or model |
| `/corvint:impact [BASE \| PATH...]` | `corvint impact` | user or model |
| `/corvint:review [BASE]` | `corvint review` | user or model |
| `/corvint:index` | `corvint index --if-stale` | user only |

Removing the plugin leaves Corvint repository data unchanged.
