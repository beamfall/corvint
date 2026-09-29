# Go semantic evidence with gopls

Corvint can add type-resolved definition and reference relationships to a task-context packet.
This is an optional Go integration, currently experimental. It works through the CLI and the
explicit MCP profile below. Core rankings and project authority do not change.

## Setup and CLI

Install the official Go language server explicitly, outside Corvint reads:

```sh
go install golang.org/x/tools/gopls@v0.23.0
```

Put its install directory on your absolute `PATH`. The required live qualification records the
exact version; v0.23.0 is the qualification target for this slice. Consult the
[official gopls support policy](https://go.dev/gopls/) for Go toolchain compatibility and
[workspace documentation](https://go.dev/gopls/workspace) for module layouts.
Dependencies must already be available locally. Corvint disables module downloads and automatic
toolchain switching. It never installs gopls or repairs a workspace during a read.

```sh
corvint context --task 'Find callers and dependencies of this code' \
  --subject internal/example.go --lsp gopls
corvint context --task 'Find callers and dependencies of this code' \
  --subject internal/example.go --lsp off
```

`--lsp off` overrides the legacy `CORVINT_CONTEXT_LSP=gopls` environment setting. If the flag is
omitted, that setting still works. Unknown or duplicate selectors are refused, as are combinations
with `--summary` or `--expand`.

## MCP

Build or use the separately distributed MCP companion and explicitly authorize the profile:

```sh
corvint-mcp --root /absolute/path/to/repository --tool-profile task-review-lsp
```

The profile exposes the task-review tools. Request enrichment only when semantic navigation helps:

```json
{"name":"corvint.context","arguments":{"task":"Find callers and dependencies of A","subject":"a/a.go","lsp":"gopls"}}
```

This is the `tools/call` payload; the client supplies protocol framing and metadata as usual.
Omitting `lsp` or specifying `off` starts no server. Existing default, task-review and flows
profiles do not accept `lsp`; ambient environment variables cannot activate it in MCP.
The MCP process resolves gopls once at startup; install/PATH changes require restarting the host.

## Reading the result

Use `external.providers` for availability and its reason. `external.path_relations` contains
`gopls:uses-definition` and `gopls:referenced-by` edges with blob-pinned endpoints, source positions,
hop provenance and a query digest. Follow those repository paths to the original code. These
relationships are external evidence, not accepted intent or proof that every dependency was found.

`external.query` reports query counts, failures, omissions and stopping reasons. Expansion is
bounded to three seed files, two hops, 64 queries, 32 relationships and a 64 KiB provider record.
There is a 15-second soft query deadline and a 20-second server deadline; total command time also
includes indexing and evidence verification. No daemon is left running. Modified/unindexed files
and paths outside the repository are omitted. A repository-state change during enrichment refuses
the packet rather than attaching evidence to the old observation.

A missing or failing gopls produces an `unavailable` row while ordinary context stays usable.
For `no package metadata for file`, check that the subject belongs to a valid Go module or go.work
workspace, its dependencies are cached, and the installed Go/gopls versions are compatible. An
all-query failure is not a successful semantic result. The older Caddy/etcd frozen benchmark
reported no contributed gopls relations for those repositories. Its release snapshots contain
truncated source and incomplete dependency material, so that run does not qualify semantic
coverage there; see the [V1-0168 build-log correction](build-log/2026-09-29-lsp-bench-snapshot-integrity.md).

## Execution and qualification boundary

The operator trusts the installed gopls and Go toolchain, absolute PATH entries, GOFLAGS, GOWORK
and configured Go cache locations. Corvint does not sandbox those programs or settings. Relative
PATH entries are removed for descendants. Telemetry, module downloads, checksum requests and
Go toolchain downloads are disabled in the child environment. The temporary gopls cache is removed;
the ordinary Go build cache may be populated. Repository, index and trace writes remain prohibited.

Run required live qualification with `gopls` installed:

```sh
python3 script/qualify-lsp.py --output /tmp/corvint-lsp-qualification.json
```

It builds both binaries, exercises committed module and go.work fixtures, verifies a concrete
callee/caller witness, exact CLI/MCP packet parity, default-off behavior and dirty-file omission,
and records versions, binary hashes, packet sizes and wall time. Missing gopls fails the check.
Synthetic semantic qualification does not establish new retrieval recall or agent productivity;
those broader evaluations remain separate. Other language servers and ranking integration are
outside this milestone.

Rollback: select `--lsp off`, unset the legacy environment setting, and use the existing
`task-review` MCP profile. No persisted semantic index or migration needs to be undone.
