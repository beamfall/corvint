# Ruby Affected-Test Adapter V0

Owner: Russell Lewis
Date: 2026-08-29
Intent status: proposed
Delivery status: experimental
Authoritative inputs: operator request dated 2026-08-29,
`live-proof-carrying-verification-v0.md`, `../SPEC-DRIVEN-DEVELOPMENT.md`, `../DOGFOOD.md`

## Agent digest
- Claim: Static Ruby analysis conservatively maps changes to RSpec, Minitest, Test::Unit, and Rails test files without executing code.
- Status: proposed/experimental
- Exists: `internal/liveverify/affected/ruby` static planning adapter, advisory autoload constant edges, and focused seam tests.
- Blocked on: qualified runtime tuple and full repository/CLI-parity evidence remain `NOT_RUN`.
- Read next: User and measurable job; Acceptance evidence and rollback; Traceability.

## User and measurable job

Given a dirty Ruby path, Corvint should discover every statically addressable Ruby test file that can
depend on it, distinguish RSpec, standalone Minitest/Test::Unit, and Rails test runners, and return a
deterministic witnessed selection. Any unsupported discovery or dependency surface must widen scope
instead of appearing to be a safe exclusion.

This is an experimental graph adapter. It does not qualify any Ruby/framework/runtime/OS tuple,
execute tests, or amend the first validation tranche in `live-proof-carrying-verification-v0.md`.

## Verified current state

At the implementation base, `internal/liveverify/affected/` has Go package and Python module
adapters. `Language` exposes only a namespace, path ownership, and units containing source paths,
test paths, and imports. It has no runner-command, framework, source-anchor, or shared-file ownership
field. `Select` supplies deterministic dependency witnesses, bounded exclusions, and global language
frontiers under LPCV-V0-013, LPCV-V0-014, LPCV-V0-016, and LPCV-V0-019.

The archived Python Ruby/Rails analyzer on `codex/corvint-integration` is reference evidence only. It
uses file-level RSpec/Minitest units, literal `require` relationships, and conservative conventional
source/test counterparts; it does not discover RSpec examples, Minitest methods, Capybara semantics,
or Cucumber scenarios.

## Requirements

- `RUBY-AFFECTED-V0-001`: The plugin namespace MUST be `ruby` and `Owns` MUST claim exactly regular
  Ruby source names ending in `.rb`. It MUST NOT claim `.feature`, `.yaml`, or `.yml` paths.
- `RUBY-AFFECTED-V0-002`: One unit MUST be one repository-relative `.rb` file, the smallest common
  unit directly addressable by the supported runners through this seam. Unit IDs MUST be stable,
  unique, framework-qualified for test files, and derived only from repository text and paths.
- `RUBY-AFFECTED-V0-003`: Discovery MUST admit all concurrently present conventional RSpec
  `*_spec.rb` files and RSpec DSL files; Minitest/Test::Unit `*_test.rb`, `test_*.rb`, class, method,
  and spec-DSL files; Rails unit, integration, and system test files; and RSpec feature/system files
  driven by Capybara or Selenium. Helper and support files without runnable tests MUST remain source
  units; a helper whose only test signal is a test base-class declaration (`< Minitest::Test`), with
  no `test_` method or `test` block, has no runnable tests. The adapter MUST detect what is present rather than select one repository-wide framework.
- `RUBY-AFFECTED-V0-004`: Imports MUST include uniquely resolved literal `require` and
  `require_relative` edges. A conventional RSpec/Minitest/Rails test-to-source counterpart edge MAY
  be added only when exactly one repository source matches. External loads are omitted because they
  cannot be dirty repository paths. Load-path aliases (`lib/`, `app/`, `spec/`, `test/` stripped)
  MUST apply to every observed Ruby file, including runnable tests and helpers, so a literal
  `require "test_helper"` is never mistaken for an external load. In a `Rakefile` that declares a
  `Rake::TestTask`, each repository-relative literal `IDENTIFIER.libs << "ROOT"` entry without a
  backslash escape MUST add `ROOT` as an alias root for every observed Ruby file; Corvint reads this
  bounded form without executing Rake or claiming the `Rakefile` as a Ruby unit.
