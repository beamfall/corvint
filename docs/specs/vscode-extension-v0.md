# VS Code Extension V0

**Owner:** Russell Lewis

**Date:** 2026-08-23

**Intent status:** proposed

**Delivery status:** deferred

**Disposition:** Deferred from the core/non-editor release by owner direction on 2026-09-16.
Return requires delivered stock support/upstream merge and exact installed host, UI, memory,
conformance and cleanup qualification. Historical failures and NOT_RUN remain; no FULL claim.

**Support status:** `FALLBACK` developer preview until the promotion gate passes

**Authoritative inputs:** `docs/PRODUCT.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/DOGFOOD.md`, `docs/specs/agent-harness-integration-v0.md`,
`docs/specs/go-live-test-provider-v0.md`, `docs/specs/mcp-server-2026-07-28-v0.md`, and the official
VS Code Extension API references listed
below, plus the official
[MCP 2026-07-28 specification](https://modelcontextprotocol.io/specification/2026-07-28)

## Agent digest
- Claim: A thin VS Code adapter presents bounded Corvint evidence and explicitly enabled test-provider feedback without owning repository authority or engine updates.
- Status: proposed/deferred
- Exists: a thin TypeScript presentation/process adapter and recorded unit/oracle conformance evidence.
- Blocked on: automated Electron/remote-host compatibility, MCP runtime conformance, residue, and performance/memory qualification.
- Read next: User job and decision; Verified current state; Traceability.

## 1. User job and decision

An engineer working in VS Code can ask Corvint for bounded, revision-pinned evidence and impact,
inspect why each item was included, and track explicitly enabled live-test providers without
leaving the editor. The extension succeeds when it removes terminal/UI glue while preserving the
same frozen receipt, authority, uncertainty, privacy, and failure semantics.

V0 is a thin native TypeScript adapter over a separately installed local Corvint engine. The default
transport is the direct Corvint CLI; an explicit `mcp-stdio` transport can instead use a separately
installed local `corvint-mcp` speaking only MCP `2026-07-28`. It owns editor presentation and process
mediation only. It does not own retrieval, an evidence graph, test execution, repository authority,
code edits, or updates to any Corvint binary. VS Code's Testing API presents observations; the
adapter starts only the explicitly configured provider and never derives an execution command
from retrieved evidence.

The simpler baseline is direct use of the Corvint CLI or MCP tools. The extension is retained only if
it is measurably faster to reach the same evidence without weakening any product contract.

## 2. Verified current state

At the specification base on 2026-08-23:

- no accepted VS Code extension contract, published extension, or completed VS Code compatibility
  matrix exists in this repository;
- the Python CLI exposes `query --task ... [--limit --budget-bytes]` and
  `impact PATH... [--limit --budget-bytes]`;
- `cmd/corvint/main.go` exposes `--version`, a Darwin/Linux experimental `.go`-path `impact`, a
  Darwin/Linux authority-start `query` restricted to `--limit 1`, and `harness event`; it does not
  expose a general editor, `why`, or live-test command;
- the installed `corvint` observed during this draft identifies itself as `Corvint 0.4.0a0` but is
  older than the worktree command surface and rejects the draft task as
  `unsupported-query-intent`; an installed binary is therefore not evidence of worktree support;
- `docs/specs/go-live-test-provider-v0.md` is proposed/not-started and freezes observation wire
  profiles, but there is no promoted CLI operation that the extension may use to execute it; and
- MCP `2026-07-28` removes the `initialize` handshake and deprecated client roots are unnecessary
  because the selected contained root is bound once in the local server's startup argv; the repository contains an
  experimental `corvint-mcp` candidate, but no published/installed client tuple or VS Code MCP
  runtime result exists, so this transport remains `FALLBACK` with runtime evidence `NOT_RUN`; and
- VS Code currently provides Workspace Trust, TreeView, diagnostics, editor decorations, status
  bar/commands, and Testing APIs. Workspace extensions execute in the workspace extension host;
  in a remote window that host and any directly spawned process are remote.

An experimental extension package, source implementation, focused unit tests, independent hostile
case corpus, and packaging documentation now exist in the worktree. That source presence does not
claim any VS Code release/OS/Corvint tuple is supported: installed trusted/untrusted Extension Host,
remote-host, MCP client, residue, and performance/memory evidence remain `NOT_RUN` unless an
exact result is recorded below. The implementation remains experimental until the traceability and
promotion gates close.

## 3. Architecture and authority boundary

```text
explicit user command / explicit local observation import
  -> VS Code trust and workspace-root gate
  -> in-memory pinned executable identity or contained observation path
  -> bounded adapter operation
  -> strict receipt/observation validation
  -> immutable in-memory view model
  -> TreeViews / status / diagnostics / decorations / Test Explorer
```

The adapter has four layers: `discovery` pins one executable identity; `runner` invokes that exact
file with fixed argument arrays and resource bounds; `model` validates and projects Corvint output;
and `presentation` renders the projection through native VS Code APIs. None may infer a stronger
Corvint conclusion than the received document. A UI item is a view of one receipt or observation,
never new evidence.

Initial support matrix:

| VS Code host | Repository/CLI location | V0 status |
|---|---|---|
| Desktop, local Darwin/Linux | same local workspace extension host | `FALLBACK` pending conformance |
| Desktop Remote SSH/dev container, Darwin/Linux | same remote workspace extension host | `FALLBACK` pending separate remote conformance |
| Desktop, Windows | same workspace extension host | inert UI only; execution `UNSUPPORTED` pending CLI and containment qualification |
| Web/browser extension host | browser/virtual workspace | `UNSUPPORTED`; no web bundle, discovery, read, or execution |

## Requirements

### 4.1 Scope, activation, and support identity

- **VSC-V0-001.** The extension MUST be a desktop native TypeScript workspace extension using only
VS Code TreeView, commands, status bar, diagnostics, decorations, and Testing APIs. It MUST NOT add
a webview, custom browser surface, daemon, database, language server, or second repository graph.
The VS Code web extension host is `UNSUPPORTED` and MUST NOT receive a browser bundle.

- **VSC-V0-002.** Support MUST be stated per
`(VS Code distribution, VS Code version, extension version, extension-host OS/architecture,
local-or-remote host, Corvint CLI kind/version/executable SHA-256)`. The executable digest is required
because two builds carrying the same version token can expose different command surfaces. Untested
tuples are `FALLBACK` or `UNSUPPORTED`, never `FULL`.

- **VSC-V0-003.** Activation MAY register commands and inert views in any workspace. It MUST NOT read
workspace contents, resolve an executable, spawn a process, read an observation artifact, create
diagnostics/decorations/test results, or request trust until the workspace is trusted and an
eligible root is selected.

- **VSC-V0-004.** Every rendered conclusion MUST retain the source Corvint ID/profile, revision and
worktree state when supplied, authority/confidence/uncertainty when supplied, and named gaps. The
extension MUST NOT call `READY`, a passing observation, an empty list, or an exit-zero process proof
of correctness, completeness, test-suite safety, or merge readiness.

- **VSC-V0-005.** Multi-root operations MUST bind to exactly one explicit workspace folder. The
active document's containing folder MAY select it; otherwise a user choice is required. Results
from different roots, revisions, commands, or receipt IDs MUST NOT be joined into one snapshot.

### 4.2 Executable discovery and immutable session pin

The configuration keys are `corvint.transport`, `corvint.executablePath`, `corvint.mcpExecutablePath`,
`corvint.testObservationPath`, `corvint.maxOutputBytes`, and `corvint.timeoutMilliseconds`. The transport
and executable settings are window-scoped; the observation setting is resource-scoped; all three
path settings are declared restricted in untrusted workspaces. Configuration is an input, not
evidence.

- **VSC-V0-006.** Discovery order is exact:

1. use a non-empty `corvint.executablePath` only when it is an absolute native path whose final
   configured basename, before realpath resolution and with an optional `.exe` removed, is exactly
   `corvint` or `corvint`; that basename fixes the CLI kind;
2. otherwise read the extension-host `PATH` once, split it with the host delimiter, skip empty or
   relative components, and inspect `corvint`, then `corvint`, in component order; and
3. if none qualifies, report `CLI_MISSING` without fallback execution.

Discovery MUST use filesystem APIs, not `which`, `where`, a login shell, shell expansion, a package
manager, an editor terminal, or command substitution. `~`, variables, relative paths, URI strings,
and NUL/newline-containing configured paths are rejected rather than expanded. On Windows the only
additional candidate suffix is `.exe`; every non-file workspace and every unqualified platform is
visible `UNSUPPORTED_PLATFORM`.

- **VSC-V0-007.** A candidate qualifies only after `lstat`, `realpath`, and `stat` establish one
existing regular executable file; its real path and containing directory are absolute; its byte
size is at most 256 MiB; its complete bytes have been SHA-256 hashed; and a bounded direct
`--version` probe exits zero with empty stderr and exactly one UTF-8 line
`Corvint <version> (build <build>)\n` for `corvint`, where `<build>` is `0|[1-9][0-9]*` (PUB-V0-021).
The build number is not part of the pin; the executable digest already distinguishes builds.
`version` is at most 64 ASCII bytes and matches exactly:

```text
(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:(?:a|b|rc)(?:0|[1-9][0-9]*)|-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?(?:\+[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?
```

This grammar admits the current `0.8.1`, which is the only V0 developer-preview token admitted
for `corvint`. Every other token is `CLI_INCOMPATIBLE` until an amendment adds an
exact tested tuple. Version is pin identity, not capability proof: installed and worktree builds
with the same token may expose different commands. Symlink chains are allowed only through the
recorded real path; dangling links, directories, special files, unreadable files, and ambiguous
output fail closed.

- **VSC-V0-008.** One in-memory pin is
`{cliKind,realPath,sha256,size,mode,device,fileId,version,workspaceRoot,extensionHost}`.
The pin is scoped to one workspace folder and extension-host lifetime. Before every spawn, the
runner MUST repeat `lstat`/`realpath`/`stat` and full-byte SHA-256 and require equality with the pin.
Any replacement, link retarget, permission/type change, identity mismatch, or unreadable byte
unpins the executable and requires explicit rediscovery. File timestamps alone are never identity.
After child close the runner repeats the same checks; mismatch invalidates all output and reports
`PIN_DRIFT`. Because pathname execution cannot exclude a hostile same-UID replacement between the
last pre-check and OS execution, the pin is substitution detection, not executable attestation;
that residual gap remains visible and V0 makes no stronger claim.

- **VSC-V0-009.** After pinning, every process launch MUST pass the recorded absolute `realPath`
directly to the OS. It MUST NOT pass a bare command name, re-read `PATH`, search the working
directory, retry another candidate, or fall back from `corvint` to `corvint`. The runner sets
`shell:false`; arguments are an array; no user/task/path value is interpolated into a command
string.

- **VSC-V0-010.** Discovery and the version probe MUST NOT download, install, update, repair, rename,
or chmod a binary. An incompatible discovered CLI remains visible and unused. A user can change the
configured path and invoke rediscovery, but no Corvint binary change is silent.

### 4.3 Workspace Trust, root, remote, and environment

- **VSC-V0-011.** The manifest MUST declare limited untrusted-workspace support and list both path
configuration keys as restricted. When `workspace.isTrusted` is false, every Corvint operation fails
closed before filesystem or process access, all prior Corvint UI state is cleared, and the status is
the native trust-required `Restricted` state. No extension-owned prompt or alternate action is
shown, and the extension MUST NOT call `requestWorkspaceTrust()` automatically.

- **VSC-V0-012.** `workspace.onDidGrantWorkspaceTrust` MAY enable a fresh discovery attempt. No prior
untrusted path, receipt, observation, or UI model may be reused. A new window/workspace/extension
host performs the same gate from the beginning.

- **VSC-V0-013.** The selected folder MUST have scheme `file`. Its lexical absolute path and real
path MUST resolve to the same contained root supplied as the CLI `--root`; non-file, unresolved,
or escaping roots are unsupported. The CLI remains responsible for Git-repository validation.

- **VSC-V0-014.** In remote VS Code, discovery, hashing, observation reads, and process execution
occur only in the workspace extension host. A locally found binary MUST NOT be used for a remote
workspace or vice versa. V0 does not copy binaries or repository data across extension hosts.

- **VSC-V0-015.** Child `cwd` MUST be the selected root. On Darwin/Linux, the complete child
environment is exactly `GIT_TERMINAL_PROMPT=0`, `LANG=C.UTF-8`, `LC_ALL=C.UTF-8`, and
`PATH=/usr/bin:/bin:/usr/sbin:/sbin`, plus `HOME` and `TMPDIR` only when each inherited value is an
absolute NUL-free native path. An absent or rejected optional value is omitted; no empty substitute
is added. No other inherited variable is present. The fixed PATH exists only for Corvint-owned tool
dependencies such as Git; the extension never consults it to find or replace the pinned Corvint
executable. Windows execution is unsupported and has no V0 environment profile. The extension MUST
NOT claim this environment is a security sandbox or proof that an independently installed CLI
cannot access the network.

### 4.4 Fixed CLI adapter and process bounds

- **VSC-V0-016.** The only automatic process is the discovery-time `--version` probe. Evidence and
impact commands run only after an explicit user command. V0 admits these fixed logical operations:

| CLI kind | Logical operation | Exact arguments after the pinned executable |
|---|---|---|
| `corvint` / `corvint` | query | `--root ROOT query --task TASK --limit 1` |
| `corvint` / `corvint` | impact | `--root ROOT impact --limit 20 -- PATH...` |

`TASK` is explicit user input of 1..2,000 Unicode scalar values and at most 16,384 UTF-8 bytes.
Impact accepts 1..256 unique normalized repository-relative paths, each at most 4,096 UTF-8 bytes,
in raw UTF-8 byte order. Absolute, empty, `.`, `..`, escaping, NUL/control-containing, or
out-of-root paths are rejected. A `corvint` unsupported option/intent/path/platform response is a
visible capability gap, not a retry through `corvint`.

- **VSC-V0-017.** The probe timeout is 2 seconds. `corvint.timeoutMilliseconds` defaults to 15,000 and
is an integer in 1,000..30,000; `corvint.maxOutputBytes` defaults to 262,144 and is an integer in
4,096..1,048,576. Invalid settings fail closed to `CLI_INCOMPATIBLE`, not a larger value. Stdout is
bounded to the configured value and stderr to 65,536 bytes, including bytes subsequently discarded
from display. At the first deadline or byte overrun, the operation is cancelled, output is drained
only within the same hard bounds, and no prefix is parsed or rendered.

- **VSC-V0-018.** Each operation owns one cancellation source. A newer operation for the same root,
VS Code cancellation, root disposal, extension deactivation, timeout, or output overflow signals
the exact direct child, waits at most 250 ms, then force-terminates that direct child. Node direct-
child signalling is not descendant containment, so V0 MUST report containment `UNKNOWN`; it relies
only on the Corvint command's documented lifecycle and MUST NOT claim descendants were killed.
Observed residue is `PROCESS_RESIDUE` and a release blocker. A descendant that inherited the child's
stdout or stderr keeps those pipes open after the direct child exits; if they are still open 250 ms
after the force-termination, the operation MUST settle as `PROCESS_RESIDUE` and release its pipes
rather than wait on the descendant past its deadline. Acceptance evidence:
`extensions/vscode/test/executable-process.test.ts` runs a shell that backgrounds a `sleep`
holding stdout past a 1-second timeout and asserts the rejection is `process-residue`. A tuple
cannot be promoted until the hostile descendant fixture demonstrates zero residue without an
extension shell or unsafe process-name kill.

- **VSC-V0-019.** Only a complete successful process may be decoded. Stdout MUST be one valid UTF-8
JSON object followed by exactly one LF and no other bytes. Duplicate object keys, dangerous object
keys (`__proto__`, `prototype`, `constructor`), invalid Unicode, unexpected top-level type,
unsupported profile, missing required identity/state fields, or a value outside adapter bounds
rejects the entire result. Decode depth is at most 32, total object/array members 65,536, one string
65,536 UTF-8 bytes, and total decoded items 65,536, all within the raw stdout bound. A value's decode
depth is the number of objects and arrays enclosing it (the top-level value is depth 0), so an empty
container one level past the bound is admitted and any member inside it is not; this matches the
conformance oracle `parseJsonStrict` (decision 0239). Stderr,
partial output, and parser exceptions are inert bounded text and never evidence.

The only admitted query/impact adapter is the current unprofiled envelope V1:

- the top-level object contains exactly `context`, `mutates`, `ok`, and `tool`; `ok` is `true`,
  `mutates` is `false`, and `tool` equals the requested `query|impact` operation;
- `context` requires `schema_version:1`, matching `mode`, exact echoed `request`, lowercase 40- or
  64-hex `revision`, `freshness`, `state`, `results`, `exclusions`, `verification`, and `coverage`;
  only query may additionally contain `abstention`, `intent`, and `learning`;
- `state` is one of `READY`, `NEEDS_WIDENING`, `OUT_OF_SCOPE`, `STALE_INDEX` (no longer produced
  since `DIRTY-CACHE-005` moved worktree dirtiness to `freshness.state`), `BUDGETED`, or
  `CRITICAL_EVIDENCE_OVERFLOW`; `freshness.revision`, when present, equals `revision`, and
  `freshness.state` is `fresh|mixed-worktree`;
- `results` has at most the requested limit. Each rendered result has bounded inert `kind`, `id`,
  `summary`, optional `title|name|status`, integral score when present, and at most 16 evidence rows;
  each evidence row has exactly `path`, positive integral `line`, lowercase Git `blob_hash`,
  `reason`, `confidence`, and `authority`; and
- `coverage.within_budget` is `true`, its integral `packet_bytes` does not exceed the stdout bound,
  and all unknown nested fields are ignored rather than rendered or treated as authority. Unknown
  context/top-level keys, modes, schema versions, or a request/revision mismatch reject the result.

This schema has no top-level profile or receipt ID. The adapter labels it `unprofiled context
schema_version 1` and uses SHA-256 of the complete stdout bytes only as a local snapshot key, never
as a Corvint receipt identity. `context.verification` contains inert suggestions and MUST NOT be
executed, registered as tests, or treated as observations.

- **VSC-V0-020.** Exit nonzero, signal, cancellation, timeout, output overflow, malformed output,
identity drift, or unsupported profile produces one closed degraded state with a stable failure
code. The previous snapshot is cleared or visibly labelled historical; it MUST NOT remain presented
as the result of the failed request. Unrelated editing stays available.

### 4.5 Native presentation semantics

- **VSC-V0-021.** The command IDs are exactly `corvint.selectExecutable`, `corvint.clearExecutable`,
`corvint.query`, `corvint.impact`, `corvint.refresh`, `corvint.importTestObservation`, and
`corvint.clearResults`. `corvint.refresh` repeats only the last explicit query/impact after trust and pin
revalidation; it never runs on activation or file change. It MUST NOT edit, fix, apply, run tests,
install/update a binary, request trust, or merge.

- **VSC-V0-022.** The status bar has exactly one of `Restricted`, `No CLI`, `Incompatible`, `Ready`,
`Busy`, `Degraded`, or `Unsupported`. Its accessible name includes `Corvint` and the full state.
`Ready` means only that a trusted root and unchanged compatible pin exist. Busy state is cancellable
and cannot hide a prior failure.

- **VSC-V0-023.** `corvint.evidence`, `corvint.impact`, and `corvint.why` are separate `TreeView`s backed by
one immutable snapshot.
Evidence shows the bounded included evidence; Impact separates proven effects, bounded candidates,
and unknowns when the receipt makes those distinctions; Why is only a projection of the same
receipt's reason, authority, confidence, derivation, exclusion, and uncertainty fields. V0 MUST NOT
invoke or imply a nonexistent `corvint why` command.

- **VSC-V0-024.** Tree nodes sort by source-defined ordinal when present, otherwise by
`(state rank, repository-relative path UTF-8 bytes, range, immutable ID)`. Node IDs use the Corvint
receipt ID when supplied, otherwise the local response snapshot key, plus immutable item ID; labels
are capped at 200 Unicode scalar values, descriptions at 300,
tooltips at 2,000, 2,000 total nodes per snapshot, and depth at 16. Truncation is visible and cannot
drop an unknown/abstention state silently.

- **VSC-V0-025.** Every CLI/observation string is untrusted display data. Control/bidi characters,
ANSI escapes, invalid URI/path text, icon identifiers, Markdown command links, and theme/color
names are rejected or escaped. Tooltips use plaintext or non-trusted Markdown. A returned path is
navigable only after independent normalization and realpath containment beneath the selected root.

- **VSC-V0-026.** Diagnostics are display-only projections of receipt items carrying a valid
contained file and valid zero-based range. They use one dedicated `DiagnosticCollection`, source
`Corvint`, immutable Corvint IDs in `code`, and bounded inert messages. `CONFLICTED|REFUTED` maps to
Error, `UNKNOWN|STALE` to Warning, and explicitly informational candidates to Information; no other
state is upgraded. The cap is 500 diagnostics per snapshot and 100 per file, with one visible
truncation diagnostic. Refresh replaces the complete collection atomically; failure, trust loss,
root disposal, and deactivation clear and dispose it.

- **VSC-V0-027.** Decorations are display-only projections of the same validated ranges in currently
visible editors, never an independent analysis or CLI trigger. They distinguish evidence,
candidate, and unknown without encoding meaning by color alone, cap at 2,000 ranges, replace
atomically, and dispose every `TextEditorDecorationType` on deactivation. Decorations MUST NOT
alter files, selections, editor options, or save state.

### 4.6 Testing API observation bridge

- **VSC-V0-028.** Until Corvint publishes and promotes a live-test CLI operation, V0 MUST NOT execute
tests and MUST NOT register a `TestRunProfile`. It may create one `TestController` solely to import
an explicit `corvint.testObservationPath` when the user invokes `corvint.importTestObservation`.
No current generic Corvint capability command exists; unsupported query, impact, why, and test
features remain explicit rather than being inferred from a version string.

- **VSC-V0-029.** The observation path MUST be a normalized workspace-relative path whose lexical and
real paths remain under the selected trusted root. The file MUST be one regular non-symlink file,
at most 4 MiB, read once to EOF, then re-statted and re-hashed; identity change during the read
rejects it. FIFOs, devices, directories, links, non-file URIs, automatic directory scans, and file
watchers are forbidden.

- **VSC-V0-030.** The importer accepts only a complete Corvint `go-live-event/0` sequence followed by
one `go-live-run/0` terminal record from the same run/WEI, or another observation profile added by
an accepted amendment. It validates framing, bounds, sequence, shared identities, terminal
uniqueness, and event-root consistency through a shared Corvint verifier when one exists. Without
that verifier, structural projection is labelled `UNVERIFIED_IMPORT` and cannot become policy or
current evidence. The extension MUST NOT reimplement Corvint identity or test semantics.

- **VSC-V0-031.** Test items use the immutable Corvint test ID as `TestItem.id`; package and test IDs
form the only hierarchy unless the observation supplies a verified parent. A missing friendly name
stays an opaque ID, not an inferred split of slash-delimited test names. Source URI/range is absent
unless the observation supplies a verified contained anchor.

- **VSC-V0-032.** One imported observation creates one finite `TestRun` labelled with its Corvint run ID
and is ended after projection. Terminal mapping is exact: Corvint pass to `passed`, fail to `failed`,
error/malformed/unknown to `errored`, and skip to `skipped`. Cancellation maps to `skipped` plus an
explicit cancelled message. An incomplete, stale, rejected, cancelled, timed-out, truncated, or
identity-inconsistent run MUST NOT display any item as passed even if an earlier event said pass.

- **VSC-V0-033.** Test output is inert and bounded to 256 KiB per imported run and 8 KiB per test.
Raw runner output remains omitted when the Corvint observation provides only a digest. The extension
MUST NOT resolve a digest by reading another path, rerun a command, or present observation as proof
that omitted tests/configurations/platforms or the repository gate passed.

### 4.7 Privacy, updates, and lifecycle

- **VSC-V0-034.** The extension performs no telemetry, analytics, crash upload, repository/source
upload, remote fetch, socket connection, HTTP request, or account operation. It never sends source,
paths, receipts, observations, configuration, or usage data to the extension publisher or Corvint.

- **VSC-V0-035.** The extension stores no evidence database, cache, index, history, prompt/task,
source body, test output, binary pin, or receipt on disk or in VS Code global/workspace state.
Configuration remains VS Code-owned. All projections and pins are memory-only and bounded.

- **VSC-V0-036.** VS Code Marketplace/administrator policy exclusively owns extension installation
and auto-update. The extension contains no self-updater. Corvint CLI discovery/update is separate:
the extension MAY report a pinned version but MUST NOT download, install, replace, or silently
switch the Corvint binary. Extension update never changes the binary pin within a running host.

- **VSC-V0-037.** The extension MUST NOT edit source, apply a `WorkspaceEdit`, register a code action,
invoke another extension's edit command, start a terminal, change Git state, write Corvint artifacts,
record an outcome, or upgrade any repository/host authority. Open/reveal is the only source action.

- **VSC-V0-038.** Disable, uninstall, workspace close, trust loss, root removal, and deactivation
cancel all work, terminate and await the direct child, expose descendant-containment uncertainty or
observed residue, dispose controllers/views/events/status items/output channels/diagnostics/
decorations, and leave no extension-created file or state. A tuple with process residue cannot be
promoted. The separately installed Corvint binary and repository are untouched.

VS Code exposes a trust-grant event but no live trust-revocation event. Revoking trust therefore
takes effect through the host's required window reload/deactivation boundary; V0 MUST cancel and
dispose before that host closes, then begin the reloaded untrusted host at VSC-V0-011 with no reused
pin or projection. Conformance MUST exercise those two host lifecycles rather than simulate an
unsupported in-process revocation.

### 5. Failure model and hostile inputs

Failures are closed for Corvint evidence and open for unrelated VS Code editing. Stable V0 failure
classes are `RESTRICTED`, `ROOT_UNSUPPORTED`, `CLI_MISSING`, `CLI_INCOMPATIBLE`, `PIN_DRIFT`,
`SPAWN_DENIED`, `CANCELLED`, `TIMEOUT`, `OUTPUT_LIMIT`, `CLI_FAILED`, `INVALID_UTF8`,
`INVALID_JSON`, `UNSUPPORTED_PROFILE`, `PATH_REJECTED`, `OBSERVATION_CHANGED`, and
`UNVERIFIED_IMPORT`, plus the VSC-V0-056 MCP codes and `PROCESS_RESIDUE` when observed. Raw OS/CLI
text may accompany a class
only after inert truncation and redaction; it never becomes a command, URI, Markdown trust grant,
or diagnostic range.

- **VSC-V0-039.** Hostile conformance MUST include: untrusted activation/commands; empty/relative PATH
components; spaces and leading hyphens in roots/paths; quotes, semicolons, backticks, `$()`, `%VAR%`,
newlines, NUL, Unicode bidi, and ANSI in every user/CLI string; malicious executable symlinks and
swap-after-pin; special files and observation path escape; cancellation and timeout with a spawned
descendant; stdout/stderr exact-bound and one-byte-over cases; invalid UTF-8; duplicate/dangerous
JSON keys; deep/large JSON; extra bytes after JSON; mixed receipt identities/revisions; hostile
Markdown/URI/path/test names; remote/local binary confusion; trust grant; root removal; and
disable/uninstall residue.

- **VSC-V0-040.** No hostile value may cause shell interpretation, PATH relookup, path escape,
network activity, source write, automatic code edit, authority upgrade, stale result reuse, false
pass, process residue, or a crash of the extension host.

### 6. Deterministic acceptance and test matrix

- **VSC-V0-041.** Pure unit tests MUST cover discovery order, pin identity/drift, root/path
normalization, fixed argv/environment construction, stable failure mapping, byte/deadline bounds,
strict decoding, output sanitization, view ordering/caps, diagnostic severity/ranges, decoration
lifecycle, and Test Explorer mapping. Clocks, process IDs, filesystem identities, and host facts are
injected. Equal inputs produce byte-identical view models and host-observation JSONL.

- **VSC-V0-042.** Process integration fixtures MUST assert the literal executable path,
`shell:false`, `pathLookup:false`, exact argv/cwd/environment, cancellation/timeout escalation,
stdout/stderr bounds, and zero surviving descendants. Filesystem fixtures cover link swaps, file
replacement, special files, and remote-root separation.

- **VSC-V0-043.** `@vscode/test-electron` MUST run pinned supported VS Code versions in distinct
trusted and untrusted user-data directories because trust cannot be changed programmatically in one
run. Tests cover activation, all command enablement, status accessibility, the three TreeViews,
diagnostic/decorations replacement and disposal, imported Test Explorer results, multi-root choice,
and deactivation. A download-blocked or headless-incompatible environment is honestly `NOT_RUN`,
never a pass.

- **VSC-V0-044.** Static gates are lockfile-clean install, lint, TypeScript typecheck, unit tests,
package-manifest validation, VSIX contents inspection, dependency/license audit, and a scan proving
no webview, telemetry, network client, database, shell execution, terminal, WorkspaceEdit, code
action, or updater surface. Generated bundles and source maps MUST contain no repository fixture
contents or credentials. `npm run check` performs the lockfile-clean install itself
(`npm ci --ignore-scripts` from the tracked `package-lock.json`) before lint, typecheck, and unit
tests, so the check passes on a checkout with no `node_modules` and refuses, installing nothing,
when the lockfile is absent. `script/vscode-check-install_test.sh` pins both.

- **VSC-V0-045.** Manual query/impact adds at most 50 ms p95 adapter overhead excluding Corvint runtime
over 100 warm fixture calls; activation before a user command adds no Corvint process and no workspace
scan. Complete UI model memory is capped at 16 MiB per root. Measurements name hardware, OS, VS Code,
extension, and Corvint versions; absent measurements are `NOT_RUN`.

#### 6.1 Optional MCP 2026-07-28 stdio client

- **VSC-V0-046.** `corvint.transport` is exactly `cli|mcp-stdio` and defaults to `cli`. Transport is an
explicit per-window choice. Selection, discovery, invocation, failure, or cancellation MUST NOT
fall back to the other transport, restart or replay an operation, search for a replacement, or
silently install/update anything. Changing transport clears every pin, operation, snapshot,
diagnostic, decoration, and imported test projection before another operation is admitted.
For every setting, only an explicitly configured `corvint.*` value is read. Registered defaults are
never treated as explicit values, and no read migrates installed configuration.

- **VSC-V0-047.** `mcp-stdio` requires a non-empty `corvint.mcpExecutablePath`. It is accepted only when
it is an absolute native path whose configured basename, before realpath resolution and with an
optional `.exe` removed, is exactly `corvint-mcp`. It is qualified and pinned with the same regular-
file, executable, size, realpath, filesystem identity, complete-byte SHA-256, workspace-root,
extension-host, pre-spawn, and post-close checks in VSC-V0-007..009, except that MCP protocol/build
compatibility is established by `server/discover`, not by interpreting a CLI version as capability
proof. No `--version` probe, `PATH`, shell, package manager, URI, workspace search, or CLI candidate
participates.

- **VSC-V0-048.** MCP V0 uses local stdio only. One explicit query or impact owns exactly one newly
spawned pinned `corvint-mcp` direct child for the complete operation. The child executable is the
pinned absolute path, its exact argv is `--root ABSOLUTE_ROOT`, `shell:false`, `cwd` and
`ABSOLUTE_ROOT` are the selected contained canonical root, and the environment is VSC-V0-015.
The extension opens no socket, Streamable HTTP transport, SSE stream, listener, authorization flow,
or session. It neither pools nor reuses a process across operations. A successful operation closes
stdin and awaits clean EOF before accepting post-close pin identity.

- **VSC-V0-049.** The first request in every MCP process is `server/discover`; the extension never
sends `initialize` or `notifications/initialized` and never falls back when discovery is absent,
legacy, malformed, timed out, or method-not-found. Every request, including discovery, carries
exactly these reserved parameter metadata fields:

```json
{
  "_meta": {
    "io.modelcontextprotocol/protocolVersion": "2026-07-28",
    "io.modelcontextprotocol/clientInfo": {
      "name": "corvint-vscode",
      "version": "EXTENSION_VERSION"
    },
    "io.modelcontextprotocol/clientCapabilities": {}
  }
}
```

`EXTENSION_VERSION` is the running extension manifest version. No roots, sampling, elicitation,
logging, progress, resource, prompt, subscription, task, experimental, or extension capability is
advertised. Client identity is informational and grants no repository or host authority.

- **VSC-V0-050.** Discovery MUST be one complete result that lists `2026-07-28` in
`supportedVersions`, advertises a `tools` capability, carries bounded server identity in the
standard response `_meta`, with the exact `corvint-mcp` name and MCPV0-006 Corvint description, and a
bounded inert version, and does not require another interaction. Server version is self-reported
display/debug metadata: the client records it alongside protocol `2026-07-28` and the complete
executable digest in the compatibility tuple but MUST NOT admit or reject a server from that value.
Wire conformance and the pinned executable digest establish admission. The client pins exactly
`2026-07-28`; another offered
version is not selected. `resultType:"input_required"`, an MCP
extension, server request, server-originated notification, or advertised-only capability does not
authorize a flow. Legacy protocol/lifecycle discovery or contradictory metadata produces
`MCP_INCOMPATIBLE`.
The accepted complete `{name,description,version}` value MUST remain identical across discovery,
the tool-list result, and the tool-call result; unknown pairs or drift
at either later response are incompatible and produce no projected receipt.

- **VSC-V0-051.** After discovery the client fetches `tools/list` in the same process. Discovery MUST
match MCPV0-006, including `ttlMs:0`, `cacheScope:"public"`, the exact tools capability, and the
standard server info. The tool-list result MUST match MCPV0-007..010, including
`ttlMs:300000`, `cacheScope:"private"`, and exactly the three frozen, ordered definitions
`corvint.impact`, `corvint.query`, and `corvint.status` with their closed schemas and annotations. The
client accepts exactly that one page and no `nextCursor`. Its bounded decoder can recognize an
optional cursor only to reject the response as `MCP_TOOLSET_MISMATCH`; it MUST NOT request another
page under V0. `ttlMs` and `cacheScope` are structurally validated but V0 performs no catalog cache:
every explicit operation rediscovers and relists. The extension invokes only the requested
`corvint.query` or `corvint.impact`; it never calls `corvint.status`. An extra, missing, reordered,
cursor-bearing, or schema-mismatched tool result is `MCP_TOOLSET_MISMATCH`. Future pagination
requires an amendment to both the MCP server and client contracts.

- **VSC-V0-052.** The MCP tool calls are closed read-only adapters. `corvint.query` receives the exact
validated VSC-V0-016 task as the closed arguments `{"task":"TASK"}` only when it also satisfies
the MCPV0-009 printable-ASCII, non-all-space profile `^[ -~]*[!-~][ -~]*$`; Unicode valid for the
CLI is not thereby valid for MCP. `corvint.impact` receives `{"paths":[...],"limit":20}` only when
every validated path also satisfies MCPV0-009's unique, normalized, repository-relative `.go` path
profile and cardinality bounds. A value valid only for CLI is rejected before MCP spawn as
`MCP_INPUT_UNSUPPORTED`, never coerced, replaced, or sent. The root exists only in the fixed startup
argv; V0 does not advertise deprecated roots, answer `roots/list`, send a root tool argument, infer
a different root from response data, or let a response replace it. Tool schemas and annotations
MUST byte-semantically match MCPV0-007..010. A schema or argument echo that weakens these
constraints is `MCP_TOOLSET_MISMATCH`.

- **VSC-V0-053.** `tools/call` MUST return one complete result with `resultType:"complete"`,
`isError:false`, and `structuredContent` containing the exact closed
`corvint-mcp-bridge-result/0` wrapper frozen by MCPV0-008. Its nested unaltered native query/impact
`receipt`, when non-null, is a context receipt object, not the CLI `{context,mutates,ok,tool}`
envelope: it MUST satisfy the context-schema, request, revision, and coverage rules in VSC-V0-019
directly, with no synthetic envelope or nested `context` requirement. `receipt:null` is valid only
for an MCPV0-008 wrapper state/reason that expressly admits it; the extension projects the wrapper's
bounded abstention/gap, repository binding, epistemic class, authority class, and reason without
inventing evidence. A valid null-receipt abstention is not `MCP_TOOL_ERROR`, an empty success, or a
license to discard wrapper metadata. The result MUST have exactly one text content block whose text
is the canonical compact JSON encoding of the same wrapper; after strict parsing it MUST equal
`structuredContent` structurally and its canonical bytes MUST equal the text bytes. Neither copy is
a second receipt. A protocol error, missing structured content, `isError:true`,
`resultType:"input_required"`, sampling, elicitation, roots, resources, prompts, tasks,
subscriptions, or a server request is rejected without UI projection or authority upgrade.

- **VSC-V0-054.** MCP stdio uses one UTF-8 JSON-RPC object plus LF per frame and rejects batches,
invalid UTF-8, duplicate/dangerous keys, invalid envelopes, wrong-type/mismatched/duplicate IDs, responses
to unknown/completed IDs, and any non-protocol stdout. Exactly one request is in flight. IDs are
positive safe integers allocated monotonically within the operation and never reused. A complete
or partial line, one decoded frame, and aggregate stdout each remain within
`corvint.maxOutputBytes`; stderr remains inert and bounded by 65,536 bytes even when non-empty.
There are at most 64 bounded server notifications totalling 65,536 bytes, but because V0 supplies
no progress token or logging level, `notifications/progress`, `notifications/message`, and every
other server notification are `MCP_PROTOCOL` and terminate the operation. Unknown messages never
become text, a URI, a command, a test, or evidence. A server that closes its stdin makes the next
request write fail (`EPIPE`); that stream error MUST terminate the operation as `SPAWN_DENIED`, never
escape as an uncaught extension-host exception. Acceptance evidence: `extensions/vscode/test/mcp.test.ts`
drives a `closed-stdin` fake server that answers discovery only after closing its stdin and asserts
the operation rejects with `spawn-failed`.

- **VSC-V0-055.** VS Code cancellation, a newer operation, timeout, output overflow, root disposal,
trust-host shutdown, and deactivation first send one bounded `notifications/cancelled` for the exact
in-flight request ID with the metadata in VSC-V0-049, then close stdin. The client waits 250 ms for
EOF, sends TERM to the direct child, waits another 250 ms, then sends KILL if required. It admits no
post-cancellation result. This sequence does not prove descendant containment; observed residue is
`PROCESS_RESIDUE` and the tuple cannot be promoted.

- **VSC-V0-056.** Stable MCP client failure codes are `MCP_MISSING`, `MCP_INCOMPATIBLE`,
`MCP_INPUT_UNSUPPORTED`, `MCP_PROTOCOL`, `MCP_TOOLSET_MISMATCH`, `MCP_TOOL_ERROR`, and
`MCP_INTERACTION_REQUIRED`, in addition to applicable root, pin, spawn, cancellation, timeout,
output, JSON, residue, and observation codes already frozen above. They clear the current result and
are bounded inert display values. No error causes replay, restart, failover, transport fallback,
catalog caching, a client capability change, or another Corvint operation.

- **VSC-V0-057.** The MCP client remains `FALLBACK` and its runtime evidence `NOT_RUN` until a
publishable `corvint-mcp` binary implementing the existing MCPV0 root/tool/bridge contract is
installed and independent black-box conformance drives the packaged extension through discovery,
pagination, query, impact, cancellation, timeout, hostile frames, clean EOF, and descendant-residue
fixtures. Server source, unit tests, a protocol document, or successful direct CLI execution is not
MCP client evidence. The current experimental server source and local tests do not close this client
tuple; its inherited Git runner still has a documented executable-pinning promotion blocker.

### 6a. Live test-provider feed (IPR-09 editor half)

Roadmap IPR-09 connects actual current receipts to Test Explorer and diagnostics. The editor half
consumes the two wire shapes the real providers emit, which differ in both framing and payload:

- `corvint-go-test-provider session --foreground --authority-bundle <bundle> --watch <path>...`
  emits newline-delimited JSON (one line per session-state transition), profile
  `corvint-go-live-session-event/0`:
  `{"profile":"corvint-go-live-session-event/0","state":"idle|running|passed|failed|stale|
  infrastructure|cancelled","identity":string,"sequence":number,"scope":[string,...]|null,
  "detail":string,"projection":<testvalidity.Projection JSON>,"testProjections":[{"package":string,
  "name":string,"action":"pass|fail|skip|none","projection":<testvalidity.Projection JSON>}]|null,
  "testProjectionsOmitted":number}`. `scope` names the package(s) in play for the run a `running`
  event opens, and the matching terminal event closes every item that `running` announced.
  `projection` (GLTP-V0-050) states that same session-level transition in the one shared projection
  shape the JS provider also emits; the editor half decodes it and cross-checks it against the
  state per VSC-V0-068, without letting it choose the outcome. `testProjections` (GLTP-V0-051)
  carries one per-test projection per test a completed run reported, and is consumed per
  VSC-V0-067; a per-test source anchor (GLTP-V0-052) is consumed per VSC-V0-069.
- `corvint-js-test-provider unit|e2e` emits exactly **one pretty-printed (multi-line) JSON document**
  at process completion, not NDJSON: `{"receipt":{"kind":"unit"|"e2e",...},
  "testProjections":[{"name":string,"state":string,"projection":<testvalidity.Projection JSON>}],
  "runProjection":<testvalidity.Projection JSON>}`. Per-test `file:line` is recovered from
  `projection.execution.anchors[0]` (format `"file"` or `"file:line"`), since the wire entry itself
  carries no anchor field.

Both shapes are discriminated by `parseProviderRecord` (`profile` present → Go session; `receipt`
present → JS provider) and both framings are read off one `child.stdout` byte stream by a single
brace/bracket-depth-aware scanner, `extractJsonRecords`, so NDJSON lines and one multi-line document
are extracted uniformly regardless of which provider is running. An earlier draft of this feed
assumed a `{"kind":"validity"|"state", inputIdentity:string}` line shape; no producer in this
repository ever emitted it, and it has been removed rather than retained as a second supported shape.
Parsing and mapping are isolated in `extensions/vscode/src/liveTestProtocol.ts`, a pure module with
no VS Code, filesystem, or process dependency, so a provider field-name change is a one-file edit.

- **VSC-V0-058.** A live test-provider process MUST NOT be spawned unless `corvint.liveTests.enabled` is
explicitly `true` for the resource and the workspace is trusted; the default is `false`. Its argv is
taken verbatim from `corvint.liveTests.command` with no shell interpretation, PATH relookup, or
implicit binary discovery, matching the CLI/MCP executable posture elsewhere in this document.
Acceptance evidence: `extensions/vscode/test/liveTests-process.test.ts` asserts the packaged
manifest's `corvint.liveTests.enabled` and `corvint.liveTests.command` defaults are `false`/`[]`.

- **VSC-V0-059.** The provider process MUST be started as its own process-group leader (POSIX
`setsid` via `detached`) so `killGroup()` can terminate it and every descendant it spawned in one
signal sequence: `SIGTERM`, then an event-driven wait for the child's actual `close`/`exit` (bounded
by a grace timeout, not a fixed sleep), then `SIGKILL` and a second bounded wait, on POSIX;
`SIGKILL` is sent to the group even when the direct child already exited, because a descendant can
ignore `SIGTERM` and outlive it (decision 0239);
`taskkill /t /f` on Windows. The provider's stderr MUST NOT be piped back to the extension host
(it is sent to the null device): V0 never displays it, and an undrained pipe would block the
provider once the OS pipe buffer fills, stalling its stdout records. This termination MUST run on
deactivation, on `corvint.liveTests.enabled` transitioning to `false`, and on workspace-folder removal;
deactivation MUST NOT settle before it completes.
Server source or a clean direct exit is
not descendant-residue evidence. Acceptance evidence: `extensions/vscode/test/liveTests-process.test.ts`
spawns a shell fixture that forks a background `sleep` descendant outliving the test's hang detector
and reports its pid on stdout, confirms both are alive, calls `killGroup()`, awaits the provider's
stdout `close` (every pipe holder exited) and then until neither pid answers signal 0, each wait
event-driven behind a hang detector rather than a latency budget, and asserts both exited; the same
file spawns a provider that writes 1 MiB to stderr before its first stdout line and asserts that
line arrives behind the hang detector. The same file spawns a provider whose descendant ignores
`SIGTERM` and holds no provider pipe, and asserts the descendant is gone after `killGroup()`;
`extensions/vscode/test/liveTests-controller.test.ts` asserts `LiveTestController.shutdown()` settles
only after a `SIGTERM`-ignoring provider was killed.

