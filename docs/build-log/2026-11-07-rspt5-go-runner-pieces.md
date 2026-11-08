# Go runner selection pieces: build-only handoff

- Intent: V1-5107; RSP-V0-002, RSP-V0-005, RSP-V0-033 through RSP-V0-036,
  and RSP-V0-040 step 3; decision 0592.
- Base: `89372490721bb960ff9261a930c94f8514a2056e` (the lane charter's integration
  base, superseding the earlier common base for this slice).
- Outcome: implementation and regression fixtures prepared; execution, independent review,
  live qualification, qualification records and production enablement are NOT_RUN. The owner
  explicitly assigned testing and review to integration and prohibited binding in this round.

The configured Go factory implements the existing T3 SPI without changing its wire shapes or
legacy registry behavior. One instance binds one package and one explicit CI build constraint;
the caller must enumerate the CI matrix and cannot reuse a capture across configurations. Native
discovery uses captured `go test -json -list=.` plus the active test files in `go list -deps -test
-json`; core capture must attest successful, uncapped output. Source bytes are regular Git blobs,
not worktree reads. Unsupported ambient GOFLAGS, missing identities, malformed or capped artifacts,
missing test declarations and dependency closure failures refuse discovery or leave evidence
incomplete. A module must start at the supplied repository root; nested-module remapping is not
inferred.

Coverage reuses the linecov profile builder and T1 adapter independently per isolated test, so
one test's instrumentation cannot manufacture another test's unexecuted ranges. Missing coverage
stays absent. Loaded files include native sources, embed inputs and test sources from the captured
dependency graph. External nonstandard dependencies stay incomplete. Go body extraction uses the
same standard Go parser as linecov's existing body selection, preserving physical positions,
excluding braces and declarations, and recording nested literal parents. It does not reinterpret
the symbol-only region hook as a body map. T1 Assemble joins the core's trace without upgrading a
missing component or changing attribution.

The Go mutation piece exports deterministic body candidates from its existing operators plus
package declaration literal replacements using the same literal operator. It does not sample or
execute mutants. Caps refuse the whole population, rather than expose a biased prefix. Corpus
and execution-model declarations expose FULL/unqualified for every claimed class. There is no
qualification flag, fabricated statistic or new harness; T6 owns retained results and admission.

## Unresolved contract question

The charter requires `go test -list` inventory for `ginkgo-v2` as well as `go-test`. That command
lists Ginkgo's Go wrapper, whereas the existing native Ginkgo profile requires individual
`GoWrapper::SuiteDescription::FullText` identities. A wrapper-only inventory cannot claim that it
enumerates those identities. The factory therefore refuses Ginkgo spec inventory with a named
reason. The orchestrator must specify the captured Ginkgo discovery artifact/command and its
binding to the CI constraints (for example, whether native dry-run reports are the intended input)
before that part can be implemented without inventing behavior. Existing execution remains
unchanged. V1-5107 is PARTIAL on that external contract clarification and the deferred integration
qualification, not a delivered qualification claim.

Rollback: remove the new configured adapter, body extraction and mutation exports. No stores,
legacy plans, frozen CLI output, existing assertions or qualification artifacts changed.
