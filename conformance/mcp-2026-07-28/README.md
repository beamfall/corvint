# Corvint MCP 2026-07-28 black-box conformance

This directory is an independent consumer of the production `corvint-mcp` stdio
executable. It does not import `internal/mcp` and does not treat implementation
tests as protocol evidence.

The suite targets only MCP `2026-07-28`. That revision is stateless: every
request carries protocol version and client capabilities in `_meta`,
`server/discover` replaces the legacy initialization handshake, and
`initialize`, `notifications/initialized`, `ping`, `logging/setLevel`,
`resources/subscribe`, and `resources/unsubscribe` are removed.

Run from the repository root:

```console
GOTOOLCHAIN=local go test -count=1 ./conformance/mcp-2026-07-28
GOTOOLCHAIN=local go test -race -count=1 ./conformance/mcp-2026-07-28
```

The runner builds `./cmd/corvint-mcp`, launches it as
`corvint-mcp --root ABSOLUTE_CLEAN_ROOT`, and communicates only through newline
delimited JSON-RPC on stdin/stdout. It verifies:

- exact discovery/version metadata and truthful tools-only capabilities;
- deterministic, closed tool catalogues for `corvint.query`, `corvint.impact`, and
  `corvint.status`, with no resources or unimplemented tools;
- stateless per-request metadata and unsupported-version errors;
- rejection of removed legacy methods;
- strict duplicate-key, invalid-UTF-8, malformed-JSON, 1 MiB frame, and depth-64
  boundaries with recovery after a complete bad frame;
- cancellation races, clean EOF, SIGINT/SIGTERM shutdown, no surviving
  observed descendants, and logging/roots behavior without inventing support;
- revision binding, output bounds, secret/error sanitization, and no repository
  mutation for read-only calls.

`cases.json` is the closed case inventory. The Go runner checks that every
listed case remains represented. The official MCP conformance server runner
requires a Streamable HTTP URL; it does not expose a stdio-server target.
Corvint P0 deliberately exposes no network listener, so the official result is
`NOT_RUN`, not passing evidence. The check on 2026-08-23 found GitHub release
`v0.1.16`; current official server usage remains `server --url URL`. The exact
dated official schema observed by this corpus has SHA-256
`ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203`.
That digest is provenance only: the schema is not vendored or loaded by this
no-network runner, so official-schema execution remains `NOT_RUN`.

The production tools do not expose a deterministic long-running progress
hook. The suite validates progress-token input, absence of unsolicited
notifications, and correlation if a notification appears, but executed
progress delivery remains `NOT_OBSERVED` rather than a compatibility claim.

One promotion blocker is inherited below this MCP layer:
`INHERITED_KERNEL_GIT_PATH_NOT_PINNED`. The in-process Corvint kernel launches
Git directly but currently resolves it from `PATH` for each operation rather
than pinning one executable identity at process start. This suite does not
mislabel that property as verified; it remains `NOT_OBSERVED` until the kernel
boundary is repaired and independently exercised.

The 2026-09-06 read-safety cases exercise private-metadata Git status through the real MCP process:
configured clean/process filters cannot execute, `core.worktree` cannot redirect observations
before or after admission, and explicit null cannot bypass the impact tool's integer schema.
Status refuses configuration includes, external attributes files, bare repositories, ambiguous
multiline config, split indexes, gitlinks and unsupported metadata rather than silently changing
their semantics. Private scratch is outside repository/Git storage, cleaned after use, and bounded
by one cancellable active status operation. This boundary applies to every MCP tool and the source
documentation commands; it does not qualify all standalone CLI Git execution or resolve the
existing PATH and external-interoperability blockers.
