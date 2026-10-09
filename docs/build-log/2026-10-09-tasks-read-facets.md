# Tasks read facets, `--count` and release readiness counts (CAL-V0-206..209, proposed)

Date: 2026-10-09

The owner asked in chat that corvint-tasks answer questions like "how many tickets to 1.0"
instantly, for every kind of search. This lane adds proposed `CAL-V0-206..209` to
`docs/specs/corvint-tasks-agent-leases-v0.md` and amendment A26. The requirements are not accepted.

## Decisions

- **Facets are opt-in.** `--facets` adds a top-level `facets` member to `ticket search`,
  `ticket list` and `roadmap`. `--count` returns only that member, with no items and `page` null.
  - Without either flag, the output bytes stay the same.
  - Why opt-in: `internal/companionrelease` decodes the roadmap envelope with a strictly closed key
    set (`coreObject` in `core_evidence.go`). A member present by default would break it.
  - The member follows the A22 pattern (absent-only and optional), so the profile stays `/0`.
- **Naming.** The member is called `facets`, not `summary`, because `--summary` already means the
  compact item projection (CAL-V0-165).
- **What gets counted.**
  - Closed enums list every value, zeros included, so a consumer can read any value without first
    checking that it is present.
  - Blocker codes count non-terminal tickets only, which matches list items (CAL-V0-173).
  - Milestones and labels are capped at the 64 most frequent keys, and the number omitted is
    reported. This keeps the envelope far below the 64 KiB bound.
- **`--count` refuses `--offset` and `--limit`.** A count is always over the whole match set, so a
  page window would otherwise be ignored silently.
- **`queue status` gets no facets.** It already reports `byStatus` and `openWithoutMilestone`, and
  `roadmap --count` gives whole-inventory facets.
- **`release readiness` adds `memberCounts` and `milestoneDrift`.**
  - The milestone is associated with the release by the existing convention that the milestone label
    equals the releaseId.
  - "Unfinished" means DRAFT, OPEN or HELD.
  - Each drift list carries an exact count and at most 100 ids.
  - Membership is read and never changed.
- **Readiness skips the source observation without a candidate (CAL-V0-209).** `release.Assess`
  returns `BLOCKED ["candidate"]` before it looks at the source, so `store.ObserveSource` (git
  status, ls-files and a whole-tree hash) was pure cost there.
  - Side effect: a candidate-less readiness no longer fails when git cannot observe the tree.

## Evidence

**Tests.** `TestCALV0206_FacetsCountTheWholeMatchSet`, `TestCALV0207_CountReturnsOnlyTheSummary` and
`TestCALV0208_ReadinessMemberCountsAndMilestoneDrift` (`internal/tasks/cli`) fail on base 02e84575
and pass after:

- On base, `--facets` and `--count` refuse `MALFORMED`.
- On base, readiness in the non-Git fixture errors `UNSUPPORTED`, which also proves CAL-V0-209.

`TestCALV0206_FacetsMemberIsAbsentOnlyAndOptional` (`internal/tasks/wire`) covers the envelope
member. The tests check facet values, byte-identical facets across page windows and repeat runs,
`--count` equal to `--facets`, refusals, readiness drift, and that membership is unchanged.

**Default output.** On the project's own store, the base and new binaries gave byte-identical
output for:

- `ticket search --milestone v1-0 --status OPEN`
- `roadmap --limit 1000`
- `ticket list --status OPEN --summary`
- `queue status`

Readiness differed only by the two new item members.

**Timings.** Medians of 5 runs of the built binary, read-only, on the project's own store (about
1,065 tickets and 3,820 receipts). The store's stat digest was identical before and after.

| Read | Base | New |
|---|---|---|
| `ticket search --milestone v1-0 --status OPEN` | 0.190 s | 0.165 s |
| `roadmap` | 0.174 s | 0.190 s |
| `release readiness v1-0` | 0.557 s | 0.130 s |
| `ticket list --limit 500` | 2.675 s | 2.211 s |
| `ticket search --milestone v1-0 --status OPEN --count` | n/a | 0.216 s (1.6 KB) |
| `roadmap --count` (whole inventory) | n/a | 0.233 s |
| `ticket list --count` | n/a | 0.221 s |

The search, roadmap and list differences without the new flags are noise, because their code path
did not change. The `--count` reads add about 30–40 ms, which is the cost of computing a view for
every matched ticket.

**Profile.** search and roadmap spend about 120 ms on the TM-V0-008 probe, body and probe read:
two intent-tree digests plus decoding every ticket. That cost is inherent to the protocol. `ticket
list` and `queue status` spend 2–3 s on a full journal audit.

## Proposed, not implemented: a derived read index

The remaining floor for every search is the two intent-tree digests plus the ticket decode. A
persisted index could remove it:

- **What it is.** An immutable record of the per-ticket facet fields, keyed by the intent tree
  digest.
- **Who writes it.** Writers only, like the writer checkpoint.
- **How reads use it.** A read would accept the index only when one tree digest matches, and would
  fall back to the full load otherwise.

It is derived state, never authority. It is not built here, for two reasons:

- A read must not create or refresh it, and only writers would keep it current.
- It needs its own requirement, format and benchmark gate (AGENTS.md invariant 7).

## Finding (out of scope)

On the project's own store `.git/taskman.checkpoint.json` is absent, so every `queue status` and
`ticket list` runs the full receipt audit, about 2–3 s. Only writers retain that checkpoint
(`store/guards.go` `retainCheckpoint`). The orchestrator should file this.

## Limits

- Live qualification: NOT_RUN.
- `make gate`: NOT_RUN, by lane rule.
- Timings come from one quiet developer host. They are not a performance budget.
