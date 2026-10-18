# 2026-10-18 V1-1830, V1-1799: stock Rails gem-group load and the Ruby profile fixture

Tickets: V1-1830 (the generated `Bundler.require(*Rails.groups)` raised `ruby:dynamic-load`, so a
stock Rails application never stayed `BOUNDED`), V1-1799 (a Rails conformance fixture with expected
impact, selection and provider rows). GitHub #1130.

## Decision

`RUBY-AFFECTED-V0-013` and `RUBY-AFFECTED-V0-014` are added to
`docs/specs/ruby-affected-test-adapter-v0.md` and accepted under the owner's standing auto-accept of
lane requirements (2026-10-11). `RUBY-AFFECTED-V0-005` now points at 013.

- 013 is lexical. The Ruby plugin skips one exact statement: unindented
  `Bundler.require(*Rails.groups)`, optionally followed by a comment, in `config/application.rb` or a
  nested `*/config/application.rb`. That statement loads Gemfile groups, never application code.
  Other arguments, indentation, a trailing statement, another owner file, and every computed
  `require` keep `ruby:dynamic-load`. The test checks all six negatives.
- 014 adds `conformance/ruby-profile-v0/`: a Rails 7.0 app, two negative overlays, a recorded RSpec
  JSON report and `rows.json`. The suite lives in the new AGPL test package `internal/rubyprofile`.
  - `conformance/**` is Apache-2.0 and its programs import no Corvint implementation package, so
    the fixture holds data only.
  - A new package joins the go-product shards' `go list ./...` universe, so CI runs the suite with
    no `.github/` change.
  - The package reads the fixture outside its own directory, so `.corvint/test-read-scopes.json`
    declares it with `conformance/ruby-profile-v0/` (`unbounded-readers-check`). Its Git and
    `contextindex` work runs in temp directories outside the repository root.
- The rows record observed behavior. Seven positive classes:
  - autoloaded constant;
  - `described_class`;
  - shared examples;
  - support helper;
  - request spec;
  - controller spec;
  - worker spec.

  Each positive class has exact impact results and unknowns, a `BOUNDED` selection with witness
  kinds, and a provider projection. The projection is `ASSOCIATED`, `PASSED`, hygiene `UNSUPPORTED`,
  strength `NOT_MEASURED`, `CURRENT`, and `STALE` after a support-helper edit. Two negative
  classes, `constantize` and `const_get`, change a constant the lookup could name. Impact reports
  `ruby-constant-lookup-dynamic`, and selection stays `UNKNOWN` with `ruby:rails-autoload`.
  `TestRubyProfileRowsCoverEveryClass_RUBYAFFECTEDV0014` refuses a table that drops a class or
  weakens a negative.
- Mutation check: with 013 disabled, the profile's positive affected rows fail on
  `ruby:dynamic-load`, and so does the 013 unit test.

Requirement IDs: `RUBY-AFFECTED-V0` on origin/main stopped at 012, and open PRs #1105, #1144, #1153
and #1154 held none, so 013 and 014 were free.

## Recorded limits

- The Rails 7.1 generated `config.autoload_lib(ignore: %w[assets tasks])` line is autoload-root
  configuration that `internal/rubyautoload` counts as unresolved (`RUBY-AFFECTED-V0-010`). A stock
  7.1 application therefore still widens to `ruby:rails-autoload`, so the fixture and the 013 test
  use the 7.0 form. This is a residual.
- Context impact has no route-dispatch rule. A controller change reaches the request spec in
  selection (advisory, `RUBY-AFFECTED-V0-011`) but not in impact.
- The provider rows come from a recorded report and a stand-in `rspec`. A real Ruby/RSpec run is
  `NOT_RUN`.
