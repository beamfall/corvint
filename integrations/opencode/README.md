# Corvint for OpenCode

This adapter (its version is `version` in `package.json`) admits unmodified OpenCode releases from
2.0.18 through the end of the 2.2 series for qualification; 2.1.3 is the tested host version.
2.2.0 passes the native-only campaign but has no qualification record yet.
It supplies awaited task context, on-demand
query and exact source expansion, edit/evidence/verification observations, explicit outcomes, and
compaction recovery from current Git state. OpenCode 1.x requires the older 0.2.9 adapter.

Call `corvint_status` (available after the first `corvint_context` call) to check the installed package, host executable, Corvint executable, OS and
architecture against `../opencode-qualification.json`. Only an exact passing tuple reports
`integrationSupport: FULL`; changed or untested builds report `UNQUALIFIED` and tell the operator
to run `tools/qualify-opencode` for that installation. The range is producer admission, not reused
evidence: each 2.0.x, 2.1.x or 2.2.x host must pass the campaign and receive its own record, including its exact
version and executable digest. Releases before 2.0.18 and 2.3.0 or later fail closed as unsupported.
Qualification is
maintainer test evidence, not execution attestation. Its scope is the stock native integration
under AHI-032, including the retained functional, latency and critical-evidence recall checks.

Execution authority is `NONE`. Core lifecycle receipts remain `FALLBACK`, and
`frontier-authority-unavailable` describes that authority boundary. Completion observations are
advisory; the plugin cannot enforce completion or keep the CLI alive for a continuation. No fork,
replacement OpenCode distribution or privileged authority service is installed.

The runner supports macOS and Linux process groups. Only tuples in the qualification report are
qualified; Windows remains unsupported until process-tree cleanup is implemented and tested.


## Change

`/corvint` opens the Change view. Follow a changed file into its affected-test dependency
paths, inspect recorded check commands/output, and open the existing evidence reader for context.
No model call or test execution is needed to inspect these records.

| Key | Action |
| --- | --- |
| 1 / 2 / 3 / 4 | Files / Impact / Checks / Attention |
| Up / Down; Page Up / Down | Select rows or scroll details |
| Enter / click | Follow file impact or read a selected check's output |
| Tab | Switch list/details in a compact panel |
| x | Clear the selected-file impact filter |
| b | Choose a branch, tag or commit to compare from |
| r | Refresh the observed change and verification state |
| e | Inspect context for the selected file, or open evidence |
| f; Escape | Toggle full-screen; close |

The base is pinned to the recorded workflow's base when present, otherwise HEAD. Selecting HEAD
shows working-tree changes; another ref includes its committed range and current worktree changes.
Affected tests are advisory selections. Declarations are displayed separately from execution results.
A passing individual check does not establish workflow completion or complete test coverage.
Recorded-check reads display Core's qualification, tested/current commits and bounded local output; stale,
unrun, failed and withheld observations remain explicit. Logs are caller-owned local records, not
attested execution. Missing records and unsupported impact remain visible under Attention. Permanent advisory/authority limitations remain available in Attention details and do not inflate its actionable count.

Refresh is explicit. The cockpit does not poll, execute displayed commands, refresh indexes, scan
transcripts, or write evidence. Observed host edits invalidate its view; external edits require a
refresh. Reads use existing Core commands and the current worktree's private completion owner.
Output is capped at 64 KiB per stream; each file/impact/check list at 64 rows, with omissions shown.

## Work queue

The sidebar shows the latest observed Corvint Tasks totals. Click **Work**, run
`/corvint tasks`, or press **t** in the change view to open the task panel. The panel shows
completed, open, draft, held and archived ticket counts; queue blockers, passed intent checks and active
attempts; and pages of 32 open tickets. Enter or click a ticket to read its blockers and acceptance
criteria. Use **r** to refresh, **←/→** to page, and **c** to return to the change view.

This is a read-only view of an initialized native `.taskman` store. The `corvint-tasks` executable
must be on `PATH`, or plugin option `tasksBinary` / environment `CORVINT_TASKS_BIN` must name it.
The panel uses only `queue status`, `ticket search --status OPEN`, and `ticket show`. Counts and
eligibility are observations of the store; gate results and completion remain separate evidence.
Its observed time and queue receipt are shown so an external store change calls for refresh.
Uninitialized, refused and changed reads show an unavailable reason, not zero tickets.

## Focused Work

