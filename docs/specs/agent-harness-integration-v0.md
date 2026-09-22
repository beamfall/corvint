# Agent Harness Integration V0

Owner: Russell Lewis
Date: 2026-08-23
Intent status: accepted direction
Delivery status: experimental
Authoritative inputs: `docs/PRODUCT.md`, `docs/TECHNICAL-BRAIN.md`,
`docs/specs/cem-0.2-canonical-binding.md`

## Agent digest
- Claim: Corvint exposes a shared bounded lifecycle-event contract whose native harness adapters remain experimental and unqualified.
- Status: accepted direction/experimental
- Exists: `internal/gokernel`, `cmd/corvint`, and native adapter previews.
- Blocked on: black-box release-matrix qualification with accepted closing authority.
- Read next: `harness-authority-relation-v0.md` (superseded by accepted decision 0009 option 2; no execution authority root) and `change-frontier-profile-1.md`.

## User and measurable job

An engineer can install Corvint into Codex, Claude Code, Gemini CLI, OpenCode, Pi, or DeepSeek
Harness and receive the same fast, revision-pinned evidence loop without manually searching or
maintaining harness-specific knowledge. V0 succeeds only when each claimed platform passes the same
black-box conformance suite on its supported release matrix.

MCP is the portable floor. Support is reported per
`(host, surface, host version, adapter version, OS)` as `FULL`, `FALLBACK`, or `UNSUPPORTED`; a
product name alone is never `FULL`. A surface is `FULL` only when Corvint ships and tests its native
lifecycle adapter. A missing capability, failed case, or untested release is `FALLBACK`.

## Verified current state

- Corvint exposes revision-pinned `query`, `impact`, CEM, OCM, LRF, and explicit outcome commands.
- Corvint does not yet expose an accepted closing Frontier or harness-owned execution authority.
- No native harness package or shared lifecycle-event adapter was present at the base revision.
- Therefore every V0 package starts as `FALLBACK`; packaging or an MCP connection alone cannot
  promote it to `FULL`.

## Transport-neutral lifecycle contract

The native packages are thin translators over one Corvint command:

```text
corvint --root ROOT harness event \
  --host HOST --host-version VERSION --surface SURFACE \
  --adapter-version VERSION --event EVENT --input - --budget-bytes BYTES
```

`EVENT` is exactly `session-start|user-prompt|file-change|post-tool|stop|session-end`. Input is one
bounded normalized JSON object, never a raw host transcript or raw tool body. Session identity may
be supplied only as a SHA-256 digest. Prompt text is accepted only for the immediate `user-prompt`
query and is neither returned nor persisted. `post-tool` accepts only evidence handles, changed
repository paths, and verification metadata that the host actually exposed. `session-end` accepts
an outcome only when it is explicit and task text has already been reduced locally to
`taskSha256`. V0 returns a non-persistent observation receipt even in a dirty worktree; durable
outcome recording remains the separate explicit `corvint record` operation against a clean revision.
`session-start` may carry only the closed `startSource` enum
`startup|resume|clear|compact`. The Claude Code adapter maps that host's SessionStart source `fork`
(a resumed transcript under a new session id, in the Claude Code 2.1.267 hook input schema) to
`resume` rather than refusing it as `invalid-start-source`. On `compact`, Corvint rehydrates a bounded impact packet from the exact
dirty paths that exist at the pinned revision. Untracked paths are reported by count and digest but
cannot become revision-pinned impact evidence; a dirty set that cannot fit degrades explicitly.
This repository-derived recovery never reads a transcript or persists a prompt, decision, or model
output, and does not claim full task-state restoration.

Every valid response uses `profile: corvint-harness-event/0`, identifies the exact adapter tuple and
repository revision/worktree state, and content-addresses the normalized receipt. A valid degraded
response exits zero and says `support: FALLBACK` with named degradations. Invalid input or an
unavailable Corvint operation exits two; a native wrapper converts that into a visible host-valid
no-op and never blocks unrelated coding. Until a separately accepted closing authority exists,
`stop` returns `frontier.state: UNAVAILABLE`, `shouldContinue: false`, and cannot continue or block
the host through that legacy profile. Accepted decision 0009 option 2 separately permits the opt-in
local completion policy in `local-completion-policy-v0.md`; its new envelope and native Stop policy
do not reinterpret this Frontier result.

## Requirements

- `AHI-001`: Every adapter MUST consume and produce the transport-neutral Corvint request, receipt,
  CEM, frontier, and outcome contracts. It MUST NOT maintain a second graph or reinterpret authority.
- `AHI-002`: Installation, discovery, upgrade, disable, and uninstall MUST be documented and tested
  using the platform's native package mechanism.
- `AHI-003`: On session/task start, the adapter MUST resolve repository identity, exact Git
  revision/tree, worktree state, and access context before requesting a minimum-witness packet. A
  response's repository envelope and nested Corvint context MUST derive from one Corvint snapshot; a
  second index build may not silently mix freshness states. A tracked source excluded only because
  its committed blob exceeds the indexing byte bound MUST remain clean, not become a dirty-path
  observation. A
  host exposing post-compaction session start MUST rehydrate exact tracked dirty-path impact, expose
  untracked paths as an unresolved count/digest, or name why it could not; it MUST NOT inspect an
  unstable transcript to reconstruct task state. The `corvint-dogfood-event/0` native adapters (Claude
  Code and Codex) meet this on a compact session start over a dirty worktree by carrying the
  compaction block of the legacy harness profile under the prompt packet's `compaction` key, compiled
  from the same index as the packet and the receipt's repository envelope, with at most half the
  response budget. Its `compaction-*` degradation codes join the receipt's `degradations`; an impact
  failure rejects the event with its engine code (`corvint-event-rejected:<code>`). A clean compact
  start carries no `compaction` key. The protected `corvint-qualified-lifecycle/0` profile carries the
  same block and appends the same codes after its support code (`QLF-V0-005`, decision 0241).
- `AHI-004`: Query and exact expansion operations MUST be available on demand. Automatic context
  injection MUST be bounded, receipt-linked, deduplicated, and observable to the user. A project-
  operations query MUST close over matching repository-owned instructions and their pinned
  references before product feature/scenario vocabulary can lead the packet. Every adapter MUST
  frame injected repository-derived content with the same byte-identical fixed envelope identifying
  it as untrusted repository data rather than instructions and labelling repository-authored free-
  text fields. If the repository-derived payload itself contains the envelope's own closing line, the
  adapter MUST refuse to inject that context rather than emit an envelope the payload can close early;
  it MUST NOT mangle or strip the collision silently, and it MUST surface the refusal as
  `corvint-envelope-terminator-collision`. Before that check, the adapter MUST rewrite every C1
  control (U+007F-U+009F), U+061C, U+200B-U+200F, U+2028-U+202E, U+2060-U+2064, U+2066-U+2069 and
  U+FEFF in the payload as literal lowercase `\uXXXX` text, which leaves the JSON value unchanged.
  This one rule binds every envelope builder: the Go builder `internal/repoenvelope` (Codex and
  Claude Code adapters, the qualified-lifecycle native output, `corvint source-view` output
  (`ESV-V0-004`), the `corvint-docs-mcp` tool-result text block (`SDD-V0-006`), the `corvint-mcp` tool-result text block
  (`MCPV0-008`), and the
  native-hook-observer comparator that re-derives it) and the JavaScript builders (gemini-cli hook, OpenCode system-prompt
  hook and the OpenCode `corvint_context`/`corvint_record_outcome` tool outputs).
- `AHI-005`: Session capture MUST record only Corvint evidence handles actually supplied or observed,
  explicit change identities, verification observations, and explicit outcomes. It MUST NOT claim
  model-internal causality or infer that a tool result was read. Automatic lifecycle adapters MUST
  NOT persist prompt/task text.