- **VSC-V0-060.** Each decoded per-test record's `projection.execution` state MUST map to exactly one
`vscode.TestRun` call: `PASSED`→`passed`, `FAILED`→`failed`, `SKIPPED`→`skipped`; `INFRASTRUCTURE`
and `CANCELLED`, and any other unrecognized execution state, map to `errored`. This mapping wins
before freshness is considered, and applies uniformly to a JS provider `testProjections[]` entry.
For the Go session stream's session-level transition, applied to its `scope` items, terminal
`passed`/`failed` states map directly to `run.passed`/`run.failed`, and `stale`/`infrastructure`/
`cancelled` map to `run.errored`, never `run.failed` — `classifyGoSessionState` is the session-level
analog of `classifyProjection` for this stream, applied through `classifyGoSessionEvent`
(VSC-V0-068); its per-test entries follow VSC-V0-067. Acceptance evidence:
`extensions/vscode/test/liveTestProtocol.test.ts` drives `classifyProjection` over every case in
`conformance/test-validity-v0/vectors.json` and asserts the resulting `kind`, and asserts
`classifyGoSessionState` maps `passed/failed/stale/infrastructure/cancelled` to exactly one outcome
each, never `failed` for `infrastructure`/`cancelled`.

- **VSC-V0-061.** When `projection.freshness.state` is `STALE`, the record's `kind` MUST be `stale`
regardless of what `projection.execution` reports, so a pass or fail measured against superseded
source is never presented as a fresh result. `vscode.TestRun` has no dedicated stale state and
`enqueued()` accepts no message; V0 therefore renders a `stale` outcome as `errored` carrying an
explicit `STALE: <freshness.reason>` message. This choice is a display decision only: freshness,
execution, and strength remain independent axes per `internal/testvalidity`, and no axis is
overwritten by another in the retained `Projection`. Acceptance evidence:
`extensions/vscode/test/liveTestProtocol.test.ts` asserts a `STALE` freshness axis overrides a
`PASSED` execution axis, and every vectors.json case with `freshness.state === "STALE"` classifies
as `stale`.

