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
- `RUBY-AFFECTED-V0-005`: Unreadable or undecodable Ruby, dynamic/interpolated loading,
  conditional loading, reflection/metaprogramming that can define tests, Rails
  autoloading that `RUBY-AFFECTED-V0-010` cannot bound, Cucumber feature presence, and a nonliteral, absolute, or repository-escaping
  escaped, nonliteral, absolute, or repository-escaping `Rake::TestTask` `.libs` declaration MUST
  produce stable frontier reasons (the stock Rails gem-group load is not dynamic loading, see
  `RUBY-AFFECTED-V0-013`). A frontier makes
  selection `UNKNOWN_SCOPE`; it MUST NOT silently omit a potentially runnable test. (amended for
  V1-2363: each reason is located and scoped by `RUBY-AFFECTED-V0-046..048`; an ambiguous repository
  load becomes edges to every candidate.)
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
  controller test (by directory) MUST also carry edges to each observed file under
  `app/controllers/` and `app/helpers/`, because routing dispatches to them without a constant
  reference in the test; those are route-dispatch edges (`RUBY-AFFECTED-V0-049`), not advisory ones.
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
- `RUBY-AFFECTED-V0-044`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2158)
  The literal mask of `RUBY-AFFECTED-V0-009` MUST mask a `/.../` regex literal that opens in an
  operand position (at the start of its line, after an operator, comma or opening bracket, or after
  one of the keywords `and`, `elsif`, `if`, `in`, `not`, `or`, `return`, `unless`, `until`, `when`,
  `while`, `yield`) and closes on the same line, with `\` escapes and `[...]` classes honoured, so
  a quote inside it neither opens a string nor hides the constants after it; `%r{...}`, `%r(...)`
  and the other `%r` delimiters are masked as percent literals. A slash after an identifier, a
  number, a method call or a closing bracket is division and masks nothing; a candidate that does
  not close on its line masks nothing.
- `RUBY-AFFECTED-V0-045`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2306,
  GH #1475) A repository with no `config/application.rb` (root or nested) whose Ruby sources still
  follow the autoload convention `corvint impact` maps, meaning a present `.corvint/ruby-autoload.json`
  declaration or a default `app/<dir>` root of `internal/rubyautoload`, MUST carry the
  `RUBY-AFFECTED-V0-009` advisory constant edges built by that same mapper (roots, ignore list,
  declared and configured inflections, lexical namespaces), so a spec reaching a changed file only
  through an autoloaded constant is selected with `ADVISORY_CONSTANT_REFERENCE` and is never a
  `NO_DEPENDENCY_PATH_TO_DIRTY_UNIT` exclusion. `Select` traverses these edges transitively with no
  hop bound, so a chain longer than impact's two advisory hops is still selected. The Rails boot and
  route edges of `RUBY-AFFECTED-V0-011` are not added. The `RUBY-AFFECTED-V0-010` bounds apply
  unchanged: a dynamic constant lookup, an unresolvable association, an invalid declaration or
  unresolved autoload configuration MUST raise `ruby:rails-autoload` when the repository has tests,
  and a Rails test without a root `config/application.rb` still raises it. A repository with neither
  a declaration nor an `app/<dir>` root (a plain library under `lib/`) keeps its require and
  counterpart edges alone. A spec that references no constant reaching the dirty file stays a
  `NO_DEPENDENCY_PATH_TO_DIRTY_UNIT` exclusion. Impact's per-path baseline
  `dynamic-dispatch-unresolved` row is a statement that syntax never proves closure, not evidence
  about the dirty set; affected keeps its own evidence-based Ruby frontiers (dynamic constant lookup,
  dynamic or conditional load, `autoload` calls) and does not copy that baseline, since doing so
  would make every Ruby plan `UNKNOWN`.
- `RUBY-AFFECTED-V0-046`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2363,
  GH #1519) Every Ruby frontier entry MUST name its site: `reason:path:line` when a line holds the
  cause, `reason:path` when only the file is known (an unreadable file, a Cucumber feature, a test
  with no visible edge, an invalid declaration, a nested `config/application.rb`), and the bare
  reason only when no observed file holds it (Rails test signals with no Rails test file). Paths are
  repository-relative. One plugin or unit frontier MUST name at most 16 sites of one reason, in
  sorted order, followed by one `reason:+N` entry counting the rest, so the list stays bounded. A
  literal repository load, or a test's conventional counterpart, that resolves to more than one
  observed file (`lib/x.rb` and `app/x.rb`) MUST carry an import edge to every candidate instead of
  raising `ruby:ambiguous-load`: the runtime loads one of them, so the closure over all of them is a
  superset.
- `RUBY-AFFECTED-V0-047`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2363,
  GH #1519) A site that bears only on the unit whose file holds it MUST be that unit's `frontier`
  member and is named only by a plan that reaches the unit (the AFP-V0-004 unit frontier rule): a
  conditional load whose target resolves (`ruby:conditional-load`, the target is an edge), a
  `require_relative` that resolves to no file (`ruby:relative-load-unresolved`), a possible generated
  test (`ruby:dynamic-test-definition`), and an ambiguous test framework
  (`ruby:ambiguous-test-framework`). A site that can hide an edge to any changed file MUST stay
  plugin-wide: a computed or interpolated load, `$LOAD_PATH`/`$:` manipulation, autoload-root
  configuration the autoload mapper does not read, a dynamic constant lookup, an association whose
  target maps to no observed file, an unresolved inflection or autoload configuration, a test with no
  visible edge (`ruby:implicit-test-dependency`), a unit whose edges exceed
  `affected.MaxPathsPerUnit`, an unreadable file, and a Cucumber feature. So a change whose reverse
  closure reaches no per-unit site, in a repository with no plugin-wide site, is `BOUNDED` with its
  tier-1 selection, and a change that reaches one stays `UNKNOWN` naming exactly that site.
- `RUBY-AFFECTED-V0-048`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2363,
  GH #1519) A committed `.corvint/ruby-frontiers.json`, strict JSON
  `{"schema":"ruby-frontiers/0","resolved":[...]}` with no other member, MUST resolve each named
  site: an exact `reason:path:line` entry resolves that site only, and a `reason:path` entry resolves
  every site of that reason in the file. Only `ruby:dynamic-load`, `ruby:conditional-load`,
  `ruby:dynamic-test-definition`, `ruby:ambiguous-test-framework`, `ruby:relative-load-unresolved`,
  `ruby:implicit-test-dependency` and `ruby:rails-autoload` sites are declarable; an unreadable file
  or a Cucumber feature is not. A declaration that is unreadable, over 1 MiB, not that exact schema,
  has an unknown or duplicate member, a missing `resolved` list, a duplicate or malformed entry, an
  undeclarable reason or a non-canonical path resolves nothing and MUST raise the plugin-wide
  `ruby:frontier-declaration-invalid:.corvint/ruby-frontiers.json`. A declared line that no longer
  matches a site resolves nothing. The declaration is the caller's claim that the site's tests are
  covered; Corvint does not verify it.
- `RUBY-AFFECTED-V0-960`: (experimental under the owner's standing lane auto-accept (2026-10-11);
  V1-2369, amends `RUBY-AFFECTED-V0-048`) A declared entry that resolves no observed site MUST NOT be
  dropped silently: an exact `reason:path:line` entry whose site moved, changed reason or is gone,
  and a `reason:path` entry whose file holds no site of its reason, MUST each raise the plugin-wide
  `ruby:frontier-declaration-stale:path[:line]` site at the entry's own path and line, bounded per
  reason like every other site (`RUBY-AFFECTED-V0-046`). Corvint MUST NOT re-anchor an entry to a
  shifted line: the declaration is an unverified claim about one site, so after a line shift both
  the stale entry and the undeclared site at its new line stay visible until the caller updates the
  declaration. Failure mode: a line shift leaves a dead entry the caller still believes resolves its
  site. Falsifier: a declaration naming a moved, changed or deleted site whose plan names no stale
  site. Rollback: drop `staleDeclarations` and its call in `Language.Units`; a stale entry again
  resolves nothing silently. Acceptance: `TestFrontierDeclarationStaleEntryIsReported_V12369`.
- `RUBY-AFFECTED-V0-049`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2448,
  GH #1565) The controller and view-helper edges of `RUBY-AFFECTED-V0-011` MUST be carried as
  `Unit.RouteDispatchImports`, a sorted list that is part of the graph digest only when non-empty
  (member `routeDispatchImports`), and MUST omit every target the test already reaches by an import
  or advisory constant edge. `Select` MUST traverse them like advisory edges, after explicit and
  advisory edges at each depth, and every unit reached through one, directly or transitively, MUST
  carry the witness `ROUTE_DISPATCH_BOOT`, so a plan or receipt names the boot fallback apart from a
  named constant (`RSpec.describe Foo::BarController`). Selection is unchanged: every
  route-dispatched test is still selected on any controller or helper change. Under `AFP-V0-162` a
  `ROUTE_DISPATCH_BOOT` unit is tier 2 unless the same-parent rule promotes it, so in a large Rails
  application a controller change puts the specs naming that controller in tier 1 and the routed
  fan-out in tier 2. A reader that knows only the older witness kinds sees an unknown kind and MUST
  treat it as it treats any other non-direct witness (selected, not narrowed); the `config/` boot
  edges stay advisory. A controller-type test keeps the fan-out to every controller and helper,
  because a controller spec can render views or call helpers of other controllers.
- `RUBY-AFFECTED-V0-050`: (experimental under the standing lane auto-accept; V1-2473, GH #1601)
  One graph build MUST list the suite-helper and support candidates of `RUBY-AFFECTED-V0-012` once
  per suite directory, and the `config/`, controller and view-helper candidates of
  `RUBY-AFFECTED-V0-011` once per observation, instead of rescanning every unit for every test.
  Each test MUST still receive exactly the edges the per-test scan gave it, so units, graph bytes,
  selection, witnesses and receipts are unchanged. Measured on synthetic Rails-shaped trees
  (`affected --no-cache --path`, one model change): 20,001 files with a branching require graph
  went from 5.82 s to 2.39 s, and a 12,804-file Rails layout went from 2.54 s to 2.14 s, with
  byte-identical output and unchanged peak RSS. Non-goals: concurrency in the scan (excluded by
  `AFP-V0-394`), peak memory (on a deep require chain it is set by the receipt size and
  `gokernel.WriteCanonicalJSON` buffering the whole document, which this requirement does not
  change), and the reporter's real-checkout timings, which stay `NOT_OBSERVED` here. Failure
  mode: a memo keyed wrongly would hand a test another suite's helpers (over-selection) or drop
  its own (under-selection); the parity test compares every test's edges against the per-test
  reference scan, twice over one memo.

- `RUBY-AFFECTED-V0-761`: (experimental under the owner's standing lane auto-accept (2026-10-11); V1-1831)
  The adapter MUST also own every file below the root `app/views/` directory (an `erb`, `haml`,
  `slim`, `jbuilder` or any other template; a `.rb` file there stays a Ruby file) and make each
  template the source unit `ruby:view:<path>`, never scanned as Ruby. In a Rails application every
  route-dispatched test (`RUBY-AFFECTED-V0-011`: a test under `requests`, `system`, `features`,
  `integration`, `routing` or `controllers`, or one declaring such a type) MUST carry every view
  unit as a `RUBY-AFFECTED-V0-049` route-dispatch edge, because a controller renders a template by
  name and no constant names it; a view change then selects those tests with the advisory
  `ROUTE_DISPATCH_BOOT` witness and adds no frontier, so the plan keeps its scope. A template
  outside the root `app/views/` (an engine's, a mailer's under `lib/`) stays unowned and raises
  `UNOWNED_DIRTY_PATH`; a view in a repository with no Rails application, or with no
  route-dispatched test, reaches no test and abstains as `NO_SELECTABLE_TEST` naming its unit.
  Non-goals: no render-call or partial resolution, so a view change over-selects every
  route-dispatched test. Rollback: drop `isView` from `Owns` and the view units and edges.

### Autoload edge failure modes

- A dynamic constant lookup or unresolvable association hides its target: the slice keeps
  `ruby:rails-autoload` (`UNKNOWN_SCOPE`) rather than guessing an edge.
- Lexical nesting falls back to the namespaces a file opens, and an ambiguous, evaluated or
  unterminated heredoc stays unmasked: both can add false advisory edges, which only widen
  selection (RUBY-AFFECTED-V0-043). A quote inside a `/.../` regex literal that is not in an operand
  position (`x.split /'/`) still opens a string in the literal mask (RUBY-AFFECTED-V0-044).
