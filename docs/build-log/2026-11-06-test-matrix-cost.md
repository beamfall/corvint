# Placement test matrix cost (V1-4742)

The owner authorized reducing repeated fixture cost while retaining placement and real boundary
coverage. The STR-V0-008 algorithm matrix now uses 17 synthetic packages, one split into two named
slices and a catch-all. It retains four run sets, both placements and every shard count 1..16.
Assertions cover exact membership, exactly-once placement, shard bounds, every-shard use, the
below-threshold whole package, split catch-all and all sibling-shard pairs. A separate witness
builds both placements over the complete committed cost/slice universe at 16 shards. Existing
production-table shard-use and FULL/digest tests remain. No production code, skip tag, full-size
corpus test or bounds/privacy test changed. Rollback is reverting the test patch.

## Local evidence

Base: `dba1f4d6c9a33098470ccf57794fb66f989f03c8`. Go `go1.27.3`, local Darwin,
`go test -json -race -p 1 -count=1 -timeout 30m`, via `script/heavy.sh` with the same local reason.
Original receipts and command output are retained in `/tmp/corvint-4742-evidence` in this execution.
The baseline source is the immutable base; the candidate changes only test fixtures/assertions
and this documentation. The source difference is intentional; toolchain, race and package
concurrency flags are identical. Host load is not controlled, and this is one paired observation,
not a CI performance guarantee or CPU attribution.

| Phase | Baseline | Candidate |
| --- | ---: | ---: |
| Placement matrix test elapsed | 22.71 s | 0.13 s |
| Separate committed-table witness elapsed | included in matrix | 0.63 s |
| Placement invocation package elapsed | 23.987 s | 2.031 s |
| Wrapper invocation wall time | 33.17 s | 5.63 s |
| Wall minus package elapsed | 9.183 s | 3.599 s |

The final row is setup/build/wrapper overhead outside Go's package elapsed, not isolated fixture
construction time. Fixture construction and placement remain inside test elapsed. Both wrapper
queue observations were zero. Package elapsed dropped 91.5%; observed invocation wall time dropped
83.0%. Test elapsed values from concurrent commands are never summed. An initial candidate compile
failed on a renamed assertion variable; the successful fixed run is `candidate-fixed.json`.

The two other expensive tests retain their original full-size inputs and are profiled separately;
no scale-only tier or reduction is proposed by this patch.

## Verification and limits

The complete owning `internal/sessiontest` package passed with race enabled (13.706 s package
elapsed). Full-size bounds/privacy passed with race enabled: 32.17 s test, 33.441 s package,
37.13 s invocation wall. Original 16 MiB and 100000-entry requirements remain exercised.

Corvint context and path impact were used before editing, followed by canonical prechange receipts
and dirty affected advice. Installed Core reported `self-binary-stale`; affected retained
`LANGUAGE_FRONTIER` unknowns rather than proving exhaustive scope. No-index calls used their
reported in-memory fallback. Canonical dogfood-change initially could not write shared Git metadata;
the same command succeeded with authorized filesystem access. The enrolled completion lifecycle
was begun after editing with the original immutable base and frozen focused check plan; this late
enrollment does not establish pre-edit enrollment. The canonical lane workflow retains the actual
prechange evidence.
Feature discovery, mutation, external providers and corpus ingestion are not applicable to this
fixture-only change. Repository-wide gate is `NOT_RUN` under the owner's focused-check policy.
Windows vet passed for both the root module and interop module.
Independent review and integration are coordinator-owned.
