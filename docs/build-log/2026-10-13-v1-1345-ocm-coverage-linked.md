# 2026-10-13 OCM coverage counts the IDs the change cites or traces (V1-1345)

Branch `claude/v1-1345-ocm-linked`, from `origin/main` `523fa4ce`. Requirements `OCM-V0-019` and
`OCM-V0-020` (new) in `docs/specs/ocm-v0-dogfood.md`, accepted under the owner's standing
auto-accept of lane requirements (2026-10-11). 1.0 scope: `PRS-V1-018` (accepted 2026-10-13,
V1-1344..V1-1348 at milestone v1-0).

## Intent

Every lane this week printed `dogfood-check: SUMMARY ... ocm=ready-for-review requirements=56
linked=0 unlinked=56`, then PASS, even when it added requirements with traced tests. The members
were meant to tell a reviewer how much of the intent this change traces, and they told them
nothing. A lane that adds `CEM-V1-226..229` with tests should read `linked=4` of those 4.

## Cause

`checkSummary` printed the `OCM-V0-013` aggregate's `coverage.total/linked/unknown`. Each OCM map
enumerates every requirement of the declared intent (`OCM-V0-003`), so `total` is the whole spec.
An obligation is `linked` only through an `ocm link` row from `DOGFOOD_OCM_LINKS`, which no lane
supplies, so `linked` was always 0 and `unlinked` always the total.

## Decision

Option A: count the requirements the change references. `dogfood-check` already reads every
needed input without a new producer: the committed map's `evidence[]` spans (path, blob, bytes),
the BASE..target diff, the target intent blobs, the change's own OCM maps, and the target tree's
test files. `internal/dogfoodflow/ocm_coverage.go` computes the counts; `checkSummary` prints them
after `ocm=STATE` (`OCM-V0-019`). Requirements are enumerated with the frozen `OCM-V0-001` syntax
and fence rules, and every name match uses the `OCM-V0-017` exact-token rule, so
`CVI-V0-002-strict` does not trace `CVI-V0-002`.

Rejected: option B (drop the members and amend the spec). It was reserved for a linker that cannot
see the inputs, and the inputs are readable; dropping them would also remove the one line a
reviewer reads for requirement coverage.

Unchanged: the `ocm/0.1-experimental` wire (frozen by `OCM-V0-014`), the aggregate and its
`OCM-V0-016` finding, `DOGFOOD_VERBOSE=1` output, and the `dogfood-check: PASS` verdict. The
members gate nothing.

## Failure modes

- An unreadable, oversized or symlinked map or OCM map, an `evidence` member that is absent, a
  missing intent blob at the target, a failed diff or a `git grep` error: all three members print
  `NOT_OBSERVED` (invariant 2), never a guessed count.
- No intent declared: `ocm=NOT_ASSESSED` with no members, as before. Aggregate absent:
  `ocm=NOT_OBSERVED` only, as before.
- A citation outside the requirements section (title, traceability table) references nothing.
  A pure deletion references nothing, because its requirement no longer exists at the target.

## Acceptance evidence

- `internal/dogfoodflow/check_test.go:TestOCMV0019SummaryCountsRequirementsTheChangeCitesOrTraces`:
  a bound map citing two traced IDs gives `requirements=2 linked=2` (`unlinked=0`); an unresolved
  cited ID gives `requirements=1 linked=0` (`unlinked=1`); a linked obligation traces it; a map
  citing none, or citing only outside the section, gives `0/0`; a diff adding two requirement
  lines, one named by a traceability row and one by a `*_test.sh` line, gives `2/2`; an adjoined
  `FIX-V0-003-extra` token traces nothing; spec edits on both sides of the requirements, under
  `diff.interHunkContext=20`, reference none of them; enumeration matches `OCM-V0-001` for a
  backtick info string holding a backtick and for a requirement line inside a fence; a map without
  `evidence` or an absent intent gives NOT_OBSERVED. PASS. With `--inter-hunk-context=0` removed
  the test fails (`requirements=4`), so the configured-context case is live.
- `internal/dogfoodflow/check_test.go:TestDCWV0033CheckSummaryAbstainsOnUnobservedFields`: the
  aggregate's 56/0/56 never reaches the line; nil coverage prints NOT_OBSERVED. PASS.
- `cmd/corvint/dogfood_flow_portable_test.go:TestDogfoodDailyPathRunsFromBinaryInForeignRepository`:
  the end-to-end SUMMARY changes from `requirements=2 linked=0 unlinked=2` to
  `requirements=1 linked=1 unlinked=0` (the plan cites `FIXTURE-LCP-001`, which a test names). PASS.
- `cmd/corvint/dogfood_ocm_dcw_test.go:TestDogfoodDCWLinksSurviveRebind`: a map citing
  `DCW-V0-013..015` with link rows prints `requirements=3 linked=3 unlinked=0`, not the DCW
  intent's whole section (`OCM-V0-020`). PASS.
- Focused run `go test -count=1 -timeout 30m ./internal/dogfoodflow/... ./cmd/corvint/ -run
  'OCM|Dogfood|Check'`: the changed tests pass. `TestClaudeNativeDogfoodLifecycle` hit its hook
  deadline once at load average 30 and passed when rerun alone.

## Independent review

One Codex round (`codex exec -s read-only`) over the staged diff reported three medium findings,
all confirmed and fixed:

1. The enumerator differed from `internal/lrfrepo` `requirementsFromBlob`: a backtick run whose
   info string holds a backtick opened a fence, and requirement lines inside a fence were skipped.
   `parseRequirements` now follows OCM on both.
2. A repository `diff.interHunkContext` merges nearby `-U0` hunks over unchanged lines, so an
   untouched requirement between two edits was referenced. The diff now passes
   `--inter-hunk-context=0`.
3. `git diff` and `git grep` output was unbounded. Both now read through a 16 MiB cap; overflow
   prints NOT_OBSERVED.

## Limits

- Name matching is textual. A test file that mentions an ID in a comment traces it, and a test
  outside `*_test.go`/`*_test.sh` (Python, fixtures) does not. Linked means "named by a test or
  row", not "tested correctly"; OCM's own claims stay the stronger evidence.
- A diff that edits an existing requirement references it, even for a typo fix.
- Coverage is computed per check run and is not recorded in the report or the sealed map.

## Rollback

Revert the change commit. `checkSummary` then prints the aggregate counts again; no wire, map or
report format changes, so no stored state needs migration.
