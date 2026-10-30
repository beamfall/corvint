# Decision 0235 — trace paths follow the dashboard trace path witness profile

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`trace.NewRecord` admitted any non-empty, Python-decodable, lexically clean repository-relative
tracked path, and `trace.DecodeStore` read the row back. The dashboard trace adapter
(`internal/dashboard/adapters/trace.go`, `normalizedPaths`) applies
`corvint-dashboard-trace-path-witness/0` and also refuses a path over 4,096 bytes, one that is not
valid UTF-8 (the recorder admits encoded surrogates), a Unicode control, a backslash, and an
ASCII-letter-colon prefix, so it marked such a recorder row `VERIFIER_REJECTED`.
`FuzzTraceRowsCountEveryRowTheRecorderWrites` found it (corpus path `\x7f`) and skipped the case.

The call: the witness profile is the contract for trace paths, and the producer adopts it. A path
outside it fails closed as `malformed-path` when supplied to `record`, when classified by
`AdmissibleCurrentPaths` as a tracked source, and when read from a stored row. A candidate outside
the tracked source set is still filtered first, so dogfood coordination over a non-source line-feed
path keeps `no-source-paths` (`TestDogfoodRecordNULSafeChangedPathAcquisition`). The dashboard spec already stated that
trace paths satisfy the profile, and its limits have platform reasons the producer lacks: the
dashboard passes paths as `ls-tree` arguments in chunks of at most 8,192 bytes and qualifies
Windows, where a backslash or drive-letter prefix is read as path syntax. Widening the dashboard was
rejected: it would need a new witness profile version and a new chunk and Windows argument story
for paths no Corvint source uses.

Consequence: a Git-tracked file with such a name (possible on Unix) cannot be recorded; `record`
refuses it with an error instead of writing a row that the dashboard refuses later. A stored row
naming one no longer decodes. `LTPM-V0-011` and its named-reason table are amended; the dashboard
fuzz target no longer skips these paths; `TestTracePathsFollowTheDashboardWitnessProfile` pins the
refusal on write and on a planted stored row. No `analyzerSchemaID` bump: `internal/trace` is not
under `internal/contextindex`. The two lexical checks remain separate copies, one in each package;
the fuzz target guards their agreement.

Rollback: revert the commit. That restores the wider producer admission, the fuzz skip, the
unamended `LTPM-V0-011`, and the `docs/agent-memory/bugs.md` entry.