- **VSC-V0-062.** `projection.strength` (when not `NOT_MEASURED`/`UNSUPPORTED`) and `projection.hygiene`
(when `INELIGIBLE`/`ABSTAINED`) MUST be surfaced as advisory text — a `TestRun` output line or
message note — and MUST NOT change the `TestRun` call chosen by VSC-V0-060/061. A killed or survived
mutation witnesses one distinction, never general adequacy, and an ineligible/abstained hygiene
state is a warning, not a failure. Acceptance evidence:
`extensions/vscode/test/liveTestProtocol.test.ts` asserts note presence/absence tracks strength and
hygiene state across every vectors.json case independent of the chosen `kind`.

- **VSC-V0-063.** A JS-provider per-test record whose projection resolves a `file`/`line` anchor
(recovered from `projection.execution.anchors[0]`, format `"file"` or `"file:line"`, falling back to
line 1 for a bare-file anchor) and whose classified outcome is `failed` MUST add exactly one
`vscode.Diagnostic` at that `file`/`line` (1-based; clamped to line 0 if reported as 0) carrying the
execution reason text, keyed by `testId` so a later non-failing record for the same `testId`/`file`
removes only that diagnostic. Each JS-provider receipt is treated as a fresh session (it is the one
output of a single completed process invocation), so every retained diagnostic from a prior session
MUST be cleared before any diagnostic for the new one is added. The Go session stream produces
diagnostics only for per-test entries that carry a source anchor (VSC-V0-069). This bookkeeping is isolated in the pure
`DiagnosticsLedger` (`liveTestProtocol.ts`); `liveTests.ts` mirrors each returned snapshot into a real
`vscode.DiagnosticCollection` entry. Acceptance evidence: `extensions/vscode/test/liveTestProtocol.test.ts`
covers ledger apply/clear-on-resolve and clear-on-reset, and anchor recovery (with and without a line
suffix, and with no anchor at all).

