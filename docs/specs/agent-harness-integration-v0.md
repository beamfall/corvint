# Agent Harness Integration V0

Owner: Russell Lewis
Date: 2026-08-23
Intent status: accepted direction; AHI-045..047 accepted (decision 0441; V1-0939, V1-0942)
Delivery status: experimental
Authoritative inputs: `docs/PRODUCT.md`, `docs/TECHNICAL-BRAIN.md`,
`docs/specs/cem-0.2-canonical-binding.md`

## Agent digest
- Claim: Corvint exposes bounded native lifecycle adapters and qualifies stock OpenCode integration separately from execution authority.
- Status: accepted direction; AHI-045..047 accepted (decision 0441; V1-0939, V1-0942)/experimental
- Exists: `internal/gokernel`, `cmd/corvint`, native adapter previews, and the experimental OpenCode inspector/change/Tasks workbench (AHI-033–041) with the owner-approved Work / Change / Evidence presentation (AHI-042).
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
`startup|resume|clear|compact`. The Claude Code and Codex adapters map the host SessionStart source
`fork` (a resumed transcript under a new session id, in the Claude Code 2.1.267 and Codex 0.153 hook
input schemas) to `resume` rather than refusing it as `invalid-start-source`, and the Codex plugin's
SessionStart matcher admits `fork`. On `compact`, Corvint rehydrates a bounded impact packet from the exact
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
  native-hook-observer comparator that re-derives it) and the JavaScript builders (gemini-cli hook, OpenCode beta
  `session.hook("context")` and the OpenCode `corvint_context`/`corvint_record_outcome` tool outputs).
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
  continues (`cmd/corvint/host_adapter.go:39,146,150@b3341e84`). These are faults under `AHI-021`.
  The OpenCode and Gemini adapters MUST SIGKILL their owned process group before completing a
  timeout, a cancellation, or a normal leader exit, since the leader's close does not end a
  same-group descendant that ignored SIGTERM and closed its stdio. Only a delivered signal, `ESRCH`,
  or `EPERM` after the leader's observed exit confirms that kill; any other answer completes as the
  visible degradation `corvint-process-cleanup-unconfirmed` instead of the timeout or the receipt.
  The OpenCode package targets only the OpenCode 2 plugin API (a default export `{ id, setup }`).
  An OpenCode 1.x host refuses it at load ("must default export an object with server()"), and an
  OpenCode 2 `plugins` entry that names a file rather than the package's `src` directory is skipped
  with "configured plugin path must be a directory". Both refusals appear only in the OpenCode
  server log. The legacy receipt MUST NOT claim more than `FALLBACK` for either host line. Native integration qualification is separately scoped by AHI-032, and its install
  documentation MUST name the `src` directory entry.
- `AHI-010`: Each release MUST publish tested host-version ranges, adapter and protocol versions,
  unavailable capabilities, known degradations, and the last conformance result. The adapter version
  in a published matrix row and in its shipped declaration identifies the host package the record
  describes, so it MUST equal that package's `AHI-020` manifest version, compared as an exact string;
  Codex's `+codex.<timestamp>` build metadata is part of the string (decision 0244). Host-version
  evidence from a native host validator ran against one package build, which a later bump does not
  re-validate: a matrix row whose evidence exercised a package names that build in
  `hostVersionEvidenceAdapterVersion` (the Claude Code declaration in `lastValidation.adapterVersion`,
  the Codex declaration in `host.staticallyValidatedAdapterVersion`), or `unknown` when no committed
  evidence ties the run to a build. The same rule holds for installed-lifecycle evidence, which
  post-dates the build: the matrix row then names the build that lifecycle run exercised, while the
  shipped declaration keeps its build-time static-validation fields, since recording the test
  inside the package would need a bump that changes the tested tuple. The last conformance result
  is the row's `lifecycleConformance`: a `host-lifecycle-qualification-v1` result with its retained
  report, or `NOT_RUN`. A row also names its `tier` (`core` or `companion`, decision 0373) and
  marks `fullSupport` as `external-dependent` when FULL needs authority or identity the host API
  does not supply. The OpenCode row's `hostVersion` is the `ctx.app.version` that the OpenCode 2
  plugin setup context supplies. Until a run is retained in committed evidence, the row keeps
  `hostVersionEvidenceAdapterVersion` `unknown`.
- `AHI-011`: Native APIs MUST remain behind versioned adapters. Recognised host identifiers MUST
  come from the single versioned host-admission table embedded in the Corvint
  binary, never from worktree-readable runtime discovery. The table MUST preserve the
  `subset-of-recognised`/`refuse` degradation rule. `harness event` refuses a host outside that
  table with `unsupported-harness-host` and an event outside the `EVENT` set with
  `unsupported-harness-event` (`internal/gokernel/harness.go:379,382`). A host API change MUST NOT
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
  the work context expires 100 ms earlier so a promptly cancelled event keeps its own reason. A panic
  inside an adapter invocation is recovered and degrades as `adapter-internal-error`, so a defect
  never becomes a failed or blocking hook. An
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
  NOT carry a `systemMessage`. The abstention itself is recorded as the `SOL-V0-010` code
  `post-tool-path-not-project-relative` (V1-0746). An in-root target's receipt ID MUST reach the model as
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
  `invalid-stop-hook-active`), and `adapter-internal-error`. An expected reason on `session-start`, `user-prompt`, or `post-tool`
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
  no Corvint process (decision 0178). When the same test finds the OpenCode 2 setup context's
  `location.directory` outside a Git repository, the OpenCode plugin's `setup` MUST register no
  hook, no tool and no event subscription, return no cleanup, and spawn no Corvint process
  (decision 0378). OpenCode 2 gives a plugin no model-visible channel for its `event.subscribe`
  stream or its `tool.hook("execute.after")` callback, and no log API, so its transcript-only
  equivalent is the OpenCode server log. A successful receipt's degradations and the expected
  `session-state-evicted` and `stop-recursion-protected` guards MUST be written with
  `console.info`; every other report keeps its `[corvint/opencode]` `console.warn`. OpenCode 2.0.18
  writes both levels unlabelled to the server log and neither to the TUI (observed 2026-09-27), so
  the split is carried by the call level, not by a separate sink. The `corvint_context` tool's
  over-bound refusal is already tool output, not a notice, and is unchanged. The `SOL-V0-001`
  ledger rows that `harness event` appends are unchanged for both hosts. OpenCode's
  `unsupported-impact-path-suffix` and `unsupported-impact-repository` refusals on `file-change` are
  also expected (decision 0379): the adapter MUST keep that structured code at `console.info`
  rather than emit a warning. Two path abstentions are expected and named at `console.info`
  (V1-0746): `post-tool-path-not-project-relative` when a completed tool call reports a non-empty
  path that `normalizeRepositoryPath` would examine and rejects, and `changed-paths-truncated` when
  a path is dropped at a cap: on `post-tool` for each call whose target or
  `metadata.corvint.changedPaths` list, or their union, exceeds 256 paths, at most once per session
  when the session set is full, and once per `file-change` batch that exceeds Core's 100-path impact
  bound (`maxImpactPaths`). The plugin stops a batch at that bound, so Core's over-bound refusal,
  which carries no code, never reaches it (V1-0773). Each named abstention also reaches Core
  as the closed `adapterCodes` input of that `post-tool` call or `file-change` batch, which
  `SOL-V0-010` records as a content-free ledger row (V1-0767); the plugin's receipt check leaves
  `adapterCodes` out of the basis as Core does. The changed paths of a completed built-in tool call come from its
  `execute.after` payload: `write` from `result.output.target`, `patch` from
  `result.output.applied[].target`, and `edit` from `input.path` resolved against
  `location.directory`; the code-mode `execute` call is skipped, because each inner tool call fires
  its own hook. `stop` is driven by `session.execution.succeeded` and `session.execution.failed`,
  with `session.idle` accepted for forward compatibility; `session.created` drives `session-start`
  and `session.deleted` drives `session-end`. Concurrent `file-change` requests MUST share one
  bounded drain with at most one `file-change` subprocess in flight; pending events are bounded by
  the existing 128 session states, one anonymous overflow bucket, and 100 paths per batch, and
  duplicate paths for one session are coalesced. This changes
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
  `integrations/claude-code/plugins/corvint/compatibility.json` (`AHI-010`) and this clause. OpenCode 2
  supplies its version as the plugin setup context's `app.version` (2.0.18 observed on 2026-09-27),
  and the OpenCode adapter sends it. Codex and the Gemini CLI, and OpenCode when `app.version` is
  absent, keep `unknown` and its degradation, a recognised code the `AHI-019` refusal accepts, which
  `AHI-022` routes off the terminal for the two JavaScript hosts.

