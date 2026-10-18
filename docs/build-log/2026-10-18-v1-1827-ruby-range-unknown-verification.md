# 2026-10-18 V1-1827: Ruby range receipts name unknown verification

Ticket: V1-1827 (range impact over a Ruby range with no proved suite drops the unknown-verification
line that path impact reports). Residual of V1-1804 (PR #1145, `NGI-V0-051`). GitHub #1130.

## Decision

`NGI-V0-059` is added to `docs/specs/non-go-impact-v0.md` and accepted under the owner's standing
auto-accept of lane requirements (2026-10-11). It replaces the `NGI-V0-051` non-goal that kept the
unknown-language line out of range receipts.

- `RangeImpact` appends the path-impact line ("no language-specific verification command is known
  for this repository") to `coverage.uncertainty` when the range admitted a `.rb` path and
  `lacksLanguageVerification`, the rule path impact uses, holds for the range plan. It sits after
  the authority lines and before the Ruby advisory line.
- A range with no admitted Ruby path never evaluates the rule, so Go range receipts keep their
  bytes. A plan that names a Go or Ruby command omits the line, as path impact does.
- `analyzerSchemaID` is bumped to `corvint-analyzer/128` because the IDX-SNAP-V0-017 audit digests
  every contextindex source; no extracted fact changed.

Requirement IDs: `NGI-V0` on origin/main stops at `NGI-V0-053` and no open PR names a higher one.
`054..058` are left free for the concurrent Ruby lane (V1-1830).

## Evidence

Real CLI built from this branch, over a committed Minitest repository with no proved runner
(`gem "minitest"`, tracked `test/`, no Rakefile) and `app/models/widget.rb` changed:

- `corvint impact --base <base>`: verification `["git diff --check"]`, and the unknown-language
  line is present. The installed `1.0.0-rc.3 (build 634)` gives the same plan without the line.
- `corvint impact app/models/widget.rb`: the same plan and the same line.

Tests: `TestNGIV0059RubyRangeReportsUnknownVerification` (fails before the change),
`TestNGIV0059RangeWithoutRubyPathKeepsUncertainty`, and the full `./internal/contextindex` package,
including `TestNonGoRangeImpactLegacyGoBytes` and the `NGI-V0-051` Go byte test.

## Rollback

Revert the change: the branch in `RangeImpact`, the tests and the schema bump go. No stored bytes
depend on it; snapshots rebuild under the previous analyzer schema.
