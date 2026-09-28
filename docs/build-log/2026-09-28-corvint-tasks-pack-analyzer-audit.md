# Pack analyzer audit at agent lease integration

The required CI run for PR315 exposed a missed analyzer audit update in S8's reviewed pack-only
snapshot loader. `TestAnalyzerSchemaInputs/IDX-SNAP-V0-017` reported source digest drift on
`48f7e114ef855f91e7258c0be0ff989b479aa366`. Its completed failure log was retained; the remaining
jobs in run36413553809 were cancelled before repairing the source.

The deliberate audit review covers `LoadContextPackSnapshotDeferred`, the explicit pack-only
branch of `readSnapshotIndex`, and masking the new load flag from table selection. The existing
source audit conservatively includes consumer-only changes. `analyzerSchemaID` and its test pin
advance from `corvint-analyzer/90` to `/91`; the digest is recomputed over the exact sorted
production-source set already named by the audit. Old /90 packs are invalidated. Default snapshot
identity remains executable-specific, and a rebuilt executable with the same analyzer schema and
Go toolchain can still reuse an explicitly enabled pack.

`go test -count=1 -timeout 30m -run TestAnalyzer ./internal/contextindex` and the complete
`internal/tasks/scopes` tests pass. They cover the source audit, cross-executable pack reuse,
changed-analyzer refusal, corrupt-pack refusal and Tasks scope abstention. Independent review of
this repair passed. It changes no lease-lock code; the earlier CAL26 measurement retains its
original source and host/configuration binding. Required GitHub CI must run on this repaired source.

Rollback must restore the old loader and its schema/audit together; restoring only the old schema
would invalidate the source-audit contract. The initial no-diff dogfood pass retained the expected
missing-map and unassessed-intent diagnostics. Final binding covers this audit repair only, against
`48f7e114ef855f91e7258c0be0ff989b479aa366`; prior sealed evidence remains intact.
