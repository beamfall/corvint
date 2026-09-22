# Decision 0233 — analyzer CLIs exit 0 on the unexpected-argv rejection

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`ACP-011` requires every candidate CLI to write the `NONCANONICAL_REQUEST` rejection, without
reading stdin, when it receives unexpected argv, but it fixed no exit code. `cmd/corvint-analyzer-ruby`
and `cmd/corvint-analyzer-shell` exited 2 with that frame. python, js, dotnet, html-css,
kotlin-android, sql-native, rust, and shader exited 0 with the same frame. Decision 0228 left this
open and filed it in `docs/agent-memory/fixes.md`.

The call: the unexpected-argv rejection frame exits 0. A nonzero exit is reserved for a frame that
cannot be written in full. This is the rule decision 0228 (`ACP-012`) set for oversize and stdin
read failure, and eight of the ten CLIs already follow it. The alternative, exit 2 everywhere, was
rejected: it would make exit status carry a second meaning beside the frame's `reason`, and it
would change eight CLIs instead of two.

Not covered: the `--version` argv that `rust`, `shader`, and `html-css` accept, and the write-failure
exit value (1 or 2 by family), which stays family-specific as long as it is nonzero.

Consequences: `ACP-011` is amended. `cmd/corvint-analyzer-ruby/main.go` and
`cmd/corvint-analyzer-shell/main.go` change one exit literal each; source size is unchanged, so every
`ACP-009` source ceiling holds. `TestRunPinsInvocationFramingAndRefusals` and the built-binary
`TestBuiltCLIEmitsExactLFAndRejectsFileArguments` (ruby) and `TestRunExactLFAndRejectsFileArguments`
(shell) pin exit 0. No `analyzerSchemaID` bump: neither
package is under `internal/contextindex`.

Rollback: revert the commit. That restores exit 2 for ruby and shell, the two test pins, the
unamended `ACP-011`, and the `docs/agent-memory/fixes.md` entry.
