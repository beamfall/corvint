# Decision 0362 — A console pane walks a sealed change from hunk to recorded verification by identifier only

Date: 2026-09-23. Status: accepted scope (experimental delivery; ticket V1-0025). Amends
`docs/specs/local-admin-console-v0.md` with LAC-V0-033 to LAC-V0-036. It changes no wire under
`protocol/**`, adds no root `corvint` verb, and changes no launch behaviour.

## Context

A reviewer of a sealed change has to open four artifacts by hand to see why a hunk exists: the
sealed CEM (`.corvint/changes/<bind>.cem.json`), the OCM maps the dogfood loop leaves untracked under
`.corvint/`, the governing spec at its pinned blob, and the local trace
`.context-corvint/traces/<rev>.jsonl` that `corvint dogfood-record` appends. Each already names the
next by identifier or digest. The console (decision 0081) is the optional loopback surface where
those joins can be shown, but a join rendered on a page is easily read as a claim the artifacts
never made: that evidence supports a change, or that a requirement's tests passed.

## Decision

1. One `/chain` pane reads only artifacts the CLI already writes, per request, and keeps nothing:
   no provenance store, database, cache, write or outbound connection. Frontier artifacts are not
   read; none exist per change.
2. An edge exists only where an artifact field names its target by identifier or digest, and the
   row names that artifact and field. An OCM map joins the chain only when its `targetRevision` is
   the change and its `cem.mapSha256` is the SHA-256 of the sealed map. Pinned spans are re-read at
   their `blobOid` and their bytes rehashed. The sealed name is confirmed against its bind commit.
3. Nothing is inferred from position, text, shared path or proximity. An edge the artifacts do not
   establish is a gap row of one of five classes (`missing`, `stale`, `ambiguous`, `unverified`,
   `unsupported`) with its reason. An ambiguous edge shows every candidate and chooses none.
4. A linked edge is structural. The trace outcome is the operator's record for the whole revision,
   not a test result and not a per-requirement result. The OCM and trace files carry no owning
   verifier, so their edges state no axes (LAC-V0-007) and each edge carries the weakest axes of
   what it joins.

## Consequences

- In a fresh clone every requirement edge is a gap: OCM maps and traces are untracked and local to
  the worktree that sealed the change. The pane shows that rather than reconstructing links.
- The dogfood loop's OCM obligations are `unknown` until `corvint ocm link` records identifiers, so
  their requirement edges render `unsupported`. Making them linked is a producer change, not a
  console change.
- The operator-time comparison against the CLI (U4) is `NOT_OBSERVED` for this pane.

## Rollback

Delete `internal/console/chain.go` and `internal/console/chain_test.go`, the `/chain` route,
`handleChain` and the `Changes`/`Chain` view fields in `internal/console/server.go`, `chainView` in
`internal/console/views.go`, and the navigation link in `internal/console/render.go`; remove
LAC-V0-033 to LAC-V0-036. The existing panes are untouched and no artifact depends on this one.
