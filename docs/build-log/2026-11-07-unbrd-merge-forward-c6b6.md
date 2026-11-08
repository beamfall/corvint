# unbrd merge-forward onto c6b6cbb0a

The host merged mainline `c6b6cbb0a862c36d3e2d3c28b0bd0dc0da800175` into
lane head `4e9df5a98e27c7854acda9a418d5c31359a6feac` as merge commit
`0886300ee44ef999376fc58468872bbed6f379b5`. The previous lane base was
`ae20b398c260933a152e6285de9cfc385a2580c7`.

## Resolution and evidence boundary

- `docs/specs/README.md`: retained the lane's detailed acceptance, capability and
  review references in the shared affected-plan-selection row, alongside every
  other mainline row. This is the sole conflicted path reported by Git's
  remerge diff and merge-tree reconstruction for the two supplied parents.
- `docs/specs/affected-plan-selection-v0.md`: retained both sides' append-only
  spec content; Git reports an automatic merge, with no unresolved Go files.
- The host regenerated the spec index. The requested post-merge
  `make -s spec-requirements-check` passed before rebinding.

No requirement IDs were renumbered. No Go file was conflict-resolved or
regenerated in this round, so the requested conflict-package test set is empty.
The spec checker runs its own focused specindex and lrfrepo tests.

Bind the complete lane range against the merged mainline SHA, retaining prior
sealed maps and honest NOT_PRODUCED reasons. Stop after clean post-commit
dogfood-change and dogfood-check; the owner explicitly says this lane was not
sealed before this round and does not authorize a seal here. Run the requested
spec-requirements and line-citation checks. The host verifier and batch CI own
independent review and broader qualification. No publication or Tasks writes
are part of this round.
