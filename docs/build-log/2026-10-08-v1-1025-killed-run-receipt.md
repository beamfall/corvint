# 2026-10-08: Killed-run receipts decode (V1-1025)

## Intent

Ticket V1-1025 (found by the V1-0597/0613 SwiftPM lane) records that a runner-killed run's
receipt, whose TEST phase has `exitCode` `-1`, could not be read back. The change adds proposed
TRE-V0-040 and TRE-V0-041 to `docs/specs/test-runner-execution-v0.md`.

## Diagnosis

The ticket's candidate site, `internal/testrunner/validate.go:118`, is not the decoder. It is
shared `Normalize`, and it correctly reports `runner-exit` so the observation stays incomplete.
The refusal came from `DecodeDocument`, which checks structure with the CEM wire parser
`cw.Parse`. That parser refuses every negative number by design. `cemcandidate.runnerBinding`
used the same parser, so a CEM candidate could not bind a killed-run receipt either.

## Decisions

- **Keep the produced bytes.** Killed runs already record `-1`, and Go's `ExitCode()` returns
  it for signal termination. Receipts with `-1` already exist, so the decoder admits them. A
  nullable or new termination field would change every receipt's bytes and digests.
- **Admit exactly one negative value in one place.** `StructuralBytes` tokenizes the document,
  blanks only the sign byte of an `exitCode` object member equal to `-1`, and passes the masked
  copy to the unchanged strict wire parser for duplicate, closed-field and encoding checks. The
  typed decode reads the original bytes. Any other negative number refuses with the named code
  `negative-document-integer`.
- **A killed run is never complete.** `CheckReceipt` refuses with `killed-run-complete` a
  receipt that claims a complete observation alongside a phase with exit `-1` or a set
  `timedOut`, `interrupted` or `overflow` flag. The producer already ensures this through
  `Normalize`; the decoders now enforce it too.
- The CEM wire parser and the exit-profile domain (0..255) are unchanged.

## Evidence

Failing before, at base `ecfff8e0`, a scratch test decoding a receipt with a timed-out
`exitCode: -1` TEST phase:

```
decode refused: invalid-json: negative numbers are outside the wire integer range
```

Passing after:

- `TestKilledRunReceiptRoundTrip` (`internal/testrunner`) runs an actual executor process that is
  killed at its one-second timeout. It checks that the phase records `-1` with `timedOut`, that
  `Normalize` leaves the observation incomplete, and that the receipt decodes and re-encodes
  byte-identically. It also checks the named refusals: `-2`, a negative `executedCount`, `-1.0`,
  `-1` inside an array, a complete claim with a killed phase, and a complete claim with a
  timed-out phase that exited 0. A string spelling `-1` is still data.
- `TestRunnerBindingAdmitsKilledRunReceipt` (`internal/cemcandidate`) shows the binding admits
  the killed receipt, refuses `killed-run-complete` and refuses exit `-9`.
- Focused packages pass: `internal/testrunner/...`, `internal/cemcandidate` and
  `cmd/corvint-test-runner`, including the historical plan and receipt byte-identity tests.

Live qualification, Apple Swift 6.4 (swiftlang-6.4.0.34.1), macOS arm64. The lane fixture was
copied to private scratch, with `testHang` changed to sleep 600 s without the lane-specific
readiness marker. `corvint-test-runner plan` and then `run` (swift-xctest, `timeoutSeconds` 20)
exited 1 after 20 s. The receipt's TEST phase has `exitCode` -1 and `timedOut` true, and the
observation is incomplete with problems `PROCESS_TIMEOUT`, `timeout`, `no-tests` and
`runner-exit`. The base decoder refused that receipt with the `invalid-json` message above. The
changed decoder admits it and re-encodes it byte-identically.

## Failure modes and limits

- SwiftPM's `xctest` child ran in its own process group and outlived the timed-out run
  (parent 1, `pgid` equal to its pid). It was verified as this run's process and terminated by
  hand. This is the known TRE-V0-008 detached-descendant limit, which this slice does not change.
- A `-1` phase whose producer omitted all termination flags still decodes, and still cannot be
  complete.
- No signal number is retained; the flags are the classification.

## Non-goals

No new receipt field, no change to exit-profile bounds, no change to `internal/cem/wire`.

## Rollback

Revert the commit. Killed-run receipts then refuse again as `invalid-json`. No other plan or
receipt byte, digest or decode result changes.
