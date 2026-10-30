# Ruby profile V0 fixture

This Apache-2.0 fixture is the Rails/RSpec profile for GitHub issue #1130
(`RUBY-AFFECTED-V0-014` in `docs/specs/ruby-affected-test-adapter-v0.md`). It
holds data only. The suite that replays it is the Go test package
`internal/rubyprofile`, which the ordinary go-product CI shards run; it needs
no Ruby, Bundler, or RSpec on the host.

- `rails-app/` is a small Rails 7.0 application with the generated
  `config/application.rb` (`Bundler.require(*Rails.groups)`, see
  `RUBY-AFFECTED-V0-013`), autoloaded models, a controller, a service, a
  Sidekiq worker, and RSpec specs: `described_class` model specs, a spec whose
  only body is `it_behaves_like`, request, controller, and worker specs, and
  `spec/support/**` helpers loaded by the `rails_helper.rb` glob.
- `overlays/dynamic-constantize/` and `overlays/dynamic-const-get/` each add
  one `constantize` or `const_get` lookup to the application for the negative
  rows.
- `rspec-report.json` is a recorded RSpec 3.13 JSON report for the
  application's six examples. A stand-in `rspec` copies it to the provider's
  report path; the profile does not claim a real Ruby run.
- `rows.json` (`corvint-ruby-profile-rows/0`) declares each fixture class, the
  path its row changes, and the expected rows:
  - `impact`: every `contextindex` impact result as `kind id` and every
    unknown code, compared exactly.
  - `affected`: the Ruby plugin's scope, each selected spec with its witness
    kind, and every widening as `REASON detail`, compared exactly.
  - `provider`: the `corvint-ruby-test-provider/0` example identity and its
    test-validity projection, plus the document's freshness after the class's
    path is edited.

Negative classes change a constant the overlay's lookup could name. Impact must
then report `ruby-constant-lookup-dynamic` and selection must stay `UNKNOWN`
with `ruby:rails-autoload`; the suite refuses a table that drops either.

Recorded limits:

- Context impact has no route-dispatch rule, so a controller change does not
  reach the request spec in `impact`, while affected selection reaches it as an
  advisory route-dispatched test (`RUBY-AFFECTED-V0-011`).
- Provider freshness binds spec and helper blobs only. Editing application
  source leaves a retained run `CURRENT`; affected selection, not freshness,
  decides which specs rerun.
- Each negative row names its site by path and line. A dynamic constant
  lookup can name any constant, so its site bears on every change
  (`RUBY-AFFECTED-V0-047`). The `const_get` site is also a possible generated
  test, but that per-unit site is named only for a change whose reverse
  closure reaches `handler_registry.rb`, which a worker change does not.
- The base application uses the Rails 7.0 `config/application.rb`. The
  positive `rails-7-1-autoload-lib` class overlays the generated Rails 7.1 form
  with `config.autoload_lib(ignore: %w[assets tasks])` and expects the same
  rows as `autoloaded-constant` (`RUBY-AFFECTED-V0-042`).