- `RUBY-AFFECTED-V0-005`: Unreadable or undecodable Ruby, dynamic/interpolated loading, ambiguous
  repository loads, conditional loading, reflection/metaprogramming that can define tests, Rails
  autoloading that `RUBY-AFFECTED-V0-010` cannot bound, Cucumber feature presence, and a nonliteral, absolute, or repository-escaping
  escaped, nonliteral, absolute, or repository-escaping `Rake::TestTask` `.libs` declaration MUST
  produce stable frontier reasons (the stock Rails gem-group load is not dynamic loading, see
  `RUBY-AFFECTED-V0-013`). A frontier makes
  selection `UNKNOWN_SCOPE`; it MUST NOT silently omit a potentially runnable test.
- `RUBY-AFFECTED-V0-006`: Units, paths, imports, and frontier reasons MUST be sorted and reproducible.
  Clean rebuilds over fixed admissible inputs MUST produce byte-identical graph and plan identities.
- `RUBY-AFFECTED-V0-007`: Framework-qualified unit IDs MUST preserve the runner class and runnable
  path. Canonical Bundler commands are `bundle exec rspec PATH`, `bundle exec rails test PATH`, and
  `bundle exec ruby -Itest PATH`; unbundled Minitest uses `ruby -Itest PATH`, and a Rails application
  may use `bin/rails test PATH`. Because the frozen seam has no structured-argument or repository
  runner-policy field, the adapter MUST NOT claim that it emitted or validated one exact command.
  Exact example/method filters remain outside the current seam.
- `RUBY-AFFECTED-V0-008`: Focused fixtures MUST cover mixed frameworks, dependency closure,
  exclusions, deterministic rebuilding, Rails/Capybara/Selenium conventions, dynamic frontiers, and
  Cucumber non-ownership. At least one real Ruby corpus MUST be compared with an independent count,
  and discovered-versus-actual counts MUST be reported without converting the observation into a
  qualification claim.

- `RUBY-AFFECTED-V0-009`: In a Rails application whose autoload mapping is resolved (see
  `RUBY-AFFECTED-V0-010`), each observed Ruby file MUST carry an advisory edge to every other observed
  file whose autoloaded constant it references, resolved by the `internal/rubyautoload` mapper against
  the lexical namespaces enclosing the reference (`RUBY-AFFECTED-V0-043`), and to the target of each `has_many`, `has_one`,
  `belongs_to`, or `has_and_belongs_to_many` association named by a literal `class_name:` or by the
  conventional camelized (singular) association name. The seam carries these as
  `Unit.AdvisoryImports`, a sorted, deduplicated list that is part of the graph digest only when
  non-empty, so graphs without advisory edges keep their existing bytes. `Select` MUST traverse an
  advisory edge like an import, and every test reached through one, directly or transitively, MUST
  carry the `ADVISORY_CONSTANT_REFERENCE` witness; an explicit import edge at the same depth wins.
  An edge to the file itself is omitted, and an explicit import is not repeated as advisory.
- `RUBY-AFFECTED-V0-010`: `ruby:rails-autoload` MUST be raised only when constant edges cannot bound
  a Rails application with tests: a `constantize`, `safe_constantize`, or `const_get` call; a
  polymorphic association, a non-literal `class_name:`, or an association whose target resolves to no
  observed file; no root `config/application.rb` or a nested one; a present but unreadable or
  invalid `.corvint/ruby-autoload.json` declaration (including a duplicate key); or an inflection or
  autoload configuration the mapper reports as unresolved. When the mapping is bounded, a Rails test
  without a literal load or counterpart but with a constant edge MUST NOT raise
  `ruby:implicit-test-dependency`.
- `RUBY-AFFECTED-V0-011`: In a bounded Rails application, every test MUST carry advisory edges to each
  observed Ruby file under `config/`, and a request, system, feature, integration, routing, or
  controller test (by directory) MUST also carry advisory edges to each observed file under
  `app/controllers/` and `app/helpers/`, because routing dispatches to them without a constant
  reference in the test.
