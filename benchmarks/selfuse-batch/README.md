# selfuse-batch (experimental, opt-in)

`go run ./benchmarks/selfuse-batch` runs one caller-frozen investigation plan (3..5 `query`/`context`/path-`impact`
views) through a single `corvint batch` at one snapshot, or one standalone process per view
(`--mode standalone`). It consumes only the accepted ordinary contract `SBQ-V0-001..006`
(decision 0052): it never sends `possessed`, refuses a plan that carries it, and defers AT-07
consumption. It never replaces the dogfood range `impact --base` phase. Output goes to a new
directory outside the inspected repository, with fixed raw file names (`batch.*`,
`<route>-<ordinal>.*`); the inspected repository is expected clean (`--allow-dirty` labels working
bytes `NOT_OBSERVED`). Raw receipts are retained as private development evidence outside the public tree, not as a wire contract;
`corvint_processes`, bytes and wall time are engineering measurements of Corvint processes only (Git
children are uncounted), never token or cost evidence. Tests: `go test ./benchmarks/selfuse-batch`. The native tests use controlled Go executables and native process supervision.

Every usable packet is checked against the verb's emitted envelope (`internal/contextindex/receipt.go`
for query/impact, `taskcontext.go` for context) and the tree the consumer captured itself; a batch
answers at one `snapshot` (exact `tree`/`commit`), a standalone or fallback answer carries its own
`revision`, and a drifted packet is unusable rather than silently accepted. On timeout, overflow or
a signal the consumer stops corvint's descendants (Git children live in their own process groups)
before the shared supervisor stops corvint. Audited cooperative `AfterStart` and `BeforeStop`
hooks retain observed PID/start identities and join the bounded observer before returning.
Each hook may add up to one second, separately from the two-second group cleanup budget.
An exited leader or failed snapshot leaves descendant completeness unverified; the receipt
never claims universal containment. Native RSS is recorded as `max_rss_bytes`. On a dirty tree (`--allow-dirty`) the mutation aggregate
is `null`: working bytes are `NOT_OBSERVED`, HEAD and the `.corvint` listing (metadata, not content)
and the ledger are reported separately.

The content-addressed artifact bundle retains the original `raw/` paths, complete streams,
failed/confounded runs, clean comparisons, historical source versions and independent reviews.
Each artifact names a SHA-256 and byte length; `blobs` stores the corresponding base64 bytes.
Every entry was decoded and hash-checked before packaging. The first intermediate repair's
source snapshot was not captured; its output remains historical evidence, not a reproducible
candidate qualification. Current source is the code alongside this document.

The historical Python-driver comparison returned four byte-equal packets: one Corvint process versus four, with
33,645 versus 33,385 raw output bytes. Its negative control retained the same failed operation
on both routes. Recorded intervals exclude initial driver preparation/repository observation and
final receipt serialization; setup, all workers, review, tokens and complete cost are not covered.
The original dirty-tree comparison is confounded. No complete-task, token-cost or AT-07 claim.

Independent review required removing false possession, preserving failed/malformed responses,
containing separate-group Git children on signal/timeout/overflow, and retaining exclusive wait4
ownership on raced exits. The historical focused run passed 47 tests, including 17 consumer tests and
the existing measurement/first-CEM callers. Canonical verification and change binding are recorded
separately; passing this experiment does not complete the owning roadmap evaluation.