- `AHI-006`: Before a platform declares a change task complete, its safest available stop lifecycle
  point MUST request a Corvint frontier check. Corvint MAY advise or block only where repository policy
  and the platform permission model explicitly allow it. A separately admitted PLE-V0-012 native
  qualification exercise MUST remain FALLBACK/UNQUALIFIED and explicitly scope all native Stops in
  one dedicated repository for at most 900 seconds; it cannot stand in for completed qualification.
- `AHI-007`: Change and merge lifecycle points MUST compute drift and re-verification against exact
  revisions. Harness events are observations, not proof that a merge or test succeeded.
- `AHI-008`: The adapter MUST obey the host's sandbox, approval, secret, environment, telemetry, and
  network rules. Missing permissions produce an explicit gap; adapters MUST NOT weaken host policy.
- `AHI-009`: Unsupported or changed host capabilities MUST fail closed for evidence capture and fail
  open for unrelated coding activity, with a visible degraded-capability receipt. No silent fallback
  may be labelled full support. The `host-adapter` translator reports an unrecognised hook event as
  the degraded reason `unsupported-hook-event`, and oversized or malformed hook input as
  `hook-input-too-large` or `malformed-hook-json`, each in a `systemMessage` that says coding
  continues (`cmd/corvint/host_adapter.go:39,121,125@b3341e84`). These are faults under `AHI-021`.
- `AHI-010`: Each release MUST publish tested host-version ranges, adapter and protocol versions,
  unavailable capabilities, known degradations, and the last conformance result. The adapter version
  in a published matrix row and in its shipped declaration identifies the host package the record
  describes, so it MUST equal that package's `AHI-020` manifest version, compared as an exact string;
  Codex's `+codex.<timestamp>` build metadata is part of the string (decision 0244). Host-version
  evidence from a native host validator ran against one package build, which a later bump does not
  re-validate: a matrix row whose evidence exercised a package names that build in
  `hostVersionEvidenceAdapterVersion` (the Claude Code declaration in `lastValidation.adapterVersion`,
  the Codex declaration in `host.staticallyValidatedAdapterVersion`), or `unknown` when no committed
  evidence ties the run to a build.
- `AHI-011`: Native APIs MUST remain behind versioned adapters. Recognised host identifiers MUST
  come from the single versioned host-admission table embedded in the Corvint
  binary, never from worktree-readable runtime discovery. The table MUST preserve the
  `subset-of-recognised`/`refuse` degradation rule. `harness event` refuses a host outside that
  table with `unsupported-harness-host` and an event outside the `EVENT` set with
  `unsupported-harness-event` (`internal/gokernel/harness.go:346,349`). A host API change MUST NOT
  alter Corvint Core, receipt identity, CEM semantics, or stored evidence.
- `AHI-012`: The integration MUST meet a cold query p95 of 500 ms after the Corvint index is available
  and add no more than 250 ms p95 to non-query lifecycle events, excluding an explicitly requested
  Corvint operation. Missed deadlines degrade visibly rather than blocking the host indefinitely; an
  `corvint-invocation-timeout` line MUST name the deadline it exceeded and state that the deadline is a
  bound, not a diagnosed fault.
  OpenCode automatic events default to the existing 2,000 ms ceiling; valid explicit overrides
  remain 25–2,000 ms. Its `timeout` notice MUST carry `deadlineMs` and the same bound-versus-fault
  distinction. A completed `FALLBACK` receipt MUST retain its actual degradation codes.
- `AHI-013`: Default injected context MUST be smaller than the manual-search baseline at equal
  critical-evidence recall. Corvint MUST publish bytes and, where the host exposes them, measured input
  tokens; serialized bytes alone are not a token-savings claim.
- `AHI-014`: Host wrappers MUST NOT send raw session IDs, transcripts, environment maps, tool
  arguments, tool output, or secrets to Corvint. They MUST normalize only the minimum event fields
  admitted by `corvint-harness-event/0`. Native Go path normalization MUST resolve the project root
  (including symlink roots and platform aliases) and require targets strictly below it. If a target
  is absent, resolve and contain its nearest existing ancestor before appending suffix components;
  each component MUST differ from `..` and its appended path MUST return `ENOENT` from `Lstat`.
  A dangling symlink exists: resolve its target under the same rule, with at most 40 dangling-link
  expansions. Escaping ancestors or targets, unresolved roots, exhausted link bounds, and any
  other filesystem error MUST abstain as non-project-relative paths.
- `AHI-015`: Adapter packages MUST share the lifecycle command above, and host admission MUST derive
  from its embedded host-admission table rather than repeated host literals. Host-specific code may
  only translate native events and render native responses; it MUST NOT reimplement retrieval,
  receipt, frontier, or authority semantics.
- `AHI-016`: A native wrapper that receives a trimmed user prompt over either task bound MAY serve
  it with a derived query only under the closed rule in "Over-bound prompts" below. The query MUST
  consist solely of the prompt's distinct explicit anchors, copied verbatim in first-occurrence
  order, and MUST be the complete anchor set; the wrapper MUST NOT choose a subset. The injected
  context MUST carry a trusted disclosure, outside the untrusted-data envelope, that states the
  prompt's character and byte counts, both bounds, the anchor count and query length, and that the
  rest of the prompt was not queried and receives no claim. The disclosure MUST NOT echo prompt text.
  The wrapper MUST NOT store, hash to a retrievable pointer, or return the prompt, and MUST write
  no file. A prompt whose anchor set is empty or itself exceeds either bound MUST keep the
  `prompt-over-query-bound` refusal. A wrapper that does not implement this rule refuses. The
  Claude Code and Codex wrappers (`cmd/corvint/prompt_bound.go`), the Gemini CLI hook and the
  OpenCode `corvint_context` tool implement it, and MUST derive byte-identical task and disclosure
  text for every boundary case in `conformance/harness-event-v0/common-logical-interaction.json`.