- **VSC-V0-064.** A terminal Go session event (`passed`/`failed`/`stale`/`infrastructure`/`cancelled`)
whose `identity` does not match the session's currently active identity (last announced by a
`state:"running"` event) MUST be dropped without changing any `TestItem` or diagnostic — an
older-identity result arriving out of order after a newer session already started never resurrects
or downgrades a newer result. A new `running` event for a *different* identity than the currently
active one starts a fresh session (clearing every tracked `TestItem` and diagnostic from the prior
identity, per VSC-V0-063), so `TestItem`/diagnostic state never accumulates unboundedly across
sessions. A `running` event for the *same* identity as the active one MUST also start a fresh run
when the previous run for that identity has already closed (a re-trigger on unchanged content), so
its terminal event has a run to close instead of being dropped. Acceptance evidence:
`extensions/vscode/test/liveTestProtocol.test.ts` asserts `shouldApplyRecord` admits an unset
identity and the current identity, and rejects a stale one, and asserts `beginsNewRun` opens a run
for a new identity and for the active identity once its run closed.

- **VSC-V0-065.** A provider record that fails to parse (malformed JSON, an unsupported record shape —
neither the Go session `profile` nor the JS provider `receipt` field present — or an unsupported
`state` token) MUST be dropped without becoming a `TestItem` update, a diagnostic, a crash, or a
silently-invented passing/failing result. Acceptance evidence:
`extensions/vscode/test/liveTestProtocol.test.ts` asserts `parseProviderRecord` throws a bounded
`ProtocolError` for empty, non-JSON, unsupported-shape (including the retired `{"kind":"validity"}`
shape), and unsupported-state input, and `liveTests.ts` swallows exactly that error type on its
stdout data handler. An unterminated trailing value that grows past the record bound is dropped
by the scanner, and complete records that precede it in the same buffer MUST still be delivered;
the same test file asserts `extractJsonRecords` returns the preceding record with `overflow` set.
After an overflow the scanner MUST discard through the next LF before it scans for a record, so a
later chunk that begins inside the dropped record cannot swallow the complete records behind it; the
same test file asserts a chunk starting at a `{` inside the dropped record still yields the record
after its LF.