From an open ticket in the Work queue, press **s** or click **Focus task**. Run
`/corvint workbench` to reopen it. The sidebar shows the focused ticket, its observed
criterion count and next-action count. Focused Work has **1 Task**, **2 Criteria**, **3 Blockers**, and **4 Gates**. Press **m** for More, then **d Doctor**, **o Cost**, or **u Sessions**. On Task, use **↑/↓** and **Enter** for observed next actions; **Tab** switches between actions and scrolling details. Press **r** to refresh, **t** for the Work queue, **c** for Change, or **e** for the session’s current Evidence view. This Evidence route does not request focused-ticket or criterion-specific context.

Focus is volatile and scoped to the OpenCode session and Tasks queue receipt. Task shows
the ticket, current branch/worktree, live attempt/lease, blockers and next actions. Criteria shows
the ticket's acceptance criteria and explicit requirement references beside declared paths,
observed changed paths, suggested checks, recorded checks, and context/change receipts. These
are task-level leads: no criterion is marked satisfied without an explicit criterion-specific
proof link. A changed queue receipt makes the workbench stale and requires selection again.

Gates shows native declared gate IDs and the native summary/completion observation separately from local workflow checks. Missing individual gate observations remain NOT_OBSERVED; a local pass never becomes a native gate pass. Missing declarations and an explicitly empty list remain distinct. Blockers shows native blockers and eligibility without treating no listed blockers as readiness.

Doctor displays the exact host/adapter/platform qualification state, available failed
conformance cases and the command to qualify this installation. Cost shows OpenCode's actual
session cost and tokens. Cost since focus is shown only when this TUI observed a cost baseline
for that exact selection; it is not a savings estimate or cost per verified criterion.
Sessions lists up to 16 members of the current OpenCode session family, their explicit Corvint
ticket focus where visible, worktree, observed lease, and overlapping focused tickets. An
unbound session may still be working. Other worktrees may not expose their Corvint focus through
this plugin location.

The workbench runs no task mutation, test, transcript scan, or background poll. It reuses the
bounded read-only Tasks and Change view commands and keeps only volatile session data.

## Context inspector

The native terminal sidebar shows **Corvint context** for the current session. Click it or run
`/corvint` to open Change and press e for evidence. Run `/corvint Locate Add in add.go` to request context
without asking a model. The panel also accepts these keys:

| Key | Action |
| --- | --- |
| Up / Down | Select evidence in the list, scroll in the source/details pane |
| Enter / click a location | Open its pinned source through Core |
| Tab | Switch between evidence and source without losing selection |
| / | Find evidence by file, symbol, reason or authority |
| x | Clear the filter and return to the list |
| Page Up / Page Down | Move through evidence or scroll a page |
| Left / Right | Scroll long source lines horizontally |
| h | Enable host syntax highlighting (may download a language parser), or return to plain text |
| l | Return to the cited source line |
| g | Show gaps and limitations |
| i | Show the full inclusion reason, summary and immutable identities |
| r | Request context for an explicit task |
| f | Toggle full-screen; wide panels show evidence beside source |
| c | Return to the Change view |
| Escape | Close the panel |

Each location shows its inclusion reason, authority and confidence. The source reader preserves
Core's tree/blob verification. Control characters are escaped for display. Freshness describes the
last observation; this UI does not continuously check Git. Observed edits or compaction mark the
view stale until another context request. Missing/unsupported evidence stays visible. Source uses native line numbers and a cited-line marker. Plain rendering is immediate and requests
no syntax parser. Press h or click Enable syntax to opt into OpenCode highlighting for this panel;
the host may download missing language parsers. Source text is parsed locally.
Unrecognized languages stay readable as plain text. Long lines scroll horizontally to preserve
line alignment. At 96 or more panel columns the list stays beside the source; compact panels use
Evidence/Source/Gaps/Details views. Theme colors always have text labels. Selection survives
receipt reorder, while source is cleared when its receipt or observed state changes.

The view is bounded and held in memory, with no receipt or prompt journal. Its RPC runs through the
existing OpenCode server and inherits that server's client trust boundary. The terminal plugin
receives only context/evidence views and hashed-session update notices, not transcripts. Removing
or disabling the plugin disposes the view and its subscriptions. Desktop/web custom panels, revision comparisons, review mode and portable handoff remain outside this slice.

`go run ./tools/qualify-opencode --inspector --host /absolute/opencode --corvint /absolute/corvint
--output /absolute/evidence` exercises the actual 2.x terminal in isolated temporary Git and XDG
locations, without a model call. It requires Python 3, Git, a POSIX PTY and permission to start the
host's temporary loopback server, plus Go for the recorded fixture test. Its UI witness is separate from full native integration
qualification; it never writes `opencode-qualification.json` or grants execution authority.

