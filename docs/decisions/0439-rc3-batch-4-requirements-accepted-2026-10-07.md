# Decision 0439 — rc3 batch 4 requirements accepted

Date: 2026-10-07. Status: accepted by the owner. On 2026-10-06 the owner replied "accept" to the
V1-0889 in-place upgrade summary. On 2026-10-07 the owner replied "accept" to the orchestrator's
batch 4 requirement list. Covers beamfall/corvint#641 to #647.

## Context

rc3 batch 4 delivered the writer checkpoint, scale and CPU work, and the dispatcher and pool changes
for issues 641 to 647, together with V1-0881 and V1-0416. Each requirement was drafted by a
delivery lane as proposed intent. Codex reviewed each one on its own branch and again over the
integrated range. AGENTS.md invariant 8 keeps acceptance human-owned. The owner asked for every
remaining proposed batch 4 requirement to be presented together before the batch 4 PR opened.

## Decision

The owner accepts these requirements as written in their specs at the batch 4 head:

| Requirement | Spec | Ticket | Summary |
|---|---|---|---|
| `CAL-V0-115`..`119` | `docs/specs/corvint-tasks-agent-leases-v0.md` | V1-0645 | Derived writer checkpoint. Writes under the lock resume their audit at it, so writer cost stays flat across receipt history. The complete route serves every decline, and a scheduled complete audit refreshes the checkpoint outside the lock. |
| `CAL-V0-122`..`124` | same | V1-0888, V1-0882 | Added pool members and `holderLiveness` changes stay handoff-compatible. `policy update` reports `handoffFences`. |
| `CAL-V0-125`..`126` | same | V1-0891 | Linux CPU utilisation is an opt-in pressure signal, with per-OS signal selection. macOS CPU ticks are not delivered. |
| `CAL-V0-127`..`129` | same | V1-0890 | Dispatcher configuration reload, role `cap: 0`, lane `minAgeSeconds`. |
| `CAL-V0-130`..`134` | same | V1-0889 | In-place binary upgrade with live attempts. Includes the two delegated choices accepted on 2026-10-06: rollback uses the same procedure when `formats` are equal and is refused (drain first) when they differ, with no downgrade reader; install by rename stays mandatory. |
| `CAL-V0-135`..`142` | same | V1-0893, V1-0894 | Pinned-descriptor receipt fold, process-table escape scan, host exit poll backoff, carried review fold, idle tick gate, shared read captures, scaled preview bound, service dispatcher pools. |
| `PSR-V0-013`..`015` | `docs/specs/corvint-tasks-pool-safe-reuse-v0.md` | V1-0892 | Read-only `pool status`. |
| `LCP-V0-016` | `docs/specs/local-completion-policy-v0.md` | V1-0881 | One repository bracket per dogfood event. |
| `GPK-V0-076` | `docs/specs/go-production-kernel-migration-v0.md` | V1-0881 | `ProbeRepositoryAround`. |
| `IDX-SNAP-V0-026` | `docs/specs/index-snapshot-v0.md` | V1-0416 | `ls-tree` runs beside the status scan. |
| `AFP-V0-034` | `docs/specs/affected-plan-v0.md` | V1-0416 | Affected sources are admitted on the descriptor that is read. |

The owner also accepts:

- **CAL-V0-131 and CAL-V0-132 amendments made during integration.**
  - The dispatcher ledger profile becomes `taskman-dispatch-state/1`.
  - A `/0` ledger is adopted only when drained. Any other version refuses `UNSUPPORTED_VERSION`.
  - Repeated, aliased and unknown members and trailing data are refused.
  - The writer checkpoint format is listed in `formats`. Another version of it only makes the writer decline to the complete route.
- **Guarantee change in `CAL-V0-116`.** Between audits, a fast writer does not detect:
  - prefix receipt or request tamper;
  - evidence or pinned-content tamper;
  - stray request files;
  - a forged consistent checkpoint;
  - non-cooperating edits.

  `receipt audit`, the complete route and the scheduled refresh still refuse all of these. The invalidation token is not rollback-resistant; this is a documented limitation.
- **Guarantee change in `CAL-V0-138`.** The carried review fold does not re-check an earlier receipt rewritten in place under its own name. `receipt audit` and the refresh detect it.
- **Upgrade note.** Upgrading main to batch 4 needs a one-time dispatcher drain. The
  `taskman-dispatch/0` configuration profile is deliberately unchanged.
- **FPK-V0-024 citation update.** The citations in this already-accepted requirement were updated. Its meaning is unchanged.

## Limits

This decision settles intent only. Delivery status is unchanged: each spec keeps its own delivery
and qualification wording. That includes:

- live fleet qualification NOT_RUN;
- macOS CPU ticks not delivered;
- `make gate` NOT_RUN.

## Rollback

Revert this decision and restore the "proposed" markers in the affected specs,
`docs/specs/README.md` and `docs/specs/INDEX.json`.
