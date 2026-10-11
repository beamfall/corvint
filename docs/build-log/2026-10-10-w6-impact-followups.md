# Wave 6 impact follow-ups: affected freshness, require-clean coverage, MCP worktree notice

Date: 2026-10-11 (wave 6 lane). Branch `claude/w6-impact` from `origin/codex/w5-impact`
(e744610cbff7528210065339f23b2c1cef4bb87f). Tickets: V1-1141 (GitHub #783), V1-1174,
V1-1173 (GitHub #781). V1-1172 (per-call root) was deliberately not implemented: owner decision
pending.

- V1-1141: `affected` warns when a `--base` range path lies outside a nonempty dirty set
  (`AFP-V0-044`) and `--require-clean` refuses a dirty worktree with exit 2 and
  `unsupported-affected-mixed-worktree` before range, graph or provider work (`AFP-V0-045`), on
  the /0, /1 compact and Playwright receipts.
- V1-1174: every dispatched verb honours (`impact`, `affected`) or refuses `--require-clean`
  (`DIRTY-CACHE-016`). The test runs each verb in a valid form with and without the flag and
  requires the flag to cause the refusal. Probing found `adapter codex` silently ignored trailing
  arguments; it now degrades with `unsupported-hook-event`, like `adapter claude-code`. The
  ticket's claim about `feature` did not reproduce: it already refused.
- V1-1173: corvint-mcp appends a closed, path-free `corvint-mcp-worktree-notice/0` text block to
  routed tool results and tool errors when the launch directory (read once at startup) is a
  different linked worktree of the bound root's repository (`MMR-V0-009`). Routing, the first
  block, structuredContent and authority are unchanged; client roots are ignored, and status-all
  and protocol errors carry no notice.

All four requirements are proposed, pending owner acceptance (V1-1141, V1-1174, V1-1173).
Implementation remains experimental.

Independent review: Codex `gpt-6.1-sol`, read-only. Round 1 FAIL raised one blocking finding (the
enumeration test did not prove flag causation) and two non-blocking ones (no Playwright witness;
MCP tool error not routed). All three were fixed. Round 2 PASS; its P3 wording note (adapter
exception scoped to hook forms) was applied.

Verification ran focused `cmd/corvint` tests (affected, impact, prove, adapter, require-clean),
`cmd/corvint-mcp`, `conformance/mcp-2026-07-28`, `go vet` on both commands, the doc gates and the
receipt repin check. `make gate` and `./...` were `NOT_RUN` under the lane rules.

Limits:
- The MCP notice depends on the launch cwd. A host that launches from a neutral directory gets no
  notice.
- `adapter` and `obligations` are dispatched only without a leading `--root`.