- **VSC-V0-066.** When the live test-provider cannot be started (an invalid/empty
`corvint.liveTests.command`, or a spawn failure such as `ENOENT` or a non-executable command) or exits
abnormally (an unrequested child `error`, or a `close` the extension did not itself request via
`stop()`/disposal, except a qualified normal JS completion under VSC-V0-071), V0 MUST surface a user-visible explanation naming the cause instead of returning
silently. Per product invariant 2, an unavailable provider MUST produce a visible unknown, never
invented certainty: V0 MUST NOT leave a prior session's `TestItem`/diagnostic state standing, or
otherwise present the absence of results as a passing or current state. Acceptance evidence:
`extensions/vscode/test/liveTestProtocol.test.ts` asserts `describeLiveTestUnavailability` returns a
distinct, cause-naming message for each of invalid-command, spawn-failure, process-error, and
abnormal-exit causes; `liveTests.ts` reports each of the three unavailability sites above through
this function onto a fresh, immediately-ended `TestRun`, which also clears the prior session's
`TestItem`s and diagnostics. Stdout, `close`, or `error` from a provider that `stop()`/disposal already
detached MUST NOT change any `TestRun`, `TestItem`, or diagnostic; `extensions/vscode/test/liveTests-controller.test.ts`
asserts a record the provider prints on `SIGTERM` after `stop()` opens no run.

