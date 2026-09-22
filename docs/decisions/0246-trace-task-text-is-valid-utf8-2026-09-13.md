# Decision 0246 — trace task text is valid UTF-8

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`trace.NewRecord` screened the task through `trimPythonSpace`, whose `pythonUnits` decoding admits
an encoded surrogate (bytes `ed a0..bf 80..bf`), so it accepted a task holding a lone surrogate.
`trace.Encode` wrote it as an escaped `\udc98`, and `trace.DecodeStore` read the row back;
`TestPythonOracleEscapedLoneSurrogate` pinned that shape as a retired-Python-writer golden. The
dashboard trace adapter (`internal/dashboard/adapters/trace.go`, `strictString`) decodes the task
with `encoding/json/v2` and requires valid UTF-8, so it refused the row as `SOURCE_INVALID_SCHEMA`.
`FuzzTraceRowsCountEveryRowTheRecorderWrites` found it (corpus `4c929010abec7aba`) and skipped it
through `dashboardRefusesRecorderTask`.

The call: stored trace text must be valid UTF-8, following decision 0235 (the dashboard profile is
the contract). The recorder refuses such a task before any trace-store write, and the store reader
(`DecodeStore`, and through it `TransformLegacy`), the unreachable-row check, and the append check
refuse a stored row whose task is not valid UTF-8, all with `trace task must be valid UTF-8`. The
dashboard spec already required it (`local-observability-dashboard-v0.md` precedence step 4,
"check UTF-8/JSON"), so it is not amended. The task is the only affected free-text field: paths
already follow the witness profile (0235), verification commands admit only ASCII
(`safeCommand`), and revision, trace ID, and outcome are closed grammars.

Admitting escaped surrogates in the dashboard was rejected. Before the call, `git grep` found no
tracked trace row, fixture, conformance vector, or manifest carrying an escaped surrogate task; the
only pin was the unit test above. No spec text requires surrogate round-trip: the Python writer is
non-authoritative and removed (decision 0012 R0, `src/` is gone), and `LTPM-V0-011` already
diverges from its string semantics for paths. Widening the dashboard would need a JSON decoder that
keeps lone surrogates in place of `encoding/json/v2` and a UTF-8 exception in the frozen dashboard
precedence, for text no Corvint task needs.

Consequence: a `record` call whose task holds a lone surrogate now fails before mutation. A local
store row written earlier with one no longer decodes, so a read over that revision file fails
closed, as a 0235 path row does. No row the dashboard accepted changes bytes or trace ID.
`LTPM-V0-011` is amended; `TestPythonOracleEscapedLoneSurrogate` is replaced by
`TestTraceTaskMustBeValidUTF8`, which keeps the old golden row as the refused stored input; the
dashboard fuzz skip is removed. No `analyzerSchemaID` bump: `internal/trace` is not under
`internal/contextindex`.

Rollback: revert the commit. That restores the surrogate-admitting task screen, the Python golden
test, the fuzz skip, the unamended `LTPM-V0-011`, and the `docs/agent-memory/bugs.md` entry.
