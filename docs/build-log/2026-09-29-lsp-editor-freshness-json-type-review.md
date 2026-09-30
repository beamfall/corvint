# Freshness complete-object JSON identity finding

Independent re-review of cd8a4150bfed74f9f59cf77bfd0d5908b4a73350 resolved the original error/limit
and READY capture findings but retained a P2: Python dictionary equality recursively admitted
bool/int and int/float drift in flexible Core receipt metadata. Frontend-only float overlay versions
also compared equal to integer wire versions. Original failed candidates d82/cd8a and review/repro
receipts remain unchanged. This second and final bounded source repair is explicitly owner-admitted;
no third repair cycle, actual client or qualification outcome is implied.

One structural JSON comparator now preserves exact bool/int/float identity recursively at the whole
frontend/wire response boundary and both complete Core boundaries. It ignores dictionary ordering,
requires string object keys and JSON value types, and rejects all nonfinite floats. No field-specific
exceptions or normalization are introduced. Independent strict packet validation and earlier closed
error/integer limit/current capture checks remain intact. Client flows, owned cleanup and frozen
server cb324729 remain unchanged; frozen fbbef086 and active 21e7 enrollment remain the same.

Required affected evidence precedes checks. Original false passes are reproduced in
/tmp/lsp-freshness-type-repair-before-repro.log, and new regression fails before correction in
/tmp/lsp-freshness-type-repair-regression-before.log. After logs
/tmp/lsp-freshness-type-repair-after-repro.log and
/tmp/lsp-freshness-type-repair-regression-after.log require FAILED with reasons for nested Core
bool/int and int/float drift on complete and READY-A/B paths, frontend-only overlay and nested Core
aliases, and nonfinite values. Positive equal-Core and reordered dictionary controls pass. Existing
context checks and syntax pass; unchanged clients/ownership/server suites are not rerun for ceremony.

All tuples remain UNQUALIFIED. Actual freshness editors, broader conformance, full gate, CEM admission
and promotion remain NOT_RUN/pending independent successor review. No source-server change,
installation, native Tasks write or publication occurred. Rollback preserves the failed cd8a snapshot
and raw evidence; it cannot be described as an admitted freshness validator.