- `AHI-017`: A native adapter that runs as a host-killed process MUST derive every Corvint deadline
  from the kill its shipped hook configuration declares for that invocation, never from a free
  constant, and MUST emit its visible degradation before that kill. The derived deadline is the
  declared kill, measured from the adapter process's own clock origin, less a process reserve for
  the wall time that clock cannot see (exec before it, output flush and exit after emit). The
  reserve is a measured loaded p95 plus a margin and is recorded beside the constant. The Gemini
  CLI hook subtracts `performance.now()` at invocation, so a slow Node start shortens the Corvint
  budget; under the declared kill its non-query hang detector applies when it is below that budget, and
  `compatibility.json` publishes the resulting query ceiling. The Claude Code and Codex adapters
  bound the invocation by a declared-kill table that MUST equal their `hooks.json`; a watchdog
  emits `adapter-host-kill-deadline` at the derived deadline while the work is still running, and
  the work context expires 100 ms earlier so a promptly cancelled event keeps its own reason. An
  invocation with no declared kill (OpenCode's in-process plugin, `adapter source-view`) keeps its
  existing hang detectors. The Claude Code `hooks.json` MUST NOT register a `FileChanged` group
  without a matcher, because Claude Code watches only the file names a matcher lists (or
  `watchPaths` a hook returns, which the adapter never does), so such a group never fires; Edit and
  Write changes already reach `PostToolUse`. `adapter claude-code file-change` therefore has no
  declared kill.
- `AHI-018`: A test harness that stands in for the host MAY state a longer host kill only through
  an explicit argument (`--corvint-test-host-kill-ms=<1..60000>` for the Gemini CLI hook), never an
  environment variable, and the shipped hook configuration MUST NOT pass it. An unrecognised or
  out-of-range argument degrades `unsupported-hook-arguments`; the declared kill otherwise applies.
  A stated kill bounds the Corvint budget exactly as the declared one does and also replaces the
  non-query hang detector, so the override cannot lift the adapter past the kill it states. OpenCode tests use its
  existing bounded options, whose maxima are production-clamped.
- `AHI-019`: A Claude Code `post-tool` event whose Edit/Write/NotebookEdit target (the
  `hooks.json` `PostToolUse` matcher) resolves outside the project root is not a project change.
  The adapter MUST still run the shared `harness event` call and exit zero, and its receipt MAY
  remain in machine-readable output or the local self-observation ledger, but the response MUST
  NOT carry a `systemMessage`. An in-root target's receipt ID MUST reach the model as
  `hookSpecificOutput.additionalContext` (`hookEventName` `PostToolUse`) and MUST NOT carry a
  `systemMessage` (decision 0161). That text MUST follow the receipt ID with `; ` and every
  degradation code the receipt carries, comma-separated, or a `+N more` count for codes beyond six or
  not matching `^[a-z0-9][a-z0-9-]{0,63}$` (the `integrations/compatibility.json` display rule). A receipt
  carrying any code outside that file's `receiptDegradationPolicy.recognised` set MUST instead be refused
  with the fault `corvint-degradations-unrecognised` in a `systemMessage` that names no code (`onUnrecognised:
  refuse`, decision 0232), as MUST a receipt whose present `degradations` value is not an array (JSON `null` included), on every event including `file-change`. The whole-receipt renderers apply the same
  refusal before framing a receipt: the Codex adapter's `SessionStart` and `UserPromptSubmit` receipt, in its
  existing fault shape, and the Claude Code `session-start` and `user-prompt` dogfood envelope, as that
  `systemMessage`; `stop` and `session-end` render no receipt and are unchanged. A `file-change` receipt MUST NOT carry either; `harness event`
  has already appended it, with its degradations, to the `SOL-V0-001` ledger. A `post-tool`
  event for a tool without a target path (outside the matcher) is unaffected either way.
- `AHI-020`: Each host package's own manifest version (`integrations/claude-code/plugins/corvint/.claude-plugin/plugin.json`
  and the matching entry in `integrations/claude-code/.claude-plugin/marketplace.json`;
  `integrations/codex/plugins/corvint/.codex-plugin/plugin.json`; `integrations/gemini-cli/gemini-extension.json`;
  `integrations/opencode/package.json`) MUST advance whenever that package's shipped content changes since its
  version was last bumped. Shipped content is every tracked path under the package directory except its version
  files and `README.md`, so a shipped file added later is covered; opencode's own published `files` list defines
  its set instead. This is distinct from `AHI-010`'s per-release compatibility publication: a host that
  caches an installed package by version (observed for Claude Code, `docs/build-log/2026-09-10.md`) never re-stages
  changed content under an unchanged version, so a stale version number silently withholds a fix or feature from an
  already-installed host. The Codex package's build-metadata suffix (`0.1.0+codex.<timestamp>`) advances by refreshing
  the timestamp; the other three packages advance by semantic-version bump.
- `AHI-021`: The Claude Code adapter MUST NOT emit a user-visible `systemMessage` for a routine
  receipt (`AHI-019`) or for an expected, non-fault degradation, and MUST keep one for every fault
  the user must act on (decision 0161). The expected set is closed: `prompt-over-query-bound`,
  `missing-prompt`, `file-change-path-not-project-relative`, `adapter-host-kill-deadline`, and
  `corvint-event-rejected:dogfood-event-deadline` (both time bounds, not diagnosed faults, per
  `AHI-012`; the dogfood event's own deadline normally expires before the watchdog's). Every other
  reason is a fault, including `corvint-event-rejected` and every other `corvint-event-rejected:<code>`, `malformed-corvint-output`,
  `invalid-input`, `project-root-unavailable`, `corvint-output-too-large`, the envelope collision code,
  `unsupported-hook-event`, `hook-input-too-large`, `malformed-hook-json`, and the host-payload
  contract reasons (`missing-session-identity`, `invalid-session-identity`, `invalid-start-source`,
  `invalid-stop-hook-active`). An expected reason on `session-start`, `user-prompt`, or `post-tool`
  MUST be written as `hookSpecificOutput.additionalContext` carrying the unchanged
  `Corvint FALLBACK degraded: <reason>; coding continues` text; on any other event it keeps the
  `systemMessage`, because that event has no model-visible channel and dropping it would leave the
  reason recorded nowhere. That additionalContext is the visible degradation `AHI-009`, `AHI-012`,
  and `AHI-017` require for these reasons. The `session-start` context carries no `systemMessage`;
  its fixed `frontier-authority-unavailable` degradation remains in the framed receipt, which
  carries no `host-version-unknown` (`AHI-023`). Claude Code records hook output in the local session transcript
  (`~/.claude/projects/<project-slug>/<session>.jsonl`, as `hook_additional_context`,
  `hook_success` and `hook_system_message` attachments), so a user sees recent adapter
  degradations with
  `rg -o 'Corvint FALLBACK degraded: [a-z0-9:-]*' ~/.claude/projects/<project-slug>/*.jsonl | sort | uniq -c`,
  and the ledgered `file-change` receipts and their degradation counts with
  `corvint --root <root> observations`. Every adapter degradation returned after root resolution
  is also a deduplicated, content-free `SOL-V0-010` ledger row, which the same command tallies as
  `ADAPTER-DEGRADATION` lines (decision 0169); the adapter still reads no host transcript
  (`AHI-005`). A hook whose working directory is not inside a Git repository (no `.git` entry at
  the resolved directory or any ancestor, where an unreadable level counts as inside) is an
  expected absence, not a fault (decision 0178). The Claude Code adapter and the Codex adapter
  (for an absolute payload `cwd`) MUST return `{}`, invoke no Corvint command and append no ledger
  row; every other failure keeps its fault notice. `AHI-022` applies the same split to the Gemini
  CLI and OpenCode adapters.
- `AHI-022`: The Gemini CLI and OpenCode adapters MUST apply the `AHI-021` split. A routine
  receipt and an expected, non-fault degradation MUST NOT reach the terminal, and every fault the
  user must act on MUST. The Gemini CLI hook's expected set is closed: `prompt-over-query-bound`,
  `prompt-unavailable`, `changed-path-unavailable`, and `host-kill-budget-exhausted` (the
  `AHI-017` time bound). On `SessionStart`, `BeforeAgent` and `AfterTool`, the hooks whose
  documented output accepts `hookSpecificOutput.additionalContext`, an expected reason MUST be
  written there with the unchanged `Corvint FALLBACK degraded (<reason>); unrelated Gemini work may
  continue.` text. On `AfterAgent` and `SessionEnd`, which have no model channel, it keeps the
  `systemMessage`. Every other reason is a fault and keeps the `systemMessage`, including a
  recursion guard, a schema, argument or cwd mismatch, a rejected, malformed, tampered or
  timed-out Corvint invocation, and the envelope collision. A successful Gemini receipt carries no
  `systemMessage`. The `AfterTool` receipt ID and its named degradations reach the model as
  `AfterTool` additionalContext. `AfterAgent` and `SessionEnd` return only `continue`, and
  `SessionStart` and `BeforeAgent` keep only their framed context. Outside a Git repository, as
  `AHI-021` defines it, the Gemini CLI hook MUST return `continue` with `suppressOutput` and spawn
  no Corvint process (decision 0178). The OpenCode plugin has no
  documented model-visible channel for its `event` and `tool.execute.after` hooks, so its
  transcript-only equivalent is OpenCode's own log. A successful receipt's degradations and the
  expected `session-state-evicted` and `stop-recursion-protected` guards MUST go to
  `client.app.log` (`service` `corvint-opencode`, `level` `info`). Every other report keeps its
  `[corvint/opencode]` `console.warn`. A plugin loaded without that client, or whose log call
  rejects, keeps the warning, so the code is still recorded somewhere. The `corvint_context` tool's
  over-bound refusal is already tool output, not a notice, and is unchanged. The `SOL-V0-001`
  ledger rows that `harness event` appends are unchanged for both hosts. OpenCode's
  `unsupported-impact-path-suffix` refusal on `file-change` is also expected: the adapter MUST
  preserve that structured code in `client.app.log` rather than emit a terminal warning. Concurrent
  `file.edited` callbacks MUST share one bounded drain with at most one `file-change` subprocess in
  flight; pending events are bounded by the existing 128 session states, one anonymous overflow
  bucket, and 256 paths per batch, and duplicate paths for one session are coalesced. This changes
  neither the Core refusal nor its non-zero exit.
