# Native coverage brace-line gap review fix

The independent P2 review found that candidate gap analysis could omit a test
whose only executable coverage shared an opening or closing brace line. This
contradicts LPCV-V0-8207: uncovered body lines must retain all tests reaching
that body. A classified literal edit on line 2 of `Body{Open: 1, Close: 3}`
previously selected neither a block on line 1 nor a block on line 3.

Decision: retain every coverage block whose effective line interval intersects
the containing body's inclusive brace-line interval. Line-only boundaries
cannot establish that executable coverage on those lines belongs elsewhere.
Keep the existing exclusive EndCol=1 adjustment. Hunk classification and strict
edit containment remain unchanged; production native exclusion stays disabled.
This repairs the accepted conservative rule without changing its contract.

`TestSelectionGapBraceLines` covers opening-brace coverage, closing-brace
coverage, a block starting before and overlapping the opening brace, an
exclusive end before that brace, and coverage after the closing brace. It
asserts both selected tests and retained block references. Existing assertions
are unchanged.

Before testing, the lane-built `corvint affected --base
cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419` selected the native package. Its overall
scope remains UNKNOWN (160 selected units, 96 unknowns across the inherited
lane diff); it is not evidence of repository-wide verification. The required
whole-package command, with no exported Git identity, passed:

```sh
GOTOOLCHAIN=local HEAVY_WHOLE_REASON='Review charter requires whole internal/nativelinecov; selected by affected against cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419' script/heavy.sh go test -count=1 -timeout 30m ./internal/nativelinecov
```

Package time was 0.291s; wrapper run 1.2s and slot wait 2s. The configured
shared semaphore worked without overrides. The wrapper warned that /tmp had
2 GiB free; TMPDIR remained the private lane directory. Citation repinning
found no moved content and `make line-citations-check` passed. Go version is
the required go1.27.3.

Original logs remain outside the repository at
`/home/russell/lanes/covlive-evidence/review-fix/`. This fixture regression does
not rerun native live qualification or LPCV-V0-963. Existing family-level
PARTIAL dispositions remain; independent review is reserved to the
orchestrator. Pre-change capture is reused as directed. Final CEM/check
receipts belong in the external handback. No seal, publication or Tasks write
is authorized in this round. Rollback reverts the selector and its regression
together, restoring the known candidate-analysis limitation.
