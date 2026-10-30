# Decision 0013 — lift the three standalone query refusals

Date: 2026-09-01. Status: accepted. Authority: repository owner, verbatim instruction "fix the
three refusal paths next" (2026-09-01), given after the Agent Retrieval Bench first runs showed
`corvint query` refusing 29 of 188 tasks before ranking.

## Scope

The instruction accepts exactly these three amendments to
`docs/specs/go-production-kernel-migration-v0.md`:

1. `GPK-V0-028`: the authority-start profile accepts every UTF-8 task and a `--limit` from 1 to
   50 (default 10), returning the Python oracle's packet at every limit: up to two ranked
   instruction documents followed by at most three advisory `learned-path` candidates. This widens
   decision 0007 D5's 1--10 to the oracle's full range because the packet above limit 1 does not
   depend on the limit. The profile still ranks no symbols, so symbol-bearing sources under the
   project-operations path prefixes remain a declared residual divergence. A task that is not
   valid UTF-8 fails closed as `unsupported-query-task`.
2. `GPK-V0-043`: a non-ASCII task is no longer refused as `unsupported-query-task`; Go
   lowercasing reproduces Python's `str.lower`, including the U+0130 special casing, so
   tokenization stays byte-compared with the oracle.
3. `GPK-V0-043`: an `agent-tooling` task is no longer refused as `unsupported-query-intent`; it
   takes the shared `BuildEval` / `EvalQuery` path with `GPK-V0-044` trace consumption.

The three retired conformance refusal rows become oracle-replay rows, and five authority-start
rows are added (`conformance/cli-parity-v0/README.md`).

## Explicit exclusions

This decision does not change the `unsupported-query-authority` or `unsupported-query-trace-state`
refusals, the relevance-floor divergence `DR-0008`, the harness profiles, or any promotion,
cutover, merging, tagging, signing, publication, or release gate.

## Consequences

Every task the oracle accepts is ranked by the Go binary; the reserved error codes remain
documented but unreachable from the standalone command. Delivery status and compatibility remain
experimental.
