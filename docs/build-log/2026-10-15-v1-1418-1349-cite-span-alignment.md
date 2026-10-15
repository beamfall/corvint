# cem cite: name the text alignment behind an ambiguous refusal (V1-1418, V1-1349)

Date: 2026-10-15. Base: `f8a963e3882749814e066293fbe2a9175799def6` (origin/main).
Tickets: V1-1418 (suspected bug), V1-1349 (fix line). Spec: `DCW-V0-063` in
`docs/specs/daily-change-evidence-workflow-v0.md`, a new lane requirement accepted under the
owner's standing auto-accept (2026-10-11). No accepted requirement text changed.

## One root cause

V1-1418 is reproduced and shares its cause with V1-1349. `cem cite` decides stability with the
verifier's drift classifier (`CEM-CB-005`): a span of a changed path is accepted when its text
occurs exactly once in the changed file. It does not use git's line alignment. In the fixture
(`TestCiteAmbiguousRefusalNamesTextAlignmentV11418`), a file of two `if/else/fi` blocks changes
line 1 and gains a third block at the end. `git diff -U0` shows `@@ -1 +1 @@` and
`@@ -12,0 +13,5 @@`, so base lines 9:11 (`else`, `skip`, `fi`) are unchanged. At BASE the cite was
refused as `cite-span-not-stable` with only `would be ambiguous`, because that text now occurs 3
times. The dogfood fix line then said the change "edits or deletes" those lines, which was false.

## Decision

Accepting such a span would contradict `CEM-CB-005`. The verifier refuses ambiguous evidence too,
so a map carrying the span would fail verification. The fix takes the ticket's second option: the
refusal names the alignment it used. Every ambiguous refusal now names the text match and its
occurrence count. When no hunk line removes the span's base lines or inserts between them, the
message also says they lie outside every hunk. Context lines of the `--unified=3` canonical diff
count as unchanged. `dogfood-change` reads that phrase from the cite stderr. It then prints a
`DCW-V0-063` fix line stating the cause and naming the plan row. The refusal code, the step reason
and the exit status are unchanged. A cite refusal without the phrase keeps the `DCW-V0-014` line.
That covers stale, deleted, in-hunk ambiguous and older-binary refusals.

## Evidence

- BASE vs fix: with the BASE `commands.go` and `change.go`, `TestCiteAmbiguousRefusalNamesTextAlignmentV11418`
  and `TestChangeDCWV0063NamesUnalignedCiteSpanCause` fail (1/1 each). Both pass with the fix.
- Mutations: removing the context-line advance in `spanOutsideHunks` fails
  `TestCiteAmbiguousRefusalNamesTextAlignmentV11418` and `TestSpanOutsideHunksFollowsGitAlignmentV11418`.
  Forcing `citeSpanUnaligned` false fails `TestChangeDCWV0063NamesUnalignedCiteSpanCause`.
- Focused packages from `corvint affected`: `internal/cem/workflow`, `internal/dogfoodflow`,
  `internal/cem/cli` and `internal/cem/wire` pass. `internal/cem/verify` failed once with
  `git-timeout` in `stable_repository_unix_test.go` at load average 29 to 31. It passed on the
  immediate rerun at the same load. The change only adds `SpanOccurrences` there.
- Doc gates: the new `change` field moved `run`'s `--absolute-git-dir` line, so the
  `docs/DOGFOOD.md` citation now names `internal/dogfoodflow/change.go:105`, the line it describes.
