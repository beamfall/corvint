# 2026-10-18 V1-1796: a known Ruby verification command

GitHub #1130 item 4. On a Rails repository with a `Gemfile`, `.rspec` and `spec/`, `corvint
impact` on a Ruby path recommended only `git diff --check` and said
`no language-specific verification command is known for this repository`, and `corvint affected`
advice listed no check although the plan selected the spec file.

The new requirements `NGI-V0-027` (`docs/specs/non-go-impact-v0.md`) and `AFP-V0-066`
(`docs/specs/affected-plan-v0.md`) are accepted under the owner's standing auto-accept of lane
requirements (2026-10-11); V1-1796. No CAL-V0 change. IDs were checked against origin/main
`70a21da5d` (highest `NGI-V0-021`, `AFP-V0-054`) and the open PRs (none carries an NGI ID; the
highest AFP ID seen is `AFP-V0-060`, #1105). Each takes a gap: peer lane V1-1794 extends the NGI
spec at the same time.

## Design

- Impact: `toolchainCommand` names `bundle exec rspec` for a `.rb` evidence path when
  `rspecProject` holds: the index tracks a root `Gemfile`, and a root `.rspec` or a path under
  `spec/`. Detection reads `Index.Tracked` (persisted in packs and snapshots) because `Gemfile`
  and `.rspec` are not admitted text sources. It is computed once per plan. The existing
  unknown-language rule then drops its line because a language command exists. Gemfile content
  is never read, and nothing runs.
- Committed-range receipts are left alone: they build verification in `rangeVerification`
  (Go-only), a file owned by the concurrent V1-1794 lane. See the residual below.
- Advice: `advisoryRSpecCheck` appends one advisory `bundle exec rspec <files>` check after the
  Go advisory entry. The files are the `tests` of the selected `ruby:rspec:` units, sorted,
  deduplicated and POSIX single-quoted. It requires a regular root `Gemfile`, observed with
  `lstat` in the working tree like the Makefile and AGENTS.md reads, and is bounded at 1024
  files. Otherwise a `NO_ADVISORY_RSPEC_COMMAND` unknown line names the reason. Plans with no
  RSpec unit keep their exact bytes.
- `internal/contextindex` non-test files are analyzer inputs, so `analyzerSchemaID` moves to
  `corvint-analyzer/124` with the `IDX-SNAP-V0-017` audit digest repinned. Pack facts and
  encoding are unchanged.

## Evidence

The GitHub #1130 reproduction fixture (commit `3306177`, then an edit to
`app/api_clients/foo_client.rb`) was checked with the installed 1.0.0-rc.3 build and with this
branch's binary:

- `corvint impact app/api_clients/foo_client.rb`:
  - Before, verification was `["git diff --check"]` and the unknown-language line was present.
  - After, verification is `["bundle exec rspec","git diff --check"]` and the line is gone.
- `corvint affected --base 3306177…`:
  - Before, `advice.checks` was `[]`.
  - After, it holds the advisory `bundle exec rspec 'spec/api_clients/foo_client_spec.rb'`.
  - `NO_ADVISORY_GO_COMMAND` stays.

Focused tests:
- `./internal/contextindex`: 104 top-level tests and 217 PASS lines, including
  `TestNGIV0027*` and `TestAnalyzerSchemaInputs`.
- `./cmd/corvint` (`TestAFPV00*`, `TestAffected*`, `TestAFUV10*`, `TestImpact*`): 100
  top-level tests and 152 PASS lines.

Neither `make gate` nor `go test ./...` was run: the owner's standing preference is focused tests.

## Residual

Committed-range impact (`corvint impact --base`) still plans `git diff --check` alone for a Ruby
range in an RSpec repository. Making `rangeVerification` share the path plan needs its own
byte-identity review of range receipts.