- `AHI-024`: The experimental Pi extension MUST use runtime-provided version `0.99.1`, adapter
  `0.3.2`, host `pi`, and surface `extension`; other versions MUST refuse visibly. The native
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
  observations MUST never forward messages, raw tool content or unselected details, or infer verification
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
  documented existing `adapter source-view` route with native selector validation. A separate
  `corvint-dogfood-event/0` bridge MAY use the exact ordinary tuple only under LCP-V0-008/009:
  its sealed read-only policy receipt can request one idle final-settlement remediation after
  explicit enrollment. This does not change the legacy adapter's false continuation, establish
  successful completion or admit the protected Pi profile. Host session identity is separately
  namespaced; stale tree/session/compaction generations cannot adopt enrollment or pending
  recovery. Slash commands are operator assertions and have no authenticated human provenance
  through the shared RPC surface. No automatic
  task/outcome or new source-selector parser is permitted. AHI-025 adds explicit in-memory
  source expansion and durable recording through existing core validators. Package/version declarations and
  compatibility evidence MUST agree. This functional extension is FALLBACK until its exact
  tuple completes the separate protected authority and full host qualification requirements.
  The wildcard Pi peer dependency only selects host-owned modules; it MUST NOT broaden exact
  runtime admission. Earlier 0.85.1 evidence is historical, not qualification of adapter 0.3.2.
  Protected Pi runtime/image contracts retain their separately pinned tuple.

  The output has exactly `profile`, `event`, `host`, `surface`, `hostVersion`, `adapterVersion`,
  `support`, `receiptId`, `context`, `degradations`, `fault`, and `shouldContinue`. Constants are
  `corvint-pi-adapter/0`, `pi`, `extension`, `0.3.2`, `FALLBACK`, and false respectively.
  Event is the admitted selector, or null only for an unsupported-event fault. Success has
  hostVersion `0.99.1`, an existing `harness-receipt:sha256:` request identity with 64 lowercase
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

- `AHI-025`: The owner-requested complete Pi integration MUST expose native model-callable
  `corvint_context`, `corvint_expand`, and explicit `corvint_record_outcome` tools. The additive
  `adapter pi-tool context|expand|record` interface MUST preserve AHI-024 lifecycle semantics.
  Its closed bounded stdin has only `hostVersion` and `input`; root comes from actual cwd.
  Context calls the existing native context compiler and returns its original bounded packet,
  SHA-256 and current commit. At most four packet handles remain in extension memory, bound to
  session and cwd and cleared on transitions/shutdown. Expansion MUST use the existing native
  source-view selector validation over those exact packet bytes, digest and commit, without
  writing packet files or independently interpreting evidence in JavaScript. An expired handle,
  malformed selector, stale commit/tree/blob or forged digest MUST refuse.

  Tool results are explicit Pi conversation content; automatic lifecycle context remains
  ephemeral. Only `details.corvint`'s typed `observedEvidenceHandles`, `changedPaths` and
  `verification` are eligible for post-tool observations. Other details and raw bodies MUST
  never be forwarded, and malformed supplied observations MUST refuse visibly. Tool-specific
  cancellation MUST reach the same owned process-group runner as lifecycle cancellation.

  Durable recording MUST occur only through an explicit tool or `/corvint-record JSON`, using
  the existing `record` producer, clean-revision admission, path containment, secret screening,
  bounded store and caller-reported provenance. It accepts task, optional openedPaths, nonempty
  changedPaths and verification command strings, and outcome passed/failed/blocked. It MUST
  NOT infer verification success, perform automatic recording, or reinterpret the nonpersistent
  `/corvint-outcome` receipt. The result MUST NOT echo task or verification text. A timeout or
  core write failure cannot establish an unchanged store and MUST disclose uncertain mutation.

  The explicit tool output cap is 65536 UTF-8 bytes; lifecycle stays at 8000. Both retain the
  2000ms transport bound. Closed `corvint-pi-tool/0` output members are profile, operation,
  hostVersion, adapterVersion, support, ok, mutation, context, packet and fault. Support is
  FALLBACK; mutation is not-attempted, recorded or unknown. Success has the exact admitted
  version, fault null and native framed context; only context has a packet object containing
  exactly json, sha256, commit and evidenceHandle. The native-produced evidenceHandle is
  `context-packet:sha256:` followed by the packet digest, supplied in tool content and forwarded
  unchanged as an observed evidence handle. Fault has ok false, empty context and packet null; its fixed
  code is unsupported-operation, invalid-input, unsupported-host-version, core-unavailable,
  context-unavailable, stale-context, source-unavailable, record-unavailable,
  invalid-core-response or output-too-large. Host version is null only before validation.
  Runtime-local AHI-024 faults retain their meaning. No tool can request authority or FULL.
- `AHI-026`: The Claude Code plugin MUST register the compaction hook events the installed host's
  hook API documents, `PreCompact` and `PostCompact` (read from the installed Claude Code 2.1.267
  hook runner, decision 0340), as matcherless command groups running
  `corvint adapter claude-code pre-compact|post-compact` under the AHI-017 declared-kill table, and
  `compatibility.json` MUST name the host version the registration was verified against as its
  maximum tested host version. The adapter admits exactly the documented triggers `manual` and
  `auto`; any other trigger degrades `invalid-compaction-trigger`. A host without these events
  ignores the registration silently, so the gap MUST stay visible without them: every compact
  `SessionStart` context carries the AHI-030 disclosure and `compatibility.json` states that the
  live compaction cycle is `NOT_RUN`. Existing event names and receipts are unchanged.
- `AHI-027`: `pre-compact` MUST emit, as plain stdout the host joins verbatim into the compactor's
  custom instructions, one instruction line and one `corvint-compaction-pin/0` line naming the
  revision of the compact `SessionStart` receipt's `context.compaction` block, its tracked and
  untracked dirty-path counts, at most 24 admitted project-relative tracked dirty paths within
  1500 path bytes, and the count of paths elided. The block comes from the same read-only
  `session-start`/`compact` call AHI-003 uses. A clean, untracked-only or over-budget worktree
  yields no block or a block without a revision; the pin then names the revision of the same
  receipt's prompt packet, the block's counts (zero when absent), and every tracked path the block
  does not list as elided (V1-0293). An invalid block or revision degrades
  `compaction-block-unavailable`. Every output on the two compaction events is text, not hook JSON,
  within the 8000-byte bound: the host (Claude Code 2.1.267) joins PreCompact stdout into the
  compactor's instructions and shows PostCompact stdout to the user, and reads neither as hook
  JSON. A degradation prints its `systemMessage` frame text; an output with no text prints an empty
  line.
- `AHI-028`: `post-compact` MUST re-read the last pin the untrusted `compact_summary` preserved,
  re-validating every field so that a malformed or absent pin degrades
  `compaction-pin-not-preserved`, and MUST verify the pinned tree and each pinned path against the
  immutable object store with one hermetic, bounded `git cat-file --batch-check`
  (`compaction-pin-revision-unavailable` when the tree is gone,
  `compaction-pin-verification-unavailable` when Git fails or exceeds 500 ms, `git-unavailable`
  without a Git executable). It then MUST emit one `corvint-compaction-report/0` line naming the
  pinned revision, whether the current revision matches or moved, the rehydrated count, every
  non-rehydratable pinned path by name, the elided and untracked counts, and the current dirty
  count. An empty `compact_summary`, which the host's cached replacement compaction sends, has no pin
  to verify and no summary that dropped one, so `post-compact` prints an empty line and records no
  degradation. The host shows that line to the user only; the model-facing packet re-emission remains
  the compact `SessionStart` receipt of AHI-003.
- `AHI-029`: Neither compaction event writes repository, index, trace, or store state; the only
  write on the path is the SOL-V0-010 self-observation row a degradation records.
- `AHI-030`: Every compact `SessionStart` context MUST begin with a fixed trusted disclosure that
  names the host version the pin hooks are registered for, that their verdict reaches the user
  only, and that this packet is the model-facing rehydration and, on a host without the compaction
  events, the only one.
- `AHI-031`: (accepted 2026-09-25, decision 0400; panel blocker B8) When no index snapshot
  matches the tree and the automatic `dogfood event` read falls back to its in-memory build
  (`IDX-SNAP-V0-012`), an event whose deadline then expires MUST report
  `dogfood-event-index-snapshot-stale` instead of `dogfood-event-deadline` (`LCP-V0-008`), under
  the same return-without-waiting rule; an expiry before any snapshot miss keeps
  `dogfood-event-deadline`. The Claude Code reason
  `corvint-event-rejected:dogfood-event-index-snapshot-stale` is a fault under `AHI-021`: its
  `systemMessage` MUST carry the unchanged `Corvint FALLBACK degraded: <reason>; coding continues`
  frame, then on following lines that no snapshot matches the current tree and the exact JSON argv
  `["corvint","--root",ROOT,"index","--if-stale"]` for the adapter-resolved root. On
  `session-start` and `user-prompt` the same text MUST also be `hookSpecificOutput.additionalContext`,
  so the model can run the refresh under supervision; `pre-compact` and `post-compact` print the
  same text as plain stdout (`AHI-027`). The `SOL-V0-010` reason is read from the frame line alone, and the code
  joins the admitted `dogfood event` rejection registry. The hook still writes no snapshot and
  starts no refresh (`IDX-SNAP-V0-012`), so later events keep this notice until that argv runs;
  the refreshed snapshot is then a hit and needs no build. The Codex adapter reports the new code
  in its existing fallback text, without the argv. Rationale: a commit changes the tree, so on a
  large or loaded host every later prompt expired in the miss build, named only the deadline, in
  model-only context, with no remediation (panel report B8, 2026-09-25). Not decided here: deciding
  a miss in milliseconds without the build, refreshing the snapshot from explicit write commands,
  or serving the parent-tree snapshot with a dirty overlay.
  (accepted amendment 2026-09-26, decision 0422; V1-0286) A snapshot miss
  whose recorded `index` build cost is at least the time left before the deadline MUST report
  `dogfood-event-index-snapshot-stale` without starting the in-memory build (`IDX-SNAP-V0-012`
  amendment), so a large repository no longer spends the whole deadline on every
  prompt; with no usable record, or a recorded cost that fits, the miss builds and, on expiry,
  reports as above. The cause line after the frame then states that no snapshot matches the
  current tree and that building one in memory does not fit the hook deadline, which covers both
  outcomes. The Codex fallback MUST carry the same cause line and argv after its unchanged
  `Corvint fallback: <reason>; unrelated coding continues.` frame, in the one channel that fallback
  already uses for the event (`hookSpecificOutput.additionalContext` on `SessionStart` and
  `UserPromptSubmit`, the only events that compile a packet), replacing "without the argv" above;
  its `SOL-V0-010` reason is still read from the frame line. This closes the first "Not decided
  here" item; the other two stay open. Rollback: remove `withCodexSnapshotRemediation` and the
  record-based skip; neither changes stored state beyond the inert `build-cost.json`.

