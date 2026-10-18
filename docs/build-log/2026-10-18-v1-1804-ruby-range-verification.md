# 2026-10-18 V1-1804, V1-1805: Ruby range verification and Gemfile-gated RSpec detection

Tickets: V1-1804 (range impact `--base` names no Ruby verification command), V1-1805 (RSpec
detected from `spec/` or `.rspec` without reading the Gemfile). GitHub #1128, #1130.

## Decision

`NGI-V0-051`, `NGI-V0-052` and `NGI-V0-053` are added to `docs/specs/non-go-impact-v0.md` and
accepted under the owner's standing auto-accept of lane requirements (2026-10-11). They replace
the `NGI-V0-027` non-goals for committed ranges, for Gemfile content and for Minitest commands,
and close its recorded Minitest failure mode.

- A range with an admitted `.rb` path names the Ruby command once, after the Go package
  commands and before `go mod verify` and the gate. A range with no Ruby path reads no manifest,
  so Go range plans keep their bytes.
- RSpec now also needs `rspec`, `rspec-core` or `rspec-rails` declared in the committed root
  `Gemfile` or listed in `Gemfile.lock`. Manifests are read only when the plan has a Ruby path:
  pinned source bytes when admitted, else one read-only `git cat-file blob <tree>:<name>` bounded
  at 256 KiB (the CODEOWNERS read precedent in `blame.go`). Absent, oversized or unreadable
  manifests declare nothing, which can only withhold a command.
- Minitest gets `bin/rails test` (rails/railties declared, `bin/rails` and `test/` tracked) or
  `bundle exec rake test` (minitest declared, `test/` tracked, Rakefile names
  `Rake::TestTask`/`Minitest::TestTask`); otherwise the unknown-language line stays.
- `analyzerSchemaID` is bumped to `corvint-analyzer/127` because the IDX-SNAP-V0-017 audit
  digests every contextindex source; no extracted fact changed.

`AFP-V0-066` (affected advice) was left unchanged: it is keyed on `ruby:rspec:` units the Ruby
provider selected, not on `spec/` presence, so it does not share this failure mode.

Requirement IDs: `NGI-V0` on origin/main and the open PRs stopped at `NGI-V0-040`. `041..050` were
left free for concurrent Ruby lanes.

## Evidence

Real CLI, built from this branch, against the GitHub #1130 fixture
(`internal/contextindex/testdata/ruby-autoload-1130`) committed with
`app/api_clients/foo_client.rb` changed:

- `corvint impact --base <base>`: verification is `["bundle exec rspec", "git diff --check"]`.
  The installed `1.0.0-rc.3 (build 634)` gives `["git diff --check"]`.
- A Minitest repository (`gem "minitest"`, tracked `spec/` and `test/`), `corvint impact
  app/models/widget.rb`: verification is `["git diff --check"]` and the unknown-language line
  is kept.

Tests: `TestNGIV0051RangeImpactNamesRubyCommand`, `TestNGIV0051GoRangePlanUnchangedByRubySignals`,
`TestNGIV0052RSpecNeedsDeclaredGem`, `TestNGIV0053MinitestCommands`, and the `NGI-V0-027` tests,
updated for the gem gate. The full `./internal/contextindex` package passes.

## Rollback

Revert the change. `ruby_verification.go`, the `rangeVerification` parameter and the tests go,
and `toolchainCommand` gets its boolean back. No stored bytes depend on the change; snapshots
rebuild under the previous analyzer schema.
