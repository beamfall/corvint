# rspt11rb merge-forward to 9761ea1f

Merged mainline `9761ea1f459c0d3b3c963953e24a76eaeda16370` into lane head
`8b407597804061cad55b084fb6ab70ea0490f0cb` without rewriting existing commits.
The whole-range evidence base is the merged mainline, per DOGFOOD's conflicted
merge-forward contract; this lane was not sealed before this round.

Conflicted path: `docs/specs/core-compatibility-freeze-v1.md`. Both sides describe
identical closed states; retained the lane's shifted affected citations and
mainline's shifted prove citation after inspecting the merged source. The
`cmd/corvint/affected.go` automatic merge retains mainline planning changes and
the lane's declaration-only runner-selection shadow. No Go file required manual
conflict resolution or regeneration, so the requested conflict-package test set
is empty.

Generated paths `docs/specs/REQUIREMENTS.tsv` and `docs/specs/INDEX.json` were
restored from the merged mainline before committing the merge, then regenerated
with `make spec-requirements`, the owning target for both files and the README
table. README prose and entries retain both sides. No requirement ID collision
was observed or renumbering needed.

Qualification is restricted to `make -s spec-requirements-check
line-citations-check` and the whole-range CEM rebind/check. Broader test selection,
doc-gates and repository-wide gates are NOT_RUN by owner instruction; the host
verifier and batch CI own those checks. Historical evidence and explicit unknown
reasons remain retained; this integration does not requalify inherited features.
