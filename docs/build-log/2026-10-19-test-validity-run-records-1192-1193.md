# 2026-10-19 V1-1931, V1-1932: run-time assertion operands and outbound calls

Tickets: V1-1932 (GitHub #1193, run-time `VACUOUS-EMPTY-EQUALITY` and row-citation drift) and
V1-1931 (GitHub #1192, run-time `ENV-UNSTUBBED-CALL`).

## Decision

The static halves (`TSA-V0-169..173`) were already on main. The remaining run-time halves were
NOT_PRODUCED (`TSA-V0-174..176`) because no receipt carried the data. `LPCV-V0-120` to
`LPCV-V0-125` are added to `docs/specs/live-proof-carrying-verification-v0.md` and accepted under
the owner's standing auto-accept of lane requirements (2026-10-11). `TSA-V0-174..176` are amended
to point at them and at `TSA-V0-250..256`.

- Carrier: the `LPCV-V0-080` pattern is reused rather than a new channel. Each record is one
  closed JSON Playwright annotation per attempt (`corvint-assertion-operands`,
  `corvint-outbound-calls`), retained as an attempt-detail member on unprofiled receipts only.
  A profiled receipt that carries one is refused (`external-profile-has-assertion-operands`,
  `external-profile-has-outbound-calls`), so the external profile is unchanged.
- Authority: both records are provider authority. Corvint ships no producer and reads no server
  log. Without a record a test states NOT_PRODUCED with a named reason; the `TSA-V0-174` reason
  `server-outbound-log-not-produced` is kept for the absent outbound record.
- Abstention: decoding is closed, and a missing or null operand size or `stubbed` flag refuses the
  record, so neither is read as empty or unstubbed. A status alone never classifies. A query string
  is refused in an outbound path, so test input is not retained.
- Credit: a vacuous finding sets `blocksCredit`, as `LPCV-V0-083` does. Neither member moves an
  axis, and a classified failure stays a failure.
- Row-citation drift: `corvint tests citations` (`TSA-V0-250..256`, V1-1934) already reads a cited
  symbol at a pinned `@REV` and reports `symbol-not-in-range`, so `TSA-V0-176` is amended to that
  check. No new code was added for it.

## Evidence

- `internal/jstestprovider`: annotation closure, secret screen, Playwright parse and profiled
  refusal tests (`TestLPCVV0120*`, `TestLPCVV0123AnnotationIsClosed`).
- `internal/testvaliditydoc`: `TestLPCVV0121VacuousEqualityReading` (#1193: passed count
  assertion with both sizes 0 blocks credit; seeded, one-sided, failed, absent and invalid cases do
  not) and `TestLPCVV0124UnstubbedCallClassification` (#1192: a 500 with an unstubbed call is
  `ENV-UNSTUBBED-CALL`; a stubbed call, a 422, a passed test and absent or invalid records are not).
  Byte-preservation, axes-unchanged and closed-retention tests cover the rest.
- Synthetic records only. A live Playwright run with a producer is `NOT_RUN`.

## Remaining

- Neither record has an automatic producer, such as a matcher wrapper or a server-log adapter.
  Projects declare the records themselves.
- There is no TypeScript mirror, Vitest or Go session support, and no consumer of `blocksCredit`
  beyond the document member.

## Rollback

Remove the two annotation parses, the two attempt members and the two document members. Documents
from receipts without the records keep their bytes.
