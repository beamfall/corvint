# Change proposal: agent packages for Claude Code, Codex, OpenCode and Pi

Change name: `agent-packages`
Ticket: none filed yet. A ticket is filed in `.taskman/` before any lane starts; it holds the queue.
Product requirement served: PROD-TEST-009 (an agent asks for the affected tests through MCP),
PROD-DOCS-001 and PROD-TASK-007 (agents use the documentation and task products) in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/agent-packages-v1.md` (new, prefix `AGT-V1`)
Requirement IDs this change adds: AGT-V1-001 to AGT-V1-030 (30 requirements; AGT-V1-029 and
AGT-V1-030 were added for review fixes F22 and F23, and the header said 001 to 028 before). The
prefix had no entry in `docs/specs/REQUIREMENTS.tsv` on 2026-11-09.
Owner answers this comes from: G19, G20, G21, G10, G10a, G25 (item 2), answer 11, answer 16,
answer 18, owner decisions R4, R5, R13, R17, R25, R27, R30, R39 and R40 (2026-11-09; R39 and R40
amend R26), and the owner instruction of 2026-11-09 ("we also want to make sure that we have support
for claude, codex, opencode and pi as part of this plan. the support should be world class and
industry leading")
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started.
Supersedes in part: decision 0594 item 1 and requirement PRS-V1-006. This proposal cannot be
accepted until the superseding decision described under "Superseding decision" is accepted.

This change follows the G14 layout: `proposal.md` (this file), `design.md`, `tasks.md` and
`specs/agent-packages/spec.md`.

## Why

An agent that edits code is the main user of "which tests must run for this diff". Today nothing
tells it. The two packages left after decision 0594 are skill-only: a Claude Code plugin at 0.4.1
with four skills and a Codex plugin at 0.3.1 with one skill
(`integrations/claude-code/plugins/corvint/.claude-plugin/plugin.json`,
`integrations/codex/plugins/corvint/.codex-plugin/plugin.json`). Neither registers MCP, neither has
a hook, nothing runs after an agent edits a file, there is no docs or Tasks skill, no package is
qualified on a real agent version, and OpenCode and Pi have nothing. The owner set a full bar for
Claude Code and Codex that blocks 1.0, a preview for OpenCode and Pi (G19), a new public repository
with its own cadence (G20), and a definition of "supported" based on retained passes on real agent
binaries (G21).

The 2026-10-26 finding that injecting retrieved context into prompts did not pay still holds. The
hooks in this proposal report only which tests are affected and whether they ran; they inject no
repository context.

## What Changes

- New public repository `beamfall/corvint-agents` under Apache-2.0 (owner decision R25, 2026-11-09;
  earlier wording: "a permissive licence") and with its own release cadence (G20). It holds the generator, the generated per-agent packages, the skill text and the
  qualification harness.
- Logic stays in the Corvint binary. Two new read commands: `corvint agent hook <event> --host <h>`
  (reads the host's hook payload on stdin, writes the host's expected reply on stdout) and
  `corvint agent manifest --json` (versioned description of the MCP servers, hook events, tools and
  supported hosts). Packages are thin shims generated from one source against a pinned Corvint
  release, with a drift check in CI (G20).
- The full bar for Claude Code and Codex (G19): one documented install step (two commands on
  Claude Code, AGT-V1-008) plus `corvint agent doctor` (singular noun, decided 2026-11-09 under
  R42, D4);
  MCP registered; three generated skills (tests, docs, tasks); an after-edit hook that records
  touched paths; an end-of-turn report of affected tests and the command to run them, "UNKNOWN, run
  all" when unsure, and a flag on edits that have no passing test receipt for the current tree,
  delivered as a blocking `Stop` decision with a reason, at most one nudge per turn (owner decision
  R5, 2026-11-09); a session-start readiness line; a restatement of pending tests after compaction
  through `SessionStart` with source `compact` (owner decision R5); a read-only instruction-file check; no context
  injection; qualification on pinned versions.
- OpenCode and Pi get the same generated package as preview. They never block 1.0. Pi is opt-in
  (G19).
- Kill switch `CORVINT_HOOKS=off` (G20). An unknown `CORVINT_SELECTION` value makes the hooks
  report "UNKNOWN, run all" with a reason, where the CLI and CI exit 2 (owner decision R13,
  2026-11-09).
- Docs review is not part of any hook. If a later hook reports it, it is advisory unless the
  repository opts in to blocking (owner decision R4, 2026-11-09).
- Qualification on real agent binaries at versions pinned at the freeze, one pinned model per host
  with the model ID recorded (owner decision R27, 2026-11-09), rerun on every package or Corvint
  change and weekly against the newest agent release (G21). It runs with the owner's logged-in
  subscription command-line clients (Claude Code, Codex CLI) on the owner's machines (Mac, WSL,
  Unraid) or a self-hosted runner there, with no API keys and no spend cap, and writes one evidence
  file per release candidate (owner decisions R39 and R40, 2026-11-09).
- `integrations/` in `beamfall/corvint` and its consumers are retired after the first qualified
  `corvint-agents` release (AGT-V1-027).

## Capabilities

### New Capabilities

- `agent-packages`: the agent commands in the binary, the generated packages, doctor and
  qualification (`docs/specs/agent-packages-v1.md`, new).

### Modified Capabilities

- `corvint-1.0-product-and-release` (`docs/specs/corvint-1.0-product-and-release-v1.md`):
  PRS-V1-006 is replaced through the superseding decision.
- `agent-instruction-doctor` (`docs/specs/agent-instruction-doctor-v0.md`, proposed,
  experimental): AGT-V1-020 reuses its read-only instruction-file check; the lane decides whether to
  cite or absorb it.
- `capability-catalog` MCP surface: no tool is removed; AGT-V1-010 depends on the existing
  `corvint.affected` tool (`internal/capcatalog/mcp.go:39`).

The retired specs `agent-harness-integration-v0.md` and `host-lifecycle-qualification-v1.md` stay
retired as history; this change does not revive them.

## Impact

- `beamfall/corvint` binary: `corvint agent hook`, `corvint agent manifest`, `corvint agent doctor`,
  session state handling (AGT-V1-022), and the `CORVINT_HOOKS` switch.
- New repository `beamfall/corvint-agents`: generator, skill sources, packages for four hosts,
  qualification harness and fixture repository, CI with drift check and weekly qualification.
- Retired from `beamfall/corvint` (AGT-V1-027), all verified in the integration checkout at head
  `9ec3a29d0`: `integrations/claude-code/**`, `integrations/codex/**` (including
  `profiles/corvint-review.config.toml`, AHI-206), `integrations/README.md`; the fix text at
  `cmd/corvint/adopt_setup.go:185` and its test at `cmd/corvint/adopt_setup_test.go:219`;
  `internal/liveverify/affected/liverepo/integration_readme_documentation_repo_test.go:22,50`;
  `script/check-host-package-versions.sh:254-266` and its test; `Makefile:88-89,636-640`
  (host-package-versions targets); the stale comment at `.github/workflows/release-gates.yml:269`;
  `docs/experiments/v1-0607/qualify.py:140` (experiment history, kept or annotated).
- Kept in `beamfall/corvint`: `integrations/instruction-profiles.json`, read by
  `internal/contextindex/instruction_loadset_test.go:150`. It moves to a non-`integrations/` path or
  stays where it is; the lane decides and records it.
- Reused from `841a58cbd^` only: the host event mappings, the payload fixtures and the qualification
  harness (G20). The shared event layer is not restored.
- Qualification uses the owner's subscription logins on the owner's machines (owner decisions R39
  and R40, 2026-11-09); no agent API keys and no CI spend are needed. Earlier wording, superseded:
  "CI spend and agent API keys for qualification (G21), not yet arranged by the owner."

## Verified at this head and assumed

Read in the integration checkout (`scratchpad/integ`, head `9ec3a29d0`) on 2026-11-09:

- No `corvint agent` or `corvint agents` verb exists. `corvint mcp` exists, and the MCP catalog
  includes `corvint.affected` (`internal/capcatalog/mcp.go:39`). `review` is a repository-guidance
  command (`cmd/corvint/main.go:646`, `cmd/corvint/repository_guidance.go:132-147`).
- `CORVINT_HOOKS` appears nowhere in code.
- `adopt --setup` detects `.claude/settings.json` and `.codex/hooks.json` read-only
  (`cmd/corvint/adopt_setup.go:39`).
- `integrations/instruction-profiles.json:17` lists `"unconfirmedHosts": ["claude-code",
  "gemini-cli","opencode","pi"]`; only Codex's instruction loading is sourced (rust-v0.153.2).
- Commit `841a58cbd` (2026-11-09, "Remove the host hook adapters, MCP plugin wiring, host lifecycle
  qualifier and the Gemini, OpenCode and Pi packages (decision 0594)") changes 209 files with 27,208
  deletions. The panel brief said 133 files and 16,800 lines; that count may cover only part of the
  commit.
- `docs/EXTENSION-ECOSYSTEM.md:123` says the OpenCode plugin API is beta and a hook failure can fail
  the intercepted operation; `:125` says to add Pi only after the other adapters pass and that Pi
  packages execute with full system access.

Upstream host behaviour, read 2026-11-09. Anything not listed as read is NOT_VERIFIED.

| Host | Source read on 2026-11-09 | What was confirmed |
| --- | --- | --- |
| Claude Code | https://code.claude.com/docs/en/hooks | Events include `SessionStart`, `PostToolUse`, `PostToolBatch`, `Stop`, `PreCompact`, `PostCompact`, `SessionEnd`, `FileChanged`. Default command timeout 600 s; `SessionEnd` budget 1.5 s. `PostToolUse`, `PostCompact`, `SessionStart`, `SessionEnd` cannot block; `Stop` and `PreCompact` can. Plugins bundle `hooks/hooks.json`. |
| Codex | https://learn.chatgpt.com/docs/hooks (redirected from developers.openai.com/codex/hooks) | Events include `PreToolUse`, `PostToolUse`, `PreCompact`, `PostCompact`, `Stop`, `SubagentStop`, `SessionStart`, `SessionEnd`, `UserPromptSubmit`, `Interrupt` and others. Configured in `hooks.json` or `[hooks]` in `config.toml` under `~/.codex` or `<repo>/.codex`. Project hooks load only for a trusted project; non-managed hooks must be reviewed and trusted through `/hooks`. Timeout in seconds, default 600; `SessionEnd` and `Interrupt` default 1 s, maximum 3 s. `Stop` can return a block decision. Plugins bundle hooks through `hooks/hooks.json` or a `hooks` entry in `.codex-plugin/plugin.json`. Hooks are on by default; `[features] hooks=false` disables them. |
| Codex | https://learn.chatgpt.com/docs/extend/mcp?surface=cli | `[mcp_servers.<name>]` in `config.toml` with `command`, `args`, `env`, `env_vars`, `cwd`; `codex mcp add`. Plugins can declare MCP servers; user config controls them under `plugins.<plugin>.mcp_servers.<server>`. |
| Codex | https://learn.chatgpt.com/docs/build-skills.md | Skills load from `$CWD/.agents/skills`, `$CWD/../.agents/skills`, `$REPO_ROOT/.agents/skills`, `$HOME/.agents/skills`; plugins can include skills. |
| Codex | https://learn.chatgpt.com/docs/plugins | Plugins bundle skills, MCP servers and hooks; installed through `/plugins` inside Codex from a configured marketplace. |
| OpenCode | https://opencode.ai/docs/plugins/ | `plugins` array in `opencode.json(c)`; auto-discovery in `.opencode/plugins/` and `~/.config/opencode/plugins/`; `opencode plugin add`, `list`, `check`, `update`, `remove`; plugins are `.ts` or `.js` modules. |
| OpenCode | https://opencode.ai/docs/en/plugins/ | Event names include `tool.execute.after`, `file.edited`, `session.idle`, `session.created`, `session.compacted`, `experimental.session.compacting`. This page and the one above differ; the config key spelling is re-checked at build. |
| OpenCode | https://opencode.ai/docs/mcp-servers/ | MCP servers in `opencode.jsonc` with `type: "local"` and a `command` array, or `type: "remote"` and `url`. |
| OpenCode | https://opencode.ai/docs/skills/ | Skills load from `~/.config/opencode/skills`, `~/.claude/skills`, `~/.agents/skills`, `.opencode/skills`, `.claude/skills`, `.agents/skills`; `SKILL.md` with `name` and `description`. |
| Pi | https://raw.githubusercontent.com/earendil-works/pi/refs/heads/main/packages/coding-agent/docs/extensions.md | Extensions in `~/.pi/agent/extensions/` and `.pi/extensions/` (the latter only after project trust), or settings `packages` / `extensions`. Events include `session_start`, `session_compact`, `session_shutdown`, `agent_end`, `agent_settled` (the final idle signal; `agent_end` is not), `turn_end`, `tool_call` (can block), `tool_result` (can modify), `tool_execution_end`. `pi.registerTool()`. An extension runs inside the Pi process with the same operating-system permissions. Pi has a built-in MCP client: `mcp.json` `mcpServers` and `pi.registerMcpServer()`. |
| Pi | https://raw.githubusercontent.com/earendil-works/pi/refs/heads/main/packages/coding-agent/docs/skills.md | Skills in `~/.agents/skills/` and `.agents/skills/`; `SKILL.md` frontmatter `name` (at most 64 characters, lowercase) and `description` (at most 1024). |
| Pi | https://raw.githubusercontent.com/earendil-works/pi/refs/heads/main/packages/coding-agent/docs/packages.md | `pi install npm:<package>@<version>`; `package.json` `pi` keys `extensions`, `skills`, `prompts`, `themes`; packages can execute extension code. The repository moved from `badlogic/pi-mono` to `earendil-works/pi`; npm package `@earendil-works/pi-coding-agent`; Node.js 22.19 or later per the README. |

NOT_VERIFIED, to be checked by the lane before the dependent requirement is built:

- Codex: a non-interactive plugin install command (AGT-V1-008 depends on it); whether
  `PostToolUse` fires for `apply_patch` edits and carries the edited paths; the exact `Stop` and
  `PostToolUse` message fields that reach the model or the user; whether plugin-bundled hooks also
  need `/hooks` trust review.
- Claude Code: plugin bundling of MCP servers and skills was not re-read on 2026-11-09.
- OpenCode: how a plugin delivers a message to the agent or user at `session.idle`; whether a plugin
  can register an MCP server.
- Pi: whether the MCP client is enabled by default; skill directories beyond the two above; how
  `pi install` treats npm install scripts.

Host event mapping proposed for the generated shims (subject to the items above):

| Bar item | Claude Code | Codex | OpenCode (preview) | Pi (preview) |
| --- | --- | --- | --- | --- |
| After edit | `PostToolUse` | `PostToolUse` | `tool.execute.after` or `file.edited` | `tool_result` or `tool_execution_end` |
| End of turn | `Stop` (block decision with reason, R5) | `Stop` (block decision with reason, R5; the reason reaching the model is NOT_VERIFIED) | `session.idle` | `agent_settled` |
| Session start | `SessionStart` | `SessionStart` | `session.created` | `session_start` |
| After compaction | `SessionStart` with source `compact` (R5) | `SessionStart` with source `compact` (R5; NOT_VERIFIED) | `session.compacted` | `session_compact` |

The compaction row said `PostCompact` for Claude Code and Codex before owner decision R5
(2026-11-09); `PostCompact` output does not reach the model (review fix F18), so it is superseded.

## Non-goals

- Injecting retrieved repository context, summaries or code into prompts (2026-10-26 finding; G19).
- Running tests from a hook, or blocking an edit or a tool call. The only block decision is the
  end-of-turn `Stop` decision of AGT-V1-013, at most once per turn (owner decision R5, 2026-11-09).
  Earlier wording, superseded by R5: "No shim returns a block decision, even where the host allows
  it on `Stop`."
- Restoring the deleted `corvint-harness-event/0` profile, the shared event layer, the FALLBACK and
  FULL host vocabulary, or the nine HLQ cases.
- Gemini CLI or any agent other than the four named.
- Windows native (answer 16).
- Any network call from the binary or the shims at hook time (invariant 7).
- Writing to a Corvint Tasks store from a hook. Task claims happen only through the agent's explicit
  use of the `corvint-tasks` CLI. There is no Tasks MCP server in 1.0 (owner decision R17,
  2026-11-09); earlier wording said "the Tasks MCP or CLI".