- **VSC-V0-067.** Each entry of a terminal Go session event's `testProjections` (GLTP-V0-051) MUST
become its own `TestItem`, keyed `<package>#<name>`, and be classified from its own projection by
`classifyGoTest` — `classifyProjection` unchanged (VSC-V0-060..062), so a `STALE` per-test freshness
renders as stale and strength/hygiene never change the call. The entry's `action` MUST NOT choose
the outcome; it is used only to name the observed go test action in the message of a result whose
execution axis abstains (`UNSUPPORTED`), so a never-terminated test is shown as an explicit
unknown, never as passed by inference. A skip is shown as `skipped` only because its projection
states execution `SKIPPED` (GLTP-V0-052), not because its `action` is `skip`. A positive `testProjectionsOmitted` MUST
be shown as one `errored` item carrying an `UNKNOWN` message with the omitted count
(`describeOmittedGoTests`), never silently dropped or presented as passing. Per-test entries follow the
same identity supersession as their event (VSC-V0-064), a malformed entry makes the whole event
unparseable (VSC-V0-065), and an event with no `testProjections` (absent or `null`) adds no
per-test item. Acceptance evidence: `extensions/vscode/test/liveTestProtocol.test.ts` asserts
`parseProviderRecord` decodes per-test entries and the omitted count, defaults an absent array to
empty, and rejects an entry with no projection; and asserts `classifyGoTest` maps pass, fail,
skip, and stale projections to `passed`/`failed`/`skipped`/`stale` and an abstained `none` to
`errored` naming the action.

- **VSC-V0-068.** The editor MUST decode a Go session event's session-level `projection`
(GLTP-V0-050). An absent or `null` projection leaves the event classified from its state alone; a
present projection that is not a JSON object MUST make the event unparseable (VSC-V0-065).
`classifyGoSessionEvent` MUST take the outcome from `classifyGoSessionState`, never from the
projection, so a projection can never upgrade a state. When `classifyProjection` of the projection
yields a different outcome kind, the scope items MUST be marked `errored` with an `UNKNOWN` message
naming the state and the contradicting projection, rather than trusting either side. Acceptance
evidence: `extensions/vscode/test/liveTestProtocol.test.ts` asserts the decoded projection, the
agreeing `failed` and `stale` cases, an absent projection, a failing projection on a `passed` state
producing `errored`, and the non-object refusal.

- **VSC-V0-069.** A Go per-test entry's source location MUST be recovered only from a first
execution anchor of the form `<path>:<line>` with a non-empty path and an integer line of at least
1, and never from an anchor beginning `input-identity:`. An entry without one stays keyed only by
`<package>#<name>`, with no file and no diagnostic. An anchored entry MUST become a file-located
`TestItem` and MUST feed the same `DiagnosticsLedger` as JS-provider records (VSC-V0-063): one
diagnostic at that line when its classified outcome is `failed`, removed when a later result for
the same test is not. Acceptance evidence: `extensions/vscode/test/liveTestProtocol.test.ts` asserts
anchor recovery from a leading `file:line` anchor and its refusal for an identity-only and a
line-less anchor.

