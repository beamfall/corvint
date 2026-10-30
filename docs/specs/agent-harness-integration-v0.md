# Agent Harness Integration V0

Owner: Russell Lewis
Date: 2026-08-23
Intent status: accepted direction; AHI-024 Pi tuple accepted (decision 0452; V1-0506); AHI-045..047 accepted (decision 0441; V1-0939, V1-0942); AHI-048 proposed (V1-0734); AHI-049 accepted (decisions 0477, 0479; V1-1040, V1-1055); AHI-050 accepted (decision 0474; V1-0371); AHI-051 proposed (V1-1070, V1-1071); AHI-052 accepted (decision 0497; V1-1116); AHI-054 accepted (decision 0519; V1-1157); AHI-055 accepted under the owner's standing auto-accept (2026-10-11) (V1-1280); AHI-056 accepted under the owner's standing auto-accept (2026-10-11) (V1-1969); AHI-057 accepted under the owner's standing auto-accept (2026-10-11) (V1-1974); AHI-070 accepted under the owner's standing auto-accept (2026-10-11) (V1-2062); AHI-171 accepted under the owner's standing auto-accept (2026-10-11) (V1-2192); AHI-172 accepted under the owner's standing auto-accept (2026-10-11) (V1-2201); AHI-071 accepted under the owner's standing auto-accept (2026-10-11) (V1-2150; Pi tuple now 0.99.1 / 0.3.8, decisions 0550, 0556, 0558 and 0583); AHI-173 and the AHI-020 Spec Kit/OpenSpec amendment accepted under the owner's standing auto-accept (2026-10-11) (V1-2145, V1-2184); AHI-174..AHI-179 accepted under the owner's standing auto-accept (2026-10-11) (V1-2346, V1-0453); AHI-190..199 accepted under the owner's standing auto-accept (2026-10-11) (V1-0403, V1-2466, V1-2440); AHI-372 accepted under the owner's standing auto-accept (2026-10-11) (V1-3086)
Delivery status: experimental
Authoritative inputs: `docs/PRODUCT.md`, `docs/TECHNICAL-BRAIN.md`,
`docs/specs/cem-0.2-canonical-binding.md`

## Agent digest
- Claim: Corvint exposes bounded native lifecycle adapters and qualifies stock OpenCode integration separately from execution authority.
- Status: accepted direction; AHI-024 Pi tuple accepted (decision 0452; V1-0506); AHI-045..047 accepted (decision 0441; V1-0939, V1-0942); AHI-048 proposed (V1-0734); AHI-049 accepted (decisions 0477, 0479; V1-1040, V1-1055); AHI-050 accepted (decision 0474; V1-0371); AHI-051 proposed (V1-1070, V1-1071); AHI-052 accepted (decision 0497; V1-1116); AHI-054 accepted (decision 0519; V1-1157); AHI-055 accepted under the owner's standing auto-accept (2026-10-11) (V1-1280); AHI-056 accepted under the owner's standing auto-accept (2026-10-11) (V1-1969); AHI-057 accepted under the owner's standing auto-accept (2026-10-11) (V1-1974); AHI-070 accepted under the owner's standing auto-accept (2026-10-11) (V1-2062); AHI-171 accepted under the owner's standing auto-accept (2026-10-11) (V1-2192); AHI-172 accepted under the owner's standing auto-accept (2026-10-11) (V1-2201); AHI-071 accepted under the owner's standing auto-accept (2026-10-11) (V1-2150; Pi tuple now 0.99.1 / 0.3.8, decisions 0550, 0556, 0558 and 0583); AHI-173 and the AHI-020 Spec Kit/OpenSpec amendment accepted under the owner's standing auto-accept (2026-10-11) (V1-2145, V1-2184); AHI-174..AHI-179 accepted under the owner's standing auto-accept (2026-10-11) (V1-2346, V1-0453); AHI-190..199 accepted under the owner's standing auto-accept (2026-10-11) (V1-0403, V1-2466, V1-2440); AHI-372 accepted under the owner's standing auto-accept (2026-10-11) (V1-3086)/experimental
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
SessionStart matcher admits `fork`. The qualified native-hook path (`cmd/corvint/native_hook.go`) applies
the same map, so `fork` is a `resume` there too (V1-0445, `TestNativeHookForkSessionStartIsResume`). On `compact`, Corvint rehydrates a bounded impact packet from the exact
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

For the Claude Code and Codex native plugin paths, `--adapter-version` and the receipt's
`adapter.adapterVersion` carry the adapter wire-format version `0.1.0`, named
`adapterWireVersion` in `cmd/corvint/host_adapter.go`. This is independent of the shipped plugin
package version. The local completion reader admits that same wire value for both hosts; the
legacy Claude harness event also sends it. V1-0444 clarifies the existing AHI-001 transport
contract without changing the flag, JSON key, value, receipt bytes, or admitted tuples. Pi's
separate exact extension tuple remains governed by AHI-024.

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
  `session.hook("context")`, the OpenCode `corvint_context`/`corvint_record_outcome` tool outputs,
  and the Pi `corvint_core_read` tool output, which refuses a colliding result as the tool error
  `corvint-envelope-terminator-collision`; V1-3031, `integrations/pi/core.test.mjs`).
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
  OpenCode and Gemini owned-process-group cleanup on a timeout, a cancellation or a normal leader
  exit, including which signal or probe answers confirm it and the visible degradation
  `corvint-process-cleanup-unconfirmed`, is governed by `AHI-050` (decision 0474), which supersedes
  this requirement's earlier post-exit SIGKILL and `EPERM`-as-confirmed text.
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
  `unsupported-harness-event` (`internal/gokernel/harness.go:384,387@2b8bd0c3`). A host API change MUST NOT
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
  NOT carry a `systemMessage`. The abstention itself is recorded under `SOL-V0-010` as the
  exclusion `post-tool-path-outside-project` only for a target that resolves outside the root, or
  otherwise as the degradation `post-tool-path-not-project-relative` (an unresolvable target, a
  symlink escape, or a case-only or alias spelling of the root; `SOL-V0-030`, V1-1975; V1-0746). An in-root target's receipt ID MUST reach the model as
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
  its set instead, including `mcp.example.json` and all paths under `skills`
  (clarification accepted, decision 0514). This is distinct from `AHI-010`'s per-release compatibility publication: a host that
  caches an installed package by version (observed for Claude Code, `docs/build-log/2026-09-10.md`) never re-stages
  changed content under an unchanged version, so a stale version number silently withholds a fix or feature from an
  already-installed host. The Codex package's build-metadata suffix (`0.1.0+codex.<timestamp>`) advances by refreshing
  the timestamp; the other three packages advance by semantic-version bump.
  Amendment (accepted under the owner's standing auto-accept (2026-10-11); V1-2184): the Spec Kit
  extension (`protocol/integrations/speckit/corvint/extension.yml`, `extension.version`; shipped
  content is that directory except `extension.yml` and `README.md`) and the OpenSpec skill
  (`protocol/integrations/openspec/skills/corvint-openspec/SKILL.md`, front-matter
  `metadata.version`; shipped content is `protocol/integrations/openspec` except `README.md`, the
  skill file included, so a skill body edit must land with its bump) are covered the same way,
  because both hosts install them by version. A failure names the `AHI-173` bump command.
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
  rather than emit a warning. So is a path the host just created and Git does not yet track
  (V1-0450): `harness event --event file-change` MUST refuse a path not tracked at the indexed
  revision with the structured code `untracked-impact-path`, keeping impact's message bytes, and
  OpenCode MUST record that code at `console.info`. The code names an expected worktree state, not
  an unsupported capability, so it appends no `SOL-V0-007` row; `corvint impact` and the MCP bridge
  keep their uncoded refusal and its `NOT_TRACKED_AT_REVISION` mapping. Two path abstentions are expected and named at `console.info`
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
  `0.3.8` (superseding `0.3.7`, `0.3.6`, `0.3.5`, `0.3.4`, `0.3.3` and `0.3.2` under `AHI-071`, decisions 0550, 0556, 0558 and 0583), host `pi`, and surface `extension`; other versions MUST refuse visibly. The native
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
  runtime admission. Earlier 0.85.1 evidence is historical, not qualification of adapter 0.3.8.
  Protected Pi runtime/image contracts retain their separately pinned tuple. The exact ordinary-tuple bridge amendment is accepted by decision 0452 (V1-0506); its FALLBACK support and qualification limits are unchanged.

  The output has exactly `profile`, `event`, `host`, `surface`, `hostVersion`, `adapterVersion`,
  `support`, `receiptId`, `context`, `degradations`, `fault`, and `shouldContinue`. Constants are
  `corvint-pi-adapter/0`, `pi`, `extension`, `0.3.8`, `FALLBACK`, and false respectively.
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
- `AHI-071`: (accepted under the owner's standing auto-accept (2026-10-11), decision 0550; V1-2150)
  Amendment to `AHI-025`. Pi 0.99.1 converts a null integer argument to 0 before validation, so a
  `corvint_expand` call with a null or missing `result` or `evidence` index MUST refuse with the
  adapter-local typed fault `invalid-expansion-index` before host conversion (through the tool's
  `prepareArguments`) and again in the handler, and MUST NOT dispatch `adapter pi-tool expand` or
  expand row 0. The refusal is a visible Pi tool error; it never echoes caller text. This schema
  change bumps the adapter to `0.3.3` together, superseding the accepted 0.99.1 / 0.3.2 tuple
  (decision 0452) in AHI-024, LCP-V0-008/009 and the PWV-V0 inventory; 0.3.2 now refuses visibly.
  Under `AHI-020`, the shipped PWV-V0-013..017 content then bumps the adapter to `0.3.4`,
  superseding 0.3.3 in the same locations (decision 0556); 0.3.3 now refuses visibly.
  The manifests alone then moved to `0.3.5` for later shipped content while the emitters and native
  admission stayed `0.3.4`; under `AHI-020` the reconciled package bumps every location to `0.3.6`
  (decision 0558). 0.3.4 and the never-emitted 0.3.5 now refuse visibly. The AHI-024 version-literal
  test in `integrations/pi/capabilities.test.mjs` fails when any emitter or native admission
  literal drifts from `package.json`. The manifests alone then moved to `0.3.7` again while the
  emitters and native admission stayed `0.3.6` (V1-2854); the reconciled package bumps every
  location to `0.3.8` (decision 0583), and 0.3.6 and the never-emitted 0.3.7 refuse visibly.
  `AHI-240` makes the bump tool own the emitter and native literals, so the half bump cannot recur.
  Evidence: `AHI-071` in `integrations/pi/tools.test.mjs`; `TestPiLocalCompletionTuple`. Failure
  mode: a host that strips null before `prepareArguments` still reaches schema validation, which
  rejects the missing required index. Rollback: revert the refusal and restore 0.3.2 everywhere
  together; the binary and package must be replaced as a pair.
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
- `AHI-057`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1974) Amendment to
  `AHI-027` and `AHI-028`. A `pre-compact` output that carries no pin (any degradation, including a
  dogfood-event deadline and the watchdog kill) MUST begin its plain stdout with one instruction
  line and the line `corvint-compaction-pin/0 unavailable`, then the unchanged degradation frame
  text; the `systemMessage` frame and its `SOL-V0-010` reason are unchanged. `post-compact` MUST
  report `compaction-pin-not-emitted` when the last pin-shaped line in `compact_summary` is that
  unavailable line (whitespace between its two tokens may be reflowed), and otherwise MUST keep
  `AHI-028` unchanged: the last pin is verified, and a summary with neither a re-validating pin nor
  the unavailable line still degrades `compaction-pin-not-preserved`. A pin-shaped line is the
  profile followed by `revision=` or `unavailable`; the last one is found before it is validated,
  so a later pin the summary truncated degrades `compaction-pin-not-preserved` and is never
  hidden behind an earlier unavailable line or replaced by an earlier whole pin. Rationale: from 2026-10-14
  to 2026-10-20 every recorded compaction summary without a pin (39 of 173) fell in an hour in
  which `pre-compact` had degraded; each of the 67 `compaction-pin-not-preserved` ledger hours had
  a `pre-compact` degradation in that hour or the one before, 64 of them a dogfood-event deadline;
  and all 511 recorded summaries that contain a pin parse and re-validate. The `compaction-pin-not-preserved` rows therefore reported a
  pin the summary never received as one it dropped. Non-goals: this does not make `pre-compact`
  meet its deadline, and the compact `SessionStart` packet still does not carry the pin.
  Failure modes: a summary that drops the unavailable line still reports
  `compaction-pin-not-preserved`; untrusted summary text that ends with a forged unavailable line
  yields `compaction-pin-not-emitted`, which is still a degradation and never a clean report.
  Evidence: `TestAHI057ClaudeCompactionDistinguishesUnemittedPinFromLostPin`. Rollback: revert the
  unavailable line and its detection, so a degraded `pre-compact` prints its frame text alone.
- `AHI-053`: (accepted, decision 0517 (V1-1128)) Amendment to `AHI-031`
  and the no-writer wording of `AHI-029`: stale Claude/Codex notices retain their frame, code,
  channels and exact manual argv but may report a background refresh started, already running,
  disabled or failed to start under `IDX-SNAP-V0-031` through `IDX-SNAP-V0-036`. The current event
  remains fail-open and never claims completion. Only the separately exec'd index writer and
  its coordination lock are admitted; ordinary reads remain non-mutating. Rollback is
  `CORVINT_INDEX_AUTO_REFRESH=0` or reverting this experimental amendment.
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
- `AHI-055`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1280) A Claude Code or Codex notice whose reason is
  `corvint-event-rejected:dogfood-event-deadline` (`LCP-V0-008`) MUST keep its unchanged frame and
  code as the first line, then on following lines say in plain words that Corvint's check for the
  event did not finish within the hook time limit, that the event got no Corvint context and coding
  continues, that a measured reproduction found heavy machine load to be the cause while 1,266
  changed task-store files under `.taskman/` added no measurable time, and that the notice did not
  measure this event's cause (V1-1524 amendment, accepted under the owner's standing auto-accept; it
  replaces the earlier unmeasured "very large set of uncommitted or staged changes, or heavy machine
  load" wording), and give the read-only next step as the exact JSON argv
  `["git","-C",ROOT,"status","--short"]` for the adapter-resolved root, then say that waiting for
  load to drop may help if machine load caused the timeout, without promising recovery, that files under `.taskman/` are written by the task store and must not be
  committed, stashed or discarded for this, and that only a very long list of other changes is worth
  committing or unstaging. The hook MUST NOT run that command or claim a cause it did not observe;
  the reproduction's cause is named as a reproduction finding, not as this event's measurement. A root long enough that the encoded output
  would exceed `adapterOutputLimit` keeps the frame-only notice rather than losing the frame to
  `corvint-output-too-large`. The text is appended in every channel the notice already uses: Claude
  `systemMessage` on `stop` and other user-visible events, `hookSpecificOutput.additionalContext`
  where `AHI-021` moves the expected deadline (`session-start`, `user-prompt`), the
  `pre-compact`/`post-compact` plain-text notice (`AHI-027`), and the one channel the Codex
  fallback chose for the event. The `SOL-V0-010` reason and the host lifecycle runner's time-bound retry still read the frame line
  alone, and the `AHI-031`/`AHI-053` stale-snapshot text is unchanged. Rationale: the Stop notice
  named only the code (V1-1280, 2026-10-11) when a checkout with thousands of dirty paths on a
  loaded host outlasted the event bound. Rollback: remove `withDeadlineRemediation`.

- `AHI-032`: At the owner’s request (2026-09-28), stock OpenCode native integration qualification
  MUST be reported separately from execution authority. The `opencode-native-integration/1`
  profile may report integration support `FULL` only for the exact host, adapter, OS and architecture
  whose retained native evidence passes every conformance case and AHI-012/013. Untested tuples
  remain `UNQUALIFIED`. Legacy `corvint-harness-event/0` receipts remain `FALLBACK`; execution
  authority is `NONE`, Frontier is `UNAVAILABLE`, and enforcement/continuation is not provided.
  This does not qualify protected lifecycle, admit an authority root, or change Core semantics.

  Adapter 0.7.3 admits an exact qualification campaign for host versions in the closed compatibility
  range `>=2.0.18 <2.1.0`; adapter 0.7.18 widens it to `>=2.0.18 <2.2.0` (AHI-174) and adapter 0.7.32 to `>=2.0.18 <2.3.0` (AHI-245). This range permits the producer to test a host; it does not transfer a
  record between versions or executable images. The producer MUST derive the actual host version
  from the executable, require the setup callback to report that same version, and bind the passing
  record to the exact version and executable digest. A compatible host without that record remains
  `UNQUALIFIED` and reports the qualification action. An earlier version, a version above the range,
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

- `AHI-056`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1969; GitHub #1218)
  Amendment to `AHI-032` latency evaluation. The OpenCode native campaign keeps the absolute AHI-012
  limits (query p95 at most 500 ms, lifecycle p95 at most 250 ms, at least 20 post-warm-up samples
  each). When the timing campaign's samples are complete and delivered but miss only a p95 limit, the
  producer MUST run exactly one further same-run timing campaign against the same host, adapter and
  Corvint images, and that re-sample alone decides the `latency` check. A short or undelivered
  sample, or a fault during either campaign, MUST NOT be re-sampled or excused. Both attempts' p95
  values, sample counts and missed limits MUST be retained under `metrics.latencyAttempts` with the
  method under `metrics.latencyMethod`; `metrics.query`/`metrics.lifecycle` hold the decisive
  attempt, which the consumer record check re-derives against the same limits. A campaign failure
  MUST name every failed check (a `latency` check with no timing attempt reads
  `latency-samples-missing`), and for `latency` each attempt's measured p95 against its limit
  (for example `latency (attempt 1: lifecycle p95 299ms > 250ms; attempt 2: ...)`), not only in
  `report.json`. Rationale: 14 of about 45 campaign tries failed on latency alone with elevated early
  samples that then dropped to baseline, which is transient host load. A steady regression misses
  both attempts and still fails; limits are not widened and no `PASS_WITH_LATENCY_WARNING` result
  is added. Failure mode: load that persists across both attempts still fails the campaign, and an
  intermittent regression that clears on the re-sample can pass once; repeated campaigns retain each
  attempt for review. Evidence: `TestAHI056LatencyResample` in `internal/opencodequalification`.
  Rollback: revert the re-sample so the first timing campaign decides alone.
- `AHI-171`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2192; GitHub #1406)
  Amendment to `AHI-056`. Each OpenCode native timing window (the first campaign and its re-sample)
  MUST record host contention at its start and end under `metrics.latencyAttempts[].contention`:
  1-, 5- and 15-minute load averages, the logical CPU count, the run-queue length where the host
  exposes it (`/proc/loadavg` runnable count on Linux) and the reading's source. A value the host
  does not expose MUST be recorded as `NOT_OBSERVED`, never zero. Each attempt and
  `metrics.latencyOutcome` (for the decisive attempt) MUST carry one of `PASS`,
  `LATENCY_HOST_CONTENDED` (a complete, delivered sample that missed only a p95 limit in a window
  whose highest observed 1-minute load average per logical CPU exceeds the declared threshold of
  0.5, recorded as `metrics.contentionThresholdLoadPerCPU`) or `LATENCY_REGRESSION` (any other
  miss, including one whose contention was not observed). When the decisive outcome is
  `LATENCY_HOST_CONTENDED` and every other check passed, the report `result` MUST be `RETRY_LATER`
  with a `retryHint` naming the load below which to retry, the campaign error MUST say the miss was
  under host contention and not a regression, and `tools/qualify-opencode` MUST exit 75 instead of
  1. `RETRY_LATER` does not qualify the host: the consumer record still requires `PASS`, and the
  absolute AHI-012 limits are unchanged. Non-goals: a same-window baseline sample of a previously
  qualified binary or no-op probe, CPU steal time, and widening limits. Failure modes: a host that
  stays loaded keeps returning `RETRY_LATER`; a genuine regression measured on a loaded host is
  reported as retry-later until it is retried on a quiet one, where it fails as a regression; load
  averages lag a short burst. Evidence: `TestAHI171LatencyHostContention` in
  `internal/opencodequalification`. Rollback: revert `internal/opencodequalification/contention.go`
  and its call sites so a latency miss is a plain `FAIL` again.
