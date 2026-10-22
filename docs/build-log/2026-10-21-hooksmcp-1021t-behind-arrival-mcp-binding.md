# Lane hooksmcp-1021t: shared-path arrival age and base observation, MCP MIXED binding

Lane `hooks-mcp-sharedpath-1021t` (branch `claude/hooks-mcp-1021t`), tickets V1-0869, V1-2219 and
V1-1723. New requirements SPC-V0-013, SPC-V0-014 and MCPV0-095 are accepted under the owner's
standing auto-accept of lane requirements (2026-10-11).

## V1-1723: `corvint changes behind` ages commits by arrival on base

**Problem.** SPC-V0-012 aged a missing shared-path commit by its committer date. A commit authored
on a side branch weeks ago and merged into base an hour ago looked stale at once, and the report did
not say how fresh the local view of base was, so a stale remote-tracking ref looked like "nothing
behind".

**Decisions.**

- SPC-V0-013: every counted commit names `arrivedVia`, the oldest first-parent commit of base within
  `head..base` that reaches it, and `arrivedAt`, that commit's committer time. Staleness uses the
  earliest `arrivedAt`. The walk reuses the one `rev-list --parents` read; arrival commits resolve
  their time once each (cached).
- SPC-V0-014: the report adds `baseRef` (`name`, `ref`, `lastObservedAt`, `observedVia`). A
  symbolic base records its full ref and the newest reflog time; for `refs/remotes/` a newer
  `FETCH_HEAD` mtime wins. A commit-ID base, or a ref without reflog or `FETCH_HEAD`, records
  `lastObservedAt: null`, `observedVia: none`. The stale finding's detail repeats the observation and
  tells the reader to fetch.
- Additive members on the closed object: the profile stays `corvint-shared-path-behind/0`
  (SPC-V0-011 key list updated). No analyzer schema bump.

**Evidence.** `TestSPCV0013AgeIsArrivalOnBase` (old side commit merged recently is not stale; direct
commit is, at threshold 48 h vs 49 h) and `TestSPCV0014BaseObservationRecordsRefAndLastObservedTime`
(local reflog, remote reflog, FETCH_HEAD precedence, commit-ID base, expired reflog). Both fail with
the fix removed; the whole `internal/sharedpath` package passes.

**Limits.** `FETCH_HEAD` is repository-wide, so its mtime can overstate the freshness of one remote
ref; a rewritten or clock-skewed merge commit skews `arrivedAt`. Both are stated in the spec's
failure modes.

## V1-2219: MCP bridge MIXED binding does not name dirty paths

**Decision.** Documented why not (MCPV0-095) rather than widening the closed
`corvint-mcp-bridge-result/0` binding: it is shared with `internal/lspstdio`, it is repeated in every
result (path text would become model input on each call), and status and `cem.report` bind from
`gokernel.ProbeRepositoryContext`, which only holds a count and digest. Callers that need paths use
the native receipt (passed through unaltered), `corvint affected` (AFP-V0-356) or `git status`.

**Evidence.** `TestMCPV0095MixedBindingIsCountAndDigestOnly` pins the seven members and the absence
of the dirty path name for `status` and `query`. It pins existing behaviour, so it has no
fail-before-fix proof.

## V1-0869: skipped

The ticket asks to drop `frontier-authority-unavailable` from receipts. Accepted AHI-054 (decision
0519), LCP-V0-001, CF-V1-020 and the session-context-dividend clause all require that degradation in
every receipt, and PR #1234 deferred the removal to this ticket. It needs an owner amendment first.

## NOT_RUN

- `corvint affected` tier 2 (81 units) is left to batch CI.
- `make gate` (not required for scoped lane work).

## Rollback

Revert the lane commit: `behind.go` returns to committer-date ageing without `baseRef`, the two spec
requirement pairs are withdrawn, and MCPV0-095 and its test are removed. No stored state changes.
