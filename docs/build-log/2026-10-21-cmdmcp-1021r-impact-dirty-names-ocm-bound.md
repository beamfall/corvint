# 2026-10-21 cmdmcp-1021r: impact names dirty paths; ocm-prepare names the bound

Lane `cmdmcp-1021r`, base `ef2a01db52d5436c262725d136060c9eb95e8580`. Tickets V1-2218, V1-2170,
V1-2066, V1-2020. Requirements DIRTY-CACHE-018 and OCM-V0-026 are accepted under the owner's
standing auto-accept (2026-10-11).

## Problem

- V1-2218: the `corvint impact` MIXED warning (DIRTY-CACHE-014) named the bound commit and the
  dirty count but not which paths were dirty, unlike `affected` (AFP-V0-356). Naming them under a
  packet budget was the open question.
- V1-2170 (V1-1322 AC2): when `ocm prepare` refused with `unsupported-resource-limit`, the message
  said only the code; the Git operation budget and the output byte bounds were collapsed with no
  value, so the dogfood `fix:` line could not say which bound fired.

## Decisions

- DIRTY-CACHE-018: the warning appends `dirty: "a", "b" (+N more)` with at most 8 sorted, quoted
  paths. A budgeted receipt selects evidence against a count-only form (`dirty path names
  omitted`) and only then spends the remaining slack on the longest fitting prefix of names
  (`fitImpactWarningNames`). Names therefore never evict a result, and `MinPacketBytes` (1216,
  pinned by the worst-case envelope test) is unchanged. The commit, count and exclusion clause
  stay in every form.
- OCM-V0-026: `gitrun` errors now carry the bound value (`at its N-operation bound`, `Git stdout
  exceeded its N-byte bound`). The stable verifier keeps the original error in a new
  `StableResult.LimitCause` field tagged `json:"-"`, so `cem-stable-verification/1` bytes, the
  pinned drift-resource-limit `result.json` and `verdictSha256` are unchanged. `ocm prepare`/read
  appends it in parentheses through `stableOCMRefusal`; the code stays `unsupported-resource-limit`.
  The measured size that would have been needed is not reported: the operation stops at the
  bound, so it is never observed.
- V1-2066 not built: decision entry `2026-10-21-receipt-producer-stays-opt-in.md` (commit
  b38cf7dac, V1-2064) keeps `producer` opt-in for 1.0 and recommends declining an MCP-only member
  because it would split MTV-V0-005 CLI/MCP equality; that is an owner/orchestrator disposition.
- V1-2020 not started: it needs new sources of truth (per-command subcommand lists, declared
  maturity, and a registry for every refusal code a row names) and a CAP-V0 non-goal amendment;
  that did not fit the lane budget after the two fixes.
- The warning change edits analyzer inputs (`impact_freshness.go`, `receipt.go`), so
  IDX-SNAP-V0-017 requires a schema bump. This lane pins `corvint-analyzer/141` (140 is reserved
  for V1-2285); the orchestrator renumbers at batch time with `script/analyzer-schema.sh bump`.
  Batch 1021s renumbered it to `corvint-analyzer/142`, because #1483 took 141.

## Evidence

- `TestImpactMixedWarningNamesDirtyPaths` and `TestImpactMixedWarningNamesWithinBudget`
  (`internal/contextindex`) fail on the base and pass with the fix. The existing
  DIRTY-CACHE-014/015 tests and `TestMinPacketBytesIsTheWorstCaseAbstainingEnvelopeRoundedUp` pass.
- `TestOCMV0026PrepareRefusalNamesTheBoundThatFired` (`internal/lrfrepo`), the LimitCause
  assertion in `TestCEMV1260DriftResourceLimitNative` (`internal/cem/verify`; drives the real
  verifier to the 113-operation bound with result bytes still matching), and the message
  assertions in `TestTypedOutputBound`/`TestTypedBudgetExhaustion` (`internal/cem/gitrun`) each
  fail without the fix and pass with it.

## NOT_RUN

`make gate` (owner standing preference); tier-2 affected units are left to batch CI.

## Rollback

Revert the two fix commits: restore the count-only warning and drop `fitImpactWarningNames`;
drop `LimitCause`, `limitSuffix`/`stableOCMRefusal` and the bound values in the gitrun messages.
Reverting also restores the base analyzer schema pins, so snapshots built under 141 are rebuilt
once; no other stored state or wire bytes depend on either change.