## Install, discover, upgrade, disable, uninstall

The package is not published to npm yet. OpenCode supplies the terminal renderer and Solid runtime. From a Corvint checkout,
add a `file://` URL for its `src` directory to OpenCode's `plugins` array:

```json
{
  "plugins": [
    { "package": "file:///absolute/path/to/corvint/integrations/opencode/src", "options": {} }
  ]
}
```

Replace the absolute path with your checkout. `opencode.example.json` uses this local-source form;
a plain string entry works when no options are needed. OpenCode 2.0.18 loads a local entry only
from a directory, through its `server.js` or `index.js`: an entry naming `src/index.js` itself is
skipped with the warning "configured plugin path must be a directory", and one naming
`integrations/opencode` is dropped without a message. Configure the adapter once to avoid registering it
twice. Upgrade by updating the checkout. Disable or uninstall by removing the `plugins` entry;
remove the checkout only when it is no longer needed.

## Qualify this installation

From a clean, committed Corvint checkout, run the first-party command below. It requires Git,
Node.js 20 or later, Go 1.27.3 (decision 0557), the real Corvint executable, and the actual
OpenCode 2.0.18 or later 2.0.x, or any 2.1.x, executable on macOS or Linux. If `opencode2` is a shell launcher, pass the
executable it launches; the command rejects shell launchers rather than recording the wrong image.

```sh
GOTOOLCHAIN=local go run ./tools/qualify-opencode \
  --host /absolute/path/to/opencode \
  --corvint /absolute/path/to/corvint \
  --output /tmp/corvint-opencode-qualification
```

This command runs the focused adapter/cleanup tests and the real native campaign itself. It freezes
package, executable, complete Go collector (including its running binary) and test inputs across both gates, then atomically writes
`integrations/opencode-qualification.json`, exactly where `corvint_status` reads it. It retains the
same accepted report and gate logs under `--output`, which must be outside the checkout. No manual
conversion or supplied PASS assertions are accepted. The lower-level
`go run ./tools/qualify-opencode --native-only` produces native campaign evidence only and cannot qualify an
installation by itself.

A requalification saves the previous record with its new evidence and first sets the active record
to INCOMPLETE. Failure or interruption therefore leaves native support UNQUALIFIED. Only complete,
unchanged passing evidence replaces that record with PASS. The local record is ignored by Git;
cloning this source does not transfer qualification to another machine. Run the command there for
its exact tuple. If installing a copy of the package, copy it unchanged together with the record at
its sibling `opencode-qualification.json` path. Then restart OpenCode and call `corvint_status`.
Execution authority remains NONE, Frontier UNAVAILABLE, and legacy receipts FALLBACK.

## MCP servers

MCP servers are configured separately from the lifecycle plugin. Copy the entries from
`mcp.example.json` into the `mcp.servers` object in `opencode.json`, replacing executable and repository
paths. Each command needs its own absolute repository root. The strict MCP profile currently
requires a repository with a `.git` directory; linked Git worktrees are not admitted.

All Corvint MCP servers default to the latest published MCP protocol, `2026-07-28`.
OpenCode clients using the older `initialize` handshake need the explicit
`--protocol-version 2025-11-25` compatibility option shown in the example. Without it,
initialization fails with a `-32602` answer that names that flag (`MCPV0-820`; a Corvint before
it answered `Method not found`). This selection changes transport negotiation only;
repository bounds, tool schemas and Corvint receipts stay the same. Modern clients should omit
the selector or select `2026-07-28` explicitly.