- `AHI-032`: At the owner’s request (2026-09-28), stock OpenCode native integration qualification
  MUST be reported separately from execution authority. The `opencode-native-integration/1`
  profile may report integration support `FULL` only for the exact host, adapter, OS and architecture
  whose retained native evidence passes every conformance case and AHI-012/013. Untested tuples
  remain `UNQUALIFIED`. Legacy `corvint-harness-event/0` receipts remain `FALLBACK`; execution
  authority is `NONE`, Frontier is `UNAVAILABLE`, and enforcement/continuation is not provided.
  This does not qualify protected lifecycle, admit an authority root, or change Core semantics.

  Adapter 0.7.3 admits an exact qualification campaign for host versions in the closed compatibility
  range `>=2.0.18 <2.1.0`. This range permits the producer to test a host; it does not transfer a
  record between versions or executable images. The producer MUST derive the actual host version
  from the executable, require the setup callback to report that same version, and bind the passing
  record to the exact version and executable digest. A compatible host without that record remains
  `UNQUALIFIED` and reports the qualification action. An earlier version, a 2.1-or-later version,
  malformed version output, missing record, or drifted record MUST fail closed without running the
  campaign from a status/read command.

  For this profile, AHI-006 pre-completion enforcement is unavailable. Cases 6 and 10 mean an advisory frontier observation at the strongest available
  native post-execution completion event and bounded duplicate/recursive invocation suppression. A host that
  cannot await continuation MUST disclose that limitation and retain the negative continuation
  probe; it MUST NOT claim an enforced stop, successful continuation or verified completion.
  All other conformance cases and the latency/recall gates remain required. Fault and cross-host
  normalization regressions may use the real adapter with deterministic transport fixtures;
  native discovery, prompt delivery, tool execution, edits, compaction and completion require the
  unmodified host and real Corvint binary. A loopback scripted provider is sufficient for transport
  qualification, but is not model-quality or real-world task-success evidence.

  At the owner's request (2026-10-01), the default awaited `session.prompt` hook MUST keep
  submitted user text unchanged. It MUST retain bounded, framed, receipt-linked task context only
  in bounded session memory and supply it through the model-request context hook, not visible or
  persisted user messages. Collection succeeds only while the original text is unchanged and the
  session remains active; late results from an older prompt generation MUST be discarded. A new
  distinct prompt MUST invalidate prior task context before a busy or failed collection can return.
  Duplicate events/message identities MUST preserve the same prompt's collected context, and a
  repeated context hook MUST NOT duplicate its frame within that request. Each complete task frame,
  including disclosure and framing, MUST be at most 8000 UTF-8 bytes. The adapter MUST count each
  in-flight request once (including first-prompt/startup overlap), bound concurrent calls and session
  state, cancel deleted or evicted sessions, and clear the task frame on compaction. It MUST NOT
  persist prompts or inspect transcripts. Native qualification MUST verify unchanged prompt drafts,
  absence of the automatic frame in provider user messages, and delivery of the same receipt-linked
  frame in model context; byte and timing evidence MUST measure this hidden delivery.
  `corvint_expand` encodes supplied pinned tree/blob/path tuples as cv1 selectors and delegates
  selection, identity validation and exact expansion to the existing Core command. It MUST refuse
  malformed, stale, oversized or envelope-colliding results without substituting worktree text.

  `session.compaction.ended` MUST invalidate prior startup context. The next awaited context hook
  requests `session-start` with `startSource: compact`; only a successfully framed bounded response
  may consume pending recovery. Overlapping recovery is serialized per generation, newer compaction
  invalidates older responses, and failures keep recovery pending without restoring stale context.
  Recovery uses current Git state only and makes no claim of complete task-state restoration.

  Promotion evidence MUST bind source digest, host executable digest, Corvint executable digest,
  exact version/platform tuple, case results, latency samples and byte/critical-recall measurements.
  `corvint_status` MUST verify the complete recorded gates, package contents, actual host image,
  configured Corvint image resolved from the invocation root, and version/platform tuple. Any drift
  reports `UNQUALIFIED`. This is maintainer evidence, not a tamper-resistant authority claim.
  Package declarations describe build-time capability; post-build qualification belongs outside the
  package so recording evidence does not change the tested package. A later source change invalidates
  qualification. The first-party qualification command MUST run both the focused adapter suite and
  native campaign itself, freeze package/executable/collector/test identities across both, and
  publish the exact consumer record at `integrations/opencode-qualification.json` only after every
  required result passes. Missing, skipped, failed or changed evidence MUST NOT promote support;
  callers cannot supply PASS overrides. A requalification MUST retain the previous record as
  evidence and atomically invalidate the active record before execution, so failure/interruption
  cannot leave stale FULL active. Final publication MUST be atomic. The record is ignored local
  derived evidence; the command documents its prerequisites and actual executable requirement.
  Under GOC-V0-008, the maintained producer, native campaign, PTY driver and their safety
  regressions MUST execute in Go. Collector identity MUST cover its entrypoint, implementation,
  embedded observers, build inputs and executing binary. Invalid executable admission MUST
  invalidate prior PASS after the source/output paths are admitted. Interrupted-child evidence
  MUST observe descendant absence before any witness rescue cleanup; a deliberately broken
  cleanup negative control MUST fail. Architecture names MUST match the Node consumer tuple.
  For issue #387, deterministic transport success MUST use the existing genuine query budget;
  it MUST NOT relabel an automatic session-start as a query or enlarge production deadlines.
  A delayed automatic fixture MUST still assert timeout and its declared deadline. The collector's
  final observed-descendant check MUST distinguish exited zombie state from a live same-generation
  PID/start identity. The passive interruption witness MAY wait up to 500 ms for captured identities
  to exit before supervisor rescue, without signalling them or expanding ownership. Missing, reused
  or zombie identities count as exited; snapshot errors and persistent live identities MUST fail.
  Regressions MUST cover an actual unreaped zombie, transient exit, persistent live survivor and
  the existing broken-cleanup negative control. Fixture success is not p95 latency evidence.
  A producer-to-consumer regression MUST cover successful publication, all retained failure classes,
  identity drift and interruption. Rollback is removal of the native plugin entry or reverting the package; no host
  fork, daemon, account, authority installation or durable outcome migration is required.


- `AHI-033`: The owner-approved first OpenCode UI slice (2026-09-28, “looks good. build it”
  following the context-sidebar and clickable-evidence proposal) MUST expose an optional native
  OpenCode 2.0.18 terminal sidebar and session panel. It MUST show the latest observed session
  context, inclusion reasons, authority/confidence labels, exact tree/blob identities, supplied
  omissions and gaps, and Core-expanded pinned source. `/corvint` opens the panel;
  `/corvint TASK` or the panel query action requests context explicitly. Keyboard and pointer
  selection, loading, empty, unavailable and stale states MUST remain usable at narrow widths.
  The measurable job is sidebar → evidence → exact cited source without a model call or manual
  CLI composition. This UI does not implement the proposed impact graph, requirement/proof tabs,
  verification timeline, desktop/web panels, or authority/coverage scoring.

  The inspector MUST retain only one bounded latest view per existing session, within the
  adapter's 128-session limit, and at most 32 evidence locations and 32 gap rows per view; display
  omissions MUST be disclosed. It MUST NOT scan transcripts, persist query text or receipts, start
  a daemon, refresh an index automatically, or create sessions during snapshot reads. RPC query
  admission MUST check the requested session's host project/location and reuse the existing bounded
  Corvint runner with cancellation, at most 16 simultaneous context/expansion operations and two
  per session, with no waiting queue. RPC is available to clients already trusted by OpenCode;
  session identifiers are routing inputs, not authentication credentials. The event bridge MUST
  disclose only hashed session identity; UI state remains client-local and volatile.

  Newer context requests, observed edits, compaction, deletion, eviction and disposal MUST prevent
  late publication and invalidate expansion from a replaced view. Expanding a source MUST require
  an exact handle from that session's currently ready receipt and recheck its identity after Core
  returns. Source reads MUST use Core's existing selector/identity verification and never substitute
  worktree bytes. Freshness is explicitly the state when observed, not a background live-HEAD check.
  Presentation MUST visibly escape terminal controls, invisible and bidi characters; exact source
  selectors remain unchanged. Repository text MUST NOT become markup, commands or instructions.

  The owner requested a stronger terminal experience on 2026-09-28 after the first slice. The
  browser MUST filter the supplied evidence by filename, symbol, reason or authority, preserve
  distinct citation selection through receipt reorder, and provide explicit no-match recovery.
  At wide panel widths it MUST show a scrollable evidence list beside details; compact terminals
  MUST expose one pane at a time with visible return navigation. Source reading MUST show line
  numbers, distinguish the cited line when valid, support keyboard scrolling and return-to-citation,
  and use host syntax support only after an explicit per-panel opt-in that discloses possible
  parser downloads. Default source reading MUST use plain rendering without requesting a parser. Presentation normalizes CRLF only;
  escaped controls MUST preserve line mapping and out-of-range citations MUST be disclosed.
  Host theme colors accompany text labels, never replace them. Query/search dialogs MUST isolate
  inspector shortcuts and discard results if their captured session/location is no longer current.
  Source caching is bounded to one expanded source in the mounted panel, invalidated by
  session/location/receipt/state changes and removed on disposal; no panel history is persisted.
  Pointer opening and visible actions MUST provide the same navigation as keyboard controls.

  Acceptance requires focused hostile-text/bounds/race/session tests and the real stock 2.0.18 TUI
  showing sidebar, context request, keyboard selection, pinned source and gap navigation at wide
  and narrow terminal widths, including short height, both theme modes, filtering/no-match recovery,
  pointer opening, dialog focus isolation, source scrolling, citation alignment and source invalidation.
  Acceptance uses current rendered frames, not accumulated terminal output. The native witness MUST
  retain source identity and demonstrate
  interruption leaves no owned descendants. It is UI evidence only and MUST NOT promote AHI-032
  integration support or execution authority. Failure leaves coding available with an explicit
  unavailable state. Rollback removes `src/tui.tsx`, the inspector RPC/view and its package export,
  restores the prior adapter under a new version, and keeps prior qualification evidence invalid.

