# Decision 0528 — AFP-V0-023 and AFP-V0-050 amended: the selector narrows declaration edits

Date: 2026-10-15. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-15
("approve V1-1399 narrowing and the V1-1400 wording").

## Context

`AFP-V0-050` (V1-1298, accepted under the owner's standing auto-accept of 2026-10-11) narrows an
edit of `.corvint/test-read-scopes.json` in `corvint affected` against the base declaration, but
its non-goals kept the `AFP-V0-012` selector (`internal/gateselect`, run by
`tools/gate-affected-select` and by `corvint test`) on the `AFP-V0-023` rule, and `AFP-V0-023`
scoped the narrowing to "the planner". The selector therefore still ran every declared package
for an edit that only adds one key (V1-1399). Separately, `AFP-V0-050` (3) described the base's
packaged directories as "outside testdata, directly holding a regular `.go` file", while the
code, after V1-1400, also requires module ownership, a regular `go.mod` and the observer's walk.
The standing auto-accept covers new lane requirements, not amendments of accepted text.

## Decision

The owner accepts two amendments in `docs/specs/affected-plan-v0.md`:

1. V1-1399: `AFP-V0-023` now says an edit of the declaration is narrowed by `AFP-V0-050` for the
   planner and for the selector, and `AFP-V0-050` gains clause (4): the selector narrows the
   same way, against the plan's `range.base`, else its `revision`, read from the base commit's
   Git objects with the same existence and ownership checks, and falls back to the `AFP-V0-023`
   rule on any unknown, including a `--snapshot` plan. The non-goal that kept the selector and
   `corvint test` on the `AFP-V0-023` rule is removed; the `--snapshot` form stays a non-goal.
2. V1-1400 (b): the `AFP-V0-050` (3) parenthetical is replaced by the full condition. A declared
   directory holds a Go package at the base only when the nearest `go.work`-listed module (the
   root module alone without `go.work`) owns it, not a nested, unlisted or testdata `go.mod`;
   that module's manifest is a regular `go.mod`; the observer's walk reaches it (no hidden or
   `affected.SkippedDirectories` component unless the Go plugin admits it); it lies outside its
   owner's testdata; and it directly holds a regular `.go` file.

## Limits

The selector reads the base with hermetic `git ls-tree` and `git cat-file` and does not re-hash
objects as the planner's verified reader does, because `internal/gateselect` must stay
standard-library only for `tools/gate-ledger`. A removed key is selected only while its directory
is still a package in the observed tree. `make gate` is `NOT_RUN`.

## Rollback

Revert this decision, restore the previous `AFP-V0-023` clause, `AFP-V0-050` (3) parenthetical
and non-goal, remove `AFP-V0-050` (4), and remove the `baseReadScopes` call in
`internal/gateselect` `selectPackages`; then regenerate `docs/specs/REQUIREMENTS.tsv`.
