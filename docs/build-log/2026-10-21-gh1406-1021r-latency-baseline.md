# OpenCode qualification: same-window latency baseline and live contention runs (V1-2201, GH #1406)

Lane gh1406-1021r, base `ef2a01db52d5436c262725d136060c9eb95e8580`. Requirement `AHI-172` in
`docs/specs/agent-harness-integration-v0.md`, accepted under the owner's standing auto-accept
(2026-10-11).

## Problem

V1-2192 (`AHI-171`, PR #1413) records host load per timing window and returns
`RETRY_LATER` / `LATENCY_HOST_CONTENDED` for a latency-only miss on a loaded host. Three parts of
GitHub #1406 were still open. There was no same-window baseline, so a slow candidate on a loaded host
could not be told apart from a slow host. The Linux `/proc/loadavg` reader had no test that ran
anywhere except on Linux CI. No live run under synthetic CPU load had been made.

## Decisions

- **Baseline probe.** The probe is a fixed no-op executable, `/usr/bin/true` (or `/bin/true`). It is
  independent of the candidate and of the harness build. A goroutine spawns it every 100 ms for the
  whole of each timing window, so its samples come from the same window as the candidate's. It
  stops when the campaign ends, including when the campaign fails.
- **Comparison.** The window's probe p95 is divided by a declared quiet-host reference of 2 ms. This
  slowdown is at least 1. The AHI-012 limits are multiplied by the slowdown and by a tolerance of 1.5.
  - The `AHI-171` contended outcome still needs a peak load above 0.5 per CPU.
  - When the baseline was observed, the candidate must also be within the scaled limits. Otherwise
    the miss is `LATENCY_REGRESSION`, because the candidate is slower than the host's slowdown
    explains.
  - When the baseline was not observed, the `AHI-171` load rule decides alone and the fields record
    `NOT_OBSERVED`.
  - On a quiet host a miss stays a regression. No baseline reading can excuse it.
- **Reported values.** Each attempt records the probe, its sample count and p95, the quiet reference,
  the slowdown, the candidate's p95 relative to the probe p95 (the relative report that #1406 asks
  for), the scaled limits, the tolerance and the comparison. `metrics.latencyBaseline` repeats the
  decisive attempt. The retry hint names the slowdown.
- **No previous-binary sample.** Sampling the previously qualified binary is a non-goal. No such
  binary is available to the native campaign.
- **Linux reader test.** The `/proc/loadavg` path is a variable, so `TestAHI172ProcLoadavgFixture`
  can run the real reader against a fixture file on every host. The fixture covers load averages,
  the runnable count as the run queue, malformed lines read as `NOT_OBSERVED`, and an unparsable run
  queue.

## Evidence

Tests (`internal/opencodequalification`, via heavy.sh): `TestAHI172|TestAHI171|TestAHI056` pass.
In a mutation check, the verdict with the baseline clause removed fails
`TestAHI172SameWindowBaseline/..._is_a_regression_under_load` (`outcome = LATENCY_HOST_CONTENDED`).

### Live Linux `/proc/loadavg` reading

The run used the local `golang:1.27.2` image under colima, with `--network none` and the package
sources piped in. The kernel was Linux 6.8.0-117-generic aarch64 with 6 CPUs, and the run had `-v`.
The `TestAHI172*` tests and the `AHI-171` live subtest passed. The retained line:

```
contention_test.go:86: live linux reading: map[loadAverage15m:0.14 loadAverage1m:1.81 loadAverage5m:0.43 logicalCPUs:6 runQueue:2 source:/proc/loadavg]
```

### Live OpenCode qualification on this macOS host

All runs used the same build. `qualify-opencode` sha256 `052b1bc5...a0a3` and `corvint` sha256
`75cf16ff...545e` were both built from this lane's tree before commit. Each run was
`qualify-opencode --native-only` on darwin/arm64 with 12 logical CPUs. The campaign uses its own
loopback model provider and needed no network or credentials.

- **The requested host was refused.** `/opt/homebrew/bin/opencode` (v2.1.1) is outside the supported
  range. The harness refused it before any campaign:
  `unsupported OpenCode version "opencode v2.1.1"; supported range is >=2.0.18 <2.1.0` (exit 1).
  The fallback was the closest supported local host:
  `~/.local/opt/opencode-2.0.18/node_modules/@opencode/cli-darwin-arm64/bin/opencode`
  (`opencode v2.0.18`, Mach-O arm64, host sha256 `6759c7f8...96bf`).
- **The host was not idle.** Peer lanes held the 1-minute load average between 45 and 280 on 12 CPUs
  for the whole session. "Idle" below means no synthetic load, not an idle host.
- **The synthetic load.** A bounded Go burner (`burn <workers> 420s`, one busy goroutine per OS
  thread) ran in the same foreground `bash -c` as the qualification. It was killed on
  EXIT/INT/TERM. No burner process remained after any run.

