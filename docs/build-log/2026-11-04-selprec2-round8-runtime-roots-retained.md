<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selection precision 2, round 8: runtime-rooted Go readers are retained

Date: 2026-11-04. Lane selprec2, on `2d714e105`, base `6f12f4704`.
Tickets: V1-4173, V1-4437.
Requirements: AFP-V0-2203 and AFP-V0-2240 to AFP-V0-2242 amended in place, AFP-V0-072 annotated;
no new IDs. Orchestrator-made, owner-delegated decisions, taken in-task: soundness over precision.

## Independent findings

The independent review of `2d714e105` returned:

1. P1, planner and rule (c): a test that walks a directory under a runtime root
   (`filepath.WalkDir(os.Getenv("FIXTURES") + "/repo", ...)` with `FIXTURES=/checkout/app/x`)
   reads `app/x/repo/lib.js`, yet placement dropped its package because no literal of the
   package spells `app/x`. Rounds 6 and 7 had retained multi-component and file-name runs; a
   single directory component was still placed.
2. P2, rule (c): `os.Getenv("PREFIX") + strings.ReplaceAll("foo..\\notes", "\\", "/")` lexed as
   `foo..` and `notes` in `internal/gateselect` but as one `foo../notes` token in the planner
   (AFP-V0-506), so the two disagreed on `app/prefoo../notes`.

## Decisions

- Close the class, not the case (P1). Placement may drop a reader only when every root its
  tokens can be joined to is a literal or proven-bounded repository-relative root. For Go source
  no syntactic proof is sound: a root can come from the environment, a flag, a variable, a helper
  in another package, a working-directory change or a walked directory. So for a Go unit outside
  the CEM paths the planner applies neither the AFP-V0-072 placement test nor the V1-0290
  file-name restriction: every token names a path by the component-run relation at any depth
  (`namesAny(..., anyDepth)` and the Go branch of `resolves` in
  `internal/liveverify/affected/readers.go`). `internal/gateselect/naming.go` is deleted and rule
  (c) is the base AFP-V0-012 relation again in `readers`, `pathReaders` and `packageBounds`.
  The limitation paragraph that accepted the missed reader is removed from the spec.
- Lexer parity (P2): `pathTokens` in `internal/gateselect/readers.go` applies the planner's
  `..\` to `../` rewrite after the printf-verb blanking, so the two lexicons agree exactly and
  `..\x` is no longer a recorded difference. The case is in the shared parity fixture and in the
  `-bounds` check.
- Kept: certified non-Go units (`Unit.PlacesFileNames`, AFP-V0-400) keep placement, because their
  plugin certifies the roots. The CEM rules (V1-0230, AFP-V0-931) are unchanged; see Residual
  gaps.

## Fail-before evidence

With the `2d714e105` versions of `internal/liveverify/affected/readers.go`,
`internal/gateselect/readers.go`, `internal/gateselect/select.go` and
`internal/gateselect/naming.go` copied over the fixed tree and the new tests kept:

- `internal/gateselect`: `TestRuleCAnchoredTokens_AFPV02240`,
  `TestRuleCRuntimeRootedFileNames_AFPV02240`, `TestRuleCRuntimeRootedDirectoryWalk_AFPV02240`,
  `TestRuleCLoneTokens_AFPV02241` (including the missing `-bounds` line for
  `app/prefoo../notes`), `TestRuleCMatchesThePlanner_AFPV02242` (the planner selected
  `backslash climb mixed prefix`, rule (c) only `backslash climb mixed`) and
  `TestFastTierLiteralLexiconDiffers_AFPV0765` failed.
- `internal/liveverify/affected/golang`: `TestAnchoredTokensResolveFromModuleBases_AFPV02203`,
  `TestRuntimeRootedFileNamesStaySelected_AFPV02203` and
  `TestRuntimeRootedDirectoryWalkStaysSelected_AFPV02203` failed.

With the fix every test passes. Each new test has a negative control: a walker of
`/elsewhere`, or a path under `elsewhere/`, `other/INDEX.json` or `other/INDEX.md`, that no
literal names, which stays unselected.

## Narrowing that remains

- Go rule (c) in the planner and the CI selector: none beyond base. The rounds 1 to 7 numbers
  (PR 86 range: `reader` lines 79 to 28, run 257 to 209 packages; the `jslinecov` probe 24 Go
  readers to 6) are withdrawn. Inferred, not re-measured: both return to the base 79 lines, 257
  packages and 24 readers.
- Shell temp roots (AFP-V0-2204, AFP-V0-2243): kept, as measured in rounds 5 to 7 (28 readers to
  23 to 16).
- Import-graph readers (AFP-V0-2200, AFP-V0-2201), which do not use rule (c) placement: kept.
- Certified non-Go unit placement (AFP-V0-400): kept.

The local planner also selects more than base for a lone Go token that now names a directory at
any depth (`script` from a `<script>` literal, `docs`, `.github`, `README.md`). This is the price
of the sound relation, and it is the relation the CI selector already applied.

## Residual gaps (suspected, not confirmed)

1. CEM paths: V1-0230 and AFP-V0-931 still narrow by placement, so a reader of a CEM path under a
   runtime root could be dropped.
2. The V1-0290 file-name prefilter still applies to a non-Go unit whose plugin does not certify
   its roots.

## Verification

Through `script/heavy.sh`, one package per command: `internal/gateselect`,
`internal/liveverify/affected` (run locally for its darwin-only files),
`internal/liveverify/affected/golang`, `.../typescript`, `.../shell`, `.../languages`,
`tools/gate-affected-select`, `tools/gate-ledger`, `tools/unbounded-readers`,
`internal/langplugin`, `internal/dogfoodflow` and `internal/sessiontest` all pass. `go vet` of the
three changed packages passes. `corvint affected` on the base returned scope `UNKNOWN` with 205
units, so only focused packages ran.

## Rollback

Round 8 only widens selection. Reverting its code commit restores round 7's placement, which
drops runtime-rooted readers.
