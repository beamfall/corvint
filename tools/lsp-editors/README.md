# Actual editor development probes

This optional tooling is outside Core. Every report remains **UNQUALIFIED**. It launches actual
VS Code with Microsoft's language client or actual Neovim with its builtin client against an
operator-selected executable. Successful startup does not qualify semantic, Unicode, rapid-edit,
multi-root, crash, interruption, held-out or outcome behavior.

Prepare a pinned Neovim executable separately. For VS Code, copy the `vscode-client/package.json`
and `vscode-client/package-lock.json` to a disposable directory, then run `npm ci --ignore-scripts`
there. The harness never downloads tools or installs extensions in the user's editor.

```sh
python3 script/qualify-lsp-editors.py --self-check
python3 script/qualify-lsp-editors.py --client neovim \
  --client-bin /absolute/path/to/nvim --server /absolute/path/to/corvint-lsp \
  --server-arg=--experimental --report /tmp/neovim-development.json
python3 script/qualify-lsp-editors.py --client vscode \
  --client-bin /absolute/path/to/code \
  --client-module /absolute/disposable/path/node_modules/vscode-languageclient \
  --server /absolute/path/to/corvint-lsp --server-arg=--experimental \
  --report /tmp/vscode-development.json
```

Repeat `--server-arg=VALUE` for server arguments; no root flag is assumed. The client supplies a
single disposable root in initialize. The client report and bounded raw wire transcript retain
negotiated capabilities/encoding, lifecycle, process exit and cleanup. Reports include private
machine paths and are local evidence; review before publication. Failures yield a nonzero exit.
Unavailable client/server prerequisites refuse before launch.

VS Code launches with a new window and isolated user-data, extension and workspace directories.
The disposable development extension quits that isolated application after the probe. The harness
signals only its unreaped owned Popen process group. Exact private-directory seeds and their
witnessed descendants are observation-only; command changes and zombie states do not prove retirement. Unretired processes fail the probe and keep
the original temporary directory as `cleanupHold`; resolve that exact hold before deleting it.
The stdio proxy separately owns the selected server's process group. Neither process observation
nor a focused mock transport regression qualifies real editor interruption behavior.

Pin the companion source commit and executable SHA256 in the retained report/build log. A
changed profile, client, provider or server build invalidates promotion evidence. Numerical floors
are proposed until real baseline and human acceptance; this harness has no promotion command.

For the separate one-witness semantic development mode, add `--semantic-development` and the
operator-selected server arguments `--server-arg=--gopls`, `--server-arg=/absolute/path/to/gopls`,
`--server-arg=--root`, and `--server-arg='{workspace}'`. The exact token resolves only to the
private fixture workspace. The fixture module declares Go 1.27.1. Its unsaved emoji-containing
buffer is applied through the actual editor; no save occurs. The report validates the definition
URI and negotiated position plus didOpen/full-text didChange and unchanged disk. This observed
witness does not qualify general Unicode, rapid edits, stale responses or semantic outcomes.

The separate `--context-development` mode calls only experimental `corvint/context` after an
actual unsaved buffer change. Use the same explicit server/root/provider arguments as the semantic
example. It creates and commits a fixed offline Go fixture, root/nested AGENTS, numbered SPEC and
test in its own canonical temporary workspace; no client-provided command executes. The validator
checks ordered open/change/query/reply, matching current URI/version/text digest, separate overlay
identity, exact Core bridge schema and actual Git commit/tree/blob/fixture-content witnesses.
The fixture must stay unchanged. ABSTAINED is retained but fails this expected proof. Raw reports
contain private paths and remain local. Only the relative-path summary is publication suitable;
no tuple or whole-session freshness/monotonicity claim is promoted from a single request.

The mutually exclusive `--context-freshness-development` mode makes one immediate actual edit
attempt between a rapid context request and its reply, after a validated baseline, then awaits the
reply before a healthy newest-overlay retry. Complete raw wire determines whether edit cancellation
was witnessed; frontend pending alone is insufficient. Scheduling misses remain NOT_WITNESSED and
exit nonzero. Defensive stale rejection is distinct from the expected cancellation. No artificial
delay or repeat-until-pass loop occurs. All tuples remain UNQUALIFIED; actual new runs are NOT_RUN.
