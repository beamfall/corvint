# 2026-10-04: testacceptance parent-complete tests under race (V1-0715)

## Finding

`internal/testacceptance` took 953.5s in hosted run
<https://github.com/beamfall/corvint/actions/runs/37198363933> (ubuntu-24.04, 4 vCPU, `-race`,
`-p 1`), the longest package in the suite. The time is in four synthetic pure-join tests:
`TestPTFV0ParentCompleteAttemptOutcome` and its `Bounds`, `NativeForgery` and `Correspondence`
siblings (38 subtests).

Cause, measured on darwin/arm64 with go1.27.1:

- `TestPTFV0ParentCompleteAttemptOutcome` alone (8 subtests) took 118s with `-race` and 8.0s
  without.
- A CPU profile of that run put about 47% of samples in `secretscreen.writerMatches`
  (`regexp.FindAllStringIndex`), reached through `behaviorfalsify.VerifyReceipt` ->
  `EncodeEvidence` -> `safeReceiptBytes`; most of the remainder is race-runtime accounting for the
  same regexp execution.
- `secretscreen.Pattern` scans at about 1 microsecond per byte: 101-113ms for the fixture's
  87,204-byte encoded receipt, 2.15s under race. One `VerifyReceipt` of the 4-attempt fixture
  costs 462ms without race because it screens every retained byte field (raw text, decoded JSON
  strings, key=value pairs) and then the encoded document that carries the same bytes as base64.
- Each subtest verifies the receipt in the test body and again inside `joinedFreshControl`; the
  16-attempt bound cases carry 32 native reports.

The subtests were serial, so the package used one of the runner's four cores.

## Change

The 38 subtests of the four tests call `t.Parallel()`. Each subtest already builds its own
fixture from `parentJoinFixture` and shares no mutable state; the race detector run of all four
passes. No case, bound (attempts 1 and 16), forgery mutation or assertion changed, and the
explicit fixture `VerifyReceipt` precondition stays.

The screening cost itself is production behavior and security-sensitive. It is not changed here
and is filed as V1-0722.

## Evidence and limits

- Local, 12 cores, host under unrelated load: the four tests under `-race` took 442.5s wall for
  1266s of user CPU with parallel subtests. Serial wall time is at least the CPU time.
- Hosted before: 953.5s (run 37198363933). Hosted after: `NOT_OBSERVED` until this change's own
  CI run; read it from the shard log's terminal `internal/testacceptance` package outcome.
- The 4-vCPU hosted bound is a ceiling of about 4x on these tests; other tests in the package are
  unchanged.

## Rollback

Remove the four `t.Parallel()` calls.
