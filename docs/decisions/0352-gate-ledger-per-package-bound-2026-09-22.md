# Decision 0352 — Resolved test packages key on a proven per-package bound

Date: 2026-09-22. Status: accepted (ticket V1-0037, a GL-V0 follow-up). Adds GL-V0-009 to
`docs/specs/gate-ledger-v0.md` and closes its first Unresolved item; amends GL-V0-001 (the key now
also covers `GOFLAGS`) and GL-V0-004 (resolved packages run under GL-V0-009's key).

## Context

The gate ledger slice (`docs/BUILD-LOG.md`, 2026-09-21 entry) found that Go's test cache
never hits across worktrees, so `ledger/go-test` kept the 93 resolved packages on Go's cache and
recorded only the unresolved set under a whole-tree key. A resolved package therefore reran in
every new worktree, and the ledger's promise "from any worktree" held for every step but the one
that dominates gate time. The affected-plan index (`affected-plan-v0.md` AFP-V0-012) already
knows why a package is resolved: rule (d) found no `runtime.Caller`, `os.Getwd`, root-climbing
literal or git read in it, so every path its tests can open is one a string literal names, which
rule (c) attributes, or one its own source tree holds, which rules (a) and (b) attribute.

## Decision

1. A resolved package's bound is the union of the files `go list -deps -test -json` compiles or
   embeds into its test binary within the module and every worktree path whose change would
   select the package under rules (a) to (c). The second half comes from the same index and
   `namesPath` relation `make gate-affected` uses, through a new `gate-affected-select -bounds`
   mode, so there is one dependency walker; the ledger only unions and digests.
2. The key is GL-V0-001's derivation over the entries in that bound plus the gate tooling, the
   package's import path, and the step name `go-test-package`. The record carries a `bound`
   proof string (file count, paths per rule, entries digested); the format is additive.
3. A package whose bound cannot be proven (no selector attribution, absent from `go list`, or a
   listed file the digest does not hold) runs in the same batch unrecorded; when bounds cannot be
   computed at all, every resolved package runs through Go's test cache unrecorded, which is the
   behaviour before this decision. Nothing narrows what a step tests: the batch runs the same
   `go test` flags `make go-test` uses, and only packages with a recorded pass are omitted.
4. Rule (c) is evaluated once per token across all paths (`pathMatcher`) rather than once per
   dirty path, because `-bounds` attributes every worktree path; the output is the same relation.

## Consequences

A resolved package passes once per distinct content of its bound on this host and hits from any
worktree; the bounds cost about 3.5 s per `ledger/go-test`. Bounds err wide (a broad path token
keys a package on hundreds of paths; a root package encloses every path), never narrow. One
failing package leaves the whole resolved batch unrecorded, because the tool does not parse test
output. `cmd/corvint` stays unresolved until its `os.Getwd` is bounded (V1-0038). Rollback is
`CORVINT_GATE_LEDGER=off` or reverting the ledger tool; records are derived state.