- `AHI-023`: An adapter MUST report the host version its host actually provides, and MUST NOT turn
  a version the host never provides into a per-receipt degradation. Claude Code provides none. Its
  documented hook input fields (`session_id`, `transcript_path`, `cwd`, `permission_mode`,
  `hook_event_name` and per-event fields, code.claude.com/docs/en/hooks, read 2026-09-12) and
  documented hook environment (`CLAUDE_PROJECT_DIR`, `CLAUDE_PLUGIN_ROOT`, `CLAUDE_PLUGIN_DATA`,
  `CLAUDE_CODE_REMOTE`, `CLAUDE_EFFORT`, `CLAUDE_PLUGIN_OPTION_<KEY>`) carry no version. A local
  Claude Code 2.1.267 `SessionStart`/`UserPromptSubmit` probe on 2026-09-12 received only
  `cwd`, `hook_event_name`, `session_id`, `source` and `transcript_path`. The only version-bearing
  value it saw was the undocumented `AI_AGENT` environment variable, which is not a versioned
  adapter API (`AHI-011`) and is not consumed. Both Claude Code call paths therefore send the one
  spelling `unreported-by-hook-api` as the adapter tuple's host version, so the kernel and the
  `corvint-dogfood-event/0` envelope append no `host-version-unknown` to its receipts. The one-time
  disclosure is the `host-version-unreported-by-hook-api` entry in
  `integrations/claude-code/plugins/corvint/compatibility.json` (`AHI-010`) and this clause. Codex,
  the Gemini CLI and OpenCode keep `unknown` and its degradation, a recognised code the `AHI-019` refusal
  accepts, which `AHI-022` routes off the terminal for the two JavaScript hosts.

- `AHI-024`: The experimental Pi extension MUST use runtime-provided version `0.85.1`, adapter
  `0.1.2`, host `pi`, and surface `extension`; other versions MUST refuse visibly. The native
  translator argv is exactly `adapter pi EVENT`, where EVENT is one of `session-start`,
  `user-prompt`, `file-change`, `post-tool`, `stop`, or `session-end`. Its UTF-8 stdin is bounded
  to 131072 bytes and contains exactly `hostVersion` and event-normalized `input`. Unknown,
  duplicate, null, mixed-identity or trailing input MUST refuse before kernel execution. Root
  comes only from actual cwd; the request cannot choose host, surface, adapter or authority.
  The translator MUST reuse the native harness kernel and repository-data envelope. The Pi
  shim MUST NOT rank evidence, compute receipts, implement Frontier, or infer accepted intent.
  All output, including faults, is bounded to 8000 UTF-8 bytes and uses the closed
  `corvint-pi-adapter/0` profile described below. Fallback MUST always report continuation false.

  Session start/reload/new/resume/fork map to startup/resume/clear/resume/resume respectively;
  successful compaction maps to compact recovery. Session-tree navigation resets transient state
  and requests resume context. Startup/compaction context is supplied once through the ephemeral
  context hook on the next model request (including same-turn compaction retries), never persisted
  in Pi session history. Pending recovery is discarded on session/root changes or prompt failure. Before-agent-start forwards only bounded
  prompt text and appends native framed data ephemerally to that turn's system prompt. Tool
  observations MUST never forward messages, raw tool content or details, or infer verification
  from a tool name. Explicit typed path observations remain subject to core containment.
  Agent-end may inspect only explicit terminal stopReason to distinguish stop/error/aborted;
  no message text is forwarded or persisted. Settlement, process exit zero and streamed text
  MUST NOT imply successful completion, verified outcome, authority or Frontier EMPTY.

  The shim MUST refuse native reads when project trust is false, hash session identity locally,
  retain only bounded in-memory receipt deduplication, and clear it on transitions. It MUST
  abort and join owned work on host cancellation or shutdown. Subprocess-group cleanup MUST
  outlive leader exit and prove TERM-ignoring grandchildren cannot escape; unsupported platform
  cleanup MUST refuse. Binary selection uses explicit configuration before `CORVINT_BIN`
  and refuses an empty value before spawning.

  `/corvint-context` maps text to user-prompt input.task. `/corvint-outcome` accepts only an
  explicit session-end outcome JSON under the existing kernel schema. The shim rejects non-object,
  duplicate-key, oversized and caller-identity input before merging the host identity; the native
  kernel retains deeper validation and invalid input returns `invalid-input`. Persistence
  degradation is shown to the user, never reported as recorded success. Faults use UI notices
  or stderr in non-UI modes, without adding automatic messages to model history. Expansion remains the
  documented existing `adapter source-view` route with native selector validation. No automatic
  task/outcome or new source-selector parser is permitted. Package/version declarations and
  compatibility evidence MUST agree. This functional extension is FALLBACK until its exact
  tuple completes the separate protected authority and full host qualification requirements.

  The output has exactly `profile`, `event`, `host`, `surface`, `hostVersion`, `adapterVersion`,
  `support`, `receiptId`, `context`, `degradations`, `fault`, and `shouldContinue`. Constants are
  `corvint-pi-adapter/0`, `pi`, `extension`, `0.1.2`, `FALLBACK`, and false respectively.
  Event is the admitted selector, or null only for an unsupported-event fault. Success has
  hostVersion `0.85.1`, an existing `harness-receipt:sha256:` request identity with 64 lowercase
  hexadecimal digits, string context (empty or native framed data), recognized degradation
  strings, and fault null. A receipt is request identity, not response-integrity evidence.
  Faults have receiptId null, empty context/degradations, and a fixed code: `invalid-input`,
  `unsupported-event`, `unsupported-host-version`, `core-unavailable`, `invalid-core-response`,
  `output-too-large`, `deadline`, or `untrusted-project`. Fault hostVersion is null unless the
  input version was successfully validated; unknown caller text MUST NOT be echoed or replaced
  by a claimed observed version. Transport-local fixed faults additionally include
  `missing-binary`, `invalid-binary-config`, `aborted`, `invalid-adapter-response`, and
  `cleanup-failed`. Unknown keys/types/codes or impossible success/fault combinations refuse.
  Input bounds apply before the generic adapter reader's larger allocation. Native output
  budgeting MUST include wrapper fields, JSON escaping and framing rather than assuming an
  8000-byte core result fits an 8000-byte wrapper. Automatic and explicit-query transport caps
  are 2000ms including cleanup; native work expires earlier with a typed-fault/output reserve.
  This deadline is not the AHI latency target; existing 250/500ms qualification targets remain.
  Cleanup faults remain visible even if the group leader exits zero. Exact source expansion is
  `corvint adapter source-view --root ROOT --packet PATH --packet-sha256 SHA --result N`, with
  optional `--evidence N`, `--commit SHA`, `--lines RANGE`, `--requirement ID`, `--max-bytes N`;
  that separate route retains its existing framed profile and native validation.

