# Design: per-test-durations

## Decisions

1. **Fields.** Each per-test outcome in a receipt gains `durationMs` (integer milliseconds, omitted
   when not measured) and `durationSource` (one of the runner source names below, or
   `NOT_MEASURED`). Both are add-only; older readers ignore them.

2. **Sources.** One source per runner, read from the runner's own output:

   | Runner | Source | `durationSource` |
   | --- | --- | --- |
   | Go | `go test -json` event `Elapsed` on the test's pass, fail or skip event | `go-test2json` |
   | Vitest | JSON report assertion `duration` | `vitest-json` |
   | Jest | JSON report assertion `duration` | `jest-json` |
   | Playwright | sum of result `duration` over attempts | `playwright-json` |
   | pytest | JUnit XML `testcase` attribute `time` | `pytest-junit` |
   | RSpec | JSON formatter example `run_time` | `rspec-json` |
   | Minitest | reporter field from `Minitest::Result#time` (added by this change) | `minitest-reporter` |

   Go subtests carry their own `Elapsed`; a parent test's value includes its subtests and is kept as
   reported.

3. **Not measured.** When the source has no value for a test, `durationMs` is omitted and
   `durationSource` is `NOT_MEASURED`. No value is estimated.

4. **File granularity.** When an outcome is recorded per file rather than per test (a legacy unit
   such as `go:<pkg>`, or a runner fallback that reports one result per file), the outcome carries
   the file's duration with `durationGranularity: file`. pytest file-level selection still reports
   per-test times, so its outcomes are per test.

5. **Invocation time stays separate.** `Invocation.ElapsedMilliseconds` keeps measuring the whole
   launch, including build and setup. It is never divided among tests.

6. **Retries.** A test run more than once records `attemptDurationsMs` and `durationMs` as their sum.

7. **Invalid values.** A negative, non-numeric or out-of-range value (above 24 hours) becomes
   `NOT_MEASURED`, and the receipt lists the test under `duration-invalid`.

8. **Consumers.** Run evidence rows copy `durationMs` and `durationSource` (RVD-V1-006). The cost
   figure (NMS-V1-002) and the fault-injection harness (FIH-V1-012) use only measured values and
   report how many selected and full-suite tests were `NOT_MEASURED`; when that count is above zero
   the figure is labelled incomplete rather than filled in.

## Failure modes

| Failure | Behaviour |
| --- | --- |
| Runner report lacks the field | `NOT_MEASURED`; receipt still valid. |
| Negative or huge value | `NOT_MEASURED`, `duration-invalid`. |
| Report missing | Run INCOMPLETE per the runner rules; no durations. |
| Minitest reporter older than this change | `NOT_MEASURED` for Minitest tests. |
| Cost figure with unmeasured tests | Figure labelled incomplete with the unmeasured counts. |

## Acceptance evidence

- One receipt fixture per runner with measured durations and the named source.
- A fixture per runner where the field is absent, giving `NOT_MEASURED`.
- A retry fixture with attempt durations and the sum.
- A test that `Invocation.ElapsedMilliseconds` is unchanged and not divided.
- The G4 measurement run citing durations from receipts (NMS-V1-002).

## Rollback

Revert the change's commits. Receipts already written keep the extra fields; readers that do not know
them ignore them. Consumers that need durations report them as `NOT_MEASURED`. PTD-V1-001 to
PTD-V1-009 stay listed as retired if the change is withdrawn.

## Open

1. Whether 24 hours is the right upper bound for a valid duration.
2. Whether a Go parent test's duration should be kept when its subtests are also selected, or only
   the subtests'.
