# Decision 0239 — VS Code live-test termination and JSON decode depth

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

Six bug-hunt hypotheses against `extensions/vscode` were filed in `docs/agent-memory/ideas.md`. Each
was settled with a unit test or a probe; no VS Code or Electron process was started.

The calls:

1. **`killGroup()` always signals the group with `SIGKILL` after the `SIGTERM` grace wait**, even when
   the direct child already exited. Before, a provider that exited on `SIGTERM` let a descendant that
   ignores `SIGTERM` survive. The alternative was to poll the group with signal 0 and give descendants
   their own grace period. It was rejected because the wait would be a poll, where `VSC-V0-059` asks
   for an event-driven wait, and because `VSC-V0-059` already lists `SIGKILL` as a step of the
   sequence. Amends `VSC-V0-059`.
2. **Deactivation waits for provider termination.** `LiveTestController.shutdown()` settles only
   after `killGroup()` settles, and the extension's `performShutdown` awaits every controller.
   `dispose()` stays synchronous for `vscode.Disposable` callers. Amends `VSC-V0-059`.
3. **A detached provider changes no run.** Stdout, `close`, and `error` from a provider that `stop()`
   or disposal already detached are ignored. Before, a record the provider printed on `SIGTERM` opened
   a new run after `stop()`, and a late `close` could end a newer provider's run. Amends `VSC-V0-066`.
4. **After an overflow, the scanner discards through the next LF.** `extractJsonRecords` takes a
   `discardingOverflow` flag and returns `overflow` to pass back. Before, a chunk that began at a `{`
   inside the dropped record scanned with inverted quote parity and swallowed the records behind it.
   Go session events are NDJSON, so the LF ends the dropped record. A JS provider prints one receipt
   and nothing follows it. Amends `VSC-V0-065`. The scanner's bound counts UTF-16 units, not bytes.
   That is left unchanged: `parseProviderRecord` enforces the byte bound on every completed record,
   so a record over the byte bound is dropped either way, and memory stays bounded.
5. **Decode depth counts the containers that enclose a value.** The top-level value is depth 0. With
   `maxDepth: 1`, `[[]]` is admitted and `[[1]]` is refused. `json.ts` and the conformance oracle
   `parseJsonStrict` (`conformance/vscode-extension-v0/run.mjs`) give the same result on every probe,
   so the code is unchanged and `VSC-V0-019` now states this meaning.

Refuted: a `once` `error` listener does not leave a second child `error` unhandled. A failed spawn
emits exactly one `error` and one `close` (probed with a missing path, a non-executable file, and a
directory). The provider handle never calls `child.kill`, `send`, or `disconnect`, which are Node's
other sources of `error`.

Also removed: the unused `UNIFIED_IMPORT` key in `stableLocalFailure`. No code, test, spec, or
conformance case names it. `UNVERIFIED_IMPORT` is the real code.

Consequences: `extensions/vscode/test/vscode-stub.ts` adds a minimal in-process `vscode` module, so
`liveTests.ts` is unit-testable under `node --test`. New tests are in `liveTests-controller.test.ts`,
`liveTests-process.test.ts`, and `liveTestProtocol.test.ts`.

Rollback: revert the commit. That restores the skipped `SIGKILL`, the unawaited disposal, the ungated
provider listeners, the scanner without the discard flag, the `UNIFIED_IMPORT` key, the unamended
`VSC-V0-019/059/065/066`, and both `docs/agent-memory` entries.
