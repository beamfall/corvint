# 2026-10-06: V1-0865 review affected section over an incomplete guidance inventory

## Intent

Ticket V1-0865 (P1). `corvint review --base` embeds an affected section that `reviewGuidance`
planned over the guidance snapshot. That snapshot is capped at 4,096 inventory entries, 256 KiB per
source and 8 MiB of source bytes. On this repository (about 7,265 tracked files), the cap omitted
2,742 entries and 3,418 sources. For PR 571's range (base `ce4ae150`, head `a3876715`), review
reported `selected: []`, provider `EMPTY_SELECTION`, `UNINDEXED_SOURCE_PATH` for every changed
source, and a false `NO_REPOSITORY_GATE_DECLARED`. The standalone `affected --base` on the same
range selected the changed units and the `make gate` / AGENTS.md Verify checks. This broke product
invariant 2: missing evidence produced claims instead of uncertainty.

## Change

- `cmd/corvint/repository_guidance.go`: when the guidance receipt records an `inventory` or
  `unread-sources` omission, review sets `review.affected` to `compileAffected` for the same root
  and base. This is the standalone `affected --base` path and its inputs. A receipt revision other
  than the captured one refuses as `HEAD drift`. A receipt-level unknown names the substitution and
  the equivalent `corvint affected --base <sha>` command. With a complete inventory, the immutable
  snapshot plan is unchanged; it moved into `snapshotReviewAffected` with the same scratch
  cleanup contract (RGV-V0-011). The `immutable source inventory incomplete` advice unknown became
  unreachable and was removed: an incomplete inventory now never reaches the snapshot plan.
- Spec `docs/specs/repository-guidance-v0.md` adds RGV-V0-013 (no affected plan over an
  incomplete inventory; byte-equal to the standalone receipt; within-cap unchanged) and RGV-V0-014
  (gate absence only from a complete read), a bounds note, and traceability rows.
- Test `TestRepositoryGuidanceReviewAffectedInventoryCompleteness` in
  `cmd/corvint/repository_guidance_test.go`: an over-cap fixture with 4,100 filler entries that
  sort before the Makefile and Go sources, and a within-cap control.

## Decisions

- **Reuse the real affected path, not abstention.** The ticket preferred reuse if the cost was
  acceptable. Measured in a private clone at PR 571's head (`a3876715`), with three runs each:
  review went from 0.92-0.98 s to 1.96-2.47 s wall time. Standalone `affected --base` took
  1.45 s. The review receipt grew from 6.2 KB to 149 KB, well under the 1 MiB guidance output
  bound. After the change, its `review.affected` is byte-equal to `affected --base` (98 selected
  units, RUNNABLE, `make gate` plus the AGENTS.md Verify commands, and no gate-absence unknown).
- **Fallback on either omission kind.** A per-source omission (oversized, binary or otherwise not
  admitted) is also missing from the materialized scratch tree. It can drop a unit or the
  Makefile/AGENTS.md declaration just as the entry cap does.
- **Inputs.** The fallback plans over the worktree, not immutable blobs. Review already requires
  a clean worktree at the captured HEAD. It checks this at open and again before emission, so
  tracked bytes equal the captured tree except during an edit-and-revert race. That race is the
  same exposure as the standalone command, which also brackets its graph build with
  status/HEAD rechecks. The unknown states this weaker binding.
- **No analyzer schema or Genesis change.** `internal/contextindex/analyzer_schema.go` and the
  guidance caps are unchanged.

## Other GuidanceSnapshot consumers

`features` and `overview` share the snapshot. They already add the receipt unknown
`inventory or source bytes omitted; absence is not negative evidence`. They make no gate or
selection absence claim, so this fix does not apply to them. Review's `changedFeatures` is inferred
from the same capped inventory and can miss features. That case is covered by the same receipt
unknown and by the existing target-tree unknown; no change was made.

## Limits

- On a repository whose affected receipt exceeds the 1 MiB guidance output bound, review now
  refuses with `guidance output exceeds 1 MiB`. Before, it would emit a false small section. This
  fails closed.
- `affected.Build` does not observe the guidance context deadline (the same behaviour as the
  standalone command).

## Evidence

- Regression: with the base `repository_guidance.go`, both over-cap subtests fail. The base
  receipt shows `selected: []`, `EMPTY_SELECTION`, `UNINDEXED_SOURCE_PATH: cmd/demo/main.go` and
  `NO_REPOSITORY_GATE_DECLARED`. With the fix they pass; the within-cap control passes both ways.
- Focused: `go test -run TestRepositoryGuidance ./cmd/corvint` passes, along with the doc gates
  listed in the lane report.
