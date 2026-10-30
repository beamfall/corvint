# Decision 0519 — w6 batch requirements accepted

Date: 2026-10-11. Status: accepted under the owner's standing auto-accept (2026-10-11). Authority:
the owner's standing auto-accept of lane-proposed requirements given in the orchestrator chat on
2026-10-11 ("auto accept. I just do that, so you may as well"), recorded in decision 0518, plus the
owner's delegation of in-task forks to the orchestrator (2026-10-05) for the two declined requests
below.

## Context

The w6 batch lands twelve w6 lane branches and the V1-0906 branch on top of the w5 batch (pull
request #795): tasks-doctor (V1-1176), flakes (V1-1152..V1-1154), obligation-alias (V1-1177),
playwright-grep (V1-1175), witness-help (V1-1196), harness-reason (V1-1157), impact (V1-1141,
V1-1173, V1-1174), docs (V1-1155, V1-1156, V1-1158), appmap-nav (V1-1159..V1-1161, V1-1165),
appmap-impact (V1-1162, V1-1163, V1-1166, V1-1167, V1-1169, V1-1170), dispatch-idle (V1-1195),
dogfood (V1-1151, V1-1171) and `codex/v1-0906-bounded-fds` (V1-0906). Each lane wrote its new or
amended requirements as "proposed, pending owner acceptance".
`CAL-V0-215` (V1-1129) came with the w5 batch and stayed proposed there (decision 0518). AGENTS.md
invariant 8 keeps acceptance human-owned; the standing auto-accept is that human acceptance.

## Decision

These requirements are accepted under the owner's standing auto-accept (2026-10-11), as written in
the spec text carried by this batch:

- `TQD-V0-013` (V1-1176) in `docs/specs/corvint-tasks-doctor-v0.md`.
- The `TRE-V0-030` amendment (V1-1152, bounded owner-token wait) in
  `docs/specs/test-runner-execution-v0.md`.
- `AFP-V0-044` and `AFP-V0-045` (V1-1141) and the `AFP-V0-009` advice-member amendment (V1-1167)
  in `docs/specs/affected-plan-v0.md`.
- `DIRTY-CACHE-016` (V1-1174) in `docs/specs/revision-cache-dirty-worktree-v0.md`.
- `MMR-V0-009` (V1-1173) in `docs/specs/mcp-multi-root-v0.md`.
- `UCL1142-V0-006` and `UCL1142-V0-007` (V1-1158) and the amendments to `UCL1142-V0-004` (V1-1155)
  and `UCL1142-V0-005` (V1-1158) in `docs/specs/upgrade-checklist-v0.md`.
- `LPCVD-V0-005` and the `LPCVD-V0-002` amendment (V1-1156), and `LPCV-V0-071` (V1-1196), in
  `docs/specs/live-proof-carrying-verification-v0.md`.
- `TOL-V0-034` through `TOL-V0-036` (V1-1177) in
  `docs/specs/corvint-tasks-obligation-ledger-v0.md`.
- `TJAA-V0-021` and the `TJAA-V0-019` amendment (V1-1175) in
  `docs/specs/typescript-javascript-affected-adapter-v0.md`.
- `ANL-V0-021` through `ANL-V0-024` (V1-1161, V1-1159, V1-1160, V1-1165), `ANL-V0-031` through
  `ANL-V0-034` (V1-1167, V1-1162, V1-1166, V1-1163) and the `ANL-V0-016` amendment (V1-1162) in
  `docs/specs/application-navigation-ledger-v0.md`.
- `AMAP-V0-078` (V1-1169) in `docs/specs/application-map-v0.md`.
- `CAL-V0-218` (V1-1195), `CAL-V0-219` through `CAL-V0-221` (V1-0906), and `CAL-V0-215` with its
  A28 amendment (V1-1129), in `docs/specs/corvint-tasks-agent-leases-v0.md`.
- `DCW-V0-039` and `DCW-V0-040` (V1-1151) and `DCW-V0-041` (V1-1171) in
  `docs/specs/daily-change-evidence-workflow-v0.md`.
- `DCP-V1-053` (V1-1196) in `docs/specs/documentation-corpus-v1.md`.
- `AHI-054` (V1-1157), the stop-frontier `cause` member, in
  `docs/specs/agent-harness-integration-v0.md`, with divergence `DR-0043` in
  `conformance/divergence-register.md`.

Their status text changes from "proposed, pending owner acceptance (<ticket>)" to accepted, citing
this decision.

Two owner-delegated orchestrator decisions are recorded here:

- V1-1172 (a per-call MCP root) is DECLINED. `MCPV0-015` is kept unchanged; the `MMR-V0-009`
  launch-worktree notice is the remedy for a server launched from another linked worktree.
- For V1-1157, the adopter acknowledge/suppress flow for frontier items is DECLINED, because missing
  evidence stays visible (product invariant 2, `AHI-019`, `AHI-022`). Only the `cause` field is
  accepted, as `AHI-054` states.

The owner approved removing the 512-directory descriptor admission caps of the first V1-0906 draft
(2026-10-11); `CAL-V0-219` records that no directory-count admission cap applies. Every other
requirement that was proposed before this batch stays proposed.

## Limits

Delivery stays experimental. Live adopter qualification and Linux remain NOT_OBSERVED. V1-0906
keeps its retained Darwin evidence (five runs each at 1,000 and 100,000 receipts, with measured
descriptor peaks and one-minute load bands, in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-10-v1-0906-bounded-fds.md`);
load envelopes outside the bands the lane build-log entries record remain NOT_OBSERVED. The acceptance admits no ranking, evidence or authority
input beyond what the accepted text states. The standing auto-accept covers only lane-proposed
requirements after review. It never covers classifier-blocked actions, releases, tags, manual
merges, force-pushes, remote-branch deletion or installs.

## Rollback

To withdraw acceptance, revert this decision and the status text. The requirements then return to
"proposed, pending owner acceptance". The implementations are experimental and need no migration.
The two declined requests can be reopened as new tickets.
