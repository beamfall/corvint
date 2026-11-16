# 2026-11-16: Ruby lexer comparison repair and record rerun (V1-4365)

Requirements: LPCV-V0-7605 (amended), LPCV-V0-7608, LPCV-V0-963. Decision: 0601. Lane base
`c7dbf717e2b2d25dd83b3f26fb7ac7def95587b6`; reviewed head
`2f3c737f891d43c94703d8bc5c4811ba29870fb2`. The independent review of
`2026-11-16-r1ruby-blank-literal-repair.md` returned CHANGES_REQUIRED with one P1. The owner
allowed this round on 2026-11-16. This entry replaces that entry's rule for a file that may hold
a blank-delimited percent literal ("every line is compared exactly as written"), its statement
that the structure guard has no live miss through such a literal, and its record. Everything
else in that entry stands.

## Finding (P1): the blank that closes a literal was never compared

In a file that may hold a percent literal a blank delimits, a line was compared exactly as
written, but only through its last assignment operator, and not at all when it had none. The
blank that closes the literal sits after the operator:

- `p 1, % a ` ends the literal at its last blank;
- `p 1, % a` runs it onto the next line, `\t\ty=1 ; x = 2`, and swallows `y=1`.

Both compared equal, and so did `z = % a ` and `z = % a`. On a counted line no test hit, the
edit was planned `exclude`, and the test that reads the later `y` failed at the edited head
(`r4/failing-before-fix.log`, both spellings, with and without Ruby).

**Fix, as the review asked.** When either side of the file may hold such a literal, no replaced
or deleted hunk of that file is bounded by its lines' text or hits. Each selects the tests of
the enclosing called method and its body lines, or falls back outside one, as an insertion
already did. The exact comparison is gone: `binding` has one mode, the squeezed text through the
last assignment operator, and it is called only for a file with no such literal.

## Second finding: that rule is not enough, and no rule above the lexer is

The tests of the enclosing method bound an edit only if the edit stays inside the method. Where
Ruby reads a literal that the lexer of `internal/rubydialect` reads as code, it need not.
Measured on Ruby 3.3.8 (`r4/probe/cross.out`); in each case both sides parse, the structure
guard sees the same keywords and brackets on both sides, and the failing test calls another
method than the one edited:

| Edit of one line of method `a` | What Ruby reads | Result |
|---|---|---|
| `z = % a ` to `z = % a` in a tab-indented file | the literal runs to the next space, four lines on, past the `end` of `a` and a later `def b` | the later `b` is gone |
| `z = % " ` to `z = %  " ` | the literal closes empty; the quote opens a string that runs to the next quote | the later `b` is gone |
| `z = %` at a line's end to `z = 1 %` | a newline-delimited literal becomes the modulo operator, so the next line turns from its body into code | `y` is redefined |

Before the fix each planned only the test that calls `a` and dropped the failing one. The first
two are in a file the blank rule matches, so the rule the review asked for does not close them.
The third holds no blank after a percent sign at all.

**Fix.** A file is narrowed only when the index's Ruby reads both of its sides as the lexer
does. `Sources.Lex` is the interpreter's own lexer (Ripper, run by `RubyLexer`). For each line
it gives the code text, with every comment, string, percent literal, regexp, character literal
and heredoc opener blanked, and whether the line begins or ends inside a literal, a heredoc
body or a block comment. `AnalyzeLines` compares that with the lexer's reading of the old and
the new side. A line that differs, a side Ruby does not lex, or no lexer keeps the static plan
and reports the file and line. `ruby-select` passes the interpreter of its `-ruby` flag, whose
version the syntax check already ties to the index runner.

Every literal and comment begins with a byte that is not a blank, so two readings that agree on
a line's text agree on which of its bytes are code. Blanks at a line's end are not compared.

What the comparison finds (`r4/lexer-agreement.log`, Ruby 3.3.8):

| Corpus | Files read alike | Files read differently | Files the blank rule matches |
|---|---|---|---|
| WebMock `lib/` | 61 | 0 | 0 |
| WebMock `minitest/`, `test/` | 7 | 0 | 0 |
| WebMock `spec/` | 60 | 2 | 0 |
| Ruby 3.3 standard library | 933 | 53 | 112 |

The 53 are real misreads of the lexer: two divisions on one line read as a regexp, a quote in
`$'`, `` $` `` or `$"` read as a string opener, a multi-line regexp, an operator method's name
beside its symbol. Each could have hidden an edit in the same way.

**What it costs.** A file Ruby reads differently is never narrowed: about one file in twenty of
the standard library. A file the blank rule matches is narrowed only to the method: about one
in nine there, because `a % b` and `"%d items"` match. No `lib/` file of WebMock is in either
group, so the qualification's selected sets do not change. Each changed file costs two more runs
of the interpreter.

The blank rule is redundant where the lexer comparison runs: if Ruby and the lexer agree on both
sides, Ruby read no blank-delimited literal there. It is kept as the rule that holds without
Ruby's reading, as the review asked.

## Tests

In `internal/rubylinecov/reach_test.go` unless named otherwise.

- The reach fixture is two files. `lib/r.rb` holds no blank-delimited literal; `lib/q.rb` holds
  the four round-3 percent methods and the review's two.
- `reachEdits` has both spellings of the review's pair. `TestLiveLineReach` commits each of the
  20 edits on a collected Minitest project. For `lib/r.rb` it asserts that Ruby reads the file as
  the lexer does and that the plan is the one the tests without Ruby compute. For `lib/q.rb` it
  asserts the fallback of a file Ruby reads differently. In both, the test that fails at the
  edited head is kept.
