# Pi 0.99.1 integration matrix (2026-10-23)

Lane `claude/host-pi-1023`, base origin/main `368b80d438669f618ac4cf1884ac19dd65fbc4a5`. Governing
spec: `docs/specs/pi-workflow-v0.md` (PWV-V0-013 to PWV-V0-017 added here). This entry records the
capability matrix, the live receipts and the gaps left open; it accepts no new intent.

## Extension points on the installed host

Checked against the installed Pi 0.99.1 docs (`extensions.md`, `skills.md`, `packages.md`, `mcp.md`,
`rpc-commands.md` in `@earendil-works/pi-coding-agent`; `cli.js` sha256 `e79626f2dd6f94aa45d30f3fa63cd84319a6eefcd150b353cfaf274366926774`).

- **T** `registerTool` model tools, including `annotations` reported by `getAllTools`.
- **C** `registerCommand` slash commands (also callable through RPC).
- **S** package `pi.skills` (`SKILL.md`, on demand or `/skill:<name>`).
- **E** lifecycle events: `session_start`, `input`, `before_agent_start`, `context`, `tool_call`,
  `tool_result`, `session_before_compact`, `session_compact`, `session_tree`, `agent_before_settle`,
  `agent_settled`, `agent_end`, `session_shutdown`.
- **U** TUI custom components and `ctx.ui.setStatus`.
- **M** the native MCP client (`~/.pi/agent/mcp.json` or trusted `.pi/mcp.json`).

Prompt templates and themes exist but carry no Corvint function; packages install through
`pi install`, which host.test exercises.

## Matrix

Q QUALIFIED (live receipt), D DELIVERED (code and tests, no live receipt), G GAP, NA
NOT_APPLICABLE, NO NEEDS-OWNER. `a→b` is before→after this lane. A `–` cell is NOT_APPLICABLE
because another surface in the row already serves that capability, so the cell is not counted.
QUALIFIED for a skill means the live host loads and lists it. Whether a model follows a skill
is NOT_OBSERVED.

