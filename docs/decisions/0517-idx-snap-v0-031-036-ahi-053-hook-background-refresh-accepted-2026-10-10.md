# Decision 0517 — IDX-SNAP-V0-031..036 and AHI-053 hook background refresh accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner answer "Accept both, include
them" in the orchestrator chat, 2026-10-11 UTC, to the question whether to accept the requirements
proposed on pull requests #770 and #771 (this decision covers #771, V1-1128).

## Context

V1-1128 lets a Claude or Codex hook that finds a stale index snapshot start one bounded, detached,
single-flight background `index` writer, so the next event reads a fresh snapshot without a manual
warmup. `IDX-SNAP-V0-031` through `IDX-SNAP-V0-036` in `docs/specs/index-snapshot-v0.md` and
`AHI-053` in `docs/specs/agent-harness-integration-v0.md` define it. The accepted `IDX-SNAP-V0-012`
(decision 0422) forbids automatic hook refresh and any detached index child. AGENTS.md invariant 8
keeps acceptance human-owned.

## Decision

The owner accepts `IDX-SNAP-V0-031`, `IDX-SNAP-V0-032`, `IDX-SNAP-V0-033`, `IDX-SNAP-V0-034`,
`IDX-SNAP-V0-035`, `IDX-SNAP-V0-036` and `AHI-053` as written in the spec text carried by this pull
request. Their status text changes from "proposed, pending owner acceptance (V1-1128)" to
"accepted, decision 0517".

This decision explicitly supersedes the no-automatic-refresh / no-detached-child clause of
`IDX-SNAP-V0-012`, for the stale-snapshot hook path only, by `IDX-SNAP-V0-031` through
`IDX-SNAP-V0-036`. The rest of `IDX-SNAP-V0-012` stays accepted under decision 0422.

## Limits

Delivery stays experimental. Real nice priority, live-host latency, Linux and the V1-1120 load
envelope remain NOT_OBSERVED, as the V1-1128 build-log entry records. The exception admits no
daemon, git-hook installer, synchronous hook build, supervision, or ranking, evidence or authority
input. This decision is not a standing rule for other requirements.

## Rollback

Set `CORVINT_INDEX_AUTO_REFRESH=0` to disable the refresh at once. To withdraw acceptance, revert
this decision and the status text; `IDX-SNAP-V0-012` then applies in full again. Snapshots are
derived state and need no migration.
