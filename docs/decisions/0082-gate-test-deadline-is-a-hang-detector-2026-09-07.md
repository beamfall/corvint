# Decision 0082 — the gate's per-package test deadline is a hang detector, set to 30 minutes

Date: 2026-09-07. Status: accepted. Authority: repository owner, verbatim instruction "fix those two
gate timeouts" (2026-09-07), acting on the two entries filed in `docs/agent-memory/tests.md` the same
day.

## Context

`make gate` had two distinct timeout failures. `cmd/corvint` panicked
`test timed out after 10m0s`, exhausting Go's former implicit per-package deadline.
`conformance/cli-parity-v0` failed as
`FAIL query-repository-limit-zero live-oracle: git update-ref ...: process-cancelled: stderr=""`.
The original attribution of that cancellation to the same package deadline was incorrect:
`TestGPKV0002ManifestReplay` starts an independent nine-minute context before the cold candidate
build and passes it into replay. Raising the Go package deadline does not extend that inner bound.

Correction (2026-09-08): isolated console, Portal and OCM lane receipts each report approximately
541 seconds at the replay failure, on different oracle operations. These observations are consistent
with exhaustion of the 540-second inner context. They do not by themselves prove contention as the
cause. The accepted 30-minute package hang detector below remains unchanged, as does the nine-minute
replay context. Neither target originally passed `-timeout`, so both also inherited Go's implicit
10-minute package deadline; that fact did not make their failure mechanisms identical.

## What was measured

On this host at `85fb378`, both packages run to completion with the two heaviest packages contending
against each other and nothing else:

| Package | Wall clock | Slowest test |
| --- | --- | --- |
| `cmd/corvint` | 595.99 s | `TestWorkFinalCheckCaptureBinding` 107.42 s |
| `conformance/cli-parity-v0` | 441.94 s | `TestGPKV0002ManifestReplay` 426.22 s |

`cmd/corvint` passes with **four seconds** of margin against the 600 s default. Under the full
`./...` fan-out it does not: the earlier gate run recorded it `FAIL ... 601.133s`. The two packages
have opposite shapes. `cli-parity-v0` is 96% one test. `cmd/corvint` is 332 top-level tests whose
durations sum to 595.6 s against a 596.0 s wall clock, so its tests run serially and there is no
single offender to shard.

BUILD-LOG's `AT10 replay timeout qualified by measurement` already recorded `cli-parity-v0` at 387 s,
418 s and 491 s on successive waves. The cost is drifting upward against a fixed ceiling.

## The decision

`GO_TEST_TIMEOUT ?= 30m`, applied to `go-test` and `interop-gate`.

The reasoning is what the deadline is *for*. A per-package test deadline exists to stop a deadlocked
package from running forever; it is a liveness check. It is not a performance budget. Using it as one
means the gate reports "this package is slow" as "this package is broken", and the two are not the
same claim — a gate that cannot tell them apart is worse than one that only makes the first.

Setting the deadline just above the measured cost would re-create the failure within a wave, because
the measurements above are of cost that is already growing. 30 minutes is roughly 3x the worst
measurement, which keeps hang detection meaningful — a genuine deadlock costs 30 minutes against a
suite that runs in about 25 — while leaving the documented growth room to be addressed as the
performance question it is.

Per-package cost stays tracked in `docs/agent-memory/optimizations.md`. The specific lever this
measurement exposes is `workProductionFixture` (`cmd/corvint/work_materialization_test.go:220@fe512fab`),
which copies the entire `internal/` tree and runs `git init`/`add`/`commit` once per subcase; the
five `TestWorkFinalCheck*` tests spend about 255 s of the package's 596 s that way.

## Scope held

CI is deliberately unchanged. `.github/workflows/ci.yml` caps `go-product` at `timeout-minutes: 12`,
so a per-package deadline can never bind there; raising one would be inert. Whether a 12-minute job
cap is right for a suite that takes about 25 minutes on this host is a real question, but it turns on
CI's hardware rather than this host's and is not answerable from here.

## Bounded scheduling experiment (2026-09-08)

The root `go-test` recipe adds only `-p 1` to serialize top-level Go build/test-program scheduling.
The selected canonical check remains `make gate`, including vet, archive/cross-build, interoperability,
and documentation stages; the enrolled total check bound remains 3600 seconds. Explicit nested
candidate-build flags, intra-package concurrency, corpus, oracle and both timeout values are unchanged.
No global `GOFLAGS` or `GOMAXPROCS` override is part of this experiment. Inspect actual launch
`GOFLAGS`, `GOMAXPROCS`, `MAKEFLAGS` and `MFLAGS` and preserve their observed values; unreviewed overrides
would change the experiment. A passing gate would support a scheduling-contention explanation without
distinguishing cache, host or ordering effects. Serial execution can increase total elapsed time; a
failure or timeout remains failed evidence. Qualification is pending the combined-target gate.

## Rollback

Revert the `GO_TEST_TIMEOUT` variable and the two `-timeout` flags in `Makefile`. The gate returns to
Go's implicit 10-minute package default; the independent replay context remains nine minutes.
The scheduling experiment can separately be rolled back by removing only root `-p 1` and its
experiment prose. Preserve the factual deadline correction and historical failure records.