- `AHI-034`: The owner-approved change cockpit (2026-09-28, “do it” after the change
  cockpit, impact navigation and proof-inspection proposal) MUST let an operator follow changed
  files → affected-test dependency witnesses → recorded verification without a model call. It
  MUST reuse the existing context/source inspector for governing evidence. Other proposed review,
  revision-diff and export modes are outside this slice. The optional native OpenCode UI remains
  a client of existing read commands, with no Core execution or authority change.

  `/corvint` MUST open the change view; an explicit task MUST retain the context-query behavior.
  The change view MUST expose files, impact witnesses, proof observations and unresolved gaps,
  with keyboard/pointer parity, wide list/details and narrow single-pane navigation, scoped dialogs,
  explicit refresh, base selection and a visible route to the evidence reader. The default base
  is the current worktree's recorded completion base when available, otherwise captured HEAD;
  an operator-selected ref MUST resolve to a full immutable commit before collection. `plan.dirty`
  supplies the advisory changed set; `range.paths` alone MUST NOT stand for dirty changes.
  Unit selection MUST retain its source path, witness kind and ordered dependency path. Suggested
  or declared checks MUST remain visibly distinct from observed executions, and absent connections
  MUST NOT imply unaffected files, safe test omission or complete requirements/coverage.

  The server MAY invoke only closed fixed-argv Git metadata/ref reads, `affected --base FULL_SHA`
  and `dogfood status --session-key HASH`, using the existing bounded owned-process runner. No
  shell, client command/path execution, test execution, index refresh, transcript scan, polling,
  persistence, provider or new daemon is admitted. Explicit reads MUST have a 1 MiB output and
  ten-second command bound, with a twenty-second aggregate call bound, shared 16-global/two-session
  admission, cancellation and descendant cleanup. Existing automatic-event bounds remain unchanged.
  One latest volatile cockpit per existing bounded session MAY retain at most 64 file, impact and
  check rows each and 128 gaps; every display omission MUST be disclosed. Snapshot reads MUST NOT
  allocate sessions or spawn processes. Host session/project/location admission MUST precede reads.

  Verification discovery MUST use only the fixed private Git directory's completion owner and
  Core status. A per-check `qualified` result MUST NOT become workflow satisfaction; `unmet`
  remains visible, and failed, stale, unrun, cancelled, timed-out and withheld observations remain
  distinct. Qualification binds the committed target: uncommitted work MUST mark prior check results
  stale for the displayed worktree even when Core still qualifies their committed target. Check commands are inert displayed text. Output reads MUST accept only a check ID from
  the current receipt, restrict paths to that owner's exact generation/check-log pattern, reject
  symlink directory components and symlink/nonregular/hardlinked files, validate the opened inode,
  and read at most 64 KiB per stream with visible truncation. Secret-screened output is withheld.
  Repository/session/receipt changes MUST discard late reads. Before and after log reading, the
  owner, Core policy/check projection, and Git identity MUST still match the captured view; check
  reruns or reenrollment require refresh. Logs remain caller-owned local bytes with no content
  attestation from the status API, and same-user concurrent mutation is not an authenticated boundary.
  Terminal controls/bidi remain escaped; output line breaks may be preserved for reading.

  Failure MUST retain a recoverable unavailable/stale state without interrupting coding. Observed
  freshness MUST NOT claim continuous monitoring of external edits. Acceptance requires focused
  input/bounds/path/owner-race/check-rerun/stale-result/admission tests and a real stock 2.0.18 native
  witness for file → impact → command/output, edit invalidation, both themes, narrow/pointer/dialog
  use and interruption without descendants. Check qualification with an unsatisfied workflow MUST
  be covered. Rollback restores the retained 0.6.0 package/configuration and leaves existing evidence
  records intact. UI witnesses do not promote harness support or execution authority.

- `AHI-035`: At the owner's request (2026-09-29), the OpenCode 2 terminal integration MUST
  expose Corvint Tasks metrics in both its sidebar and `/corvint` panel. The operator's job is to
  see the current queue shape, open-ticket eligibility and gate uncertainty, then inspect one
  ticket's blockers and acceptance criteria without a model call or task-store mutation.
  `/corvint tasks` and the sidebar Tasks action open a task view; the change view routes to it.

  The adapter MUST use only the task manager's read-only `queue status`, bounded
  `ticket search --status OPEN`, and selected `ticket show` verbs. It MUST use the existing
  owned-process cleanup, fixed argument validation, twenty-second aggregate admission, and
  session/project/location checks. The view retains one volatile snapshot per existing session:
  status counts (including drafts), blocked and active-attempt counts, intent-check count, at most 32 open tickets per
  page, observed time, queue identity and receipt digest. Queue summary, open page and detail reads
  MUST agree on the task manager's head receipt before they are joined. Refresh and page navigation
  are explicit; no polling, transcript read, task mutation or execution is permitted.

  Sidebar and panel MUST distinguish completed, open, draft, held and archived counts from blocked
  eligibility, gate observations and completion evidence. They MUST show uninitialized, missing,
  refused, malformed, changed and cancelled reads as unavailable or stale with a reason, never as
  zero counts or a pass. An absent page is labelled as omitted; a detail action accepts only a
  ticket from the current page and rechecks the queue binding. Untrusted titles, blockers and
  criteria MUST render as inert escaped text with bounds. Narrow and wide terminals support
  keyboard and pointer navigation, details, refresh and recovery. The view is an observer: it
  grants no execution authority or native task completion.

  Acceptance requires focused argument, envelope, snapshot-race, hostile-text, pagination,
  cancellation and process-cleanup tests; a real task-store read and a stock OpenCode 2 terminal
  witness for sidebar → task page → detail → refresh. A missing or unsupported host keeps this
  slice experimental and visible as such. Rollback removes the read-only Tasks RPC and terminal
  view, leaving the task store and its receipts untouched.

- `AHI-036`: At the owner's request (2026-09-29, “do all 6”), the optional OpenCode 2 workbench
  MUST let an operator focus one OPEN ticket from the current bounded Tasks page. Focus is explicit,
  volatile and session-scoped; it MUST bind the selected ticket revision and queue head receipt to
  its acceptance detail and reported live attempt. The overview MUST show ticket, branch/worktree,
  holder, phase, lease expiry, blockers, gate observation and completion observation separately.
  A changed queue receipt, session/location switch, deletion or eviction MUST make joins stale or
  unavailable. No title-based task ownership inference, claim, lease renewal or task mutation occurs.
- `AHI-037`: The workbench MUST show each selected ticket criterion alongside only explicitly
  declared requirement references, declared touch paths, observed changed paths, suggested checks,
  recorded check states and current context/change receipt identities. It MUST label every criterion
  `UNOBSERVED` until an accepted criterion-specific proof link exists; task-level coincidences,
  passing individual checks, a CEM, or an OCM MUST NOT silently satisfy it. Missing, stale,
  withheld and unrun proof remains visible. At most 16 criteria, 32 references and paths, and 32
  check observations are displayed, with omissions disclosed. Opening existing Change or Evidence
  views provides the actual witness, not a generated proof claim.
- `AHI-038`: An explicit qualification doctor MUST display the installed host version, adapter
  version, OS/architecture, exact native integration state and available failing conformance cases
  from the existing AHI-032 record, plus a concrete qualification action. A missing, mismatched or
  incomplete record remains `UNQUALIFIED`. Doctor output never promotes execution authority or
  treats a terminal rendering witness as native integration qualification.
- `AHI-039`: The workbench MUST derive at most five next actions from recorded ticket blockers,
  failed/stale/withheld checks, missing criterion proof and qualification state. Each action names
  its underlying observation and leads to the relevant existing view. It MUST NOT rank inferred
  intent, auto-run checks, or turn absence of a blocker into readiness.
- `AHI-040`: The terminal MUST show OpenCode's observed per-session cost and token/cache totals
  when available. Cost since ticket focus MUST require a local cost baseline taken at explicit
  focus of that ticket in this TUI; it MAY include other work after focus and MUST NOT be labelled
  exclusive ticket cost. Without the baseline, the focus interval is `NOT_OBSERVED`. Cost per verified
  criterion, savings and latency remain `NOT_OBSERVED` without complete corresponding evidence.
  Negative, malformed or missing host values MUST NOT become zero. No transcript or provider log
  scan is permitted.
