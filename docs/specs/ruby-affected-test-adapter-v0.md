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
- Exists: `internal/liveverify/affected/ruby` static planning adapter and focused seam tests.
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
  autoloading, Cucumber feature presence, and a nonliteral, absolute, or repository-escaping
  escaped, nonliteral, absolute, or repository-escaping `Rake::TestTask` `.libs` declaration MUST
  produce stable frontier reasons. A frontier makes
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
| Real Ruby corpus | independent discovered/actual file count | observed: 24/24, 40/40, and conservative 34/31; not qualification |
| Full repository and CLI parity | unchanged existing receipts with explicit candidate proof | `NOT_RUN` until recorded |

## Non-goals and promotion gate

This slice does not execute Ruby tests, inspect runtime coverage, parse the full Ruby grammar,
discover individual examples/scenarios/methods, claim Cucumber assets, change `Language`, change
`Select`, qualify safe omission, or advertise Ruby support. The simpler recovery remains each
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

## Unresolved decisions

1. Accept an ownership/composition contract for `.feature`, `.yaml`, and other neutral test assets.
2. Decide whether a future seam version carries structured runner arguments and scenario/example
   anchors instead of encoding the runner class in a unit ID.
