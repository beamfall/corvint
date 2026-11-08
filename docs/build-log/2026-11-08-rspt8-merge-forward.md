<!-- SPDX-License-Identifier: Apache-2.0 -->
# rspt8 merge-forward to 9761ea1f

Merged mainline `9761ea1f459c0d3b3c963953e24a76eaeda16370` into lane head
`99b8b46733701d0f233210fd8ef4639484ca9af7` with merge commit
`931b3df04b0aa3678b564434ae771a1bbff323fd`; neither parent was rewritten.

Conflict: `docs/specs/core-compatibility-freeze-v1.md`. Kept all three enum rows
and their unchanged values. The merged affected command has the lane's two-line
shift (advice status line 228; provider state line 251); prove has mainline's
three-line shift (state function line 1791). Each citation preserves its content pin.

`cmd/corvint/affected.go` merged automatically and retains both runner-selection
shadow advice and mainline snapshot import-set change detection. No Go file had
an unresolved conflict or was regenerated. No lane requirement ID needed renumbering.

Took mainline's generated REQUIREMENTS.tsv and INDEX.json before committing the
merge, then ran `make spec-requirements` to regenerate both from the combined staged
specs. This target owns the spec index and README projection too; no separate
spec-index target exists. The auto-merged README retains both sides' spec entries.
The generator succeeded, restoring all runner-selection requirement rows.

This round rebinds the whole lane against the merged mainline SHA above. The lane
was not sealed before this round; the requested endpoint is dogfood-check, without
a seal. Earlier failing tests and NOT_PRODUCED reasons in the lane's retained
build logs remain evidence; this merge makes no new qualification claim. Host
verification owns wider selection and integration checks. Local checks are limited
to spec-requirements-check and line-citations-check; no conflict-resolved or
regenerated Go package requires a separate package test in this round.

Observed verification: `make spec-requirements` and
`make -s spec-requirements-check line-citations-check` both returned 0. The latter
ran the target-owned specindex and lrfrepo checks through heavy.sh. No wider test
selection was executed. The toolchain check observed go1.27.3. Citation checking
retains its note that 180 basename citations were skipped.

The first `make dogfood-change` against the new base refused before preparation:
`sealed-cem-in-change`, inherited map
`.corvint/changes/3f32d7ad7ca93f9328cb492d53f3f08dc347b1bb.cem.json`,
`state=unsupported code=unsupported-process-containment` (make rc 2).
The map and all original commits remain unchanged. No deletion, weakening of the
verifier, or substitute qualification is authorized by this merge round.
NOT_PRODUCED: a new bound CEM and complete author/reviewer report; a passing
check depends on a verifier environment that can validate the inherited seal.
The existing shared sidecar is historical, not a binding of this merged head.
Raw round logs and refreshed input drafts are retained outside the repository in
`/home/agent/lanes/rspt8.tmp/`; the final handback records exact check return codes.
