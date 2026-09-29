# Public LSP quality baseline V0

`public-v0.json` freezes two source witnesses on Corvint's Go tree at
`cf93522e149c754e002426e422270f24c88777fe`. Each has a task, subject, exact gopls
position, manually checked definition path and expected directed relation. The first
witness asks for `lspprovider.Expand` from `lspevidence.Attach`; the second asks for
`lspevidence.Attach` from the context CLI. The direct gopls definition is an independent
semantic baseline for each witness. This public development corpus is intentionally small
and must never be treated as a protected or representative held-out set.

From a clean Corvint checkout whose Go files and module files match the pinned base, install
or point to a compatible gopls explicitly, build the Corvint CLI, then run:

```sh
GOTOOLCHAIN=local go build -o /tmp/corvint-lsp-measure ./cmd/corvint
GOCACHE=/tmp/corvint-go-build-cache python3 script/measure-lsp-quality.py \
  --manifest benchmarks/lsp-quality/public-v0.json \
  --root "$PWD" --corvint /tmp/corvint-lsp-measure \
  --gopls /absolute/path/to/gopls \
  --output /tmp/corvint-lsp-public-baseline.json --repeat 3
```

The script requires a clean checkout, validates that Go source and module files have not
changed from the manifest base, pins the running HEAD, manifest and executable hashes,
and runs upstream-only gopls, Corvint-only context and combined context in rotating order.
It disables Go module/network fetches and automatic toolchain switching. Dependencies and
Go build cache must already be available. The script creates a private direct-gopls cache;
Corvint's provider creates its own disposable cache per call. This cache difference is
reported and must be considered when comparing times. Every child is in an owned process
group with bounded execution and interruption cleanup. Output goes outside the repository.

The report records exact definition resolution, the ranked Core path, the expected
combined relation, Core packet parity, provider state, query/omission counts, wall time and
output bytes. Its nearest-rank p50/p95 with only three repetitions is descriptive; p95 is
simply the largest observation. The upstream, Core and combined gold checks must all pass
for a zero exit; a process failure, timeout or semantic miss exits nonzero after saving the
report. Tool stderr is recorded only as a byte count and digest. Timings are not comparable
across arms because direct gopls retains a cache while Corvint's provider starts fresh each
time. The first retained Corvint run exits 1 because one expected combined relation is
missing; see [`2026-09-29-lsp-public-baseline.md`](../../docs/build-log/2026-09-29-lsp-public-baseline.md).
Publish only a reviewed aggregate.

This corpus measures two Go navigation tasks, not agent task success, editor response time,
precision or recall on real projects generally. Before promotion, freeze a broader labelled
public set and a separately protected held-out set across the exact selected language,
server, repository, workspace and client tuples; include negative/unknown cases. Measure
at least 30 repetitions for each cold and warm latency profile and hold tool/cache policy
constant across arms. Proposed floors for owner review: zero invented committed facts,
stale overlay promotion, unauthorized edits/execution and secret disclosure; at least 95%
recall and 99% precision on admitted known navigation labels; combined agent p95 no more
than twice upstream-only p95 at the same profile/cache state; and an independently scored
task-success gain over the better single arm. These numbers are proposals, not accepted
promotion thresholds or evidence that the current implementation meets them. See
[`lsp-quality-platform-v0.md`](../../docs/specs/lsp-quality-platform-v0.md) and V1-0478.
