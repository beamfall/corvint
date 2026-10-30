# Decision 0228 — analyzer CLIs classify oversize and stdin read failure the same way

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

The `cmd/corvint-analyzer-*` CLIs disagreed on the two failures that happen before a request is
decoded. On an oversize request, ruby, shell, rust, and the native bridge (when the request had no
trailing LF) reported `NONCANONICAL_REQUEST`, and ruby and shell exited 2. On a stdin read failure,
rust and shader wrote `ANALYZER_FAILURE` and exited 1, kotlin-android and structured-data wrote
`ANALYZER_FAILURE` and exited 0, ruby and shell wrote `NONCANONICAL_REQUEST` and exited 2, and
sql-native wrote nothing and exited 2. `docs/specs/analyzer-candidate-profiles.md` lets the
pre-envelope sentinel carry only seven reasons, and `ANALYZER_FAILURE` is not one of them.

The call:

(a) Every stdin-reading analyzer CLI emits the fixed sentinel with `LIMIT_EXCEEDED` for a request
longer than 1,500,000 bytes, whatever its content or framing.

(b) A stdin read failure (reader error, descriptor stat failure, or detected descriptor identity
drift) emits the fixed sentinel with `NONCANONICAL_REQUEST`.

(c) Both frames exit 0. A nonzero exit is reserved for a frame that cannot be written in full.

This is what go, javascript-typescript, dotnet, html-css, and python already did, and it keeps the
sentinel inside the permitted list. The suggested alternative, `ANALYZER_FAILURE` with exit 1 for a
read failure, was rejected because that reason is not a permitted sentinel reason. The rule is added
as `ACP-012`; the kotlin-android rejection matrix is amended to match.

Not covered: `swift` reads a `--request-file` descriptor under its own profile. Library entry points
that reject an oversize buffer as `NONCANONICAL_REQUEST` (`internal/analyzerrust`,
`internal/analyzershell`, `internal/analyzerruby`) keep their pinned behavior; the CLI checks size
first. The exit code for unexpected argv (ruby and shell still exit 2) is outside this call and is
filed in `docs/agent-memory/fixes.md`.

Consequences: ruby, shell, rust, shader, sql-native, structured-data, the kotlin-android reader, and
the native bridge `readWire` changed. Each CLI's `main_test.go` (or the shared library test for the
bridge and kotlin-android) pins the new frame and exit code. The structured-data source digest pins
in `internal/analyzerstructured/evidence_test.go` were repinned. Every ACP-009 source ceiling still
holds without amendment. No `analyzerSchemaID` bump, because `TestAnalyzerSchemaInputs` does not audit
these packages.

Rollback: revert the commit. That restores the per-language reasons and exit codes, the previous
kotlin-android rejection matrix, the structured-data digest pins, and removes `ACP-012`.
