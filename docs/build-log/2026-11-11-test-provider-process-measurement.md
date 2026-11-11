<!-- SPDX-License-Identifier: Apache-2.0 -->
# Test-provider process boundary: size and latency measurement, no-go for all three (V1-4824)

Lane `clikit`, base `b55635262788`, 2026-11-11. Question (expert-panel restructuring plan step 25):
should the Go, JavaScript or Ruby test-provider code move out of the core `corvint` process? The
rule `RSP-V0-064` fixes was set before measuring: a provider moves only when the core bytes its
packages account for are at least 5% of the core binary and the move adds no more than the
provider's in-process p95 to a call's p95. The affected-selection providers stay in process
whatever the numbers (panel decision: hot path, they never execute repository code).

## What is in process today

The three providers already ship as their own executables. `go list -deps ./cmd/corvint` shows
which of their packages the core links:

| Provider | Core links | Through |
|---|---|---|
| Go (`corvint-go-test-provider`) | nothing (`internal/gotestrun`, `internal/liveverify/{gorunner,godiscovery,parentverify,provider,session}` absent) | none |
| JavaScript (`corvint-js-test-provider`) | `internal/jstestprovider` (and its subpackages) | the document readers `testvaliditydoc`, `doccorpus`, `appflows`, `appmap`, `testsource`, `flowcoverage`: receipt types (`Receipt`, `TestOutcome`, `Attempt`), state constants, `EncodeQualified`, `ParsePlaywrightJSON` |
| Ruby (`corvint-ruby-test-provider`) | `internal/rubytestprovider` | the same readers: `Document`, `Example`, `BlobID`, `Validate`, projections |

So "in process" means the core decodes and projects provider documents with the provider's own
types. "Out of process" would replace those types with a call to the provider executable per
document.

## Method

Host: Unraid, Linux 6.18.58 x86_64, Intel Core i7-8700K (12 threads), 31 GiB RAM, load average
1.3 to 1.7 during the runs; Go `go1.27.3` (`GOTOOLCHAIN=local`).

- Size: each binary built as the release builder builds it (`tools/release-binary/build.go`:
  `CGO_ENABLED=0 go build -trimpath`, unstripped, no stamps) at the base commit.
- Core attribution: `go tool nm -size` on that `corvint` binary, summing every symbol whose name
  contains the provider's package path (functions, type descriptors and data). Shared dependencies
  are not attributed, so this is the most a move could save from those packages themselves.
- Latency: a temporary test in each provider's `cmd` package (not committed), run once per
  provider through `script/heavy.sh`. Each series is 2 discarded warm-up calls, then 30 timed calls
  (`time.Now` around the call); p50 and p95 are nearest-rank. The fixed call is the provider's
  `--help` dispatch, the same code both ways: in process, the package function `main` calls
  (`runOrTestRun` for Go, `companionHelp().Serve` for JavaScript, `run` for Ruby) with discarded
  output; out of process, `exec` of the built binary with `--help` and discarded output. It needs
  no toolchain, browser or repository, so the difference is the process boundary itself. For
  JavaScript the core's own in-process work is also timed on the checked-in fixtures
  `internal/jstestprovider/testdata/playwright-mixed.json` and `vitest-mixed.json`. Two full runs
  were taken; both are listed.

## Numbers

Binary sizes (bytes):

| Binary | Size |
|---|---|
| `corvint` | 40,191,465 |
| `corvint-go-test-provider` | 7,815,458 |
| `corvint-js-test-provider` | 12,909,533 |
| `corvint-ruby-test-provider` | 6,587,059 |

Core bytes attributable to each provider's packages:

| Provider | Symbols | Bytes | Share of `corvint` |
|---|---|---|---|
| Go | 0 | 0 | 0% |
| JavaScript | 135 | 111,734 | 0.28% |
| Ruby | 29 | 23,280 | 0.06% |

Per-call latency, 30 calls each (run 1 / run 2):

| Call | p50 | p95 |
|---|---|---|
| Go `--help` in process | 5.3 µs / 2.4 µs | 17.9 µs / 11.7 µs |
| Go `--help` exec | 5.78 ms / 5.01 ms | 6.53 ms / 5.18 ms |
| JavaScript `--help` in process | 106 µs / 101 µs | 495 µs / 362 µs |
| JavaScript `--help` exec | 5.60 ms / 4.88 ms | 6.79 ms / 5.03 ms |
| JavaScript `ParsePlaywrightJSON(playwright-mixed.json)` in process | 695 µs / 680 µs | 1.16 ms / 0.86 ms |
| JavaScript `ParseVitestJSON(vitest-mixed.json)` in process | 12.7 µs / 12.6 µs | 20.7 µs / 24.9 µs |
| Ruby `--help` in process | 20.0 µs / 14.9 µs | 33.1 µs / 43.4 µs |
| Ruby `--help` exec | 4.24 ms / 3.96 ms | 5.76 ms / 4.53 ms |

## Decision per provider

- Go: no-go. The core links none of the Go provider's code, so nothing can move; the provider is
  already out of process.
- JavaScript: no-go. Moving would save at most 111,734 bytes (0.28%, below the 5% bar) and add
  about 4.8 to 5.6 ms p50 (5.0 to 6.8 ms p95) per call, 10 to 19 times the in-process p95 of the
  same call and several hundred times the core's in-process Vitest parse at p50, so it also fails
  the latency bar.
- Ruby: no-go. At most 23,280 bytes (0.06%) for about 4.0 to 4.2 ms p50 (4.5 to 5.8 ms p95) added
  per call, more than 100 times the in-process p95.

No code changed, so no build tag or rollback path was added (`RSP-V0-064`).

## Limits

One host, one architecture, and a light but non-zero background load; the warm-up calls keep page
cache effects out of the timed exec series. The attribution counts each package's own symbols, not
dependencies only it pulls in, so the true saving could be somewhat larger, but it would need to grow
about 18 times (JavaScript) to reach the size bar. Nothing checks the threshold automatically: if
the core starts importing more provider code, the decision stands until someone remeasures. The
harness source is in the lane's private log directory, not in the repository; the method above is
enough to reproduce it.
