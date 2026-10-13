# 2026-10-13 Committed-corpus citation baseline after the cem/0.x removal (V1-1278 S3)

Run from the V1-1278 S3 branch `claude/v1-1278-s3-retire-packets` at 5fcbf06bb (PR beamfall/corvint#854,
the retired packets, bound and sealed) in the lane's scratch clone, after the S1, S2a and S2b merges.
Purpose: the same committed-corpus measure as `2026-10-13-v1-1336-citation-arms.md` (registration
`2026-10-13-cem-citation-prospective-registration.md`, decision 0527) over every map the tree holds once
the stable producer is the only producer, so the prospective run (V1-1337) starts from a figure taken
with the shipped `tools/cem-citation-baseline` and no `cem/0.x` reader in the tree. It is a baseline
rerun, not the prospective run, and it decides nothing.

## Inputs

- Tool: `tools/cem-citation-baseline` as committed at 5fcbf06bb (unchanged since PR #852, dc6811176).
- Command, from the clone root with T' commits built by the V1-1336 procedure
  (`GIT_AUTHOR_DATE=GIT_COMMITTER_DATE=2026-10-13T00:00:00Z`, message `T' for <bind> (CEM-V1-104
  scratch)`): `GOTOOLCHAIN=local go run ./tools/cem-citation-baseline --root . --maps .corvint/changes
  --targets docs/build-log/evidence/v1-1278/tprime-targets.txt --summary
  docs/build-log/evidence/v1-1278/citation-baseline-summary.json >
  docs/build-log/evidence/v1-1278/citation-baseline-maps.jsonl`.
- 635 maps under `.corvint/changes/`; 615 T' targets (the same 615 migrated maps as V1-1336; the
  twenty newer maps were produced as `cem/1.0` and need none). Exit 0, 24 min 35 s
  (10:48:58Z to 11:13:33Z), host load average between about 9 and 28.

## Result (`evidence/v1-1278/citation-baseline-summary.json`, `all`)

- 634 of 635 maps scored with empty `armsIssues`; f0fda542 again has no status
  (`unsupported-resource-limit`, 877 hunks). Every map is class `change`; no committed map carries a
  role. 618 maps and 23,814 supported hunks enter the pooled null (16 single-supported-hunk maps
  have no null).
- Authored `anchoredExcess` 1.94% [1.53%, 2.34%]; excluding diff-adjacent bases 1.94% [1.54%, 2.34%]
  (V1-1336: 2.03% [1.62%, 2.44%] and 2.04% [1.64%, 2.44%]).
- Grep bot: anchored 17,737 of 23,830 hunks (74.4%); excess 67.1% [64.6%, 69.7%]; excluding
  diff-adjacent bases 66.7% [64.2%, 69.2%] (V1-1336: 75.0%, 67.4%, 67.0%).
- Preimage: anchored 295 (1.2%); excess -0.38% [-0.66%, -0.09%]; excluding diff-adjacent bases 0.
- Self-citation: 2,646 of 26,314 authored bases same-path (10.1%), 855 diff-adjacent (3.2%); on
  2,520 (10.6%) and 846 (3.6%) of 23,830 supported hunks.
- Tiers: anchored 1,250, cited 22,580, unknown 43, held 0, mechanical-proved 0. Invariant maps 200;
  specificity positive 225, zero 361, negative 48.

The five maps added since the V1-1336 run (its own map 883d62c0 and the S2b and S3 maps 50e99b32,
59c66e6d, b3285373, 11447d3b) contribute 1,177 supported hunks and one anchored hunk: the two large
lane maps (617 and 529 hunks) anchor nothing while the grep bot anchors 521 and 206 of their hunks.
They are the reason the pooled authored excess moves from 2.03% to 1.94%: the maps this lane bound
with one or two spans per directory (the git-operation budget, `2026-10-13-v1-1278-s3-retire-packets.md`)
are exactly the shape the registration's kill criterion names. Read against decision 0527 the
committed corpus stays a kill (E = 1.94% ≤ 2.1%; L below 2.5% and far below G). The prospective run
decides; this entry records the figure the run starts from.

## Retained limits

- Same limits as the V1-1336 run: T' commits are reproducible from the listed inputs but were not
  re-checked; f0fda542 is unscored; the maps are not re-bound.
- The run used the S3 branch head, not the merge commit. PR #854 merged as 94a7d3dc5 with no
  further commits; its tree lists the same 635 maps under `.corvint/changes/` (checked at the
  V1-1278 closeout, which completed natively against 94a7d3dc5).
- The entry is a measurement record; it accepts no requirement and changes no contract.