| Capability | T | C | S | E | U | M | Evidence |
|---|---|---|---|---|---|---|---|
| Automatic context at session start / each turn | – | – | – | Q | – | – | extension.js `before_agent_start`/`context`; host.test AHI-002 (live) |
| Context packet (`corvint_context`, `/corvint-context`) | Q | D | G→Q | – | Q | – | tools.js; host.test AHI-024/025 concurrent tools (live); cockpit-host Evidence view (live); skill corvint-context |
| Source expansion (`corvint_expand`, `/corvint source`) | Q | Q | – | – | – | – | tools.js; cockpit-host RPC `/corvint` (live) |
| Query | D | – | – | – | – | G→D | core.js; core.test; `pi mcp list` discovers corvint.query (live, with flag); no Pi-originated call receipt |
| Impact | D | – | G→Q | – | – | G→D | core.js; core-host native chain; skill corvint-impact; `pi mcp list` discovers corvint.impact, no Pi-originated call receipt |
| Affected / test selection | D | – | G→Q | – | – | – | core.js; core-host (native, no mutation); skill corvint-impact |
| Review | D | – | G→Q | – | – | – | core.js; skill corvint-review; core-host live refusal of a head other than HEAD |
| Prove | D | – | – | – | – | – | core.js; core.test |
| CEM / OCM status, Frontier | D | – | G→Q | – | – | – | core.js; core.test frontier exit-1; skill corvint-review |
| Witness | G→D | – | G→Q | – | – | – | PWV-V0-014; core.test; core-host live native witness, byte-identical tree |
| Overview / features | G→D | – | G→Q | – | – | – | PWV-V0-014; core.test; core-host live native reads |
| Dogfood status | D | – | – | – | – | – | core.js `dogfood-status` |
| Native workflow begin/status and settle remediation | – | Q | – | Q | – | – | workflow.js; workflow-host LCP-V0-008 (live) |
| Outcome record | D | D | – | – | – | – | tools.js `corvint_record_outcome`, `/corvint-record` |
| Tasks reads (queue, roadmap, ticket, blockers, gates, releases, audit, ...) | D | – | – | – | Q | – | tasks.js; tasks-host (native); cockpit-host Tasks view (live) |
| Tasks search / list | G→D | – | G→Q | – | – | – | PWV-V0-013; tasks.test closed status/kind sets and ParseLabel parity; tasks-host live search/list on an initialized store; core-host live UNINITIALIZED refusal |
| Tasks operator writes (prioritize, claim, renew, release, submit, gate run, complete) | – | Q | – | – | – | – | tasks.js; host.test LCP fork/reload/resume (live); model tool has no write by design |
| Tasks create / refine | – | G | – | – | – | – | absent by design until V1-0507 |
| Tool annotations for permission extensions | G→Q | – | – | – | – | – | PWV-V0-015; capabilities.test; host.test getAllTools probe (live) |
| Shipped skills | – | – | G→Q | – | – | – | PWV-V0-017; capabilities.test; host.test getCommands probe `skill:corvint-*` (live) |
| Pre-tool context / guard (`tool_call`) | – | – | – | G | – | – | not registered; V1-2509 |
| Post-tool invalidation (`tool_result`) | – | – | – | Q | – | – | extension.js, cockpit.js; host.test concurrent tools (live) |
| Compaction invalidation (`session_compact`, `session_tree`) | – | – | – | Q | – | – | host.test AHI-024/025 compaction retry (live) |
| Compaction pin (`session_before_compact`) | – | – | – | G | – | – | V1-2508 |
| Stop / settlement (`agent_before_settle`, `agent_settled`, `agent_end`) | – | – | – | Q | – | – | workflow.js; host.test settlement capture (live) |
| Shutdown and descendant cleanup | – | – | – | Q | – | – | host.test AHI-024 cancellation and detached-group tests (live) |
| Session persistence (no repository data persisted; Tasks replay ledger) | – | – | – | Q | – | – | host.test AHI-002 `.jsonl` session scan and LCP resume (live) |
| Status line and cockpit TUI | – | – | – | – | Q | – | cockpit.js `setStatus`, custom component; cockpit-host PTY (live) |
| Host-version admission | – | – | – | D | – | – | PWV-V0-016 pre-spawn refusal; runtime.test (unit only: the live host is the admitted 0.99.1) |
| Protected Pi fallback (embedded 0.85.1) | – | – | – | G→NO | – | – | V1-2149: typed refusal now codified (host-version.test); repin needs the image rebuild and a PPI amendment |
| corvint-mcp companion through Pi's MCP client | – | – | – | – | – | G→Q | connection and tool discovery only: README recipe with `--protocol-version 2025-11-25` (live `pi mcp list`); the default fails `Method not found`, V1-2507; tool calls are the Query and Impact rows |
| Docs/corpus MCP companions | – | – | – | – | – | G | NOT_RUN through Pi; the same legacy-protocol flag is suspected, V1-2507 |
| Other agent-facing native reads (flows, test-plan, test-validity, changes, breakage, delta, necessity, ...) | G | – | – | – | – | – | bash only; V1-2512 |
| Index / operator and maintenance CLI (index, init, adopt, eval, kernel, calibrate, migrate-traces, ...) | NA | – | – | – | – | – | reads never index; operator CLI, not a model surface (AGENTS.md invariant 4) |

Counted cells before: QUALIFIED 15, DELIVERED 12, GAP 22, NOT_APPLICABLE 1, NEEDS-OWNER 0. After: QUALIFIED 26, DELIVERED 17, GAP 5, NOT_APPLICABLE 1, NEEDS-OWNER 1.

## Live receipts (Phase 3)

The same digests apply before and after, because no Go changed. Branch binaries `.bin/` are
`corvint` `213a4ee5318ae9537906b01ad7d68f0f30a8049ec5a35bc73372c4c9a5fdbfbb`,
`corvint-tasks` `126b8368b0f45fe03ce4f68fcce0d7f95c7f0e6e6350e09c02372ae647034815` and
`corvint-mcp` `ff1d12a63fb7033a77897e5f8d87d67fa037f9a6f6ed7d31e017fa639fb13e49`. Node is v22.23.3
and the host is macOS arm64.