## Native platform profiles

| Platform | Embedded host-admission key | Maintained Corvint package | Native surfaces | Stability rule |
|---|---|---|---|---|
| Codex CLI/Desktop | `codex` | Codex plugin | skills, hooks, MCP | test startup/resume/clear/compact separately; compaction recovery remains dirty-path-only |
| Codex IDE | `codex` (shared host; separate surface status) | standalone Codex integration | standalone skill, shared MCP, supported hooks | plugins are unavailable; never inherit CLI/Desktop status |
| Claude Code | `claude-code` | Claude Code plugin | skills, hooks, MCP | publish minimum/maximum tested plugin API versions |
| Gemini CLI | `gemini-cli` | Gemini CLI extension | context file, commands, skills, hooks, MCP | validate extension environment filtering and hook schemas |
| OpenCode | `opencode` | OpenCode plugin | stable session/tool/file events, tools, MCP; beta context/session hooks isolated | remain `FALLBACK` until a pinned version passes safe frontier/continuation conformance |
| Pi | `pi` (experimental; AHI-024) | Pi extension | native session/tool lifecycle plus Corvint protocol | claim only the Pi releases in the tested matrix |
| DeepSeek Harness | not admitted | Cordis plugin | services/events and append-only trajectory observations | treat developer-preview API changes as adapter changes |

## Conformance gate

For every claimed host version, an isolated fixture repository MUST prove:

1. clean install/discovery and complete uninstall;
2. exact single-snapshot repository/revision identity, dirty-state handling, and clean oversized-
   source exclusion;
3. bounded startup/task context with a verifiable receipt, including project-instruction authority
   before product vocabulary for an operational task;
4. on-demand query and exact expansion without manual repository search;
5. capture of supplied evidence, one edit, one verification observation, and one explicit outcome
   receipt without persistence or raw task text;
6. frontier behavior at the strongest safe native stop point;
7. permission denial, missing Corvint, timeout, malformed output, and incompatible-version degradation;
8. zero secret/environment leakage and no unauthorized network or filesystem access; and
9. byte-identical canonical normalized requests across adapters for every common logical event,
   allowing only host events that a surface does not expose; and
10. bounded stop/frontier invocation with recursion/continuation-loop protection.
11. compaction rehydration from exact dirty paths without transcript access, prompt persistence, or
    a claim of complete task-state recovery where the host exposes a compaction lifecycle.

The matrix fails if any adapter silently drops a critical event, invents observed evidence, changes
Corvint semantics, prevents unrelated host work during degradation, or requires a second knowledge
store. A passing MCP smoke test alone cannot promote a platform to fully supported.

## Limits, failure policy, and non-goals

- Lifecycle input is at most 131,072 bytes; task text is at most 2,000 Unicode characters and
  16,384 UTF-8 bytes; path and evidence arrays contain at most 256 entries; the output budget is at
  least 4,096 bytes and bounds the complete response, not merely its nested Corvint packet. Native
  wrappers never truncate task text over either bound into a different query. They either serve it
  through the disclosed anchor query of `AHI-016` or reject it before invoking Corvint and name
  `prompt-over-query-bound`.
- Adapter commands receive a hard host timeout no greater than two seconds for automatic events.
  Timeout, missing executable, permission denial, malformed JSON, and version mismatch degrade
  visibly and fail open for unrelated coding.
- Failure mode (`AHI-017`): a process start slower than the declared kill less the process reserve
  leaves no Corvint budget. The Gemini hook then degrades `host-kill-budget-exhausted` without
  spawning Corvint; if the start alone reaches the kill, the host drops the hook output and coding
  continues. When the Go watchdog fires, the abandoned event work ends with the process exactly as
  it would under the host kill, so any write it had not finished is lost. The reserve is a p95, not
  a maximum: a tail exit slower than the reserve can still meet the kill after the degradation is
  written.
- Failure mode (`AHI-018`): a harness that passes a stated kill longer than the kill it actually
  enforces sees the adapter killed without output; the shipped configuration cannot reach this path.
- The adapter inherits Corvint's local-only Git and repository boundary. It never fetches, starts a
  daemon, adds a database, phones home, reads a transcript, copies a host knowledge graph, or
  weakens host sandbox/approval policy.
- V0 does not claim model-internal observation, causal attribution, test execution authority,
  complete tool interception, an empty Frontier, or compatibility outside the tested matrix.

The simpler baseline is a user invoking Corvint CLI or MCP tools manually. Native packages exist only
to remove repeated lifecycle glue while preserving the same receipts and explicit gaps.

### Failure codes

Beyond the codes named above, the shared `harness event` core (`internal/gokernel/harness.go`), its
repository probe (`internal/gokernel/repository.go`), and the native adapter
(`cmd/corvint/host_adapter.go`) emits the kebab-case codes below (decision 0100). Each row cites
the first emitting site and quotes the message returned there or states the condition checked
there, which is the whole of what the row asserts.

| Code | First emitting site | At the cited site |
|---|---|---|
| `corvint-output-too-large` | `cmd/corvint/host_adapter.go:847@1b317e61` | adapter output cannot be marshaled, or with its final LF exceeds 8000 bytes; a degraded `systemMessage` naming this reason is written instead |
| `canonical-json-failed` | `internal/gokernel/harness.go:458` | "cannot encode receipt basis" |
| `harness-input-too-large` | `internal/gokernel/harness.go:361` | "harness input exceeds its byte limit" |
| `harness-output-too-large` | `internal/gokernel/harness.go:466` | "harness response exceeds its byte budget" |
| `invalid-harness-adapter` | `internal/gokernel/harness.go:70` | "invalid <label>" |
| `invalid-harness-budget` | `internal/gokernel/harness.go:340` | "harness budget must be at least <value> bytes" |
| `invalid-repository-root` | `internal/gokernel/harness.go:376` | "cannot resolve repository root" |
| `malformed-corvint-output` | `cmd/corvint/host_adapter.go:605@2c724e09` | Claude adapter: the `harness event` stdout is not JSON; the degraded `systemMessage` names this reason |
| `project-root-unavailable` | `cmd/corvint/host_adapter.go:302@2100b4c9` | Claude adapter: the project root (`CLAUDE_PROJECT_DIR`, else the working directory) cannot be made absolute; the degraded `systemMessage` names this reason |
| `repository-identity-malformed` | `internal/gokernel/repository.go:172` | "Git object identity is malformed" |
| `repository-probe-cancelled` | `internal/gokernel/repository.go:161` | "Git repository probe was cancelled" |
| `repository-probe-timeout` | `internal/gokernel/repository.go:159` | "Git repository probe exceeded its 10-second deadline" |
| `repository-probe-too-large` | `internal/gokernel/repository.go:142` | "Git output exceeds its byte limit" |
| `repository-profile-malformed` | `internal/gokernel/repository.go:262` | "Git profile path is not valid UTF-8" |
| `repository-status-malformed` | `internal/gokernel/repository.go:209` | "Git status output is malformed" |
| `repository-status-too-large` | `internal/gokernel/repository.go:232` | "Git status exceeds the <value>-path limit" |
| `unsupported-git-object-format` | `internal/gokernel/repository.go:186` | "unsupported Git object format: <value>" |

## Over-bound prompts

Intent (decision `0097-over-bound-prompt-anchor-query-2026-09-12.md`): a long pasted prompt should
still receive repository evidence for the files, symbols and requirements it names explicitly,
without Corvint inventing a query from text it did not use and without storing the prompt.

