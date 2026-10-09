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

## Finding: absent read checkpoint (V1-1051)

On the project's own store `.git/taskman.checkpoint.json` is absent, so every `queue status` and
`ticket list` runs the full receipt audit, about 2 s.

**Diagnosis: no writer-code defect; the deployed runtime predates the checkpoint.**

- In-tree writers retain the checkpoint after their audit (`internal/tasks/store/guards.go:107`
  `retainCheckpoint`, called at `store/mutate.go:154`, `store/lease_write.go:471` and
  `store/writer_route.go:127,267`). `TestCALV0060_WritersRetainACheckpointReadsResumeFromIt` covers
  that.
- A read-only in-process audit of the real store at seq 3829 returned `FULL`/`CONSISTENT`, with no
  pending receipt and no staging. `Result.Checkpoint()` (`journal/checkpoint.go:63`) is non-nil, so a
  current writer would retain it.
- The `corvint-tasks` on PATH reports `0.0.0-tcp01-unverified+build.202`. That is the latest
  published standalone tag, `tasks-dev-20260929.2` (0f231225, 2026-09-29). The checkpoint work
  (5802f353, 2026-10-01) is not its ancestor.
- The binary contains none of these strings: `checkpoint`, `CHECKPOINT_PLUS_TAIL`,
  `retainCheckpoint`. So every write on this store ran a runtime with no checkpoint at all. Nothing
  deletes the file, and nothing invalidates it.
- In-process on the real store, read-only, median of 5: a full audit takes 2.01 s. Resuming from the
  checkpoint the current code derives (`CHECKPOINT_PLUS_TAIL`) takes 0.088 s.

**Remedy (owner fork, not done here).** Publish a Tasks dev release from `origin/main`, or install a
clean `origin/main` build, so that writes retain the checkpoint. The checkpoint stays verified: a
read rebinds it, and a stale or forged one falls back to the full audit (CAL-V0-059/060). P0 V1-0841
(Darwin descriptor exhaustion on a 5000-receipt store) should be checked against a new runtime on
this store, which holds 3829 receipts.

## Read probe overhead (V1-1051 follow-up)

A CPU profile of 20 in-process `ticket search --milestone v1-0 --status OPEN --count` reads on the
real store (1081 intent files) attributes most of each read to TM-V0-008's two `intent.TreeDigest`
probes, about 41 ms each, plus a `LoadTree` decode of about 23 ms. Within each probe:

- Phase 1 `statRegular` went through `root.Lstat("tickets/<id>.json")`, which reopens `tickets/` for
  every record: 0.56 s of the 3.6 s total.
- The change opens one `os.Root` for each subdirectory and stats each record beneath it. This uses
  the same no-follow Lstat and the same refusals, and only while `pinTreeDirs` is set.
- Phase 2 still reads and re-checks every record and keeps the pinned-directory rebind check.
- After the change, `statRegular` costs 0.10 s and the 20 reads cost 2.9 s.
- `TestTMV0008_PinnedTreeDigestParity` gains three phase-1 refusals: a symlink, an empty file and a
  directory record. The pinned and per-file captures refuse identically.

The second probe is the TM-V0-008 stability check. Removing it, or caching the tree across reads,
needs a contract change. It is not proposed here.

Interleaved medians of 5, real store, read-only, before and after this step:

| Read | Before | After |
|---|---|---|
| `queue status` (full audit, no checkpoint) | 2.131 s | 2.160 s |
| `ticket list --count` | 0.189 s | 0.167 s |
| `ticket search --milestone v1-0 --status OPEN --count` | 0.173 s | 0.144 s |
| `roadmap --count` | 0.168 s | 0.150 s |
| `ticket search --milestone v1-0 --status OPEN` | 0.190 s | 0.163 s |

## Limits

- Live qualification: NOT_RUN.
- `make gate`: NOT_RUN, by lane rule.
- Timings come from one quiet developer host. They are not a performance budget.