- `AHI-172`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2201; GitHub #1406)
  Amendment to `AHI-171`; it replaces that requirement's same-window baseline non-goal. While each
  OpenCode native timing window runs, the harness MUST spawn a fixed no-op probe that does not
  depend on the candidate (`/usr/bin/true`, else `/bin/true`) every 100 ms and record, under
  `metrics.latencyAttempts[].baseline` and, for the decisive attempt, `metrics.latencyBaseline`: the
  probe path, the sample count, the probe p95, its declared quiet-host reference of 2 ms, the
  slowdown (probe p95 over that reference, at least 1), the candidate query and lifecycle p95 each
  relative to the probe p95, the AHI-012 limits scaled by the slowdown and the declared tolerance of
  1.5, and the comparison `WITHIN_SCALED_LIMITS` or `BEYOND_SCALED_LIMITS`. A window with fewer than
  20 completed probe spawns, or with no probe, MUST record each measured field as `NOT_OBSERVED`.
  A miss that meets every `AHI-171` condition for `LATENCY_HOST_CONTENDED` MUST instead be
  `LATENCY_REGRESSION` when the baseline was observed and the candidate is beyond its scaled limits,
  because the candidate is slower than the host's slowdown explains. When the baseline was not
  observed, the `AHI-171` load rule decides alone. A contended `retryHint` MUST also name the
  baseline slowdown. A miss on a host below the load threshold stays `LATENCY_REGRESSION` however
  slow the probe was. The Linux `/proc/loadavg` reader MUST be exercised from a fixture file on every
  host: load averages, the runnable count as the run queue, and `NOT_OBSERVED` for a malformed
  line. Non-goals: sampling a previously qualified binary, a per-host calibrated reference, CPU steal
  time, and widening the absolute limits. Failure modes: the 2 ms reference is a declared constant,
  so a host whose quiet probe spawn is slower shows a standing slowdown and the load rule then
  decides as in `AHI-171`; process-spawn cost and hook latency need not scale alike; the macOS
  scheduler favours short-lived processes over equal-priority CPU-bound threads, so a loaded window
  can still pass. Evidence: `TestAHI172SameWindowBaseline` and `TestAHI172ProcLoadavgFixture` in
  `internal/opencodequalification`; the live OpenCode 2.0.18 runs and the live Linux reading in
  `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-gh1406-1021r-latency-baseline.md`. Rollback: revert
  `internal/opencodequalification/baseline.go` and its call sites so `AHI-171` decides alone.
- `AHI-372`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3086; GitHub #1982)
  Amendment to `AHI-172`. A decisive miss that meets every `AHI-171` condition for
  `LATENCY_HOST_CONTENDED`, whose `AHI-172` baseline was observed and `WITHIN_SCALED_LIMITS`, and
  whose query and lifecycle p95 are each at most the AHI-012 limit times the qualifying scale MUST be
  `QUALIFIED_UNDER_CONTENTION` instead. The qualifying scale is the same-window slowdown capped at
  the declared maximum scale factor of 1.5, with no tolerance (so at most 750 ms query and 375 ms
  lifecycle p95). A contended miss beyond the capped limits but within the `AHI-172` scaled limits
  stays `LATENCY_HOST_CONTENDED` (`RETRY_LATER`, exit 75), and one beyond them stays
  `LATENCY_REGRESSION`. When latency is the only failed check, the native report `result` MUST be
  `QUALIFIED_UNDER_CONTENTION`, never `PASS`; the `latency` check stays false and its failure entry
  carries that reason; and `contendedQualification` MUST record `latencyQualification: CONTENDED`,
  the measured query and lifecycle p95, the quiet limits, the baseline slowdown, the maximum and
  applied scale factors, the scaled limits, the peak load per CPU and the threshold.
  `tools/qualify-opencode` then exits 0. The consumer record check MUST accept that result only by
  re-deriving the scale from `metrics.latencyBaseline.slowdown` (a number at least 1 with comparison
  `WITHIN_SCALED_LIMITS`), matching the recorded scale, cap and peak load above the threshold,
  requiring every other check, requiring a sample above a quiet limit and within the scaled limits,
  and setting `conformance.latency` to `QUALIFIED_UNDER_CONTENTION`. A quiet record is unchanged:
  `result` and `conformance.latency` stay `PASS` with no `contendedQualification`, so a consumer that
  requires quiet qualification requires `result: PASS`. Non-goals: widening the quiet limits, a
  per-host cap, and a contended qualification without an observed baseline. Failure modes: a
  regression of up to 1.5 times the limits measured only on a contended host qualifies as contended
  until it is measured on a quiet one; the 2 ms reference may overstate a slowdown, which the cap
  bounds. Evidence: `TestAHI372ContendedQualification` in `internal/opencodequalification` with the
  GitHub #1982 shapes (lifecycle 257 and 296 ms at slowdown 7.7 and 10 qualify; 400 ms stays
  retry-later; 3000 ms is a regression). Rollback: revert the AHI-372 branches in `contention.go`,
  `native.go` and `record.go` so a contended miss is `RETRY_LATER` again.
- `AHI-173`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2145) One command
  MUST bump or verify every version location of a host package together:
  `script/host-package-version.sh bump PKG [X.Y.Z]` (`make host-package-version-bump PKG=...`) and
  `verify [PKG...]` (`make host-package-version-verify`). The locations are the `AHI-020` manifest
  version and every copy `AHI-010` requires to equal it: the Claude Code `plugin.json`,
  `marketplace.json` entry and `compatibility.json` `adapter.version`; the Codex `plugin.json` and
  `compatibility.json`; the Gemini CLI extension and `compatibility.json`; the OpenCode and Pi
  `package.json` versions and `corvintIntegration.adapterVersion`, and Pi's `compatibility.json`;
  each host's `integrations/compatibility.json` entry; the Spec Kit `extension.version`; and the
  OpenSpec `metadata.version`. `bump` MUST refuse when the locations disagree, default to the next
  patch version, change only the one quoted value at each location (format preserved, each JSON
  edit checked against jq's `setpath` of the same path) and never rewrite recorded evidence
  versions (`lastValidation`, `lastConformance`, `hostVersionEvidenceAdapterVersion`). `verify`
  MUST name each disagreeing location. The `AHI-020` failure message MUST name the bump command.
  Non-goals: choosing whether a change is a patch or minor bump; the Codex build-metadata suffix
  (no package carries one today). Failure modes: a location the script does not list, or a value
  that cannot be rewritten uniquely, is refused rather than edited by guess. Evidence:
  `script/host-package-version_test.sh` against copies of the real manifests and the Spec Kit
  case in `script/check-host-package-versions_test.sh`, both run by `make host-package-versions-test`.
  Rollback: delete `script/host-package-version.sh`, its test and the two make targets, and restore
  the earlier failure message.
- `AHI-240`: (experimental under the standing lane auto-accept (2026-10-27); V1-2854) Amendment to
  `AHI-173` and `AHI-020`. The Pi package has one version source, `integrations/pi/package.json`
  `version`. Every other Pi adapter version literal is a location `script/host-package-version.sh`
  bumps and verifies with it: the manifest `corvintIntegration.adapterVersion`, both
  compatibility manifests, the `runtime.js` `PI_ADAPTER_VERSION` constant and the native
  `piAdapterVersion` constant in `cmd/corvint/pi_adapter.go` (a `lit:NAME` location: the one quoted
  string assigned to NAME). `workflow.js` and the Pi node tests carry no literal of their own;
  they use `PI_ADAPTER_VERSION`. `make host-package-versions-check`, which CI runs on every change,
  MUST also run `verify` for every package, so a manifest-only bump fails in the same check that
  demanded it. Non-goals: loading `package.json` at runtime (the protected Pi build bundles
  `runtime.js`, and an import of the manifest is unqualified on the Pi host); recorded evidence
  versions. Failure modes: a `lit:` location with zero or two assignments refuses as "no single
  value" rather than editing by guess; a literal the tool does not list is caught by the AHI-024
  node test. Evidence: `script/host-package-version_test.sh` (pi bump changes six lines; a drifted
  `runtime.js` or `pi_adapter.go` literal is named by verify; the `host-package-versions-check` make
  recipe runs verify) and `AHI-024` in `integrations/pi/capabilities.test.mjs`. Rollback: remove the two `lit:`
  locations and the verify call; the AHI-024 node test still catches drift, later.
- `AHI-249`: (experimental under the standing lane auto-accept (2026-10-28); V1-2505) Amendment to
  `AHI-173` and `AHI-240`. The OpenCode `integrations/opencode/src/runtime.js` `ADAPTER_VERSION`
  constant is a `lit:ADAPTER_VERSION` location of the `opencode` package, so
  `script/host-package-version.sh bump opencode` rewrites it with `package.json` `version`,
  `corvintIntegration.adapterVersion` and the matrix entry, and `verify` (run by
  `make host-package-versions-check`) names it when it differs from the manifest version. Before
  this, a bump left the runtime literal stale, the check passed, and the OpenCode qualification
  producer/consumer test failed later with qualification-tuple-or-gate-mismatch. Non-goals:
  loading `package.json` at runtime; recorded evidence versions. Failure mode: zero or two
  `ADAPTER_VERSION` assignments refuse as "no single value". Evidence:
  `script/host-package-version_test.sh` (opencode bump changes four lines; a drifted `runtime.js`
  `ADAPTER_VERSION` is named by `verify opencode` and blocks `bump opencode`). Rollback: remove the
  `lit:ADAPTER_VERSION` location.
- `AHI-340`: (experimental under the standing lane auto-accept (2026-10-28); V1-2911; GitHub #1917)
  An observed qualification command (`internal/opencodequalification` `runCommand`) whose owned
  process group was cleaned and whose only descendant-observation failure lines are snapshot gaps
  beyond the observation bound (`procgroup.SnapshotGapFailurePrefix`) MUST end as a retry-later
  outcome (exit 75, as `AHI-171`), not `observed command cleanup incomplete` (exit 1). The message
  names the gap, the 1-minute load average with the logical CPU count (`NOT_OBSERVED` when unread)
  and the load below which to retry. A survivor failure, `groupClean: false`, an empty failure list
  with `absent: false`, or a missing observation stays `cleanup incomplete`, and every cleanup error
  names `groupClean`, `absent`, the process count and the quoted failures, never the process
  records. Non-goals: widening the 10 s bound; treating a gap as proof of cleanup (the report keeps
  `absent: false`). Evidence: `TestObservedCleanupClassification`. Rollback: restore the `%+v`
  error; busy hosts again fail as incomplete cleanup.
- `AHI-174`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2346) Amendment to
  `AHI-032`. Adapter 0.7.18 admits an exact qualification campaign for OpenCode `>=2.0.18 <2.2.0`:
  2.0.x with x at least 18 and every canonical 2.1.x. The Go producer
  (`ParseSupportedHostVersion`, `SupportedHostRange`), the JavaScript consumer
  (`compatibleHostVersion`, `SUPPORTED_HOST_RANGE`) and `package.json`
  `corvintIntegration.compatibleHostRange` MUST state the same range. A prerelease, a leading-zero
  component, an over-long component, 2.2.0 or later, and anything before 2.0.18 MUST stay
  unsupported and fail closed. The range still only admits a campaign: each exact version and
  executable digest needs its own passing record. Evidence: the live 2.1.3 native campaign retained
  under `conformance/opencode-native-v1/results/2.1.3-darwin-arm64/native-report.json`;
  `TestSupportedHostVersion` (`AHI-009 AHI-032 AHI-174` cases) and the AHI-177 range test. Failure
  modes: a 2.1 point release that changes a plugin API passes admission but fails its own campaign
  and stays `UNQUALIFIED`. Rollback: restore `<2.1.0` in the three locations; 2.1.x records then
  fail closed as unsupported. `AHI-245` widens the range to `<2.3.0`.
- `AHI-245`: (experimental under the standing lane auto-accept (2026-10-27); beamfall/corvint#1936)
  Amendment to `AHI-174`. Adapter 0.7.32 admits an exact qualification campaign for OpenCode
  `>=2.0.18 <2.3.0`: 2.0.x with x at least 18, every canonical 2.1.x and every canonical 2.2.x.
  The Go producer (`ParseSupportedHostVersion`, `SupportedHostRange`), the JavaScript consumer
  (`compatibleHostVersion`, `SUPPORTED_HOST_RANGE`) and `package.json`
  `corvintIntegration.compatibleHostRange` MUST state the same range. A prerelease, a leading-zero
  component, an over-long component, 2.3.0 or later, and anything before 2.0.18 MUST stay
  unsupported and fail closed. Admission permits a campaign and does not qualify 2.2: each exact
  version and executable digest still needs its own passing record, and `testedHostVersions` stays
  `2.1.3` until a 2.2 MCP probe is retained as well (README "Tested matrix").
  Evidence: `TestSupportedHostVersion` (`AHI-245` case: 2.2.0, 2.2.1 and 2.2.999 admitted; 2.3.0,
  2.10.0, prereleases, leading zeros and 2.0.17 refused); the `AHI-245` range test in
  `integrations/opencode/evidence.test.mjs`. Live `qualify-opencode --native-only` on the loopback
  provider (no paid model), darwin/arm64, adapter 0.7.32: OpenCode 2.2.0 (executable sha256
  `0614b30fd17c27258b16cd6ea9ab03e1160fa9897e4cefd2d9a5d86f71a0af11`) `PASS` with 27 of 27 native
  checks true, and OpenCode 2.1.3 `PASS` 27 of 27 on the same build, recorded in
  `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-opencode-2-2-range.md`. Not run: full (recording) qualification, so no
  2.2 qualification record is produced; the MCP probe on 2.2; the inspector UI campaign fails with
  `native UI witness missing: sidebar` identically on 2.1.3 and 2.2.0, so OpenTUI 0.7 rendering of
  the TUI plugin is `NOT_OBSERVED`.
  OpenCode 2.2 changes, each checked against the 2.2.0 executable's embedded source or the live run:
  (1) plugin `ctx.session.instructions.entry.put/list/remove` exists in the 2.2.0 plugin context
  (only the HTTP client had it in 2.1.3); entries are durable session state announced at the next
  step boundary, which conflicts with `AHI-032`'s rule that repository data stays out of persisted
  input, and their delivery role is `NOT_OBSERVED`, so the prompt frame (`AHI-210`, `AHI-215`) keeps
  its per-request system text. (2) With `execute` denied, 2.2.0's tool snapshot exposes every
  permitted tool directly, Code Mode tools included (2.1.3 drops them); no adapter change: with
  `execute` allowed 2.2.0 still lists only `codemode: false` tools directly, so `corvint_context`
  keeps `options.codemode: false`, and the `AHI-214` guard stays because the allowed `execute`
  path is unchanged; a live execute-denied run is `NOT_RUN`. (3) Plugin-ready agent lists: the
  adapter has no agent-list retry, and the `AHI-218` `data` unwrap is still needed for 2.1.x; the
  live 2.2.0 run passes Code Mode under the stock build agent. (4) Event filtering is adopted by
  `AHI-246`. (5) Reload-before-step: `extend` retries only a thrown reload and has no race
  handling to remove. (6) The OpenTUI peer range `>=0.5.12` already admits 0.7.
  Failure modes: a 2.2 point release that changes a plugin API passes admission but fails its own
  campaign and stays `UNQUALIFIED`. Rollback: restore `<2.2.0` and the `0`/`1` minor test in the
  three locations; 2.2.x then fails closed as unsupported.
- `AHI-246`: (experimental under the standing lane auto-accept (2026-10-27); beamfall/corvint#1936)
  The OpenCode adapter MUST subscribe with `ctx.event.subscribe({ signal, types })`, where `types`
  is exactly the event types it handles (`session.compaction.ended`, `session.created`,
  `session.deleted`, `session.execution.failed`, `session.execution.succeeded`, `session.idle`).
  OpenCode 2.2.0's plugin event fan-out drops other types (and passes `server.connected`); 2.0.18 to
  2.1.x ignore the key and send everything, so the handler keeps its own type dispatch and no version
  gate is needed. Acceptance evidence: `AHI-246 the OpenCode event subscription asks only for the
  event types the adapter handles` in `integrations/host-adapters.test.mjs`, run by
  `TestHostAdapterJavaScriptHosts`; the live 2.2.0 and 2.1.3 native campaigns above pass
  `native-compaction`, `bounded-stops` and the session checks. Failure mode: a handler added
  without its type is never called on 2.2; the list is derived from the handler table to prevent
  that. Rollback: drop `types` from the subscribe call.
- `AHI-248`: (experimental under the standing lane auto-accept (2026-10-11); V1-2972) A well-formed
  OpenCode user-prompt envelope that holds an exact copy of the task MUST be withheld as the
  abstention `prompt-echo-withheld`, recorded at info level, and MUST NOT be reported as
  `incompatible-corvint-output` or warn `prompt-context-unavailable`. A one-word task such as `Add`
  can equal a repository symbol name that Core rightly returns in `context.results[].name`, so the
  echo is not a Core contract fault. The echo guard is unchanged: no frame is injected. Acceptance
  evidence: `AHI-248 a one-word prompt equal to a returned symbol name is withheld as an
  abstention` in `integrations/host-adapters.test.mjs`, run by `TestHostAdapterJavaScriptHosts`
  with the real binary; it fails at base with `incompatible-corvint-output`. Failure mode: a
  malformed envelope that also echoes the task still reports `incompatible-corvint-output`, because
  envelope validation runs first. Rollback: move the echo check back into `validEnvelope` and drop
  the `prompt-echo-withheld` branch in `integrations/opencode/src/index.js`.
- `AHI-175`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2346) Full OpenCode
  qualification (`tools/qualify-opencode` without `--native-only`) MUST admit exactly the one Go
  toolchain decision 0557 pins, `go1.27.3` (amended by decision 0557; decision 0547 admitted
  `go1.27.2` and `go1.27.3`), and refuse any other reported version, including the former
  `go1.27.2` pin and a development build, before running the focused suite. Evidence:
  `TestAdmittedGoVersion`. Failure mode: a go1.27.2-only host now refuses, and a future pin is
  refused until this list and its decision change together. Rollback: revert the decision 0557
  change, which restores the 0547 two-revision set.
- `AHI-176`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2346) At plugin
  start (since 0.7.26, on the first `corvint_context` call; `AHI-212`) the OpenCode adapter MUST register its bundled `skills/corvint/SKILL.md` through OpenCode's
  native skill registry (`ctx.skill.transform`, Skill.Info `{id, name, description, path,
  content}`), so the host lists `corvint` in the model's available skills without a copy step. The
  bundled file is read with a bound of 16,384 bytes and MUST parse exactly: LF-only frontmatter
  with exactly the unique `name: corvint` and `description:` lines, and a non-empty body; anything
  else is unreadable and is not registered. A skill already registered under the same id (project,
  user or earlier plugin) is project authority and MUST NOT be replaced. A host without a skill
  registry is tolerated silently. An unreadable skill reports `bundled-skill-unreadable` and a
  throwing registry reports `bundled-skill-registration-failed`; neither stops the tools or hooks.
  The native campaign MUST add the `context` check `native-bundled-skill`, which passes only when
  an actual outgoing provider request lists `<id>corvint</id>` inside OpenCode's
  `<available_skills>` guidance of a system or developer message. Non-goals: registering commands
  or agents, rewriting the skill per repository, and permission changes (a denied skill stays
  hidden). Failure modes: a host that renames the registry or the guidance element makes the check
  fail and the tuple `UNQUALIFIED`; a project skill named `corvint` with older guidance wins.
  Evidence: `integrations/opencode/skill.test.mjs`, `TestBundledSkillAdvertised`, and the retained
  2.1.3 campaign, whose scratch repository holds no skill directory. Rollback: remove
  `src/skill.js`, its call in `src/index.js` and the `native-bundled-skill` check; the README copy
  step remains valid.
- `AHI-177`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0453) Every version
  in `integrations/opencode/package.json` `corvintIntegration.testedHostVersions` MUST name
  committed evidence under `conformance/opencode-native-v1/results/<version>-<os>-<arch>/`: a
  passing `opencode-native-integration/1` `native-report.json` for that exact host version with
  every one of the 27 mandatory checks present and true, and a passing `mcp-probe.json` (`opencode-mcp-live-probe/0`, written by
  `conformance/opencode-native-v1/mcp_probe.py`) from the same host digest in which stock OpenCode
  loaded `corvint-mcp` from `mcp.servers` and completed a `corvint_status` call. The
  `integrations/compatibility.json` host version MUST also retain a passing `native-status-probe.json`
  (`opencode-native-status-live-probe/0`, the same script in `native=` mode) from the same host
  digest, in which the native plugin `corvint_status` tool returned `integrationSupport` FULL with
  reason `exact-tuple-conformance-passed`. The probe refuses an existing output directory and runs
  OpenCode in its own process group, terminated on timeout, SIGINT and SIGTERM. The
  `integrations/compatibility.json` OpenCode `hostVersion` MUST be a tested version, and
  `hostVersionEvidenceAdapterVersion` MUST equal the adapter version of that version's native
  report. Tested versions are evidence pointers, not qualification records: `corvint_status` still
  reads only the local exact-tuple record. Evidence: `integrations/opencode/evidence.test.mjs`.
  Failure modes: evidence from an earlier adapter version, a different host image, or a missing
  probe fails the test, as does a report that omits any mandatory check. Rollback: empty `testedHostVersions` and restore the previous
  compatibility entry; the retained results stay as history.