- `AHI-041`: The terminal MUST show at most 16 members of the current OpenCode session family,
  their worktree, parent, explicit ticket focus where this plugin location can observe it, and
  reported holder/lease. It MUST flag duplicate explicit focus without claiming two active
  leases or proving a collision. Cross-worktree bindings that the location cannot observe remain
  `UNOBSERVED`; session titles MUST NOT establish task identity. Family reads and updates remain
  bounded, local and non-polling.

  AHI-036 through AHI-041 are an experimental read-only UI slice on the existing transport-neutral
  index and Tasks store. Acceptance requires focused receipt-race, malformed/hostile-data,
  attribution, bounds, family and cancellation tests; a real initialized Tasks read; and a stock
  OpenCode 2 terminal witness covering focus, all workbench tabs, keyboard/pointer navigation and
  cleanup. The source change invalidates earlier exact-package qualification. A missing stock
  witness or failing host campaign remains visible and does not turn unit tests into FULL support.
  Rollback restores the previous adapter package/configuration, removes workbench RPC/UI state,
  and leaves Tasks and evidence receipts untouched.

- `AHI-042`: The owner-approved OpenCode presentation polish (2026-09-30, “ok, update it”, explicitly including Corvint Tasks) MUST provide consistent Work, Change and Evidence destinations. Work observes the native Tasks queue and focused ticket; Change exposes Files, Impact, Checks and Attention; Evidence retains exact pinned source inspection. Slash, sidebar, palette and visible navigation labels MUST name their actual destination. Selected task identity and its observed revision/receipt MUST survive navigation without becoming a claim or execution permission.

  Focused Work MUST expose Task, Criteria, Blockers and Gates with disjoint primary shortcuts. Existing Doctor, Cost and Sessions remain reachable through a visible supplementary destination with keyboard/pointer parity. Native completion and gate summaries MUST remain separately labelled observations. The private read DTO MUST retain declared gate IDs separately as requiredGates {state, ids}: absent source is NOT_OBSERVED, an explicitly empty list is OBSERVED, malformed data refuses the detail, and at most32 escaped IDs are shown after validating the complete native list of at most256 nonempty bounded strings. Truncation and shortened IDs are disclosed. Declared IDs are not executed. Individual gate results and criterion-specific proof absent from the current native projection remain NOT_OBSERVED/UNOBSERVED; passing checks, task-level references and no listed blockers MUST NOT establish eligibility, criterion satisfaction or completion. Opening retained session Evidence MUST NOT be labelled as focused-ticket or criterion-linked evidence.

  Change MUST separate its exact source-owned permanent advisory/authority limitations from actionable Attention counts, while keeping the limitations accessible. Arbitrary unknown strings, missing executions, workflow obligations and display omissions MUST remain visible. Recorded check labels MUST distinguish passed, failed, stale, cancelled, timed-out, withheld, unverified and not-run observations. A historical pass MUST name its recorded commit and lose a success presentation when the snapshot is stale. Loading, empty, stale and unavailable screens MUST name the current section and a supported recovery action without fabricating counts. Theme colors accompany text; compact terminals preserve list/detail navigation.

  This presentation slice MUST preserve AHI-033 through AHI-041 bounds, fixed read commands, escaping, binding, explicit refresh, cancellations and disposal. It adds no Tasks writes, initialization, auto-run, inferred ownership or authority. Acceptance requires focused state/count/status tests, independent review and stock OpenCode current-frame witnesses for Work → Criteria → Change → Checks → Evidence and return, both themes, wide/compact/short layouts, keyboard/pointer navigation, supplementary views, invalidation and interruption cleanup. A rendering witness remains distinct from exact-package AHI-032 qualification. Failure preserves recoverable unavailable/stale states and source bindings. Rollback restores the retained 0.7.5 package/configuration under a new version and leaves native Tasks/evidence receipts intact and changed-package qualification invalid.

- `AHI-043`: For an unchanged committed tree, worktree state, hook input and session key, the Claude Code
  SessionStart and UserPromptSubmit adapters MUST write byte-identical stdout whatever the wall clock
  reads, so an injected packet never invalidates the host's prompt cache by itself (V1-0713, from the
  claude-mem finding that minute timestamps changed injected bytes every 60 s). The model-visible
  packet MUST NOT carry wall-clock time, elapsed time, process identity or any other per-invocation
  value; such a value belongs in the private degradation ledger or the user-only notice instead.
  Two exemptions are deliberate and are not byte-stability defects. First, whether an invocation
  finishes inside its deadline (`AHI-012`, `AHI-017`) depends on host load, so the same state can
  yield the full packet once and a deadline-derived outcome another time: a
  `dogfood-event-deadline`, `dogfood-event-index-snapshot-stale` or `adapter-host-kill-deadline`
  FALLBACK packet, or, only with the opt-in `CORVINT_EXPERIMENTAL_COMPACTION_KERNEL=1`, a trailing
  experimental-kernel `NOT_RUN` note. That variance is tracked as deadline reliability (V1-0607),
  and each outcome is itself byte-stable. Second, a changed tree, dirty path set, prompt, session key, index
  snapshot or installed binary changes the packet by design. Acceptance: a test runs both adapters
  three times, 61 s and then 25 h apart on a fake clock, against an unchanged fixture repository with a dirty path, and
  requires identical bytes and a full repository envelope from each run. Rollback removes this
  requirement and its test; packets keep their current content.
- `AHI-044`: Every shipped hook adapter MUST fail open under the faults a host or machine can impose
  (V1-0711). A table-driven test reads every Claude Code and Codex command from the shipped
  `hooks.json` files and runs the real binary for each under a cold start without a snapshot, empty
  stdin, malformed stdin, stdout closed before the write, an unavailable index, a host that never
  closes stdin, and slow Git on `PATH`. Every run MUST exit 0, the non-blocking status of every
  native host. When a fault changes the result, stdout MUST name it with a `FALLBACK degraded:` or
  `Corvint fallback:` code. Empty or malformed input yields `malformed-hook-json` before any spawn.
  A closed stdout yields `hook-stdout-unwritable` on stderr: the Go adapter receives `SIGPIPE` as an
  error instead of dying from it, and the Gemini wrapper handles the stream error the same way. The
  fixture's `.corvint/.gitignore` lets the ledger writers run, and at least one run MUST write the
  self-observation ledger. Each run MUST leave the repository, home and temporary trees unchanged
  apart from the declared local ledgers and the writer's `.self-observations.*` temporary. That
  includes the private `corvint-git-status-*` scratch, which a deadline-abandoned status read used
  to leave behind. The process now removes it at exit (`gitstatus.CloseScratch`). The test counts
  every process spawned through `PATH`. A healthy invocation needs at most 15, the cap the test
  asserts: 15 for session start, prompt and pre-compact, 10 for stop and Codex session end, at most
  10 for Claude session end and 5 for post-tool. Unparseable input MUST spawn none. Shims for `sh`,
  `bash`, `zsh`, `dash`, `ksh`, `fish`, `csh` and `tcsh` MUST never run, so no adapter spawns a
  login shell through `PATH` or `$SHELL`. A spawn by absolute path or a re-exec of the binary itself
  is invisible to these counts. The Gemini wrapper's empty, malformed, absent-binary and
  closed-stdout cases run under `TestHostAdapterJavaScriptHosts`.
  Exemptions:
  - For Claude Code and Codex, a missing `corvint` binary is the host's own spawn failure, and no
    Corvint code runs. How each host reports that failure is NOT_OBSERVED.
  - OpenCode is an in-process plugin, not a hook. Its missing-binary path is covered by the
    existing OpenCode cases.
  - A root outside a Git repository stays silent `{}` (decision 0178).
  - The ticket's case of a host deadline shorter than the adapter's budget is replaced, not met. No
    budget the adapter derives can meet a kill earlier than the declared one.
    `TestAHI017AdapterHostKillMatchesDeclaredHooks` keeps the shipped timeouts equal to the declared
    kill table. The two delay cases force each adapter onto its own deadline first. A host that
    kills earlier anyway sends `SIGKILL`, so the exit cleanup cannot run. The status scratch, any
    `.self-observations.*` temporary and running Git children can then remain. That consequence is
    inferred, not observed. The owner accepted this disposition on 2026-10-04 (decision 0430).
  - Elapsed time and how long an abandoned Git child outlives the adapter are logged, not asserted,
    because the parallel matrix runs under load (decision 0082). The watchdog bound itself stays
    under AHI-017. Children outliving the adapter are tracked as V1-0734.
  - Real hosts remain NOT_OBSERVED.
  Rollback: delete the tests and this requirement. Restoring default `SIGPIPE` handling and the
  plain scratch removal reintroduces the observed death on a closed stdout and the scratch leak.
- `AHI-045`: (accepted by decision 0441; V1-0942) The Claude Code and Codex
  `corvint-dogfood-event/0` adapters MUST inject, inside the unchanged `AHI-004` envelope, the
  `corvint-hook-context/0` projection of the engine receipt rather than the whole receipt. The
  projection carries `profile`, `event` and only these actionable fields when present: task
  evidence rows with their blob pins, declared-scope rows (only non-current ones outside a
  main-thread SessionStart), critical omissions as `omitted`, unavailable selectors as
  `unavailable`, degradation codes (outside a main-thread SessionStart only those other than the
  per-installation `frontier-authority-unavailable` and `host-version-unknown`), the compaction
  block reduced to revision, state, mode, paths, results, verification, rehydration counts, named
  omissions and a non-clean freshness, and, once any of those is present, governance rows, an
  unresolved anchor resolution, a non-clean freshness and a non-inactive policy (lifecycle,
  satisfied, unmet). It MUST NOT carry the adapter block, repository identity, request or result
  digests, coverage counters, the constant Frontier result or the completion decision. The engine
  receipt, its profile and its validation (`LCP-V0-011`, Pi `workflow.js`) are unchanged, so
  `localCompletionProfile` and `compatibility.json` keep `corvint-dogfood-event/0`; the projection
  is derived from the receipt the same invocation produced. This amends `AHI-004`'s receipt-linked
  clause for these two adapters: the receipt digests are no longer model-visible.
  Compatibility: an old plugin with a new binary receives the projection; a new plugin with an old
  binary receives the full receipt; both are framed identically.
  Non-goals: no change to Stop gating, PreCompact/PostCompact plain-text output, degraded and
  FALLBACK outputs, the `AHI-016` disclosure or the opt-in compaction kernel suffix.
  Failure modes: a consumer that parsed `repository` or `adapter` from additionalContext finds them
  absent; `tools/native-hook-observer` (experimental, UNQUALIFIED) still compares the full receipt.
