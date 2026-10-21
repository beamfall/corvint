# 2026-10-21 affected-tiers-1021f: affected tiers and the local lane subset

Tickets: V1-2256 (delivered), V1-2252 (recipe delivered; charter citation and the
10-batch measurement stay with the orchestrator), V1-2261 (not reproduced; regression test added).

## Problem

`corvint affected` selected 135 packages for a change confined to one package, too many to run
under the host semaphore, so lanes hand-picked focused tests and missed siblings and direct
importers that batch CI then failed on (11 of about 24 batches needed two 31-minute runs).

## Decisions

- AFP-V0-162 (accepted under the owner's standing auto-accept, 2026-10-11): the `affected-plan/1`
  default carries an optional `tiers` member `{counts, summary, units}` computed from each
  selection's witness: tier 0 owns or encloses a dirty path, tier 1 is a one-edge dependent, a
  named-path reader or a same-parent sibling of a tier-0 unit, tier 2 the rest. The selection,
  `provider.go.packages` and the `--full` `affected-plan/0` document are unchanged. The member is
  in the shared compact projection, so the review embedding (RGV-V0-015) keeps equal to the default
  document. Compatible under CCF-V1-006; the `affected committed range` core-freeze golden is
  regenerated.
- There is no text output mode for `affected`; the per-tier count line is the `tiers.summary`
  string, so stdout stays one canonical JSON line and stderr stays quiet.
- Lane recipe (V1-2252) recorded in AFP-V0-162 and `corvint help affected`: tier 0+1 Go packages
  locally, capped at 15, `./cmd/corvint` only when changed directly, tier 2 and the remainder
  `NOT_RUN` for CI. No `.github/` change.
- V1-2261: both commit paths (`finishLane.gitPassthrough`, `check.gitPassthrough`) pass
  `os.Environ()`, and `script/dogfood-seal.sh` / the Makefile do not scrub the environment. The
  likely cause of the observation is identity exported in a different agent shell call (shell state
  does not persist between tool calls). `TestDogfoodCommitsHonourExportedGitIdentity` pins the
  behaviour with a different global identity; it fails when the finish-lane commit is switched to
  `gokernel.SanitizedGitEnvironment()`.

## Evidence

- `TestAFPV0162AffectedTiersRankTheFullSelection`, `TestAFPV0162EmptySelectionOmitsTiers`,
  `TestDogfoodCommitsHonourExportedGitIdentity` pass; the affected, review, guidance and help tests
  in `cmd/corvint` pass.
- `TestCoreVerbsEmitTheFrozenProfiles/cem_verify` and `/cem_status` fail on this host only because
  the local toolchain is go1.27.3 and their goldens record go1.27.2; unrelated to this change.

## NOT_RUN

`make gate` (owner preference); packages beyond the charter's selection rule.

## Rollback

Revert this change: drop `Tiers` from `affectedCompactReceipt`, delete `affected_tiers.go`, restore
the golden, AFP-V0-162 and the help paragraph; delete the identity regression test.