- `AHI-178`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0453)
  `make host-adapter-test`, which CI runs for TypeScript/JavaScript changes, MUST run every
  `integrations/opencode/*.test.mjs` file with `node --test`, so the OpenCode workbench, cockpit,
  inspector, task-metric, presentation, skill and evidence tests gate changes. Evidence: the
  Makefile target and a local run (44 plus the AHI-177 tests passing). Failure mode: a test that
  needs a host-only module fails the target instead of being skipped. Rollback: remove the line.
- `AHI-179`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2346) The OpenCode
  interruption-cleanup evidence MUST retain the full descendant observation beside the other
  interruption evidence as `<output>.observation.json` and embed only a fixed-size summary (`scope`,
  `interval_ms`, `absent`, `failures`, `limitations`, `processCount`, `processesFile`,
  `processesSHA256`), so the native report and the qualification record stay under the
  131,072-byte consumer bound however long the host takes to become ready. Failure mode: on a
  loaded host the 20 ms observation grew to 665 processes and the full-mode record was refused
  with `qualification record exceeds consumer bound`; a summary whose digest does not match the
  retained file is not evidence. `integrations/opencode/evidence.test.mjs` MUST verify each
  committed report's sidecar: the file exists in the evidence directory, its SHA-256 equals
  `processesSHA256`, and the summary's process count, `absent`, `scope`, `interval_ms` and
  `failures` agree with it; a missing, corrupted or zero-digest sidecar fails. Evidence:
  `TestCommandInterruption`, the evidence test's negative cases, and the live 2.1.3 full-mode
  campaign. Rollback: embed the observation again and remove the file write.
- `AHI-070`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2062) The generic
  `corvint harness event --event user-prompt` keeps refusing a task whose raw input, counted before
  surrounding whitespace is trimmed, exceeds either query bound (2,000 characters or 16,384 bytes)
  with `invalid-harness-input`, and reports those raw counts; only an accepted task is trimmed. It
  does not truncate, because a
  silently truncated query would rank context for text the caller did not ask about and would break
  frozen-kernel parity. The refusal message MUST name the observed character and byte counts and
  both bounds, and MUST NOT echo task text, so a direct caller (a script passing ticket text) can
  bound its own query or apply the disclosed anchor derivation of `AHI-016`. Failure mode: a caller
  that ignores the message still gets no ranked context. Evidence:
  `TestAHI070OverBoundTaskRefusalNamesLimitAndLength` in `internal/gokernel`. Rollback: restore the
  count-free message.


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
  - Elapsed time is logged, not asserted, because the parallel matrix runs under load (decision
    0082). The watchdog bound itself stays under AHI-017. How long a Git child outlives the adapter
    is asserted under `AHI-048`.
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
  instead of the full projection. The Codex plugin registers no subagent signal (Codex 0.162 has
  `SubagentStart`/`SubagentStop` events, left unregistered by `AHI-208`), so every Codex
  SessionStart projects in full.
  Failure modes: a host that omits `agent_id` in a subagent gets the main-thread packet (the former
  cost, not a loss); a host that sets it on the main thread loses the guidance until the next
  SessionStart without it. `agent_id` semantics are read from the Claude Code 2.1.267 binary's
  hook schema strings only; a live subagent run is NOT_OBSERVED.
  Acceptance (`AHI-045` to `AHI-047`): the tests in the traceability table and the before/after
  replay in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-07-hook-context-and-dogfood-summary.md`.
  Rollback (`AHI-045` to `AHI-047`): revert `cmd/corvint/host_adapter_projection.go` and its call
  sites; adapters again inject the whole receipt with guidance on every context event. No store,
  ledger or wire format changes, so either side reads the other's evidence.
- `AHI-230`: (experimental under the standing lane auto-accept; V1-2754; amends `AHI-047`) The
  Claude Code trusted workflow argv guidance MUST appear only when the hook root is the Corvint
  repository (its `go.mod` first line is `module github.com/Beamfall/corvint`) or its private
  worktree Git directory already holds `corvint/local-completion` (a dogfood enrollment exists).
  Any other repository gets the context projection with no guidance, so a foreign repository pays
  no per-session overhead for a workflow it never enrolled in. An unreadable `go.mod` or `.git`
  marker counts as not applicable. Non-goals: changing the projection, the Codex adapter, a new
  enrollment marker or setting, and the Claude Code versus OpenCode UserPromptSubmit selection
  difference of the V1-2754 audit, which stays an open measurement (NOT_OBSERVED here). Failure
  mode: a Corvint-using repository that has not yet enrolled loses the hint until it runs
  `corvint dogfood begin` or the skills say so; an unenrolled fork of Corvint with a renamed
  module loses it. Rollback: revert `dogfoodWorkflowApplies` and its call in
  `claudeSessionGuidance`. Amendment 2026-10-28 (V1-3022): the module line is read from a regular,
  non-symlinked `go.mod` only, opened without blocking, and only when the first line ends within
  4,096 bytes; a FIFO, device, symlink or over-long first line counts as not applicable, so the
  hook never blocks or buffers an unbounded line on it.
- `AHI-048`: (proposed; V1-0734) When an adapter process exits through `exitProcess`, including
  `os.Exit` after the `AHI-017` watchdog or a dogfood event deadline (`LCP-V0-008`) abandoned a
  read, it MUST first SIGKILL, synchronously, every child process group it started whose leader
  has not yet exited. Exec's context cancellation runs asynchronously and `os.Exit` used to win
  that race, leaving the Git child and its descendants running for seconds (3.7-4.8 s observed in the codex Stop and
  claude-code session-end slow-Git cases). `internal/groupreap` keeps a process-wide record. A
  Setpgid child started through `StartLive` or `Drain` is recorded from its start until `Wait`,
  `Drain`, or an `Owner`, releases it; `internal/contextindex` starts and reaps through `Drain`
  (`PGO-V0-008`), which keeps the group recorded through its output-pipe drain and pre-reap sweep.
  That release happens while the exited leader is still unreaped. `KillLive` signals
  every recorded group under an exclusive gate that starts and releases share. So it never
  signals a group whose leader was reaped, never misses a started one, and refuses a start after
  it. The Git spawns of `internal/gokernel`, `internal/contextindex` and `internal/cem/gitrun`,
  and the `xcrun --find git` lookup of `internal/gitstatus`, start through `StartLive` (`contextindex`
  through `Drain`); in an owned
  worker, the lookup stays in the worker's group, as its Git does. Groups are recorded only on
  Darwin and Linux, where `waitid` leaves the exited leader unreaped. Elsewhere `Wait` reaps
  before it could release, so nothing is recorded. The scratch removal of `AHI-044` follows the
  kill.
  Acceptance: `TestAHI048ExitProcessKillsLiveChildGroups` runs `exitProcess` in a helper process
  holding a live recorded group and requires its leader and a sleeping member gone afterwards.
  `TestAHI044HookAdaptersFailOpen` asserts, for every slow-Git case, that every Git child the
  shims recorded is gone less than 100 ms after the adapter exits, across `-count=5`. Other cases
  log that time only: a Git child that is running, not sleeping, can take longer to act on the
  SIGKILL under host load (decision 0082). The `TestAHI048*` tests in `internal/groupreap` cover
  the kill, the start refusal after it, release before reap, and concurrent starts and releases
  against one kill under `-race`.
  Non-goals: no daemon, no shell, and no change to hook exit status or stdout. A child in an owned
  worker's inherited group stays the enclosing runner's to retire. A host `SIGKILL` of the adapter
  still bypasses all exit cleanup (decision 0430).
  Failure modes: a child started with plain `command.Start` instead of `StartLive` is not
  recorded. A failed exit observation (ECHILD after a foreign reaper) releases the record at
  once, so such a group is not killed at exit. A start that loses the race to the exit kill fails with `groupreap.ErrExiting`, which
  callers report as a Git start failure.
  Rollback: revert `internal/groupreap/live.go`, the `StartLive` call sites and the `KillLive`
  call in `exitProcess`. Children again outlive an exiting adapter until their own deadline.
- `AHI-049`: (accepted by decision 0477, amended by decision 0479; V1-1040, V1-1055) Every Setpgid or Setsid spawner reachable from a point where an
  adapter abandons running work and exits (the `AHI-017` watchdog's `recoveredHostAdapterOutput`,
  the `LCP-V0-008` `dogfoodEvent` deadline, and `piAdapterResult`) MUST record the group it leads
  for the `AHI-048` exit kill, or be documented as unreachable from those points. The inventory
  is the table in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-09-v1-1040-adapter-exit-procgroup-registry.md` and MUST
  be redone when a new spawner or a new abandon point is added. `internal/procgroup` `Run`, which
  `captureSourceHandoff` reaches through `sourceGit`, records its own group with
  `groupreap.StartLive` and releases it with `groupreap.ReleaseLive` after its last pre-reap group
  signal and before it allows the reap, so the exit kill stays one pre-reap group signal and no
  numeric group signal follows the reap. A failed exit observation (for example ECHILD after
  another reaper) releases the record at once, and `Run` then sends no group signal or probe
  and reports the cleanup as unproved. (V1-1055) `Run` proves the group's quiescence before that
  release, while the exited leader is held unreaped and its PID pins the group ID, with
  `groupreap.HeldGroupQuiet` (the `PGO-V0-006` Linux /proc proof, or Darwin signal 0 confirmed by a
  zombie-only process table); after the release no numeric group operation, signal 0 included,
  targets the group ID. Every numeric group signal, probe and proof of `Run` is decided under the
  same lock as the release, so none follows it on any path, including the waitid goroutine's
  release after a foreign reap. Such reaping stays unsupported (`AHI-050`): between the foreign reap
  and that release the group ID is not pinned. An exit not observed by the shutdown deadline, a
  proof failure, or a live member at the deadline reports the cleanup as unproved. A `JoinAncestorProcessGroup` run leads no
  group and is not recorded; its observed exit is its quiescence. In an owned worker, the `xcrun --find git` lookup of `internal/gitstatus` stays in the
  worker's group (`AHI-048`).
  Acceptance: `TestAHI049ExitProcessKillsProcgroupGroup` runs `exitProcess` in a helper process
  while a `procgroup.Run` child and its sleeping member are live, and requires both gone; it fails
  before this change. `TestAHI049RunReleasesGroupBeforeReap` requires `Run` to release the record
  exactly once while the leader is still unreaped.
  `TestAHI049FailedExitObservationSendsNoGroupSignal` requires release before `BeforeStop`, no
  group signal, and unproved cleanup after an injected ECHILD. `TestAHI048LookupKeepsOwnedWorkerGroup` covers
  the owned-worker lookup. `TestV11055NoNumericGroupProbeAfterReap` requires no numeric group
  operation after the release while a killed sleeping member is proved gone; it fails before
  V1-1055. `TestV11055QuiescenceProvedWhileLeaderHeld` requires every proof while the leader is
  unreaped and a failed proof to leave the cleanup unproved.
  `TestV11055NoGroupOperationAfterConcurrentRelease` reaps the leader from another goroutine during
  timeout cleanup and requires no numeric group operation after the resulting release, with the
  cleanup unproved; it fails on the first V1-1055 revision.
  Non-goals: other binaries (`corvint-go-test-provider`, `tools/*`, `conformance/*`, interop
  runners), which never run `exitProcess`; spawners linked into `corvint` but unreachable from an
  abandon point (`workContain`, `doccompiler`, `taskman` reads, `liveverify/mutate`,
  `groupreap.RunContainedRetiring`) are listed and left unrecorded. The native hook's host
  watchdog is a host `SIGKILL`, not an abandon point.
  Failure modes: the inventory comes from a static VTA call graph with the string-dispatched
  `runContext` edges narrowed by hand, so a dynamic dispatch it misses can hide a reachable
  spawner. A `Run` start refused after the exit kill reports the process start failure with
  `groupreap.ErrExiting`. The quiescence proof of `procgroup` runs before the reap (V1-1055). An observation that fails after `Run`'s post-`BeforeStop` poll can
  still meet that run's own termination signal.
  Rollback: revert the `StartLive` and `ReleaseLive` calls in `internal/procgroup` and
  `groupreap.ReleaseLive`; `procgroup` children again outlive an exiting adapter until their own
  timeout.
- `AHI-050`: (accepted by decision 0474; V1-0371) The OpenCode and Gemini adapters MUST send a numeric process-group
  signal to their owned group only while the group's leader is unreaped. For Node, an unrecorded
  `exitCode` and `signalCode` is that proof only when read outside libuv's reap batch: libuv reaps
  every exited child before it dispatches any exit callback, so inside another child's exit callback,
  or a tick or microtask between two of them, a reaped leader still reads as unrecorded. The adapter
  therefore decides every group signal in a later `setImmediate` turn, after the poll phase has
  dispatched the whole batch, and rereads the exit fields there. On a timeout or a cancellation it
  sends SIGTERM to the group in that turn and holds the event loop for its termination grace
  (`Atomics.wait`; OpenCode 25 ms, Gemini 10 ms), so the leader cannot be reaped between that SIGTERM
  and the group SIGKILL that follows. OpenCode terminations requested before that turn share one hold.
  A delivered
  pre-reap SIGKILL confirms cleanup, since a SIGKILL cannot be caught. After the leader is reaped the
  adapter MUST NOT signal the group: a `kill(-pid, 0)` probe, which delivers nothing, confirms cleanup
  only on `ESRCH`. A normal leader exit sends no signal. A same-group descendant still alive at close
  completes as the visible degradation `corvint-process-cleanup-unconfirmed`, never as the receipt,
  and is not killed. Completion after a termination does not wait for close: OpenCode completes 100 ms
  after the request and Gemini one grace after its group decision, so a descendant holding an inherited
  pipe cannot hold the adapter; Gemini then closes its pipe ends and unreferences the leader, so a
  leader that survived an unconfirmed cleanup cannot hold the hook process either. Any other signal or probe answer, including Darwin `EPERM`, completes as that
  degradation instead of the timeout, cancellation or receipt. The host deadline and the `FALLBACK`
  shapes are unchanged.
  This supersedes the earlier `AHI-009` text that required a SIGKILL before completing a normal
  leader exit and counted `EPERM` after the leader's exit as confirmation: after a normal exit the
  leader is already reaped, so that SIGKILL could reach a reused group ID (decision 0474).
  Acceptance: the eleven `V1-0371` tests in `integrations/host-adapters.test.mjs`, under
  `TestHostAdapterJavaScriptHosts`, fail on the base and pass after. Six install a spy that records
  any non-zero group signal sent after the leader's `exit` was emitted, and require none. For each host
  they require that a timeout and a cancellation leave the TERM-ignoring `orphan-hang` descendant gone,
  with the unchanged `deadlineMs`, `FALLBACK` and exit codes. A normal exit leaving the `orphan-valid`
  descendant alive must report `corvint-process-cleanup-unconfirmed`, and an injected failure of the
  group SIGKILL must name that degradation instead of the timeout. For each host, a sibling child and
  the leader are blocked into zombies so one libuv batch reaps both, and the termination is requested
  from the sibling's exit callback: `ps` must show the leader reaped with its exit unrecorded, and no
  non-zero signal may reach its group. A Gemini leader that exits at once while a same-group descendant
  keeps its stdout open must complete within the deadline as `corvint-process-cleanup-unconfirmed`,
  and a Gemini hook whose TERM-ignoring leader survives a failed group SIGKILL must report that
  degradation and exit within its deadline.
  Sixteen OpenCode runners cancelled together must block the event loop for one grace, not sixteen.
  Non-goals: no shell or sentinel wrapper, no immediate SIGKILL that skips the Corvint SIGTERM
  cleanup, and no change to the Pi or Go process owners. Descendants that left the owned group are not
  covered.
  Failure modes: the event loop is blocked for one grace per termination turn. Sixteen OpenCode
  cancellations in one turn measured 25.5 to 30.4 ms; cancellations in separate turns each block for
  25 ms. A foreign in-process `waitpid(-1)`, or a runtime that reaps off the event-loop thread or
  outside the poll phase (Bun's behaviour inside OpenCode was not observed), could reap the leader
  early. Such reaping is outside this requirement's supported conditions and voids its signalling
  proof: a group signal may then reach a reused ID, and a delivered SIGKILL still confirms cleanup
  without a probe, so neither safety nor an honest result is guaranteed there. A killed
  member that init has not yet reaped also reads as unconfirmed.
  Rollback: revert the `requestGroupTermination`, `terminateGroup`, `signalGroup`, `holdUnreaped` and
  `cleanupConfirmed` changes in
  `integrations/opencode/src/runtime.js` and `integrations/gemini-cli/hooks/corvint-hook.mjs`, and
  their tests. The adapters then again send a group SIGKILL after the leader is reaped.
- `AHI-051`: (proposed; V1-1070, V1-1071) The Claude Code plugin MUST present itself with
  complete host-visible metadata and explicit read-only commands, without adding routine
  user-visible notices. Its manifest and marketplace entry carry `displayName`, a description
  that names what Corvint provides, `author`, `homepage`, `repository` and the `LICENSING.md`
  license, and its `metadata.support` stays `FALLBACK`. Every registered hook declares a
  `statusMessage` that states its user-visible action in plain words. Besides `context`, the plugin
  ships the skills `impact`, `affected`, `review`, `status` and `index`. Each maps to one existing
  verb (`impact`, `affected`, `review`, `dogfood status` with `observations` and `--version`,
  `index --if-stale`), keeps that verb's uncertainty, omissions and refusals in its report, and
  pre-approves only that verb and read-only `git` or `ls` commands through `allowed-tools`.
  `status` and `index` set `disable-model-invocation`, so only the user starts them. Of these five,
  `index` is the only one that writes, and only the derived snapshot under the Git common
  directory; `context` keeps its existing enrolled-workflow writes.
  `status` never invents a session key (`LCP-V0-011`). Decision 0161 is unchanged: hooks add no
  `systemMessage` for routine receipts, so `/corvint:status` is the on-demand user view, and a
  persistent status line or pane remains V1-1054.
  Non-goals: no MCP declaration (V1-0191; amended by `AHI-198`), no new verb, no change to hook output or the
  `corvint-dogfood-event/0` receipt, and no Codex skill parity.
  Failure modes: a host older than skill `allowed-tools` or `disable-model-invocation` support
  prompts for permission or lets the model start `status`/`index`; both remain read-only apart from
  the derived snapshot. A skill report is model prose over the verb's output, not a receipt.
  Acceptance: `claude plugin validate` passes for the plugin and marketplace,
  `integrations/host-adapters.test.mjs` checks the metadata, status messages and skill frontmatter,
  and `conformance/host-lifecycle-v1` discovery reports the six skills on the installed host.
  Rollback: restore the 0.2.3 manifest, marketplace and `hooks.json` and delete the five skill
  directories under a new plugin version; no store, ledger or wire format changes.
- `AHI-190`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0403) `corvint adapter
  done-list [--root DIR] [--base FULL_SHA] [--mutate]` MUST print one `corvint-done-list/0` receipt
  inside the `AHI-004` untrusted-data envelope, naming what the current change still owes:
  `affected` (state, scope, selected-unit count, at most 8 units with their test counts, unknown
  count and at most 8 reasons, dirty count, tier summary), `witness` (the `corvint witness` summary
  and at most 8 non-`CLOSED` obligation paths) and `mutation` (one entry per test file the change
  adds). Each part that did not run is `NOT_RUN` (or `NOT_APPLICABLE`) with a stable reason. The
  receipt carries `mutates: false`; the command writes nothing in the repository and runs the
  existing verbs in process. The plugin ships a user-only `/corvint:done [BASE]` skill
  (`disable-model-invocation`, because `--mutate` executes test code) that pre-approves only
  `corvint adapter done-list` and `git rev-parse`; this amends the `AHI-051` skill set.
- `AHI-191`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0403) The Claude Code
  `Stop` hook `corvint adapter claude-code stop-done-list` MUST emit `{}` unless the hook
  environment sets `CORVINT_STOP_DONE_LIST` to exactly `1` (settings `env`); the done-list is opt-in
  and off by default.