- `TestLiveEditBeyondTheLexer`: the three edits of the table above, live.
- `TestEditBeyondTheLexerNeedsRuby`: the same three without Ruby. With a lexer that agrees, each
  selects only the edited method's test, which is the gap. With no lexer, or a reading that
  differs in code, in either state, in length or by error, each keeps the static plan.
- `TestLiveRubyLexerAgainstTheLexer`: Ruby and the lexer agree on a source with every kind of
  literal, and differ on the right line for nine constructs the lexer misreads.
- `TestBlankDelimitedFileBoundsNoHunk` (was `TestBlankDelimitedFileComparesExactText`) and
  `TestAnalyzeLinesBlankDelimitedFileHunkKinds`: a replaced, a deleted and an inserted line of a
  matching file select the enclosing method's tests.
- `TestAnalyzeLinesBoundedReach` has squeezed-mode bounded cases on `lib/r.rb` that run without
  Ruby: a re-indented local and blank runs around its operator select no test.
- `TestLanguageDecideRubyAdmission` (`tools/linecov`): an admitted plan whose edited line Ruby
  reads as a literal keeps the static plan.

Assertions that changed, all in a file the blank rule matches, all from bounded or
"bindings change" to the new rule:

- three reasons in the former `TestBlankDelimitedFileComparesExactText`, from
  `a line whose bindings change` to `a line of a file that may hold a blank-delimited percent
  literal`. The hunk was unbounded before and is unbounded now;
- the exact-mode grouping of `TestLiveBindingSignatureAgreesWithRuby` is removed, because
  nothing is compared in that mode. Its 1,758 lines with such a literal are now asserted
  unbounded instead;
- the round-3 case "re-indented local in a file with a blank-delimited literal" moved from
  `lib/r.rb:19` to `lib/q.rb` with the same expectation, the method's test.

## Qualification rerun

The record binds `internal/rubylinecov/lines.go` and `tools/linecov/languages.go`. The fault run
was repeated with the selector built at `80fa8d4084554f2cd091f4132a0ff45f409c98d5`. Nothing else
changed:

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
runners. The failing sets are equal too, with one exception. At the
`lib/webmock/util/query_mapper.rb:212` fault the RSpec suite failed 143 examples, where the
previous run failed 142: `spec/acceptance/httpclient/httpclient_spec.rb[1:6:1:7:17:2]` failed
this time. The plan keeps all 4,846 examples there, so neither run is a miss.

`linecov ruby-qualify -record` wrote the record at commit `294b2be1f`, which touches only
`.github/`. The diff is the report digest (SHA-256 `d2445c4c…99eb0`) and the blobs of `lines.go`
and `languages.go`.

The selector built at `294b2be1f`:

- admits Minitest at all 21 fault heads, gate-eligible, with the effective plan equal to the
  candidate, no failing test dropped and the control's effective plan empty;
- refuses RSpec at all 21 (`ruby qualification does not qualify runner`, static plan of 4,846,
  `coverage-collector-unqualified`).

The Ruby family state is unchanged:

- **QUALIFIED** for `ruby 3.3.8 minitest 5.26.2 coverage=lines,methods`, at a revision that holds
  the record and its 38 bound blobs;
- **UNQUALIFIED** for `ruby 3.3.8 rspec 3.13.7 coverage=lines,methods` and every other Ruby tuple.

V1-4365 criterion 6 is met for Minitest only.

## Not closed

The comparison proves that the lexer read the literals and comments of a file as Ruby does. It
does not prove the rules above the lexer (which words are block keywords, what continues a
line, what a line binds); those stand on the earlier rounds' tests. A Ruby without Ripper, or
older than the `raise_errors` option of `Ripper.lex`, has no reading, so nothing is narrowed
there.

## Retained evidence (outside the repository, `/home/agent/lanes/r1ruby-evidence/`)

| File | SHA-256 |
|---|---|
| `r4/ruby-report.json` (fault-run report) | `d2445c4c8c2e341d40a56fae6a1cc70da56d7177eb9dffc3daa07f648ab99eb0` |
| `r4/ruby-qualify-summary.json` | `bc6b8036a2dc2198cf58048abfad19dba6dcf5519bdda198982d920f9394507c` |
| `r4/admitted-plans.json` (selector `294b2be1f`) | `03fdaeac9d4cc29526c17e4486f2afdcde1af907d094b080a6ef4a4e60168387` |
| `r4/harness/qualify-ruby.py` (the round-2 harness with round-4 paths) | `e3dd924d980711ac2dc9305b49efc94f5821f650933df3c3a6a253f25c725399` |
| `r4/harness/admitted-plans.py` | `72f4172909242797d5b1d40e8a04df28b3d7b435b5cbbb6df0df98a5fb116e12` |
| `r4/probe/cross.out` (the review's pair and the cross-method edits on Ruby) | `975096f63a998d887f974664ffdd4e2cf98de6912ea074ed8f1b22be17ea53e2` |
| `r4/lexer-agreement.log` (the lexer against Ruby on WebMock and the standard library) | `415f47e6a218fd7f44252eec06772fb7ca7ccbca1f79c716b180b8c81350ab17` |
| `r4/failing-before-fix.log` (the new tests against the reviewed head) | `b9e58d3127ba48fface084d9d26dbfebc39f18f08ba99f48900c42081d242ceb` |
