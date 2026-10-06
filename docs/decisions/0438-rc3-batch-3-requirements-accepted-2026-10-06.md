# Decision 0438 — rc3 batch 3 requirements accepted

Date: 2026-10-06. Status: accepted by the owner on 2026-10-06 ("accept the proposed requirement IDs",
in reply to the orchestrator's batch 3 report). Covers beamfall/corvint#640.

## Context

rc3 batch 3 delivered seven requirements, each drafted by a delivery lane as proposed intent and
reviewed by Codex on its own branch and again over the integrated range. AGENTS.md invariant 8 keeps
acceptance human-owned, so they stayed proposed until the owner answered.

## Decision

The owner accepts these requirements as written in their specs at the batch 3 head:

| Requirement | Spec | Ticket | Summary |
|---|---|---|---|
| `AFP-V0-031` | `docs/specs/affected-plan-v0.md` | V1-0867 | The Go nested-module frontier is decided per unlisted module and closes only for a module that neither builds against nor is built by an observed module |
| `AFP-V0-032` | `docs/specs/affected-plan-v0.md` | V1-0865 | A mandatory declaration path that exists but is not a readable regular file is `MANDATORY_DECLARATION_UNREADABLE`, never `NO_REPOSITORY_GATE_DECLARED`, and is read without blocking |
| `AFP-V0-033` | `docs/specs/affected-plan-v0.md` | V1-0868 | Every unbounded-reader unit names its unbounded read; a read-scope declaration needs identical confined and unconfined per-test outcomes |
| `CAL-V0-114` | `docs/specs/corvint-tasks-agent-leases-v0.md` | V1-0864 | With several journal-audit faults of one kind, the refusal names the byte-smallest path every run |
| `CAL-V0-120` | `docs/specs/corvint-tasks-agent-leases-v0.md` | V1-0625 | Optional policy `holderLiveness.heartbeatTTLSeconds` drives the derived holder status (amendment A24) |
| `CAL-V0-121` | `docs/specs/corvint-tasks-agent-leases-v0.md` | V1-0625 | `STALE_HOLDER` is advisory input to a coordinator's evidence handoff, never release authority |
| `IDX-SNAP-V0-025` | `docs/specs/index-snapshot-v0.md` | V1-0870 | Snapshot eviction ranks the writing engine's live-worktree trees ahead of its others, within the unchanged entry and byte bounds, and the receipt names every removed file |

`AFP-V0-033` was drafted as `AFP-V0-028`. It was renumbered during integration because decision 0431
holds `AFP-V0-028` to `AFP-V0-030`.

## Limits

This settles intent only. Delivery status is unchanged: each spec keeps its own delivery and
qualification wording, and the evidence recorded in the batch remains as it is. That includes live
fleet qualification NOT_RUN for the CAL requirements, Linux-only read-scope measurement for
`AFP-V0-033`, and `make gate` NOT_RUN.

## Rollback

Revert this decision and restore the "proposed" markers in the three specs, `docs/specs/README.md`
and `docs/specs/INDEX.json`.
