# Decision 0093 — Record the acceptance of `GPK-V0-055`/`GPK-V0-056` as a numbered decision

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12). This record does not change any requirement's
meaning. It gives an existing acceptance a numbered decision, and the rest of the governance record
cites it.

## Context

`GPK-V0-055` (query terms split camel/acronym boundaries before lowercasing) and `GPK-V0-056`
(path `impact` ranks same-package tests by declaration references) read "accepted 2026-09-05 by
repository-owner repair instruction". Both clauses, the Go change, and register entries `DR-0027`
and `DR-0028` landed together in `1e4db5d7` (2026-09-05 08:16). The only acceptance record was that
in-text phrase plus the 2026-09-05 build-log entry (an internal working record, not part of the public tree). No numbered decision named either
requirement. The 2026-09-12 counter-review of the divergence register rated `DR-0027`/`DR-0028`
PARTIAL for that reason. It inferred that the owner instruction was genuine but found no record
naming the wire change or its relation to `GPK-V0-002`/`GPK-V0-003`.

## Decision

1. **`GPK-V0-055` and `GPK-V0-056` are accepted as written**
   (`docs/specs/go-production-kernel-migration-v0.md:1108-1127@ece392ae`), effective 2026-09-05, and this
   decision is their numbered record.
2. **Each clause is a wire change and is named as one.** `GPK-V0-002`
   (`docs/specs/go-production-kernel-migration-v0.md:109-116@07cc5f22`) makes a normalization change a wire
   change owned by its existing spec. `GPK-V0-055` changes query term normalization, which moves
   which results `query`, `eval`, and `harness event --event user-prompt` return. `GPK-V0-056`
   changes `impact` test-row scores and appends syntax evidence to convention test rows. The
   owning clauses are this spec's own `GPK-V0-043` (query) and `GPK-V0-027` (impact) profiles, so
   the amendment belongs in this spec. It is not a migration exception.
3. **`GPK-V0-003` is not overridden.** Neither clause adds a receipt member, enum, or
   runtime-tagged variant. Each amends the one accepted ranking rule that every conforming runtime
   must follow. The frozen Python oracle was left unrepaired under `GPK-V0-033`, and its
   disagreement is recorded as `python-defect` in `DR-0027`/`DR-0028`. Decision 0088 has since
   retired the oracle, so those entries are historical records of why frozen oracle bytes would not
   match. `GPK-V0-003` still binds after decision 0088, and nothing here licenses a Go-only member.
4. **`LTA-V0-005`.** The accepted trust-boundary paragraph of
   `docs/specs/learned-trace-admission-v0.md` (bounded replay-window read kind, diagnostic, and
   truncated count) is given the requirement ID `LTA-V0-005` with its words unchanged, so `DR-0030`
   cites a numbered requirement and exact lines. Decision 0058 accepted the spec's intent and item 5
   permits registering Go learned-path changes. This item only numbers the text.

## Evidence

- Commit `1e4db5d7` carries both clauses, the Go change in `internal/contextindex/eval_query.go`
  and `internal/contextindex/impact.go`, and the regressions
  `TestEvalQueryCamelSplitsTaskBeforeLowering/GPK-V0-055` and
  `TestImpactRanksSamePackageTestsByDeclarationReferences/GPK-V0-056`
  (`internal/contextindex/ranking_regression_test.go:12@4e2f3325`, `:45@490f0cda`). Both failed on the pre-repair code
  (recorded in the 2026-09-05 build-log entry, an internal working record).
- The observed bytes and fixture hashes are in `DR-0027` and `DR-0028`
  (`conformance/divergence-register.md`).
- Blind-v3 recall moved 0.181818 to 0.363636 and development recall stayed 1.0, both measured
  after first observation. That is development evidence, not a held-out pass
  (recorded in the 2026-09-05 build-log entry, an internal working record).
- The discriminating `cli-parity-v0` rows are still `NOT_YET_AUTHORED`. This decision promotes
  nothing, and query and impact promotion stay BLOCKED under `GPK-V0-034`.

## Rollback

Revert this decision's commit. That restores the unnumbered in-text acceptance, the unnumbered
`LTA-V0` paragraph, and the prior citations. No runtime code, expectation, or manifest row changes.
Withdrawing either clause itself would need a new decision that reverts the `1e4db5d7` ranking
changes and closes `DR-0027`/`DR-0028`.