- `AHI-192`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0403) Opted in, the
  hook MUST also emit `{}` on a continued Stop (`stop_hook_active`), inside a subagent, and when the
  tree is clean (no base, no dirty path, no new test). Otherwise it MUST emit exactly one
  `systemMessage` and nothing else: on `Stop`, `additionalContext` or a block decision would continue
  the turn (`AHI-021`, decision 0161), so the done-list is shown to the user only and never reaches the
  model. The opt-in is the user's request for this one notice; routine receipts still add none.
- `AHI-193`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0403) The message MUST
  be `Corvint done-list:` and three lines (tests, unwitnessed ranges, new-test mutation), at most
  2,000 bytes with a ` [truncated]` suffix when cut, with every repository-derived value passed
  through the `AHI-004` hidden-character escape and control characters removed, so a path cannot
  add lines or hidden text.
- `AHI-194`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0403) Mutation budget
  and the hook timeout: the `stop-done-list` hook is registered with a 10-second host kill (the
  `AHI-017` table entry `claude-code stop-done-list`, deadline 9.6 s). `corvint prove --mutate
  --attack-tests` runs every selected test once per mutant, minutes on a real package, so the hook
  MUST NOT run witness or mutation: it runs only `affected` against the working tree, reports
  witness as `NOT_RUN witness-requires-committed-range` and mutation as `NOT_RUN
  mutation-not-run-from-hook`, each with the exact argv to run after committing (`corvint --root R
  witness --base HEAD` and `corvint adapter done-list --root R --base HEAD --mutate`, with `HEAD`
  resolved to its full SHA). Past the deadline it fails open under `AHI-044`. The explicit command
  has no hook deadline and runs mutation only with `--base` and `--mutate`
  (`mutation-not-requested` otherwise).
- `AHI-195`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0403) The explicit
  command MUST exit 2 with `invalid-arguments` for an unknown flag, a flag without its value, a base
  that is not a full 40- or 64-hex object name, and `--mutate` without `--base`; with
  `project-root-unavailable` when the root cannot be resolved; and with `not-a-git-repository`
  outside a repository. Each refusal is a `corvint-done-list/0` receipt with `ok: false`. A receipt
  that cannot be encoded is replaced by the refusal `encoding-failed` (exit 1).
- `AHI-196`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0403) With `--base B`
  the receipt describes `B..HEAD` plus the dirty paths `affected` sees: `affected --base B`,
  `witness --base B --json`, and the new tests are the files the range adds (`git diff
  --diff-filter=A B HEAD`) whose names follow the test conventions (`_test.go`, `test_*`, `_test.py`,
  `_spec.rb`, `.test.*`, `.spec.*`). Without a base the new tests are untracked or added paths in
  the isolated `gitstatus` status on private metadata, the read `affected` uses, so no
  repository-defined clean or process filter runs; a refused status leaves mutation `NOT_RUN`
  `new-tests-unavailable`. A verb that fails or passes the deadline is `NOT_RUN` with
  `<verb>-failed:<code>` or `deadline`. The receipt is bound to the commit HEAD names before any
  verb runs and names it as `head`: when HEAD names another commit after the last verb, the
  receipt is replaced by an `ok: false` refusal `head-moved` (exit 1). Either HEAD probe must name a
  full commit identifier; a failed, unborn or malformed probe is the refusal `head-unavailable`
  (exit 1), never a match between two failures. The Stop hook reports only the refusal.
- `AHI-197`: (accepted under the owner's standing auto-accept (2026-10-11); V1-0403) With `--base`
  and `--mutate`, each new Go test file MUST carry the `prove --mutate --attack-tests` row for that
  file: `mutants`, `killed`, `survived` and a verdict, `KILLS_MUTANTS` when it kills at least one,
  `KILLS_NO_MUTANT` when mutants executed (killed or survived) and it killed none (a test that
  asserts nothing), and `NOT_RUN` with `no-mutants`, `no-mutant-executed` (every mutant was
  uncompilable or skipped), `mutation-unsupported-language` (non-Go tests), `no-affected-test-row`
  or the prove status otherwise. A change that adds no test file reports mutation as
  `NOT_APPLICABLE` `no-new-tests`. The tree MUST be byte-identical afterwards.
  Non-goals (`AHI-190` to `AHI-197`): no mutation in any hook; no `additionalContext` or blocking
  Stop; no new mutation engine or language; no witness of an uncommitted tree; no change to
  `corvint-dogfood-event/0` or to the existing Stop completion hook.
  Failure modes: a large dirty tree can push `affected` past the 9.6 s deadline, and the hook then
  degrades visibly and silently drops the list; a test file added under an unconventional name is
  not judged; a mutation verdict covers only the mutants `prove` generates for `B..HEAD`, so
  `KILLS_MUTANTS` is not proof the test is adequate.
  Acceptance: `cmd/corvint/host_adapter_done_list_test.go`:
  `TestAHI191StopDoneListOptInSilenceAndShape`, `TestAHI193DoneListMessageBoundedAndPlain`,
  `TestAHI195DoneListCommandRefusals`, `TestAHI197DoneListJudgesNewTestsByMutation` (a tautological
  new test reports `KILLS_NO_MUTANT`, an asserting one `KILLS_MUTANTS`, tree unchanged, and
  `mutation-not-requested` without `--mutate`); `TestAHI017AdapterHostKillMatchesDeclaredHooks`
  for the 10 s registration; `TestAHI044HookAdaptersFailOpen` for the deadline path; the `AHI-051`
  skill test in `integrations/host-adapters.test.mjs` for `/corvint:done`.
  Rollback: remove the second `Stop` hook from `hooks.json`, the `done` skill, the `done-list` verb
  branch, the `stop-done-list` event and table entry, and `cmd/corvint/host_adapter_done_list.go`
  with its test, under a new plugin version; no store, ledger or wire format changes.
- `AHI-198`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2466, GitHub #1588;
  amends the `AHI-051` non-goal) The Claude Code plugin MUST declare the read-only `corvint-mcp`
  stdio server in `plugins/corvint/.mcp.json` as server `corvint`, command `corvint-mcp`, args
  `--root ${CLAUDE_PROJECT_DIR} --protocol-version 2025-11-25`, and nowhere else, so `corvint.help`
  and the other `corvint.*` tools are reachable in-session. The legacy protocol is the
  `initialize` handshake Claude Code speaks; the server's default `2026-07-28` mode answers
  `initialize` with method-not-found. `compatibility.json` records `surfaces.mcp: FALLBACK` and the
  verified host version. Non-goals: no bundled binary, no install step, no official MCP conformance
  claim. Failure modes: `corvint-mcp` absent from `PATH` shows the server as failed in `/mcp` and
  `claude mcp list` (`ENOENT`) while hooks and skills keep working; a project root that is not a Git
  repository makes the server refuse its tools. Acceptance: `AHI-198 Claude Code plugin declares
  the read-only corvint-mcp server at the project root` in `integrations/host-adapters.test.mjs`;
  the live `claude mcp list` health check on Claude Code 2.1.293 (`✔ Connected`) and a stdio
  `corvint.help` call recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-claude-code-integration-matrix.md`.
  Rollback: delete `.mcp.json` and restore `surfaces.mcp: UNAVAILABLE` under a new plugin version.
- `AHI-199`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2440) Heavy repository
  targets MUST run their children below interactive priority so hook adapters keep their CPU share
  under lane test load: `script/heavy.sh` runs its command at `HEAVY_NICE` (default 10, `0` keeps
  the caller's priority, outside 0..19 refused with `REFUSE invalid-nice`, never raising a caller
  already nicer), and the Makefile prefixes `nice -n $(HEAVY_NICE)` to the `ledger/%` gate steps,
  `GO_TEST_COMMAND`, `dogfood-change` and `dogfood-check` when `nice(1)` exists. The `go-test` and
  `ledger/go-test` recipes stay the exact lines `STR-V0-006` pins, so a bare `make gate` runs its
  Go test step at the caller's priority; `script/heavy.sh make gate` (the lane wrapper) lowers it.
  Non-goals: no change to slot counts, `-p`, the hook timeouts or the `STR-V0-006` pins. Failure modes: priority does not help when the starved resource is memory or I/O, and a
  load that saturates every core can still slow a hook. Acceptance: the `AHI-199` group in
  `script/lane-scripts_test.sh` (default 10, `HEAVY_NICE=0`, refusal, Makefile dry runs); 50
  consecutive `corvint adapter claude-code user-prompt` runs under 3 and 6 concurrent package test
  runs, timings in the matrix build-log entry. Rollback: drop `HEAVY_NICE` from `script/heavy.sh`
  and `HEAVY_NICE_PREFIX` from the Makefile.
- `AHI-052`: (accepted, decision 0497; V1-1116; amends `AHI-050` for OpenCode only) When
  the OpenCode adapter's cleanup probe `kill(-pid, 0)` answers `EPERM`, the adapter MUST repeat that
  signal-0 probe, each repeat a timer at least 5 ms after the last, until it answers anything other
  than `EPERM` or 40 repeats have run, and MUST NOT send any other signal to the group meanwhile. Only
  `ESRCH` confirms cleanup; an `EPERM` on the last repeat, or any other answer, completes as
  `corvint-process-cleanup-unconfirmed`, as in `AHI-050`.
  Darwin answers `EPERM` to every signal, signal 0 included, for a group whose members are all
  unreaped zombies. Node reaps the leader only in libuv's poll phase, and libuv runs due timers right
  after the check phase, so a termination turn that ends after the 100 ms completion timer is due (a
  stalled host) probes while the leader that SIGTERM killed is still a zombie; an orphaned member
  stays one until launchd reaps it. For OpenCode this replaces `AHI-050`'s "OpenCode completes 100 ms
  after the request" with "100 ms after the request, plus the `EPERM` repeats (40 at 5 ms, about
  200 ms on an unloaded host; a stalled event loop stretches them)", and its "Any other signal or probe answer, including Darwin `EPERM`, completes as
  that degradation" with the bounded re-probe above.
  Acceptance: `V1-1116 OpenCode completion after a stalled termination turn re-probes the zombie
  group` (Darwin only) holds the termination grace until the leader is a zombie and 150 ms have
  passed, so the completion timer fires before the leader is reaped; it requires the group SIGKILL and
  the first probe to answer `EPERM` and a later probe `ESRCH`. On the base it failed 5 of 5 with a single
  `EPERM` probe, the hosted failure, and it passes after. `V1-1116 OpenCode EPERM re-probe confirms
  only on ESRCH and is bounded` injects the probe answers on any platform: three `EPERM`s then `ESRCH`
  confirm after four probes, a persistent `EPERM` completes as the degradation after exactly 41
  probes, and no non-zero group signal follows the SIGKILL. A hosted macos-15 run passing
  `host-adapter-test` and `TestHostAdapterJavaScriptHosts`, with its run id retained, is still required.
  Non-goals: no change to Gemini, which keeps `AHI-050` as accepted; no change to which answers
  confirm, to the group signals, the SIGTERM grace or the 100 ms timer; no wait for close.
  Failure modes: a group holding a live member of another user answers `EPERM` throughout, so it
  completes after all 40 repeats and still unconfirmed. A zombie that is not reaped within the repeats
  (an init that reaps an orphan slower than that) still reads as unconfirmed. The repeat timers are
  referenced, so they hold the host process until the repeats end.
  Rollback: restore the single `groupGone` probe in `integrations/opencode/src/runtime.js` in place of
  `confirmCleanup`, `probeGroup` and `groupProbe`, and remove the two tests. OpenCode again completes
  100 ms after the request and reads a zombie-only group as unconfirmed.
- `AHI-058`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1122; amends `AHI-050`
  for Gemini, as `AHI-052` does for OpenCode) When the Gemini hook's cleanup probe `kill(-pid, 0)`
  answers `EPERM`, the hook MUST repeat that signal-0 probe, each repeat a timer at least 5 ms after the
  last, until it answers anything other than `EPERM` or 40 repeats have run, and MUST NOT send any other
  signal to the group meanwhile. Only `ESRCH` confirms cleanup; an `EPERM` on the last repeat, or any
  other answer, completes as `corvint-process-cleanup-unconfirmed`, as in `AHI-050`. The Gemini
  termination grace (10 ms) and its completion bound are unchanged; a confirmed cleanup now reports after
  the repeats (about 200 ms at most on an unloaded host), inside the hook's 400 ms process reserve; a stalled event loop stretches them.
  Acceptance: `V1-1122 Gemini EPERM re-probe confirms only on ESRCH and is bounded` delivers the group
  SIGKILL through a preload and answers it as Darwin's `EPERM`, then injects the probe answers: three
  `EPERM`s then `ESRCH` report the timeout cause after four probes, a persistent `EPERM` reports the
  degradation after exactly 41 probes, and no non-zero group signal follows the SIGKILL. On the base
  the single probe reported the degradation for the first case. Non-goals: no change to which answers
  confirm or to the group signals. Failure modes: as `AHI-052` (a live member of another user, or a
  zombie not reaped within the repeats, stays unconfirmed). Rollback: restore the single probe in
  `cleanupConfirmed` in `integrations/gemini-cli/hooks/corvint-hook.mjs` and remove the test.

- `AHI-054`: (accepted, decision 0519 (V1-1157); GitHub #785) The
  `corvint-harness-event/0` `stop` receipt's `frontier` object gains an optional member, `cause`,
  beside its unchanged `reason`, `shouldContinue` and `state`. On the wire the member is optional: a
  `/0` stop receipt without it, as every Core before this change emits, stays conforming, and
  consumers MUST accept its absence. This Core MUST emit it on every `stop` receipt, with the value
  `no-accepted-closing-authority`. `reason` keeps naming the degradation code,
  `frontier-authority-unavailable`; `cause` says why it holds: no separately accepted closing
  authority exists. The value is fixed. The event never
  consults a CEM, an OCM or `corvint frontier`, so it MUST NOT name a cause it did not observe, such as
  a missing or unverified CEM, and a committed `.corvint/change.cem.json` leaves the value unchanged.
  The member is additive: the profile stays `corvint-harness-event/0` (the `TOL-V0-001` additive
  precedent), the `frontier-authority-unavailable` degradation stays in every receipt, and the receipt
  basis, and so `receiptId`, still excludes the frontier. Consumers MUST tolerate the member, and,
  because the degradation is expected rather than a fault (`AHI-021`, `AHI-022`), MUST NOT render it as
  a user-visible notice. The frozen `harness-stop` parity row records the extra member as `DR-0043`.
  Non-goals: no adopter acknowledge, suppress or hide flow for frontier items; the owner declined it
  on 2026-10-11 (V1-1157, decision 0519), because missing evidence stays visible (product invariant 2, `AHI-019`,
  `AHI-022`) and the degradation remains in every receipt. No cause on the non-`stop` events, on
  `corvint-dogfood-event/0`, on the Pi workflow profile or on the qualified lifecycle; no
  documentation-link member; no change to `shouldContinue`, `state` or the degradation list.
  Failure modes: a strict consumer that compares the whole frontier object to a fixed value rejects
  the new member; the shipped Gemini CLI and OpenCode consumers read only `state` and
  `shouldContinue`. A consumer that renders every frontier member would surface an expected
  degradation, which `AHI-021`/`AHI-022` forbid.
  Acceptance: `internal/gokernel/harness_test.go::TestAHI054StopFrontierCarriesFixedCause` (exact
  frontier with and without a committed CEM, degradation kept, no frontier on the other events);
  `conformance/cli-parity-v0` `TestGPKV0002ManifestReplay` reporting
  `PASS-WITH-KNOWN-DIVERGENCE harness-stop register=DR-0043 clause=AHI-054 rewrites=1` and
  `TestAHI054FrontierCauseDivergenceIsPinnedAndOutOfPacket`; the Gemini CLI and OpenCode stop cases
  in `integrations/host-adapters.test.mjs` under `TestHostAdapterJavaScriptHosts`, whose fixture
  core emits the member, and `AHI-054 stop receipts with and without the frontier cause are accepted
  and the cause is never rendered` there (fixture modes `valid` and `legacy-frontier`).
  Rollback: remove the `cause` member and `FrontierCauseNoClosingAuthority` from
  `internal/gokernel/harness.go`, the fixture member and `legacy-frontier` mode and its test, the
  `DR-0043` declaration, validator and runner branch, and the register entry; the frozen row passes
  again unchanged. No store, ledger or receipt identity changes.

### Codex CLI 0.162 surfaces (2026-10-23)

Live receipts for every requirement below are in
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-codex-integration-matrix.md` (Codex CLI 0.162.0, model `gpt-6-astra`,
isolated `CODEX_HOME`, branch-built binaries).

- `AHI-206`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2326) The Codex
  package MUST ship `integrations/codex/profiles/corvint-review.config.toml`, a Codex profile with
  exactly `sandbox_mode = "read-only"`, `approval_policy = "never"` and
  `shell_environment_policy.set.CORVINT_TASKS_READ_ONLY = "1"`, and the Codex README MUST document
  the review invocation `codex exec -p corvint-review` with the same three keys as `-c` overrides.
  Codex 0.162 layers a trusted project `.codex/config.toml` above the profile and only `-c`
  overrides above that, so the documented run can neither escalate out of the sandbox nor mutate
  the Tasks store through `corvint-tasks` (`ATR-V0-212`) whatever the project config says.
  Failure modes: a write that bypasses the `corvint-tasks` CLI is not stopped; a run that selects
  the profile without the overrides inherits a trusted project's sandbox and variable (observed
  live: `danger-full-access` and `0`).
  Rollback: delete the profile; reviews fall back to the documented flag recipe.
- `AHI-207`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2326) The Codex
  plugin MUST declare exactly one MCP server, `corvint`, through `"mcpServers": "./.mcp.json"`. It
  runs `corvint-mcp --protocol-version 2025-11-25 --root R` from `PATH`, where `R` is the Git
  top level of the directory Codex launches it in, and exits 2 with a fixed stderr line, without
  starting `corvint-mcp`, when that directory is not inside a Git worktree. The server stays
  read-only (`MCPV0`), and the compatibility surface is `FALLBACK`.
  Failure modes: a missing `corvint-mcp` or a non-worktree launch leaves the session without Corvint
  tools (Codex continues); the root is bound once, when Codex starts the server in the session
  working directory, and no later notice detects a switch, so after moving to another worktree
  the client must start a new Codex session there to rebind (each receipt names its bound root
  and revision).
  Rollback: remove `.mcp.json` and the manifest key and bump the package.
