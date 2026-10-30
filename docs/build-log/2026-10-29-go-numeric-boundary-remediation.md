# Go numeric boundaries — V1-3179

The owner-requested security triage repairs eight CodeQL numeric-conversion alerts
(#48, #49, #57, #63, #64, #69, #71, #89), the independently confirmed delivery-screen
PR rendering site, and removes the uncertain DSSE capacity addition (#23).
This is a scoped implementation; alert resolution still requires the delivered-head
CodeQL run. The other 50 numeric alerts were independently source-reviewed by the
parent investigation, not reclassified from these tests.

## Contract and implementation

- TCQ-V0-055: hunk/mutant CLI bounds must fit the existing native-int runner API.
  Invalid bounds retain `invalid-arguments`; there is no new workload cap.
- TCD-V0-159/163/188: preserve exact positive int64 PR identities as decimal strings
  in the public delivery screen, batch envelope/rows, close report and hashed merge
  evidence. An external identifier does not inherit the Count cardinality ceiling.
- PMR-V2-006: descendant intervals must fit the existing native-int field exactly;
  unrepresentable values fail the existing malformed-observation boundary.
- CAL-V0-035/037: a persisted leader PID must be positive and representable before
  process lookup. Owner checks retain their ordering; invalid leader identities return
  `SUPERVISOR_LOST`, and absent lanes retain `FENCED`.
- FPK-V0-015: remove only the optional summed DSSE capacity hint. Existing PAE bytes,
  payload types and signature verification stay unchanged. No practical exploit or
  huge-allocation witness was established, and no OOM/DoS protection is claimed.

These clarify existing owning requirements without changing IDs, wire schemas,
platform support, accepted external PR range or persisted data. The reviewed Gate A
plan and authorized adjacent-site addendum remain the implementation boundary.

## Executed evidence

On darwin/arm64 with Go 1.27.3, all five selected package groups passed: workflow,
Tasks CLI, process proof, Tasks store and attestation. Focused vet, Windows vet,
format checking, spec-index tests, requirement definitions and traceability checks
passed. The staged spec generator produced no index changes: requirement identity,
headers and digest claims are unchanged. `make gate` was not run for this scoped work.

New `TestV13179_*` cases cover native bounds and exact large PR identifiers; public
CLI fixtures verify batch rows, ordering, verdicts and unchanged read-only stores.
The gated document test checks independently constructed canonical bytes and digest.
The process fixture admits an unchanged full proof, then rebinds a malformed interval
and receives `process-native-reference-invalid`. Existing DSSE tests preserve the
hand-computed PAE and signing/verification roundtrips.

The supervisor fixture initializes and admits real ATTACH/DISPATCH/BOOT transitions
and a successful heartbeat. Canonical injected attempt records and their request
receipts pass the journal read before lookup assertions. Zero, native overflow and
MaxUint64 perform only the owner lookup and no heartbeat write. The wide positive
alias case is architecture-aware; missing lane, mismatched identity and stale
generation retain their refusal behavior.

A pre-change witness using baseline `9477995d776ddf88d7f6a82790dd9c22d51da765`
`program.go` fails specifically at lookup calls `[99, 0]`,
`[99, -9223372036854775808]` and `[99, -1]`; expected calls are `[99]`.
The fixed source is restored and the selected store tests pass. Fixture development
first failed on stale policy, stale timestamp, seed-claim capacity and a missing
request afterimage; those were setup failures, not product-side-effect evidence.
The final fixture separately asserts admission, reached lookup calls and no mutation.

## Limits and remaining integration

32-bit execution and cross-compilation are NOT_RUN. Native64 preservation tests do
not reproduce the 32-bit PR/interval/runner loss, and Windows vet is type-checking,
not execution. No new platform qualification or live-process signalling is claimed.
Independent implementation review, final CEM/OCM binding and seal, PR integration,
delivered-head CodeQL and native ticket completion remain parent-owned closeout.
Local evidence lives in `/private/tmp/corvint-security-20261029/go-implementation/`
(`final-results.json`, exact check logs and `pid-witness-bindings.json`).

Rollback reverts this task's source, tests and contract clarifications; there is no
store migration, wire migration or external process state to undo.
