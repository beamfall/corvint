# 2026-11-16: Ruby blank-delimited literal repair and record rerun (V1-4365)

Requirements: LPCV-V0-7605 (amended), LPCV-V0-7608, LPCV-V0-963. Decision: 0601. Lane base
`c7dbf717e2b2d25dd83b3f26fb7ac7def95587b6`; reviewed head
`c38c6f9f643ea6b90edb97f34f4d250b3cb68dc0`. The independent review of
`2026-11-16-r1ruby-binding-signature-repair.md` returned APPROVE with one confirmed miss (P2)
and one precision note (P3). The owner allowed this round on 2026-11-16 to fix the miss before
landing. This entry replaces that entry's statement that a binding is compared with each blank
run read as one space, and its record. Everything else in that entry stands.

## Finding (P2): a blank run can delimit a percent literal

The binding signature read each run of spaces and tabs as one space. Ruby 3.3.8 accepts a space
or a tab as the delimiter of a bare, `q`, `Q`, `s`, `r` and `x` percent literal (`r3/probe/letters.out`;
`w` and `i` refuse it). The literal closes at the next blank of the same kind, so the run's
length decides where it ends and what follows:

- `%  =>x ` is `"" => x` and declares `x`;
- `% =>x ` is the string `"=>x"` and declares nothing.

Both had the signature `% =>x`. On a counted line no test hit, the edit between them was planned
`exclude` and the covering test was dropped. The same holds with a tab, with a type letter, and
on the assignment path: `[%q  ,x=1 ]` declares `x`, but `[%q ,x=1 ]` does not
(`r3/probe/probe.out`). The review's reading-only case `foo %  ,x=1 ` does not parse: after a
name and a blank, Ruby reads `% ` as the modulo operator.

**Fix.** When either side of the file may hold such a literal, every line of the file is
compared exactly as written, indentation included. "May hold" means a `%`, at most one letter,
then a space or tab, anywhere in the text. Any other file keeps the old comparison.

Why the whole file, and why a coarse match:

- The lexer (`internal/rubydialect`) does not read a blank-delimited literal, so it cannot say
  whether the `%` is in code, a string or a comment.
- Such a literal can open on an earlier line and run into this line's indentation. Measured
  (`r3/probe/indent.out`): after `p 1, % a`, the line `    ,y=1 , x = 2` declares `y` and `x`,
  because the first blank of the indentation closes the literal. With a tab-only indentation
  (`\t\t,y=1 , x = 2`) it declares only `x`, because the literal runs on to the blank after
  `y=1`. Ruby does not count that continuation line, so the reach rule's uncounted-line path
  already sent it to the enclosing method. The comparison no longer relies on that.

In a file with no such literal, a run of blanks either separates tokens or sits inside a string,
regexp or comment body. Its length can change a value but not a name. No other construct makes
the run's length significant on a signature line:

- `? ` is not a character literal: Ruby reads the `?` as the ternary operator.
- A heredoc body line and a line inside a multi-line literal are never whole statements, so they
  are never bounded.
- A quoted heredoc tag with a blank run changes which line ends the heredoc, which the structure
  guard sees.

**What it costs.** Only in a file that matches, an edit of an assignment or `=>` line that
changes only blanks is no longer bounded by the line's hits. It selects the enclosing called
method's tests, or falls back outside a method. `a % b` and `"%s items"` also match, so modulo
written with blanks and format strings put a file in this mode. No `lib/` file of the WebMock
corpus matches, so the qualification's selected sets do not change.

**String bodies.** Squeezing blanks inside a string body is not a miss. On an assignment line
the signature ends at the operator, so the body is not compared at all. On a line no test hits
the value is never computed, and on a line a test hits that test is selected by the hits.
`TestLiveBlankRunInStringBody` measures this on Ruby 3.3.8 and passed before the fix too:

| Edit | Line hit by | Plan | Suite at the edited head |
|---|---|---|---|
| `s = "a  b"` to `s = "a b"` | `test_text` | selects `test_text` | `test_text` fails |
| a `=>` pattern string from `"a  b"` to `"a b"` | `test_pair` | selects `test_pair` | `test_pair` fails |
| the same assignment on a line no test hits | none | selects none | nothing fails |

**Tests.**

- Four `reachEdits` (`internal/rubylinecov/reach_test.go`), each run by `TestLiveLineReach` and
  `TestAnalyzeLinesUnboundedReach`. They cover a space and a tab delimiter, with and without a
  type letter, on the `=>` path and the assignment path. Each edit fails exactly one test at
  the edited head, and the plan must keep that test.
- Nine pairs in `TestBindingSignatureKeepsLineText`.
- Six percent values in the differential `TestLiveBindingSignatureAgreesWithRuby`, which now
  groups the lines twice: squeezed among lines with no such literal, and exact among all lines.
- `TestBlankDelimitedFileComparesExactText`, for the match and for the earlier-line case.

Against the reviewed head (`r3/failing-before-fix.log`):

- each new reach edit was planned `exclude` and its test dropped;
- the nine pairs shared a signature;
- the differential failed 759 times.

## Note (P3): radix-prefixed numbers