- `AHI-208`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2326) Codex 0.162
  exposes `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `PermissionRequest`,
  `PreCompact`, `PostCompact`, `SubagentStart`, `SubagentStop`, `Stop` and `SessionEnd`. The plugin
  MUST register only `SessionStart`, `UserPromptSubmit`, `Stop` and `SessionEnd`, each as the
  bounded `corvint adapter codex` command hook (timeout at most 2 s), with at least one handler
  per event and the `SessionStart` matcher `startup|resume|clear|compact|fork`; every other event
  stays unregistered. If one is delivered anyway, the adapter abstains silently: it returns the
  non-blocking `{}` (`AHI-044`) and records `unsupported-hook-event` in the local self-observation
  ledger (`SOL-V0-010`), with no host-visible response.
  Failure modes: compaction pins, subagent context and permission guards stay unavailable on Codex
  until they are mapped under their own requirements.
  Rollback: none needed; the registration is unchanged from adapter 0.2.9.
- `AHI-209`: (accepted under the owner's standing auto-accept (2026-10-11); CODEX-NATIVE-APPROVAL-RELAY-VISIBILITY-1)
  The Codex skill and README MUST state that a human approval reaches Codex automatic approval review only as a root-thread user message, that the
  approved action must be requested exactly (no rewording or retargeting), that approval review is
  not a Corvint gate, and that each decision is recorded as a `guardian_review` session under
  `$CODEX_HOME/sessions/`, not in `codex exec --json`. Observed on 0.162: a subagent's review
  receives the root user messages; the exact approved ref update was allowed
  (`user_authorization: high`), the same update retargeted by an untrusted file was denied, and an
  unapproved low-risk write was allowed with `user_authorization: low`.
  Failure modes: the reviewer is a model and its risk threshold is not Corvint-controlled; Codex
  Desktop and `codex queue` grants are NOT_RUN.
  Rollback: remove the guidance paragraphs.
- `AHI-210`: (accepted 2026-10-25, standing auto-accept; V1-2717) Prompt orientation, not the packet.
  The OpenCode adapter's awaited `session.prompt` hook (adapter 0.7.22) MUST supply through the
  model-request context hook a compact orientation frame instead of the full ranked packet: the
  receipt id, the query `state`, at most eight distinct confident evidence paths in packet order,
  the count of omitted paths, and any receipt degradations, inside the untrusted-data envelope and
  preceded by a fixed plugin-authored line that names `corvint_context` as the tool that returns the
  ranked packet and cv1 handles. The encoded orientation object MUST stay within 1024 UTF-8 bytes:
  trailing paths are dropped and counted in `omittedPaths` until it fits, so eight long or
  non-ASCII paths cannot grow the frame past the compact size the trial measured. The frame keeps
  every `AHI-032` property (unchanged prompt text,
  hidden delivery, deduplication, generation invalidation, compaction clearing, the 8000-byte bound,
  marker and receipt id present for `hiddenPromptDelivery`; for the candidate frame the receipt
  link satisfies this, `AHI-219`). The `corvint_context` description MUST
  say when to call it (a task that names behaviour but not files; orientation paths not yet read)
  and that it abstains and never proves completion. Each of the four plugin tools MUST declare
  `options.codemode: false`: OpenCode 2.1 lists only such tools as directly callable and exposes
  every other plugin tool solely through its code-mode `execute` tool (the V1-0415 and V1-2717 smoke
  sessions reported the Corvint tools callable only through `execute`). The ranked packet, expansion
  handles and inspector bookkeeping are unchanged for the tool call; the hook and the tool share one
  bounded query path. Measured first-step prompt overhead and tool adoption are recorded per paired trial in
  the build log; a trial result is evidence, never a promotion.
  Failure modes: a model may still never call the tool, in which case the trial stays negative and
  is reported so; orientation paths are repository-authored names and stay inside the envelope.
  Rollback: restore the previous `onPrompt` body (full `corvint_context` content as the frame) and
  the previous tool description; the frame shape is not a wire contract.
  From adapter 0.7.23 the frame shape, the empty-frame case, the per-tool Code Mode clause and the
  tool-description guidance are replaced by `AHI-211`; every other clause above still holds.
- `AHI-211`: (accepted 2026-10-26, standing auto-accept; V1-2717) Explained candidates, small standing surface.
  The OpenCode adapter (0.7.23) MUST replace the `AHI-210` orientation frame with a candidate frame:
  for a `READY` packet whose abstention is not active and that ranks at least one result with an
  evidence path, a fixed plugin-authored line saying the candidates are revision-pinned and
  receipt-linked, not proof, and naming `corvint_context` for the full ranked packet with cv1
  handles, followed by any `AHI-016` disclosure and, inside the untrusted-data envelope, the receipt
  id, the state, at most five distinct candidates in packet order (path, positive line, symbol name,
  and the evidence authority, confidence and reason that explain the row), the count of omitted
  candidates, the packet's coverage uncertainty strings and any receipt degradations. When every
  shown candidate has the same authority, confidence and reason, those are carried once as `why`
  instead of per row. The encoded candidate object MUST stay within 1024 UTF-8 bytes as delivered, after the envelope
  escapes hidden characters: trailing
  candidates are dropped and counted in `omittedCandidates` until it fits. Any other packet (not
  `READY`, active abstention, or no result with a path) MUST inject no frame: the response is still
  recorded by the inspector, the prompt is still deduplicated and the stop step still armed, and the
  adapter records `prompt-context-withheld` at info level, so nothing is shown that the packet did not
  support. Only `corvint_context` MUST declare `options.codemode: false`; `corvint_expand`,
  `corvint_record_outcome` and `corvint_status` MUST stay in OpenCode's Code Mode catalog (reached
  through `execute`), the `corvint_context` description is at most three short sentences, and the other three tool
  descriptions and the bundled skill description stay one sentence each, so the plugin's standing first-step prompt cost stays at or below 500 tokens by the
  smoke attribution method of the V1-2717 build log (same host, model, charter and repository, plugin
  present minus plugin absent). Measured on 2026-10-26: 0.7.22 +1085 tokens, frozen 0.7.23 +479
  before and after the paired rerun (`benchmarks/results/v2717-opencode-adoption-paired-2026-10-26.json`,
  which records the rerun's cost criterion as not met). The
  candidate frame keeps every `AHI-032` property `AHI-210` lists, and the qualification's scripted
  provider calls `corvint_context` directly.
  Failure modes: the bound sits close to the measured cost, so host or model prompt changes can move
  it past 500 (re-measure per host release); candidates are syntax-ranked and can be wrong, so the
  frame says they are not proof and carries the packet's own uncertainty; a model may still never
  call a tool, which the paired trial reports.
  Rollback: restore the 0.7.22 `orientationFrame`, the per-tool `options.codemode: false` and the
  longer descriptions; the frame shape is not a wire contract. The frame's abstention, shape and
  bound are amended by `AHI-215` (0.7.29).
- `AHI-212`: (accepted 2026-10-26, standing auto-accept; V1-2717) Near-zero idle surface with
  lazy follow-ups. The OpenCode adapter (0.7.26) MUST register only `corvint_context` (direct,
  `options.codemode: false`, a one-sentence description of at most 80 characters) at setup.
  `corvint_expand`, `corvint_record_outcome`, `corvint_status` (Code Mode, as `AHI-211`) and the
  bundled skill (`AHI-176`, same project-owned precedence) MUST be added only on the first
  `corvint_context` call in the plugin instance, through a transform gated on that call and one
  `tool.reload()`; OpenCode re-runs the gated transform and appends its catalog and skill-change
  notices to the conversation, so the cached prompt prefix is not rewritten. Later calls MUST NOT
  reload again. The `AHI-211` prompt injection is unchanged: a confident packet still injects the
  candidate frame, any other packet injects nothing, and neither widens the tool surface. With
  nothing injected, the plugin's first-step prompt cost MUST stay at or below 150 tokens by the
  `AHI-211` smoke attribution method. The scripted qualification provider calls `corvint_context`
  before it reaches the Code Mode tools, and `native-bundled-skill` accepts the corvint skill in
  either OpenCode's available-skills guidance (system or developer message) or its appended
  skill-change notice (`New skills are available in addition to those previously listed:`, which
  the host may deliver in a user-role message).
  Acceptance evidence: `AHI-212` in `integrations/host-adapters.test.mjs` (idle surface is exactly
  `corvint_context`; a confident packet is injected and an abstaining one is not, without widening
  the surface; the first call reloads once and adds the three Code Mode tools and the skill; a
  second call does not reload), the lazy-skill case of the `AHI-176` setup test in
  `integrations/opencode/skill.test.mjs`, `TestBundledSkillAdvertised`, and the 2026-10-26 live
  smoke on OpenCode 2.1.3 with `claude-opus-5-5`: plugin absent 6926 and present 7023 first-step
  prompt tokens (+97), and an unlock session in which the agent called `tools.corvint_status` through
  `execute` after `corvint_context` and saw the corvint skill listed
  (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-26-v2717-opencode-autocontext-rerun.md`).
  Non-goals: no change to what is injected or when; no new tool; no Code Mode routing for
  `corvint_context`; no claim that lower idle cost improves outcomes, which the build log's paired
  rerun reports separately.
  Failure modes: a host without `tool.reload` keeps the follow-ups unavailable after the first call
  (the call still answers; the adapter reports `tool-extension-failed` only when reload throws, and
  a thrown reload leaves the surface idle and is retried by the next `corvint_context` call); an
  agent that never calls `corvint_context` never sees the follow-ups or the skill, which is the
  intended idle trade; a host that delivers the skill notice in another role fails
  `native-bundled-skill` until this check is extended; the 150-token bound moves with host and model
  prompt changes, so re-measure per host release.
  Rollback: restore the 0.7.25 single eager transform and eager skill registration and the
  three-sentence `corvint_context` description; restore the system/developer-only skill check.
