# 2026-11-11: gob linecov indexes refuse an inverted block span (V1-5371)

V1-5371 reported, from a reading of the code and without a run, that `linecov.Decode` checks a
block's `Start <= End` only for the compact `linecov-index/4` encoding. Base:
`b55635262788687c3eea9a08a94378e4ab8856a4`.

## Probe

`internal/linecov/gob_block_span_test.go` (`TestDecodeRefusesInvertedBlockInGobFormats`) gob-encodes
an index whose one block is `Start 5, End 3` in each gob format `Decode` still reads:
`linecov-index/2`, `linecov-index/3`, and the RSP-V0-008 evidence extension written in gob under
`linecov-index/4`. Before the fix, all three decoded and returned the inverted block:

```text
--- FAIL: TestDecodeRefusesInvertedBlockInGobFormats/linecov-index/2
    gob_block_span_test.go:39: decoded an inverted block: {Start:5 StartCol:1 End:3 EndCol:2 Tests:[0] Segs:[]}
--- FAIL: TestDecodeRefusesInvertedBlockInGobFormats/linecov-index/3   (same)
--- FAIL: TestDecodeRefusesInvertedBlockInGobFormats/linecov-index/4_evidence_extension   (same)
```

The defect is confirmed. The compact decoder cannot produce such a block (it reads `End` as `Start`
plus an unsigned length) and its encoder refuses one, so only the gob path was open.

## Fix

`Decode` now calls `checkBlockSpans` on every gob-decoded index before the segment and evidence
checks. It refuses a block with `End < Start` or a negative line or column, the shapes the compact
encoder already refuses, with `linecov: malformed index: PATH block span S.SC,E.EC`, the same
`malformed index` wording as the compact reader's `block span` refusal. Block order is not checked
here; that was not part of the report and a misordered gob index is a separate question.

This brings the gob reader in line with `Decode`'s documented contract ("refuses ... an inconsistent
body, so a damaged index decides nothing"); no requirement text changes and no ID is added.

## Verification

- Probe before the fix: FAIL (above). After: PASS for all three formats, and a valid
  `linecov-index/3` block still decodes.
- `HEAVY_WHOLE_REASON=... script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m
  ./internal/linecov/`: `ok` in 58.6 s.
- `go vet` and `GOOS=windows go vet ./internal/linecov/`: pass. `go build ./...`: pass.
