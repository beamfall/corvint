# 2026-10-12 CEM citation baseline over the 593 committed maps (V1-1262)

Branch `claude/v1-1262-cem-tiers` at 2f9f32d4e (lane bbdc73b61 merged with `origin/main`
f0d0e6106). Decision 0523 P0, requirements CEM-V1-080..082 (proposed, renumbered +20 in the same
lane). This is the pre-migration reference measurement that decision 0525 §3 step 1 names: the
citation block of every committed `cem/0.2` map, computed by the native consumer before any map
is converted to `cem/1.0`.

## Method

`corvint cem status --map .corvint/changes/<bind>.cem.json --expected-base <baseRevision>
--target <bind>` for each of the 593 maps under `.corvint/changes/`, binary built from the lane
before the review repairs below. The per-map `verification.citation` block was retained; the two
maps the `origin/main` merge added were run afterwards with both the pre-repair and the repaired
binary (byte-identical output), as were three sampled maps from the main run. Retained evidence:
`evidence/v1-1262/citation-baseline-593-maps.jsonl` (one compact row per map: class counts,
distinct evidence, fan-out, concentration, invariant flag, specificity) and
`evidence/v1-1262/citation-baseline-summary.json` (aggregate, with the specificity and fan-out
histograms). The full per-hunk blocks (3.3 MB) are reproducible from the command above and were
not committed.

## Result

| Measure | Value |
| --- | --- |
| Maps | 593 (`cem/0.2`), 592 with a citation block |
| Hunks classified | 20604 |
| `held` / `mechanical-proved` | 0 / 0 |
| `anchored` | 1198 (365 maps have at least one) |
| `cited` | 19367 |
| `unknown` | 39 hunks in 33 maps |
| Annotation hunks | 30 in 22 maps |
| Invariant maps | 180 of 592 (171 of them cite exactly one evidence item) |
| Specificity | 220 positive, 327 zero, 45 negative |
| Max fan-out ≥ 100 | 17 maps (largest 291) |

One map, `f0fda542` (877 hunks, 418 KB), fails with `git-budget-exceeded` before any citation is
computed. The `origin/main` 5e664ce9c binary, which predates the citation code, fails the same
way, so this is a pre-existing verifier budget limit, not a P0 effect. It is retained as a known
limit for the migration run (decision 0525 §4): a map the consumer cannot verify before conversion
is not a conversion difference.

Reading against decision 0523's expectation: no committed map reaches `held` or
`mechanical-proved` (as predicted, since no producer emits runner or structural witnesses yet);
about three in ten maps are invariant (0523 estimated about half), and the 171 single-evidence
invariant maps are the dogfood `chore: bind` and `chore: seal` maps that cite one evidence item
for every hunk. The 45 negative-specificity maps and the 17 maps with fan-out of 100 or more are
the concrete targets for P1's cover and discrimination work (V1-1263/V1-1264).

## Review repairs

Codex review (gpt-6-astra, read-only) of the lane diff raised two confirmed findings, repaired in
the same branch with no fixture, golden or baseline change:

- `internal/cem/verify/verify.go`: the retained span was a window into the whole blob, so every
  record kept its blob alive; the verifier now copies the span.
- `internal/cem/verify/support.go` and `interop/cem01-go/citation.go`: the defining-line scan
  split the span into a line slice once per cited ID; both consumers now scan each span once per
  evidence item with a streaming loop and look IDs up in the resulting set.

Evidence: the P0 citation tests in `internal/cem/verify`, `internal/cem/workflow` and
`interop/cem01-go` pass unchanged after the repairs, and the sampled maps above are byte-identical
between the pre-repair and repaired binaries.