| run | synthetic load | result | decisive attempt |
|---|---|---|---|
| idle 1 | none | PASS, exit 0 | attempt 1 query 565 / lifecycle 421 ms `LATENCY_HOST_CONTENDED` (load 45→76); re-sample 261 / 181 ms PASS |
| A | 12 burners | PASS, exit 0 | 305 / 141 ms PASS (load 78→85) |
| B | 48 burners | PASS, exit 0 | 341 / 166 ms PASS (load 85→104) |
| C | 48 burners, qualification at `nice 20` | FAIL, exit 1 | 2106 / 965 ms; `timed-delivery` failed, with prompt-hook 2000 ms deadline faults |
| D | 12 burners, `nice 20` | FAIL, exit 1 | 1866 / 1082 ms; `timed-delivery` failed |
| E | 12 burners, `nice 5` | PASS, exit 0 | 294 / 213 ms PASS |
| **F** | **12 burners, `nice 12`** | **RETRY_LATER, exit 75** | **both attempts `LATENCY_HOST_CONTENDED`; only the latency check failed** |
| **idle 2** | **none, right after F** | **PASS, exit 0** | **223 / 149 ms PASS, first attempt** |

Equal-priority burners (runs A and B) did not push the candidate past its limits. The macOS
scheduler favours the short-lived hook processes over CPU-bound threads. The contended miss needed
the qualification to compete at reduced priority (run F). Heavier settings (runs C and D) broke the
prompt-hook deadline instead, which is correctly a plain `FAIL`.

Run F output, retained:

```
stdout: {"report":".../ev-load12n12/report.json","result":"RETRY_LATER"}
stderr: native campaign retry-later: latency (attempt 1: query p95 774ms > 500ms, lifecycle p95 539ms > 250ms; attempt 2: query p95 823ms > 500ms, lifecycle p95 530ms > 250ms); inspect .../ev-load12n12/run-962823951; latency missed under host contention, not a regression; retry when the 1-minute load average is below 6.0 (0.50 x 12 logical CPUs); the timing window peaked at 13.80 per CPU; the same-window no-op baseline ran at 60.49x its quiet reference
exit=75
decisive attempt: {"outcome":"LATENCY_HOST_CONTENDED","queryP95Ms":822.8,"lifecycleP95Ms":530.3,"querySamples":21,"lifecycleSamples":44,
 "contention":{"start":{"loadAverage1m":158.04,"loadAverage5m":157.6,"loadAverage15m":109.65,"logicalCPUs":12,"runQueue":"NOT_OBSERVED","source":"sysctl vm.loadavg"},
               "end":{"loadAverage1m":165.56,"loadAverage5m":159.35,"loadAverage15m":111.66,"logicalCPUs":12,"runQueue":"NOT_OBSERVED","source":"sysctl vm.loadavg"}},
 "baseline":{"probe":"/usr/bin/true","samples":242,"p95Ms":120.99,"quietReferenceP95Ms":2,"slowdown":60.49,"queryRelative":6.8,"lifecycleRelative":4.38,
             "scaledQueryLimitMs":45369.75,"scaledLifecycleLimitMs":22684.88,"tolerance":1.5,"comparison":"WITHIN_SCALED_LIMITS"}}
```

Idle 2 output, retained:

```
stdout: {"report":".../ev-idle2/report.json","result":"PASS"}
stderr: (empty)
exit=0
decisive attempt: {"outcome":"PASS","queryP95Ms":222.8,"lifecycleP95Ms":149.3,"failures":[],
 "contention":{"start":{"loadAverage1m":139.33,"logicalCPUs":12,"source":"sysctl vm.loadavg"},"end":{"loadAverage1m":125.62,"logicalCPUs":12,"source":"sysctl vm.loadavg"}},
 "baseline":{"probe":"/usr/bin/true","samples":116,"p95Ms":8.09,"slowdown":4.05,"queryRelative":27.53,"lifecycleRelative":18.45,"comparison":"WITHIN_SCALED_LIMITS"}}
```

## Limits and NOT_RUN

- The 2 ms reference is a declared constant. On this loaded host the slowdown reached 4–89x, so the
  scaled limits were far above any candidate value. Under heavy load the comparison can tell
  contention from regression only when the probe stays fast while the candidate slows.
- The candidate-to-probe ratio moved between 3x and 28x across runs, so spawn cost and hook latency
  do not scale alike. A per-host calibrated reference is a recorded non-goal.
- The OpenCode 2.1.1 host qualification is NOT_RUN, refused as out of range.
- A Linux OpenCode campaign is NOT_RUN. The Linux evidence is the reader's live reading, not a
  campaign.
- A run on a truly idle macOS host is NOT_RUN. Both no-synthetic-load runs passed on a host already
  loaded by peers.

## Rollback

Revert `internal/opencodequalification/baseline.go` and its call sites in `native.go` and
`contention.go`. `AHI-171` then decides alone, as before.
