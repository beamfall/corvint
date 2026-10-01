# V1-0168: frozen Caddy/etcd LSP benchmark interpretation

Date: 2026-09-29. Owner: Russell Lewis. The V1-0099 off/on run remains a valid measurement of
unchanged Core rank lists and descriptive latency under its recorded host load. Its Caddy and etcd
rows do **not** demonstrate useful gopls evidence: the capture had zero contributed relations on
13 Caddy and 23 etcd runs. The capture predates `failed_queries`, so zero relations cannot be
retroactively converted into a count of successful or failed server queries. No result is rerun
or promoted here.

The [official v2_edit2ripple release](https://huggingface.co/datasets/eyuansu71/agent_retrieval_bench/tree/main/releases/v2_edit2ripple)
archive `agent_retrieval_bench_v2_edit2ripple.tar.zst` was checked against its release SHA-256
`a174196d69b531d176a65c76fea928b3f1c893710baa4efccc48e901ff404b2c`.
The bench's `writeChunkRows` reconstruction was repeated for exact corpus records
`caddyserver/caddy@aed1af59763d54520a9c72f1fd0222d43904ebfd` and
`etcd-io/etcd@5485c710674b5990f62e83984d180b4b552fa52c`, with temporary Git commits.
Only `kind: file` rows were written, as the bench does; no repository source or Go dependencies
were downloaded. The local repro used Go 1.27.1 and gopls v0.23.0 with `GOPROXY=off`,
`GOSUMDB=off`, `GOTOOLCHAIN=local`, private Go/gopls caches and telemetry off. The original
measurement used gopls v0.22.0; this is a snapshot-integrity investigation, not a reproduction
of that exact runtime tuple.

| Snapshot | Original corpus observation | Offline repro |
| --- | --- | --- |
| Caddy | 600 file rows; 122 end in `...[truncated]`, including 113 Go files, `listeners.go` and root `go.mod`; no `go.sum` row | `go list -e -json .` refuses `go.mod` with an unterminated `require` block; `gopls definition listeners.go:45:26` returns `no package metadata for file` |
| etcd | 1,322 file rows; 208 end in `...[truncated]`, including 166 Go files and `server/embed/etcd.go`; no root or `server/go.sum` row; root and nested module manifests exist, but no `go.work` row | From `server/embed`, `go list -e -json .` is incomplete; `gopls check server/embed/etcd.go` reports missing dependency checksums and a parse error at the truncation boundary. A current gopls definition on one etcd symbol succeeds, so an all-query metadata failure is **not** established for etcd by this repro. |

The release material is lossy for language-server qualification even though it is the exact
frozen retrieval corpus. Caddy's malformed module directly explains its metadata failure.
Etcd's nested-module layout and absent checksum files constrain offline package loading, while
truncated queried source prevents faithful semantic comparison. They do not prove which failure
produced each historical zero-relation row. The old `loaded with 0 relations` label means only
that the server initialized under the old capture logic. For Caddy/etcd, read TCP-V0-046 as
**measured Core parity and cost with no gopls relation contribution in that run**, never as
evidence of semantic coverage or an LSP quality gain. The same caveat applies to the one Gin
zero-relation row until separately investigated. The remaining benchmark rows retain their
original observational limits.

An honest future semantic qualification needs complete, pinned source and dependency material,
an explicit workspace/module mapping, the exact Go/gopls/environment tuple, and direct query
success/failure counts. That is separate from the unchanged-result parity gate and the public
LSP quality baseline. Rollback of this interpretation change is a documentation revert; no
runtime, wire format or benchmark data was altered.
