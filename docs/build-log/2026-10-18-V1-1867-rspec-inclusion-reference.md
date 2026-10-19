# 2026-10-18 V1-1867: RSpec shared examples through one inclusion

Ticket: V1-1867 (a shared example is only ever `SHARED-EXAMPLE-ONLY`, even when one inclusion ran
it). GitHub #1164.

## Decision

`TSA-V0-089` to `TSA-V0-092` are added to `docs/specs/test-source-analysis-v0.md`. They are
accepted under the owner's standing auto-accept of lane requirements (2026-10-11).

- The qualified reference `spec/x_spec.rb:<include_line>>spec/support/y.rb:<example_line>`, or
  `same:<line>`, names one inclusion. The shared body is spliced where RSpec evaluates it
  (`it_behaves_like` nests a group before its customization block; `include_examples` and
  `include_context` expand inline), and the example gets the normal verdict with locations kept in
  their own files. A bare reference keeps `SHARED-EXAMPLE-ONLY`.
- Each inclusion is read independently, so two sites of one group can differ.
- Every unbindable case abstains with its own reason: inclusion not found, in an example or hook,
  loop-generated or non-literal; shared group not found, ambiguous, nested or unbound; example not
  in the group or loop-generated.
- Receipt discovery joins an example whose RSpec id names another spec than its file: the scoped
  id `[1:3:2:1]` is walked as RSpec numbers it, the `full_description` must contain the
  `it_behaves_like` description, and the including spec's blob comes from another retained
  example. Each failure abstains with a named reason.
- `testvaliditydoc` cannot import `testsource`, so `DiscoverWithReaders` takes the Ruby readers;
  the CLI and the MCP bridge pass them.

## Evidence

Focused tests: `TestRSpecInclusion*` in `./internal/testsource`,
`TestSourceValidityQualifiedReference` and `TestDiscoverRubyInclusionJoin`, with mutation checks
recorded in the PR. Fixtures are synthetic; live qualification is NOT_RUN.

## Residuals

- Shared group block parameters and `it_behaves_like 'x', args` arguments are not bound.
- Module `include` and `config.include_context` are not followed.
