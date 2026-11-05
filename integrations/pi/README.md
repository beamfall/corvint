# Corvint for Pi — experimental FALLBACK

Pi **0.99.1**, Corvint Pi adapter **0.3.8**, macOS arm64. The last recorded live-host conformance
(adapter 0.3.3) used the real host and an offline fixture provider in print, RPC and interactive TUI modes. Other Pi versions refuse visibly; Linux has not been qualified and
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

The package declares the Pi host as a wildcard peer so Pi supplies its own modules; this is
packaging policy, not runtime compatibility. The closed native/runtime guards admit only 0.99.1.
The older 0.85.1 adapter remains historical qualification and is not admitted by adapter 0.3.8; adapters 0.3.2 through 0.3.7 are superseded
(decisions 0550, 0556, 0558 and 0583) and refuse visibly.
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
warnings or stderr in non-UI modes. The legacy Stop adapter never requests continuation. The separately enrolled local workflow below may request one final-settlement remediation, without protected authority.
`agent_settled` and process exit zero are not successful completion evidence. Startup SIGINT and
SIGTERM abort owned work before print-mode prompts reach a provider; normal interactive handlers
remain Pi's responsibility. Shutdown joins owned subprocess work and removes signal listeners.

Run the dependency-free regressions with `node --test integrations/pi/runtime.test.mjs
integrations/pi/extension.test.mjs integrations/pi/tools.test.mjs`. The canonical `make gate` includes them. The separate native
host check is `node --test integrations/pi/host.test.mjs` (requires Pi 0.99.1 on PATH and the
repository's pinned Go toolchain; `PI_BIN` may select another executable). It uses temporary
settings and an offline provider, and never reads model credentials or calls an external model.

This remains FALLBACK. Protected identity/topology, authority, permission qualification,
latency/recall and the full OS/host matrix remain unqualified.
The owner explicitly requires protected FULL; its separate Pi runtime admission and qualification
remain required. The explicit tool result bound is 65536 bytes; automatic context remains 8000.


## Native workflow and cockpit

`corvint_core_read` exposes closed native query/context/impact/affected/review/prove,
overview/features (clean HEAD only), witness (`base`, optional `head`), CEM/OCM status,
Frontier and local-policy status operations. It retains original receipt,
exit status and raw output. It does not index, execute suggested tests or write reports.
`corvint_tasks` exposes native Tasks reads, including closed-selector `search` and `list`;
set `CORVINT_TASKS_BIN` to a compatible
native binary when PATH selection is unsuitable. Current qualification uses developer
build202, not a stable release or qualification of the application's queue policy.

Every model tool declares Pi `annotations` (read-only, or the explicit outcome record), which
permission extensions can read through `getAllTools`; they are hints, not authority. The
package also ships the `corvint-context`, `corvint-impact`, `corvint-review` and
`corvint-tasks` skills, loaded on demand or with `/skill:<name>`; `--no-skills` disables them.

Pi 0.99.1 also has a native MCP client. From the repository root, register the
`corvint-mcp` companion in this project only:

```sh
pi mcp add corvint --local -- corvint-mcp --protocol-version 2025-11-25 --root "$PWD"
```

`--local` writes `.pi/mcp.json` in the current project and leaves
`~/.pi/agent/mcp.json` unchanged. Without it, Pi writes global configuration.
Start Pi in the project and explicitly trust it before `pi mcp list` will use its local
configuration. Keep the legacy protocol flag: without it Pi reports `corvint: failed`;
the server's `-32602` answer names the required flag (V1-2507, `MCPV0-820`).
The extension does not register it for you. Remove this registration with
`pi mcp remove corvint --local` from the same project.

`/corvint` opens Evidence, Changes, Tasks, Verification and Gaps. Use left/right or
1–5 to select a view, `r` to refresh, arrows/PageUp/PageDown to scroll and `q`/Escape
to close. Reads run only on explicit refresh. RPC receives structured results;
print/JSON use no terminal UI. Source expansion uses the existing bounded packet:
`/corvint source packet-1 {"result":0,"evidence":0,"lines":"1:20"}`.
Cached views and packets bind canonical Git-directory identity and filesystem device/inode pins. They invalidate on observed edits, session/tree navigation, repository replacement and compaction. The legacy packet scope does not independently detect every same-branch external commit; native immutable source validation remains authoritative.

`/corvint-tasks JSON` is the operator-command write boundary. It is not a model tool,
and the same command is callable through RPC, so it asserts operator intent without
claiming authenticated human provenance. Example:

```json
{"operation":"ticket prioritize","requestId":"operator-priority-001","input":{"ticketId":"ticket:project:main:V1-0001","expectedRevision":"2","priority":"P1","order":"3"}}
```

Supported writes are ticket create, refine and prioritize, claim, renew, release, submit, gate run and complete,
subject to native capabilities, policy, current revisions and attempt ownership.
Create takes `input:{"payload":{...}}`; refine takes `input:{"ticketId":"...","expectedRevision":"...","payload":{...}}`. Both send the approved native payload over stdin, and native Tasks validates schema, secrets and queue policy. Initialization, migration and manual-completion bypass are absent.
A private bounded Git-directory ledger stores replay metadata and terminal truth, never
a competing queue. After uncertainty, inspect the returned audit/ticket/attempt receipts
and repeat the identical command with `resume:true` when native replay is safe; do not
issue a new request ID. An uncertain `gate run` remains `gate-replay-unavailable`:
the native runtime can execute a gate before replay lookup, so the plugin dispatches
no retry and retains the pending identity for explicit reconciliation. Initial claim’s
`expectedRevision` is a preflight check; the native lease command has no atomic
requested-ticket-revision CAS and may bind a newer acceptance revision. Inspect the
native attempt receipt before further work; this qualification gap remains open.
An old ledger entry without terminal status stays `native-outcome-unknown`.

Since adapter 0.3.2, the adapter owns the bounded Git identity subprocess and joins cancellation before
session recovery or shutdown. Reads support detached HEAD; operator writes require
an attached branch. Durable replay binds the canonical repository, filesystem identity,
branch and original session, while transient lifecycle generations fence in-flight work.
An identical pending request can recover in its original session after fork, reload and
resume. Forks cannot take over that request. Existing 0.3.1 ledger bindings are not
migrated, rewritten or deleted: they refuse under 0.3.2 and later. Preserve the old package for
rollback and inspect native audit/attempt receipts before explicit reconciliation.

`/corvint-workflow begin PLAN_FILE` explicitly enrolls a caller-owned native dogfood
plan; `/corvint-workflow status` inspects it. The plan uses the existing native
`corvint dogfood begin` schema. Session identity is separately namespaced for Pi.
Only a completed idle final-settlement boundary may request one remediation for a
validated incomplete policy. A second boundary releases with a visible unresolved
notice; aborts, errors and queued work never become successful completion. Fork/resume retains a spent remediation allowance; a fresh extension also inspects its
existing branch remediation entries. Missing branch history or more than 4096 entries
conservatively releases until new interactive/RPC input. Recovery also binds the resolved HEAD commit, including loose/packed and linked-worktree refs; a same-branch external commit invalidates cached recovery. Missing or over-bound identity refuses. No extra transcript is saved. Recovery
packets remain ephemeral. Settlement runs no tests, report writers or outcome recording.
Verification and finishing remain explicit native CLI operations with the shown session key.

Run `npm --prefix integrations/pi test` for the complete Pi suite and
`npm --prefix integrations/pi-protected test` for the protected suite. Both are in
`make host-adapter-node-test`. Optional native prerequisites skip with named reasons;
`CORVINT_REQUIRE_PI=1` makes a missing prerequisite fail qualification. The Linux
`Pi host qualification` workflow installs Pi 0.99.1 and native candidates, then runs
the full Pi suite with that requirement. `.github/` changes need the existing
ci-control-plane admin status before landing; a workflow file is not CI-run evidence.
Set `PI_BIN` for a non-PATH host, `CORVINT_PI_SDK_ROOT` for the npm installation prefix,
and `CORVINT_PI_WORKFLOW_BIN` for the matching Core candidate. Native Tasks fixture
writes can be explicitly disabled with `CORVINT_PI_TASK_WRITES=0`; they then remain
unqualified. The protected SEA checks require their built Darwin artifacts.

Workflow recovery resolves the HEAD commit through `workflowRevision`, including
loose, detached and linked-worktree common/packed refs, and refuses unresolved
identity (including unsupported reftable or unborn branches). A same-branch commit
invalidates cached recovery and in-flight settlement (PWV-V0-8104). The explicit
operator Tasks command supports negotiated `ticket create` and `ticket refine`
with a native `input.payload`; refine also requires `ticketId` and fresh
`expectedRevision` (PWV-V0-8102). Malformed envelopes refuse before native dispatch;
native schema, secret screening, queue policy and role admission remain authoritative.

Preserve installed official binaries; a local candidate is a source build with separate
provenance and rollback, never a released update. Supervisor participation remains
blocked on its host-neutral native foundation. Comparative productivity, provider tokens,
cache usage, billed cost and superiority over OpenCode remain NOT_OBSERVED.
