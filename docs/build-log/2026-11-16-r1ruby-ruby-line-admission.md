# 2026-11-16: Ruby line selection, admission and WebMock qualification (V1-4365)

Requirements: LPCV-V0-7605 (amended), LPCV-V0-7608, LPCV-V0-7609, LPCV-V0-963, GLS-V0-032.
Decision: 0601 (owner, 2026-11-16: "Build Ruby admission"). Lane base:
`c7dbf717e2b2d25dd83b3f26fb7ac7def95587b6`. This entry updates the Ruby row of the family ledger in
`2026-11-16-r1docs-line-qualification.md`; that entry's Go, Beamfall and earlier Ruby evidence is
unchanged.

## What was built

- **Line-level selection** (`rubylinecov.AnalyzeLines`). An edit of executable lines selects the
  tests whose measured coverage hit them, so an edit of zero-hit lines selects none. A line Ruby
  does not count (`when`, `else`, continuation lines) or an insertion selects every test of the
  enclosing called method. A Ruby 3.3.8 probe confirmed which constructs have no line counter
  before this rule was written. These cases keep the static plan, each with its reason reported:
  load-time reach; a block-structure change (the keyword, bracket and continuation sequence of
  the edited lines, or the lexed code outside them, differs); a new side that fails `ruby -c` under
  the index's Ruby version, or has no check; missing blobs; and every earlier `Analyze` fallback.
  `ruby-select` reports this as its candidate. The method-level `Analyze` stays the unqualified
  rule (`linecov watch`), so `TestUncalledMethodFallback` and `checkCalc` are unchanged.
- **Admission** (`linecov.CheckRubyQualification`). The effective plan follows the candidate only
  when a passing `.github/linecov/ruby-qualification.json` exists at the selector executable's
  clean VCS revision in `-selector-root`, binds all 38 `RubyQualifiedSources` blobs at that
  revision, and names the index runner with zero misses and passing controls. Every refusal keeps
  the static plan with `coverage-collector-unqualified` and reports why in `Admission`.
  `CheckQualification` (Go) now shares the record and binding checks; its behaviour is unchanged.
  `linecov ruby-qualify` folds a fault-run report into the record.
- **Collector.** `ruby-collect -state DIR -parallel N -budget D` is resumable and parallel; the
  merge reads results back in outcome order. Three defects stopped WebMock's RSpec collection;
  each is fixed with a test:
  1. Coverage cost: 54 ms per test over 1,113 loaded files. Files outside the root now compile
     without coverage through a `load_iseq` hook, giving 7 ms over 120 files. A C-level maximum
     also skips files with no hits. On the 482-example unit slice the run fell from 33 s to 16 s,
     and the dumps were identical apart from method-list order.
  2. Unobserved fork: the suite forks its WEBrick server in `before(:suite)`, and every fork was
     unobserved, so the load bucket refused the whole run. A fork now carries a launch token that
     the child's completed dump joins. An abrupt child still leaves its record incomplete.
  3. Merge memory: `mergeShapes` multiplied file shapes 4,846-fold, and the memory cgroup killed
     the merge at 10 GB. Shapes are now kept once each. Re-merging the Minitest state gave
     identical bytes.

## Collection (host: `nproc` 12, load average 16–24 from other lanes)

| Runner | Command (NO_CONNECTION=1, BUNDLE_GEMFILE pinned) | Outcomes | Index SHA-256 |
|---|---|---|---|
| Minitest 5.26.2 | `bundle exec ruby -Ilib -Iminitest -e 'Dir["minitest/**/*.rb"].sort.each { … }' -- --seed 20261116`, `-parallel 6`, 30 s test / 2 min suite | 23/23 isolated, 0 unordered | `d1e56378…6a96eb1` |
| RSpec 3.13.7 | `bundle exec rspec --pattern 'spec/**/*_spec.rb' --require ./spec/spec_helper.rb`, `-parallel 6 -budget 7m20s`, 2 min test / 8 min suite | 4,846 isolated runs; 2,930 unrecorded; 11 unordered files (4,371 tests) | `43827403…1277f782` |

The RSpec collection ran in 14 foreground chunks, each under 9 minutes. The first two attempts are
retained in the collection log: one where the ordinary run took longer than 535 s, and one where
the unobserved fork refused the run. The final merge took 98 s. Unrecorded RSpec outcomes, by
cause:

- **2,927 order-dependent.** Their ordinary-order reach exceeds their isolated reach. For example,
  `WebMock.after_request` callbacks registered by earlier examples run inside later ones
  (`async_http_client_spec.rb:142`).
- **2 with unobserved children.** These are in `quality_spec.rb`: `git ls-files`, `gem build` and
  `rm` run through backtick and `system`. "has no malformed whitespace" reads the text of every
  tracked non-fixture file.
- **1 that fails alone.** `errors_spec.rb[1:1:1:6:2]` passes in order but fails in isolation.

The 11 files holding these outcomes run whole whatever the change (`LPCV-V0-962`).

## Qualification run

The pin is WebMock `f90fd503000279076c02269299067e08bf1ca1ee` (MIT, `LICENSE` SHA-256
`b3c3b74c…37e837`). Seed 20261116 orders the same seven faults as the r1docs run, with the same
zero-hit control (`lib/webmock/util/query_mapper.rb:107`, `false` changed to `true`). Each fault
is a separate commit. Both complete suites ran at each fault head as the oracle; `ruby-select`
planned each head at selector `0cf0a4d95`.

