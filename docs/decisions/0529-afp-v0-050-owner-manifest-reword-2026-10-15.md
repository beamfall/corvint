# Decision 0529 — AFP-V0-050 (3) reworded: the manifest check covers unlisted `go.mod` files only

Date: 2026-10-15. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-15
("approve V1-1425 option (a)").

## Context

Decision 0528 amended `AFP-V0-050` (3) to say that a declared directory holds a Go package at the
base only when its owning module's "manifest is a regular `go.mod` file". Review of V1-1399
(V1-1425) found that neither reader checks the manifest of a `go.work`-listed owner: the planner's
`committedOwner`/`observeModules` and `internal/gateselect` `baseOwner` accept a listed owner
(or the root) as it stands, and the observer keeps a listed module whose `go.mod` is missing and
raises `FrontierModulePath`. Only a `go.mod` on the path that is not listed is inspected: a
regular file or a symlink ends ownership (a regular one below a listed module's testdata is
fixture data), and a directory named `go.mod` is no manifest. The wording said more than the
code does. Option (a) rewords the text to the code; option (b) would have added the check to both
readers and was not chosen.

## Decision

The owner accepts option (a). `AFP-V0-050` (3) no longer says the owner's manifest is a regular
`go.mod`. It says an unlisted `go.mod` on the path counts when it is a regular file or a symlink
(a directory named `go.mod` is no manifest), and that the listed owner's own `go.mod` is not
checked, the observer raising `FrontierModulePath` for a missing one. No code, test or other
requirement text changes.

## Limits

A base whose listed owner has a missing, symlinked or directory `go.mod` is still read by both
readers as owning its packages. `make gate` is `NOT_RUN`.

## Rollback

Revert this decision and the `AFP-V0-050` (3) rewording and status lines, restoring the 0528
clause, then regenerate `docs/specs/REQUIREMENTS.tsv`.
