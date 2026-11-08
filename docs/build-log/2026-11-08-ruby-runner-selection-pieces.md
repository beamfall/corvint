# Ruby runner selection pieces: build-only V1-5113

Base: `89372490721bb960ff9261a930c94f8514a2056e` (the accepted integration
base named in the lane charter). Owning intent: decision 0592 and
`docs/specs/runner-selection-plugins-v0.md`, RSP-V0-001 through RSP-V0-005
and RSP-V0-048. This entry records implementation choices, not qualification.

The bound `registry.NewRubyPlugin` factory extends the existing `dynamic.Build`
profiles and native parsers for rspec, minitest, test-unit and rails-test. RSpec
uses its JSON dry run; the other profiles list framework suite/runnable-method
APIs with source locations and the existing owned reporter document shape.
Discovery never intentionally runs test bodies, including on loader failure.
A native filter always runs the configured full scope and reports a superset;
identities are never converted to regular expressions. Core reconciliation owns
missing/extra results, terminal skip reasons and the bounded FULL retry.

The evidence adapter reuses Ruby Coverage's existing collector and isolation
observations. It checks immutable source blob hashes, unions load-time lines into
each test, and retains incomplete attribution. Ruby's dialect supplies lexical
completeness checks; regions conservatively enclose the whole file because the
lexical dialect cannot certify exact method ends. Missing sources, malformed or
capped shapes and computed reads widen. Loaded-feature inventory stays explicitly
absent; coverage alone cannot prove that inventory complete. Core capture must
scrub inherited collector settings, stage the reporter and fresh capture directory,
retain all child artifacts, and supply separately captured T1 trace components.
No plugin writes a record store or supplies trace or qualification authority.
The existing Ruby mutation sub-plugin remains the owner of mutation operators;
this lane adds no mutation engine or qualification harness.

Activation is PARTIAL: `TestRSPV0001EveryExistingRunnerAbsent` requires all three
pieces of every global manifest/registry runner to remain absent. The lane's
explicit prohibition on weakening existing assertions prevents changing that
registration here. The bound factory follows the already integrated script
factory pattern without replacing or duplicating a legacy execution profile.
Integration must resolve the frozen assertion before switching global declarations.
Even the bound factory keeps qualification absent with reason `unqualified`;
passing a qualification boolean alone cannot bypass that absence. All classes
therefore remain FULL. Native data-driven Test::Unit identities that cannot be
mapped exactly also refuse rather than dropping cases.

Fixture tests are written for the six scoped requirements, including evidence plus
core trace assembly. Tests, live tools, qualification records, independent review,
CEM binding and exhaustive gates are NOT_RUN under the build-only charter; the
orchestrator owns those integration steps. Formatting, package vet and focused
spec/citation checks are the only lane verification. No delivery-status or shared
traceability row is changed. Rollback is removal of the new bound factory/adapters
and restoration of the single owned traceability row; legacy execution and records
need no migration.
