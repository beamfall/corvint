# Decision 0236 — analyzer CLIs: `--version` admission and the write-failure exit

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`ACP-011` makes unexpected argv the `NONCANONICAL_REQUEST` rejection with exit 0, and decisions
0228 and 0233 reserve a nonzero exit for a frame that cannot be written in full. Decision 0233 left
two points open: the `--version` argument that some CLIs accept, and CLIs whose exit paths had not
been audited.

Audit of every `cmd/corvint-analyzer-*` command at this revision:

- `html-css`, `rust`, and `shader` accept exactly one `--version` argument and write a version
  receipt without reading stdin. Their family contracts depend on it: `html-css` in
  `analyzer-candidate-profiles.md` ("`--version` is the only non-analysis option"), `RAC-011`, and
  `SHA-V0-006`. Tests pin each receipt. `--version` with a further argument already rejects.
- `go` had no argv guard. Any argv analyzed stdin, which violates `ACP-011`. Its single
  `writer.Write` returned exit 0 when a writer accepted zero bytes without an error, so a frame that
  was not written still exited 0.
- `python`, `js`, `dotnet`, `kotlin-android`, `sql-native`, `ruby`, and `shell` reject any argv,
  `--version` included, and exit nonzero only when the frame write fails.
- `c-jni`, `java`, `objective-c` (profile `corvint-analyzer-native-bridge/v0`), `structured-data`
  (profile `corvint-structured-data/experimental-v1`, which also has `--descriptor`), and `swift`
  (`--request-file`) use their own profiles and are outside `ACP-011`.

The call:

(a) `ACP-011` admits `--version` explicitly, and only as a single argument on `html-css`, `rust`,
and `shader`. The receipt reads no stdin and exits 0 unless it cannot be written in full. On every
other `ACP-011` CLI, `--version` is unexpected argv. Removing the argument was rejected because
three family contracts and their receipt tests depend on it. Adding it to every CLI was rejected
because it adds source to candidates near their `ACP-009` ceilings (`javascript-typescript` has 21
bytes of headroom) and no contract asks for it.

(b) `cmd/corvint-analyzer-go` gains the `ACP-011` guard and a full-write loop. Unexpected argv,
`--version` included, writes the sentinel with exit 0. A zero-byte or failed write exits 2.

Not covered: the native-bridge CLIs analyze stdin under any argv other than a sole `--version`. That
is filed in `docs/agent-memory/fixes.md`, because it needs a call under that family's own profile. In
`js`, `kotlin-android`, `go`, and `structured-data --descriptor`, the branches that exit nonzero when
the frame cannot be produced are not reachable from the current encoders, so they are unchanged.

Consequences: `ACP-011` and its traceability row are amended. `cmd/corvint-analyzer-go/main.go`
grows from 540 to 767 bytes. The `ACP-009` go closure goes from 63,437 to 63,664 bytes, under its 65,536-byte base ceiling.
`TestRunRejectsUnexpectedArgv` and `TestRunReturnsNonzeroWhenFrameIsNotWritten` (go) failed on the
old code. `TestRunReturnsNonzeroWhenFrameIsNotWritten` (js) is a test-only pin. No `analyzerSchemaID`
bump, because neither package is under `internal/contextindex`.

Rollback: revert the commit. That restores the unguarded go CLI and its single write, the two go
tests and the js pin, the unamended `ACP-011` text and row, and removes the fixes entry.
