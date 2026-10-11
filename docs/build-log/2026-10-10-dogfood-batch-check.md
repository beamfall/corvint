# 2026-10-10 — Dogfood batch-check for merge batches of sealed changes (V1-1151, V1-1171)

The w4 batch (beamfall/corvint#789, branch `claude/batch-2026-10-11-w4`) merged 18 lanes that had
each been bound, checked and sealed. Rebinding the batch as one change refused
`git-budget-exceeded`, `ocm-prepare NOT_PRODUCED`, `intent-scope-drift` and `admitted-path-limit`,
then `dogfood-report-drift`. Every one of those limits is a deliberate bound of one change.

## Decision

Do not raise a limit and do not add a batch binding format. Add a read-only verb,
`corvint dogfood batch-check BASE` (`make dogfood-batch-check`), that treats the sealed CEMs a range
adds as its constituents: it re-verifies each against its own base with `cem status`, and requires
every other commit of the range to be a seal commit or a merge whose tree `git merge-tree` re-derives
from its two parents (default, `-Xours`, `-Xtheirs`, or first-parent tree). Only canonical
(`cem/0.2`, `cem/0.3`) maps count, and only a verifier response that reports canonical `ready-for-ci`
verification of exactly that bind and base, never an exit status alone. Merge re-derivation runs in a
private temporary bare repository with the repository's objects as alternates and none of its
configuration or attributes, so no merge driver or hook runs and the repository is unchanged; the
shared dogfood Git reader now also disables fsmonitor and the untracked cache, which a
repository-configured hook could otherwise have used (independent review round 1). The cleanliness
probe is Core's hardened status, which refuses a configured clean or process filter instead of
running it; system attributes are off during derivation; and a `TMPDIR` inside the repository is
refused before any scratch file exists (round 2). What the check cannot prove (OCM, local outcome,
composition, dropped conflict sides, verifier independence) is printed as `NOTE` lines, and each
failure names its recovery. The over-limit refusals of `dogfood change` now name this path
(DCW-V0-039, DCW-V0-040, proposed).

V1-1171: `cem cite` already accepts any regular file at `BASE` with any registered relation, and a
hunk may take several rows. The gap was guidance: `docs/DOGFOOD.md` now says so with an example, and
the `invalid-arguments` and `missing-evidence` refusals print a `fix:` line naming the plan row
(DCW-V0-041, proposed).

## Evidence

- Real data, scratch clone of the w4 branch (objects repacked, alternates removed), current-tree
  binary:
  - The batch as landed (`8d4dd6c7f`): exit 1 in 44 s over 247 commits; all 18 constituents
    `ready-for-ci`; `FAIL merge-resolution-unbound count=18` (mostly hand-resolved
    `docs/decisions/README.md` conflicts) and `FAIL unbound-commits count=16` (the orchestrator's
    acceptance, docs and receipt-repin commits made directly on the batch branch). The object
    directory's file count was the same before and after.
  - A replay that merged only the 18 lane seal tips onto the same base with `--no-ff`, falling back
    to `-X ours` (354 paths, 873 hunks; 914 constituent hunks): `PASS constituents=18 commits=228`
    in 30 s, derivations `ort=5 ort-ours=14`.
- Tests: `internal/dogfoodflow` `TestBatchCheck*` and `TestChangeDCWV0040*` (with `-race`), and the
  built-binary `TestDogfoodBatchCheckDCWV0039SealedLanesWithCodeCitations` (two lanes, one citing
  code and spec on the same hunk, bound, sealed, merged and passed with the real verifier).

## Limits

- Retroactive `dogfood-bind-range` bindings are not constituents; their map lives only in the
  binding commit, and `cem status` reads a map from a worktree path.
- The bootstrap allowance is wider than a lane's own check: every whole-file `no-evidence` unknown
  on a path absent at the lane's base, each listed, because the sealed CEM does not record intents.
- The landed w4 batch itself would still fail; passing needs the orchestrator's own edits sealed as
  changes and conflicts resolved with `-X ours/theirs` plus a sealed fix-up. An append-friendly
  merge for index files such as `docs/decisions/README.md` would remove most hand resolutions.
- Two lanes in one repository share `.context-corvint`, so the second lane's pass refuses
  `unsupported-query-trace-state`; lanes need separate clones.
