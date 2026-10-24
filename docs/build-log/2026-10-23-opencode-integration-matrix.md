# OpenCode 2.1.3 integration matrix and host-lane delivery

Date: 2026-10-23. Tickets: V1-2346 (range), V1-0453 (tested versions and MCP evidence), V1-2201
(closeout note), V1-1563 (OpenCode half). Requirements: AHI-174..AHI-179 in
[the AHI spec](../specs/agent-harness-integration-v0.md). Base: origin/main `368b80d4`.

## Installed host

OpenCode `2.1.3`, `/opt/homebrew/Cellar/opencode-v2/2.1.3/bin/opencode`, sha256
`1638ccfd0936dc10b15df34268b541a219822f4735d564359420d426c03383f1`, darwin-arm64. Extension points
were read from the installed binary and its `--help`, not from memory: the plugin context exposes
`tool.transform`, `tool.hook("execute.before"|"execute.after")`, `session.hook("prompt"|"context"|
"compaction"|"generate"|"model.request"|"title"|"retry"|...)`, `skill.transform`,
`command.transform`, `agent.transform` (no add operation observed), `mcp.transform`,
`permission.hook("evaluate")`, `event.subscribe`, `rpc.register` and the TUI entry. In 2.1 the model
sees only the built-in tools plus `execute` (Code Mode); plugin tools are called as
`tools.corvint_context(...)` and MCP tools as `tools.corvint.<tool>`.

## Matrix

Columns: **Tool** plugin tools (`tool.transform`), **Hook** session/tool hooks and events,
**Skill** skill registry, **Cmd** command registry, **Agent** agents/subagents, **MCP** `mcp.servers`
with `corvint-mcp`, **Perm** permission evaluation, **Instr** AGENTS.md/instructions, **TUI** TUI
plugin and inspector RPC. Q = QUALIFIED, D = DELIVERED, G = GAP, `was X` marks a cell this lane
changed. Q cells cite the retained 2.1.3 receipts: N =
`conformance/opencode-native-v1/results/2.1.3-darwin-arm64/native-report.json` (named check), M =
`mcp-probe.json`, MG = `mcp-probe-global-config.json`. A blank cell is NOT_APPLICABLE: the
capability has no behaviour at that extension point (for example a TUI cell for outcome recording).

| Capability | Tool | Hook | Skill | Cmd | Agent | MCP | Perm | Instr | TUI |
|---|---|---|---|---|---|---|---|---|---|
| Host-version admission (2.1.x) | Q N `native-host-image` (was G: 2.1.3 failed closed, V1-2346) | | | | | | | | |
| Qualification status | Q N `direct-install` | | | G V1-2498 | | Q M `corvint_status` READY | | | D cockpit |
| Task context / query | Q N `native-query` | Q N `awaited-current-prompt`, `bounded-prompt` | | G V1-2498 | | D MG listed, not called (V1-2503) | | | |
| Exact expansion | Q N `native-exact-expansion` | | | | | | | | |
| Outcome recording | Q N `explicit-outcome` | | | | | | | | |
| Impact | | | | G V1-2498 | | D MG listed, not called (V1-2503) | | | |
| Session start packet | | Q N `native-discovery`, `snapshot` | | | | | | | |
| Pre-tool context | | G V1-2499 | | | | | | | |
| Pre-edit guard | | | | | | | G V1-2500 | | |
| Post-tool observation | | Q N `verification-observation`, `native-edit` | | | | | | | |
| File-change / dirty paths | | Q N `dirty-path-recovery` | | | | | | | |
| Compaction rehydration | | Q N `native-compaction` | | | | | | | |
| Stop / session end | | Q N `bounded-stops`, `native-exit`, `interruption-cleanup` | | | | | | | |
| Project governance | | | | | | | | Q N `governance` | |
| Corvint skill guidance | | | Q N `native-bundled-skill` (was D: copy step only) | | | | | | |
| Review / CEM delegation | | | D via skill text | G V1-2498 | G V1-2501 | | | | |
| corvint-mcp registration | | | | | | Q M project, MG global (was D: 1.x-only evidence, V1-0453); auto-registration G V1-2502 | | | |
| test-validity / docs MCP | | | | | | G V1-2503 (unprobed on 2.x) | | | |
| Task metrics / workbench | | | | | | | | | D JS tests, no live TUI receipt |
| Inspector RPC | | | | | | | | | D JS tests, no live TUI receipt |
| JS adapter tests in CI | | | | | | | | | |