Closed anchor rule. Every inline backtick span of 1 to 128 printable ASCII characters (without the
backticks) is an anchor. With those spans blanked, every maximal token of `[A-Za-z0-9_./-]` that
starts and ends with `[A-Za-z0-9_]` is an anchor when it is (a) path-like: one or more `/`-separated
segments, or a stem of at least two characters followed by one `.` and a letter-led extension of at
most eight characters; (b) an identifier: `[A-Za-z_][A-Za-z0-9_]*` containing an inner underscore or
a lower-to-upper case change; or (c) a requirement ID: upper-case alphanumeric groups joined by `-`
ending in a digit group. The query is the distinct anchors joined by one space. Every anchor is
ASCII, so the character bound governs. Serving the query uses the unchanged normalized `task`
field; the lifecycle wire, the kernel bound and the receipt shape do not change.

Design call. Three shapes were weighed. Truncation stays forbidden: it is an invented query
(invariant 2). A stored prompt behind a local content-addressed pointer was rejected. `user-prompt`
is a read path, and a write under `.corvint/` outside the self-observation ledger breaks AGENTS.md
invariant 4. It would also persist prompt text against `AHI-005` and this spec's
"neither returned nor persisted" rule. A secret screen can only refuse such a write, never make it
safe. The pointer would also resolve to bytes the host already holds in its own context. The derived
anchor query needs no state. It makes no claim about the elided text because it names exactly which
kind of text it used and says the rest was not queried.

Non-goals: summarizing, compressing, ranking or sampling prompt text; resolving anchors against the
index before querying; changing the kernel task bound or the MCP `task` schema; routing the
JavaScript adapters' derivation through a new `corvint` flag or subprocess, which would put the
raw over-bound prompt on the lifecycle wire; and a retrieval verb for prompt text.

Failure modes: an anchor-free or over-bound anchor set refuses as before; lexical noise such as
`and/or` may enter the query, and the disclosure's anchor count makes that visible; a pasted secret
shaped like an identifier reaches the in-process query exactly as it would inside a within-bound
prompt, and is neither returned nor persisted; the disclosure bytes are reserved from the Corvint
budget, so the complete host frame stays within its bound.

Acceptance evidence: `TestAHI016OverBoundPromptDerivesVerbatimAnchorQuery` pins the exact derived
query, the disclosure counts, no prompt echo, and unchanged within-bound prompts.
`TestAHI016OverBoundPromptKeepsRefusalWhenAnchorsCannotServe` pins the refusal for an empty and an
over-bound anchor set. `TestAHI016ClaudeOverBoundPromptInjectsDisclosedContextWithoutStoring` runs
the Claude adapter against a real repository and pins a non-degraded response, the disclosure ahead
of the guidance and envelope, no prompt echo, and byte-identical repository and `.corvint` state.
The JavaScript adapters port the rule as `integrations/gemini-cli/hooks/prompt-bound.mjs` and its
byte-identical twin `integrations/opencode/src/prompt-bound.js`; each adapter trims first, sends
only the derived `task`, and places the disclosure ahead of its untrusted-data envelope. Trimming
removes exactly Go `strings.TrimSpace` whitespace (Unicode `White_Space`) through the twins'
`trimSpace`, not `String.prototype.trim`: U+0085 is trimmed and U+FEFF is kept, so a prompt is
measured and disclosed identically on every host. The OpenCode `corvint_record_outcome` session-end
`task` follows the same order before it is reduced to `taskSha256`: `boundedTask` trims with
`trimSpace`, then checks both task bounds on the trimmed text.
The boundary cases in `conformance/harness-event-v0/common-logical-interaction.json` cover an
anchor-free prompt, an anchor query, a non-ASCII prompt with an unclosed backtick span, an
over-bound anchor set, a trailing U+FEFF that keeps a 2,000-character prompt over the bound, and
U+0085 edges that are trimmed out of the disclosed counts. `TestAHI016OverBoundPromptMatchesCrossHostBoundaryCases` pins the Claude and
Codex task, refusal code and disclosure to them. The `AHI-016` test in
`integrations/host-adapters.test.mjs`, run by `TestHostAdapterJavaScriptHosts`, pins the same bytes
for the Gemini CLI hook and the OpenCode tool, requires that a refused case never invokes Corvint, and
checks that the two twins are byte-identical.

Rollback: restore the unconditional `prompt-over-query-bound` refusal in `normalizeAdapterInput`
and delete `cmd/corvint/prompt_bound.go` with its tests. For the JavaScript adapters, restore the
pre-trim bound refusal in `corvint-hook.mjs` and `corvint_context` and delete both `prompt-bound` twins,
the anchor-bearing boundary cases, and their tests. No stored state, wire field or index
format needs migration.

## Rollout, rollback, and promotion

1. Ship the shared lifecycle adapter and native packages as `FALLBACK` developer previews.
2. Pin and publish host-version/OS conformance results independently per surface.
3. Add an accepted harness execution authority and closing Frontier profile; rerun all stop and
   continuation-loop fixtures.
4. Promote one exact tuple to `FULL` only after every conformance case passes. Other tuples retain
   their prior status.

Rollback disables or removes the native package. Corvint Core artifacts and repository state remain
unchanged; a failed adapter must never require graph migration or cleanup. `AHI-021` and the
`AHI-019` receipt routing roll back by restoring `degradedAdapterOutput` for every reason and the
`systemMessage` receipt and `session-start` notice in `cmd/corvint/host_adapter.go`; no stored
state or wire field changes. `AHI-022` rolls back by restoring the Gemini hook's `systemMessage`
receipt and degradation, OpenCode's `console.warn` for every report, and immediate concurrent
`file-change` dispatch, with a package version bump each (`AHI-020`). `AHI-023` rolls back by restoring `unknown` as the Claude Code host version
in `dogfoodHostVersions`; the receipt then carries `host-version-unknown` again. The decision 0178
not-a-repository rule rolls back as that decision's Rollback section describes.

## Traceability

