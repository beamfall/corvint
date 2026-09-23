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
- deterministic, closed tool catalogues for `corvint.cem.report`, `corvint.context`,
  `corvint.impact`, `corvint.query`, and `corvint.status`, with no resources or
  unimplemented tools;
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
The schema is not vendored, so the default no-network run skips
`TestServerTrafficMatchesOfficialSchema` and official-schema execution stays
`NOT_RUN` there. To execute it, fetch the pinned file and name it:

```console
curl -fsSL -o /tmp/mcp-2026-07-28-schema.json https://raw.githubusercontent.com/modelcontextprotocol/modelcontextprotocol/271ecc9accafdd9b83a3c869fa67c22953b2af80/schema/2026-07-28/schema.json
CORVINT_MCP_OFFICIAL_SCHEMA=/tmp/mcp-2026-07-28-schema.json GOTOOLCHAIN=local go test -count=1 -run OfficialSchema ./conformance/mcp-2026-07-28
```

The test refuses a file whose SHA-256 differs from the pin, then checks the
suite's discovery, tool-list, tool-call and error requests and the live
server's responses against the schema's `$defs`. Its checker covers exactly
the JSON Schema 2020-12 keywords that schema uses and fails on any other
keyword, and `format` stays an annotation as 2020-12 defaults.

The production tools do not expose a deterministic long-running progress
hook. The suite validates progress-token input, absence of unsolicited
notifications, and correlation if a notification appears, but executed
progress delivery remains `NOT_OBSERVED` rather than a compatibility claim.

`corvint-mcp` pins one absolute Git executable at start and refuses to start
when Git does not resolve. `TestGitPlantedOnPathAfterStartNeverRuns` plants a
`git` earlier on the server's `PATH` after start, calls `corvint.status`,
`corvint.impact` with a planning snapshot, and `corvint.cem.report` on a missing
and a present map, and observes that it never runs. It also observes the refusal
to start without Git. This closes the former
`INHERITED_KERNEL_GIT_PATH_NOT_PINNED` blocker.

The proposed `corvint.context` and `corvint.cem.report` tools (`MCPV0-024`,
`MCPV0-025`) have four cases. The read-only case compares a digest of the whole
root, `.git` included, before and after both calls, requires the bound
envelope and receipt, and requires that `.git/corvint/cem-review.md` was never
written. The rejection case sends malformed task, subject, limit, map, revision
and ceiling arguments and requires `-32602`. The escape case links the map, and
separately its parent directory, to a file outside the root. It requires
`cem-map-unavailable` without the outside path in the result and requires both
trees to be unchanged. The last case plants a hunk path carrying the envelope
terminator and requires `corvint-envelope-terminator-collision`, then renders a
1,500-hunk report and requires `ABSTAINED`/`OUTPUT_BUDGET_EXCEEDED` with a null
receipt. The official-schema exchanges cover the success, tool-error and
`-32602` shapes of both tools.

Negative controls, run once on 2026-09-23 and then reverted: without the
start-time Git pin, `TestGitPlantedOnPathAfterStartNeverRuns` fails because the
planted Git ran. With the report tool publishing instead of previewing,
`TestContextAndCEMReportAreBoundReadOnlyAndFramed` fails on the written report.
Both suites were green again after the controls were reverted.

The 2026-09-06 read-safety cases exercise private-metadata Git status through the real MCP process:
configured clean/process filters cannot execute, `core.worktree` cannot redirect observations
before or after admission, and explicit null cannot bypass the impact tool's integer schema.
Status refuses configuration includes, external attributes files, bare repositories, ambiguous
multiline config, split indexes, gitlinks and unsupported metadata rather than silently changing
their semantics. Private scratch is outside repository/Git storage, cleaned after use, and bounded
by one cancellable active status operation. This boundary applies to every MCP tool and the source
documentation commands; it does not qualify all standalone CLI Git execution or resolve the
existing PATH and external-interoperability blockers.
