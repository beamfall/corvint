# 2026-11-16: Ruby admission review repair: bounded reach, strict controls, RSpec unqualified (V1-4365)

> Superseded in part by `2026-11-16-r1ruby-binding-signature-repair.md`: the review of this
> round found that the binding signature of a `=>` line dropped sigils and punctuation. The
> signature rule and the record (report `173e91d5…9b78`, selector `e5c974af8`) are replaced
> there; the rerun has the same faults, selected-set sizes and verdicts.

Requirements: LPCV-V0-7605, LPCV-V0-7608, LPCV-V0-7609 (each amended), LPCV-V0-963, GLS-V0-032.
Decision: 0601. Lane base `c7dbf717e2b2d25dd83b3f26fb7ac7def95587b6`; reviewed head
`8c1202a4d95c4ee3d5e10f3385736882c5becd47`. The independent review of
`2026-11-16-r1ruby-ruby-line-admission.md` returned CHANGES_REQUIRED with six findings. This
entry records what each one changed and the rerun; it replaces that entry's line rule, record
and Ruby family state.

## Findings and outcomes

| Finding | Outcome |
|---|---|
| P0: a counted line's hit set was taken as its full reach | Fixed. A hunk narrows only where its hits bound the reach (below). |
| P1: no fault narrowed, so 0 misses held for any selector | Fixed. A runner qualifies only with a killed fault on a narrowed set; faults were seeded on lines only tests reach. |
| P1: the RSpec control kept 4,371 tests | RSpec is not qualified. The strict reading of `GLS-V0-032` is now the rule in the fold and in admission. |
| P2: the collection state left out the environment | Fixed. The state binds the Ruby and gem environment, declared variables and each isolated run's runner. |
| P3: three helpers lived in a file the record does not bind | Fixed. They moved into `tools/linecov/main.go`; a test keeps the Ruby commands off unbound declarations. |
| P2: missing ticket trailers | The closing commit of this round names the ticket and lists the earlier commits; none was rewritten. |

## Reach rule (LPCV-V0-7605)

A probe of Ruby 3.3.8 line coverage (`r1/probe/`) measured, with each method called once:

| Construct | Counted lines hit | What ran uncounted |
|---|---|---|
| `kind(3)` over `when 1 then :one` / `when 3 then :three` | the `case` line and the second `when` line | the first `when` condition (its line counts 0) |
| `handlers.keys` over a multi-line hash of lambdas | the first element's line only | the second element's key (its line counts 0) |
| `local(false)` over `if flag` / `x = 1` / `end` / `x` | the `if` line and the read | nothing, but the unexecuted `x = 1` makes the later `x` a local |
| a `lambda do` / `_1 + 1` / `end` whose arity is read | the `lambda` line and the read | nothing, but the unexecuted `_1` gives the block one parameter |
| a multi-line `foo = [` literal | the first element's line | the opening line has no counter |

So a replaced hunk selects the tests that hit its old lines, and none when no test hit them,
only when each old line is a counted line holding one whole statement, each new line holds one
whole statement, and both sides bind the same names in the same order. A whole statement begins
and ends on its line outside every bracket and literal, with no block keyword, second statement
or continuation. A binding is compared as the line's text through its last assignment operator,
or its whole word and operator sequence when it holds `=>`; a line with a named capture or a
numbered or implicit block parameter is never bounded. Every other replaced hunk selects the
enclosing called method's tests, or falls back outside one.

`TestLiveLineReach` proves the four review reproductions end to end on a collected Minitest
project: each edit is committed, the suite at the edited head fails exactly one test, and the
plan keeps it. Against the reviewed head all four plans were `exclude`
(`r1/finding1-failing-before-fix.log`). The `lines_test.go` case that asserted no selection for
an edit declaring a local was corrected with the code.

The structure guard and the whole-statement check now share one continuation rule: a method
name's `?` or `!` suffix and a literal at a line's edge are not continuations. Before that, the
rerun's Minitest kills on test-reached lines (`raise E, "msg"`, `reset!`) fell back as
block-structure changes (first rerun, `r1/qual-run1/`).

## Qualification rerun

WebMock `f90fd503000279076c02269299067e08bf1ca1ee`, seed 20261116, selector
`e5c974af8b2d6a49b228b151b800836c58654a27`. Faults: the seven faults and the control of the
first run in their seeded order, then faults on lines only tests reach. For each runner these
are the `lib/` lines its index records as hit by a test and not by the load bucket, mutated by
the first operator the line admits and kept when the mutated file passes `ruby -c`: all 6 for
Minitest, and a seeded sample of 7 of 90 for RSpec. Every fault ran both complete suites; a
failing test outside the selected set is a miss.

| | Minitest | RSpec |
|---|---|---|
| Baseline | 23 tests, 0 failing, 0 skipped | 4,846 examples, 0 failing, 0 pending |
| Index | every test recorded | 11 files kept whole (4,371 tests; 2,930 outcomes unrecorded) |
| Faults | 20 | 20 |
| Killed | 9 | 13 |
| Killed on a narrowed set | 2 (1 of 23 tests each) | 3 (4,389, 4,383 and 4,383 of 4,846) |
| Selected-set sizes | 23 ×7, 1, 1, 17, 17, 23, 23, 23, 0, 23, 23, 0, 23, 0 | 4,846 ×7, 4,371, 4,371, 4,846 ×5, 4,389, 4,846, 4,846, 4,378, 4,383, 4,383 |
| Misses | 0 | 0 |
| Control (`query_mapper.rb:107`, 0 hits) | selects 0, kills 0 | kills 0, keeps 4,371 (all kept whatever the change) |
| Verdict | qualifies | no control: not qualified |

