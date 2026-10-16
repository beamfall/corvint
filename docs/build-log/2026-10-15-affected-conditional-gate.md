# 2026-10-15 — V1-0682: a scoped full gate is advised as `conditional`

Base: `f81114713c96cf629462dfd9e91e7d370ee848c4` (origin/main). Ticket: V1-0682. Requirement:
AFP-V0-009, amended by the owner (decision 0531, citing decisions 0052 and 0424). The proposed
amendment is in `docs/build-log/2026-10-15-gate-tooling.md`.

## Contract

- `advice.checks[].kind` is now one of `mandatory`, `conditional` or `advisory`. The member set is
  unchanged.
- A scoping sentence is prose under the exact `Verify` heading of AGENTS.md, outside fences, that
  contains "unless", "only when" or "only if". A paragraph's wrapped lines are joined before it is
  split into sentences.
- A declared `mandatory` command named in backticks by such a sentence becomes `conditional`. Its
  reason quotes the first such sentence verbatim. It keeps its place and counts toward the cap.
- `conditional` still suppresses `NO_REPOSITORY_GATE_DECLARED`. It is not echoed as mandatory by
  ETS-V0-009 selection or `corvint test`, which both filter on `kind == mandatory`.
- Commands no sentence names stay `mandatory`. Requiring too much is still the default.

## Decisions

- Only the Verify section counts. It is the same project-owned authority that declares the checks,
  so a sentence elsewhere ("`make gate` fails unless Docker runs") cannot relax a check.
- The command must be named exactly in backticks. Prose that refers to "the exhaustive commands
  below" names no command, so Corvint's four Verify fence commands stay `mandatory`. This is a
  stated non-goal, not a parsing gap.
- No new `unknown` line: the condition sits in the reason and is never evaluated.

## Evidence

- Observed advice at BASE (installed corvint 1.0.0-rc.3, `corvint affected --base BASE` in the lane
  clone): `make gate` `mandatory`, source `Makefile`, reason "the repository Makefile declares a
  gate target, so the full gate stays mandatory".
- Same command with the lane build: `make gate` `conditional`, reason quoting "Do not start
  `make gate` unless the owner asks for it or a governing release policy explicitly requires
  repository-wide validation." The four Verify commands stay `mandatory`.
- `TestAffectedAdviceLabelsScopedGateConditional_V1_0682` covers five cases: scoped (wrapped
  across lines), unscoped, sentence outside Verify, a sentence naming a Verify fence command, and
  no AGENTS.md. Only the scoped cases are `conditional`.
- Mutation: removing the `scopeConditionalChecks` call fails the scoped and Verify-command cases
  (1 test, 2 failing cases); restored, it passes.
- `go test -run 'TestAffectedAdvice|TestAffectedReceiptMembers|TestShellQuote' ./cmd/corvint`
  passed with `-parallel 1` (409.7 s, load averages 62 to 66 on 12 cores). The default parallelism
  failed six existing advice tests with `git-cancelled` at load 69; that is host load, not this
  change.
- `make gate` is `NOT_RUN` (owner's standing preference for scoped issue work).
