# Decision 0531 — AFP-V0-009 amended: a scoped full gate is `conditional`

Date: 2026-10-15. Status: accepted by the owner. Authority: owner approval in chat to the
orchestrator, 2026-10-15 (approve amending accepted AFP-V0-009 so that a project-scoped full gate
is labelled `conditional` with its condition named). Ticket: V1-0682.

## Context

Decision 0052 accepted AFP-V0-009: a Makefile `gate:` target yields a `mandatory` `make gate`, and
each AGENTS.md Verify fence command is `mandatory`. Decision 0424 (V1-0342) narrowed the Verify
heading match and kept "requiring too much is safe". Corvint's own AGENTS.md Verify section says
"Do not start `make gate` unless the owner asks for it or a governing release policy explicitly
requires repository-wide validation", yet `corvint affected` labelled `make gate` mandatory because
the Makefile target exists. The advice contradicted project-owned authority (invariant 3) and was
recorded as owner-decision work in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-gate-tooling.md`.

## Decision

AFP-V0-009 gains a third `kind`, `conditional`. A declared mandatory command whose exact text
appears in backticks in a prose sentence of the AGENTS.md Verify section that states a condition
("unless", "only when" or "only if") is `conditional`, and its reason quotes that sentence. The
condition is named, never evaluated. A conditional entry is still a declared gate (no
`NO_REPOSITORY_GATE_DECLARED`) but is not a `mandatory` check for ETS-V0-009 or `corvint test`.
Every command no such sentence names stays `mandatory`, as before.

## Limits

Prose that scopes commands without naming them (Corvint's "the exhaustive commands below") does not
relabel them, so Corvint's four Verify fence commands stay `mandatory`. Conditions stated in other
files are not read. `make gate` is `NOT_RUN` for this change.

## Rollback

Revert this decision, the AFP-V0-009 amendment paragraph and `kind` wording, and
`scopeConditionalChecks`/`agentsVerifyScopes` in `cmd/corvint/affected.go`, then regenerate
`docs/specs/REQUIREMENTS.tsv`. Every declared command is `mandatory` again.
