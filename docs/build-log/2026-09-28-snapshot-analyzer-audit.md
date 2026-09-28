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