- **VSC-V0-070.** Producer retention (`LPCV-V0-055`) MUST be passed only on explicit opt-in
(decision 0230). A resource-scoped `corvint.liveTests.retainEvidence` setting defaults to `false`, and
while it is `false` the spawned argv MUST be exactly `corvint.liveTests.command`. While it is `true`,
V0 MUST insert `--retain` directly after the subcommand of a command whose program basename is
`corvint-js-test-provider` with subcommand `unit` or `e2e`, or `corvint-go-test-provider` with
subcommand `session`. It MUST
refuse to spawn any other command, and any command that already names a `retain`
flag, and MUST report that refusal through the `VSC-V0-066` unavailability path rather than run the
command without retention. The extension itself writes nothing: the provider writes into
`.corvint/test-evidence`, where `LPCV-V0-053` discovery reads it. Acceptance evidence:
`extensions/vscode/test/liveTestProtocol.test.ts` asserts `withEvidenceRetention` leaves argv
unchanged when off, inserts `--retain` after each supported subcommand when on, and refuses an
unsupported program, a non-retaining subcommand, a shell wrapper, and a pre-existing retain flag
with a message naming the setting; `extensions/vscode/test/liveTests-process.test.ts` asserts the
manifest default is `false`. No `@vscode/test-electron` run has observed a retaining provider
(`NOT_RUN`). Host attempt 2026-09-13 (macOS, local VS Code 1.136.1, whose executable is
`Visual Studio Code.app/Contents/MacOS/Code`, not `Electron`): `@vscode/test-electron` is not in
`package.json` or `package-lock.json`, and `npm view @vscode/test-electron version --offline`
returns `ENOTCACHED`, so under the cache-only install rule no integration script was written or run.
The extension itself does not block a headless run: a live run is configuration-triggered, not
command-triggered (`extensions/vscode/src/extension.ts:192`, `extensions/vscode/src/liveTests.ts:58`),
so an extension test can update `corvint.liveTests.*` on the workspace folder; it spawns only in a
trusted workspace (`extensions/vscode/src/liveTests.ts:64`), so a fresh `--user-data-dir` run needs
`--disable-workspace-trust` or an explicit trust grant. Any such launch MUST pass both
`--user-data-dir` and `--extensions-dir` temp directories: launching the app binary without them
starts the user's default profile, which can auto-update installed extensions.

Real-client evidence: a real, un-mocked `corvint-go-test-provider session --foreground` and both
`corvint-js-test-provider unit` and `corvint-js-test-provider e2e` were run against this repository and a
fixture app, and their genuine stdout was driven through the compiled adapter's
`extractJsonRecords`/`parseProviderRecord`/`classifyGoSessionState`/`classifyProjection` functions —
the same functions `liveTests.ts` calls — confirming a real pass→fail→pass Go session sequence and
real JS per-test pass/fail/skip classification with recovered `file:line` anchors
(internal evidence transcript of 2026-09-12, not part of the public tree). This closes the provider-stream half of the promotion gate; it
does not launch the VS Code extension host itself (no `@vscode/test-electron` run observing the real
Test Explorer UI or Problems panel), which remains open per this document's existing promotion gate
in section 8.


### Interactive alpha lifecycle (owner release scope, decision 0286)

- **VSC-V0-071.** On the POSIX alpha profile, a one-shot JS result MUST remain visible after exit code zero without a
signal only when stdout contains exactly one valid complete JS JSON value (newline optional),
followed only by JSON whitespace, and bounded process-group cleanup succeeds. A second value,
mixed Go/JS stream, malformed/truncated/trailing input, overflow, process error or failed cleanup
MUST produce visible unavailability. Execution and freshness come from the projection; exit zero
is never a test pass. This normal completion is the sole exception to VSC-V0-066.
- **VSC-V0-072.** In an explicitly enabled trusted file workspace, saving a file MUST trigger a
250 ms coalesced rerun of the full configured `corvint-js-test-provider unit|e2e` command when it
lies within declared `corvint.liveTests.watchPaths` (default `["."]`, at most 64 normalized relative
files/directories). Resolve both saved files and watch locations canonically inside the workspace;
refuse symlink escape. Exclude `.git`, `.corvint`, `node_modules`, `dist`, `build`, `out` and
`coverage` path segments. Route a nested workspace file to its containing folder once. Other
commands do not acquire save triggers. External filesystem edits are outside this alpha profile.
- **VSC-V0-073.** A relevant save, configuration change, disable, root removal or shutdown
MUST supersede prior process identity before awaiting cleanup. A save clears prior current results
and displays pending/unknown until the replacement completes. Serialize cleanup and replacement,
apply the newest configuration, suppress late output/close/error, and cancel pending timers on
disable/shutdown. Preserve completed results until an explicit superseding event.
- **VSC-V0-074.** Every provider termination, including a normal exit-zero leader, MUST await
one shared bounded process-group cleanup promise before its result is retained. On POSIX, verify
group absence after termination; signalling alone is not proof. Cleanup failure MUST block
replacement and report unavailability. A real subprocess regression MUST prove interruption and
normal leader exit leave no group descendants, including descendants ignoring SIGTERM. This is
process-group containment, not an OS sandbox or a claim about deliberately detached groups.

- **VSC-V0-075.** The read-only extension export `liveTestSnapshot(root)` MUST return undefined
for absent, untrusted or non-file exact workspace-folder roots. Its closed `corvint-vscode-live-tests/0`
snapshot contains phase, generation, provider kind, existing Go input identity (JS unknown/null),
SHA-256 of the exact complete provider JSON value excluding surrounding JSON whitespace, at most
2048 submitted item IDs/states, and an explicit omission count. It MUST copy memory without IO,
execution, subscriptions or mutation authority, expose no raw provider record, and update only
after synchronous VS Code API calls. Supersession clears terminal identity immediately. JS
completion follows successful group cleanup; Go completion means a foreground session event.
The snapshot is evidence of submitted values and lifecycle, not independent UI rendering or
provider verification. Installed checks separately observe real diagnostics and MCP projections.

- **VSC-V0-076.** Only after trust and explicit enablement, resource-scoped/restricted
`corvint.liveTests.toolchainPaths` (default empty) MAY prepend at most 16 explicitly configured
absolute normalized directories to the live provider's fixed PATH. Reject controls, the platform
PATH delimiter, entries over 4096 characters and aggregate UTF-8 bytes over 8192 before spawn;
deduplicate in declared order. Other environment and core CLI/MCP calls remain unchanged. Keep
provider stdin open for the foreground lifetime; shared cleanup closes it, allows bounded EOF
shutdown, then terminates and verifies the entire group regardless of stdin errors. This does
not supply or invent any Go parent-verifier attachment. The configured provider executable itself
MUST be an absolute lexical-normal path, never resolved through these toolchain directories.
Windows live-provider execution remains explicitly unsupported in this alpha.

Acceptance evidence for VSC-V0-071..076 is the controller/process regression suite plus the exact
installed qualification required by PUB-V0-016. Unit stubs alone do not qualify an editor host.

## 7. Rollout, compatibility, rollback, and maintenance

1. Freeze independent host-observation and hostile vectors before implementation.
2. Ship an unpackaged developer preview marked `FALLBACK`; run lint/typecheck/unit/process tests.
3. Run `@vscode/test-electron` on one pinned local trusted/untrusted tuple, then remote SSH or
   dev-container tuples separately.
4. Publish a VSIX/Marketplace preview only after owner approval, permissions/contents review, and
   the privacy/rollback gates pass.
5. Promote exact tuples independently; API or CLI changes add an adapter/matrix row and rerun every
   affected vector.

Rollback is disable/uninstall of the extension or reversion to the prior Marketplace version.
Because V0 creates no repository state, database, cache, binary installation, or migration,
rollback requires no cleanup. The one exception is opt-in: with `corvint.liveTests.retainEvidence`
(`VSC-V0-070`) the spawned provider, not the extension, writes `.corvint/test-evidence`, which can be
deleted. The user-installed Corvint CLI remains untouched. If a VS Code API
changes, the extension degrades while Corvint Core and wire profiles remain unchanged.

## 8. Promotion and kill criteria

Promotion from `FALLBACK` requires an accepted specification; all in-scope requirements linked to
implementation and passing evidence; independent security review; clean VSIX inspection; passing
hostile conformance; trusted and untrusted `@vscode/test-electron` on every claimed tuple; zero
process residue across 1,000 cancellation/timeout cycles; and the automated performance/memory
qualification in `VSC-V0-045`.

**2026-09-15 owner amendment (prospective):** No human study is required as a release criterion. Automated
qualification may establish only the listed correctness, host/editor conformance, performance,
memory, cleanup, and security gates; it makes no measured human usability or comparative speed-
improvement claim. The 2026-08-23 evidence note that marked user outcome `NOT_RUN` because it
required a human study remains historical evidence, superseded only as a prospective release
criterion.

