# Corvint MCP server

Status: experimental; protocol `2026-07-28`; local stdio only; compatibility `FALLBACK`.

`corvint-mcp` exposes bounded read-only Corvint receipts to a client that implements the
[MCP 2026-07-28 specification](https://modelcontextprotocol.io/specification/2026-07-28). It does
not expose repository source, prompts, writes, test execution, updates, merges, telemetry, HTTP, or
a network listener. The binding contract is
[`docs/specs/mcp-server-2026-07-28-v0.md`](specs/mcp-server-2026-07-28-v0.md).

## Build and run

The server requires Go 1.27.4 and one absolute repository root:

```console
$ GOTOOLCHAIN=local go build -o ./bin/corvint-mcp ./cmd/corvint-mcp
$ /absolute/path/to/corvint-mcp --root /absolute/path/to/repository
```

Configure an MCP client to own that child process. The portable shape is:

```json
{
  "command": "/absolute/path/to/corvint-mcp",
  "args": ["--root", "/absolute/path/to/repository"],
  "transport": "stdio"
}
```

Use absolute paths. Do not wrap the command in a shell. Do not configure a URL, port, legacy
`initialize` request, environment credential, or writable working directory as authority. One
process is bound to one root for its entire lifetime; request arguments cannot switch it. The root
is capped at 4,096 UTF-8 bytes, canonicalized through existing symlinks once, and must contain a
non-symlink `.git` directory. Strict V0 rejects a regular-file `.git` marker, including linked
worktrees.

Every request, including `server/discover`, must contain:

```json
"_meta": {
  "io.modelcontextprotocol/protocolVersion": "2026-07-28",
  "io.modelcontextprotocol/clientCapabilities": {}
}
```

The 2026-07-28 protocol is stateless. It has no `initialize`/
`notifications/initialized` lifecycle. A client supporting modern and legacy MCP should probe
`server/discover`; this server returns method-not-found for a legacy `initialize` request and
ignores a legacy `notifications/initialized` notification without responding.

## Advertised surface

`server/discover` advertises only `tools` with `listChanged: false`. `tools/list` returns:

| Tool | Arguments | Meaning |
|---|---|---|
| `corvint.impact` | `{"paths":["relative/file.go"],"limit":10}` | Revision-bound impact evidence for Go, Ruby, JavaScript and TypeScript paths (non-Go paths are experimental `non-go-syntax-v0` syntax evidence); `limit` is optional, 1–50 |
| `corvint.status` | `{}` | Exact commit/tree/object-format/profile/worktree/dirty-count-and-digest binding |
| `corvint.help` | `{}` or `{"topic":"guide"}` | The `corvint help --json` command catalog, or the `corvint help guide` agent guide, byte for byte from the binary; reads no repository state and is listed on every profile (`MCPV0-100`); `corvint-docs-mcp` and `corvint-test-validity-mcp` list the same tool (`MCPV0-101..105`) |

### Flows profile and tool groups (opt-in)

`--tool-profile LIST` takes a comma list of the profile `flows` and the groups `test-validity`,
`test-evidence`, `docs`, `corpus` and `affected` (`MCPV0-026`, `MMR-V0-012`, `MMR-V0-1118`):

```console
$ /absolute/path/to/corvint-mcp --root /absolute/path/to/repository --tool-profile flows
```

| Entry | Adds |
|---|---|
| `flows` | `corvint.flows.coverage`, `corvint.flows.gaps`, `corvint.flows.impact`, `corvint.flows.map`, `corvint.flows.navigate` and `corvint.flows.status` (experimental; `gaps` takes an optional `provider`, the E2E inventory it compares; `status` reports each flow, variation, attached test and last result, optionally only the flows a `base` reaches; AFU-V1-150 to AFU-V1-165) |
| `test-validity` | `corvint.test_validity` |
| `test-evidence` | `corvint.test_evidence` |
| `affected` | `corvint.affected`: which tests a change requires, as the exact `affected-summary/1` bytes `corvint affected --summary` prints for the root, with `base` planning `base..HEAD` plus the dirty worktree and no `base` the dirty worktree alone. One planner at a time per root; a verdict too large for one message is the tool error `verdict-oversized` (read it with the CLI); a `self-binary-stale` degradation means rebuild `corvint` and restart the server. Served only by `corvint mcp`, whose binary holds the planner; `corvint-mcp --tool-profile affected` exits 2 naming that reason |
| `docs` | `corvint.docs_draft` and `corvint.docs_consume`; each call runs the `corvint-docs` executable beside the server or on PATH (its `docs-tool` verb, decision 0594), and a missing one answers a `not-installed` tool error |
| `corpus` | The `corvint.docs_*` corpus tools the artifact supports; needs `--corpus-artifact [ALIAS=]PATH`, a path relative to the root |

An unknown entry, a repeated selector, or the selector with `--version` exits 2 before the
repository is opened. Without the selector the server lists exactly the three tools above, and a
call to any other tool fails as an unknown tool (`-32602`). The selector composes with
`--protocol-version 2025-11-25`, in either order. The [capability catalog](CAPABILITIES.md#mcp-tools)
lists every tool with its profile.

Discovery is root-independent and returns `cacheScope: "public"`, `ttlMs: 0`, and fixed server info:
name `corvint-mcp`, build version, and Corvint description
`Local read-only Corvint context and repository evidence server.` Tool listing is root-bound and
returns `cacheScope: "private"`, `ttlMs: 300000`, and no next cursor.

All input schemas are closed. Impact accepts
1–100 unique normalized repository-relative paths, each at most 1,024 Unicode scalar values, with
the lowercase suffix `.go`, `.rb`, `.js`, `.jsx`, `.mjs`, `.cjs`, `.ts` or `.tsx` (NGI-V0-001). A
non-Go path answers with the experimental `non-go-syntax-v0` profile: Ruby gives the path, markers
and advisory autoload constant references; JavaScript and TypeScript give the path and literal
relative, profile and tsconfig-alias imports. Dynamic dispatch, test closure, reverse imports of a
Ruby path outside mapped autoload roots and unresolved bare imports stay unknowns, never results. The receipt is the CLI
`corvint impact` syntax-profile receipt for the same paths, revision and limit, unbudgeted unless
`budgetBytes` is given (NGI-V0-068). The CLI admits more suffixes and the `--base` range form;
`corvint impact --capabilities` lists them. `corvint.impact` stays single-root (MMR-V0-005,
MMR-V0-007): a question that spans an application root and a separate test root uses the CLI
`corvint impact --app-root DIR --test-root ALIAS=ROOT` (ANL-V0-064..067) instead. Absolute paths, `.`/`..`/empty segments, other
suffixes, path escapes, extra fields, and invalid limits are rejected before repository access.

Tool names are unique, case-sensitive, 1–128 characters, and use only letters, digits, `_`, `-`,
or `.`. All `corvint.*` names, in every profile, follow the official 2026-07-28 grammar.

Successful calls return the complete canonical `corvint-mcp-bridge-result/0` receipt both as one text
content block and as the identical parsed `structuredContent`. `READY` means the bounded operation
returned its native receipt; it does not mean Corvint proved the requested proposition.
`ABSTAINED`, `UNKNOWN`, `NOT_OBSERVED`, exclusions, mixed-worktree state, and unsupported intent are
results, not missing fields. Source and diff bodies are never returned.

An impact receipt with native state `OUT_OF_SCOPE` is retained while its bridge wrapper says
`ABSTAINED`/`NOT_OBSERVED` with reason `OUT_OF_SCOPE`; this preserves the exclusion evidence.
Unsupported and output-budget abstentions have a null native receipt.
The canonical bridge receipt is capped at 393,216 bytes and must also fit its duplicated
text-plus-structured MCP response with an 8,192-byte reserve; over-budget output abstains instead of
truncating JSON.

Resources, prompts, subscriptions, completion, sampling, elicitation, tasks, standalone evidence,
dashboard snapshots, frontier/why/live operations, logging, and list-change notifications are not
advertised. Calling one returns method-not-found. `io.modelcontextprotocol/logLevel` does not opt
this profile into logs.

## Limits and lifecycle

- One UTF-8 JSON object per LF-delimited input line; one JSON object per output line.
- Input and output JSON are each at most 1,048,576 bytes before LF; nesting depth is at most 64.
- String request IDs are at most 128 UTF-8 bytes; integer IDs are signed 64-bit. Larger IDs produce
  `-32600` without echoing the rejected ID. Protocol-version values are at most 64 UTF-8 bytes;
  larger values produce `-32602` without reflection.
- Duplicate keys, invalid UTF-8/JSON, over-limit input, and excess depth receive one sanitized
  parse error after the frame is drained. The next complete frame may proceed.
- A valid `notifications/cancelled` cancels only its matching in-flight request and, if it wins the
  output race, no further message for that request is emitted. Clean EOF stops admission and drains
  admitted responses; SIGINT, SIGTERM, and transport failure cancel owned work. Shutdown always
  waits for descendants.
- Progress is emitted only when the request supplied a progress token and a real intermediate state
  exists. No delivered tool currently exposes a truthful long-running progress point, so emitted
  progress execution is `NOT_OBSERVED`. The server sends no unsolicited logging or telemetry.
- Tool listing is deterministic and currently one page. A supplied non-empty or stale cursor is
  invalid; tool receipts are never paged or silently truncated.
- Client-advertised roots are ignored. Repository authority is exclusively the startup `--root`.

## Error interpretation

| Code/state | Meaning | Action |
|---|---|---|
| `-32700` | Invalid, duplicate-key, too-deep, or over-limit JSON frame | Fix framing; the server never echoes rejected input |
| `-32600` | Invalid JSON-RPC request envelope | Send JSON-RPC `2.0`, a string/integer ID, method, and object params |
| `-32601` | Unknown, removed legacy, or unadvertised request method | Re-run `server/discover`; do not assume an older MCP surface |
| `-32602` | Missing request metadata, invalid cursor, unknown tool, or invalid closed arguments | Correct the request; no repository operation ran |
| `-32022` | Unsupported protocol version | Retry only with one of the returned `supported` versions |
| `-32603` | Sanitized unexpected server failure | Treat the operation as unobserved; inspect local process health |
| `ABSTAINED` | Corvint could not safely produce the requested bounded receipt | Preserve the receipt's reason and gaps; widen through an explicit supported operation |

A tool error with `code` `repository-probe-timeout` means the repository probe outlasted its own bound while the request was still live; the
repository may be healthy, so retrying on a quieter host is reasonable (`MCPV0-038`). `repository-unavailable` remains every other repository refusal.

Errors never include request bodies, repository paths, task text, environment values, Git output,
Corvint output, or secrets. stdout is exclusively MCP. Fixed startup, invalid-argument,
repository-unavailable, and fatal transport diagnostics use sanitized stderr; per-request MCP logs
are never emitted.

### Reason-class tool errors (opt-in)

A tool error is normally the closed `corvint-mcp-tool-error/0` object, whose `code` (for example
`repository-unavailable`) never says why a repository was refused. Starting the server with
`--error-profile reason-class` makes every tool error `corvint-mcp-tool-error/1`: the same object
plus a required `reasonClass` from a closed set (`git-filter`, `config-include`, `attributes-file`,
`ref-storage`, `worktree-config`, `config-malformed`, `submodule`, `split-index`, `gitdir-pointer`,
`metadata-unreadable`, `metadata-limit`, `metadata-directory`, `metadata-drift`, `scratch-dir`,
`root-unresolved`, `unclassified`) (`MCPV0-027..028`, decision 0383). The class never carries text;
run `corvint status` for the full reason. Read an unknown value as `unclassified`. The selector
follows the same closed-argv rules as `--tool-profile` and composes with it and with
`--protocol-version 2025-11-25`. Without it, output is unchanged.

## Privacy and authority

Corvint reads local Git and repository evidence under the configured root. It does not fetch or make
network requests. Receipts may contain repository-relative paths and content identities even though
source bodies are omitted; treat them as sensitive local metadata. Discovery and tool-list results
carry different cache hints: root-independent discovery is public and immediately stale; the fixed
tool registry is private with a five-minute TTL. Never infer correctness, exhaustive coverage, or a
clean worktree from a successful transport response.

`corvint.status` does not claim Corvint index freshness, dashboard/evidence availability, or an observed
root digest. Those fields are outside the delivered repository-binding status and remain
`NOT_OBSERVED`.

The server performs no Corvint executable PATH lookup because it calls the Go kernels in process.
Every Git runner it uses, including the `internal/cem/gitrun` runner, is pinned to
one absolute start-time Git executable (`internal/mcp/cli/main.go:164`, `internal/gitstatus`'s
`Pin`/`Executable`, `internal/cem/gitrun.PinBinary`; `MCPV0-016`); a Git binary placed on PATH after
start never runs. This is observed by the adversarial conformance case `TestGitPlantedOnPathAfterStartNeverRuns`
in `conformance/mcp-2026-07-28`, plus
`internal/gitstatus.TestPinFixesExecutableAgainstLaterPathChanges`; both pass.

## Verification status

The required gate is a compiled-process black-box suite under `conformance/mcp-2026-07-28`, plus Go
1.27 unit, race, vet, fuzz, process-cleanup, security, and supported cross-build checks. The focused
unit and compiled-process suites passed locally on 2026-08-23 with Go 1.27.0 and an isolated build
cache. Official 2026-07-28 MCP conformance remains `NOT_RUN`: the official server runner requires a
Streamable HTTP URL and offers no stdio-server target, while this profile deliberately exposes no
listener. The manifest records the exact official schema URL and observed SHA-256
`ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203` but does not execute schema
validation; official-schema-driven conformance is also `NOT_RUN`. The local suite
observes a cancellation race and post-cancel health. Its Unix-gated compiled-process fixture also
passed locally for both INT and TERM: an in-flight fake-Git process and its background child exited
with no surviving descendant. A notification-cancellation-won long-running operation, emitted
progress, and equivalent non-Unix cleanup remain `NOT_OBSERVED`. The local pass is not an official
interoperability or host-compatibility claim.

Do not promote this surface from experimental or mark a host tuple `FULL` until the owning spec's
entire acceptance matrix passes. Disable or remove the child-process configuration to roll back;
the server creates no repository, trace, database, network, or migration state.