The example enables `corvint-mcp` and `corvint-test-validity-mcp`. Build them from this checkout
with `go build -o /absolute/bin/corvint-mcp ./cmd/corvint-mcp` and
`go build -o /absolute/bin/corvint-test-validity-mcp ./cmd/corvint-test-validity-mcp`, or use
release binaries containing the compatibility selector. Verify discovery with `opencode mcp list`;
a connected status alone does not verify a tool call or qualify native lifecycle support.
Development probes with OpenCode CLI 1.17.18 and 1.18.31 connected the core, test-validity and
docs servers built from this checkout and completed one call per tool through this selector; both
clients send `notifications/cancelled` after each completed call, which the server ignores. These
probes are transport evidence only, not a host-version support claim. On OpenCode 2.1.3 a retained
live probe (`conformance/opencode-native-v1/results/2.1.3-darwin-arm64/mcp-probe.json`) loaded the
core server from a project `opencode.json` `mcp.servers` entry and completed a `corvint_status` call
returning `READY`; a global-configuration probe (`mcp-probe-global-config.json`) listed the tools the
model sees only through Code Mode: `tools.corvint.corvint_status`, `corvint_help`, `corvint_impact`
and `corvint_query`. OpenCode 2.1 connects MCP
servers asynchronously, so the first model turn may report `Unknown tool namespace 'corvint'`; a
later turn sees them. OpenCode 2.1 defaults to its `legacy` MCP protocol (revisions up to
2025-11-25), so keep the selector above; an entry may instead set `"protocol": "2026-07-28"` and
omit the selector, which a retained global-configuration probe also completed
(`mcp-probe-protocol-2026-07-28.json`). The test-validity and docs servers were not
probed on 2.x. The manifest's `testedHostVersions` names only versions whose native campaign and MCP
probe are retained under `conformance/opencode-native-v1/results/`; it is not a qualification record.
The optional docs and experimental corpus servers accept the same selector, but remain separately
configured companions with their existing prerequisites. Disabling a server uses `disabled: true`;
uninstalling it removes its `mcp.servers` entry.

See the [MCP transport contract](../../docs/specs/mcp-server-2026-07-28-v0.md) for the exact
supported profiles, recorded host probes and remaining qualification boundaries.

## Tool selection

The native plugin supplies general task context and explicit outcome observations. The core MCP
server supplies repository status, narrow workflow-authority queries and tracked-Go impact; the
separate test-validity server reads retained test evidence. Installing both surfaces makes these
complementary tools available without expanding either server's frozen registry.

The plugin registers the bundled `corvint` skill through OpenCode's native skill registry on the
first `corvint_context` call (`AHI-212`), so OpenCode lists it in the model's available skills with
no copy step. A project, user or
earlier plugin skill named `corvint` takes precedence and is never replaced; a skill denied by
permission stays hidden. An unreadable bundled skill or a failed registration is reported as a
warning and leaves the tools and hooks running. Copying `skills/corvint/` into `.opencode/skills/`
remains a way to pin or edit the guidance. It explains which tool
answers each question, how to interpret unsupported/missing evidence, and when to use the CLI.
The skill is loaded on demand through OpenCode's native skill tool. The optional documentation
servers are useful when working with their supported draft or corpus artifacts; they are not
prerequisites for ordinary code context and test evidence.

## Lifecycle options

Corvint must be available as the `corvint` executable. Plugin `options` may set `corvintBinary`,
`tasksBinary`, `hostVersion`, `automaticTimeoutMs`, `queryTimeoutMs`, and `enableBetaContext`; equivalent explicit
environment settings are `CORVINT_BIN`, `CORVINT_TASKS_BIN`, `CORVINT_OPENCODE_HOST_VERSION`,
`CORVINT_OPENCODE_TIMEOUT_MS`, `CORVINT_OPENCODE_QUERY_TIMEOUT_MS`, and
`CORVINT_OPENCODE_BETA_CONTEXT=1`. Values are never added to lifecycle payloads. The child process
receives only a small non-secret environment allowlist.

Explicit `corvintBinary`, `tasksBinary`, `hostVersion`, `automaticTimeoutMs`, and `queryTimeoutMs` options take
precedence over the ambient `CORVINT_BIN` and `CORVINT_OPENCODE_*` variables. Without a `hostVersion`
option the version OpenCode reports to the plugin is used, then `CORVINT_OPENCODE_HOST_VERSION`.

Automatic events and explicit context queries default to 2,000 ms. Automatic overrides accept
integers from 25 to 2,000 ms; query overrides accept 25 to 10,000 ms. Invalid values use the default.
These are complete-command deadlines, separate from latency targets. A timeout warning includes
the applied deadline; it does not diagnose the underlying cause. `FALLBACK` also appears on successful
receipts when authoritative frontier evidence is unavailable, so inspect `ok` and the named code.