Any source/path/secret/receipt/test-output upload, telemetry, unauthorized workspace read/process,
shell interpretation, PATH relookup after pin, executable identity bypass, silent binary install or
replacement, automatic code edit, false passing test, silent truncation, stale result presented as
current, process residue, or authority upgrade is an immediate release blocker. If the adapter
requires its own graph/index, a webview/database, or editor-specific Corvint semantics, remove it and
retain CLI/MCP.

## 9. Traceability

| Requirements | Implementation surface | Required evidence |
|---|---|---|
| `VSC-V0-001..005` | manifest, activation, snapshot model | manifest/static audit; multi-root and mixed-snapshot tests |
| `VSC-V0-006..010` | discovery and pin manager | discovery order, version, symlink/swap, hash, and no-relookup vectors |
| `VSC-V0-011..015` | trust/root/remote/environment gates | trusted/untrusted Electron runs; remote and environment-spy fixtures |
| `VSC-V0-016..020` | bounded process runner and decoder | exact invocation, cancellation, timeout, output, malformed/hostile JSON, residue tests |
| `VSC-V0-021..027` | commands, status, TreeViews, diagnostics, decorations | VS Code integration tests, accessibility assertions, deterministic UI snapshots |
| `VSC-V0-028..033` | observation importer and VS Code test API bridge | contained-file, identity-drift, sequence/terminal, mapping, cancellation, no-RunProfile tests |
| `VSC-V0-034..038` | privacy, storage, update, lifecycle policy | source/dependency scan, network/storage spies, VSIX inspection, disable/uninstall residue |
| `VSC-V0-039..045` | conformance and release gates | hostile JSONL corpus, unit/process/Electron results, measurements or explicit `NOT_RUN` |
| `VSC-V0-046..057` | transport selector and MCP 2026-07-28 stdio client | independent discovery/tool/schema/pagination/frame/cancellation/clean-EOF/runtime vectors |
| `VSC-V0-058..070` | live test-provider adapter (`liveTestProtocol.ts`, `liveTests.ts`, `process.ts` process-group spawn) | stream-parsing, vectors.json mapping, stale-supersession, diagnostics-ledger, disabled-by-default, process-group-residue (including a `SIGTERM`-ignoring descendant and awaited shutdown), detached-provider output, post-overflow resync, unavailability-explanation, and opt-in retention-argv unit/process tests; real-provider Electron run, with or without retention, remains `NOT_RUN` |
| `VSC-V0-071..076` | `liveTests.ts`, provider process lifecycle, save routing and restricted settings | Controller regressions cover retained JS results, invalid framing, save races, scope/trust/toolchain refusal and cleanup; real EOF-aware and normal-exit descendant checks pass. Installed VSIX/real-host qualification remains pending under PUB-V0-016. |

Focused local evidence on 2026-08-23:

- `npm ci --ignore-scripts` completed from the lockfile; npm audited 290 installed development/
  package dependencies with zero reported vulnerabilities. All 304 non-root lockfile package
  entries are development-only and declare a license; the VSIX has zero npm production
  dependencies. npm retained deprecation warnings for transitive packaging dependencies
  `whatwg-encoding@3.1.1` and `prebuild-install@7.1.3`; neither is in the VSIX/runtime.
- `npm run check` passed real Biome lint with warnings fatal over 15 files, strict TypeScript
  typecheck, compilation, and 20/20 Node unit/process tests. The tests cover executable pin/realpath,
  exact/over output bounds, pre-cancellation, lifecycle draining, filesystem-error redaction,
  modern MCP discovery/tool/result/cancellation, wrapper/native authority correlation, null-receipt
  abstention, repository identity coupling, server-version drift, toolset and interaction rejection,
  MCP-only input preflight, strict hostile JSON, deterministic projection/receipt identities, unsafe
  identity/blob widths, and fail-closed observation import.
- `node --test conformance/vscode-extension-v0/run.test.mjs` passed 9/9 oracle-runner tests over 73
  frozen cases, including 18 MCP-specific cases. This validates the independent corpus/driver, not
  a real Extension Host.
- `npm run package:vsix` passed after the same check and produced a 17-file, 45.4 KiB developer
  VSIX with SHA-256 `63bb2851b3341581032a5143a2a587563fe1029d4a4257e25cacd2c4cba92518`.
  Inspection found only manifest metadata, the exact license, README/changelog, ten compiled
  runtime JavaScript files, and the activity icon; source, tests, source maps, lockfile, tooling,
  and `node_modules` are absent. The packager honestly warned that repository metadata is missing
  because no correct public remote is configured.
- A targeted source and packaged-JavaScript scan found no webview, telemetry/analytics, Node
  network/listener, workspace edit, code-action, terminal, persistent VS Code state, shell-true,
  private-key marker, or credential-key surface. This is static evidence only; hostile runtime
  network/storage/effect spies remain part of the unrun Extension Host matrix.
- One real local compiled VS Code MCP client-to-compiled `corvint-mcp` impact smoke completed and
  preserved the mixed-worktree state (then reported as `STALE_INDEX`, now carried in
  `freshness.state=mixed-worktree`) with eight evidence rows. It is a narrow transport smoke, not an installed
  extension, official interoperability, or promoted compatibility tuple.

`@vscode/test-electron` trusted/untrusted installation, real public-surface host driving, remote
host, Windows/web/virtual workspace, 1,000-cycle residue, performance/memory, user-outcome, positive
Test Explorer import through a shared Corvint verifier, packaged/digest-pinned MCP tuple, and official
MCP client conformance remain `NOT_RUN`. None of these has a bare reason: `node_modules` is not
installed in this checkout and installing it needs a network fetch this lane will not perform, so no
`@vscode/test-electron` run, real Extension Host, 1,000-cycle residue check, or Extension-Host
performance/memory measurement can execute here even before its VS Code download; remote host,
Windows, and web/virtual-workspace runs additionally need a host this lane does not have; the
user-outcome gate needs a human study, not a command; Test Explorer import needs a shared Corvint
verifier that does not exist yet; and the packaged MCP tuple and official MCP client conformance need
both a packaged `corvint-mcp` binary (`docs/specs/mcp-server-2026-07-28-v0.md` §Unresolved decisions)
and an official MCP client-side conformance suite, neither of which this repository vendors.

Experimental implementation paths now exist, but a file path, test name, static pass, or packaged
VSIX does not promote a compatibility tuple. Each unrecorded evidence cell remains `NOT_RUN` until
the exact command, environment, and result are retained.

## 10. Unresolved decisions

- Exact minimum/maximum VS Code and Node extension-host versions remain open until the first pinned
  Electron matrix is green.
- Windows execution remains unsupported until Corvint CLI parity and descendant containment are
  independently demonstrated; inert activation may still be tested.
- A future accepted live-test CLI may add an explicit RunProfile. V0 observation import cannot be
  silently promoted into execution authority.
- Marketplace publisher identity, signing, release cadence, and supported remote surfaces require
  owner approval before publication.
- The experimental `corvint-mcp` binary must be packaged/installed and its inherited Git executable-
  pinning blocker closed before the MCP client can move beyond `NOT_RUN`.

## 11. Official VS Code references

Retrieved through Context7 library `/websites/code_visualstudio_api` on 2026-08-23:

- [VS Code API reference](https://code.visualstudio.com/api/references/vscode-api)
- [Workspace Trust extension guide](https://code.visualstudio.com/api/extension-guides/workspace-trust)
- [Tree View extension guide](https://code.visualstudio.com/api/extension-guides/tree-view)
- [Testing API extension guide](https://code.visualstudio.com/api/extension-guides/testing)
- [Extension testing and `@vscode/test-electron`](https://code.visualstudio.com/api/working-with-extensions/testing-extension)
- [Remote extension architecture](https://code.visualstudio.com/api/advanced-topics/remote-extensions)
- [Publishing extensions](https://code.visualstudio.com/api/working-with-extensions/publishing-extension)
- [MCP 2026-07-28 specification](https://modelcontextprotocol.io/specification/2026-07-28)
- [MCP 2026-07-28 release notes](https://blog.modelcontextprotocol.io/posts/2026-07-28/)

These references establish current host APIs, not Corvint compatibility. Only the published Corvint
matrix and evidence above may make a support claim.

### Conformance observation coverage (2026-09-15)

The independent runner manifest now names VSC-V0-001..076. Live-provider vectors require
ordered public snapshot/diagnostic observations and selected fixture/OS lifecycle events; mutation
tests reject missing fields, false passes, surviving groups and shortened debounce intervals.
This is checker coverage, not installed-host qualification. The oracle returns NOT_RUN for these
live vectors. Stable public VS Code APIs do not expose TestRun messages/output for independent
reading; advisory-text and cause-message rows stay NOT_RUN without genuine host/UI observations.

The historical ENOTCACHED attempt above remains historical. The current dependency lock includes
@vscode/test-electron 3.1.0; extensions/vscode/test/installed/run.mjs pins Code 1.137.0 and Go 1.27.1.
One isolated development-host fixture proof at ebf715049c010315606261c4ee8a451c65fc27ad observed
a failed JS item, exact record digest, real diagnostic and absent provider group. It does not
qualify packaged artifacts or the full trusted/untrusted matrix. Final integrated canonical and
retained installed gates remain mandatory; focused editor checks do not replace them.
