# 2026-11-16: Ruby binding signature repair and record rerun (V1-4365)

Requirements: LPCV-V0-7605, LPCV-V0-7608 (each amended), LPCV-V0-963. Decision: 0601. Lane base
`c7dbf717e2b2d25dd83b3f26fb7ac7def95587b6`; reviewed head
`ec016c34b04af2beaa67edda0c56c52910473d49`. The independent review of
`2026-11-16-r1ruby-review-repair.md` confirmed its six fixes and returned CHANGES_REQUIRED with
one blocking finding and one note acted on here. This entry replaces that entry's signature rule
and record; its reach rule, family state and collector sections stand.

## Finding (P1): the `=>` signature dropped sigils and punctuation

A replaced hunk narrows to its lines' hits only when both sides bind the same names. On a line
holding `=>` the signature of those names was the line's words plus its `=` and `=>` tokens. It
dropped everything else, so `x = {:a => 1}` and `@x = {:a => 1}` compared equal. On a line no
test hit, the edit was planned `exclude` with no test selected, although the first line declares
the local `x` and the second does not.

The signature is now the line's own text with each run of spaces and tabs read as one space:
through the last assignment operator when the line assigns, and the whole line when it holds
`=>`, whose target follows the operator. Nothing that tells two lines apart is dropped, so two
lines share a signature only when they declare the same names. Comment and string text is
compared as written, behind its `#` or quote, and is never read as code. An edit of a `=>` line
that changes more than blanks is therefore never bounded: it selects the enclosing called
method's tests, or falls back outside one.

The class was checked on Ruby 3.3.8 (`r2/probe/`), with the edited line never executed and a
later read of `x`:

| Old line | New line | Dropped by the old signature | Effect on the later `x` |
|---|---|---|---|
| `x = {:a => 1}` | `@x = {:a => 1}`, `$x = {:a => 1}` | the sigil | nil local becomes a call (NameError) |
| `@x = {:a => 1}` | `x = {:a => 1}` | the sigil | a method call becomes a nil local |
| `puts x = {:a => 1}` | `puts.x = {:a => 1}` | the dot | nil local becomes a call |
| `puts flag # x = "=>"` | `puts flag, x = "=>"` | the comment mark | a method call becomes a nil local |
| `puts 1 => x` | `puts => x` | the number | hash argument becomes a pattern that declares `x` |
| `puts [1] => x` | `puts[1] => x` | the blank | hash argument becomes a pattern that declares `x` |
| `puts x = 1` | `puts<NBSP>x = 1` | nothing, but the no-break space was read as a blank | Ruby reads it as part of a name, so `x` is no longer declared |
| `/(?'w'.)/ =~ 'a'` | `/(?'x'.)/ =~ 'a'` | not a signature: only `(?<` was recognised | the quoted spelling of a named capture declares `x` too |

Each row is an edit of `reachEdits`, so `TestLiveLineReach` commits it on a collected Minitest
project, runs the suite at the edited head, sees exactly one test fail and requires the plan to
keep it. Against the reviewed head every one of them was planned `exclude`
(`r2/p1-signature-failing-before-fix.log`, and the final test set against the reviewed
`lines.go` in `r2/p1-signature-failing-before-fix-final-tests.log`). The review's own comment pair
(`foo(a) # x = {b => c}` against `foo a, x = {b => c}`) also changes the line's brackets, so the
structure guard already kept the static plan for it; it is in the list with that expectation,
and the bracket-free row above goes through the signature.

`TestLiveBindingSignatureAgreesWithRuby` checks the rule against Ruby's parser instead of
against examples: over 7,917 combinations of an assignment target, an operator and a value,
6,179 parse, and whenever two of them share a signature the method scope's local table Ruby
reports is the same. The old signature fails it 643 times
(`r2/p1-differential-failing-before-fix.log`).

A second probe (`r2/probe/glue.out`) covered block keywords written without a blank, which Ruby
accepts (`r = 1if v`, `f(v)and` continuing onto the next line). The whole-statement check read
`1if` as one word and looked for ` and` with a blank. No miss follows from it: Ruby counts such a
line when it starts the statement and gives the continuation line no counter. The check now
reads the keyword as Ruby does, so the code matches the rule's text.

The expected signatures of `h = {a => 1}` in `TestLineReachRules` changed with the signature's
definition, from the token list `h = a =>` to the line's text.

## Note acted on: the fold qualified what admission refuses (LPCV-V0-7608)

`foldRubyReport` did not look at the index's unordered files, while `CheckRubyQualification`
refuses a qualifying run with any. When no baseline test belonged to such a file the fold named
the runner and `ruby-qualify -record` wrote a record admission then rejected. A run with no
fault had the same divergence. The fold now restates the first as having no control and fails
on the second. The record-only checks of admission are one method, `Qualified.RubyAdmits`, and
the command calls it for every runner it names before writing. Test:
`TestRubyQualifyRefusesWhatAdmissionRefuses`, failing against the reviewed head
(`r2/item3-fold-failing-before-fix.log`).

## Qualification rerun