- `RUBY-AFFECTED-V0-012`: `spec_helper.rb`, `rails_helper.rb`, and `test_helper.rb` MUST remain
  source units even when a runner hint names them. Every test in a `spec/` or `test/` suite MUST
  import that suite's helpers and each observed file below its `support/` directory, so a change to
  any of them selects every test of the suite. A literal support-loading glob in such a helper
  (`Dir[Rails.root.join("spec/support/**/*.rb")]` or `Dir[File.join(__dir__, "support", "**",
  "*.rb")]`, iterated with `require`) is covered by these edges and MUST NOT raise
  `ruby:dynamic-load`; any other glob still does. `shared_examples` and `shared_context` definitions
  in support source files do not define runnable tests and MUST NOT raise the dynamic-test frontier.
- `RUBY-AFFECTED-V0-013`: The unindented statement `Bundler.require(*Rails.groups)`, alone on its line
  except for a trailing comment, in a Rails application's `config/application.rb` (the root file or a
  nested `*/config/application.rb`) loads only the Gemfile's gem groups, never application code, and
  MUST NOT raise `ruby:dynamic-load`, so a generated Rails application whose other loads are literal
  stays `BOUNDED` for an `app/models` change. Every other spelling MUST keep `ruby:dynamic-load`: other
  arguments (`*Rails.groups(:assets)`, a computed group list), indentation (a conditional or nested
  call), a further statement on the line, the same call in any other file, and any other computed
  `require`, `load`, or `.require(` call. The recognition is lexical; it reads no Gemfile and does not
  evaluate Bundler. The Rails 7.1 `config.autoload_lib` line is read under `RUBY-AFFECTED-V0-042`.
- `RUBY-AFFECTED-V0-014`: `conformance/ruby-profile-v0/` MUST hold a Rails/RSpec fixture application
  and an expected-row table that cover autoloaded constants, `described_class` specs, shared
  examples, `spec/support/**` helpers, and request, controller, and worker specs. Every positive
  fixture class MUST have an impact row (`contextindex.Impact` results and kinds for its changed
  path), an affected-selection row (`BOUNDED` scope and the exact selected spec set), and a
  provider-projection row (the `corvint-ruby-test-provider/0` example identity and its
  `testvalidity` outcome projection from a recorded RSpec JSON report). Negative rows MUST add a
  `constantize` or `const_get` lookup, change a constant that lookup could name, and expect impact's
  `ruby-constant-lookup-dynamic` unknown and affected's `ruby:rails-autoload` frontier (`UNKNOWN`
  scope). The suite MUST run as an ordinary Go test of `internal/rubyprofile`, so the existing
  go-product CI shards execute it; it MUST NOT need Ruby, Bundler, or RSpec on the host.
- `RUBY-AFFECTED-V0-041`: In a repository whose root declares a Go module (`go.mod`), a Ruby or
  `.feature` file with a `testdata` directory component is fixture data by the go tool's convention
  and MUST NOT become a Ruby unit, mark the host as a Rails layout, or raise a frontier; the Go
  adapter owns any reader of that tree (V1-1816). Without a root `go.mod` the same file stays a unit,
  because RSpec's default `spec/**/*_spec.rb` pattern can run a spec under a `testdata` directory.
- `RUBY-AFFECTED-V0-042`: Without a committed autoload declaration, the generated Rails 7.1+ line
  `config.autoload_lib(ignore: %w[assets tasks])` (any literal `%w[...]` or `%w(...)` word list,
  alone on its line but for a trailing comment) MUST be read as the `lib` autoload root minus exactly
  the listed `lib/<word>` subdirectories, so a stock 7.1 application stays `BOUNDED` for an
  `app/models` change. A non-literal ignore list, a further statement on the line, a second such
  line, and `autoload_lib_once` MUST still raise `ruby:rails-autoload` (V1-1850). The
  `rails-7-1-autoload-lib` class of `conformance/ruby-profile-v0/` pins the stock line.
