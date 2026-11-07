# Selwide merge-forward to c3b29cbb (2026-11-07)

Merge `a5cc8be02a5fa3da1664c09ec012b4d09ec14566` preserves lane head
`f33745b8a1a842b2654a88726a42fde5d987058b` and mainline
`c3b29cbb54d31c8526d3cce4ad74356b6fb9b943` as its two parents.

Conflicted path: `docs/specs/falsifiable-packet-v0.md`. Both sides changed only
one pinned citation to `NormalizePaths`; resolve it to the function's location
in the merged code, preserving the surrounding contract and its content hash.

`internal/liveverify/affected/select.go` merged automatically: retain the lane's
compiled-source claim guard and mainline's reader/selection behavior. No Go file
required hand conflict resolution. The lane's AFP-V0-9012, AFP-V0-9013 and
AFP-V0-9014 are not defined at the merged mainline; no requirement is renumbered.

Generated `docs/specs/REQUIREMENTS.tsv` and `docs/specs/INDEX.json` were taken
from mainline for the merge and regenerated afterward by `make spec-requirements`.
That owning target also projects spec headers into the README table and index;
README prose and both sides' spec entries are preserved.

Rebind the whole lane range against the merged mainline, preserving the original
commits and all evidence. This lane was unsealed before the round, so stop after
the committed CEM passes dogfood-check; do not seal. The operator requests only
focused tests and spec/line-citation checks here. Repository-wide validation,
doc-gates, corvint test and gate-affected are NOT_RUN in this round; the host
verifier and batch CI own the remaining validation. Keep all dogfood
NOT_PRODUCED and NOT_OBSERVED reasons visible in the private reports and handback.
