# Corvint for Pi — experimental FALLBACK

Pi **0.85.1**, Corvint Pi adapter **0.1.2**, macOS arm64: tested with the real host and an
offline fixture provider. Other Pi versions refuse visibly; Linux has not been qualified and
Windows is unsupported because descendant cleanup requires POSIX process groups.

Use a Corvint build containing the matching `adapter pi` version. Set `CORVINT_BIN` to its
absolute path; missing, empty or relative values refuse before execution.

```sh
export CORVINT_BIN=/absolute/path/corvint
pi install /absolute/path/integrations/pi
pi
```

Local packages are linked in place. Replace the package and matching Corvint binary together,
run `pi update /absolute/path/integrations/pi`, then `/reload` or restart Pi. Use `pi config`
to disable/re-enable the extension. `pi remove /absolute/path/integrations/pi` removes its
package registration; restart or `/reload` to unload it from a running session. Add `-l` to
install/remove for project-local settings. No published npm package is assumed.

For one run without changing settings:

```sh
CORVINT_BIN=/absolute/path/corvint pi -e /absolute/path/integrations/pi/index.ts
```

The package uses `pi.extensions` discovery and the host's runtime `VERSION`. Untrusted projects
refuse native reads. Startup, reload, new/resumed/forked sessions and tree navigation refresh
context. Successful compaction supplies recovery to the next model request, including an automatic
retry in the same turn. Startup/recovery messages and per-turn prompt context are ephemeral,
framed as untrusted repository data, and are not saved by the extension in Pi session history.
No prompt, transcript, tool content, model-message body or image is added to adapter storage.
The existing bounded core self-observation ledger exception remains.

`/corvint-context TEXT` shows bounded context without starting a model turn. This explicit
command's displayed message participates in Pi's session history. Task text is limited by the
native query bound (2,000 characters / 16,384 bytes); invalid input is reported visibly.

`/corvint-outcome JSON` submits an explicit existing session-end schema. An outcome requires
`taskSha256`, nonempty `changedPaths`, and nonempty `verification` entries containing
`commandSha256` and `status`. The command accepts only objects, rejects duplicate/unknown keys
and caller-supplied session identity, and lets the native core validate values and paths.
**Outcome persistence is unavailable** in this fallback kernel; the command reports that
limitation and never claims it recorded success. Tool results currently send empty observations;
changed paths or passed tests are not inferred from tool names or text.

Exact source expansion uses the existing native command:

```sh
corvint adapter source-view --root ROOT --packet PATH --packet-sha256 SHA --result N
```

Optional selectors: `--evidence N`, `--commit SHA`, `--lines RANGE`, `--requirement ID`, and
`--max-bytes N`. Native validation owns their meaning.

Automatic and command transport caps are 2000ms including normal cleanup reserve; these are
not the 250/500ms p95 qualification targets. Faults and actionable degradations appear as UI
warnings or stderr in non-UI modes. Stop never requests continuation or claims protected authority.
`agent_settled` and process exit zero are not successful completion evidence. Startup SIGINT and
SIGTERM abort owned work before print-mode prompts reach a provider; normal interactive handlers
remain Pi's responsibility. Shutdown joins owned subprocess work and removes signal listeners.

Run the dependency-free regressions with `node --test integrations/pi/runtime.test.mjs
integrations/pi/extension.test.mjs`. The canonical `make gate` includes them. The separate native
host check is `node --test integrations/pi/host.test.mjs` (requires Pi 0.85.1 on PATH and the
repository's pinned Go toolchain; `PI_BIN` may select another executable). It uses temporary
settings and an offline provider, and never reads model credentials or calls an external model.

This remains FALLBACK. Protected identity/topology, authority, permission qualification,
interactive TUI/RPC behavior, latency/recall and the full OS/host matrix remain unqualified.
The core release's FULL qualification requirement is still open.
