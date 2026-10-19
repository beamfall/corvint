# 2026-10-18 V1-1887, V1-1889: intra-example stub path and value origin in test-validity

Tickets:
- V1-1887 (GitHub #1171): STUB-ON-PATH and STUB-NEVER-INVOKED.
- V1-1889 (GitHub #1173): ASSERTED-VALUE-NOT-FROM-SUBJECT.

## Decision

`TSA-V0-120` to `TSA-V0-130` are added to `docs/specs/test-source-analysis-v0.md`. They are
accepted under the owner's standing auto-accept of lane requirements (2026-10-11). `TSA-V0-131..139`
stay free. The rules extend the existing RSpec (`TSA-V0-062`) and Jest/Vitest (`TSA-V0-073`) example
readers. They add no parser and no output member.

- Every finding is advisory (TSA-V0-120). An intra-example trace never sees the code under test, so
  it cannot prove that a stub replaced the behaviour under test. A wrong INELIGIBLE would refuse a
  sound witness, and the tickets forbid that. Hygiene and strength are unchanged.
- Each rule follows values only inside one example: its body, the hooks and `let`s that apply to
  it, and its groups' or describes' declarations, up to 4 bindings deep. Any unresolved binding, too
  deep a chain or unbalanced text gives no finding (TSA-V0-129).
- Receivers are subject, boundary, first-party or unresolved (TSA-V0-121, TSA-V0-123).
  - RSpec boundaries are a closed constant-root list plus the enqueue and delivery methods.
  - Jest/Vitest boundaries are non-first-party modules, the TSA-V0-079 boundary modules and globals,
    and `Date`.
  - Only first-party receivers can be on-path. Subject stubs stay with DESCRIBED-CLASS-STUBBED and
    JS-SUBJECT-MOCKED.
- STUB-ON-PATH needs one of two pieces of evidence (TSA-V0-122):
  - an assertion that calls the stubbed method itself;
  - an action line that names the subject, plus an assertion on a subject-derived value that expects
    the stub's informative return value.
  Empty, nil and boolean returns are not evidence.
- STUB-NEVER-INVOKED is read statically only (TSA-V0-124). The silent stub must be on a fresh object
  that nothing in the example's scope names again.
- The run-based form needs a receipt that records calls, and none exists for RSpec or Jest/Vitest.
  Playwright route stubs cover only the HTTP boundary. That form stays the named residual
  TSA-V0-125.
- ASSERTED-VALUE-NOT-FROM-SUBJECT covers three shapes:
  - values only the test built and handed to nobody (TSA-V0-126);
  - a DOM the test wrote with nothing in scope that renders or calls code under test (TSA-V0-127);
  - enqueue and call expectations that the test's own call meets (TSA-V0-128).
  Persisted reads (`reload`, finders), handed-over values, subject results and constant expectations
  (JS-CONSTANT-EXPECTATION) are excluded.
- Cost: the readers mask and tokenize each RSpec line once per file, map JS tokens to their example
  once per file, and cache subject names per group or describe. A synthetic file with 600 examples
  reads in about 0.1 s (RSpec) and 0.4 s (JS, mostly the existing mock reading).

## Residuals

- The run-based STUB-NEVER-INVOKED needs a call-recording receipt input.
- Cross-module reachability from the action to a stub (a dependency-graph reading) is not built.
- `receive_messages`, `receive_message_chain`, a `jest.fn()` assigned onto an object, and module
  factory mocks are not read as stubs.

## Evidence

- Focused fixtures: `TestStubOnPathRSpec`, `TestStubOnPathJS`, `TestStubNeverInvokedRSpec`,
  `TestAssertedValueNotFromSubjectRSpec`, `TestAssertedValueNotFromSubjectJS`,
  `TestStubOnPathMessages` and `TestFlowFindingsAdvisoryAndDeterministic` in `internal/testsource`.
- Live qualification on a real suite: NOT_RUN.
