# Corvint for OpenCode (developer preview)

Support is `FALLBACK`. No OpenCode release has passed Corvint safe-frontier and continuation
conformance. The adapter was written against the official `@opencode-ai/plugin` 1.18.21 API; this is
an API reference, not a tested host-version claim. This preview runs only on macOS and Linux: it
uses a detached POSIX process group so timeout and interruption can kill and reap Corvint descendants.
Windows is `UNSUPPORTED` until equivalent process-tree cleanup is implemented and tested.

## Install, discover, upgrade, disable, uninstall

Install the package using the package manager that owns your OpenCode configuration, then add
`"@corvint/opencode"` to the native `plugin` array in `opencode.json`. OpenCode discovers npm
plugins from that array. Use `opencode.example.json` as the minimal configuration.

Upgrade the package with that package manager. Temporarily disable it by removing the array entry
while leaving the package installed. Uninstall it by removing both the array entry and the package.
OpenCode's local-development alternative is a JavaScript or TypeScript loader in
`.opencode/plugins/` that re-exports this package.

`@corvint/opencode` is not yet published to the npm registry, and OpenCode installs `plugin`
array entries from that registry, so the array entry alone fails today. Until it is published, run
`npm install` in `integrations/opencode` of a Corvint checkout (it fetches `@opencode-ai/plugin`) and
use the loader form with an absolute import of that directory's `src/index.js`.

Corvint must be available as the `corvint` executable. Package options may set `corvintBinary`,
`hostVersion`, `automaticTimeoutMs`, `queryTimeoutMs`, and `enableBetaContext`; equivalent explicit
environment settings are `CORVINT_BIN`, `CORVINT_OPENCODE_HOST_VERSION`,
`CORVINT_OPENCODE_TIMEOUT_MS`, `CORVINT_OPENCODE_QUERY_TIMEOUT_MS`, and
`CORVINT_OPENCODE_BETA_CONTEXT=1`. Values are never added to lifecycle payloads. The child process
receives only a small non-secret environment allowlist.

Explicit `corvintBinary`, `hostVersion`, `automaticTimeoutMs`, and `queryTimeoutMs` options take
precedence over the ambient `CORVINT_BIN` and `CORVINT_OPENCODE_*` variables.

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