- `RUBY-AFFECTED-V0-043`: The constant scan of `RUBY-AFFECTED-V0-009` MUST mask each heredoc body
  and terminator line before masking string literals, so a constant-looking word, or a quote, inside
  heredoc text neither adds an edge nor hides the code after it; a body whose opener line passes it
  to an evaluating call (`eval`, `class_eval`, `module_eval`, `instance_eval` or an `_exec` form),
  or whose terminator never appears, MUST stay unmasked. A heredoc start MUST open a body only
  with a quoted or upper-case identifier and not directly after an identifier or closing bracket, so
  a shift such as `items<<Widget` masks nothing. Code inside `#{...}` in a body that is not
  single-quoted MUST stay; an interpolation that does not close on its line keeps the rest of the
  body. A reference MUST resolve against the
  `class` and `module` bodies that lexically enclose it, matched to their `end` through `def`
  (not endless), `begin`, `case`, `do`, `for`, and statement-position `if`, `unless`, `while` and
  `until`; when that scan cannot balance the file, or the file holds `=begin`, `__END__` or a
  slash-delimited span holding a block keyword (a possible regex literal such as `/end/`), it MUST
  fall back to every namespace the file opens (V1-1832).

### Autoload edge failure modes

- A dynamic constant lookup or unresolvable association hides its target: the slice keeps
  `ruby:rails-autoload` (`UNKNOWN_SCOPE`) rather than guessing an edge.
- Lexical nesting falls back to the namespaces a file opens, and an ambiguous, evaluated or
  unterminated heredoc stays unmasked: both can add false advisory edges, which only widen
  selection (RUBY-AFFECTED-V0-043). A quote inside a regex literal still opens a string in the
  literal mask, as before this requirement.
- A constant defined outside an observed `.rb` file (gem, engine, generated code) has no edge; the
  referencing test still selects on its own file and on boot/support edges, and an unmapped
  association raises the frontier.
- More than `affected.MaxPathsPerUnit` imports or advisory edges on one unit drop both lists and raise
  `ruby:implicit-test-dependency` instead of truncating.
- Non-Ruby views, templates, and assets stay unowned and produce `UNOWNED_DIRTY_PATH` when dirty.

## Cross-language ownership decision required

Cucumber `.feature` files are language-neutral Gherkin documents whose scenarios bind to Ruby,
JavaScript, Java, or another host. Maestro `.yaml`/`.yml` flows and Appium configurations have the
same host-neutral problem. `Language.Owns(path) bool` has no provider binding, shared ownership,
priority, or arbitration result.

If two plugins claim one path through their returned `Sources` or `Tests`, `Build` returns
`ErrDuplicateOwner`; overlapping `Owns` booleans alone remain ambiguous until a dirty path is mapped.
Changing registration order cannot make shared evidence composable. If no plugin claims it, a dirty
path becomes `UNOWNED_DIRTY_PATH`, and scenario-level discovery is absent. Arbitrarily assigning it
to Ruby would misattribute a feature bound to another host and make cross-provider exclusion
universes unstable. A Cucumber scenario is otherwise directly addressable as
`bundle exec cucumber FEATURE:LINE`, but neither the `.feature` ownership nor the line anchor fits
the current unit shape.

A resolution must define whether neutral assets have exclusive or shared ownership; how repository
configuration binds each asset or scenario to one or more providers; stable provider-qualified unit
and source-anchor identities; deterministic arbitration and composition; inclusion/exclusion
witnesses for shared inputs; and invalidation when bindings, step definitions, drivers, or provider
configuration change. Until that contract is owner-accepted, the Ruby plugin only detects `.feature`
presence and raises `ruby:cucumber-feature-ownership`; it does not claim or enumerate scenarios.

## Boundaries, failures, and rollback

- The plugin reads bounded repository source only. It does not invoke Ruby, Bundler, Rails, RSpec,
  Minitest, Cucumber, Selenium, a package manager, or repository code.
- File-level units deliberately trade example/method granularity for a selector every covered
  runner can execute. The current `Unit` seam cannot carry exact example names, source lines,
  structured commands, repository runner policy, or a versioned runtime provider on an exclusion;
  framework-qualified IDs preserve deterministic runner class choice without widening it. This is
  partial LPCV-V0-014 structure, not complete provider evidence.
