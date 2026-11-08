# rspt14 merge-forward onto 9761ea1

Merged mainline `9761ea1f459c0d3b3c963953e24a76eaeda16370` into lane
`41818641c2b069cb81316fc3cd439cb107e57653` with merge commit
`9f5a4a0ef`; prior commits remain unchanged.

Conflict: `docs/specs/core-compatibility-freeze-v1.md` differed only in the line
positions of three existing enum citations. Retained all three rows and their
content anchors; affected.go uses the lane positions 228 and 251, and prove.go
uses mainline position 1791, matching the composed code. Git automatically
combined the lane runner shadow and mainline snapshot import-set behaviour in
`cmd/corvint/affected.go`; neither behaviour was removed.

Took mainline REQUIREMENTS.tsv and INDEX.json for the merge, then regenerated
both with `make spec-requirements`, which owns both outputs and the README
projection. The merged README retains both sides. No requirement ID collision
or renumbering was needed. No Go file needed manual conflict resolution or
regeneration, so the requested package-test set is empty. The requested spec
requirements and line citation checks are recorded in the external handback.

Rebinding uses the merged mainline SHA as BASE. This lane was not sealed;
the requested terminal step is a passing dogfood-check, without sealing.
Host verification and batch CI retain responsibility for broader checks and
independent integration review. No new production qualification is claimed.

Friction: merge commit succeeded, but a Git maintenance attempt reported an
unwritable shared packed-refs lock outside the permitted metadata paths.
No permissions or shared Git configuration were changed.
