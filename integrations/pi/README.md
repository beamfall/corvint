# Corvint for Pi — experimental FALLBACK

Pi **0.85.1**, Corvint Pi adapter **0.2.0**, macOS arm64: tested with the real host and an
offline fixture provider in print, RPC and interactive TUI modes. Other Pi versions refuse visibly; Linux has not been qualified and
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
limitation and never claims it recorded success. Typed `details.corvint` evidence handles, changed paths and verification observations are forwarded
without raw tool output. Changed paths or passed tests are not inferred from tool names or text.

The model can call `corvint_context` and then `corvint_expand` using the returned packet handle,
result/evidence indexes, and either inclusive lines or a requirement ID. Only four recent packets
remain in memory; switching sessions or repositories invalidates them. Expansion validates the
original digest and immutable source through the existing native source-view implementation.

`corvint_record_outcome` and `/corvint-record JSON` explicitly persist through the existing
`corvint record` writer. Supply `task`, `changedPaths`, `verification` (command strings),
`outcome` (`passed`, `failed`, or `blocked`) and optional `openedPaths`. The repository must be
clean, with the core trace directory `.context-corvint/` ignored by Git. The core validates paths and screens secrets; the receipt identifies caller-reported
learning, not verified success. No automatic hook records outcomes. A failed or interrupted record
may have written; inspect the local trace store before retrying. The legacy `/corvint-outcome`
continues to return only a nonpersistent observation.

Exact source expansion also remains available through the existing native command:

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
integrations/pi/extension.test.mjs integrations/pi/tools.test.mjs`. The canonical `make gate` includes them. The separate native
host check is `node --test integrations/pi/host.test.mjs` (requires Pi 0.85.1 and Python 3 on PATH and the
repository's pinned Go toolchain; `PI_BIN` may select another executable). It uses temporary
settings and an offline provider, and never reads model credentials or calls an external model.

This remains FALLBACK. Protected identity/topology, authority, permission qualification,
latency/recall and the full OS/host matrix remain unqualified.
The owner explicitly requires protected FULL; its separate Pi runtime admission and qualification
remain required. The explicit tool result bound is 65536 bytes; automatic context remains 8000.