The last row is a repository gate, not a host cell: before this change `make host-adapter-test`
ran none of `integrations/opencode/*.test.mjs` (a degrading gap for every D cell above); AHI-178
adds them.

Counts over non-blank cells (30; the registration cell counts its Q and its G). Before: Q 12,
D 8, G 10. After: Q 15, D 6, G 9: the range G, the skill D and the MCP-registration D became Q, and
the CI gate gap (outside the cell count) closed. Every remaining G has a ticket. NEEDS-OWNER: 0; the permission-guard
design (V1-2500) carries an owner question in its ticket.

## Delivered

- AHI-174: range `>=2.0.18 <2.2.0` in Go, JS and the manifest (adapter 0.7.18).
- AHI-175: full qualification admits go1.27.2 and go1.27.3 (decision 0547); go1.27.3 was refused.
- AHI-176: the plugin registers its bundled skill through `ctx.skill.transform`; a project or user
  `corvint` skill wins; the new native check `native-bundled-skill` reads the outgoing provider
  request's `<available_skills>` guidance.
- AHI-177: `testedHostVersions` `["2.1.3"]` names committed native and MCP evidence;
  `integrations/opencode/evidence.test.mjs` enforces it, and the README MCP section covers 2.x.
- AHI-178: `make host-adapter-test` runs the OpenCode JS tests.
- AHI-179: the interruption-cleanup evidence keeps its full process observation in a sibling
  file and embeds a digest-bound summary. The first full-mode run on a loaded host (load about 17)
  was refused with `qualification record exceeds consumer bound`: the 20 ms observation held 665
  processes (82 KB).

## Live receipts

- Native campaign `opencode-native-integration/1` on 2.1.3: PASS, every check true, retained as N.
  The committed report is from the full-mode run against the committed tree.
- MCP: project-config legacy selector PASS (M); global-config listing PASS (MG); global-config
  `"protocol": "2026-07-28"` PASS (`mcp-probe-protocol-2026-07-28.json`). The first model turn
  reported `Unknown tool namespace 'corvint'` in two runs: 2.1 connects MCP servers asynchronously.
- Dispatch context (V1-1563, OpenCode half): the 2.1.3 `run --format json` stream was parsed by
  `supervisor.OpenCodeLineUsage`: six `step_finish` lines, all admitted, the usage step giving
  input 130000 + output 5000 = estimate 135000 from the loopback provider. A live dispatched session
  with a real model, a warn crossing and a CONTEXT hard stop is NOT_RUN: no account or network
  provider is available to this lane and loopback counters are synthetic. V1-1563 stays open. The
  stream carries `tokens.cache.read/write` and `reasoning`, which the estimate ignores (suspected
  under-count, V1-2504).
- TUI, workbench and inspector: NOT_RUN live (no interactive terminal in the lane).

## V1-2201 closeout note

V1-2201 (same-window latency baseline, live Linux `/proc/loadavg`, run under synthetic CPU load)
was delivered by PR #1501; GitHub #1406 is closed. This lane did not change that code. The owner
closes the ticket after reading the PR's retained evidence.

## Tickets filed

V1-2498 commands, V1-2499 pre-tool context, V1-2500 permission guard, V1-2501 reviewer agent,
V1-2502 `mcp.transform` registration, V1-2503 companion MCP servers unprobed, V1-2504 suspected
cache-token under-count. V1-2505: `host-package-version.sh bump opencode` left `src/runtime.js`
`ADAPTER_VERSION` at 0.7.17 and the version check passed; fixed by hand here. Deduplicated against V1-0454 (notice channel), V1-1768 (context delivery)
and V1-0451 (Bun runtime), which stay as they are.

## Rollback

Each AHI-174..AHI-179 entry names its own rollback. Removing the evidence directory alone makes
`evidence.test.mjs` fail, so empty `testedHostVersions` in the same change.
