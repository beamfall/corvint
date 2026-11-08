# Swift runner selection pieces

V1-5113 builds the XCTest and Swift Testing child of the accepted runner-selection
spec, scoped to RSP-V0-001 through RSP-V0-005. The lane starts at
`89372490721bb960ff9261a930c94f8514a2056e`; the separately requested affected and
initial dogfood baseline is `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`.

## Decisions and boundaries

The compiled-in `registry.NewSwiftSelectionPlugin` factory takes core-captured,
configuration-bound inputs, like the configured script factory. It does not
replace the unconfigured `SelectionPlugin` absence path, modify frozen Build or
Parse wires, or change existing assertions. Its per-runner pieces declare
inventory and evidence present and qualification absent with reason `unqualified`.
Even a session passed `qualified=true` remains FULL until qualification support is
integrated with T6; no plugin can attest its own admission.

SwiftPM's list has native specifiers but no tracked source path. The factory
therefore requires an explicit core source/identity map and admits a list only
when it matches that entire map. This detects duplicate identities, missing
lines including a line-boundary truncation, foreign keys and unknown names;
no file naming convention or Swift Testing location stripping supplies identity.
The core must certify successful uncapped discovery under the same configuration.
These are internal inputs, not a new discovery wire or a live SwiftPM collector.

XCTest uses anchored escaped filters for inventoried method IDs. Swift Testing
uses an explicitly reported FULL superset for requested subsets until its filter
mapping is qualified. Empty selected sets are short-circuited by the existing
core session; calling the plugin command directly with an empty set refuses it.
The existing core run-set planner and reconciliation own mandatory tests,
freshness, new tests, missing results, one FULL retry and sticky divergence.

The plugin reuses platform native parsers with the actual core-captured process
exit and lifecycle facts. Swift Testing skip messages supplement the legacy
observation's empty skip message; a skip with no native reason remains a missing
terminal result. Captured report handles, source bindings and coverage bytes are
copied; they never authorize filesystem reads or process launch.

Collection descriptions enable code coverage and use process/module-specific
LLVM profiles in the core run directory. As in `internal/nativelinecov`, the
core execution/export boundary joins the matching profiles and exports
`llvm-cov export -format=lcov` for the observed tree; the SPI does not launch the
exporter or infer profile completeness from directory contents. Evidence reuses
that package's native line decoder and T1 component types/validation. Every
core-observed test in a shared tree receives its coverage union. Missing,
incomplete, malformed, unsupported, unmapped or capped artifacts publish nothing.
Bounds cover captures, attribution expansion and serialized output.

LLVM DA records cannot establish executable body boundaries or a complete
build/load graph. These components remain absent with reasons
`llvm-cov-lines-no-body-map` and `swift-build-graph-unavailable`. No region parser
or build graph is invented from line counts. Core-supplied trace components remain
separate. The existing Darwin backend keeps them absent with
`darwin-mechanism-unqualified` and `unsupported-platform`; this lane grants no
trace-dependent class. All classes stay FULL through the unqualified declaration.
No runner-specific mutation harness is added: T6 owns qualification.

## Evidence and remaining verification

New fixtures and requirement-named positive and negative tests cover piece
validation, byte-equal inventories, dropped and unexpected results, FULL retry,
filter supersets, pinned executed/unexecuted lines and conservative tree unions.
The requirement-to-test mapping and actual command exit codes are in the lane
handback. Fixtures are not retained qualification records.

Per the owner's final BUILD-ONLY instruction, whole-package tests, live tools,
qualification, independent review and final dogfood bind/check/seal are NOT_RUN
in this lane. Integration owns those checks. In particular the existing
`spec-requirements-check` target runs Go tests; only its non-test generation and
drift checks can run under that restriction. Shared spec traceability rows,
the Agent digest and the status line are deliberately unchanged for integration.

Corvint context was used at the initial HEAD, retaining bounded-source omissions
and the absence of indexed Tasks state. Affected planning used the requested
older baseline; inherited integration paths broaden its frontier, and the
installed build reports itself stale against HEAD. The initial dogfood command
returned incomplete for missing citations, intent scope and outcome inputs; no
binding was claimed. Core and Tasks updater freshness is UNKNOWN because sandbox
process-group/descendant checks failed. No update verification was bypassed.

Rollback removes only this factory and its fixtures/tests. It changes no stored
record format, evidence admission, selector or existing runner execution path.
