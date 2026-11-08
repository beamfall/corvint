# Runner selection: disappearing ancestor listings

The owner's self-review repair request identifies a soundness gap in `RSP-V0-012` (f):
deleting or renaming away the last tracked descendant of a directory changes the listing of
an ancestor even though Git emits only a file deletion. The spec's immediate-parent check
could exclude a test that observes that listing. `internal/linecov/git.go` uses `--no-renames`
and reports a rename as deletion plus addition; it does not emit synthetic directory deletions.

Decision: select on every ancestor listing of the base-side removed path, through the repository
root. This conservative rule covers arbitrary disappearing directory depth without adding tree
inputs, changing diff shapes or attempting to prove which directories survive. A surviving sibling
may cause overselection; exact minimality is already a non-goal. Apply the source rule even when
the rename destination has no matching dependency.

The closed allowlist for certifying a listing non-match is fully decoded, normalized
repository-relative component paths with known repository identity and a complete ancestor chain
to that root. Anything outside it selects the test. Unknown path forms are not silently treated
as unrelated paths. This closes the class of ancestor-listing omissions rather than special-casing
one depth or filename.

`RSP-V0-012` now requires this behavior. Its acceptance fixtures include the reported last-file
deletion, rename-away in both representations, deeper ancestry, surviving siblings, an unrelated
known path and an opaque unknown form. `RSP-V0-034` / `RSP-V0-036` require qualification of the
changed selector before narrowing. These are planned fixtures: the RSP selector remains
not-started, and this change claims no executable selector implementation or qualification.

Scope is the requested contract repair; runtime code, wire shapes, errors, goldens and existing
test assertions are unchanged. Rollback is the existing FULL / qualification invalidation route,
never restoration of the unsound exclusion rule. The Windows tracing study's absent fallback
and unobserved Windows qualification remain unchanged.

Validation: `corvint affected --base 8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734` selected
`internal/specindex`; the range retains five `LANGUAGE_FRONTIER` unknowns and is not a claim of
complete coverage. `make -s spec-requirements-check line-citations-check` passed, including the
whole `internal/specindex` package and the citation target's focused `internal/lrfrepo` check,
both through `script/heavy.sh`. The toolchain was `go1.27.3`. Staged citation repinning found no
moved citations; the existing checker skipped 180 untracked-basename citations. No runtime Go
package changed. Independent review and execution of the future selector fixtures remain
NOT_RUN here; the orchestrator owns review and the selector has not been implemented.