- Ruby constant lookup, Rails Zeitwerk autoloading, framework configuration globs, generated tests,
  fixtures outside `.rb`, native extensions, and runtime-conditioned loads are not proved by a
  static edge. Known occurrences widen scope.
- False-positive static edges are acceptable because they widen selection. Missing or ambiguous
  edges are not; they raise a frontier.
- Rollback is removal of the `ruby` plugin from callers and fixtures. Existing Go/Python graph bytes
  must remain unchanged when the plugin is not supplied.
- Rollback of `RUBY-AFFECTED-V0-013` restores `ruby:dynamic-load` for the stock gem-group load;
  rollback of `RUBY-AFFECTED-V0-014` removes the fixture and its suite. Neither changes Go or Python
  graph bytes.
- Rollback of `RUBY-AFFECTED-V0-009..012` is reverting their change: the Ruby plugin returns to the
  coarse `ruby:rails-autoload` frontier, and because `advisoryImports` is omitted when empty, no Go or
  Python graph digest or plan changes in either direction.

## Deterministic acceptance matrix

| Surface | Required evidence | Qualification state |
|---|---|---|
| RSpec, including feature/system specs | fixture discovery, literal edge, runner-class ID and canonical command recommendation | `FALLBACK` |
| Minitest class and spec DSL | fixture discovery, literal edge, runner-class ID and canonical command recommendation | `FALLBACK` |
| Test::Unit and Rails unit/integration/system | fixture discovery, conventional/literal edge, runner-class ID and canonical command recommendation | `FALLBACK` |
| Capybara and Selenium | RSpec/Rails test unit retained; driver is not misidentified as runner | `FALLBACK` |
| Cucumber | `.feature` unclaimed; explicit ownership frontier and written decision | `UNKNOWN_SCOPE` |
| Dynamic/reflection-driven tests | explicit frontier; no safe-exclusion claim | `UNKNOWN_SCOPE` |
| Determinism and seam composition | shared conformance suite, repeated canonical bytes | experimental |
| Rails autoload constant edges (#1130) | fixture change to an autoloaded client selects its spec and the referencing service spec with `ADVISORY_CONSTANT_REFERENCE`; frontier only for unbounded lookup | experimental |
| Suite helpers and support | helpers stay source; a helper or `support/**` change selects every suite test | experimental |
| Stock Rails gem-group load | `Bundler.require(*Rails.groups)` in `config/application.rb` keeps an `app/models` change `BOUNDED`; other dynamic requires widen | experimental |
| Ruby profile conformance fixture (#1130 S6) | `conformance/ruby-profile-v0` impact, affected and provider rows per fixture class, negative dynamic-lookup rows, run by the go-product CI shards | experimental; Ruby runtime `NOT_RUN` |
| Real Ruby corpus | independent discovered/actual file count | observed: 24/24, 40/40, and conservative 34/31; not qualification |
| Full repository and CLI parity | unchanged existing receipts with explicit candidate proof | `NOT_RUN` until recorded |

## Non-goals and promotion gate

This slice does not execute Ruby tests, inspect runtime coverage, parse the full Ruby grammar,
discover individual examples/scenarios/methods, claim Cucumber assets, change `Language`, qualify
safe omission, or advertise Ruby support. The only seam change is the additive, digest-neutral-when-
empty `Unit.AdvisoryImports` edge and its `Select` witness (`RUBY-AFFECTED-V0-009`); it does not
evaluate Ruby, boot Rails, or read Zeitwerk at runtime. The simpler recovery remains each
repository's declared full test command.

Promotion requires an accepted provider/runner contract, owner resolution of neutral-file
ownership, the full LPCV-V0-045 matrix for every claimed tuple, and the preregistered LPCV shadow
corpus. One confirmed missed release-blocking Ruby test disables autonomous narrowing for the
affected tuple.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `RUBY-AFFECTED-V0-001..003`, `RUBY-AFFECTED-V0-006..007` | `internal/liveverify/affected/ruby/` | focused package and seam conformance tests |
| `RUBY-AFFECTED-V0-004..005` | literal-load and frontier logic in `internal/liveverify/affected/ruby/` | `TestRakeTestTaskLiteralLoadPathResolvesRequire`, `TestRakeTestTaskDynamicLoadPathRaisesFrontier`, `TestRakeTestTaskEscapedLoadPathRaisesFrontier`, and existing load-resolution/frontier tests |
| `RUBY-AFFECTED-V0-008` | Ruby fixtures and real-corpus observation | build-log evidence |
| `RUBY-AFFECTED-V0-009` | `Unit.AdvisoryImports` and `Select` in `internal/liveverify/affected/`; `internal/liveverify/affected/ruby/autoload.go` | `TestAdvisoryEdgeSelectsWithAnAdvisoryWitness_RUBYAFFECTEDV0009`, `TestConstantEdgeSelectsAutoloadedDependentSpec_RUBYAFFECTEDV0009` |
| `RUBY-AFFECTED-V0-010` | `newRailsAutoload` and `edges` in `internal/liveverify/affected/ruby/autoload.go` | `TestUnboundedConstantLookupKeepsRailsAutoloadFrontier_RUBYAFFECTEDV0010`, `TestUnitsDetectsConcurrentFrameworksAndRunnerIdentities` |
| `RUBY-AFFECTED-V0-011` | `bootEdges` in `internal/liveverify/affected/ruby/autoload.go` | `TestRailsBootAndRouteEdgesSelectTheirSpecs_RUBYAFFECTEDV0011` |
| `RUBY-AFFECTED-V0-012` | helper classification and `supportEdges` in `internal/liveverify/affected/ruby/` | `TestSuiteHelpersAndSupportSelectEverySpec_RUBYAFFECTEDV0012` |
| `RUBY-AFFECTED-V0-013` | `stockGemGroupLoad` in `internal/liveverify/affected/ruby/ruby.go` | `TestStockBundlerRequireKeepsRailsAppBounded_RUBYAFFECTEDV0013` |
| `RUBY-AFFECTED-V0-014` | `conformance/ruby-profile-v0/` fixture and rows; `internal/rubyprofile` suite | `TestRubyProfileImpactRows_RUBYAFFECTEDV0014`, `TestRubyProfileAffectedRows_RUBYAFFECTEDV0014`, `TestRubyProfileProviderRows_RUBYAFFECTEDV0014`, `TestRubyProfileRowsCoverEveryClass_RUBYAFFECTEDV0014` |
| `RUBY-AFFECTED-V0-041` | `goModuleRoot`, `withoutTestdata` in `internal/liveverify/affected/ruby/ruby.go` | `TestGoModuleTestdataRubyFixtureIsNotAUnit_RUBYAFFECTEDV0041` |
| `RUBY-AFFECTED-V0-042` | `readStockAutoloadLib` in `internal/rubyautoload/rubyautoload.go`; `rails-7-1-autoload-lib` class in `conformance/ruby-profile-v0/rows.json` | `TestStockAutoloadLibIsReadAsLibRoot_RUBYAFFECTEDV0042`, `TestRubyProfileAffectedRows_RUBYAFFECTEDV0014`, `TestRubyProfileImpactRows_RUBYAFFECTEDV0014` |
| `RUBY-AFFECTED-V0-043` | `maskHeredocBodies`, `scanLexicalScopes` in `internal/liveverify/affected/ruby/nesting.go` | `TestHeredocAndLexicalNestingBoundConstantEdges_RUBYAFFECTEDV0043`, `TestHeredocMaskKeepsEvaluatedAndUnterminatedBodies_RUBYAFFECTEDV0043`, `TestLexicalScopesBalanceModifiersAndFallBack_RUBYAFFECTEDV0043`, `TestHeredocMaskRejectsShiftsAndKeepsInterpolation_RUBYAFFECTEDV0043`, `TestRegexKeywordFallsBack_RUBYAFFECTEDV0043` |

## Unresolved decisions

1. Accept an ownership/composition contract for `.feature`, `.yaml`, and other neutral test assets.
2. Decide whether a future seam version carries structured runner arguments and scenario/example
   anchors instead of encoding the runner class in a unit ID.