The seven first-run faults still keep the whole suite in both runners (six reach load-time
code, one changes block structure). The Minitest suite reaches only six `lib/` lines outside
the load bucket, because 18 of its 23 test windows load a file or call `eval` and so join the
load bucket; its two narrowed kills are the two `raise` lines of `api.rb`.

`linecov ruby-qualify -record` wrote the record (report SHA-256 `173e91d5…9b78`) at commit
`dad28ab03`, which touches only `.github/`. The selector built there admits Minitest for all 21
fault heads (effective plan equal to the candidate, no failing test dropped, the control's
effective plan empty) and refuses RSpec for all 21 (`ruby qualification does not qualify
runner`, static plan of 4,846, `coverage-collector-unqualified`).

The first rerun, at selector `93b413dd0`, had the same kills and 0 misses, but no Minitest kill
narrowed; it is retained under `r1/qual-run1/` with its report (`0048e48c…7cd1`).

## Ruby family state

**QUALIFIED** for `ruby 3.3.8 minitest 5.26.2 coverage=lines,methods`, at a revision that holds
the record and the 38 bound blobs. **UNQUALIFIED** for
`ruby 3.3.8 rspec 3.13.7 coverage=lines,methods` and every other Ruby tuple: their plans stay
static.

RSpec stays unqualified until its index records every test and its control keeps none. That is
not reachable on this corpus without dropping tests that can fail:

- 2,927 outcomes reach more code in suite order than alone (for example `WebMock.after_request`
  callbacks registered by earlier examples run inside later ones), so their isolated record does
  not bound them.
- Two `spec/quality_spec.rb` examples start `git` and `gem build` children the collector cannot
  observe, and one of them reads every tracked file, so any edit can fail it.
- `spec/unit/errors_spec.rb[1:1:1:6:2]` fails when run alone.

V1-4365 criterion 6 (a public Ruby corpus qualified with Minitest and RSpec) is therefore met
for Minitest only. Criterion 3 holds for the qualified runner.

## Collector (LPCV-V0-7609)

The state binding (`rubylinecov-state/2`) adds digests of `PATH`, `RUBYOPT`, `RUBYLIB`, every
`GEM_`, `BUNDLE_`, `RUBY`, `RBENV_`, `RVM_`, `CHRUBY_` and `ASDF_RUBY` variable, and each
variable `ruby-collect -env` declares. A resume under another value is refused naming the
variable; an isolated run that reports another runner than the retained ordinary-order run stops
the collection and is not retained.

The Minitest index was collected again with the repaired collector (`-env NO_CONNECTION`,
6 workers on 12 cores): every per-line and per-method test set equals the first index; only the
recorded per-test durations differ. The RSpec index was not collected again: it takes about two
hours of isolated runs, the collector script digest is unchanged (`66e20a18…f931`), the merge
code is unchanged, and RSpec is not qualified. The rerun used the first RSpec index.

## Retained evidence (outside the repository, `/home/agent/lanes/r1ruby-evidence/`)

| File | SHA-256 |
|---|---|
| `r1/ruby-report.json` (fault-run report) | `173e91d593cf76966edcbe49f37b5092e78ac1947906b3521709c01f04319b78` |
| `r1/ruby-qualify-summary.json` | `3fad1fb4484a53340bade5af5445c439b9f341a3fde62b9a67366e442f6b972d` |
| `r1/admitted-plans.json` (selector `dad28ab03`) | `9e04fc2171ee71b4bda2fefc2d25fc3b90018cbf5286500f42eac86d551f6ca6` |
| `r1/minitest.idx` | `4d3ab5e779b3dfd88a93425e852442dbad5f212eefaa0c88e7326c0625b20b0e` |
| `rspec.idx` | `438274033870c9a0408f668fe7b6b8e3168ef4ee980d82675f706b8a1277f782` |
| `r1/minitest.lines.jsonl` (per-line dump of the index) | `cf7152062d47e7b44862553f86f62d7573927dd8da57ca9d13e891451fa01bb2` |
| `r1/rspec.lines.jsonl` | `a5f94684760c60d234dd521800bf4af089bf8fbffe3fe76eab72e4eb588644f2` |
| `r1/harness/qualify-ruby.py` | `a108837776c88b395761a891f34caaa0edb233d8119125905ad7bfed034f4d02` |
| `r1/harness/admitted-plans.py` | `97729db1f00d80f4e40e7d4bf38ad03e90388146df9f760a1e0eb41328fd745f` |
| `r1/qual-run1/ruby-report.json` (first rerun) | `0048e48c2cd6e312804862f0bac4b73314c8e3309143a4060eb3e226f78f7cd1` |
| `r1/probe/probe.out` (Ruby 3.3.8 coverage probe) | `a88b9efb682f1e34b844537cb500526d041e45660c66c8a60e8a55180bb25713` |

Each code finding's test output against the reviewed code is under `r1/` as
`finding*-failing-before-fix.log`.

## Not done here

- Corvint Go requalification (V1-4365 criterion 4): the Go record was already stale and this
  round changes three more of its bound files (`internal/linecov/admission.go`,
  `tools/linecov/main.go`, `tools/linecov/qualify.go`).
- RSpec qualification, for the reason above.
