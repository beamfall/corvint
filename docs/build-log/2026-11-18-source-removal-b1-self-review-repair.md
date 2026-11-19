# Source-removal B1: surviving ownership readers

V1-5694; RMV-V1-005/007/031 and AZU-V0-8926. Repair parent:
`d7cedd04212316038d9d53c4e1f2106e79a51660`.

The retained miss-class regression still required the deleted Python replay test for both
shard tables. Replace those expectations with the surviving Go shard package, which embeds
both tables. Keep the Python macOS shard test for the cost table, both table cases, and the
ownership and selection assertions. The AZU-V0-8926 clause names the surviving readers.
The owner map and selection implementation are unchanged.

This finding is stale reader inventory after source removal, not an analysis soundness gap.
No recognition allowlist changes or unknown-form regression apply. Each replacement reader
has direct source evidence: the macOS test reads the committed cost table, and the Go shard
package embeds the cost and split tables. No missing selection unit is invented.

## Verification

Before tests, `corvint affected` against
`55decc6a041a1df3f8ae43e95ba21cea669e7d80` selected the live repository package by
DIRECT_TEST_CHANGE and specindex by PATH_LITERAL_READER. The plan remains UNKNOWN:
345 selected, 344 excluded, 80 unknowns and 16 unresolved packages. Unknown reasons are
LANGUAGE_FRONTIER=58, UNINDEXED_SOURCE_PATH=5 and UNOWNED_DIRTY_PATH=17.
The raw plan is outside the repository; SHA-256:
`9171c6f36c1dcc753487790635d921aceb3cc44b9b20dfbaf560a5d098936c0e`.

The entire existing `internal/liveverify/affected/liverepo` package ran through
`script/heavy.sh go test -count=1 -timeout 30m`, with the live tier enabled. It FAILED
in 137.502s. Neither shard-table selection expectation failed. Three checks still fail
because the earlier B1 change removed the benchmark owner:

- `TestOwnershipAuditRatchet_AZU_V0_002`: 174 tracked benchmark paths are unowned.
- `TestRepositoryOwnershipComplete_AZU_V0_8900`: the same unclaimed benchmark paths.
- `TestOwnershipReaderMissClasses_AZU_V0_8926`: the unchanged W-class case for
  `benchmarks/README.md` no longer has an ownership rule.

These remaining failures are outside the requested shard-table repair. Their assertions
remain intact; the package result is not a passing qualification. The full failure log is
`/home/russell/lanes/sr-b13-evidence/self-review-liverepo-test.log`.

Linux vet for the touched package, Windows vet, changed-input checks, spec generation,
spec-requirements-check and line-citations-check PASS. The spec check ran the whole existing
specindex package through the heavy wrapper (1.159s, PASS). The initial generator invocation
refused the unstaged spec; staging it and rerunning succeeded, with no generated diff.
Citation repin found no moved anchors; 184 existing basename-only skips remain.

`make dogfood-check` exits 2: the target is absent. Decision 0594 and `docs/DOGFOOD.md`
retired CEM binding/checking. Binding and validated CEM artifacts remain NOT_PRODUCED;
no capture or seal was attempted. Broad gates and independent review remain host-owned.

## Disposition and rollback

The listed replay-reader regression is repaired, but the full package is not green.
V1-5694 stays PARTIAL for the prior batch blockers and the retained benchmark-owner
failures. B2/B3 are unchanged. Revert this repair commit to restore its test/spec edits;
that restores the obsolete replay-reader expectations too.