- `AHI-213`: (experimental under the standing lane auto-accept, 2026-10-11; V1-2804; GitHub #1900)
  Exact expansion handle from a tool result, and named failure reasons. Since `AHI-210` the prompt
  hook no longer carries the full packet, so the scripted qualification provider found no `cv1`
  handle for `add.go` and sent `corvint_expand` a placeholder; `native-exact-expansion` failed on
  every try (regression from d75d90654, reaching main in #1857). The scripted provider in
  `internal/opencodequalification/provider.go` MUST issue, after its governance query, a second
  `corvint_context` call for the campaign task (`Locate Add in add.go.`), and MUST take the expansion
  handle only from an earlier tool result in the request. When no tool result carries a `cv1`
  handle for `add.go`, it MUST NOT call `corvint_expand` with a placeholder. Instead it MUST
  record the provider fault `context-packet-carried-no-expansion-handle` for
  `native-exact-expansion` and continue the script. The native `report.json` MUST carry
  `failures`, a list sorted by check name of `{check, reason, detail}` for every false check, and
  `[]` on PASS. A provider fault overrides that check's default reason. `latency` reports
  `LATENCY_REGRESSION` or `LATENCY_HOST_CONTENDED` with every attempt's measured values, and an
  unmapped check reports `check-failed`. The campaign error MUST name every failed check with its
  reason. The `native-exact-expansion` predicate is unchanged: it still requires the exact lines
  1-3 selection for the pinned handle.
  The interruption observation summary (`cleanupEvidence.observation.failures`) and its retained
  sidecar MUST encode no failures as `[]`, never `null`. An observation decoded from evidence that
  retained `null` still decodes and is re-encoded as `[]`.
  Acceptance evidence: `TestAHI213ScriptedSourceQuerySuppliesExpansionHandle` (the source query
  result supplies the `1-3` handle and no placeholder is sent; a missing handle is a typed fault
  with no expand call; benchmark and smoke scripts are unchanged) and
  `TestAHI213EveryFalseCheckNamesAReason` (all 27 mandatory checks get a reason; PASS encodes
  `[]`), `TestAHI213ObservationFailuresEncodeEmptyArray` (a retained `null` decodes and encodes
  `[]` in sidecar and summary; recorded failures are kept), plus the updated `AHI-056` failure-message case. The live campaign is NOT_RUN for this
  change, because it needs a paid model and network.
  Non-goals: no change to the adapter's prompt injection or tool surface; no weakened or skipped
  check; no restoration of handles to the hidden prompt frame.
  Failure modes: if the source query stops ranking `add.go` (for example, through a ranking change),
  `native-exact-expansion` fails with `context-packet-carried-no-expansion-handle` rather than an
  unexplained false; any OpenCode request that does not answer the script (title or summary
  generation) shifts the script by one step exactly as before; a new check without a reason entry
  reports `check-failed` until it is mapped.
  Rollback: remove the second scripted query, the provider fault and `failures`, and restore the
  placeholder handle. `failures` is additive, so older consumers ignore it.
- `AHI-214`: (experimental under the standing lane auto-accept, 2026-10-27; V1-2753) Code Mode
  network refusal. OpenCode 2.1 adds its Code Mode `execute` tool whenever permission `execute` is
  not denied, and the JavaScript it runs has a global `fetch` that no `webfetch`/`websearch` rule
  governs: in the V1-2717 Pilot A p07 session the build agent, with both network tools denied,
  fetched `proxy.golang.org` through `execute`. The OpenCode adapter (0.7.28) MUST register a
  `tool.hook("execute.before")` guard that, for a call whose `tool` is `execute`, reads the effective
  permission rules as the host composes them (the calling agent's `permissions` from
  `ctx.agent.get`, then the session's `permissions` from `ctx.session.get`, each read from the
  response's `data` object when it has one, since stock OpenCode 2.1.3 answers `ctx.agent.get` with
  `{location, data}` (`AHI-218`); last matching rule wins,
  host wildcard semantics) and MUST throw, failing the call before its program runs, unless both
  `webfetch` and `websearch` are allowed: the last matching rule with resource `*` allows the
  action and no later matching rule asks or denies for any resource. It MUST fail closed: a missing
  `ctx.agent` or `ctx.session.get`, a missing session or agent id, an unknown agent, a throwing
  read, or a malformed rule list or rule refuses the call. A refusal reports the info notice
  `code-mode-network-denied` and returns a fixed message naming the direct tools and
  `permission.execute: "deny"`. Calls of any other tool, including inner calls an allowed program
  makes, are left to the host. Hosts and trial profiles that deny `webfetch`/`websearch` SHOULD also
  set `permission.execute: "deny"`, which removes Code Mode before the model sees it; this guard is
  the plugin-side backstop when they do not.
  Acceptance evidence: `AHI-214 V1-2753 Code Mode execute is refused when the host denies webfetch
  or websearch (p07)` in `integrations/host-adapters.test.mjs` (the p07 rules refuse and a simulated
  `fetch` never runs; all-allow, ask-then-allow and the observed stock 2.1.3 build-agent rules, in
  the host's wrapped and in an unwrapped response, keep Code Mode; ask, a narrower later deny,
  a session-level deny and no matching rule refuse; a missing agent API, a throwing agent read,
  an unknown or missing agent id, a malformed effect and a malformed session rule list refuse; a
  non-`execute` call passes; the notice is recorded), run by `TestHostAdapterJavaScriptHosts`; the
  loopback native qualification (`AHI-218`) runs Code Mode under the stock build agent and passes.
  Live OpenCode qualification of the refusal is `NOT_RUN` (it needs a paid model session).
  Non-goals: no per-URL enforcement inside Code Mode (a narrower rule refuses instead); no removal
  of `fetch` from the sandbox, which the plugin cannot reach; no change to the direct
  `corvint_context` tool or to injected context; no change to the Corvint Tasks OpenCode dispatcher's
  permission profile.
  Failure modes: a refused session also loses `corvint_expand`, `corvint_record_outcome` and
  `corvint_status`, which live only in Code Mode (`AHI-212`), the same trade as
  `permission.execute: "deny"`; a host that changes `execute.before` so a thrown error no longer
  fails the call, renames the network actions, or changes rule composition makes the guard
  ineffective or over-refusing until it is re-derived from that host build; a host that adds another
  network-capable tool is not covered; a plugin opened outside a Git repository registers no hooks
  (decision 0378), so the guard does not run there.
  Rollback: remove the `execute.before` registration and `src/code-mode-guard.js`, and return the
  adapter to 0.7.27.
- `AHI-215`: (experimental under the standing lane auto-accept, 2026-10-27; V1-2755) Prompt frame
  abstains by default. The 2026-10-26 value audit found the `AHI-211` frame cost-neutral at best:
  in Pilot A every shown row was a syntax match, agents reached a gold file within their first steps
  without it, and most of its about 1.8 KB was envelope, receipt and disclosure. The OpenCode adapter
  (0.7.29) amends `AHI-211` as follows; its tool surface, deduplication, inspector, stop arming and
  every `AHI-032` property are unchanged.
  1. Abstention. The prompt hook MUST inject no frame, and MUST record one info notice, when:
     the packet is not `READY`, Core's abstention is active (no anchor resolves) or no row has an
     evidence path (`prompt-context-withheld`); no shown candidate's evidence authority is one of
     `project-instructions`, `instruction-reference`, `repository-spec` or `git-history`, so the
     rows carry only syntax-class authority, an unknown authority counting as syntax-class
     (`prompt-context-syntax-only`); or the prompt already names a repository-relative path that
     exists in the checkout as a file or directory (`prompt-names-path`; a leading `./` or `/` is
     read relative to the repository root, `..` segments and tokens over 512 characters are never
     checked, and at most 256 tokens are checked). The query still runs, so the inspector and the
     stop step see the packet as before; `AHI-247` (0.7.33) skips the query for `prompt-names-path`.
  2. Shape. An emitted frame MUST be a fixed plugin line saying the candidates are not proof and
     naming `corvint_context`, then for an over-bound prompt one trusted line giving the kept and
     dropped anchor counts in place of the `AHI-016` disclosure, then the untrusted-data envelope
     (its first two lines, no free-text field list) around
     `{"receipt":L,"candidates":[...],"omittedCandidates":N}`, where `L` is the query's receipt link
     (`AHI-218`, `AHI-219`). A candidate carries only path, positive line, symbol name and evidence authority: no
     state, reason, confidence, uncertainty or degradations (all remain in `corvint_context`).
  3. Rows. Candidates are in descending packet score (stable; unscored rows after scored ones), a
     symbol name declared by rows in two or more distinct non-test files is demoted behind every
     other row, and only the first row per file is kept; at most five are shown.
  4. Bound. The whole frame text as delivered, after hidden-character escaping, MUST be at most 600
     UTF-8 bytes (200 tokens at a conservative 3 bytes per token); trailing candidates are dropped
     and counted until it fits, and a frame whose first candidate cannot fit is withheld
     (`prompt-context-withheld`), never cut.
  5. Junk anchors. On the prompt hook only, an over-bound derived query (`AHI-016`) MUST drop words
     with no letter or digit, words starting with `/`, and markup tag names the prompt closes
     (`</name>`) that do not also occur outside tags; it never drops every word. The
     `corvint_context` tool and every other host keep the byte-identical `AHI-016` query.
  Acceptance evidence: `AHI-210 AHI-211 AHI-215 prompt hook abstains by default and delivers
  compact candidates, not the packet` in `integrations/host-adapters.test.mjs`, run by
  `TestHostAdapterJavaScriptHosts` (real-binary syntax-only packet withheld; a corroborated row
  framed within 600 bytes with its receipt link and no reason, uncertainty or degradations; syntax, vocabulary
  and unknown authorities withheld but inspected and stop-armed; named existing paths withheld and
  missing, `..` or non-path tokens not; empty, pathless, not-`READY` and abstained packets withheld;
  score order, distinct files and same-name demotion; rows past the bound counted; escaped bytes counted; an
  unfittable first row withheld; over-bound junk anchors dropped on the hook and kept by the tool).
  Offline replay on the ten V2717 Pilot A tasks (local `corvint` index of each task checkout, the
  plugin driven by the Pilot A frame probe, no model): 0.7.26 emitted 8 frames, 14,259 bytes in all;
  0.7.29 emits 0 and abstains on all 10 (8 `prompt-context-syntax-only`, 2 `prompt-context-withheld`
  for `BUDGETED` packets); 5 of the 10 prompts also name an existing path. Recorded in
  `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-v2755-prompt-frame-abstains.md`.
  Non-goals: no change to retrieval, ranking or Core abstention; no repository-wide declaration
  lookup for demotion; no change to other hosts, to `corvint_context`, or to the `AHI-016` twins
  and their conformance goldens; no model-tokenizer measurement.
  Failure modes: same-name demotion sees only the packet's rows, not every declaration in the
  repository; the 600-byte bound estimates tokens, and a tokenizer that spends more than 3 bytes
  per token could exceed 200 (`NOT_OBSERVED`); a within-bound prompt's junk words reach Core
  unchanged; a prompt that names a path in prose that is not meant as a target still abstains, which
  costs nothing the audit could measure; with today's packets the frame almost never emits, so a
  later corroborating authority is what makes it visible, and the paired effect of that is unmeasured.
  Rollback: restore the 0.7.28 `candidateFrame` and `collectTaskContext`, remove
  `src/frame-policy.js`, and return the adapter to 0.7.28; the frame shape is not a wire contract.
- `AHI-247`: (experimental under the standing lane auto-accept, 2026-10-28; V1-2944) Decide a
  packet-independent abstention before the query. `AHI-215` withheld the frame for a prompt that
  names an existing checkout path but still ran the `user-prompt` query, so hook CPU and latency
  were paid for a packet nobody saw. The OpenCode adapter (0.7.33) amends `AHI-215` item 1:
  1. When the prompt names an existing repository-relative path (the `AHI-215` rule, unchanged),
     the prompt hook MUST decide `prompt-names-path` before any Corvint process starts and MUST run
     no `user-prompt` query for that prompt. It is decided before the busy check, so a path-naming
     prompt never reports `prompt-context-busy`.
  2. Everything else `AHI-215` keeps for a withheld prompt still holds: no frame is injected, one
     info notice `prompt-names-path` is recorded (it carries no receipt, since no query ran), the
     prompt is deduplicated by message id, and the stop step is re-armed. The inspector's latest
     view becomes `unavailable` naming `prompt-names-path` (as an over-bound prompt's does,
     `AHI-016`); the packet stays one `corvint_context` call or explicit inspector request away
     (`AHI-033`).
  3. The other abstentions cannot be decided before the query and keep it: not `READY`, active
     Core abstention, no row with an evidence path, syntax-only authorities, and a first row that
     does not fit 600 bytes all depend on the packet's rows. No cheap pre-query signal predicts
     them: `git-history` corroboration can come from any Git checkout, and `project-instructions`
     or `repository-spec` rows depend on Core ranking for this task, not on the files merely
     existing. An over-bound prompt's anchor refusal (`AHI-016`) was already pre-query.
  4. Abstention stays the default; the `corvint_context` tool, `corvint_expand`, session start,
     post-tool, stop and session-end events, and their governance and lifecycle evidence are
     unchanged.
  Acceptance evidence: `AHI-210 AHI-211 AHI-215 prompt hook abstains by default and delivers
  compact candidates, not the packet` in `integrations/host-adapters.test.mjs`, run by
  `TestHostAdapterJavaScriptHosts`: each of the four path-naming prompts records
  `prompt-names-path`, the fixture binary sees zero `user-prompt` invocations, the inspector view
  is `unavailable` naming the code, a repeated message id is deduplicated and `session.idle` still
  runs one stop; each non-path prompt still queries once. Measurement (darwin/arm64, real `corvint`
  build of this change on an indexed two-file fixture, prompt `fix Add in add.go`, 20 hook calls,
  wrapper-counted): adapter 0.7.32 ran 20 `user-prompt` queries, median hook time 109.30 ms,
  p95 119.07 ms, 2386.5 ms in all; 0.7.33 ran 0 queries, median 0.02 ms, p95 0.51 ms, 1.9 ms in
  all. Recorded in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-v2944-v2955-prompt-early-exit-live-dispatch.md`.
  Non-goals: no change to which prompts abstain, to retrieval, to the frame shape or to other hosts.
  Failure modes: a path created while a query is in flight is still caught by the post-query check
  in `candidateFrame`; a prompt naming a path in prose still abstains (as in `AHI-215`), now without
  a packet in the inspector until context is requested.
  Rollback: remove the pre-query branch in `onPrompt` and return the adapter to 0.7.32.
- `AHI-218`: (experimental under the standing lane auto-accept, 2026-10-27; V1-2804; GitHub #1900)
  The OpenCode native qualification MUST still exercise an emitted prompt frame after `AHI-215`.
  The `AHI-213` campaign prompted with its source task, `Locate Add in add.go.`, which names an
  existing path and ranks only syntax-authority rows, so the `AHI-215` hook withholds its frame and
  the `AHI-032` delivery checks (`awaited-current-prompt`, `bounded-prompt`, `timed-delivery`,
  `direct-install`, `critical-recall-and-bytes`) would fail with no frame to observe. `AHI-215` had
  also dropped the receipt id from the frame, which those checks bind the frame to.
  1. The campaign prompt (`framePrompt`) MUST name no fixture path and be governed by the fixture's
     `AGENTS.md`, so its packet carries a `project-instructions` row and the frame is emitted; it is
     also the first scripted `corvint_context` task. The second scripted query keeps `sourceTask`
     and supplies the expansion handle (`AHI-213`).
  2. An emitted frame MUST carry the query's `receiptId` (amending `AHI-215` item 2 within the
     unreleased adapter 0.7.29); the 600-byte bound includes it. `AHI-219` replaces the full id
     with its 28-byte receipt link.
  3. `critical-recall-and-bytes` counts recall of `Add` at `add.go:3` in any query packet, since
     the frame query orients and the source query recalls.
  4. The `AHI-214` guard MUST read the agent's rules from the `data` object that stock OpenCode
     2.1.3's `ctx.agent.get` returns (`{location, data}`); reading only a top-level `permissions`
     refused every Code Mode call on the stock host, `corvint_expand` included.
  5. A report with no latency samples MUST still be written: an unobserved p95 is `NOT_OBSERVED`,
     never the unencodable `+Inf` that lost the whole report (`json: unsupported value: +Inf`).
  Acceptance evidence: `TestAHI218CampaignPromptEmitsAFrame`
  (`internal/opencodequalification/provider_test.go`); the `AHI-215` JavaScript case asserting the
  frame's receipt id within 600 bytes, run by `TestHostAdapterJavaScriptHosts`. Live
  `qualify-opencode --native-only` on the loopback provider (OpenCode 2.1.3, darwin/arm64, no paid
  model): `PASS` with all 27 native checks true, recorded in
  `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-v2804-qualification-frame.md`.
  Non-goals: no change to the `AHI-215` abstention rules, to retrieval, or to which checks qualify.
  Failure modes: if Core stops ranking the fixture `AGENTS.md` for `framePrompt`, the frame is
  withheld again and the delivery checks fail with their named `AHI-213` reasons rather than
  passing; the receipt id costs about 100 of the 600 bytes, so at most four short candidates fit
  (three with symbol names) and the five-row cap of `AHI-215` item 3 no longer binds. The receipt is
  kept because `AHI-032` and `AHI-210` require a receipt-linked frame; since `AHI-215` frames rarely
  emit, the aggregate cost is small. `AHI-219` (V1-2870) delivers the shorter receipt link, so
  five short candidates fit again.
  Rollback: prompt with `sourceTask` again, drop `receiptId` from `candidateFrame`, restore the
  `queryPackets[0]`-only recall, read only a top-level `permissions`, and report `p95` directly.
- `AHI-219`: (experimental under the standing lane auto-accept, 2026-10-27; V1-2870) Short frame
  receipt link. The `AHI-218` receipt id (`harness-receipt:sha256:` and 64 hex digits, 87 bytes)
  left room for at most four short candidates in the 600-byte frame, so the `AHI-215` five-row cap
  never bound.
  1. An emitted prompt frame MUST name its query by the receipt link `cvr:` followed by the first 24
     hex digits of the receipt's SHA-256 (28 bytes, at most 32), in the payload key `receipt`. A
     response whose receipt id is not `harness-receipt:sha256:` and 64 lowercase hex digits gets no
     frame (`prompt-context-withheld`).
  2. `corvint_expand` MUST resolve a well-formed link only against the frames delivered in the
     calling session (the newest eight), returning that query's ranked packet with its full
     `receiptId` and cv1 expansion handles. An unknown link, or one from another session, refuses
     with `receipt-link-unknown`; a link two different receipts in the session share is kept as
     ambiguous and refuses with `receipt-link-ambiguous` rather than resolving to either. Other
     handles are expanded by Core as before.
  3. The native qualification binds delivery to the link: `hiddenPromptDelivery`, the timed-delivery
     observer and the `direct-install` check look for the query's link (or any well-formed link for
     the install check) instead of the full receipt id; this replaces, for the candidate frame, the
     `AHI-210` "receipt id present" binding.
  Acceptance evidence: `TestAHI219FrameReceiptLink` (`internal/opencodequalification/native_test.go`);
  the `AHI-215` JavaScript case (link at most 32 bytes; five short rows framed within 600 bytes with
  the link and the sixth counted; link expanded to the packet and full receipt in its session,
  refused in another session and when unknown; ambiguous and eight-link retention by
  `rememberFrameLink`), run by `TestHostAdapterJavaScriptHosts`. Live `qualify-opencode
  --native-only` on the loopback provider (OpenCode 2.1.3, no paid model): `PASS`, all 27 checks
  true, injected bytes 416 to 355 (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-v2870-frame-receipt-link.md`).
  Non-goals: no change to Core receipts, to the session-start, tool or compaction frames (which keep
  the full receipt id), to the candidate rules or to the 600-byte bound.
  Failure modes: 96 bits of the receipt digest are shown, so a collision within one session's eight
  links is negligible and refused if it happens; a link is not portable across sessions or adapter
  restarts (the session's links are in memory), so it refuses there and `corvint_context` remains
  the way to the packet. Rows with long paths or symbol names may still fit fewer than five.
  Rollback: put `receiptId` back in `candidateFrame`, drop the link branch of `corvint_expand`, and
  bind the qualification checks to the full receipt id again.
- `AHI-216`: (experimental under the standing lane auto-accept, 2026-10-27; V1-2762) The Claude
  Code hook projection (`AHI-045`) MUST NOT repeat governance the host already loads. Before a
  Claude Code projection is rendered, governance rows whose path is the root `CLAUDE.md`,
  `.claude/CLAUDE.md`, or a root-relative file those import with `@path` (outside code fences and
  code spans, resolved against the importing file, followed up to five hops and 64 files, files over
  256 KiB not read) are removed; a projection left with no row omits `governance`. Home, absolute,
  `..`-escaping and missing import targets are not host-loaded, so their rows stay. The engine
  receipt keeps every governance row and blob pin; only the model-facing repeat is removed. Codex
  and every other host are unchanged.
  Acceptance evidence: `TestAHI216ClaudeHostLoadedGovernanceIsNotRepeated` (import parsing, nested
  hop, fence/span/home/absolute/escape/missing exclusions, Codex unchanged, and a main-thread
  SessionStart rendered without the row and with its workflow argv). Offline re-projection of the
  200 most recent Corvint-repository Claude Code sessions with envelopes (this repository's
  `CLAUDE.md` is `@AGENTS.md`): 1,602 of 1,752 envelopes drop the `AGENTS.md` row; envelope bytes
  2,228,878 to 1,975,762 and median 844 to 686. Recorded in
  `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-v2762-hook-governance-dedupe.md`.
  Non-goals: subagent detection beyond `agent_id` (`AHI-047`); Codex `AGENTS.md` loading; any change
  to the receipt, the compaction block or the prompt resolution of `LCP-V0-039`.
  Failure modes: Claude Code 2.1.311 builds SessionStart hook input without the subagent's
  `agent_id` (its `source=compact` SessionStart inside a subagent carries the main thread's
  `session_id`, `transcript_path` and `agent_type`), so `AHI-047` cannot tell a subagent compaction
  from a main-thread one and the subagent still receives the workflow argv and compaction block; all
  495 subagent SessionStart envelopes in the sample were `source=compact`. V1-2860 compared the two
  payloads in the 2.1.311 bundle: a main-thread and a subagent compaction call the same SessionStart
  builder with the shared session, so `session_id`, `transcript_path`, `cwd`, `scratchpad_dir`,
  `prompt_id`, `agent_type` (the main-thread type), `source`, `session_title` and the hook
  environment are identical; `permission_mode` and `agent_id` are unset in both, and the subagent's
  agent context reaches only telemetry and output handling. Only `model` (the compacting thread's
  model) can differ, and it is not a discriminator: it is equal whenever the subagent shares the
  main model, and a main-thread model switch would make a parent compaction look like a subagent's
  and drop its rehydration. The missing host field is `agent_id` (with the subagent's own
  `agent_type`) on SessionStart `source=compact` input; no suppression ships until the host sets
  it, and `AHI-047` then applies unchanged. A captured hook stdin pair is `NOT_OBSERVED`; this
  lane's own subagent compaction received the main thread's session key and root
  (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-v2860-subagent-compaction-host-gap.md`). An `@path` the host resolves differently (for example a user-level import
  setting) leaves a row in place, which is the safe direction.
  Rollback: marshal the projection directly in `renderAdapterResult` again.
- `AHI-217`: (experimental under the standing lane auto-accept, 2026-10-27; V1-2756) Paired agent
  trials are analysed from per-session receipts, never from ad hoc arithmetic. `tools/paired-trial`
  MUST read `corvint-paired-trial-receipt/0` JSON Lines (task, replicate, arm `off|on`, resolved,
  cost, the injected frame's byte count and SHA-256 with the base tree it was probed on, the per-step
  ledger of tool, tokens and cost, and the grader's per-test outcomes), pair the arms by task and
  replicate, and report per-pair cost deltas and resolution flips, the pooled on/off cost ratio with
  a seeded percentile bootstrap interval over pairs, and the exact two-sided McNemar p-value on the
  discordant pairs. A receipt missing the frame, the frame's base tree, the step ledger or the grader
  tests MUST be counted as `NOT_OBSERVED` in the report, and `--require-complete` MUST refuse it. A
  step ledger that does not sum to the session cost and a task with one arm are named. Duplicate
  arms, another profile, a missing outcome, no pair, or a zero off-arm total are refused.
  Acceptance evidence: `TestAHI217PairedTrialAnalysis` (`tools/paired-trial/main_test.go`).
  Non-goals: running a trial or a model; changing the trial harness, which lives outside the
  repository and must emit these receipts; a shell-enabled harness mode.
  Failure modes: the harness that ran V2717 does not yet emit this receipt, so its runs can be
  analysed only after conversion and their frame, ledger and per-test fields stay `NOT_OBSERVED`;
  the bootstrap resamples pairs, so with ten pairs its interval is wide and its tails coarse.
  Rollback: remove `tools/paired-trial`; no other code reads its report.

## Native platform profiles

| Platform | Embedded host-admission key | Maintained Corvint package | Native surfaces | Stability rule |
|---|---|---|---|---|
| Codex CLI/Desktop | `codex` | Codex plugin | skills, hooks, MCP | test startup/resume/clear/compact separately; compaction recovery remains dirty-path-only |
| Codex IDE | `codex` (shared host; separate surface status) | standalone Codex integration | standalone skill, shared MCP, supported hooks | plugins are unavailable; never inherit CLI/Desktop status |
| Claude Code | `claude-code` | Claude Code plugin | skills, hooks, MCP | publish minimum/maximum tested plugin API versions; `PreCompact`/`PostCompact` pin hooks verified against 2.1.267 only, live cycle `NOT_RUN` |
| Gemini CLI | `gemini-cli` | Gemini CLI extension | context file, commands, skills, hooks, MCP | validate extension environment filtering and hook schemas |
| OpenCode | `opencode` | OpenCode 2 plugin (`@corvint/opencode` 0.3.0 and later; adapter 0.7.18 admits exact campaigns for `>=2.0.18 <2.2.0` (AHI-174), tested 2.1.3; adapter 0.7.32 admits `>=2.0.18 <2.3.0` (AHI-245; 2.2.0 native-only PASS, not qualified); adapter 0.7.22 delivers prompt orientation (AHI-210); adapter 0.7.23 delivers explained candidates with a smaller standing surface (AHI-211); adapter 0.7.26 registers only `corvint_context` until its first call (AHI-212); adapter 0.7.28 refuses Code Mode `execute` unless network tools are allowed (AHI-214); adapter 0.7.29 withholds the prompt frame unless it adds signal (AHI-215); adapter 0.7.33 runs no prompt query for a path-naming prompt (AHI-247); 1.x unsupported) | stable `session.created`/`session.execution.*`/`session.deleted` events, `tool.hook("execute.after")`, plugin tools, MCP `mcp.servers`; beta `session.hook("context")` isolated | legacy receipts remain `FALLBACK`; native integration qualification uses AHI-032 |
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

Accepted, decision 0517 (V1-1128; `AHI-053`): failed or disabled background refresh
retains the stale code and manual argv. A started notice is not completion; later child failures
remain stale on the next event. No daemon, synchronous hook build or authority is introduced.

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
- Failure mode (`AHI-057`): a degraded `pre-compact` emits no pin; when the summary keeps its
  unavailable line `post-compact` degrades `compaction-pin-not-emitted` instead.
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
| `corvint-output-too-large` | `cmd/corvint/host_adapter.go:1078@1b317e61` | adapter output cannot be marshaled, or with its final LF exceeds 8000 bytes; a degraded `systemMessage` naming this reason is written instead |
| `canonical-json-failed` | `internal/gokernel/harness.go:458` | "cannot encode receipt basis" |
| `compaction-block-unavailable` | `cmd/corvint/host_adapter_compaction.go:134@e26bd5d6` | Claude adapter: the compact `session-start` receipt carries no `context.compaction` block, or its revision is not a Git object ID |
| `compaction-pin-not-emitted` | `cmd/corvint/host_adapter_compaction.go:99@8de67696` | Claude adapter: the last pin-shaped line in `compact_summary` is the `corvint-compaction-pin/0 unavailable` line a degraded `pre-compact` emitted (`AHI-057`) |
| `compaction-pin-not-preserved` | `cmd/corvint/host_adapter_compaction.go:103@5fbc4775` | Claude adapter: `compact_summary` holds no pin line whose every field re-validates |
| `compaction-pin-revision-unavailable` | `cmd/corvint/host_adapter_compaction.go:110@81c5dd24` | Claude adapter: the object store reports the pinned tree as missing |
| `compaction-pin-verification-unavailable` | `cmd/corvint/host_adapter_compaction.go:244@1c06bb2b` | Claude adapter: the pin's `cat-file --batch-check` failed, timed out, or answered a different number of queries |
| `git-unavailable` | `cmd/corvint/host_adapter_compaction.go:231@1f1f42d0` | Claude adapter: no `git` executable is on `PATH` when `post-compact` verifies a pin |
| `harness-input-too-large` | `internal/gokernel/harness.go:361` | "harness input exceeds its byte limit" |
| `harness-output-too-large` | `internal/gokernel/harness.go:466` | "harness response exceeds its byte budget" |
| `invalid-compaction-trigger` | `cmd/corvint/host_adapter_compaction.go:77@3ccad220` | Claude adapter: the `PreCompact`/`PostCompact` `trigger` is not `manual` or `auto` |
| `invalid-harness-adapter` | `internal/gokernel/harness.go:73` | "invalid <label>" |
| `invalid-harness-budget` | `internal/gokernel/harness.go:378@b2bc66fa` | "harness budget must be at least <value> bytes" |
| `invalid-repository-root` | `internal/gokernel/harness.go:376` | "cannot resolve repository root" |
| `malformed-corvint-output` | `cmd/corvint/host_adapter.go:686@2c724e09` | Claude adapter: the `harness event` stdout is not JSON; the degraded `systemMessage` names this reason |
| `project-root-unavailable` | `cmd/corvint/host_adapter.go:337@2100b4c9` | Claude adapter: the project root (`CLAUDE_PROJECT_DIR`, else the working directory) cannot be made absolute; the degraded `systemMessage` names this reason |
| `qualification-in-progress` | `internal/opencodequalification/record.go:433` | AHI-032: the producer atomically writes the active record as `INCOMPLETE` after preserving any previous record; integration support stays `UNQUALIFIED` until complete passing evidence replaces it |
| `repository-identity-malformed` | `internal/gokernel/repository.go:178` | "Git object identity is malformed" |
| `repository-probe-cancelled` | `internal/gokernel/repository.go:167` | "Git repository probe was cancelled" |
| `repository-probe-timeout` | `internal/gokernel/repository.go:165` | "Git repository probe exceeded its 10-second deadline" |
| `repository-probe-too-large` | `internal/gokernel/repository.go:150` | "Git output exceeds its byte limit" |
| `repository-profile-malformed` | `internal/gokernel/repository.go:270` | "Git profile path is not valid UTF-8" |
| `repository-status-malformed` | `internal/gokernel/repository.go:215` | "Git status output is malformed" |
| `repository-status-too-large` | `internal/gokernel/repository.go:240` | "Git status exceeds the <value>-path limit" |
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

Accepted, decision 0517 (V1-1128; `AHI-053`): `CORVINT_INDEX_AUTO_REFRESH=0` restores
manual-only notices for new events. Revert the amendment to restore the prior implementation;
an already started child retains its finite bound. Owner acceptance and live qualification remain
separate from fixture evidence.

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

For AHI-001, `TestAHI001AdapterWireVersion` in `cmd/corvint/local_completion_event_test.go`
pins the literal `0.1.0`, both native plugin producers and local completion reader acceptance,
and rejection of package versions as wire versions (V1-0444).

| Requirement | Implementation surface | Required evidence |
|---|---|---|
| `AHI-001`, `003`, `005`, `014` | shared `corvint harness event` core and `internal/projectpath` | canonical receipt, bounds, privacy, revision, and event fixtures; `TestHostAdapterAbsentPathContainment` and `TestRelativeAliasesAndUncertainty` cover `AHI-014` path containment, and the `integrations/host-adapters.test.mjs` test `AHI-014 Gemini classifies changed paths on resolved symlinks like internal/projectpath` under `TestHostAdapterJavaScriptHosts` covers the Gemini hook's symlink resolution; `TestClaudeAdapterForkSessionStartIsResume` covers the Claude `fork` start source; `TestAHI003ClaudeCompactSessionStartRehydratesDirtyPaths` drives the Claude `SessionStart(source=compact)` hook entrypoint over a mixed dirty worktree and requires the tracked impact, the untracked count and `compaction-untracked-paths-not-rehydratable` from the receipt's own snapshot; `TestQualifiedLifecycleCompactSessionStartRehydratesDirtyPaths` requires the same for the qualified profile under FULL and FALLBACK and refuses a reordered, extra or dropped code; `TestAHI014EventExpectationsAreHostConsistent` (`conformance/harness-event-v0/host_schema_test.go`) pins each `common-logical-interaction.json` event's closed host set and requires every present host's golden `expected` object to be byte-identical, so a per-host field or host-membership mutation of that fixture fails here |
| `AHI-032` | `internal/opencodequalification`, `tools/qualify-opencode`, native prompt hook and qualification consumer | `internal/opencodequalification/record_test.go::TestRecordValidation`, `::TestAtomicRecord`, `::TestArchitecture`, `::TestProducerConsumer`; `internal/opencodequalification/command_test.go::TestInvalidHostInvalidatesQualification`; `internal/opencodequalification/witness_posix_test.go::TestGateInterruptionWitness`; first-prompt startup overlap, quiet-context busy/compaction regressions in `TestHostAdapterJavaScriptHosts`, and `internal/opencodequalification/native_test.go::TestAHI032HiddenPromptDelivery`; `internal/opencodequalification/bounded_read_test.go::TestEvidenceReadsRefuseOverBoundFiles` (16 MiB evidence bound before allocation; open rows accept unknown members); exact-tuple native campaign required |
| `AHI-033` | `integrations/opencode/src/inspector.js`, `src/tui.tsx`, and inspector RPC in `src/index.js` | `integrations/opencode/inspector.test.mjs`, AHI-033 cases under `TestHostAdapterJavaScriptHosts`, and `tools/qualify-opencode --inspector` (stock native rendering, pinned source, narrow keyboard use and interruption cleanup) |
| `AHI-034` | `integrations/opencode/src/cockpit.js`, `cockpit-tui.tsx`, `index.js`, `runtime.js` | `integrations/opencode/cockpit.test.mjs` and `tools/qualify-opencode --inspector`: bounded fixed reads, safe output paths, stale/owner/check-rerun races, advisory impact navigation, independent workflow/check state and real native change/proof workflow |
| `AHI-042` | `integrations/opencode/src/ui-presentation.js`, `cockpit-tui.tsx`, `tui.tsx`, `task-tui.tsx`, `workbench-tui.tsx`, `task-metrics.js` and `workbench.js` | `integrations/opencode/ui-presentation.test.mjs`, AHI-042 declared-gate cases in `task-metrics.test.mjs` / `workbench.test.mjs`, and the stock native inspector witness for bounded Work → Change → Evidence navigation and honest status/limit presentation |
| `AHI-043` | `cmd/corvint/host_adapter.go` Claude SessionStart/UserPromptSubmit output | `cmd/corvint/host_adapter_stability_test.go::TestAHI043ClaudeContextPacketsAreByteStableAcrossTime` (fake-clock 61 s and 25 h gaps, unchanged dirty fixture, full envelope required) |
| `AHI-044` | `cmd/corvint/host_exit.go` (`adapterStdout`, `hookStdout`, `exitProcess`), `cmd/corvint/signals_unix.go` `notifyBrokenPipe`, `internal/gitstatus/scratch.go`, `integrations/gemini-cli/hooks/corvint-hook.mjs` | `cmd/corvint/host_adapter_fail_open_test.go::TestAHI044HookAdaptersFailOpen` (every shipped Claude Code and Codex hook × seven faults: exit 0, named cause, spawn cap, no shell, no writes outside live ledgers), `internal/gitstatus/scratch_test.go` (`TestAHI044ScratchRemovedAtClose`, `TestAHI044ScratchCloseRacesReads`) and the AHI-044 Gemini case under `TestHostAdapterJavaScriptHosts` |
| `AHI-230` (experimental; V1-2754) | `cmd/corvint/host_adapter_dogfood_scope.go` (`dogfoodWorkflowApplies`, `privateGitDir`), `claudeSessionGuidance` | `cmd/corvint/host_adapter_projection_test.go`'s `TestAHI230GuidanceOnlyForCorvintOrEnrolledRepository`; V1-3022 (`goModFirstLine`): `TestDogfoodWorkflowGoModReadIsBounded` |
| `AHI-045`–`AHI-047` (accepted by decision 0441; V1-0939, V1-0942) | `cmd/corvint/host_adapter_projection.go` (`hookContextProjection`, `hookCompaction`, `claudeSubagent`, `claudeSessionGuidance`, `withHookContextSuffix`), `renderAdapterResult` and both adapters in `cmd/corvint/host_adapter.go`, `recordDeliveredPacket`, `conformance/host-lifecycle-v1` | `cmd/corvint/host_adapter_projection_test.go` (`TestAHI046HookContextProjectionSilenceRule`, `TestAHI046SilentProjectionRendersNothing`, `TestAHI047GuidanceIsMainThreadSessionStartOnly`, `TestAHI046CodexPromptSilenceAndProjection`); `TestClaudeNativeDogfoodLifecycle` subtests for the first blocked Stop, the anchored prompt, the silent anchorless prompt and main-thread versus `agent_id` SessionStart; `TestAHI003ClaudeCompactSessionStartRehydratesDirtyPaths` (projected compaction results equal the receipt's); the silent-prompt case of `TestClaudeAdapterUnplannedReadCallSites`; `conformance/host-lifecycle-v1` projection case |
| `AHI-048` (proposed; V1-0734) | `internal/groupreap/live.go` (`StartLive`, `KillLive`), the release in `groupreap.Wait`, `groupreap.Drain` and the `Owner` reap, `StartLive` in `internal/gokernel/repository.go`, `Drain` in `internal/contextindex/git_execution.go`, `internal/cem/gitrun`, `internal/gitstatus/executable.go`; `cmd/corvint/host_exit.go` `exitProcess` | `cmd/corvint/host_exit_unix_test.go::TestAHI048ExitProcessKillsLiveChildGroups`; `cmd/corvint/host_adapter_fail_open_test.go::TestAHI044HookAdaptersFailOpen` (slow-Git outlive bound, `failOpenOutliveBound`); `internal/gitstatus/executable_unix_test.go::TestAHI048LookupKeepsOwnedWorkerGroup`; `internal/groupreap/live_unix_test.go` (`TestAHI048KillLiveRetiresRecordedGroups`, `TestAHI048StartLiveRecordsOnlyOwnGroups`, `TestAHI048WaitAndOwnerReleaseBeforeReap`, `TestAHI048DrainKeepsGroupRecordedUntilReap`, `TestAHI048ConcurrentStartsReleasesAndKill`) |
| `AHI-049` (accepted by decision 0477, amended by decision 0479; V1-1040, V1-1055) | `internal/procgroup/process_posix.go` (`startProcessCommand`, `releaseProcessCommand`), the release before the reap in `internal/procgroup/process.go` `run`, `internal/groupreap/live.go` `ReleaseLive`; V1-1055 pre-reap `proveProcessGroupQuiescent`, the `groupOwnership` lock of `internal/procgroup/process.go` and `internal/groupreap/owner_waitid.go` `HeldGroupQuiet` | `cmd/corvint/host_exit_unix_test.go::TestAHI049ExitProcessKillsProcgroupGroup`; `internal/procgroup/process_posix_test.go` (`TestAHI049RunReleasesGroupBeforeReap`, `TestAHI049FailedExitObservationSendsNoGroupSignal`, `TestV11055NoNumericGroupProbeAfterReap`, `TestV11055QuiescenceProvedWhileLeaderHeld`, `TestV11055NoGroupOperationAfterConcurrentRelease`); `internal/gitstatus/executable_unix_test.go::TestAHI048LookupKeepsOwnedWorkerGroup` |
| `AHI-050` (accepted by decision 0474; V1-0371) | `requestGroupTermination`, `terminatePendingGroups`, `signalGroup`, `holdUnreaped`, `cleanupConfirmed` and `groupGone` in `integrations/opencode/src/runtime.js`; `terminateGroup`, `signalGroup`, `holdUnreaped` and `cleanupConfirmed` and the bounded `settle` in `integrations/gemini-cli/hooks/corvint-hook.mjs` | `integrations/host-adapters.test.mjs` under `TestHostAdapterJavaScriptHosts`: `V1-0371 <host> timeout and cancellation kill a TERM-ignoring descendant before the leader is reaped`, `V1-0371 <host> normal exit with a surviving descendant reports unconfirmed cleanup, never success`, `V1-0371 <host> timeout names a failed group kill instead of the timeout`, for `opencode` and `gemini`; `V1-0371 OpenCode never signals a leader reaped in the same batch as a sibling exit`, `V1-0371 Gemini never signals a leader reaped in the same batch as a sibling exit`, `V1-0371 Gemini completes with unconfirmed cleanup when a descendant keeps the leader's stdout open`, `V1-0371 Gemini exits after reporting a leader whose group kill failed`, `V1-0371 OpenCode concurrent cancellations share one SIGTERM grace` |
| `AHI-058` (accepted under the owner's standing auto-accept (2026-10-11); V1-1122) | `confirmCleanup` and `probeGroup` in `integrations/gemini-cli/hooks/corvint-hook.mjs` | `V1-1122 Gemini EPERM re-probe confirms only on ESRCH and is bounded` (`integrations/host-adapters.test.mjs`, run by `TestHostAdapterJavaScriptHosts`); failed on the base hook, passes after |
| `AHI-051` (proposed; V1-1070, V1-1071) | `integrations/claude-code/plugins/corvint/.claude-plugin/plugin.json`, `hooks/hooks.json`, `skills/*/SKILL.md`, `integrations/claude-code/.claude-plugin/marketplace.json` | `integrations/host-adapters.test.mjs` (`AHI-051 Claude Code plugin presents complete metadata, plain status messages and read-only skills`); `claude plugin validate`; `conformance/host-lifecycle-v1` discovery case |
| `AHI-036`–`AHI-041` | `integrations/opencode/src/workbench.js`, `workbench-tui.tsx`, `session-metrics.js`, `task-metrics.js`, `qualification.js`, and inspector RPC | `integrations/opencode/workbench.test.mjs`, focused AHI-036 task-detail receipt test in `task-metrics.test.mjs`, and stock OpenCode 2 terminal witness; exact-package AHI-032 qualification remains separate |
| `AHI-025` | `cmd/corvint/pi_tools.go`, `integrations/pi/tools.js` | `TestPiToolContextExpansion`, `TestPiToolRecord`, `TestPiToolClosedInput` and native Pi tool/RPC fixtures |
| `AHI-026` | `integrations/claude-code/plugins/corvint/hooks/hooks.json`, `compatibility.json` `compactionHooks`, `cmd/corvint/host_adapter.go` declared-kill table | `TestAHI026ClaudeCompactionHooksRegisteredAgainstHostAPI` (matcherless `PreCompact`/`PostCompact` groups, verified host version equals the tested maximum, closed trigger set) and `TestAHI017AdapterHostKillMatchesDeclaredHooks` (the two new declared kills) |
| `AHI-027` | `cmd/corvint/host_adapter_compaction.go` (`runClaudeCompactionEvent`, `compactionBlockFor`, `compactionPinLine`), `emitAdapterOutput` plain-stdout branch | `TestAHI027ClaudePreCompactEmitsPinFromCompactionBlock` (instruction plus pin as text, pin equals the fixture's HEAD tree and tracked dirty path, 24-path bound with hostile paths elided); `TestAHI027ClaudeCompactionPinsCleanAndUntrackedOnlyTrees` (clean and untracked-only trees pin the HEAD tree and report without a fault; an over-budget block elides its unlisted tracked paths); `TestClaudeCompactionDegradationIsPlainText` (degradations print frame text through `compactionPlainOutput`, an empty summary prints an empty line) |
| `AHI-028` | `cmd/corvint/host_adapter_compaction.go` (`runClaudePostCompact`, `parseCompactionPin`, `compactionPinMissing`, `compactionReportLine`) | `TestAHI028ClaudePostCompactReportsNonRehydratablePaths` (exact report naming the pinned path the tree lacks; lost, escaping and unresolvable pins degrade by name) |
| `AHI-029` | both compaction events | `TestAHI029ClaudeCompactionHooksMutateNothing` (byte-size snapshot of the whole fixture including `.git` is unchanged across a pin and its verification); `TestAdapterDegradationAdmitsCompactionEventsAndCodes` (every compaction degradation is admitted to the SOL-V0-010 ledger on both events) |
| `AHI-030` | `cmd/corvint/host_adapter_compaction.go` (`compactSessionDisclosure`), Claude branch of `runClaudeAdapter` | `TestAHI003ClaudeCompactSessionStartRehydratesDirtyPaths` (compact `SessionStart` additionalContext begins with the disclosure) |
| `AHI-057` (accepted under the owner's standing auto-accept (2026-10-11); V1-1974) | `cmd/corvint/host_adapter_compaction.go` (`compactionPinUnavailable`, `compactionPinNotEmitted`, the pre-compact branch of `compactionPlainOutput`) | `TestAHI057ClaudeCompactionDistinguishesUnemittedPinFromLostPin` (a deadline-degraded pre-compact prints the unavailable line before its frame; a recorded-shape summary that keeps it, reflowed or after a carried-forward pin, reports not-emitted; one that keeps neither line, or whose last pin is mangled or truncated after an unavailable line or a whole pin, reports not-preserved; a later pin is verified); `TestClaudeCompactionDegradationIsPlainText` |
| `AHI-031` | `cmd/corvint/local_completion_event.go` (`dogfoodExpiryCode`, snapshot-miss flag in `localEventContext`, `dogfoodMissOutlastsDeadline`), `cmd/corvint/host_adapter.go` (`withSnapshotRemediation`, `adapterDegradationReason`, `snapshotRemediation`, `withCodexSnapshotRemediation`), `internal/observations` rejection registry | `TestDogfoodEventSnapshotMissExpiryNamesStaleSnapshot`, `TestClaudeAdapterStaleSnapshotNamesRemediation` (fail at base 489701ca with `dogfood-event-deadline` and no argv); V1-0286 amendment (accepted, decision 0422): `TestDogfoodEventSnapshotMissUsesRecordedBuildCost`, `TestCodexAdapterStaleSnapshotNamesRemediation` |
| AHI-053 (accepted, decision 0517) | `snapshotRemediation` | `TestSnapshotRefreshCommitThenEvent`, `TestSnapshotRefreshDisabledAndFailures` |
| `AHI-070` (accepted under the owner's standing auto-accept (2026-10-11); V1-2062) | `internal/gokernel/harness.go` `validateInput` | `TestAHI070OverBoundTaskRefusalNamesLimitAndLength` (counts and bounds named, text never echoed; a task at the bound once trimmed but over it raw is refused with its raw counts) |
| `AHI-171` (accepted under the owner's standing auto-accept (2026-10-11); V1-2192) | `internal/opencodequalification/contention.go`, `native.go` (window readings, `latencyOutcome`, `RETRY_LATER`), `tools/qualify-opencode` exit 75 | `TestAHI171LatencyHostContention` (a latency-only miss at load 11.5 on 12 CPUs is `LATENCY_HOST_CONTENDED` and retry-later with a hint; the same miss on a quiet host, with unobserved contention, with another failed check or with a short sample is not; absent readings are `NOT_OBSERVED`; a live host reading) |
| `AHI-172` (accepted under the owner's standing auto-accept (2026-10-11); V1-2201) | `internal/opencodequalification/baseline.go` (`sampleBaseline`, `baselineComparison`, `baselineEvidence`), `contention.go` (`latencyOutcome`, `retryHint`, `procLoadavgPath`), `native.go` window sampling | `TestAHI172SameWindowBaseline` (both slow under contention is host-contended with the comparison recorded; a candidate beyond its slowdown-scaled limits is a regression under load; a slow probe on a quiet host excuses nothing; an unobserved baseline keeps the AHI-171 rule with `NOT_OBSERVED` fields; a live sampler), `TestAHI172ProcLoadavgFixture` (the Linux reader from a fixture, malformed lines unobserved) |
| `AHI-173` (accepted under the owner's standing auto-accept (2026-10-11); V1-2145) | `script/host-package-version.sh`, `Makefile` (`host-package-version-bump`, `host-package-version-verify`) | `script/host-package-version_test.sh` (every package bumped against copies of the real manifests, one line per location, evidence versions untouched, explicit version, disagreement named and bump refused, unknown package exit 2); the failure-message case in `script/check-host-package-versions_test.sh` |
| `AHI-174` (accepted under the owner's standing auto-accept (2026-10-11); V1-2346) | `internal/opencodequalification/record.go` (`SupportedHostRange`, `ParseSupportedHostVersion`), `integrations/opencode/src/qualification.js`, `package.json` | `internal/opencodequalification/record_test.go::TestSupportedHostVersion` (2.0.18..2.1.999 admitted; prerelease, leading zero, 2.2.0, 2.0.17 refused), `integrations/opencode/evidence.test.mjs` range test, live 2.1.3 campaign |
| `AHI-175` (accepted under the owner's standing auto-accept (2026-10-11); V1-2346) | `internal/opencodequalification/command.go` (`admittedGoVersion`) | `internal/opencodequalification/record_test.go::TestAdmittedGoVersion` |
| `AHI-176` (accepted under the owner's standing auto-accept (2026-10-11); V1-2346) | `integrations/opencode/src/skill.js`, `src/index.js`, `internal/opencodequalification/native.go` (`bundledSkillAdvertised`), `record.go` context case | `integrations/opencode/skill.test.mjs`, `internal/opencodequalification/native_test.go::TestBundledSkillAdvertised`, live 2.1.3 campaign `native-bundled-skill` |
| `AHI-177` (accepted under the owner's standing auto-accept (2026-10-11); V1-0453) | `integrations/opencode/package.json` `testedHostVersions`, `integrations/compatibility.json`, `conformance/opencode-native-v1/` | `integrations/opencode/evidence.test.mjs` (mandatory check set, native status probe), live 2.1.3 `native-status-probe.json` |
| `AHI-178` (accepted under the owner's standing auto-accept (2026-10-11); V1-0453) | `Makefile` `host-adapter-test` | the target's `node --test integrations/opencode/*.test.mjs` run |
| `AHI-179` (accepted under the owner's standing auto-accept (2026-10-11); V1-2346) | `internal/opencodequalification/probe.go` (`retainObservation`) | `internal/opencodequalification/process_test.go::TestCommandInterruption`, `integrations/opencode/evidence.test.mjs` sidecar negative cases, live 2.1.3 full-mode campaign `interruption-cleanup` |
| `AHI-190`..`AHI-197` (accepted under the owner's standing auto-accept (2026-10-11); V1-0403) | `cmd/corvint/host_adapter_done_list.go` (`runDoneListCommand`, `runClaudeStopDoneList`, `doneList`, `renderDoneList`), `cmd/corvint/host_adapter.go` (`done-list` verb, `stop-done-list` event, `AHI-017` table), `integrations/claude-code/plugins/corvint/hooks/hooks.json`, `skills/done/SKILL.md` | `TestAHI191StopDoneListOptInSilenceAndShape`, `TestAHI193DoneListMessageBoundedAndPlain`, `TestAHI195DoneListCommandRefusals`, `TestAHI196DoneListRefusesWhenHeadMoves`, `TestAHI196DoneListReceiptNamesHead`, `TestAHI196DoneListRefusesWithoutHead`, `TestAHI196DoneListRunsNoRepositoryFilter`, `TestAHI197DoneListJudgesNewTestsByMutation`, `TestAHI197DoneListVerdictNeedsExecutedMutants` (`cmd/corvint/host_adapter_done_list_test.go`); `TestAHI017AdapterHostKillMatchesDeclaredHooks`; `TestAHI044HookAdaptersFailOpen`; the `AHI-051` skill test under `TestHostAdapterJavaScriptHosts` |
| `AHI-198` (accepted under the owner's standing auto-accept (2026-10-11); V1-2466) | `integrations/claude-code/plugins/corvint/.mcp.json`, `compatibility.json` (`surfaces.mcp`, `mcpServer`) | `AHI-198 Claude Code plugin declares the read-only corvint-mcp server at the project root` under `TestHostAdapterJavaScriptHosts`; live `claude mcp list` receipt in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-claude-code-integration-matrix.md` |
| `AHI-199` (accepted under the owner's standing auto-accept (2026-10-11); V1-2440) | `script/heavy.sh` (`HEAVY_NICE`), `Makefile` (`HEAVY_NICE_PREFIX`); the pinned `ledger/go-test` stays unprefixed | the `AHI-199` group in `script/lane-scripts_test.sh`; the user-prompt timing runs in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-23-claude-code-integration-matrix.md` |
| `AHI-206` (accepted under the owner's standing auto-accept (2026-10-11); V1-2326) | `integrations/codex/profiles/corvint-review.config.toml` | `TestAHI206CodexReviewProfileClosesBothWriteDoors` (exactly the three keys, repeated as the README's `-c` overrides); live receipt with a conflicting trusted project config in the 2026-10-23 Codex matrix entry |
| `AHI-207` (accepted under the owner's standing auto-accept (2026-10-11); V1-2326) | `integrations/codex/plugins/corvint/.mcp.json`, `.codex-plugin/plugin.json` | `TestAHI207CodexPluginMCPServerBindsLaunchWorktree` (nested launch binds the top level with the legacy protocol; a non-worktree launch exits 2 before exec); live receipt in the 2026-10-23 Codex matrix entry |
| `AHI-208` (accepted under the owner's standing auto-accept (2026-10-11); V1-2326) | `integrations/codex/plugins/corvint/hooks/hooks.json` | `TestAHI208CodexHooksRegisterOnlyMappedEvents` (with empty-group, extra-event and matcher negatives), `TestAdapterSilentAbstentionsAreLedgered` (unregistered events); live hook-invocation log in the 2026-10-23 Codex matrix entry |
| `AHI-209` (accepted under the owner's standing auto-accept (2026-10-11); CODEX-NATIVE-APPROVAL-RELAY-VISIBILITY-1) | `integrations/codex/plugins/corvint/skills/corvint/SKILL.md`, `integrations/codex/README.md` | `TestAHI209CodexApprovalGuidanceNamesRootGrantAndRecord`; live guardian verdicts in the 2026-10-23 Codex matrix entry |
| `AHI-210` (accepted 2026-10-25; V1-2717) | `orientationFrame`, `collectTaskContext`, `onPrompt`, `corvintContext.description` in `integrations/opencode/src/index.js` | `AHI-210 AHI-211 AHI-215 prompt hook abstains by default and delivers compact candidates, not the packet` (`integrations/host-adapters.test.mjs`); `TestHostAdapterJavaScriptHosts`; paired trial in `docs/build-log/2026-10-25-v2716-v2717-query-recall-opencode-adoption.md` |
| `AHI-211` (accepted 2026-10-26; V1-2717) | `candidateFrame`, `onPrompt`, `directTool`, the tool descriptions in `integrations/opencode/src/index.js`; `Results` and `Context` in `integrations/testfixture/main.go`; step 0 in `internal/opencodequalification/provider.go` | `AHI-210 AHI-211 AHI-215 prompt hook abstains by default and delivers compact candidates, not the packet` (`integrations/host-adapters.test.mjs`); `TestHostAdapterJavaScriptHosts`; smoke attribution and paired trial in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-26-v2717-opencode-adoption.md` |
| `AHI-212` (accepted 2026-10-26; V1-2717) | `extend`, the gated `ctx.tool.transform`, `registerSkill`, `corvintContext.description` in `integrations/opencode/src/index.js`; `bundledSkillAdvertised` in `internal/opencodequalification/native.go` | `AHI-212 idle surface is one direct tool; its first call unlocks the follow-ups and the skill` (`integrations/host-adapters.test.mjs`); `AHI-176 plugin setup registers the skill and a failing registry leaves tools intact` (`integrations/opencode/skill.test.mjs`); `TestBundledSkillAdvertised`; `TestHostAdapterJavaScriptHosts`; idle smoke and paired rerun in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-26-v2717-opencode-autocontext-rerun.md` |
| `AHI-240` (experimental, lane auto-accept 2026-10-27; V1-2854) | `lit:` locations in `script/host-package-version.sh`; `Makefile` `host-package-versions-check`; `PI_ADAPTER_VERSION` import in `integrations/pi/workflow.js` | `script/host-package-version_test.sh` (pi bump changes six lines; drifted `runtime.js` / `pi_adapter.go` named; make recipe runs verify); `AHI-024 every Pi adapter version literal equals the package manifest version` (`integrations/pi/capabilities.test.mjs`); `TestPiLocalCompletionTuple` |
| `AHI-249` (experimental, lane auto-accept 2026-10-28; V1-2505) | `lit:ADAPTER_VERSION` opencode location in `script/host-package-version.sh` | `script/host-package-version_test.sh` (opencode bump changes four lines; drifted opencode `runtime.js` named by verify) |
| `AHI-245` (experimental, lane auto-accept 2026-10-27; #1936) | `internal/opencodequalification/record.go` (`SupportedHostRange`, `ParseSupportedHostVersion`), `integrations/opencode/src/qualification.js` (`compatibleHostVersion`, `SUPPORTED_HOST_RANGE`), `package.json` | `internal/opencodequalification/record_test.go::TestSupportedHostVersion` (`AHI-245` case), `integrations/opencode/evidence.test.mjs` `AHI-245` range test, live 2.2.0 and 2.1.3 native-only campaigns (PASS 27/27 each) |
| `AHI-246` (experimental, lane auto-accept 2026-10-27; #1936) | `SUBSCRIBED_EVENTS` and the `ctx.event.subscribe` call in `integrations/opencode/src/index.js` | `AHI-246 the OpenCode event subscription asks only for the event types the adapter handles` (`integrations/host-adapters.test.mjs`), run by `TestHostAdapterJavaScriptHosts`; live 2.2.0 native-only campaign |
| `AHI-248` (experimental, lane auto-accept 2026-10-11; V1-2972) | `envelopeFault` in `integrations/opencode/src/runtime.js` and the `prompt-echo-withheld` branch of `onPrompt` in `integrations/opencode/src/index.js` | `AHI-248 a one-word prompt equal to a returned symbol name is withheld as an abstention` (`integrations/host-adapters.test.mjs`), run by `TestHostAdapterJavaScriptHosts`; build log `2026-10-28-moved-test-base-package` |
| `AHI-213` (experimental, standing lane auto-accept; V1-2804) | `scriptedCalls`, `sourceTask`, `reasonNoExpansionHandle`, `ServeHTTP` in `internal/opencodequalification/provider.go`; `checkFailures`, `checkFailureReasons`, `campaignFailure` in `internal/opencodequalification/native.go`; `retainObservation` in `internal/opencodequalification/probe.go` | `TestAHI213ScriptedSourceQuerySuppliesExpansionHandle`; `TestAHI213EveryFalseCheckNamesAReason`; `TestAHI213ObservationFailuresEncodeEmptyArray`; `TestAHI056LatencyResample` (failure message) |
| `AHI-214` (experimental 2026-10-27; V1-2753) | `codeModeNetworkGuard`, `effectiveRules`, `networkPermitted`, `permissionMatches` in `integrations/opencode/src/code-mode-guard.js`; the `execute.before` registration in `integrations/opencode/src/index.js` | `AHI-214 V1-2753 Code Mode execute is refused when the host denies webfetch or websearch (p07)` (`integrations/host-adapters.test.mjs`); `TestHostAdapterJavaScriptHosts`; live OpenCode refusal `NOT_RUN` (paid model) |
| `AHI-215` (experimental 2026-10-27; V1-2755) | `namedExistingPaths`, `stripJunkAnchors`, `frameCandidates`, `corroborated` in `integrations/opencode/src/frame-policy.js`; `candidateFrame`, `collectTaskContext`, `onPrompt` in `integrations/opencode/src/index.js` | `AHI-210 AHI-211 AHI-215 prompt hook abstains by default and delivers compact candidates, not the packet` (`integrations/host-adapters.test.mjs`); `TestHostAdapterJavaScriptHosts`; offline ten-task replay in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-v2755-prompt-frame-abstains.md`; paired model trial `NOT_RUN` (paid) |
| `AHI-247` (experimental 2026-10-28; V1-2944) | `onPrompt` (pre-query `namedExistingPaths` branch, `settlePrompt`) in `integrations/opencode/src/index.js` | `AHI-210 AHI-211 AHI-215 prompt hook abstains by default and delivers compact candidates, not the packet` (`integrations/host-adapters.test.mjs`); `TestHostAdapterJavaScriptHosts`; hook measurement in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-v2944-v2955-prompt-early-exit-live-dispatch.md` |
| `AHI-218` (experimental 2026-10-27; V1-2804) | `framePrompt`, `scriptedCalls` in `internal/opencodequalification/provider.go`; `Native` (campaign prompt, probe template, `critical-recall-and-bytes`) in `internal/opencodequalification/native.go`; `probe.js.tmpl`; `candidateFrame` in `integrations/opencode/src/index.js`; `body`, `effectiveRules` in `integrations/opencode/src/code-mode-guard.js`; `p95Evidence` in `internal/opencodequalification/native.go` | `TestAHI218CampaignPromptEmitsAFrame` (`internal/opencodequalification/provider_test.go`); `AHI-214 V1-2753 Code Mode execute is refused when the host denies webfetch or websearch (p07)` and `AHI-210 AHI-211 AHI-215 prompt hook abstains by default and delivers compact candidates, not the packet` (`integrations/host-adapters.test.mjs`); `TestHostAdapterJavaScriptHosts` |
| `AHI-219` (experimental 2026-10-27; V1-2870) | `RECEIPT_LINK`, `receiptLink`, `rememberFrameLink` in `integrations/opencode/src/frame-policy.js`; `candidateFrame`, `corvintExpand` and the prompt hook in `integrations/opencode/src/index.js`; `receiptLink`, `frameLinkPattern`, `Native` in `internal/opencodequalification/native.go`; `observer.js.tmpl` | `TestAHI219FrameReceiptLink` (`internal/opencodequalification/native_test.go`); `AHI-210 AHI-211 AHI-215 prompt hook abstains by default and delivers compact candidates, not the packet` (`integrations/host-adapters.test.mjs`); `TestHostAdapterJavaScriptHosts` |
| `AHI-216` (experimental 2026-10-27; V1-2762) | `hostLoadedGovernance`, `claudeMemoryImports`, `withoutHostLoadedGovernance` in `cmd/corvint/host_adapter_projection.go`; `renderAdapterResult` in `cmd/corvint/host_adapter.go` | `TestAHI216ClaudeHostLoadedGovernanceIsNotRepeated` (`cmd/corvint/host_adapter_projection_test.go`); offline re-projection in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-v2762-hook-governance-dedupe.md`; live forensic rerun on new sessions `NOT_RUN` (no sessions on this build yet) |
| `AHI-217` (experimental 2026-10-27; V1-2756) | `tools/paired-trial/main.go` (`readReceipts`, `analyse`, `bootstrapRatio`, `mcnemarExact`) | `TestAHI217PairedTrialAnalysis` (`tools/paired-trial/main_test.go`); paired model trial `NOT_RUN` (paid) |
| `AHI-055` (accepted under the owner's standing auto-accept (2026-10-11); V1-1280) | `cmd/corvint/host_adapter.go` (`deadlineRemediation`, `withDeadlineRemediation`), `cmd/corvint/host_adapter_compaction.go` | `TestClaudeAdapterStopDeadlineFailsOpenVisibly`, `TestClaudeCompactionDeadlineNamesRemediation`, `TestDeadlineRemediationKeepsFrameWithinLimit`, `TestClaudeAdapterDogfoodEventDeadlineCarriesNoNotice`, `TestCodexAdapterDeadlineNamesRemediation`, `TestClaudeAdapterStaleSnapshotNamesRemediation`, `TestCodexAdapterStaleSnapshotNamesRemediation` |
| `AHI-054` (accepted, decision 0519; V1-1157) | `internal/gokernel/harness.go` `HandleEvent` stop frontier, `FrontierCauseNoClosingAuthority`; `conformance/cli-parity-v0` `validFrontierCauseDivergence`, `outOfPacketKnownDivergence`; `integrations/testfixture/main.go` | `TestAHI054StopFrontierCarriesFixedCause`, `TestAHI054FrontierCauseDivergenceIsPinnedAndOutOfPacket`, `TestGPKV0002ManifestReplay` (`DR-0043`), Gemini CLI and OpenCode stop cases and the `AHI-054` with-and-without-cause case under `TestHostAdapterJavaScriptHosts` |
| `AHI-004` | native adapter renderers, shared lifecycle command, `internal/repoenvelope`, and the JavaScript envelope builders | byte-identical untrusted-data envelope with hidden-character escaping and terminator refusal (`internal/repoenvelope`, `cmd/corvint`, `tools/native-hook-observer` and `integrations/host-adapters.test.mjs` tests), injection bounds, authority order, and query fixtures |
| `AHI-011`, `015` | embedded `internal/gokernel/host-schema.json` admission table and shared lifecycle command | schema/admission tests plus one host-keyed golden fixture per admitted host |
| `AHI-002`, `006..010` | four native packages and release matrix | install/uninstall, lifecycle, degradation, and version fixtures; for `AHI-010`, the `integrations/host-adapters.test.mjs` test under `TestHostAdapterJavaScriptHosts` binding each `integrations/compatibility.json` row to its shipped declaration and its row's adapter version to the package manifest version, and asserting `globalDegradations` disjoint from `receiptDegradationPolicy.recognised`; for the owned-group cleanup governed by `AHI-050` (accepted by decision 0474, superseding the earlier `AHI-009` kill text), the `V1-0371` OpenCode and Gemini timeout-and-cancellation, normal-exit, failed-kill, same-batch, inherited-pipe and shared-grace tests in that file, whose `orphan-hang` and `orphan-valid` fixture descendant ignores SIGTERM and closes its stdio |
| `AHI-016` | `cmd/corvint/prompt_bound.go`, Claude and Codex native wrappers; `integrations/gemini-cli/hooks/prompt-bound.mjs` and `integrations/opencode/src/prompt-bound.js` in the Gemini CLI hook and OpenCode `corvint_context` tool | `TestAHI016OverBoundPromptDerivesVerbatimAnchorQuery`, `TestAHI016OverBoundPromptKeepsRefusalWhenAnchorsCannotServe`, `TestAHI016ClaudeOverBoundPromptInjectsDisclosedContextWithoutStoring`, `TestAHI016OverBoundPromptMatchesCrossHostBoundaryCases`, and the `AHI-016` cross-host test in `integrations/host-adapters.test.mjs` under `TestHostAdapterJavaScriptHosts` |
| `AHI-017` | `cmd/corvint/host_adapter.go` declared-kill table and watchdog; `integrations/gemini-cli/hooks/corvint-hook.mjs` derived budget | `TestAHI017AdapterHostKillMatchesDeclaredHooks` (which also fails on a matcherless `FileChanged` group), `TestAHI017HostAdapterWatchdogDegradesBeforeHostKill`, `TestHostAdapterPanicDegradesInsteadOfBlocking`, and the two `AHI-017` Gemini tests in `integrations/host-adapters.test.mjs` under `TestHostAdapterJavaScriptHosts` |
| `AHI-018` | `integrations/gemini-cli/hooks/corvint-hook.mjs` argument parser and the `host-adapters.test.mjs` fixture harness | the `AHI-017` Gemini declared-kill test, which pins the shipped command to carry no override, under `TestHostAdapterJavaScriptHosts` |
| `AHI-019` | `cmd/corvint/host_adapter.go` (`postToolTargetAbstention`, `invokeLegacyClaudeEvent`, `claudeReceiptOutput`, `claudeDegradationsRecognised`, `renderAdapterResult`) | `TestClaudeAdapterRoutineReceiptAndExpectedDegradationCarryNoNotice` asserts no output for an out-of-root Edit target and a `PostToolUse` additionalContext receipt with no `systemMessage` for an in-root one; `TestAdapterSilentAbstentionsAreLedgered` requires the out-of-root Write target's `{}` output and its `post-tool-path-outside-project` ledger row (V1-0746, SOL-V0-030); `TestAHI019ClaudePostToolReceiptNamesEveryDegradation` drives a realistic Edit `PostToolUse` payload through the hook entrypoint and requires the receipt's degradation codes in that additionalContext; `TestAHI019ClaudeReceiptRefusesUnrecognisedDegradation` calls `claudeReceiptOutput` with an unrecognised code and requires the named `corvint-degradations-unrecognised` fault `systemMessage` without the code; `TestAHI019CodexWholeReceiptRefusesUnrecognisedDegradation` and `TestAHI019ClaudeDogfoodEnvelopeRefusesUnrecognisedDegradation` call `renderAdapterResult` with an unrecognised code and require exactly the Codex and Claude Code fault output, which carries no code, and the Codex test also requires that fault for a string, object, or `null` `degradations` value |
| `AHI-020` | the four host package manifests listed in the requirement body | `script/check-host-package-versions.sh` (`make host-package-versions-check`) compares, per package, the last commit that changed a version field against the last commit that changed any other shipped file, by Git ancestry between the two commits rather than by committer-second timestamp (two distinct commits, such as either side of a rebase, can share a committer second); `script/check-host-package-versions_test.sh` proves it fails on a content-only change, passes after a version bump, and fails on a newly added shipped file with no bump, its fifth case proves a bump and a later content change sharing one committer second still fails, and its OpenCode cases cover skills-only additions, MCP-example edits, subsequent bumps and unpublished-file exclusions (V1-0452); its Spec Kit and OpenSpec cases cover a command change without a bump, another YAML block's `version` key not counting as the bump, and a skill body edit with and without its bump (V1-2184); CI's `doc-gates` job (`.github/workflows/ci.yml`, full-history checkout) runs both on every pull request, merge-queue entry and main push (V1-0782), not only in the nightly `make gate` |
| `AHI-021` | `cmd/corvint/host_adapter.go` (`claudeExpectedDegradation`, `claudeDegradedOutput`, `renderClaudeContext`) | `TestClaudeAdapterRoutineReceiptAndExpectedDegradationCarryNoNotice` asserts `prompt-over-query-bound` is `UserPromptSubmit` additionalContext with no `systemMessage`; `TestClaudeAdapterDogfoodEventDeadlineCarriesNoNotice` asserts an expired dogfood event deadline is `UserPromptSubmit` additionalContext with no `systemMessage`; `TestClaudeAdapterFaultKeepsNotice` asserts a `missing-session-identity` fault inside a repository keeps its `systemMessage`; `TestClaudeAdapterNotARepositoryIsSilent` asserts Claude `session-start`, Edit and Write `post-tool`, and Codex `SessionStart` outside a Git repository emit nothing and create no `.corvint` (decision 0178) |
| `AHI-022` | `integrations/gemini-cli/hooks/corvint-hook.mjs` (`EXPECTED_DEGRADATIONS`, `degradation`, `successOutput`); `integrations/opencode/src/index.js` (`report`, `record`, `CHANGED_TARGETS`, `afterTool`, `EVENT_HANDLERS`, `MAX_FILE_CHANGE_PATHS`, `queueFileChange`, `drainFileChanges`); `integrations/opencode/src/runtime.js` (`receiptIdentity`); `internal/gokernel/harness.go` (`takeAdapterCodes`); V1-0450: `cmd/corvint/harness_context.go` (`harnessIndexedBlock`), `internal/contextindex/impact.go` (`ErrImpactPathsUntracked`, `IsImpactPathsUntracked`), `integrations/opencode/src/index.js` (`EXPECTED_FILE_CHANGE_REFUSALS`) | under `TestHostAdapterJavaScriptHosts`, the `integrations/host-adapters.test.mjs` tests `AHI-022 Gemini routine receipt and expected degradation carry no notice`, `AHI-022 Gemini fault keeps notice`, `AHI-022 decision 0178 Gemini outside a Git repository emits and invokes nothing`, `AHI-022 OpenCode routine receipt goes to the info log and a fault keeps its warning` (including the structured suffix refusal), the Go test `TestAHI022HarnessFileChangeCodesUntrackedPath` (V1-0450 `untracked-impact-path` envelope, unchanged `impact` refusal, no ledger row), `AHI-022 decision 0378 OpenCode outside a Git repository registers and invokes nothing`, `AHI-022 decision 0378 OpenCode repository detection follows subdirectories, symlinks and linked worktrees`, `AHI-022 decision 0379 OpenCode lifecycle against the real binary writes nothing to the terminal` (the real `cmd/corvint` build, which pins the `invalid-arguments` non-repository refusal the permissive fixture cannot, and V1-0450's newly created untracked file at info level), `AHI-022 V1-0746 OpenCode names the path cap and an out-of-project path at info level` (including the `adapterCodes` each carries), `SOL-V0-010 AHI-022 V1-0767 OpenCode path abstentions reach the self-observation ledger against the real binary`, `AHI-022 V1-0773 OpenCode file-change of 101 to 256 paths stays within Core's impact bound against the real binary`, and `AHI-022 OpenCode serializes a burst of file-change subprocesses`; `CRB-V0-010 CRB-V0-011 OpenCode loaded plugin keeps exact aliases, option precedence, session isolation, payload bounds and repeat-stop suppression` covers the `execute.after` changed-path mapping and the `app.version` host version |
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
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/BUILD-LOG.md` (2026-09-06 lifecycle repair). Compatibility remains `FALLBACK`.

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
