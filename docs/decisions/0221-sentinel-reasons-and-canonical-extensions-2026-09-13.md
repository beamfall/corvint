# Decision 0221 — Go canonical extensions bind UNKNOWN_FIELD; shader echo bounds use listed reasons

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`docs/specs/analyzer-candidate-profiles.md` lets the sentinel carry only seven reasons, and requires
an otherwise safe canonical extension to produce the bound `UNKNOWN_FIELD` envelope. Two analyzers
broke this. `internal/analyzergo` emitted an unbound `UNKNOWN_FIELD` sentinel for every unknown
member. `internal/analyzershader` emitted an `ANALYZER_FAILURE` sentinel for an unbounded echoed
target, feature, or digest field, and for a rejection frame over the output ceiling (decision
0213 (d)). The spec did not say where an extension member sits in canonical member order.

The call:

(a) Canonical extension placement follows the HTML/CSS precedent (`canonicalObject`). An unknown
member follows all of its object's schema fields (root, `target`, or an `inputs` element), in
increasing name order. Its value has no whitespace, shortest string escapes, integer-only numbers
(no fraction, exponent, leading zero, or `-0`), and increasing nested object names.

(b) Go: a request whose only defect is such extensions, and whose remaining bytes are the canonical
schema encoding, is the bound `UNKNOWN_FIELD` rejection once the echo envelope passes. When the echo
fails, the request is that echo's listed sentinel. Any other request with unknown members is the
`NONCANONICAL_REQUEST` sentinel. A noncanonical known field takes precedence over an unknown member.

(c) Shader echo bounds map to listed reasons. Nil features are `NONCANONICAL_REQUEST`. More than 64
features are `LIMIT_EXCEEDED`. A bad OS, architecture, ABI, or feature identifier is
`INVALID_IDENTIFIER`. Unsorted or repeated features are `DUPLICATE_VALUE`. An invalid input digest is
`DIGEST_MISMATCH`. A rejection frame over the output ceiling is `LIMIT_EXCEEDED`.

Consequences: `TestSafeCanonicalExtensionBindsUnknownField` (`internal/analyzergo`) and
`TestUnboundedEchoedFieldsUseListedSentinelReasons` (`internal/analyzershader`) failed before the
change. Go vectors that expected the `UNKNOWN_FIELD` sentinel now expect `NONCANONICAL_REQUEST`. The
shader oversize-frame branch cannot be reached while `maxOutput` is 1,048,576 bytes, so it has no
vector. The Python and Kotlin unbound `UNKNOWN_FIELD` sentinels, and the shader rejecting a safe
extension at decode, are filed in `docs/agent-memory/bugs.md`.

Rollback: revert the commit. That restores the Go unbound `UNKNOWN_FIELD` sentinel, the shader
`ANALYZER_FAILURE` sentinel, and the pre-amendment spec text.