- A constant defined outside an observed `.rb` file (gem, engine, generated code) has no edge; the
  referencing test still selects on its own file and on boot/support edges, and an unmapped
  association raises the frontier.
- More than `affected.MaxPathsPerUnit` imports or advisory edges on one unit drop both lists and raise
  `ruby:implicit-test-dependency` instead of truncating.
- Non-Ruby views, templates, and assets stay unowned and produce `UNOWNED_DIRTY_PATH` when dirty.
- A per-unit site bears on its own unit's tests only (`RUBY-AFFECTED-V0-047`). A conditional load,
  an unresolved relative load, a generated test or an ambiguous framework in a unit no change reaches
  cannot change which of the reached tests run. A construct that hides an outbound edge stays
  plugin-wide, so a large Rails application with any `constantize` stays `UNKNOWN` until each named
  site is declared (`RUBY-AFFECTED-V0-048`). A declaration is unverified caller authority.
- Text scanning finds a site by line: a construct split over lines is named at the line the scan
  matches, and a declaration keyed to an old line stops resolving it after an edit above it.

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
- Rollback of `RUBY-AFFECTED-V0-049` is reverting its change: route edges return to
  `advisoryImports` with the `ADVISORY_CONSTANT_REFERENCE` witness and tier 1. Selection does not
  change in either direction; Rails graph digests do, and non-Ruby graphs keep their bytes.
