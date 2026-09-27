## 2026-09-26 V1-0363, V1-0364, V1-0365, V1-0372: fenced examples, explicit tracked paths and a once-per-node PageRank degree

V1-0363 (FPK-V0-029). A `Status:` line inside a fenced example set the document's own status. An
unaccepted document showing an example status became `accepted-spec`, and an example above the
genuine field shadowed it. `documentRecord` now follows backtick and tilde fences through
`nextFence` and reads no status from a fenced line or a fence line. FPK-V0-029 is amended.

V1-0364 (ATI-V0-007). `documentCitations` toggled a fence only on a line starting with three
backticks. A citation inside a tilde fence, or after a shorter fence nested in a longer one, became
a live authority trigger. Fences now close as `nextFence` closes them. ATI-V0-007 is amended.

V1-0365 (TCP-V0-004). `contextPathTokens` dropped every token without a dot after its first byte
before the tracked-path lookup. A task naming `Dockerfile`, `Makefile`, `.gitignore` or
`.github/workflows/ci.yml` therefore got no `mentioned` row and no sufficiency path anchor. A token
equal to a tracked path is now admitted whatever its shape. The dot rule still gates basename
resolution, so a bare word never names a file. An unindexed named path keeps TCP-V0-003's low
confidence and `evidence_gap`. TCP-V0-004 is amended.

V1-0372 (TCP-V0-031). `personalizedPageRank` rescanned a node's adjacency for its weighted degree
on every pop and on every push that reached it. A 2000-leaf star rescanned its hub 7905 times in
one walk. The walk now keeps each node's degree for the walk, computed by the same uint64 sum.
Ranks, convergence and push-bound refusals are bit-identical to a verbatim copy of the old walk.
No latency claim is made.

Schema pin. The branch merges the rc1 Git runner hardening change, which already moves the schema
to `corvint-analyzer/88`, so these extraction changes move it once more, to `/89`, with the
audited-input digest `873598aff4e88f0b3b2b7d254fef9ea25e9e32eade7376f9d138bf07b1950187` in
`TestAnalyzerSchemaInputs`. No cmd package pins the schema.

Checks. After merging the Git runner hardening branch (0d4651c6, on origin/main 4c607266),
`go test ./internal/contextindex ./internal/specindex` passes (contextindex 163 s at load ~75).
With the fence, status and path fixes reverted, their three new tests fail. With the degree memo
disabled, `TestPersonalizedPageRankScansEachDegreeOnce` fails (7905 hub scans); the bit-identity
test is an equivalence check and passes either way. The documentation checks and `go vet` pass.

NOT_RUN. `make gate`, `go test ./...`, the interop module, and the full `corvint context` command
with the graph flag on a synthetic star. Only `internal/contextindex` Go code changed; CI runs the
full suite.
