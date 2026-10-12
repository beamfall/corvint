# Reviewer-trial scorer precision and registration (V1-1261)

Intent: decision 0523 P4. Requirements CRT-V0-013, CRT-V0-014 and CRT-V0-015 and the 2026-10-11
amendments to CRT-V0-005, CRT-V0-007 and CRT-V0-010 in `docs/specs/cem-reviewer-trial-v0.md` are
proposed. Baseline: `f0d0e6106d8f7ffeaac46339ba274df3aafc712f`. Scope is the scorer and the
harness's run-time checks; the trial run itself is post-rc.3 and `NOT_RUN`.

## Contract and implementation

- Precision (CRT-V0-013): every scored lane records `cited_not_gold` beside `miss`. The path rate
  is distinct cited paths outside gold over distinct cited paths. The span rate is merged distinct
  cited lines outside the same gold path's recorded secondary spans over distinct cited lines. A
  malformed range counts as one not-gold line. Both rates are `NOT_OBSERVED` for an empty citation
  set; the span rate is also `NOT_OBSERVED` for a report with no `gold_spans`. Arm blocks and the
  seeded contrast report per-arm means. A scripted cite-every-suggestion agent wins on miss
  (Δ 0.5) and loses on both rates in `TestCiteEverySuggestionAgentLosesOnCitedNotGold`.
- Registration (CRT-V0-014): `run --registration FILE` reads a closed JSON object (profile,
  hypotheses, primary_metric `miss`, alpha `0.05`, n, scorer_sha256, model, effort, partition).
  The file must be committed byte for byte at `HEAD`; the report records its path, digest, commit
  and committer time. The scorer digest is the sha256 of the embedded
  `{gold,main,register,score,select}.go`, so
  `cat tools/cem-trial/{gold,main,register,score,select}.go | shasum -a 256` reproduces it. A
  heldout run needs a registration; mismatches are refused before dispatch. On rescore the report
  is invalid if the commit is newer than the earliest lane `started_at`, if no start was recorded,
  or if any field or the running scorer disagrees.
- Treatment arm (CRT-V0-015): the registered treatment is the shipped profile's artifact. The
  harness cannot build it yet, so a registered run with the `treatment` or `seeded` arm, both still
  carrying the all-`unknown` cem/0.1 worklist, is refused and invalid on rescore. Any run before
  decision 0523 P2 has a baseline is invalid as outcome evidence; the 2026-09-04 pilot is one.

Rollback: revert this change. Prior reports still score: the span rate reads `NOT_OBSERVED`
without `gold_spans`, and a report without a registration stays valid unless it is `heldout`.

## Limits

Committer times are self-asserted, so the order check catches a late registration by mistake, not
a forged date. Registration before selection is a procedural obligation the tool does not check:
the manifest records no selection time, so the tool compares only with the first lane. Gold spans
are new-side ranges at `C` while citations are at `C^`. Gold is test-or-spec only, so a valid
implementation citation is not gold in every arm alike. Read the rates as paired comparisons
between arms, not as absolute error rates. The shipped-profile arm builder follows decision 0523
P2.
