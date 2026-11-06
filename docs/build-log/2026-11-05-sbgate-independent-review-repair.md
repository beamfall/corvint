<!-- SPDX-License-Identifier: Apache-2.0 -->
# sbgate independent review repair

Independent review of `58bff8ca838d8f8d9c1707b21cce56481dd33710` required
two V1-4724 repairs. Both preserve the existing ASD-V0-944 and ASD-V0-925 /
DTL-V0-006 contracts, wire output, diagnostics, goldens and existing assertions.

- P1: Make supplies `inputDecision` with `"."`, while Go reports an absolute
  main-module directory. Normalize the module root with `filepath.Abs` before
  resolving symlinks and comparing directories. The existing input-decision
  test now also changes into its fixture checkout and passes `"."` with a
  documentation-only diff, requiring the disjoint skip (ASD-V0-944).
- P2: distinguish failed cgroup discovery reads from successfully observing no
  memory controller. A failed membership or mountinfo read caps the available
  budget at two GiB, preserving a smaller host budget. The new
  `TestBaselineCgroupDiscoveryASDV0925` covers missing and permission-denied
  discovery files, both denied, a one-GiB host, and observed absence of a memory
  controller. Existing ancestor and exhausted-bound tests remain unchanged.

Before the fixes, whole-package tests reproduced P1 and all five ample-host
discovery failures (16 workers rather than two). After the fixes, the entire
existing and added `tools/selection-baseline` package passed without a filter:
`TMPDIR=/home/russell/lanes/sbgate.tmp script/heavy.sh
HEAVY_WHOLE_REASON='affected selects tools/selection-baseline; charter requires whole touched package after review fixes'
env GOTOOLCHAIN=local go test -count=1 -timeout 30m ./tools/selection-baseline`.
Package wall time was 1.675 seconds. Git identity variables were unset.

The current-source binary's `corvint affected --base
c5379b4cf039fa8dd2df2aa4ee6e5b7b897cf514` selected this package; its two
LANGUAGE_FRONTIER unknowns remain. The installed binary's initial plan reported
stale source and was superseded by this current-source plan. Raw plans and logs
stay under `/home/russell/lanes/sbgate-evidence/` in `review-affected-current.json`,
`review-regressions-before.log` and `review-package-tests.log`.
Their SHA-256 digests, respectively, are
`babfe2526a8c70621cfd6dba5aaabc7ee7d75a219cfcde9445fabfce116c2c10`,
`170df2c102049605d866c03d0438fc9bed4ecdd47354e584415f6f4a0fd768af` and
`7fd048e1f4b9e734003a6e581cbd651afba5f391352b1772cfe9c10e235328d7`.

No intent amendment is needed: these fixes restore the existing required skip
and unobservable-memory fallback. Rollback is reverting these four code/test
edits; that restores the reproduced failures. Pre-change context is reused.
Missing original impact and optional guarding-test evidence remain unproduced.
Wider host qualification, baseline replay, independent review and ticket writes
are outside this repair round. V1-4329 remains PARTIAL for its external full
lane-closeout/seal qualification blocker; ASD-V0-1990 deferral is unchanged.
