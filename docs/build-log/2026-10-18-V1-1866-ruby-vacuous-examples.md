# 2026-10-18 V1-1866: Ruby vacuous examples and cross-file helpers in test-validity

Ticket: V1-1866 (RSpec examples whose assertion cannot fail pass `corvint tests validity`).
GitHub #1163.

## Decision

`TSA-V0-084` to `TSA-V0-088` are added to `docs/specs/test-source-analysis-v0.md`. They are
accepted under the owner's standing auto-accept of lane requirements (2026-10-11).

- Three invalid rules make the example `INELIGIBLE`:
  - `RB-BLOCK-NEVER-YIELDED`: a block passed to a helper whose `def` takes no `&`/`...`
    parameter and holds none of a closed set of yield, forwarding and metaprogramming words;
  - `RB-CHANGE-ON-IDENTITY`: `not_to change { x.reload }`;
  - `RB-ASSIGNMENT-IN-PREDICATE`: a `satisfy` block that ends in an assignment.
- `RB-RAISE-ERROR-ONLY` is advisory only, as the ticket asks.
- Helper resolution crosses files: the example's groups, then shared contexts they include from
  the spec or the bound support set (`spec_helper`, `rails_helper`, `spec/support/**`), then the
  file level. Support files are read at the CLI's commit, or from the receipt's retained helpers
  only when each blob still matches.
- Abstain, never guess: an unresolvable, non-literal, unreadable or module-defined helper gives
  `RB-HELPER-UNRESOLVED` and two definitions give `RB-HELPER-AMBIGUOUS`. Both are a new
  `abstained` severity: `ApplySourceFindings` turns `ELIGIBLE` into `ABSTAINED` when no finding is
  invalid. They fire only for a block-passing call that could reach a never-yielding `def`, to
  keep noise down.
- The reader stays the TSA-V0-062 line reader; one-line `def …; end` and endless defs are read.

## Evidence

Focused tests in `./internal/testsource` (`TestRSpecBlockNeverYielded*`,
`TestRSpecHelperAbstentions`, `TestRSpecChangeOnIdentity`, `TestRSpecAssignmentInPredicate`,
`TestRSpecRaiseErrorOnlyIsAdvisory`), with mutation checks recorded in the PR. Fixtures are
synthetic; live qualification on a real suite is NOT_RUN.

## Residuals

- Module `include`, `config.include`, `define_method` and `alias` are not followed.
- A helper that hands its block on through a method the reader cannot see is excluded by the `&`
  rule, so the rule under-reports.
