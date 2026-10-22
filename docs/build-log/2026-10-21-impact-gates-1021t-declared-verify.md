# 2026-10-21 impact-gates-1021t: declared verify commands close impact plans

Tickets: V1-2071 (declared verify command ignored by `affected`/`impact`), V1-1848 (range and path
impact choose different gates). Requirements: `NGI-V0-060`, `NGI-V0-061`, `NGI-V0-062`
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
  orchestrator renumbers at batch time).

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

## Rollback

Revert this change: `closingGates` back to `gateCommand`, `rangeVerification` back to the profile
gate, drop `setVerification`'s authority member and `internal/projectprofile/verify.go` (moving the
fenced-block parser back into `cmd/corvint/affected.go`), and restore the analyzer schema pins.
