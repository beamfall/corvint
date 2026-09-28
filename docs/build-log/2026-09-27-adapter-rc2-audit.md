## 2026-09-27 Host adapter audit for rc.2: OpenCode 2 port and adapter fixes

Before rc.2, each host package was audited against its installed host: OpenCode 2.0.18, Pi 0.85.1,
Claude Code 2.1.267, Codex 0.153 and the VS Code extension. This change fixes the defects that stop
a host from working. Every other finding is filed as a ticket (V1-0437 to V1-0448).

- **OpenCode 2 port (`@corvint/opencode` 0.3.0, closes V1-0371).** The plugin now uses only the
  OpenCode 2 plugin API:
  - a default export `{ id, setup }`;
  - `event.subscribe` for session start, stop and end;
  - `tool.hook("execute.after")` for changed paths;
  - `tool.transform` for the two tools;
  - beta `session.hook("context")`;
  - `app.version` as the host version.

  OpenCode 1.18.31 refuses the new package at load, so OpenCode 1 support is dropped. Keeping it
  would need a second hook mapping, the zod dependency, and a V1 turn that needs non-loopback
  provider packages.

  An OpenCode 2 `plugins` entry must name the package's `src` directory:
  - an entry naming a file is skipped with a warning;
  - an entry naming the package root is dropped silently.

  OpenCode 2 has no plugin log API. Routine notices use `console.info` and faults use
  `console.warn`, and both reach only the server log.
- **Codex `fork` (0.2.3).** Codex 0.153 sends SessionStart source `fork`. The plugin's matcher
  omitted it, so a forked session never started. The matcher now admits `fork`, and the adapter
  maps it to `resume`, as it already does for Claude Code.
- **Adapter panics.** A panic inside `corvint adapter` used to become a failed hook. It now
  degrades as `adapter-internal-error` and is recorded in the ledger (`AHI-017`, `AHI-021`).
- **Claude Code compaction (`AHI-027`, `AHI-028`, `AHI-031`).** Claude Code 2.1.267 treats
  PreCompact stdout as the compactor's custom instructions and shows PostCompact stdout as plain
  text. A degradation's JSON body therefore reached the compactor verbatim. Compaction output is
  now always plain text. The host's cached replacement compaction sends an empty
  `compact_summary`, which now prints an empty line instead of a false `compaction-pin-not-preserved`.
- **VS Code (`VSC-V0-019`, `051`, `053`).**
  - MCP tool text is now checked as the `AHI-004` envelope on success and as bare canonical JSON on
    error. Every successful MCP call had failed the canonical comparison.
  - On Darwin, process-group cleanup now treats `EPERM` as "still present" until its deadline.
    Before this, cleanup was flaky.
  - The `..` escape check is fixed.
  - `unparsed` and extraction fields are handled in the model.
- **Pi.**
  - The protected runtime refuses a CLI provider override that would move a configured `endpoint`
    to another provider (`PPI-V0-003`).
  - The qualified runner frames context in the `AHI-004` envelope.
  - The TUI fixture types its prompt after the host is ready, not after a fixed delay.

Evidence:
- **opencode2 2.0.18, live.** Loopback-only, with isolated HOME and XDG directories. The plugin
  reports `active`. A stub-driven turn reached the real binary for session-start, user-prompt,
  post-tool, file-change, stop and session-end, each call carrying `--host-version 2.0.18`. The run
  logs are not committed, so the matrix row keeps `hostVersionEvidenceAdapterVersion` `unknown`.
- **opencode2 binary schema.** The binary was read to confirm the changed-path mapping: `edit`
  takes `path`, `patch` results carry `applied[].target`, and `write` results carry `target`.
- **`TestHostAdapterJavaScriptHosts`.** Passed, including the 44 `host-adapters.test.mjs` cases.
- **Go.** The adapter, compaction, fork and panic tests pass, as do `conformance/harness-event-v0`
  and `internal/specindex`.
- **Pi.** `integrations/pi` passed 25/25 against Pi 0.85.1.
- **VS Code.** 84/84 tests pass, lint and typecheck are clean, and the vsix packages.

Not run:
- **Signed-SEA `pi-protected` startup test.** It needs `make pi-protected-test`.
- **Live Claude Code compaction.** Not exercised in a real session.
- **Live OpenCode `edit` and `patch` payloads.** Only `write` ran live.
- **OpenCode beta context hook.** Not run live.
- **OpenCode 2 MCP (`mcp.servers`).** Not exercised.
- **`session.idle`.** Whether OpenCode 2 fires it at all is unknown.

Rollback: revert the change. OpenCode then needs the 0.2.9 package republished under a new version.
Codex forks stop starting sessions again.
