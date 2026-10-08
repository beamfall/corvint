# 2026-10-08: .NET TRX failure evidence matrix (V1-0600)

## Intent

Ticket V1-0600 recorded four false passes in the pre-integration experimental
VSTest TRX parser: a namespaced `p:outcome="Passed"` alias overwrote a native
`Failed`; a `Passed` row with `ErrorInfo`, summary `ErrorInfo` and an unknown
`FatalError` element all stayed complete. Acceptance asks that all four, including
namespace aliases, refuse or stay incomplete, while actual NUnit, MSTest and xUnit
pass/fail/skip reports stay correctly classified and native counters reconcile.

This change adds proposed `TRE-V0-024` to `docs/specs/test-runner-execution-v0.md`.

## Finding

At base `722904f9` the four reproduced cases already refuse or stay incomplete for
both the VSTest parser (`parseTRX`) and the MTP parser (`parseMTP`). The repair
landed with the runner foundation integration (`084334ed`): the closed TeamTest
grammar in `native/trx_xml.go` refuses namespaced, prefixed, duplicate and unknown
attributes and elements before Go XML projection, and the parsers add
`*_CONTRADICTORY_ERROR`, `*_SUMMARY_ERROR` and outcome problems. The previous
tests covered one VSTest NUnit and one MTP MSTest fixture, with one alias form.
No parser change was needed.

## Decisions

- **One cross-framework matrix.** `TestTRXConflictingEvidence` replaces the two
  single-fixture adversarial tests. It applies seventeen mutations to eight
  captured reports: VSTest NUnit pass and mixed, MSTest mixed, xUnit mixed, and MTP
  NUnit, MSTest and xUnit pass plus MSTest mixed. Each mutation must change the
  bytes, and each unmodified report must stay complete.
- **Alias forms.** The matrix covers a foreign-namespace prefix, a TeamTest-namespace
  prefix, an undeclared prefix, a duplicate `outcome`, `xml:space`, a child default
  namespace redeclaration, prefixed TeamTest elements and a foreign-namespace
  `Output` subtree. Each must refuse.
- **Error evidence.** A `Passed` row with message, empty or stack-only `ErrorInfo`
  must stay incomplete with a contradictory-error problem. Summary `ErrorInfo` is
  inserted into the runner's existing summary `Output` when it wrote one (VSTest),
  else into a new `Output` (MTP), and must yield a summary-error problem. A second
  summary `Output`, `FatalError` and an unknown result child must refuse. Case or
  whitespace variants of `Passed` must yield an outcome problem.

## Evidence

- Failing before / passing after: the original reproduction lived in the
  experimental build tree and is not present in the repository; at `722904f9` the
  repository parsers already refuse it. A mutation check confirms the new matrix
  discriminates: disabling the `Passed`-row `ErrorInfo` check in both parsers makes
  `TestTRXConflictingEvidence` fail (25 failure lines). Disabling only the attribute
  namespace test does not, because the grammar's local-name and duplicate checks
  still refuse every alias form (defence in depth).
- `go test ./internal/testrunner/native` passes, including `TestLiveNativeReports`
  (VSTest counters reconcile with rows) and `TestMTPNativeMatrix`.
- Live VSTest requalification: NUnit 4.3.2/NUnit3TestAdapter 5.0.0, MSTest 3.6.4 and
  xUnit 2.9.2/runner 2.8.2 probes were rebuilt offline from the local NuGet cache
  with .NET SDK 9.0.316 (runtime 9.0.18). `TestVSTestLiveExecution` passed all nine
  mixed/pass/skip cases through the shared executor, each complete with the
  expected states and exit codes. Retained privately at
  `/private/tmp/claude-501/dotnet-td/evidence/vstest-live-qualification.json`.

## Non-goals

No parser, wire, limit or problem code changes. No new TRX producer, VSTest or MTP
version, or framework is qualified.

## Failure modes

An unrecognised but legitimate TRX shape from another producer version refuses, so
a run reports no result rather than a pass. Message text in `ErrorInfo` is retained
as data and never parsed for status.

## NOT_RUN

- MTP live requalification for this requirement: no .NET 10 runtime locally.
- Linux execution.

## Rollback

Restore the two previous adversarial test files and remove `TRE-V0-024` and its
section. Parsers are unchanged, so no behaviour changes.