The `session.created`, `session.execution.succeeded`, `session.execution.failed`, `session.idle`
and `session.deleted` events and the tool `execute.after` hook are translated to
`corvint harness event`. Raw session IDs are hashed; raw prompts, transcripts, tool arguments, tool
output, and environment maps are never sent. A completed built-in `write`, `edit` or `patch` call
reports its target files as a session-scoped `file-change`; the adapter serializes those
subprocesses and coalesces duplicate paths for one session so an edit burst cannot overlap Corvint
invocations. An `unsupported-impact-path-suffix` or `unsupported-impact-repository` refusal from that
best-effort event is recorded at info level instead of as a warning. So are
`post-tool-path-not-project-relative`, when a tool call reports a path outside the project,
and `changed-paths-truncated`, when a call, session or file-change batch passes its 256-path
cap. Opened in a directory outside
any Git repository, the plugin registers no hooks or tools and runs no Corvint command (decision
0378). OpenCode 2.1 exposes a plugin tool to the model only through its code-mode `execute` tool
unless the tool declares `options.codemode: false`. Only `corvint_context` does (adapter 0.7.23,
`AHI-211`), so the model calls it directly; `corvint_expand`, `corvint_record_outcome` and
`corvint_status` stay in the Code Mode catalog. Since adapter 0.7.26 (`AHI-212`) those three and the
bundled skill are added only after the first `corvint_context` call; OpenCode appends them to the
conversation as catalog and skill-change notices, so an idle session carries only `corvint_context`
(measured +97 first-step prompt tokens on 2.1.3). A code-mode `execute` call is not reported, its inner
tool calls are. Code Mode JavaScript has a global `fetch`, so since adapter 0.7.28 (`AHI-214`) the
plugin refuses every `execute` call (`code-mode-network-denied`) unless the session's effective
permission rules allow both `webfetch` and `websearch` for every resource; a host whose rules it
cannot read is refused too. A refused session loses the Code Mode follow-ups, as with
`permission.execute: "deny"`, which is what a host or trial profile denying network should also set
so the model never sees `execute`. The
prompt hook and `corvint_context` tool query the current bounded task. A task over the
2,000-character/16,384-byte bound is served by its disclosed anchor query (`AHI-016`) or refused as
`prompt-over-query-bound`, never truncated. Only the `metadata.corvint` namespace can contribute evidence handles or verification observations.
`corvint_record_outcome` records a caller-reported `passed`, `failed`, or `blocked` outcome only when
the caller also supplies a bounded task, changed paths, and closed-schema verification observations.
The task is hashed locally; only `taskSha256` enters the non-persistent session-end receipt.

The optional `enableBetaContext` setting repeats cached startup context. Default prompt context
and compaction recovery use awaited native hooks; all injected additions are bounded and framed.

## Automatic context and exact expansion

The prompt hook awaits Corvint and keeps the submitted prompt text unchanged. Since adapter 0.7.29
(`AHI-215`) it injects nothing by default. A frame reaches the model, through the context hook and
outside the visible conversation, only when the packet is `READY`, Core does not abstain, at least one
candidate carries more than syntax authority (`project-instructions`, `instruction-reference`,
`repository-spec` or `git-history`), and the prompt does not already name an existing checkout path.
Otherwise the adapter records `prompt-context-withheld`, `prompt-context-syntax-only` or
`prompt-names-path` at info level; the query still runs for the inspector and the stop step. An
emitted frame is a fixed line saying the candidates are not proof and naming `corvint_context`, then
the query's receipt link (`cvr:` and 24 hex digits, which `corvint_expand` resolves to the ranked packet in the same session; `AHI-219`) and up to five candidates (path, line, symbol, authority) from distinct files in score order, with a
name declared in several non-test files demoted, and the omitted count; the whole frame stays within
600 UTF-8 bytes. For a prompt over the query bound, the hook drops markup tag names, absolute paths
and punctuation-only anchors from the derived query and states the kept and dropped counts in one line.
The full
packet is not injected on every step; the model requests it when the task needs it. Each frame
stays within 8000 UTF-8 bytes, including framing and any anchor-query disclosure. Repeated events/messages are deduplicated; new prompts
invalidate prior task context even when busy, and deleted or evicted sessions cancel pending work.
Compaction discards the previous task frame. The adapter writes no prompt files and reads no transcript.

`corvint_context` returns `expansionHandles` encoded from the evidence's pinned Git tree/blob/path.
Pass a handle to `corvint_expand`; replace its `all` range with an explicit `START-END` when a full
file exceeds the 6000-byte source cap. The core verifies identity and returns exact selected bytes;
invalid, stale, oversized or unsafe output is refused. Selection does not read changed worktree bytes.

After `session.compaction.ended`, the next context hook awaits fresh `startSource: compact` context.
Failure keeps recovery pending and never restores an old packet. Recovery covers current dirty
paths; it does not reconstruct previous conversation decisions. `enableBetaContext` additionally
repeats startup context on model requests; ordinary task context and compaction recovery are enabled
by default.
