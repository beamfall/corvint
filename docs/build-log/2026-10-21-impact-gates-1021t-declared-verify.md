# 2026-10-21 impact-gates-1021t: declared verify commands close impact plans

Tickets: V1-2071 (declared verify command ignored by `affected`/`impact`), V1-1848 (range and path
impact choose different gates), V1-1823 acceptance criteria 1-2 (Ruby RSpec classification and the
repository-wide bounded count; see the last section). Requirements: `NGI-V0-060` to `NGI-V0-064`
(`docs/specs/non-go-impact-v0.md`) and `AFP-V0-196` (`docs/specs/affected-plan-v0.md`), accepted
under the owner's standing auto-accept (2026-10-11).

## Problem

Reproduced on synthetic repositories shaped like the ticket's beamfall repositories:

| Fixture | Command | Before | After |
|---|---|---|---|
| npm, no Makefile, AGENTS.md `**Verify:** `npm run build`` | `impact` | `npm test`, `git diff --check` | `npm test`, `npm run build`, `verification_authority` {mandatory `npm run build`, inferred `npm test`} |
| same | `affected` | no check, `NO_REPOSITORY_GATE_DECLARED` | `npm run build` mandatory |
| npm + Makefile gate + Verify block `make gate` | `impact` | `npm test`, `make gate` (unlabelled) | same commands, `npm test` labelled inferred |
| Ruby + rspec + Makefile `test` target | path `impact` | `bundle exec rspec`, `make test` | unchanged |
| same | range `impact --base` | `bundle exec rspec`, `git diff --check` | `bundle exec rspec`, `make test` |

`impact` never read AGENTS.md; `affected` read only fenced blocks under `## Verify`; range impact
took its gate from the profile alone while path impact read the Makefile first.

## Decisions

- Grammar (the fork earlier lanes skipped on): accept an inline `**Verify:**`/`**Verify**:` line,
  whose directly following code spans are commands; the first other text ends the declaration, so
  ``**Verify:** `make gate` (Tasks gate id `gate`)`` yields only `make gate`. Do not accept
  `## Build / gate` or other headings: V1-0342 deliberately takes only the exact "Verify" heading,
  and widening it would turn arbitrary build notes into mandatory checks.
- One parser in `internal/projectprofile/verify.go` serves `affected` and `impact`, so they cannot
  disagree about one file. Behavior change: a fence with a non-shell info string now hides its
  content from every rule (previously only the scope reader skipped it, and an inline label inside a
  `markdown` example would have counted).
- Declared commands replace the inferred closing gate rather than being appended beside it; the
  language commands before them stay, labelled `inferred` (AGENTS.md invariant 3: project authority
  outranks inference, and inferred commands are still useful hints).
- Path and range impact share `closingGates`, which closes V1-1848.
- Corvint's own AGENTS.md declares four Verify commands, so `corvint impact` on this repository now
  closes with them instead of `make gate`; that is the intended reading of its declaration.
- The analyzer inputs changed (`internal/contextindex`, `internal/projectprofile`), so the schema
  moves to `corvint-analyzer/143` (139 on the base; 141 and 142 are taken by open PRs; the
  orchestrator renumbers at batch time). Batch 1021v renumbered it to `corvint-analyzer/144`,
  because batch #1505 took 142 and #1509 took 143.

## Evidence

- `TestNGIV0060DeclaredVerifyCommandClosesMakefileLessPlan`, `TestNGIV0061InferredCommandBesideDeclaredGate`,
  `TestNGIV0062RangeAndPathChooseOneGate` failed with the receipt changes reverted
  (`[npm test git diff --check]`, missing `verification_authority`, range `[bundle exec rspec git diff --check]`)
  and pass with them.
- `TestAFPV0196InlineVerifyLine`, `TestNGIV0060MandatoryVerifyCommands` (new parser; no inline
  branch existed before), `TestAFPV0196AffectedInlineVerifyIsMandatory` (base `affected` has no
  inline grammar, so it emits no AGENTS.md check).
- Full `./internal/contextindex` and `./internal/projectprofile` packages and the affected advice
  tests in `./cmd/corvint` pass; `script/analyzer-schema.sh check` passes.

## NOT_RUN

`make gate`; tier-2 packages of `corvint affected` (left to batch CI); live runs against the real
beamfall repositories (private, not cloned in this lane).

## V1-1823: Ruby request rows and the bounded unknown

Problem: PR #1137's round-1 limits. A Ruby request classified RSpec paths as tests only in the
autoload rows, so a `spec/*_spec.rb` feature marker reached from a mixed request kept
`source-marker` authority; and `ruby-reference-index-bounded` was the repository-wide count of cut
references, emitted on every mapped changed path whether or not a cut could hide its referrers.

Decisions: `NGI-V0-063` takes a shallow index view (`withRubyTests`) for any path or range request
naming a Ruby path, so every row uses `isRubyTestPath`; requests naming no Ruby path never take it,
and `TestRubySpecPathNonRubyRequestBaseBytes` still matches its base-commit golden. The receipt
coverage gap rule stays on the shared rule (non-goal). `NGI-V0-064` records each cut reference
(`rubyCut`) and counts, per plan, only those outside the changed file that could name a traversed
constant; nesting overflow stays conservative. `TestAnalyzerBoundsFailClosed/ruby-resolutions` now
changes the constant past the bound, the one a truncated reference could name.

Evidence: the four new or changed assertions (`TestNGIV0063RubyRequestClassifiesSpecMarkersAsTests`,
`TestNGIV0063RubyRangeClassifiesSpecMarkersAsTests`, `TestRubyAutoloadResolutionBound`,
`TestRubyAutoloadSiteBound`) failed with the behavior lines reverted and pass with them; the full
`./internal/contextindex` package passed. The analyzer schema stays `corvint-analyzer/143`; its
digest was repinned.

Residual: V1-1823 has no criterion beyond these two; none.

## Rollback

V1-1823: drop the `withRubyTests` view and emit `rubyAutoload.bounded` again.

Revert this change: `closingGates` back to `gateCommand`, `rangeVerification` back to the profile
gate, drop `setVerification`'s authority member and `internal/projectprofile/verify.go` (moving the
fenced-block parser back into `cmd/corvint/affected.go`), and restore the analyzer schema pins.
