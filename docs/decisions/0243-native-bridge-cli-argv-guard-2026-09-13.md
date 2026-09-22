# Decision 0243 — native-bridge CLIs: argv guard and `--version` receipt

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

Decision 0236 left `cmd/corvint-analyzer-c-jni`, `cmd/corvint-analyzer-java`, and
`cmd/corvint-analyzer-objective-c` (profile `corvint-analyzer-native-bridge/v0`) outside `ACP-011`. Each
CLI checked only `len(os.Args) == 2 && os.Args[1] == "--version"`, and every other argv analyzed
stdin. `native-jvm-bridge-analyzer-candidates-v0.md` documented neither the receipt nor argv
refusal. Before this change, `--version extra` with a valid request on stdin returned a `CANDIDATE`
response from all three CLIs.

The call: the bridge profile gets its own argv rule, `NJB-008`, which mirrors `ACP-011` without
amending it.

- The CLI reads stdin only when it receives no arguments.
- Exactly one `--version` argument writes the existing receipt `corvint-analyzer-<command>/v0` plus LF.
  `TestFreshProcessPermutationMatrix` already pins that receipt, so it stays.
- Any other argv writes the `NJB-002` minimal sentinel with `NONCANONICAL_REQUEST` and exit 0. That
  includes `--version` with extra arguments and a single empty argument. No new wire code is added,
  because the profile already defines this sentinel. Nothing is left for the error-code ownership
  gate to own.
- A nonzero exit (1, as before) means only that the output was not written in full.

A bridge exemption was rejected. A CLI that analyzes stdin under arbitrary argv can silently ignore
a caller's `--input-file` intent, and `ACP-011` already treats that as a defect.

Implementation: all three CLIs call one guard, `analyzernativebridge.Run`. `WriteFull` had only the
three CLIs as production callers, so it is folded into `Run`, and its short, broken-pipe, and
zero-write vectors now exercise `Run`. Adding the guard next to a separate `WriteFull` put each
closure 19 to 102 bytes over its `ACP-009` ceiling. Folding it in keeps the ceilings unchanged.

Consequences:

- `ACP-009` closure: `c-jni` 76,255 → 76,323 of 76,380 bytes, `java` 76,257 → 76,325 of 76,382, and
  `objective-c` 76,270 → 76,338 of 76,395. Headroom drops from 125 to 57 bytes per CLI.
- `TestRunGuardsArgvWithoutReadingStdin` did not compile on the old code, which had no `Run`.
- The argv refusal cases in the three CLIs' `TestCLIInvocationFramingAndRefusal` failed on the old
  code, which returned the `CANDIDATE` response.
- There is no `analyzerSchemaID` bump, because no package under `internal/contextindex` changes.

Rollback: revert the commit. That restores the per-CLI `--version` checks, `WriteFull`, the old
tests, and the spec without `NJB-008`, and brings back the fixes entry.
