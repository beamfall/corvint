# 2026-10-04: writer-screen prefilter (V1-0722)

Human-owned intent: ticket V1-0722, recorded as owner-directed, proposed `LTA-V0-014`. What the
writer screen detects does not change.

## Finding

`secretscreen.Pattern` is one case-insensitive alternation of 37 top-level branches. Go's regexp
runs it as an NFA simulation whose cost is the sum of its branches: timed alone on a 664 KB
captured input, the 37 branches took 11 to 84 ms each and 750 ms together, against 705 ms for the
whole pattern. The three assignment branches and the JWT-shaped branch are the dear ones (about
82 ms each). Nearly every screened text can match almost none of them.

## Change

- `internal/secretscreen/prefilter.go`: each branch carries a necessary condition, evaluated on
  the text with ASCII letters lowered: a required literal, or a short scan (a credential field
  name followed by `:` or `=`, three dotted token runs, `sk` plus 32 hexadecimal digits,
  `xox?-`). The screen runs `Pattern` restricted to the branches whose condition holds, in the
  same order, compiled once per subset and cached (128 subsets, then the complete pattern). No
  live branch means no match. A branch that can match nowhere in a text contributes nothing to a
  leftmost-first search, so the matches are those of `Pattern`.
- A text holding U+212A or U+017F, the two non-ASCII runes that fold to an ASCII letter under
  `(?i)`, is screened with the complete pattern.
- The Go verbose `PASS:` masking copy and its marker scan are skipped when the text lacks the
  literal `--- PASS: ` every marker contains.
- `Pattern`'s source text is unchanged (digest pinned by `TestLTAV0014PatternSourceIsUnchanged`);
  it is now assembled from the branch list. `StoredV1Pattern` is unchanged (existing digest test).

- The analyzer schema moves to `corvint-analyzer/102`: `secretscreen` is one of the inputs pinned
  by `TestAnalyzerSchemaInputs`, whose rule is to bump on any change to them. Extracted facts are
  unchanged; existing analyzer packs are rebuilt once.

Not done: `behaviorfalsify.VerifyReceipt` still screens each retained byte field and then the
encoded document. Dropping either pass would change what a receipt is refused for, which is a
detector decision and not an optimization.

## Evidence and limits

Measured on darwin/arm64 (Apple M2 Max), go1.27.1, without `-race`, the complete pattern against
the screen on the same input in the same process:

| Input | `Pattern` | Screen |
|---|---|---|
| captured 87,393-byte encoded evidence receipt | 90.9 ms | 5.4 ms |
| captured 12,673-byte native report | 12.4 ms | 0.67 ms |
| all 2,058 texts screened by `internal/behaviorfalsify` tests and `TestPTFV0ParentCompleteAttemptOutcome` (7.7 MB) | 8.25 s | 0.40 s |
| `BenchmarkScreen/receipt-87KB` (synthetic, retained) | 85.8 ms | 3.6 ms |
| `BenchmarkScreen/native-report-13KB` (synthetic, retained) | 13.2 ms | 0.31 ms |

Package time under `-race`, same host, before at `431d6b15` and after:

| Package | Before | After |
|---|---|---|
| `internal/behaviorfalsify` | 128.2 s | 90.9 s |
| `internal/testacceptance` | 1272.5 s | 14.2 s |

The `internal/testacceptance` baseline is one observation, taken while the host ran other test
jobs. A repeat on a quiet host gave 123.3 s for `internal/behaviorfalsify` and was stopped at a
600 s limit with `internal/testacceptance` still running, so the baseline is above about 470 s;
its exact value is not established.

- Equivalence: `TestLTAV0014PrefilterReturnsPatternMatches` compares the screen with `Pattern` on
  every parity fixture, its upper-case, fold-rune, split, JSON-string and base64 variants and every
  ordered fixture pair (about 6,000 texts), and checks each branch's condition wherever that
  branch alone matches. The package `TestMain` applies the same check to every text any existing
  test screens. `FuzzLTAV0014PrefilterReturnsPatternMatches` ran 90 s locally (312,439 executions)
  with no disagreement.
- `TestLTAV0014GeneratedBranchTextsKeepTheirBranchLive` draws 12,000 texts from the branches' own
  expressions (fixed seed, boundary runes preferred, subset cache emptied every 100 texts) and
  requires every branch to be matched by at least 30 of them.
- Mutation check: 29 hand-made unsound mutants of the conditions (19 mine, 10 from the review)
  each fail the tests. Three of mine survived a first pass and have boundary fixtures now. The
  independent review found no defect in the conditions but seven more surviving mutants (a
  dropped `ghr_`, `_test_`, `shpca_` or `asia` literal, `sk-` narrowed, `curl` requiring a space,
  the Slack hook requiring `https`); the generated test was added for them and rejects all seven.
  The differential test also caught a real defect during development (an overlapping `xoxoxb-`
  occurrence skipped by the scan).
- `corvint affected` selects 186 units for this change, because nearly every package depends on
  the screen. Run locally: `internal/secretscreen`, its direct importers, and the two ticket
  packages under `-race`. The rest is left to the pull request's full run.
- Hosted race times for the two packages are `NOT_OBSERVED` until this change's pull-request run.
  `make gate` was not run (`NOT_RUN`).
- A text in which most conditions hold is screened at close to the old cost. The conditions are
  necessary, not sufficient.

## Rollback

Make `writerMatches` call `Pattern.FindAllStringIndex` directly and delete `prefilter.go`. No
stored state depends on the prefilter.
