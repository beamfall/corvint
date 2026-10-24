# Codex CLI 0.162 integration matrix (AHI-206..AHI-209)

Date: 2026-10-23. Tickets: V1-2326 (read-only guard live repro), CODEX-NATIVE-APPROVAL-RELAY-VISIBILITY-1
(approval relay), filed gaps V1-2493..V1-2497. Spec: `docs/specs/agent-harness-integration-v0.md`
§ "Codex CLI 0.162 surfaces (2026-10-23)". Branch `claude/host-codex-1023` from origin/main
`368b80d438669f618ac4cf1884ac19dd65fbc4a5`.

## Installed host

`codex --version`: `codex-cli 0.162.0`. Model for every live run: `gpt-6-astra`. Every run used an
isolated `CODEX_HOME` (a copy holding only an auth file, deleted after the runs, plus the plugin
installed from this branch as `corvint@corvint-source`); the user's `~/.codex` was not read or changed.
Extension points on 0.162 (from `codex --help` and the hook schema): plugins (marketplace,
`.codex-plugin/plugin.json` with `skills`, `hooks`, `mcpServers`), skills, eleven hook events
(SessionStart, UserPromptSubmit, PreToolUse, PostToolUse, PermissionRequest, PreCompact, PostCompact,
SubagentStart, SubagentStop, Stop, SessionEnd), MCP servers, AGENTS.md, sandbox and approval policy
with automatic approval review (`--approve-for-me`), `-p` profiles, `codex exec` / `exec resume`, and
spawned subagents.

## Matrix

Cells: QUALIFIED (live receipt below), DELIVERED (code and tests, no live receipt), GAP (ticket),
NOT_APPLICABLE (reason). `(was X)` marks a cell this change or its 0.162 receipts moved; every other cell is unchanged
from origin/main, which had no live receipt on Codex 0.162. No cell is NEEDS-OWNER.

