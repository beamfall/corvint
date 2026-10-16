# V1-0249: flows S2 run evidence, delivered-but-open audit

Date: 2026-10-16
Ticket: V1-0249 (feature, flows S2)
Requirements: AFU-V1-011..014, AFU-V1-037, AFU-V1-038 (`docs/specs/application-flow-understanding-v1.md`).
No requirement was added or amended.

## Finding

S2 was delivered on 2026-09-25 and 2026-09-27. The closed `docs/BUILD-LOG.md` records it in four
entries: "AFU S2 remainder", "decision 0417", "PWP-V3 every-attempt profile" (a spec draft) and
"AFU-V1-041/042 run registry". The qualified `/3` profile that closes AFU-V1-012 landed in commit
`82cde9bc7` and is recorded in `2026-09-28-issues-167-175-retained-evidence.md`. The ticket stayed
open. An audit at base `08195a48314d9cf0a8bdeeeb92ba8ab932b52c9a` checked each acceptance criterion
against the code and tests. Every criterion is met, so this change adds no code.

The spec had one gap. Its Traceability table had no implementation row for AFU-V1-037, although
the verification matrix already named the tests. This change adds that row and marks it `partial`,
for the following reason. The requirement says the bounds apply "before decode and walk". The byte
bound and the traversal bound do apply first, and the JUnit and `go test -json` adapters count
records, attempts and links during their token walks. The Playwright JSON adapter and the `/3`
receipt adapter count them only after they decode the byte-bounded document (`internal/appflows/runingest.go`,
`parsePlaywrightRun` and `parsePlaywrightReceipt`). The `/3` receipt also has no `maxRunDepth`
traversal bound. The ticket's criterion, that a bound yields incomplete with a named code, holds in
every adapter. The decode ordering is left as a residual and is not fixed here. AFU-V1-036 (S1) also
has no Traceability row. It is outside this ticket.

## Acceptance evidence

1. Adapter fixtures (AFU-V1-011..014). Each of the three run-evidence adapters has a retry-passed
   test, a timed-out attempt and a failed negative control:
   - Playwright JSON: `playwrightRun` (`pays` failed then passed, `slow` timedOut) and a control
     that passed against a `failed` expectation (`TestAFUV1NegativeControlFailed`).
   - JUnit XML: `flakyFailure` (failed then passed), `flakyError` timed out then passed, an error
     with `TestTimedOutException`, and a failed control (`TestAFUV1JUnitAdapterKeepsEveryAttempt`,
     `TestAFUV1JUnitTimedOutAttempt`, `TestAFUV1AdapterObservesFailedControl`).
   - `go test -json`: `TestPay` failed then passed, `TestSlow` panicked with a timeout, and
     `TestDecline` as a failed control (`TestAFUV1GoTestAdapterKeepsEveryAttempt`,
     `TestAFUV1AdapterObservesFailedControl`).

   In every adapter, a control observed against a different expectation does not verify its
   subject.
2. Every attempt is retained (AFU-V1-012). `TestAFUV1PlaywrightProviderKeepsEveryAttempt` reads
   the two-attempt fixture `internal/jstestprovider/testdata/playwright-mixed.json`. For the first
   (failed) attempt it checks the state, retry, duration, failure, anchor and three attachments. For
   the second (passed) attempt it checks the state, retry, duration, an empty failure and no
   attachments. On the receipt wire it checks two attempts and the first attempt's duration,
   failure and attachment count. `TestAFUV1PlaywrightAdapterKeepsEveryAttempt` checks the appflows
   adapter: the first attempt's duration, failure, anchor and attachments, and the second attempt's
   duration and ordinal. Mutation: changing `internal/jstestprovider/playwright.go:170` to keep only
   the current attempt made the regression test fail ("want a flaky test with two attempts, got
   flaky with 1"). The file was restored.
3. Bounds and the secret screen (AFU-V1-037, AFU-V1-038). `TestAFUV1RunEvidenceBoundsIncomplete`
   has ten cases. Together they exceed all five bounds, though not every bound in every format:
   bytes in `go test -json` and JUnit; records, attempts and links in Playwright JSON and
   `go test -json`; traversal in Playwright JSON and JUnit. Each case returns zero records and the
   named `run-evidence-*-bound` code. `TestAFUV1FlowsCLIIngest` checks that the CLI exits 2 with
   `run-evidence-byte-bound` and writes nothing to stdout. The screen runs in `EncodeRunEvidence`,
   and every ingested record passes through it before it is returned. `flows ingest` writes nothing
   until every record has been encoded. `TestAFUV1RunEvidenceSecretsDropped` checks that a seeded
   `ghp_` token makes the ingest fail. Mutation: disabling the `secretscreen.MatchString` check in
   `EncodeRunEvidence` made that test fail. The file was restored.

## Review

One Codex round (read-only) found four inaccuracies in the first draft of this change. All four are
corrected above:

- It cited the wrong entry for the `/3` profile.
- The new row said every bound applied before decode.
- It overstated which attempt fields the regression tests assert.
- It claimed each bound was tested in all three formats.

## Verification

Focused runs at the base, with load averages around 45 to 62 on the shared 12-core host:
`internal/appflows` passed 17 of 17 S2 tests, `internal/jstestprovider` passed 2 of 2, and
`cmd/corvint` passed 2 of 2 (`TestAFUV1FlowsCLIIngest`, `TestAFUV1FlowsCLIPassingControlNeverVerifies`).
Both mutations failed as expected. The ticket's `full-gate` is run by the orchestrator, not by this
lane.
