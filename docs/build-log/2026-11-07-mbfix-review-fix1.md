# mbfix review fix round 1: TestMain guards, symlink aliases and refused-link selectors

Date: 2026-11-07. Branch `claude/mbfix`, base origin/main
`4052b8540b5fff818a819db37b55371ea5428c2c`. The round started at the bound head `f7f99ebd2`. The
independent review of the batch (`docs/build-log/2026-11-06-mbfix.md`) returned
CHANGES_REQUIRED with one P1 and two P2 findings. All three are in code the batch carried from
pk-dogfood. Each is fixed in a new commit with a regression test. Every regression test was run
with the production change reverted and failed, then passed with it.

## Findings and decisions

1. **P1, guard candidates (TCQ-V0-7408).** The batch had stopped imported selectors from carrying
   package closure. As a result, a test reaching a changed declaration only through a callback
   that `TestMain` registers (`flag.Usage = Usage`, then `flag.Usage()` in a test) named no reached
   declaration, and the hunk could end as `no-candidate-test`. The batch's initialization safeguard
   recognized only `init` funcs and side-effecting var initializers. Decision: a test file's
   `TestMain` counts as code that runs before the tests. When the reached set feeds it, every base
   test of the package stays a candidate. This holds whether `TestMain` names the declaration
   directly, through a helper, from an external test package or from an importing package. A
   declaration is also treated as feeding the initializer when the initializer's own name is
   reached, for example when the change edits `TestMain` itself. Over-selection costs only extra
   revert runs; a missed guard is a false `no-candidate-test`. Test:
   `TestTCQV07408TestMainKeepsCallbackTests`. Each of its four forms is paired with a negative
   control that stays narrow. Commit `2f8d1ef50`.
2. **P2, symlink aliases (TCP-V0-7406).** Symbol selectors grouped candidates in a one-to-one
   target→candidate map, so two tracked links to one target overwrote each other. A selector reaching
   both was reported resolved, with one arbitrary identity. Decision: each target keeps every
   candidate, and symbol matches expand back to all of them. The row dedupe also compares link
   identity exactly. Before, a row naming a target directly was dropped when a row through a link to
   that target sorted first. Line selectors already kept both aliases and serve as the control. Test:
   `TestTCPV07406SymlinkAliasesKeepEveryIdentity` covers two links to one target and a link sorting
   before the target it names. For both symbol and line selectors it checks the ambiguity result and
   each row's own `link_path` and `link_blob_hash`. Commit `ce913f3f8`.
3. **P2, refused-link selectors (TCP-V0-7406).** `selectorSubject` dropped the link-admission
   refusal and returned the link itself. A dangling, chained, out-of-tree or cyclic `link.go#Target`
   therefore searched the link's own symbols and ended as `anchor-not-found`. `link.md:10` was
   judged against the one-line link blob. Decision: `selectorSubject` returns the refusal. Line and
   symbol resolution keep the refused candidate unchecked, so `addEvidence` records the typed
   refusal in `unavailable_selectors` before any matching, and the anchor is
   `anchor-evidence-unavailable`. Test: `TestTCPV07406RefusedSymlinkSelectorsKeepTheirRefusal`
   covers the dangling, chain, outside and cycle cases. Each case is checked with `link.md:10` and
   `link.go#Target`. A `docs/link.md:1` control already recorded the refusal before the fix.
   Commit `ce913f3f8`, shared with finding 2 because both change `selectorSubject` and
   `resolvePathSymbol`.

The spec rows `TCQ-V0-7408` and `TCP-V0-7406` now state these rules and name the new tests. No
requirement ID was added or renumbered.

## Verification

- `corvint affected --base f7f99ebd2…`, from this checkout's own build because the installed binary
  was stale: 356 units selected, scope UNKNOWN (69 LANGUAGE_FRONTIER unknowns). Within the touched
  packages it selects `internal/cem/verify` and `internal/contextindex`, both whole. Both ran
  whole through `script/heavy.sh`, one package per command, and passed. The 354 units outside the
  touched packages were not run, per the round's scope. A search found no test outside the two
  packages that asserts on the changed behavior (`link_path` dedupe, refused-link selectors or
  `TestMain` guard widening).
- `make spec-requirements` reported no changes. `make spec-requirements-check`,
  `script/check-line-citations.sh` and `make windows-vet` passed. The CEM rebind for the whole
  range is recorded in the lane handback.

## Rollback

Revert `ce913f3f8` and `2f8d1ef50`. Neither changes a wire field: the packet keeps its keys. Only
which rows and refusals appear and which tests are guard candidates change.