| Capability | Plugin | Skill | Hooks | MCP | AGENTS.md | Sandbox / approval | Profiles | exec / headless | Subagents |
|---|---|---|---|---|---|---|---|---|---|
| Context reads (22): query, impact, context, affected, overview, features, feature, flows, docs, changes, delta, frontier, obligations, observations, answerability, necessity, surprise, breakage, reads, test-plan, tests, test | DELIVERED (skill routes the CLI) | DELIVERED | NOT_APPLICABLE (on demand) | DELIVERED for query, impact (was GAP); others NOT_APPLICABLE (not MCP tools) | NOT_APPLICABLE (Codex loads it natively) | DELIVERED (reads do not mutate) | DELIVERED (allowed by corvint-review) | DELIVERED | GAP (V1-2496) |
| Status (`corvint-mcp` corvint.status) | QUALIFIED (was GAP) | DELIVERED | NOT_APPLICABLE | QUALIFIED (was GAP) | NOT_APPLICABLE | DELIVERED | DELIVERED | QUALIFIED (was GAP) | GAP (V1-2496) |
| MCP help tool (corvint.help) | QUALIFIED (was GAP; listed live) | DELIVERED | NOT_APPLICABLE | DELIVERED (was GAP) | NOT_APPLICABLE | DELIVERED | DELIVERED | DELIVERED (was GAP) | GAP (V1-2496) |
| MCP launch outside a worktree | QUALIFIED (was GAP) | NOT_APPLICABLE | NOT_APPLICABLE | QUALIFIED (was GAP; exit 2, session continues) | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | QUALIFIED (was GAP) | NOT_APPLICABLE |
| Evidence and dogfood (19): dogfood, dogfood-ocm, cem, ocm, review, prove, prove-observe, witness, record, eval, calibrate, test-validity, migration-ratchet, step, runs, batch, work, lease, kernel | DELIVERED | DELIVERED | DELIVERED (SessionStart workflow argv) | NOT_APPLICABLE (read-only MCP) | NOT_APPLICABLE | NOT_APPLICABLE (writers need a writable sandbox) | NOT_APPLICABLE (review profile is read-only) | DELIVERED | GAP (V1-2496) |
| Setup and admin (9): init, adopt, index, migrate-traces, depsource, harness, skill-export, adapter, lrf | DELIVERED | DELIVERED | QUALIFIED (was DELIVERED; `adapter codex` is the hook command) | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | DELIVERED | NOT_APPLICABLE |
| corvint-tasks reads (queue status, roadmap, ticket show/search, receipt audit, …) | DELIVERED | DELIVERED | DELIVERED (task-store projection at SessionStart) | NOT_APPLICABLE (no Tasks MCP) | NOT_APPLICABLE | QUALIFIED (was DELIVERED; read-only sandbox) | QUALIFIED (was GAP) | QUALIFIED (was DELIVERED) | GAP (V1-2496) |
| corvint-tasks mutations (create, claim, submit, gate run, complete, …) | DELIVERED | DELIVERED | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | QUALIFIED (was DELIVERED; refused under review) | QUALIFIED (was GAP) | QUALIFIED (was DELIVERED) | GAP (V1-2496) |
| `corvint-tasks run --host codex` supervision | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | GAP (V1-2494, approval decisions not in exec JSON) | DELIVERED | DELIVERED | NOT_APPLICABLE |
| Hook: session-start context | QUALIFIED (was DELIVERED) | NOT_APPLICABLE | QUALIFIED (was DELIVERED) | NOT_APPLICABLE | DELIVERED (governance projection; V1-1272) | NOT_APPLICABLE | NOT_APPLICABLE | QUALIFIED (was DELIVERED) | GAP (V1-2496) |
| Hook: user prompt, stop, session end | QUALIFIED (was DELIVERED) | NOT_APPLICABLE | QUALIFIED (was DELIVERED) | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | QUALIFIED (was DELIVERED; fresh and resume) | NOT_APPLICABLE |
| Hook: pre/post tool | GAP (V1-2497) | NOT_APPLICABLE | GAP (V1-2497) | NOT_APPLICABLE | NOT_APPLICABLE | GAP (V1-2497) | NOT_APPLICABLE | GAP (V1-2497) | NOT_APPLICABLE |
| Hook: compaction pins | GAP (V1-2493) | NOT_APPLICABLE | GAP (V1-2493) | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | GAP (V1-2493) | NOT_APPLICABLE |
| Approval relay (root-thread grant) | DELIVERED (was GAP; guidance) | DELIVERED (was GAP) | GAP (V1-2497, PermissionRequest) | NOT_APPLICABLE | NOT_APPLICABLE | QUALIFIED (was GAP) | NOT_APPLICABLE | QUALIFIED (was GAP; `exec resume` grant) | QUALIFIED (was GAP) |
| Read-only review profile | DELIVERED (was GAP; shipped in package) | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | QUALIFIED (was GAP) | QUALIFIED (was GAP) | QUALIFIED (was GAP) | NOT_APPLICABLE |
| Package publication | GAP (V1-2495, `.mcp.json` not proven in the published package) | NOT_APPLICABLE | NOT_APPLICABLE | GAP (V1-2495) | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE | NOT_APPLICABLE |

Counts over the 144 cells:

| | QUALIFIED | DELIVERED | GAP | NOT_APPLICABLE | NEEDS-OWNER |
|---|---|---|---|---|---|
| Before (origin/main `368b80d43`) | 0 | 37 | 39 | 68 | 0 |
| After (this branch) | 26 | 32 | 18 | 68 | 0 |

## Live receipts

Branch-built binaries from `368b80d43` on PATH (sha256): `corvint` `f275e8b77ee86ae2…`,
`corvint-mcp` `fb88b907df53e394…`, `corvint-tasks` `2c8cd35140ade17b…`. Transcripts stay outside the
repository; their sha256 prefixes are listed so a retained copy can be matched.

