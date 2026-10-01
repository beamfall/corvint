# New E2E test assessment: attached order evidence and change-request body

Date: 2026-10-01
Human intent: GitHub #394 (part of #388), native V1-0541.
Contract: `docs/specs/new-e2e-test-acceptance-v0.md` NEA-V0-008 and NEA-V0-009, proposed/experimental.
Public base: `ff3da727e95a1999cbc4a58a471cdd498693f902`.

## Decision

The 2026-09-30 slice retained file-order probes only as anonymous runs, and its body listed test
verdicts alone. The issue asks for the flaky test's order-dependence evidence to be attached and for
the verdict to render into a change-request body without other tools. This slice makes three changes:

- Each assessment now carries its repeat counts, duration range, worst cleanup fact and order
  evidence. Isolation runs for another test do not count against its cleanup.
- A test with mixed repeats is run alone twice, through a title-anchored `--grep` on its own file.
  File reversal cannot vary order within one spec file, which is the common case for a new test and
  the shape of the live fixture. Isolation separates order dependence (`failures-not-reproduced-in-isolation`)
  from nondeterminism (`nondeterministic-in-isolation`). Single-file probes are labelled `not-varied`
  rather than counted as an order comparison. Two runs are suggestive, not a proof; the status names
  what was observed, and the verdict stays rejected.
- The body is a fixed Markdown record of validated values only: verdict, product and test commits and
  trees, environment, Corvint build, executable and request digests, a per-test table, reasons and
  unknowns. Any value that fails its closed shape renders `UNVALIDATED`. Test titles never enter it.

`accepted` remains unreachable. Per-test freshness is still UNKNOWN (V1-0556), so stable asserting
tests stay blocked. #394 remains open for that capability.

## Evidence

- Focused: `go test -count=1 ./internal/testacceptance ./cmd/corvint-tests-accept` and `go vet` pass.
  `TestNEAV0008AttachedOrderEvidence` covers all four isolation statuses, attribution of cleanup to
  the isolated test only, and grep anchoring with trailing tags. `TestNEAV0009ChangeRequestBody`
  covers bindings, the per-test row and `UNVALIDATED` substitution of an injected reason and build.
- Live (darwin/arm64, Node v22.23.3, Chromium 153.0.8010.12, Playwright CLI 1.63.0 fixture, 36.5 s):
  six runs (two repeats, two file-order probes, two isolation runs of `flaky`). Results: `stable`
  blocked, `nonasserting` rejected (control survived, Strength SURVIVED), `flaky` rejected with
  isolated states `passed, failed`, so `nondeterministic-in-isolation` and file order `not-varied`.
  The fixture server alternates per request, so that classification is the expected one.
- The order-dependent branch has synthetic coverage only; no live order-dependent fixture was run.
- Not run: `make gate`, per the owner's focused-test preference.

Rollback: revert this change. The request schema is unchanged; report consumers that ignore the new
`repeats`, `cleanup`, `order` and `test_id` fields are unaffected, and the body text changes shape.
