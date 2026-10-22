# Test providers and OCM sweep: synthetic-only REUSED, v1 trace identity, NATIVE-only obligations

Date: 2026-10-21. Lane: claude/testprov-1021w (base f29ddc0db). Tickets: V1-1031 and V1-1035
delivered; V1-0571 partly delivered (trace identity only); V1-1077, V1-1079, V1-1235, V1-1536,
V1-0585 and V1-0332 not delivered. `TCN-V0-016` in `docs/specs/test-consolidation-planner-v0.md`
and the `TOL-V0-001` amendment in `docs/specs/corvint-tasks-obligation-ledger-v0.md` are accepted
under the owner's standing auto-accept (2026-10-11).

## Problem

- V1-1031: every `REUSED` test in the consolidation planner came from synthetic witnesses. No test
  drove real provider documents through `Run`, so nothing showed whether a real receipt can reuse.
- V1-0571: the dashboard snapshot conformance kit derived a v1 trace row's `trace_id` with Go
  `strconv.Quote` escaping. The producer (`internal/trace/codec.go`) hashes ASCII JSON with `\uXXXX`
  escapes. A v1 row whose task has non-ASCII text failed with `SNAPSHOT_IDENTITY`.
- V1-1035: the Core reader (`internal/taskman`) admitted the `obligations` member on IMPORT and
  shadow-overlay records. The Tasks codec refuses it there (`internal/tasks/ticket/obligations.go`).

## Decisions

- TCN-V0-016 records that `REUSED` is synthetic-only in V0. A real qualified external-profile
  receipt binds with freshness UNKNOWN `retained-external-app-lifecycle-unverifiable`, a Go session
  binds UNKNOWN `retained-session-identity-unverifiable`, and an unprofiled or freshness-profile
  receipt carrying identity is refused. `TestRealProviderDocumentsNeverReuse` keeps this pinned. It
  asserts only a non-empty rejection for the external receipt, because an incomplete fixture fails
  the qualified prerequisites and projects as `witness-not-associated`.
- The kit derives every trace schema's `trace_id` with `canonicalTypedTrace`, the producer's ASCII
  encoding. `TestTraceV1NonASCIITaskMatchesProducerIdentity` uses a producer-computed digest.
- `decodeTicket` refuses `obligations` unless the source kind is NATIVE and `shadowOverlay` is not
  true. TOL-V0-001 names the rule for both readers, and one test per side pins it.

## Evidence

- Fail-before: blanking the external-lifecycle reason in `internal/testvaliditydoc/discover.go`
  failed `TestRealProviderDocumentsNeverReuse`; the trace test failed with `SNAPSHOT_IDENTITY` before
  the kit fix; the Core obligations test decoded the IMPORT record before the reader fix.
- Focused packages pass: `./internal/testplan`, `./conformance/dashboard-snapshot-v0`,
  `./internal/taskman` (`TestTOLV0001`), `./internal/tasks/ticket` (`TestTOLV0001`), plus the
  `./internal/tasks/cli` and `./internal/tasks` traps.

## NOT_RUN and residuals

- V1-0571: the ADVISORY versus ADAPTER_QUALIFIED authority and zero-bucket adjudication stay open.
  The frozen fixtures disagree (`valid-observed.json` against `four-invalid-legacy-members.json`),
  and LOD-V0-035 freezes the snapshot bytes, so this needs an owner decision.
- V1-0332 needs a decision on the decisions index; V1-1536 needs a live GoogleTest run. V1-1077,
  V1-1079, V1-1235 and V1-0585 were not started within the lane budget.
- `corvint affected` tier 2 is NOT_RUN locally and is left to batch CI.

## Rollback

Revert the lane commits. That removes TCN-V0-016, restores the kit's v1 basis, and returns the Core
reader to admitting `obligations` on any source kind. There is no state migration.