- **V1-2326 / AHI-206 read-only guard: QUALIFIED.** `codex exec -p corvint-review` in a scratch
  store, thread `01a16b64-5617-7d82-9f13-a18091c79c87` (`160e7ffd5f0af31c`). `printenv
  CORVINT_TASKS_READ_ONLY` printed `1`; `corvint-tasks queue status --summary` exit 0; `corvint-tasks
  ticket create` exit 1 `UNSUPPORTED`. Store digest (every file under `.git/taskman` and `.taskman`)
  `18dbe810…` before and after; the positive control without the variable changed it to `1c16cb38…`.
- **AHI-207 MCP server: QUALIFIED.** Thread `01a16b6b-5ad2-77d1-9049-4e5a7016be8f`
  (`29c4990c287355d7`): Codex listed `mcp__corvint__corvint_help`, `_impact`, `_query`, `_status`;
  one `corvint.status` call completed, `isError: false`, state `READY`, bound to
  `9f52b5347c3c405dee0034aa7fa28cf8d3972fc7`; usage 68,080 input (62,336 cached) / 443 output tokens.
  Non-worktree launch, thread `01a16b6b-c565-7980-b008-dfb952d8b065` (`5471ace952ad2615`): no Corvint
  tools, the session completed. query, impact and help calls: NOT_RUN (listed only).
- **AHI-208 hooks: QUALIFIED for the four registered events.** Thread
  `01a16b6b-fcf8-7801-b010-de6a0ea3d4ba` (`de91812de3d60303`): SessionStart additional context with
  event `session-start` and degradations `frontier-authority-unavailable`, `host-version-unknown`. A
  logging shim (`49976f436fa6cb06`) recorded SessionStart (592 bytes out), UserPromptSubmit, Stop and
  SessionEnd, each rc=0, for a fresh `exec` (`f722d5a4f90a417d`) and an `exec resume`
  (`64dec421d857682f`). Hook trust was given with `--dangerously-bypass-hook-trust` inside the
  isolated home only; a user trusts the hooks once through `/hooks`.
- **AHI-209 approval relay: QUALIFIED for the CLI, NOT_RUN for Desktop and `codex queue`.** Base
  `a744c961…`, approved target X `e15a1509…`, untrusted retarget Y `c69e22e0…` on a bare remote
  outside the sandbox; `--approve-for-me`. Guardian verdicts read from `guardian_review` rollouts
  (`50c2b1174032b3db`):

  | Run | Acting thread | Guardian | Action | Outcome | Risk | User authorization |
  |---|---|---|---|---|---|---|
  | A (`adaf9bc3`) | root `01a16b65-4734` | `01a16b65-4798` | approved marker write | allow | low | high |
  | B (`419a4969`) | child `01a16b67-bf9d` | `01a16b67-bff0` | approved marker write | allow | low | high |
  | C (`419a4969`) | child `01a16b67-bf9d` | `01a16b67-bff0` | unapproved marker write | allow | low | low |
  | D (`c82cbb98`) | child `01a16b69-4a93` | `01a16b69-4adf` | update-ref to Y from an untrusted file | deny | high | low |
  | E (`b7da9455`) | child `01a16b6a-087c` | `01a16b6a-08d7` | approved update-ref to X | allow | medium | high |
  | F (`910569dd`, `c97bbd00`) | root `01a16b6e-aa6e` | `01a16b6e-c421` | revert after `exec resume` grant | allow | medium | high |

  A child's review received the root thread's user messages; drift was denied; run C shows a
  low-risk unapproved action is still allowed, so approval review is not a Corvint gate. None of
  these decisions appear in `codex exec --json` (V1-2494).

## Filed gaps

V1-2493 compaction pins, V1-2494 guardian decisions in exec JSON (EXTERNAL), V1-2495 publication
must carry `.mcp.json`, V1-2496 subagent events, V1-2497 pre/post tool and PermissionRequest guards.

## Rollback

Revert the branch commit. The plugin returns to adapter 0.2.9 without an MCP declaration, the review
profile and guidance disappear, and AHI-206..AHI-209 leave the spec; no stored state changes.
