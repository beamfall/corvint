// Package tcq implements Test Claim Qualification V0 (`tcq/0`) as specified in
// docs/specs/test-claim-qualification-v0.md.
//
// TCQ is a deterministic, source-body-free matcher over one verified OCM, only
// its selected claim anchors, their exact target-tree test units, and optionally
// one command plus one JUnit observation. It emits one result per selected
// `(obligationId, claimId)` edge.
//
// Every emitted edge carries `authorityClass: CALLER_REPORTED` (TCQ-V0-006). The
// single positive relation `test-report-matched-v0` is a caller report, never
// execution attestation: per TCQ-V0-046 no consumer may alias it to
// harness-observed or mechanically proved execution, and no input can select
// another authority class.
//
// Per TCQ-V0-045 this package is library and wire conformance only. It adds no
// CLI, no filesystem path API, no artifact persistence, and no runner: every raw
// artifact arrives as caller-supplied immutable bytes, and target source is read
// exclusively through the inherited CEM/OCM repository boundary (TCQ-V0-044).
package tcq
