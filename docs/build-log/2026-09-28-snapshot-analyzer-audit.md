# Snapshot freshness analyzer audit repair

CI for issues #326–#330 exposed `TestAnalyzerSchemaInputs/IDX-SNAP-V0-017`:
the snapshot freshness change modified a pinned analyzer input while retaining
`corvint-analyzer/93`. The guard intentionally includes consumer changes.

Advance the analyzer identity to `/94` and refresh the source audit digest.
Existing `/93` packs are conservatively invalidated; extraction and encoding
remain unchanged. The existing changed-analyzer refusal test covers the runtime
boundary. Rollback restores the version and audit pins together with the
snapshot change.

Focused analyzer, snapshot, and freshness tests passed on Go 1.27.1. Required
GitHub CI will verify the final pushed revision before merge. This mechanical
maintenance follow-up does not claim the earlier sealed CEMs cover its new commit.

The same CI run also found trusted-profile integration mismatches. Normalize its
spec header, digest and README to the indexed experimental status, and show the
inspection usage with the global root option. Preserve the native-routing guard
for all default entrypoints; the two explicitly trusted entrypoints exempt only
the fixed probe and isolated Git text helper edges. Independent review rejected
an initial direct-root exception; the final helper extraction keeps new direct
execution detectable. Focused help, spec-index, routing and real pinned MkDocs
regressions passed after repair; independent review found no remaining issues.
