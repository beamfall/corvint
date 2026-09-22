# Decision 0213 — shader sentinel reasons follow the analyzer-candidate-profiles list

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`internal/analyzershader` kept its own sentinel reason set: `INVALID_IDENTIFIER`, `LIMIT_EXCEEDED`,
`MALFORMED_INPUT`, `NONCANONICAL_REQUEST`, `UNKNOWN_FAMILY`, and `UNKNOWN_FIELD`. Under that set a
`DIGEST_MISMATCH` or `DUPLICATE_VALUE` sentinel reduced to `ANALYZER_FAILURE`, so a duplicate input
handle could only be reported as a bound rejection that echoed the invalid handles. The
analyzer-candidate-profiles rule allows the sentinel to carry only `NONCANONICAL_REQUEST`,
`INVALID_IDENTIFIER`, `INVALID_PATH`, `DIGEST_MISMATCH`, `UNKNOWN_FAMILY`, `DUPLICATE_VALUE`, or
`LIMIT_EXCEEDED`, and keeps a rejection unbound until every echoed value passes its duplicate checks.

The call:

(a) The shader sentinel reason set is exactly the analyzer-candidate-profiles list. `SHA-V0-005` is
amended to say so; no requirement ID is added.

(b) A frame that fails to decode into the closed request schema (unknown field or type mismatch) is
the `NONCANONICAL_REQUEST` sentinel. It was previously the `UNKNOWN_FIELD` or `MALFORMED_INPUT`
sentinel, and neither is on the list.

(c) A duplicate input handle is the `DUPLICATE_VALUE` sentinel, checked before the envelope binds.

(d) Unchanged: an out-of-bound echoed target, feature, or digest field, or a rejection frame that
would exceed the output ceiling, still emits the `ANALYZER_FAILURE` sentinel. This is outside the
list and is filed in `docs/agent-memory/bugs.md`.

Consequences: `TestDuplicateHandleRejectionStaysUnbound` (`internal/analyzershader/binding_test.go`)
failed before the change. The sentinel literal vectors now cover the seven listed reasons. The frozen
built-CLI vector digests are unchanged.

Rollback: revert the commit. That restores the previous sentinel set, the decode-failure reasons, the
bound duplicate-handle rejection, and the pre-amendment `SHA-V0-005` text.