- `AHI-046`: (accepted by decision 0441; V1-0942) Outside a main-thread
  SessionStart, an event whose projection carries no task evidence, non-current scope, omission,
  unavailable selector, non-baseline degradation or compaction block MUST inject nothing: the
  adapter writes `{}` (or only the opt-in kernel suffix as its own context). Governance rows, an
  unresolved anchor count and baseline degradations alone never trigger a packet: with nothing to
  name, silence is the abstention (invariant 2), and `explicit-task-anchor-required` stays in the
  engine receipt. The `URE-V0-008` ledger records a silent prompt as an empty planned set; a
  degraded or undelivered packet still refuses it.
  Failure modes: an anchorless prompt that needed governance gets none until an anchor, a direct
  query or the next SessionStart; the direct query command is unchanged.
- `AHI-047`: (accepted by decision 0441; V1-0939) The Claude Code trusted
  workflow argv guidance (`LCP-V0-011`) MUST appear only on a main-thread SessionStart, including
  `source=compact`, and never on UserPromptSubmit or another event; a blocked Stop names only its
  `Next:` recovery argv. A Claude Code hook payload with a
  non-empty `agent_id` is a subagent: its SessionStart gets no guidance and the `AHI-046` rule
  instead of the full projection. Codex exposes no subagent signal, so every Codex SessionStart
  projects in full.
  Failure modes: a host that omits `agent_id` in a subagent gets the main-thread packet (the former
  cost, not a loss); a host that sets it on the main thread loses the guidance until the next
  SessionStart without it. `agent_id` semantics are read from the Claude Code 2.1.267 binary's
  hook schema strings only; a live subagent run is NOT_OBSERVED.
  Acceptance (`AHI-045` to `AHI-047`): the tests in the traceability table and the before/after
  replay in `docs/build-log/2026-10-07-hook-context-and-dogfood-summary.md`.
  Rollback (`AHI-045` to `AHI-047`): revert `cmd/corvint/host_adapter_projection.go` and its call
  sites; adapters again inject the whole receipt with guidance on every context event. No store,
  ledger or wire format changes, so either side reads the other's evidence.

## Native platform profiles

| Platform | Embedded host-admission key | Maintained Corvint package | Native surfaces | Stability rule |
|---|---|---|---|---|
| Codex CLI/Desktop | `codex` | Codex plugin | skills, hooks, MCP | test startup/resume/clear/compact separately; compaction recovery remains dirty-path-only |
| Codex IDE | `codex` (shared host; separate surface status) | standalone Codex integration | standalone skill, shared MCP, supported hooks | plugins are unavailable; never inherit CLI/Desktop status |
| Claude Code | `claude-code` | Claude Code plugin | skills, hooks, MCP | publish minimum/maximum tested plugin API versions; `PreCompact`/`PostCompact` pin hooks verified against 2.1.267 only, live cycle `NOT_RUN` |
| Gemini CLI | `gemini-cli` | Gemini CLI extension | context file, commands, skills, hooks, MCP | validate extension environment filtering and hook schemas |
| OpenCode | `opencode` | OpenCode 2 plugin (`@corvint/opencode` 0.3.0 and later; adapter 0.7.3 admits exact campaigns for `>=2.0.18 <2.1.0`; 1.x unsupported) | stable `session.created`/`session.execution.*`/`session.deleted` events, `tool.hook("execute.after")`, plugin tools, MCP `mcp.servers`; beta `session.hook("context")` isolated | legacy receipts remain `FALLBACK`; native integration qualification uses AHI-032 |
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
- Compaction non-goals (`AHI-026`..`030`): the pin is not a survival manifest and the
  `compact_summary` is never trusted beyond the re-validated pin fields; `PostCompact` cannot
  block compaction or reach model context, so the report is user display only; `PreCompact` exit
  status never blocks compaction; neither event reads the transcript; the CEP §3 survival trial is
  not claimed by any of this.
- Failure mode (`AHI-027`): a summary that drops the pin line leaves `post-compact` with nothing
  to verify; it degrades `compaction-pin-not-preserved` and the compact `SessionStart` packet is
  the whole recovery.
- V0 does not claim model-internal observation, causal attribution, test execution authority,
  complete tool interception, an empty Frontier, or compatibility outside the tested matrix.

The simpler baseline is a user invoking Corvint CLI or MCP tools manually. Native packages exist only
to remove repeated lifecycle glue while preserving the same receipts and explicit gaps.

### Failure codes

Beyond the codes named above, the shared `harness event` core (`internal/gokernel/harness.go`), its
repository probe (`internal/gokernel/repository.go`), native adapter
(`cmd/corvint/host_adapter.go`), and OpenCode qualification producer emit the kebab-case codes below
(decision 0100). Each row cites
the first emitting site and quotes the message returned there or states the condition checked
there, which is the whole of what the row asserts.

| Code | First emitting site | At the cited site |
|---|---|---|
| `corvint-output-too-large` | `cmd/corvint/host_adapter.go:958@1b317e61` | adapter output cannot be marshaled, or with its final LF exceeds 8000 bytes; a degraded `systemMessage` naming this reason is written instead |
| `canonical-json-failed` | `internal/gokernel/harness.go:458` | "cannot encode receipt basis" |
| `compaction-block-unavailable` | `cmd/corvint/host_adapter_compaction.go:119@e26bd5d6` | Claude adapter: the compact `session-start` receipt carries no `context.compaction` block, or its revision is not a Git object ID |
| `compaction-pin-not-preserved` | `cmd/corvint/host_adapter_compaction.go:88@5fbc4775` | Claude adapter: `compact_summary` holds no pin line whose every field re-validates |
| `compaction-pin-revision-unavailable` | `cmd/corvint/host_adapter_compaction.go:95@81c5dd24` | Claude adapter: the object store reports the pinned tree as missing |
| `compaction-pin-verification-unavailable` | `cmd/corvint/host_adapter_compaction.go:209@1c06bb2b` | Claude adapter: the pin's `cat-file --batch-check` failed, timed out, or answered a different number of queries |
| `git-unavailable` | `cmd/corvint/host_adapter_compaction.go:196@1f1f42d0` | Claude adapter: no `git` executable is on `PATH` when `post-compact` verifies a pin |
| `harness-input-too-large` | `internal/gokernel/harness.go:361` | "harness input exceeds its byte limit" |
| `harness-output-too-large` | `internal/gokernel/harness.go:466` | "harness response exceeds its byte budget" |
| `invalid-compaction-trigger` | `cmd/corvint/host_adapter_compaction.go:67@3ccad220` | Claude adapter: the `PreCompact`/`PostCompact` `trigger` is not `manual` or `auto` |
| `invalid-harness-adapter` | `internal/gokernel/harness.go:73` | "invalid <label>" |
| `invalid-harness-budget` | `internal/gokernel/harness.go:340` | "harness budget must be at least <value> bytes" |
| `invalid-repository-root` | `internal/gokernel/harness.go:376` | "cannot resolve repository root" |
| `malformed-corvint-output` | `cmd/corvint/host_adapter.go:677@2c724e09` | Claude adapter: the `harness event` stdout is not JSON; the degraded `systemMessage` names this reason |
| `project-root-unavailable` | `cmd/corvint/host_adapter.go:332@2100b4c9` | Claude adapter: the project root (`CLAUDE_PROJECT_DIR`, else the working directory) cannot be made absolute; the degraded `systemMessage` names this reason |
| `qualification-in-progress` | `internal/opencodequalification/record.go:433` | AHI-032: the producer atomically writes the active record as `INCOMPLETE` after preserving any previous record; integration support stays `UNQUALIFIED` until complete passing evidence replaces it |
| `repository-identity-malformed` | `internal/gokernel/repository.go:178` | "Git object identity is malformed" |
| `repository-probe-cancelled` | `internal/gokernel/repository.go:167` | "Git repository probe was cancelled" |
| `repository-probe-timeout` | `internal/gokernel/repository.go:165` | "Git repository probe exceeded its 10-second deadline" |
| `repository-probe-too-large` | `internal/gokernel/repository.go:148` | "Git output exceeds its byte limit" |
| `repository-profile-malformed` | `internal/gokernel/repository.go:268` | "Git profile path is not valid UTF-8" |
| `repository-status-malformed` | `internal/gokernel/repository.go:215` | "Git status output is malformed" |
| `repository-status-too-large` | `internal/gokernel/repository.go:238` | "Git status exceeds the <value>-path limit" |
| `unsupported-git-object-format` | `internal/gokernel/repository.go:192` | "unsupported Git object format: <value>" |

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
`file-change` dispatch, with a package version bump each (`AHI-020`). The OpenCode 2 port rolls
back by republishing the 0.2.9 OpenCode 1 plugin (`server` export, `client.app.log`, `file.edited`)
under a new version and restoring the opencode matrix row; no stored state or wire field changes. `AHI-023` rolls back by restoring `unknown` as the Claude Code host version
in `dogfoodHostVersions`; the receipt then carries `host-version-unknown` again. The decision 0178
not-a-repository rule rolls back as that decision's Rollback section describes. `AHI-031` rolls
back by restoring the fixed `dogfood-event-deadline` code in `runLocalCompletionEvent` and removing
`withSnapshotRemediation`; no stored state or wire field changes.

