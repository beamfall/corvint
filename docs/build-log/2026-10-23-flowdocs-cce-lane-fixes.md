# 2026-10-23 flowdocs, appmap and change-compiled E2E fixes (V1-2414, V1-2415, V1-2416, V1-2427, V1-2428)

## Problem

Five residuals from lanes bf-1022q (beamfall/corvint#1547) and cce2-1022u (beamfall/corvint#1553),
all OPEN on `origin/main` 18873330:

- V1-2414: `docs flows generate` over 256 flows returned the untyped `flowdocs-refused: flow limit
  exceeds 256; narrow scope`.
- V1-2415: the web lexical mask reads a TypeScript generic call `useMemo<T>(` as a JSX open tag and
  blanks the rest of the statement and following lines as markup text.
- V1-2416: `docs consume` refused with `documentation-limit-exceeded` without the limit's name,
  value or the observed size.
- V1-2427: a BBF control whose hook run failed in its own teardown read `control-not-killed`; the
  hook's `teardown-failed at` detail was dropped by BBF's aggregate reasons.
- V1-2428: `harvest` read only `appmap.json` to bind proposals.

The lane was limited to `internal/flowdocs`, `internal/appmap`, `internal/changee2e` and the
refusal text of `cmd/corvint/docs.go`.

## Decisions

- V1-2414 is AFU-V1-125: the owned code `flowdocs-flow-limit-exceeded`. Generation keeps counting
  candidates past the limit without building their flows, so the message gives the exact
  whole-scope count rather than "257".
- V1-2427 is CCE-V0-087. BBF's vocabulary and its `infrastructure_failed` status are unchanged;
  `internal/changee2e` reads each attempt's hook receipt and carries a `teardown-failed at` detail
  into the control proof and the verdict note. A survivor in another control still wins, since it
  is evidence about the test.
- V1-2428 is CCE-V0-088: `HarvestOptions.Manifest` with the catalog's `appmap.json` default.
  Review repair: `corvint-test e2e harvest --manifest` now reaches `HarvestOptions`
  (`TestCCEV0088HarvestManifestFlag`), and `docs flows` lifts `flowdocs-flow-limit-exceeded` to its
  own stderr wire code like `flowdocs-generator-stale` (`TestAFUV1125FlowLimitCLICode`). Both CLI
  tests failed on the prior code.
- V1-2416 governs `docs consume`, whose contract is SDD-V0-005 in
  `docs/specs/source-documentation-draft-v0.md`. The lane's reserved FDM-V0-008..012 belong to the
  `flows docs maintain` profile, which never emits this code, so no FDM ID was used. SDD-V0-005 is
  amended in place, with no new ID, because the lane reserved no SDD range.
- V1-2415 is blocked in this lane: the mask is `flowWebMask` in
  `internal/contextindex/flowlexical.go`, shared by flowdocs, appmap's `scanTestRefs` and
  contextindex's own non-Go impact. A proposed patch (blank only a one-line type-argument list that
  directly follows an identifier and precedes `(`; `&&`/`||` end the match) was tested in a scratch
  worktree and not committed. The flowdocs fixture `useMemo<number>(...)` followed by
  `export function render()` found no declarations before the patch and both after. The same patch
  changes `TestAnalyzerSchemaInputs` (IDX-SNAP-V0-017), so it also needs `script/analyzer-schema.sh
  bump` and use-case receipt repins, both outside this lane.

## Residuals

- The `internal/doccompiler/draft.go` limit refusals, which `docs draft` and `docs maintain` return,
  name their bound but not the observed size.
- The declared-inventory bounds in flowdocs (`in-scope declarations exceed 16384`, `flow inventory
  must declare 1 to 256 flows`) remain untyped `flowdocs-refused`.