| Requirement | Implementation surface | Required evidence |
|---|---|---|
| `AHI-001`, `003`, `005`, `014` | shared `corvint harness event` core and `internal/projectpath` | canonical receipt, bounds, privacy, revision, and event fixtures; `TestHostAdapterAbsentPathContainment` and `TestRelativeAliasesAndUncertainty` cover `AHI-014` path containment, and the `integrations/host-adapters.test.mjs` test `AHI-014 Gemini classifies changed paths on resolved symlinks like internal/projectpath` under `TestHostAdapterJavaScriptHosts` covers the Gemini hook's symlink resolution; `TestClaudeAdapterForkSessionStartIsResume` covers the Claude `fork` start source; `TestAHI003ClaudeCompactSessionStartRehydratesDirtyPaths` drives the Claude `SessionStart(source=compact)` hook entrypoint over a mixed dirty worktree and requires the tracked impact, the untracked count and `compaction-untracked-paths-not-rehydratable` from the receipt's own snapshot; `TestQualifiedLifecycleCompactSessionStartRehydratesDirtyPaths` requires the same for the qualified profile under FULL and FALLBACK and refuses a reordered, extra or dropped code; `TestAHI014EventExpectationsAreHostConsistent` (`conformance/harness-event-v0/host_schema_test.go`) pins each `common-logical-interaction.json` event's closed host set and requires every present host's golden `expected` object to be byte-identical, so a per-host field or host-membership mutation of that fixture fails here |
| `AHI-004` | native adapter renderers, shared lifecycle command, `internal/repoenvelope`, and the JavaScript envelope builders | byte-identical untrusted-data envelope with hidden-character escaping and terminator refusal (`internal/repoenvelope`, `cmd/corvint`, `tools/native-hook-observer` and `integrations/host-adapters.test.mjs` tests), injection bounds, authority order, and query fixtures |
| `AHI-011`, `015` | embedded `internal/gokernel/host-schema.json` admission table and shared lifecycle command | schema/admission tests plus one host-keyed golden fixture per admitted host |
| `AHI-002`, `006..010` | four native packages and release matrix | install/uninstall, lifecycle, degradation, and version fixtures; for `AHI-010`, the `integrations/host-adapters.test.mjs` test under `TestHostAdapterJavaScriptHosts` binding each `integrations/compatibility.json` row to its shipped declaration and its row's adapter version to the package manifest version, and asserting `globalDegradations` disjoint from `receiptDegradationPolicy.recognised` |
| `AHI-016` | `cmd/corvint/prompt_bound.go`, Claude and Codex native wrappers; `integrations/gemini-cli/hooks/prompt-bound.mjs` and `integrations/opencode/src/prompt-bound.js` in the Gemini CLI hook and OpenCode `corvint_context` tool | `TestAHI016OverBoundPromptDerivesVerbatimAnchorQuery`, `TestAHI016OverBoundPromptKeepsRefusalWhenAnchorsCannotServe`, `TestAHI016ClaudeOverBoundPromptInjectsDisclosedContextWithoutStoring`, `TestAHI016OverBoundPromptMatchesCrossHostBoundaryCases`, and the `AHI-016` cross-host test in `integrations/host-adapters.test.mjs` under `TestHostAdapterJavaScriptHosts` |
| `AHI-017` | `cmd/corvint/host_adapter.go` declared-kill table and watchdog; `integrations/gemini-cli/hooks/corvint-hook.mjs` derived budget | `TestAHI017AdapterHostKillMatchesDeclaredHooks` (which also fails on a matcherless `FileChanged` group), `TestAHI017HostAdapterWatchdogDegradesBeforeHostKill`, and the two `AHI-017` Gemini tests in `integrations/host-adapters.test.mjs` under `TestHostAdapterJavaScriptHosts` |
| `AHI-018` | `integrations/gemini-cli/hooks/corvint-hook.mjs` argument parser and the `host-adapters.test.mjs` fixture harness | the `AHI-017` Gemini declared-kill test, which pins the shipped command to carry no override, under `TestHostAdapterJavaScriptHosts` |
| `AHI-019` | `cmd/corvint/host_adapter.go` (`postToolChangeOutOfRoot`, `invokeLegacyClaudeEvent`, `claudeReceiptOutput`, `claudeDegradationsRecognised`, `renderAdapterResult`) | `TestClaudeAdapterRoutineReceiptAndExpectedDegradationCarryNoNotice` asserts no output for an out-of-root Edit target and a `PostToolUse` additionalContext receipt with no `systemMessage` for an in-root one; `TestAHI019ClaudePostToolReceiptNamesEveryDegradation` drives a realistic Edit `PostToolUse` payload through the hook entrypoint and requires the receipt's degradation codes in that additionalContext; `TestAHI019ClaudeReceiptRefusesUnrecognisedDegradation` calls `claudeReceiptOutput` with an unrecognised code and requires the named `corvint-degradations-unrecognised` fault `systemMessage` without the code; `TestAHI019CodexWholeReceiptRefusesUnrecognisedDegradation` and `TestAHI019ClaudeDogfoodEnvelopeRefusesUnrecognisedDegradation` call `renderAdapterResult` with an unrecognised code and require exactly the Codex and Claude Code fault output, which carries no code, and the Codex test also requires that fault for a string, object, or `null` `degradations` value |
| `AHI-020` | the four host package manifests listed in the requirement body | `script/check-host-package-versions.sh` (`make host-package-versions-check`) compares, per package, the last commit that changed a version field against the last commit that changed any other shipped file, by Git ancestry between the two commits rather than by committer-second timestamp (two distinct commits, such as either side of a rebase, can share a committer second); `script/check-host-package-versions_test.sh` proves it fails on a content-only change, passes after a version bump, and fails on a newly added shipped file with no bump, and its fifth case proves a bump and a later content change sharing one committer second still fails |
| `AHI-021` | `cmd/corvint/host_adapter.go` (`claudeExpectedDegradation`, `claudeDegradedOutput`, `renderClaudeContext`) | `TestClaudeAdapterRoutineReceiptAndExpectedDegradationCarryNoNotice` asserts `prompt-over-query-bound` is `UserPromptSubmit` additionalContext with no `systemMessage`; `TestClaudeAdapterDogfoodEventDeadlineCarriesNoNotice` asserts an expired dogfood event deadline is `UserPromptSubmit` additionalContext with no `systemMessage`; `TestClaudeAdapterFaultKeepsNotice` asserts a `missing-session-identity` fault inside a repository keeps its `systemMessage`; `TestClaudeAdapterNotARepositoryIsSilent` asserts Claude `session-start`, Edit and Write `post-tool`, and Codex `SessionStart` outside a Git repository emit nothing and create no `.corvint` (decision 0178) |
| `AHI-022` | `integrations/gemini-cli/hooks/corvint-hook.mjs` (`EXPECTED_DEGRADATIONS`, `degradation`, `successOutput`); `integrations/opencode/src/index.js` (`report`, `record`, `queueFileChange`, `drainFileChanges`) | under `TestHostAdapterJavaScriptHosts`, the `integrations/host-adapters.test.mjs` tests `AHI-022 Gemini routine receipt and expected degradation carry no notice`, `AHI-022 Gemini fault keeps notice`, `AHI-022 decision 0178 Gemini outside a Git repository emits and invokes nothing`, `AHI-022 OpenCode routine receipt goes to the host log and a fault keeps its warning` (including the structured suffix refusal), and `AHI-022 OpenCode serializes a burst of file-change subprocesses` |
| `AHI-023` | `cmd/corvint/host_adapter.go` (`claudeHostVersion`, `dogfoodHostVersions`); `cmd/corvint/local_completion_event.go` (`dogfoodDegradations`) | `TestClaudeAdapterReceiptOmitsHostVersionUnknown` asserts a Claude Code receipt carries `unreported-by-hook-api` and only `frontier-authority-unavailable`, while codex keeps `host-version-unknown`; `TestDogfoodEventReadOnlyEnrolledStopAndPrompt` renders a codex `host-version-unknown` envelope through `renderAdapterResult` and requires framed additionalContext, so the recognised code is not refused; the `integrations/host-adapters.test.mjs` AHI-023 test under `TestHostAdapterJavaScriptHosts` asserts the Claude Code plugin's `compatibility.json` discloses `host-version-unreported-by-hook-api` and the Codex plugin's discloses `host-version-unknown` |

Prior experimental implementation: `src/context_corvint_harness.py`, the `corvint harness event`
command in `src/corvint_cli.py`, `integrations/{codex,claude-code,gemini-cli,opencode}/`, and the
`tests/test_harness_*.py` conformance fixtures; decision 0012 R0
(`docs/decisions/0012-expert-panel-ratifications-2026-09-01.md`) rules this Python surface
non-authoritative and slated for separate removal. The native surface is `corvint harness event`
(`internal/gokernel`, `cmd/corvint`; `docs/specs/go-production-kernel-migration-v0.md`,
`GPK-V0-038`). The common-event golden is
`conformance/harness-event-v0/common-logical-interaction.json`. The published matrix remains
`FALLBACK`. A real Codex 0.149.0 package discovery/install/uninstall cycle passed locally, and a
wheel-installed startup hook produced five receipts without deadline fallback after non-query
lifecycle events stopped rebuilding the full source index. Complete install-to-session conformance,
the full release matrix, and promotion evidence remain `NOT_RUN`; the five observed startup calls
are not a p95 measurement and do not close `AHI-012`.

### Claude/Codex lifecycle repair (2026-09-06)