- Rollback of `RUBY-AFFECTED-V0-050` is reverting its change: the per-test scans return and no
  output changes in either direction.

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
| Rails route-dispatch tiers (#1565) | a controller change ranks the spec naming it tier 1 and another controller's spec tier 2 with `ROUTE_DISPATCH_BOOT` | experimental |
| Suite helpers and support | helpers stay source; a helper or `support/**` change selects every suite test | experimental |
| Stock Rails gem-group load | `Bundler.require(*Rails.groups)` in `config/application.rb` keeps an `app/models` change `BOUNDED`; other dynamic requires widen | experimental |
| Ruby profile conformance fixture (#1130 S6) | `conformance/ruby-profile-v0` impact, affected and provider rows per fixture class, negative dynamic-lookup rows, run by the go-product CI shards | experimental; Ruby runtime `NOT_RUN` |
| Real Ruby corpus | independent discovered/actual file count | observed: 24/24, 40/40, and conservative 34/31; not qualification |
| Full repository and CLI parity | unchanged existing receipts with explicit candidate proof | `NOT_RUN` until recorded |

## Non-goals and promotion gate

This slice does not execute Ruby tests, inspect runtime coverage, parse the full Ruby grammar,
discover individual examples/scenarios/methods, claim Cucumber assets, change `Language`, qualify
safe omission, or advertise Ruby support. The only seam change is the additive, digest-neutral-when-
empty `Unit.AdvisoryImports` edge and its `Select` witness (`RUBY-AFFECTED-V0-009`), and its
route-dispatch counterpart `Unit.RouteDispatchImports` (`RUBY-AFFECTED-V0-049`); it does not
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
| `RUBY-AFFECTED-V0-044` | `maskQuotedLiteralsAndInlineComments`, `rubySlashRegex` in `internal/liveverify/affected/ruby/ruby.go` | `TestRegexLiteralMaskHidesQuotesNotDivision_RUBYAFFECTEDV0044` |
| `RUBY-AFFECTED-V0-046..048` | `UnitsSource`, `scanLoads`, `resolveLoads` in `internal/liveverify/affected/ruby/ruby.go`; `internal/liveverify/affected/ruby/frontier.go`; `edges`, `mapAutoload` in `autoload.go`; `UnresolvedConfigSites` in `internal/rubyautoload/rubyautoload.go` | `TestLocatedFrontierCountsOnlyCrossedSites_RUBYAFFECTEDV0047`, `TestRepositoryWideLoadSitesStayPluginWideAndLocated_RUBYAFFECTEDV0046`, `TestFrontierDeclarationResolvesNamedSites_RUBYAFFECTEDV0048`, `TestLoadsResolveOnlyUniqueRepositoryTargets`, `TestRubyProfileAffectedRows_RUBYAFFECTEDV0014` |
| `RUBY-AFFECTED-V0-960` | `staleDeclarations` in `internal/liveverify/affected/ruby/frontier.go`, its call in `ruby.go` | `TestFrontierDeclarationStaleEntryIsReported_V12369` |
| `RUBY-AFFECTED-V0-049` | `bootEdges` in `internal/liveverify/affected/ruby/autoload.go`; `UnitsSource` in `internal/liveverify/affected/ruby/ruby.go`; `Unit.RouteDispatchImports` and `traverse` in `internal/liveverify/affected/`; `tierAffectedSelections` in `cmd/corvint/affected_tiers.go`; `request-spec` and `controller-spec` rows in `conformance/ruby-profile-v0/rows.json` | `TestRouteDispatchEdgesCarryTheirOwnWitness_RUBYAFFECTEDV0049`, `TestRubyRouteDispatchSpecsAreTierTwo_RUBYAFFECTEDV0049`, `TestRubyProfileAffectedRows_RUBYAFFECTEDV0014` |
| `RUBY-AFFECTED-V0-050` | `supportIndex.supportEdges` and `bootEdges` in `internal/liveverify/affected/ruby/autoload.go`; `UnitsSource` in `internal/liveverify/affected/ruby/ruby.go` | `TestMemoizedSupportAndBootEdgesMatchPerTestScan_RUBYAFFECTEDV0050`, `TestSuiteHelpersAndSupportSelectEverySpec_RUBYAFFECTEDV0012`, `TestRailsBootAndRouteEdgesSelectTheirSpecs_RUBYAFFECTEDV0011` |
| `RUBY-AFFECTED-V0-045` | `newConventionAutoload`, `mapAutoload` in `internal/liveverify/affected/ruby/autoload.go`; `UnitsSource` in `internal/liveverify/affected/ruby/ruby.go` | `TestConventionAutoloadEdgesWithoutRailsApplication_RUBYAFFECTEDV0045`, `TestConventionAutoloadFailsClosed_RUBYAFFECTEDV0045`, `TestRailsSignalsWithoutApplicationStayUnbounded_RUBYAFFECTEDV0045`, `TestAffectedSelectsEveryImpactAutoloadTest_RUBYAFFECTEDV0045` |
| `RUBY-AFFECTED-V0-761` | `Owns`, `isView`, `viewUnitID` and the view route edges in `UnitsSource` in `internal/liveverify/affected/ruby/ruby.go` | `TestViewTemplateSelectsRouteDispatchedSpecs_RUBYAFFECTEDV0761` (a view change selects every route-dispatched spec with `ROUTE_DISPATCH_BOOT` and stays `BOUNDED`; a template outside `app/views` stays unowned; a library view abstains as `NO_SELECTABLE_TEST`) |

## Unresolved decisions

1. Accept an ownership/composition contract for `.feature`, `.yaml`, and other neutral test assets.
2. Decide whether a future seam version carries structured runner arguments and scenario/example
   anchors instead of encoding the runner class in a unit ID.