`blockKeyword` looked for a block keyword at the end of any word that starts with a digit, so
`0xdef` and `0xabcdef` were read as holding `def`. Ruby takes a radix's digits as far as they go
(`r3/probe/hex.out`): `0xdef` is one number, `0x1fif` is `0x1f` then `if`, and `0xdeend` is
`0xdee` then the name `nd`. For a number with a `0x`, `0b`, `0o` or `0d` prefix, the keyword is
now looked for only after that run of digits. The line-continuation check reads it the same way.
A decimal word is still searched whole, which only refuses more lines. Rows 43 to 49 of
`TestLineReachRules` test it.

## Qualification rerun

The record binds `internal/rubylinecov/lines.go`. The fault run was repeated with the selector
built at `1102113df87428c8fed38b8dfb1301e05e6132c7`. Nothing else changed:

- WebMock `f90fd503000279076c02269299067e08bf1ca1ee` and seed 20261116;
- the same 20 faults and one control per runner, in the same order;
- the round-1 indexes;
- every fault against both complete suites.

The harness is the round-2 script with only its paths changed.

| | Minitest | RSpec |
|---|---|---|
| Baseline | 23 tests, 0 failing, 0 skipped | 4,846 examples, 0 failing, 0 pending |
| Faults / killed | 20 / 9 | 20 / 13 |
| Killed on a narrowed set | 2 (1 of 23 tests each) | 3 (4,389, 4,383 and 4,383 of 4,846) |
| Misses | 0 | 0 |
| Control (`query_mapper.rb:107`, 0 hits) | selects 0, kills 0 | kills 0, keeps 4,371 (all kept whatever the change) |
| Verdict | qualifies | no control: not qualified |

Every selected set and every fallback reason equals the previous run, fault by fault, for both
runners. The failing sets are equal too, with one exception. At the `lib/webmock/config.rb:9`
fault the RSpec suite failed one example, where the previous run failed two: it did not fail
`spec/acceptance/httpclient/httpclient_spec.rb[1:6:1:7:14:2]`, the example the previous entry
reported as an extra failure. The plan keeps all 4,846 examples there, so neither run is a miss.

`linecov ruby-qualify -record` wrote the record at commit `fba6112dc`, which touches only
`.github/`. The diff is the report digest (SHA-256 `422a261a…c8be3`) and the `lines.go` blob.

The selector built at `fba6112dc`:

- admits Minitest at all 21 fault heads, gate-eligible, with the effective plan equal to the
  candidate, no failing test dropped and the control's effective plan empty;
- refuses RSpec at all 21 (`ruby qualification does not qualify runner`, static plan of 4,846,
  `coverage-collector-unqualified`).

The Ruby family state is unchanged:

- **QUALIFIED** for `ruby 3.3.8 minitest 5.26.2 coverage=lines,methods`, at a revision that holds
  the record and its 38 bound blobs;
- **UNQUALIFIED** for `ruby 3.3.8 rspec 3.13.7 coverage=lines,methods` and every other Ruby tuple.

V1-4365 criterion 6 is met for Minitest only.

## Retained evidence (outside the repository, `/home/agent/lanes/r1ruby-evidence/`)

| File | SHA-256 |
|---|---|
| `r3/ruby-report.json` (fault-run report) | `422a261a073556d9eaa6779c6aa6f213f983b519808e5bad89d7a66c063c8be3` |
| `r3/ruby-qualify-summary.json` | `3fad1fb4484a53340bade5af5445c439b9f341a3fde62b9a67366e442f6b972d` |
| `r3/admitted-plans.json` (selector `fba6112dc`) | `cb340451e218c1dbd4847b1ff4ee0928ffcc90a92d5ddabcf1797a7a52fe26f8` |
| `r3/harness/qualify-ruby.py` (the round-2 harness with round-3 paths) | `61c4d3ca4a69a2d77269ee1d0ded1ee7ca0fe8d9237559b1be80128058a0035a` |
| `r3/harness/admitted-plans.py` | `a2baaf4811c65141662a8e021bd4f64cc798fe62bf89d64cc0268080fbfc2c53` |
| `r3/probe/probe.out` (blank-delimited literals and their locals) | `dbcd0141f4659bce64eb33b2bc7c81418dbc37f838ca2ea57e17e1402fe74324` |
| `r3/probe/letters.out` (percent types that accept a blank delimiter) | `226cf374afc2793640376778df93cf71597ccda246772d39a42b1adfa8142525` |
| `r3/probe/cov.out` (the fixture lines are counted) | `94dd9b509651f6097704a981fb4a08291ae112357a43ee6ef3441f2175380bda` |
| `r3/probe/indent.out` (a literal open into the next line's indentation) | `920be6a3cd11d87760e7e3e872288241d22f6c10c5f99374aa15790d203f440d` |
| `r3/probe/hex.out` (where Ruby ends a radix-prefixed number) | `b1d041361bc81eb8e2ad5dd4a13de078d470126e9e394c4c7ee913da01d74201` |
| `r3/failing-before-fix.log` | `449feb176c0236630637eebc1724cb760795a62dcd56180fd6c0a2c7709df611` |

## Not done here

- Corvint Go requalification (V1-4365 criterion 4). This round changes no file the Go record
  binds.
- RSpec qualification, for the reasons in `2026-11-16-r1ruby-review-repair.md`.
- The lexer does not read a blank-delimited percent literal at all. That affects the
  whole-statement and structure checks as well as the signature. This round handles it inside
  the reach rule; `internal/rubydialect` is outside the round's scope.
