# Parity snapshot discovery (development screen)

The parity runner observes repository state before and after every fresh process. This slice
combines its two Git directory lookups when the result is unambiguous, then performs the same
status, whole-tree, Git/common metadata, mode and byte observations. Known newline roots keep
the original two lookups; ambiguity in resolved metadata or a failed combined lookup falls back.
The fallback can add one bounded invocation: up to 90 seconds of configured timeout instead of 60,
excluding bounded shutdown and subject to earlier caller cancellation. This changes no corpus,
worker default, independent fixture, cache behavior, oracle expectation or production CLI.

[Results](results-2026-09-06.json) retain both candidates, every fixed paired timing (including
warmups), exact raw-stream hashes, failures and qualifications. The original candidate regressed
on newline roots; [its source](candidate-r1.json) remains preserved. The repaired candidate ran
the same 14 cells, two warmup pairs and 12 measured AB/BA pairs per cell. The pooled median of all
168 paired whole-snapshot relative changes is the preregistered statistic; the threshold is 5%.
A full old/new replay is a separate <=105% baseline-time screen with exact ordered output.
These are development observations, not population estimates or complete-task/token savings.

The repaired screen matched every snapshot field and measured a 31.4% pooled median whole-snapshot
improvement. Known newline-root cells returned to approximately baseline timing. One full replay
fell from 379.475 to 359.226 seconds (5.34%, 20.249 seconds), with all 12,350 ordered output bytes
identical, the same subject binary, 133 parity cases and three refusals. The outer supervisor merged
stdout/stderr; the replay itself independently compared the subject's two streams. Canonical gate
and post-commit CEM/OCM results are recorded separately. The process matrix remains `PARTIAL` and
detached-descendant qualification `NOT_RUN`.

[The frozen probe](probe_test.go.txt) contains the exact original snapshot function with its name
changed. [Provenance](probe-provenance.json) pins its source and hash. A caller-owned Go overlay can
map this text file to `conformance/cli-parity-v0/snapshot_probe_test.go`; run the three
`TestLoop4*` tests separately under the repository's bounded process supervisor. Shared helper
implementations must still match the pinned base: changing both sides cannot qualify equality.
Complete outer streams and full snapshots are retained under the local Git evidence path in the
results; projections here preserve timings and qualifications, not a replacement wire receipt.
