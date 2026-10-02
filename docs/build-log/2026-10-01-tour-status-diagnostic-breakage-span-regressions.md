# Tour status diagnostic and breakage span regressions — V1-0518 / V1-0517 / V1-0414

Owner-approved leftover work (2026-10-01) on top of `d2425f7f` and `29902ae5`. Base is
`25ba271a1f34cc7de5990ef1f2662135f6ab1b0d`.

## V1-0518: proof tour Git status diagnostic

`d2425f7f` made a failed `git status` refuse with `git-status-failed`, but Git's own message
went only to the tour's transient stderr. A baseline run of `script/proof-tour-test.sh`
against a real paused tour showed the corrupt-index case leaving an empty `receipts/`
directory while `fatal: .git/index: index file smaller than expected` existed only in the
caller's capture. The acceptance criterion asks for the original diagnostic to be retained.

Resume now computes and preflights its round before the status read and runs status as the
round's `status` receipt step, so argv, stdout, stderr, exit and timeout are retained like
every other owned command. A round counts as used once its status or patch receipt exists;
a retry after a refused status starts the next round and leaves the earlier refusal intact.
Refusal precedence moves only among existing refusals: output collisions are now reported
before dirty-fixture and stale-byte refusals. `PT-V0-004` and `PT-V0-007` state the retention
and the retry test.

The new assertions failed against the `origin/main` script (`status-exit-not-retained`) and
pass after the change. A separate mechanical clean resume, run on a private copy with a mock
`MOCK_MECHANICAL_ONLY` ACK, reached `outcome=complete`; it only shows that the extra status
step does not break the positive path and is not independent-review qualification. The actual
independently acknowledged positive tour was not rerun.

## V1-0517: breakage map span regressions

`29902ae5` added physical-line anchors with checks. These regressions were added:

- `TestBreakageLineDirectivesOutsideNarrowedSpan`: inner `//line` directives in the declaration
  and the caller, with a supplied span that ends inside the physical span. Before the fix both
  panicked with `slice bounds out of range [2:1]`. After the fix both stay unresolved with their
  named unknown, and no syntax witness is emitted.
- `TestBreakageOversizeProviderAnchorCannotAdvance`: an oversize captured provider span panicked
  before the fix (`[:1000000] with capacity 2`) and now stays unresolved.
- `TestBreakageSuppliedSourceSpanBounds`: reversed, zero and oversize manifest spans, and a
  span past the blob, are refused. This guard predates the fix, so it passes on both sides and
  is a boundary regression, not a red/green one.

The red runs reverse-applied only the `29902ae5` source hunks onto the current tree. The
breakage spec now states the physical-line and bounded-anchor rule under `BKM-V0-003` and names
the tests. Fresh independent re-review is not part of this change.

## V1-0414: hidden Unicode in governing files

Not delivered. `internal/contextindex` has no zero-width, bidi-control or tag-character
screening, and packet/CEM have no self-modified authority signal. No docs cite the two external
threat references. Only `docs/RELEASE-NOTES.md` and `agent-instruction-doctor-v0.md` mention
the gap. All three criteria are UNMET, and the ticket stays open outside this change.
