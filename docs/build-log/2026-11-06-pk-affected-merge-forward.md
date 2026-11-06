# pk-affected mainline merge-forward

Owner intent: merge mainline `9208cc7e25bb82503ef9d61b0ce6a5f02039e649`
into lane head `d33d685fd8e9fc5dbd50a0a1cec233ee9e28a38c`, preserving
both behaviours and rebinding the complete lane against the merged mainline.
The previous base was `9db7a94169821d9b28701eaa88866aee2fec1083`.
Existing commits remain intact; rollback is a new revert of this round.

Conflict resolutions:

- `cmd/corvint/transport_recovery_test.go` and
  `cmd/corvint/work_adopt_test.go`: use mainline's `workBuildEnvironment`.
  It preserves the lane's CORVINT_GOCACHE, GOCACHE, temporary-cache precedence
  and local toolchain. The lane's executable-cache regression remains intact.
- `internal/liveverify/affected/graph.go` and
  `internal/liveverify/affected/readers.go`: use mainline's immutable `tokens`
  field, computed before frontier analysis, with the uncached fallback for
  internally assembled graphs. Both implementations' cache reuse is preserved.
  The lane's concurrent selection parity test now clears `tokens` to exercise
  the fallback; mainline's allocation/reuse test remains intact.
- `docs/specs/falsifiable-packet-v0.md`: retain the NormalizePaths construction
  citation, mechanically relocated to the merged source with its unchanged hash.

No new lane requirement ID collides with mainline. The spec prose and README
entries compose. Generated REQUIREMENTS.tsv and INDEX.json were reset to the
mainline versions during the merge; `make spec-requirements` regenerates all
three spec projections, restoring the ten lane requirement rows. This Makefile
has no separate spec-index generator target.

Qualification is restricted by owner instruction to the two packages containing
resolved Go files and `spec-requirements-check line-citations-check`.
Whole-selection, repository-wide and doc-gates runs, independent review,
publication, Tasks writes and sealing are NOT_RUN in this round. The host
verifier and batch CI own the broader qualification. Historical lane evidence
keeps its original scope and base; it is not fresh merge-composition evidence.
The round's exact command return codes and retained NOT_PRODUCED reasons are in
`/home/russell/lanes/pk-affected.merge.handback.md` and its external scratch logs.