| Fault | Minitest killed / selected / missed | RSpec killed / selected / missed | Candidate fallback |
|---|---|---|---|
| statement-delete, verifier:10 | yes (1) / 23 / 0 | no / 4,846 / 0 | load-time code |
| loop-header, query_mapper:212 | yes (2) / 23 / 0 | yes (142) / 4,846 / 0 | block structure (`[]` added) |
| case-expression, query_mapper:249 | yes (2) / 23 / 0 | yes (116) / 4,846 / 0 | load-time code |
| constant-change, verifier:70 | no / 23 / 0 | yes (224) / 4,846 / 0 | load-time code |
| else-if, verifier:21 | yes (14) / 23 / 0 | yes (1,175) / 4,846 / 0 | load-time code |
| closure, query_mapper:254 | no / 23 / 0 | yes (7) / 4,846 / 0 | load-time code |
| condition-negate, verifier:24 | yes (14) / 23 / 0 | yes (1,105) / 4,846 / 0 | load-time code |
| **control**, query_mapper:107 | **no / 0 / 0** | **no / 4,371 / 0**, 0 by coverage | none |

Totals: 14 faults, 11 killed (Minitest 5/7, RSpec 6/7), 0 misses, 2 controls passed. Both
baselines passed: Minitest 23 tests and RSpec 4,846 examples, each with 0 failures, 0 skipped and
0 errors outside examples.

The control line has 0 measured hits across all 9,696 RSpec dumps and all 24 Minitest dumps:
ordinary test windows of every status, load buckets, children, and isolated runs
(`harness/line-hits.py`). The Minitest control selects and kills nothing. The RSpec control kills
nothing, causes no fallback and keeps no test by coverage. It keeps exactly the 4,371 tests of the
11 files that run whole whatever the change. Under `GLS-V0-032`, the accepted control rule for
this ticket, those tests "are kept whatever the change and do not fail a control". The record
restates them (`keptRegardless`, `controlsKept`).

A zero-kept RSpec control is not reachable soundly on this corpus. `quality_spec.rb` fails on a tab
or trailing space anywhere in a tracked file, so any edit of any line may fail it.

Selection narrows none of the seven faults. Six reach Ruby's load bucket, which includes the
windows of tests that loaded a repository file or called `eval`, and one changes block structure.
The only narrowing measured is the control.

`linecov ruby-qualify -record` wrote the record (report SHA-256 `baa55c50…a5`) at commit
`2a242fcf6`. The selector built there admits both runners: all 16 plans are admitted and
gate-eligible, each effective plan equals its candidate, and none misses a test. The selector at
`0cf0a4d95`, which has no record at its revision, refuses all 16 plans and keeps them static with
`coverage-collector-unqualified`.

## Ruby family state

**QUALIFIED** for `ruby 3.3.8 minitest 5.26.2 coverage=lines,methods` and
`ruby 3.3.8 rspec 3.13.7 coverage=lines,methods`. The basis is the record above and the
`GLS-V0-032` control rule for RSpec. Admission holds only at a revision that contains the record
and the 38 bound blobs; any later change to a bound source refuses it until the family is
requalified. Other Ruby versions and runner versions remain unqualified and static.

Not qualified here: Corvint Go (V1-4365 criterion 4) and every Beamfall family; the earlier
ledger stands for them.

## Retained evidence (outside the repository, `/home/agent/lanes/r1ruby-evidence/`)

| File | SHA-256 |
|---|---|
| `ruby-report.json` (fault-run report) | `baa55c50f401d4625249075350fcb478fbb8b9d4562c3874d33a17d29c6dda5a` |
| `ruby-qualify-summary.json` | `2d61ac6aa8559bb0351edd642a895cfa4eb075e97ef77bf281ac981b8807e03b` |
| `admitted-plans.json` (selector `2a242fcf6`) | `41aeacda7f959ec592204345b53f8aa5c3d87f92e6bfab999711e18a6a811cf6` |
| `refused-plans.json` (selector `0cf0a4d95`) | `ab6b725abef60f160dabf3c944c5c8322493f48e0919714b4e4b2d1fdc1d6345` |
| `minitest.idx` | `d1e56378f5ab773bd0d5835721d204f13f74752e8821460eaf9d7f6fd6a96eb1` |
| `rspec.idx` | `438274033870c9a0408f668fe7b6b8e3168ef4ee980d82675f706b8a1277f782` |
| `line-hits-rspec-state.json` | `ed8559f7de510d4c575bd5ff9c233701a0d87c89ff3bd3a0f3f6c197aee2d222` |
| `line-hits-minitest-state.json` | `f88f7f1e9bc6bbd43e5bcf25fad5d4182939d8c0a4d26acdd6a3b9382cc1035a` |
| `rspec-unrecorded.txt` | `be1df5636095406e0e104c90b342ce73008e4fdd60bc6e8e874dbf4b5b4cd742` |
| `harness/qualify-ruby.py` | `826df73561626a83f7dbbdb5a23fccf13f79e9f2f52105495b51b2a0846089ab` |

The first qualification run, with the same numbers at selector `dc402c515` before the control
rule, is kept under `qual-run1/`.