## Traceability

| Requirement | Implementation surface | Required evidence |
|---|---|---|
| `AHI-001`, `003`, `005`, `014` | shared `corvint harness event` core and `internal/projectpath` | canonical receipt, bounds, privacy, revision, and event fixtures; `TestHostAdapterAbsentPathContainment` and `TestRelativeAliasesAndUncertainty` cover `AHI-014` path containment, and the `integrations/host-adapters.test.mjs` test `AHI-014 Gemini classifies changed paths on resolved symlinks like internal/projectpath` under `TestHostAdapterJavaScriptHosts` covers the Gemini hook's symlink resolution; `TestClaudeAdapterForkSessionStartIsResume` covers the Claude `fork` start source; `TestAHI003ClaudeCompactSessionStartRehydratesDirtyPaths` drives the Claude `SessionStart(source=compact)` hook entrypoint over a mixed dirty worktree and requires the tracked impact, the untracked count and `compaction-untracked-paths-not-rehydratable` from the receipt's own snapshot; `TestQualifiedLifecycleCompactSessionStartRehydratesDirtyPaths` requires the same for the qualified profile under FULL and FALLBACK and refuses a reordered, extra or dropped code; `TestAHI014EventExpectationsAreHostConsistent` (`conformance/harness-event-v0/host_schema_test.go`) pins each `common-logical-interaction.json` event's closed host set and requires every present host's golden `expected` object to be byte-identical, so a per-host field or host-membership mutation of that fixture fails here |
| `AHI-032` | `internal/opencodequalification`, `tools/qualify-opencode`, native prompt hook and qualification consumer | `internal/opencodequalification/record_test.go::TestRecordValidation`, `::TestAtomicRecord`, `::TestArchitecture`, `::TestProducerConsumer`; `internal/opencodequalification/command_test.go::TestInvalidHostInvalidatesQualification`; `internal/opencodequalification/witness_posix_test.go::TestGateInterruptionWitness`; first-prompt startup overlap, quiet-context busy/compaction regressions in `TestHostAdapterJavaScriptHosts`, and `internal/opencodequalification/native_test.go::TestAHI032HiddenPromptDelivery`; `internal/opencodequalification/bounded_read_test.go::TestEvidenceReadsRefuseOverBoundFiles` (16 MiB evidence bound before allocation; open rows accept unknown members); exact-tuple native campaign required |
| `AHI-033` | `integrations/opencode/src/inspector.js`, `src/tui.tsx`, and inspector RPC in `src/index.js` | `integrations/opencode/inspector.test.mjs`, AHI-033 cases under `TestHostAdapterJavaScriptHosts`, and `tools/qualify-opencode --inspector` (stock native rendering, pinned source, narrow keyboard use and interruption cleanup) |
| `AHI-034` | `integrations/opencode/src/cockpit.js`, `cockpit-tui.tsx`, `index.js`, `runtime.js` | `integrations/opencode/cockpit.test.mjs` and `tools/qualify-opencode --inspector`: bounded fixed reads, safe output paths, stale/owner/check-rerun races, advisory impact navigation, independent workflow/check state and real native change/proof workflow |
| `AHI-042` | `integrations/opencode/src/ui-presentation.js`, `cockpit-tui.tsx`, `tui.tsx`, `task-tui.tsx`, `workbench-tui.tsx`, `task-metrics.js` and `workbench.js` | `integrations/opencode/ui-presentation.test.mjs`, AHI-042 declared-gate cases in `task-metrics.test.mjs` / `workbench.test.mjs`, and the stock native inspector witness for bounded Work → Change → Evidence navigation and honest status/limit presentation |
| `AHI-043` | `cmd/corvint/host_adapter.go` Claude SessionStart/UserPromptSubmit output | `cmd/corvint/host_adapter_stability_test.go::TestAHI043ClaudeContextPacketsAreByteStableAcrossTime` (fake-clock 61 s and 25 h gaps, unchanged dirty fixture, full envelope required) |
| `AHI-044` | `cmd/corvint/host_exit.go` (`adapterStdout`, `hookStdout`, `exitProcess`), `cmd/corvint/signals_unix.go` `notifyBrokenPipe`, `internal/gitstatus/scratch.go`, `integrations/gemini-cli/hooks/corvint-hook.mjs` | `cmd/corvint/host_adapter_fail_open_test.go::TestAHI044HookAdaptersFailOpen` (every shipped Claude Code and Codex hook × seven faults: exit 0, named cause, spawn cap, no shell, no writes outside live ledgers), `internal/gitstatus/scratch_test.go` (`TestAHI044ScratchRemovedAtClose`, `TestAHI044ScratchCloseRacesReads`) and the AHI-044 Gemini case under `TestHostAdapterJavaScriptHosts` |
| `AHI-045`–`AHI-047` (accepted by decision 0441; V1-0939, V1-0942) | `cmd/corvint/host_adapter_projection.go` (`hookContextProjection`, `hookCompaction`, `claudeSubagent`, `claudeSessionGuidance`, `withHookContextSuffix`), `renderAdapterResult` and both adapters in `cmd/corvint/host_adapter.go`, `recordDeliveredPacket`, `conformance/host-lifecycle-v1` | `cmd/corvint/host_adapter_projection_test.go` (`TestAHI046HookContextProjectionSilenceRule`, `TestAHI046SilentProjectionRendersNothing`, `TestAHI047GuidanceIsMainThreadSessionStartOnly`, `TestAHI046CodexPromptSilenceAndProjection`); `TestClaudeNativeDogfoodLifecycle` subtests for the first blocked Stop, the anchored prompt, the silent anchorless prompt and main-thread versus `agent_id` SessionStart; `TestAHI003ClaudeCompactSessionStartRehydratesDirtyPaths` (projected compaction results equal the receipt's); the silent-prompt case of `TestClaudeAdapterUnplannedReadCallSites`; `conformance/host-lifecycle-v1` projection case |
| `AHI-036`–`AHI-041` | `integrations/opencode/src/workbench.js`, `workbench-tui.tsx`, `session-metrics.js`, `task-metrics.js`, `qualification.js`, and inspector RPC | `integrations/opencode/workbench.test.mjs`, focused AHI-036 task-detail receipt test in `task-metrics.test.mjs`, and stock OpenCode 2 terminal witness; exact-package AHI-032 qualification remains separate |
| `AHI-025` | `cmd/corvint/pi_tools.go`, `integrations/pi/tools.js` | `TestPiToolContextExpansion`, `TestPiToolRecord`, `TestPiToolClosedInput` and native Pi tool/RPC fixtures |
| `AHI-026` | `integrations/claude-code/plugins/corvint/hooks/hooks.json`, `compatibility.json` `compactionHooks`, `cmd/corvint/host_adapter.go` declared-kill table | `TestAHI026ClaudeCompactionHooksRegisteredAgainstHostAPI` (matcherless `PreCompact`/`PostCompact` groups, verified host version equals the tested maximum, closed trigger set) and `TestAHI017AdapterHostKillMatchesDeclaredHooks` (the two new declared kills) |
| `AHI-027` | `cmd/corvint/host_adapter_compaction.go` (`runClaudeCompactionEvent`, `compactionBlockFor`, `compactionPinLine`), `emitAdapterOutput` plain-stdout branch | `TestAHI027ClaudePreCompactEmitsPinFromCompactionBlock` (instruction plus pin as text, pin equals the fixture's HEAD tree and tracked dirty path, 24-path bound with hostile paths elided); `TestAHI027ClaudeCompactionPinsCleanAndUntrackedOnlyTrees` (clean and untracked-only trees pin the HEAD tree and report without a fault; an over-budget block elides its unlisted tracked paths); `TestClaudeCompactionDegradationIsPlainText` (degradations print frame text through `compactionPlainOutput`, an empty summary prints an empty line) |
| `AHI-028` | `cmd/corvint/host_adapter_compaction.go` (`runClaudePostCompact`, `parseCompactionPin`, `compactionPinMissing`, `compactionReportLine`) | `TestAHI028ClaudePostCompactReportsNonRehydratablePaths` (exact report naming the pinned path the tree lacks; lost, escaping and unresolvable pins degrade by name) |
| `AHI-029` | both compaction events | `TestAHI029ClaudeCompactionHooksMutateNothing` (byte-size snapshot of the whole fixture including `.git` is unchanged across a pin and its verification); `TestAdapterDegradationAdmitsCompactionEventsAndCodes` (every compaction degradation is admitted to the SOL-V0-010 ledger on both events) |
| `AHI-030` | `cmd/corvint/host_adapter_compaction.go` (`compactSessionDisclosure`), Claude branch of `runClaudeAdapter` | `TestAHI003ClaudeCompactSessionStartRehydratesDirtyPaths` (compact `SessionStart` additionalContext begins with the disclosure) |
| `AHI-031` | `cmd/corvint/local_completion_event.go` (`dogfoodExpiryCode`, snapshot-miss flag in `localEventContext`, `dogfoodMissOutlastsDeadline`), `cmd/corvint/host_adapter.go` (`withSnapshotRemediation`, `adapterDegradationReason`, `snapshotRemediation`, `withCodexSnapshotRemediation`), `internal/observations` rejection registry | `TestDogfoodEventSnapshotMissExpiryNamesStaleSnapshot`, `TestClaudeAdapterStaleSnapshotNamesRemediation` (fail at base 489701ca with `dogfood-event-deadline` and no argv); V1-0286 amendment (accepted, decision 0422): `TestDogfoodEventSnapshotMissUsesRecordedBuildCost`, `TestCodexAdapterStaleSnapshotNamesRemediation` |
| `AHI-004` | native adapter renderers, shared lifecycle command, `internal/repoenvelope`, and the JavaScript envelope builders | byte-identical untrusted-data envelope with hidden-character escaping and terminator refusal (`internal/repoenvelope`, `cmd/corvint`, `tools/native-hook-observer` and `integrations/host-adapters.test.mjs` tests), injection bounds, authority order, and query fixtures |
| `AHI-011`, `015` | embedded `internal/gokernel/host-schema.json` admission table and shared lifecycle command | schema/admission tests plus one host-keyed golden fixture per admitted host |
| `AHI-002`, `006..010` | four native packages and release matrix | install/uninstall, lifecycle, degradation, and version fixtures; for `AHI-010`, the `integrations/host-adapters.test.mjs` test under `TestHostAdapterJavaScriptHosts` binding each `integrations/compatibility.json` row to its shipped declaration and its row's adapter version to the package manifest version, and asserting `globalDegradations` disjoint from `receiptDegradationPolicy.recognised`; for the `AHI-009` owned-group kill, the `V1-0371` OpenCode and Gemini timeout-and-cancellation and normal-exit tests in that file, whose `orphan-hang` and `orphan-valid` fixture descendant ignores SIGTERM and closes its stdio |
| `AHI-016` | `cmd/corvint/prompt_bound.go`, Claude and Codex native wrappers; `integrations/gemini-cli/hooks/prompt-bound.mjs` and `integrations/opencode/src/prompt-bound.js` in the Gemini CLI hook and OpenCode `corvint_context` tool | `TestAHI016OverBoundPromptDerivesVerbatimAnchorQuery`, `TestAHI016OverBoundPromptKeepsRefusalWhenAnchorsCannotServe`, `TestAHI016ClaudeOverBoundPromptInjectsDisclosedContextWithoutStoring`, `TestAHI016OverBoundPromptMatchesCrossHostBoundaryCases`, and the `AHI-016` cross-host test in `integrations/host-adapters.test.mjs` under `TestHostAdapterJavaScriptHosts` |
| `AHI-017` | `cmd/corvint/host_adapter.go` declared-kill table and watchdog; `integrations/gemini-cli/hooks/corvint-hook.mjs` derived budget | `TestAHI017AdapterHostKillMatchesDeclaredHooks` (which also fails on a matcherless `FileChanged` group), `TestAHI017HostAdapterWatchdogDegradesBeforeHostKill`, `TestHostAdapterPanicDegradesInsteadOfBlocking`, and the two `AHI-017` Gemini tests in `integrations/host-adapters.test.mjs` under `TestHostAdapterJavaScriptHosts` |
| `AHI-018` | `integrations/gemini-cli/hooks/corvint-hook.mjs` argument parser and the `host-adapters.test.mjs` fixture harness | the `AHI-017` Gemini declared-kill test, which pins the shipped command to carry no override, under `TestHostAdapterJavaScriptHosts` |
| `AHI-019` | `cmd/corvint/host_adapter.go` (`postToolChangeOutOfRoot`, `invokeLegacyClaudeEvent`, `claudeReceiptOutput`, `claudeDegradationsRecognised`, `renderAdapterResult`) | `TestClaudeAdapterRoutineReceiptAndExpectedDegradationCarryNoNotice` asserts no output for an out-of-root Edit target and a `PostToolUse` additionalContext receipt with no `systemMessage` for an in-root one; `TestAdapterSilentAbstentionsAreLedgered` requires the out-of-root Write target's `{}` output and its `post-tool-path-not-project-relative` ledger row (V1-0746); `TestAHI019ClaudePostToolReceiptNamesEveryDegradation` drives a realistic Edit `PostToolUse` payload through the hook entrypoint and requires the receipt's degradation codes in that additionalContext; `TestAHI019ClaudeReceiptRefusesUnrecognisedDegradation` calls `claudeReceiptOutput` with an unrecognised code and requires the named `corvint-degradations-unrecognised` fault `systemMessage` without the code; `TestAHI019CodexWholeReceiptRefusesUnrecognisedDegradation` and `TestAHI019ClaudeDogfoodEnvelopeRefusesUnrecognisedDegradation` call `renderAdapterResult` with an unrecognised code and require exactly the Codex and Claude Code fault output, which carries no code, and the Codex test also requires that fault for a string, object, or `null` `degradations` value |
| `AHI-020` | the four host package manifests listed in the requirement body | `script/check-host-package-versions.sh` (`make host-package-versions-check`) compares, per package, the last commit that changed a version field against the last commit that changed any other shipped file, by Git ancestry between the two commits rather than by committer-second timestamp (two distinct commits, such as either side of a rebase, can share a committer second); `script/check-host-package-versions_test.sh` proves it fails on a content-only change, passes after a version bump, and fails on a newly added shipped file with no bump, and its fifth case proves a bump and a later content change sharing one committer second still fails |
| `AHI-021` | `cmd/corvint/host_adapter.go` (`claudeExpectedDegradation`, `claudeDegradedOutput`, `renderClaudeContext`) | `TestClaudeAdapterRoutineReceiptAndExpectedDegradationCarryNoNotice` asserts `prompt-over-query-bound` is `UserPromptSubmit` additionalContext with no `systemMessage`; `TestClaudeAdapterDogfoodEventDeadlineCarriesNoNotice` asserts an expired dogfood event deadline is `UserPromptSubmit` additionalContext with no `systemMessage`; `TestClaudeAdapterFaultKeepsNotice` asserts a `missing-session-identity` fault inside a repository keeps its `systemMessage`; `TestClaudeAdapterNotARepositoryIsSilent` asserts Claude `session-start`, Edit and Write `post-tool`, and Codex `SessionStart` outside a Git repository emit nothing and create no `.corvint` (decision 0178) |
| `AHI-022` | `integrations/gemini-cli/hooks/corvint-hook.mjs` (`EXPECTED_DEGRADATIONS`, `degradation`, `successOutput`); `integrations/opencode/src/index.js` (`report`, `record`, `CHANGED_TARGETS`, `afterTool`, `EVENT_HANDLERS`, `MAX_FILE_CHANGE_PATHS`, `queueFileChange`, `drainFileChanges`); `integrations/opencode/src/runtime.js` (`receiptIdentity`); `internal/gokernel/harness.go` (`takeAdapterCodes`) | under `TestHostAdapterJavaScriptHosts`, the `integrations/host-adapters.test.mjs` tests `AHI-022 Gemini routine receipt and expected degradation carry no notice`, `AHI-022 Gemini fault keeps notice`, `AHI-022 decision 0178 Gemini outside a Git repository emits and invokes nothing`, `AHI-022 OpenCode routine receipt goes to the info log and a fault keeps its warning` (including the structured suffix refusal), `AHI-022 decision 0378 OpenCode outside a Git repository registers and invokes nothing`, `AHI-022 decision 0378 OpenCode repository detection follows subdirectories, symlinks and linked worktrees`, `AHI-022 decision 0379 OpenCode lifecycle against the real binary writes nothing to the terminal` (the real `cmd/corvint` build, which pins the `invalid-arguments` non-repository refusal the permissive fixture cannot), `AHI-022 V1-0746 OpenCode names the path cap and an out-of-project path at info level` (including the `adapterCodes` each carries), `SOL-V0-010 AHI-022 V1-0767 OpenCode path abstentions reach the self-observation ledger against the real binary`, `AHI-022 V1-0773 OpenCode file-change of 101 to 256 paths stays within Core's impact bound against the real binary`, and `AHI-022 OpenCode serializes a burst of file-change subprocesses`; `CRB-V0-010 CRB-V0-011 OpenCode loaded plugin keeps exact aliases, option precedence, session isolation, payload bounds and repeat-stop suppression` covers the `execute.after` changed-path mapping and the `app.version` host version |
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
  `https://code.claude.com/docs/en/hooks`, and `https://code.claude.com/docs/en/mcp`; the
  installed 2.1.267 hook runner documents `PreCompact` (trigger, custom_instructions; stdout joined
  into the compactor's instructions), `PostCompact` (trigger, compact_summary; stdout shown to the
  user only), and `SessionStart` with source `compact`
- Gemini CLI: `https://geminicli.com/docs/extensions/reference/` and
  `https://geminicli.com/docs/hooks/reference/`
- OpenCode: `https://opencode.ai/v2/docs/build/plugins` and
  `https://opencode.ai/v2/docs/mcp-servers` (read 2026-09-27; plugin API pinned to 2.0.18)
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

AHI-025 functional witnesses (2026-09-22): the actual Pi 0.85.1 offline fixture calls native
context, exact expansion, performs an edit and explicit verification observation, and durably
records the explicit caller-reported outcome at a clean revision. Native RPC new-session recovery
and interactive TUI prompt/shutdown are exercised separately. The PTY harness has a regression
for interrupted cleanup of a TERM-ignoring descendant. These functional witnesses do not establish
protected FULL, arbitrary extension qualification, latency or equal-critical-recall promotion.