The Codex adapter routes the existing native Go prompt and compact profiles, trimming immediate
prompt text before receipt binding. Claude preserves absent/startup/resume/clear/compact source
semantics (and, from 2026-09-13, maps its `fork` source to `resume`); a supplied unsupported or non-string source produces a named fallback before a lifecycle-event
call. Both validate the closed repository envelope and exact receipt. These repairs implement
`AHI-001`, `AHI-003`, `AHI-005`, `AHI-009`, `AHI-012`, and `AHI-014` without new wire fields.

Bounded read children have a retained process-group supervisor, bounded nonblocking pipes and
one automatic-hook deadline including cleanup. Command exit status is carried separately from
the supervisor's cleanup status. Interruptions, blocked writes, oversized output, exited commands
with surviving descendants and concurrent Claude context reads cannot leave an owned bounded
read running after return. A supervisor liveness pipe and hard deadline also terminate its owned
group when the wrapper is killed or crashes before cleanup. Identity mismatch refuses to signal
an unrelated process group.
Claude's hooks start no persistent refresh under amended `IDX-SNAP-V0-012`; explicit supervised
`index --if-stale` preparation stays outside the hook. A bounded synchronous in-memory read on a
snapshot miss remains supported and writes no snapshot. The 2026-09-08 owner-directed amendment
supersedes decision 0049 item 3, preserving its original acceptance and writer semantics.

Claude session-start/user-prompt/stop/session-end now use the existing separate LCP native profile;
post-tool/file-change retain this legacy harness contract. Native root/key guidance is trusted
adapter text outside the unchanged untrusted-data envelope and the full host frame is bounded.
`TestClaudeNativeDogfoodLifecycle` and actual Python adapter stdio tests cover these boundaries;
`TestClaudeSourceHandoffCLI` verifies the separate opt-in single-capture source handoff. Formal
host conformance stays FALLBACK/NOT_RUN until actual native-host qualification is recorded.

`tests/test_harness_native_go.py` exercises adapter-to-real-Go prompt whitespace/privacy and
session-source/dirty-compaction receipt parity. It does not substitute for native-host invocation.
The host-specific adapter suites cover the cancellation and source-refusal boundary; the shared
protocol fixture now includes Codex prompts. Native-host probes obey `AHI-008`: isolated deny or
read-only permission settings, external deadlines, and no implicit approval of a host hold.
Observed qualification and omissions are recorded in
`docs/BUILD-LOG.md` (2026-09-06 lifecycle repair). Compatibility remains `FALLBACK`.

## Promotion and kill criteria

- One leaked raw session ID, transcript, environment value, prompt echo, tool body, or secret is an
  immediate release blocker.
- One silent downgrade, false `FULL`, unauthorized block/continuation, or unbounded recursion is an
  immediate release blocker.
- A native adapter that cannot stay within the automatic-event latency budget remains manual/MCP
  fallback; Corvint Core is not complicated to hide host latency.
- If host-specific translators require a second knowledge model or more than thin event/rendering
  code, remove that native path and retain the portable fallback.

## Official extension references

- Codex: `https://developers.openai.com/codex/plugins`,
  `https://developers.openai.com/codex/hooks`, and
  `https://developers.openai.com/codex/extend/mcp`; the documented compaction lifecycle includes
  `PreCompact`, `PostCompact`, and `SessionStart` with source `compact`
- Claude Code: `https://code.claude.com/docs/en/plugins`,
  `https://code.claude.com/docs/en/hooks`, and `https://code.claude.com/docs/en/mcp`
- Gemini CLI: `https://geminicli.com/docs/extensions/reference/` and
  `https://geminicli.com/docs/hooks/reference/`
- OpenCode: `https://opencode.ai/v2/docs/build/plugins` and
  `https://opencode.ai/v2/docs/mcp-servers`
- Pi: `https://github.com/earendil-works/pi/blob/d981de1229ef899957bbe968bc8dcda02a21f477/packages/coding-agent/docs/extensions.md`
- DeepSeek Harness: `https://deepseek.com/harness/`

These URLs document current integration surfaces, not Corvint compatibility. The release matrix is the
only compatibility claim.

## Protected native qualification bootstrap

PLE-V0-012 resolves qualification bootstrap without manufacturing a completed host certificate: independent execution admission and signed observations come first, then a separate short-lived root-owned campaign over the exact release, repository and boot/app/engine lifetimes. Candidate and normal paths share actual protected computation, currentness, cwd and recursion behavior; only the completed independently accepted tuple may report FULL. Candidate output always names the UNQUALIFIED native exercise. Actual task repository cwd must be observed in the native matrix; a payload cwd or app-wide cwd cannot substitute.

Use a dedicated campaign repository with all same-repository native Stops explicitly in the reviewed scope. Quiesce/exclude other tasks or obtain independent acceptance of that bounded scope; no task-ID or Desktop/CLI origin is inferred from a shared daemon. Exercise cases 6 and 10 using real OPEN/block, EMPTY/release and recursive release; all eleven cases, per-surface latency (AHI-012), equal-critical-recall and resource/cleanup evidence remain necessary for FULL. Remove/expire campaign admission and verify restoration of hook configuration before independently admitting completed qualification. Candidate receipts and timing are evidence for review, never self-promotion. Current native campaign evidence remains NOT_PRODUCED.

Prospective AHI-024 acceptance: focused native input/envelope negatives; actual pinned Pi ephemeral-context and lifecycle fixture; descendant interruption regression; closed package/profile inventories. These witnesses are required and not yet produced by this contract commit. Rollback removes Pi admission/package/writer while retaining prior exact bundle readers.

### Owner core-release boundary (2026-09-16)

Required runtime FULL/authority qualifications remain exact and separate for Codex CLI, Codex
Desktop, Claude Code, Gemini CLI and Pi. Stock OpenCode remains FALLBACK. VS Code stock support,
upstream merge/delivery and editor qualification are deferred/nonblocking for the core release;
no human study is required. Existing failures, NOT_RUN and authority gaps retain their meaning.
CRB-V0-019's five packaged host trees remain labelled FALLBACK: bundle inclusion and native local
SATISFIED are not completed runtime qualification or authority admission.

AHI-024 implementation witnesses: `TestPiClosedInput`, `TestPiUnknownVersion`, and
`TestPiNativeStopReceipt` cover native refusal/receipt behavior; `TestPiClosedProfileInventories`
covers profile-specific Pi artifact admission and tampering. `integrations/pi/runtime.test.mjs`
checks closed output, explicit-option precedence and a TERM-ignoring grandchild after leader exit. Actual
Pi0.85.1 local-provider fixtures verify two-turn ephemeral native context and distinguish startup
SIGINT cancellation from successful completion. These are functional FALLBACK witnesses only;
protected authority, permissions, all native surfaces and latency/recall qualification remain open.


### Pi functional repair evidence (2026-09-22)

`TestPiInvalidOutcomeInput` and the Pi JavaScript runtime/extension regressions are part of the
canonical host-adapter gate. `integrations/pi/host.test.mjs` separately exercises actual Pi 0.85.1
on macOS arm64 with an in-process offline provider: native local-package install, disable, update,
re-enable and remove; two-turn context delivery without session persistence; explicit outcome
refusal/degradation; startup SIGINT/SIGTERM cleanup and descendant cancellation. Handler tests
cover startup/reload/new/resume/fork/tree, same-turn compact recovery, root/session drift, trust,
private tool/message content and shutdown cleanup. These tests add functional evidence only;
interactive TUI/RPC, Linux/Windows, latency/recall and protected FULL qualification are not established.
Rollback reverts this repair and its package/native adapter version together; the prior known
recovery and outcome defects return. No outcome writer or authority claim is introduced.