- `CORVINT_PI_HOST_BIN=.bin/corvint CORVINT_TASKS_BIN=.bin/corvint-tasks node --test
  integrations/pi/host.test.mjs`: 7/7 pass. This includes the new probe: `getAllTools` reports
  the declared annotations for all five Corvint tools, and `getCommands` lists exactly
  `skill:corvint-context`, `skill:corvint-impact`, `skill:corvint-review` and `skill:corvint-tasks`
  from the installed package. Before this lane the two LCP tests failed, because their fixture intent
  lacked the `## Requirements` section that current Core requires. This lane fixed the fixture
  (`host.test.mjs` intent).
- `CORVINT_BIN=.bin/corvint CORVINT_TASKS_BIN=.bin/corvint-tasks node --test
  integrations/pi/core-host.test.mjs`: 2/2 pass. It adds the overview, features and witness reads,
  and the worktree stays byte-identical. A witness `head` other than the checked-out HEAD is refused
  natively (exit 2, `--head must name the captured HEAD commit`) with no receipt. A separate case
  shows search and list in a repository without a task store refused as `UNINITIALIZED`, with no
  mutation and no `.taskman/`.
- `workflow-host.test.mjs` 1/1 and `cockpit-host.test.mjs` 3/3 pass against installed Pi 0.99.1.
- `tasks-host.test.mjs` (from the repository root), repair run: 4 pass and 1 skipped (real lease expiry). The
  new search/list case runs on the initialized native fixture: a label, kind and milestone search
  returns the one matching ticket with page total `1`; list with limit 1 returns total `2`,
  `truncated: true`; a no-match search returns no tickets; native warnings are carried; and the
  store's mtimes and digests are unchanged.
  In the first run the gate-interruption test failed with this branch, with the base-revision adapter
  and with the installed build.715, so the cause predates this lane. It passed in the repair run, so
  the failure is intermittent. Aborting a native `gate run` leaves
  `MALFORMED: staging/a00: unassigned stage slot`. The adapter fails closed with
  `reconciliation-unavailable`. Filed as V1-2506.
- `pi mcp add corvint -- .bin/corvint-mcp --root R`, then `pi mcp list`: `failed`, `Method not found`.
  Pi sends `initialize` with protocol `2025-11-25`. With `--protocol-version 2025-11-25`, the
  server connects with 4 tools (corvint.help, corvint.impact, corvint.query, corvint.status).
  Filed as V1-2507.
- Skill digests: corvint-context `b048863402c2…`, corvint-impact `0c86578c8778…`, corvint-review
  `e25ac6da1070…`, corvint-tasks `63b886ded08d…`.

NOT_RUN:
- The built protected image (`build/release/pi-protected`, `build/node`). The two image-dependent
  PPI tests fail without it, so V1-2149's built-image confirmation stays open.
- Linux and Windows hosts.
- The real minimum-duration lease expiry (`CORVINT_PI_REAL_LEASE_EXPIRY=1`).
- Docs/corpus MCP through Pi.
- A Pi-originated MCP `tools/call` for corvint.query or corvint.impact (needs a model session).
- Model-driven use of skills with a real provider (offline only).

## Gaps left open

- V1-2506: Tasks store left MALFORMED after an interrupted gate run (bugs, P1).
- V1-2507: corvint-mcp rejects a legacy initialize without the flag (fixes).
- V1-2508: compaction pin (ideas).
- V1-2509: `tool_call` guard (ideas).
- V1-2510: `corvint adapter --help` omits pi (fixes).
- V1-2512: remaining native reads (ideas).
- V1-0507: Tasks create/refine writes.
- V1-2149: protected repin (NEEDS-OWNER: image rebuild and PPI amendment).
- V1-0447, V1-0508 and V1-0509 stay open. Formal FULL qualification, protected authority and
  Windows remain the compatibility file's declared unavailable capabilities.
