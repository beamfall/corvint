<!-- SPDX-License-Identifier: Apache-2.0 -->
# Selection precision 2, round 9: Go lexicon rewrites only add tokens

Date: 2026-11-04. Lane selprec2, on `5dc00511c`, base `6f12f4704`.
Tickets: V1-4173, V1-4437 and V1-4284.
Requirements: AFP-V0-2241 and AFP-V0-2242 amended in place; AFP-V0-505, AFP-V0-506, AFP-V0-840
and AFP-V0-765 difference (4) annotated; AFP-V0-2243 and AFP-V0-2244 disabled for 1.0. No new IDs.

## Independent findings

The independent review of `5dc00511c` returned one P1, in rule (c) of `internal/gateselect`
(`pathTokens` in `readers.go`) and in the planner's Go lexicon alike. Round 8 replaced each token
of a literal by its `..\` to `../` rewrite, so a reader lost the tokens it had as written. A test
in `reader/` that reads
`os.ReadFile(os.Getenv("FIXTURES") + strings.Split("foo..\\notes", "\\")[1])` with
`FIXTURES=/checkout/` consumes the root file `notes`, but its only token was `foo../notes`, so a
change to `notes` skipped it in CI with no fallback.

## Decisions

- Orchestrator-made decision: a normalization may only add tokens, never remove any. Both
  lexicons (`pathTokens` and `spelledTokens` in `internal/gateselect/readers.go`, `literalTokens`
  and `spelledTokens` in `internal/liveverify/affected/golang/golang.go`) keep the union of the
  tokens of the literal as written and of its `..\` to `../` rewrite, after the printf-verb
  blanking. The same rule covers the other two rewrites on this path, which also replaced the
  written token: the `/`-anchored spelling of an import path under the module is added beside the
  token, and the closing-tag spelling without its `/` is added beside it in both lexicons (the
  planner had stripped it, AFP-V0-505 and AFP-V0-840). The planner and the CI selector keep exact
  parity; `TestRuleCMatchesThePlanner_AFPV02242` now also compares the planner's unbounded readers
  with the selector's unresolved set, and its fixture holds the `strings.Split` reader and a
  walker of a runtime root joined to the module's own import path, each with a control.
- Owner-approved cap: round 9 is the lane's last fix round. Any narrowing that this round cannot
  prove sound with a test is disabled and keeps the base selection. On that rule AFP-V0-2243 and
  AFP-V0-2244 are disabled (below); this is an orchestrator-made, owner-delegated decision.
- Kept: the base `wholeShortEnds` rule (V1-1818) in the planner, which the CI selector does not
  apply, so CI stays a superset there. A `..\x` literal keeps a bare `..`; rule (d) reads it as
  reaching the root from a test file deep enough, which fails closed. Measured with
  `go run ./tools/gate-affected-select -unresolved`: 39 unresolved packages at `5dc00511c` and in
  the round 9 tree, the same set.

## Fail-before evidence

With the round 9 tests over the `5dc00511c` lexicons, both new tests failed:

- `internal/gateselect`: `TestRuleCParentBackslashKeepsWrittenTokens_AFPV02242` reported
  `readers(notes) = []`, the verdict for `notes` without `example.com/fixture/reader`, no
  `-bounds` line for it, and the same three misses for the `srcwalk` walker of
  `vendored/example.com/fixture/a.txt`, whose written token the anchored spelling had replaced.
- `internal/liveverify/affected/golang`: `TestParentBackslashKeepsWrittenTokens_AFPV0506`
  reported no path-literal reader of `notes` or of `vendored/example.test/m/a.txt`.

With the fix both pass, and each has negative controls (the same `strings.Split` reader of
`foo..\other` and a walker of `example.com/elsewhere`) that stay unselected. The updated
expectations in `TestRuleCLoneTokens_AFPV02241`, `TestFastTierLiteralLexiconDiffers_AFPV0765`,
`TestLexiconClosingTagAndParentBackslash_AFPV0505`,
`TestPathLiteralSelectsItsReaderPackage_AFPV0021` and `TestPathLiteralBoundary_AFPV0840` only add
tokens or readers.

## Disabled for 1.0

1. AFP-V0-2243 (temp roots after `&&` or `||`). `unconditionalAssignment` in
   `internal/liveverify/affected/shell/shell.go` again refuses a temp root's assignment followed by
   `&&`, `||`, `|` or `&`, as base AFP-V0-2204 does. Reason: where such a list ends is not proven
   by a test; an `&` after a compound command, a continued line or an empty compound can
   background the assignment, and rounds 6 to 8 each found one more such form. Repro:
   `tmp=$(mktemp -d) || exit 1` then `cat "$tmp/repo/internal/x/y.json"` now keeps the
   `repo/internal/x/y.json` token (`TestTempRootPathsAreNoTokens_AFPV02204`, `or exit`). Kept,
   because they only refuse more: the one-word assignment refusal, the refusing-operator lead after
   a compound closer and the whole-operator lexer advance.
2. AFP-V0-2244 (history-input mentions). `.corvint/shell-foreign-callers.json` is the base
   declaration again, so `internal/dogfoodflow` is a caller of `script/batch-regen.sh`. Reason:
   soundness needs every test that imports it, transitively (`cmd/corvint`,
   `internal/localcompletion`, `tools/capabilities-check`), to read only the committed copy, and no
   test proves that. Repro: an edit of `script/heavy.sh` over the lane base selects the
   `BINARY_EXEC` units again (inferred: 52 units back to 71, not re-measured).

## Narrowing that remains

None beyond base in the lane's rules. Rule (c) is the base AFP-V0-012 relation (round 8), both Go
lexicons hold every base token plus the added spellings, the shell temp-root rule is base
AFP-V0-2204, the foreign-caller declaration is base, and AFP-V0-2247 moves the TypeScript profile
with the same 151 units. The base narrowing the lane did not touch stays: AFP-V0-2200, AFP-V0-2201,
AFP-V0-2204 and certified non-Go placement (AFP-V0-400).

## Residual gaps (suspected, not confirmed)

1. V1-1818 `wholeShortEnds`, planner only and present at base: a short token at a literal end is
   read whole (`"x's/a"`), so the planner may drop a reader that CI keeps.
2. A lone token at a literal or verb boundary is read as a whole component in both lexicons
   (`os.Getenv("X") + "notes"` names `notes`, not `Xnotes`), so a reader of `app/prefixnotes` is
   not attributed by that token.
3. As in round 8: CEM paths (V1-0230, AFP-V0-931) still narrow by placement, the V1-0290 file-name
   prefilter still applies to a non-Go unit whose plugin does not certify its roots, and
   `foreignTestFile` (ACX-V0-852) is planner only.
4. Shell, present at base: the scan in `internal/liveverify/affected/shell/fresh.go` accepts a
   trailing `&&` or `||` at a line end.

## Verification

Through `script/heavy.sh`, one package per command: `internal/gateselect`,
`internal/liveverify/affected` (run locally for its darwin-only files),
`internal/liveverify/affected/golang`, `.../shell`, `.../typescript`, `.../languages`,
`internal/liveverify/mutate`, `internal/dogfoodflow`, `tools/gate-affected-select`,
`tools/gate-ledger`, `tools/unbounded-readers`, `tools/test-read-scopes-check`,
`tools/declaration-compat-check` and `tools/compat-trial` pass. `go vet` of the three changed
packages passes. `corvint affected` on the base returned scope `UNKNOWN` with 205 units, so only
focused packages ran. Not run: `cmd/corvint` and `make gate`.

## Rollback

Round 9 only widens selection. Reverting its code commit restores round 8's replacing lexicon,
the AFP-V0-2243 list rule and the AFP-V0-2244 declaration.