The record binds `internal/rubylinecov/lines.go`, `internal/linecov/admission.go` and
`tools/linecov/rubyqualify.go`, so the fault run was repeated with the selector built at
`c48817cf9b78c6e4de2a90ba4432a3c1012d9021`. Nothing else changed: WebMock
`f90fd503000279076c02269299067e08bf1ca1ee`, seed 20261116, the same 20 faults and one control
per runner in the same order, the round-1 indexes, every fault against both complete suites.

| | Minitest | RSpec |
|---|---|---|
| Baseline | 23 tests, 0 failing, 0 skipped | 4,846 examples, 0 failing, 0 pending |
| Faults / killed | 20 / 9 | 20 / 13 |
| Killed on a narrowed set | 2 (1 of 23 tests each) | 3 (4,389, 4,383 and 4,383 of 4,846) |
| Selected-set sizes | 23 ×7, 1, 1, 17, 17, 23, 23, 23, 0, 23, 23, 0, 23, 0 | 4,846 ×7, 4,371, 4,371, 4,846 ×5, 4,389, 4,846, 4,846, 4,378, 4,383, 4,383 |
| Misses | 0 | 0 |
| Control (`query_mapper.rb:107`, 0 hits) | selects 0, kills 0 | kills 0, keeps 4,371 (all kept whatever the change) |
| Verdict | qualifies | no control: not qualified |

Every selected set and every fallback reason equals the round-1 run fault by fault. The failing
sets are equal too, with one exception: at the `lib/webmock/config.rb:9` fault the RSpec suite
failed `spec/acceptance/httpclient/httpclient_spec.rb[1:6:1:7:14:2]` in addition to the example
it failed before. The plan for that fault keeps all 4,846 examples, so it is not a miss.

`linecov ruby-qualify -record` wrote the record (report SHA-256 `0a9ad3ef…c312`) at commit
`4b6cb89c2`, which touches only `.github/`: the report digest and the three rebound blobs. The
selector built there admits Minitest for all 21 fault heads (effective plan equal to the
candidate, no failing test dropped, the control's effective plan empty) and refuses RSpec for
all 21 (`ruby qualification does not qualify runner`, static plan of 4,846,
`coverage-collector-unqualified`).

The Ruby family state is unchanged: **QUALIFIED** for
`ruby 3.3.8 minitest 5.26.2 coverage=lines,methods` at a revision that holds the record and the
38 bound blobs, **UNQUALIFIED** for `ruby 3.3.8 rspec 3.13.7 coverage=lines,methods` and every
other Ruby tuple. V1-4365 criterion 6 is met for Minitest only.

## Retained evidence (outside the repository, `/home/agent/lanes/r1ruby-evidence/`)

| File | SHA-256 |
|---|---|
| `r2/ruby-report.json` (fault-run report) | `0a9ad3efc2f7e5a0af09de3edb5ad1b9be9e92c59139a1ad68b1a2bb006bc312` |
| `r2/ruby-qualify-summary.json` | `ca93c82afdf5135c6129144a999a65139db61e36dfaa1cac9e0ef8b02ec8f2ea` |
| `r2/admitted-plans.json` (selector `4b6cb89c2`) | `af92c2f3510953e965680c069a6270678a92bfc929edb0c608ce130865434c6e` |
| `r2/harness/qualify-ruby.py` (the round-1 harness with round-2 paths) | `f45b188fd8df4ee474a51f70345ba6c05c3d106567325e5d294adb34751f0071` |
| `r2/harness/admitted-plans.py` | `a4d33fe3934d00dfd45c38abac9d81b12146590ef38bd3d9d3d570a5df8b6817` |
| `r2/probe/probe.out` (bindings on an unexecuted line) | `c4597805f20d347c54b5701075d65972537afeebb6275c6db97554f6da760e44` |
| `r2/probe/glue.out` (keywords written without a blank) | `e08a764e53c4df5782812aca266790e84b31ce0c3cb0cfa96c12c0c89c974c7e` |
| `r2/p1-signature-failing-before-fix.log` | `7f410c6f083db6bb9bbf99a69e2fd43d3eb9fa6e69123e205b1f36cb235514ad` |
| `r2/p1-signature-failing-before-fix-final-tests.log` | `e7c8e5ed2688ad4b6065eeb142e66752105e222ea7fd5ff13fadd34b31870258` |
| `r2/p1-differential-failing-before-fix.log` | `e5b5f42b1d1229a0db8a06e01d033dc6212411786a44b4ab92ad398f10d113bd` |
| `r2/item3-fold-failing-before-fix.log` | `0b7de7de3de3fdd3535c262508e3d2958388bbbc59d3a5a2bde9dfc5f88dd3c5` |

The indexes and the per-line dumps that seed the reach faults are the round-1 files recorded in
`2026-11-16-r1ruby-review-repair.md`.

## Not done here

- Corvint Go requalification (V1-4365 criterion 4). The Go record was already stale; this round
  changes one more of its bound files (`internal/linecov/admission.go`).
- RSpec qualification, for the reasons in `2026-11-16-r1ruby-review-repair.md`.
- Review notes left as reported: line-count changes (`__LINE__`, `caller`, backtraces) are not
  modelled as reach; a foreign-runner isolated run that fails is kept as a divergent result
  whose message does not name the cause; Minitest narrowing rests on two kills.
