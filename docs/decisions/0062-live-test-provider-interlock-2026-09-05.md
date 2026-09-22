# Decision 0062 — The live test provider carries a tested no-caller interlock until containment exists

Date: 2026-09-05. Status: accepted. Authority: repository owner, delegated call; the owner instructed on 2026-09-05 that open calls be resolved through the expert panel, and this record applies the panel memo `docs/reviews/panels/panel-5-release-platform.md` after independent review of its cited evidence.

## Context

`docs/reviews/r10-security.md` section 2.3: the live test provider runs `go test` unsandboxed in the
real worktree. No `cmd/corvint` command calls it and the release profile builds `./cmd/corvint`
alone, so the exposure is latent; the only control is an unwritten review invariant with no test.

## Decision

A new `GLTP-V0-044` in `docs/specs/go-live-test-provider-v0.md`, with a pointer sentence in
`GLTP-V0-029`: (a) no `cmd/corvint` command may invoke the provider or `gorunner` until an accepted
containment profile exists; (b) the run's existing nested containment emission is the only
containment statement, because the `go-live-run/0` terminal schema is frozen and a new top-level
member would change its identity (correction recorded on implementation, 2026-09-05); (c) once a
containment profile is accepted under a new profile version, `Execute` refuses to launch with
`EXECUTION_CONTAINMENT` when the host sandbox is unavailable. The gate asserts, as a dependency check over `go list -deps
./cmd/corvint`, that neither package is reachable from the product binary. A refuse-to-launch check
in front of confinement that does not exist is rejected as theatre; reusing the mutate sandbox is a
redesign tracked in the ideas backlog and blocked on that containment profile.

## Consequences

Wiring any product command to the provider makes the sandbox a blocker, not backlog.

## Rollback

Drop `GLTP-V0-044`, the pointer sentence and the dependency test.
