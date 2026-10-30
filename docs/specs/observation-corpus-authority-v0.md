# Observation Corpus Authority V0

Owner: Russell Lewis
Date: 2026-08-31
Intent status: accepted (decision 0047, 2026-09-04)
Delivery status: experimental
Authoritative inputs: `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `docs/DOGFOOD.md`, and owner acceptance.

## Agent digest
- Claim: Bounded harness observations and sealed labelled-corpus results cannot turn missing evidence into authority.
- Status: accepted (decision 0047, 2026-09-04)/experimental
- Exists: unwired pure library `internal/obscorpus` with capture fixtures, sealed replay, and privacy/bounds tests.
- Blocked on: harness wiring and local retention for `OCA-V0-001..002`; an owner-run sealed corpus.
- Read next: Verified current state; Requirements; Acceptance and traceability.

## User and measurable job

A single repository owner can retain bounded harness observations and run a sealed labelled-corpus
evaluation without converting missing telemetry, labels, or a passing result into authority.

## Verified current state

`internal/obscorpus` is an experimental pure library (no file, network, or process access) that no product
path imports; no harness emits into it, no observation is retained, and no owner corpus is sealed.

## Requirements

- `OCA-V0-001`: each harness observation MUST bind one event/turn identity, immutable revision when
  known, and bounded token and latency fields. A field not directly observed is exactly
  `NOT_OBSERVED`, never zero, estimated, or silently omitted.
- `OCA-V0-002`: observations MUST be local, bounded, source-body-free, and fail open: capture
  failure cannot block the harness event and records `NOT_OBSERVED` with a closed reason.
- `OCA-V0-003`: a corpus run MUST seal its subject identities, source revisions, labels, evaluator
  revision, and scoring rules before results are revealed. Reuse requires an identical sealed
  subject/label/scoring identity; changed inputs are a new corpus.
- `OCA-V0-004`: one owner may label and evaluate a corpus, but its result is owner-labelled evidence,
  not independent validation, promotion authority, or a truth claim about unobserved behavior.
- `OCA-V0-005`: corpus output MUST distinguish `PASS`, `FAIL`, `NOT_RUN`, and `NOT_OBSERVED`; a
  missing subject, label, telemetry field, or verifier result cannot enter a denominator as success.
  A telemetry field is observed only when its value re-binds under `OCA-V0-001`'s bounds; an
  `OBSERVED` status written over a missing, unbounded, or mutable value is not observed.
- `OCA-V0-006`: raw prompts, source bodies, credentials, ambient command output, network upload,
  daemon collection, and automatic agent-memory writes are out of scope.

## Acceptance and traceability

| Requirement | Required evidence |
|---|---|
| `OCA-V0-001..002` | deterministic capture fixtures including `NOT_OBSERVED` and fail-open cases |
| `OCA-V0-003..005` | sealed-corpus replay, reuse refusal, and denominator tests |
| `OCA-V0-006` | privacy/bounds and no-write tests |

Delivered experimental slice (`internal/obscorpus`, tests in `internal/obscorpus/obscorpus_test.go`,
fixtures in `internal/obscorpus/testdata/`):

| Requirement | Delivery | Evidence |
|---|---|---|
| `OCA-V0-001` | partial | `TestCaptureFixturesBindBoundedFieldsAndNotObserved` |
| `OCA-V0-002` | partial | `TestCaptureFromFailsOpenOnErrorAndPanic`, `TestCaptureAllDropsPastCountBound`, `TestEncodedObservationStaysWithinRowBound` |
| `OCA-V0-003` | experimental | `TestSealedReplayFixtureIsDeterministic`, `TestReuseRefusesChangedCorpusInputs` |
| `OCA-V0-004` | experimental | `TestReplayReportIsOwnerLabelledEvidenceNotAuthority`, `TestPackageIsPureAndUnwired` |
| `OCA-V0-005` | experimental | `TestMissingLabelTelemetryOrVerifierNeverCountsAsSuccess`, `TestForgedTelemetryStatusNeverCountsAsSuccess`, `TestSealedReplayFixtureIsDeterministic` |
| `OCA-V0-006` | experimental | `TestCaptureDropsPromptSourceAndSecretText`, `TestPackageIsPureAndUnwired` |

Slice contract. An observation binds one identity (at most 128 bytes of `[A-Za-z0-9._:-]`, secret-
screened by `internal/secretscreen`); an unbound identity makes every field `NOT_OBSERVED` so no
measure is recorded unbound. Each field is `{status, value}` or `{status: NOT_OBSERVED, reason}`
with closed reasons `not-reported`, `capture-failed`, `out-of-bounds`, `revision-not-immutable`,
`unbounded-text`, `secret-screened`, `identity-not-observed`, and `count-bound`. A revision is
observed only as 40 or 64 lowercase hex; tokens are bounded at 10,000,000 and latency at
3,600,000 ms; a batch keeps at most 256 observations and counts the rest; an encoded observation is
at most 1 KiB. A sealed corpus (`oca-corpus/1`) hashes its sorted subjects and immutable revisions,
labels, evaluator revision, and the one scoring rule `exact-label-match`; reuse with any different
identity and results bound to another seal are refused. Replay (`oca-replay/1`) checks, per subject,
result, label, verifier outcome, then match: a missing result is `NOT_RUN`, a missing label or
verifier outcome is `NOT_OBSERVED`, an observed mismatch is `FAIL`, and a match with any telemetry
field `NOT_OBSERVED` is `NOT_OBSERVED`. The denominator is every sealed subject, only `PASS` counts
as success, and the overall verdict is `FAIL` if any subject failed, else `NOT_OBSERVED`, else
`NOT_RUN`, else `PASS`. Every report carries `evidence: owner-labelled`, `authority: none`, and
`independentValidation: false`, and verdicts carry no outcome or label text.

Undelivered: `OCA-V0-001` binding to a real harness event (no harness emits observations) and
`OCA-V0-002` local retention (nothing is persisted) are `NOT_RUN`; no owner-labelled corpus has
been sealed or replayed. `TestPackageIsPureAndUnwired` fails if any non-test file under `cmd`,
`internal`, `tools`, `benchmarks`, `conformance`, or `experiments` imports the package, so wiring
into ranking, learning, evidence, authority, or promotion requires its own accepted change.

## Non-goals, rollout, and decisions

This does not define an authority root, external benchmark, Stop hook, promotion gate, telemetry
service, or policy acknowledgement. Rollback removes an unaccepted implementation and retained
experimental observations; it does not rewrite sealed receipts. Owner acceptance is required before
implementation or use in any promotion/kill logic.
