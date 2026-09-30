# One real-editor unsaved Unicode definition development witness, 2026-09-29

The independently reviewed lifecycle repair is retained separately. This new bounded tooling delta
adds explicit `{workspace}` operator argument resolution and `--semantic-development`. It uses one
private Go 1.27.1 module, actual Neovim builtin client or actual Microsoft's VS Code LanguageClient,
unsaved emoji-containing text, one definition request and exact negotiated target position. It
requires wire didOpen/full-text didChange, the same URI and returned target, an actual unsaved
buffer flag, unchanged disk, and the existing strict lifecycle/owned-process retirement. It saves
no editor buffer. This is one development observation, not a general Unicode or semantic gate;
all tuples remain UNQUALIFIED and the broader matrix remains NOT_RUN.

Official Context7 documentation already fetched during the frozen review confirmed
`Client:request(method, params, handler, bufnr)` and `vim.str_utfindex(text, encoding)` for Neovim,
and LanguageClient request/converter APIs for VS Code. Each client applies its own document change;
the frontend is not replaced by a synthetic protocol script. The target is line 1 character 13 in
UTF-8, 11 in UTF-16 or 10 in UTF-32; the query is the ASCII line 2 character 8.

The first actual runs started against `/tmp/corvint-lsp-semantic` digest
`beb086e3b7b8fb2a68f2b5c29c5640ca42571b41978251998b9492b51bef2a87`, caller source claim
`1e22d60b84b786fc61236c2b8c270fe3e575a2cf`. VS Code failed initialization with Backend unavailable.
The integrating owner traced it to canonical-root admission: macOS `/var/folders/...` aliases
`/private/var/folders/...`, and goplsclient.Start requires an already canonical root. The harness
now resolves its disposable directory once before creating the module and uses that same root in
both client initialization and the explicit operator token. This preserves server admission.
The Neovim early run was precisely SIGTERMed after the separate server backpressure lifecycle
input was invalidated; its outer report and owned cleanup were retained. No failed run is reported
as a semantic success. Original reports: `/tmp/lsp-editor-vscode-semantic-first.json` and
`/tmp/lsp-editor-neovim-semantic-first.json`.

Focused transport/outer lifecycle regressions, Python syntax, Node syntax and staged requirement-index
checks passed. Actual semantic observations remain NOT_RUN pending the repaired immutable connected
server. No sealed holdout, native ticket mutation, push, merge or CEM seal occurred in this substream.

## Semantic evidence-order repair

Independent review reproduced a validator gap: a synthetic definition response placed before
actual didOpen/full-text didChange could pass. `/tmp/lsp-editor-semantic-order-before.log` retains
that original failing negative assertion. The narrow repair now requires exact ordered
open < fixture change < query < matching response, a strictly increasing document version, the
client's request snapshot version matching that change, no newer change before the response,
and the fixture change being the latest document state at the query. Neovim's actual request
handler context captures its LSP document version; VS Code records its document.version.
Negative checks cover out-of-order events, foreign URI, non-increasing version, stale client
snapshot and a newer change between request and response for UTF-8 and UTF-16.

Two real-client runs had already completed against repaired clean server source
`4834e84edcc967b4cea980d2ae377e228d4fed59`, executable digest
`337e7537090f21a2318ae74dbcb256902afa19d7ebdd2448f0c2cc6eb1292075`. Neovim returned the same-file
Unicode target at line 1 character 13; VS Code returned line 1 character 11. Both observed actual
didOpen/full-text didChange and unchanged disk, completed strict lifecycle and retired owned
processes. Their raw pre-validator-repair reports are retained as observations, not evidence for
the final validator. Final committed-harness refresh follows separately; all tuples remain UNQUALIFIED.
