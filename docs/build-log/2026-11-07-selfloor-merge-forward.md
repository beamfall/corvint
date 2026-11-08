# Selfloor merge-forward binding

The owner requested a merge-forward of lane `claude/selfloor` from
`538e823f4a1d220a3fb5b12c0529248f18924823` to mainline
`c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`, preserving the lane's experimental
driver-certification behavior and existing evidence. The host committed merge
`277b46d1d84be5d19cfad1ee9c2e3ea05d5d9043` and regeneration
`41b34c972476ed95e665a21fee7d5015b502cb21` before this binding round.

Conflict resolution: `docs/specs/REQUIREMENTS.tsv` took mainline's generated
version at the merge and was then regenerated from the combined specs, restoring
the lane's rows. `script/dogfood-merge-base.sh --paths` reports only this path.
Only this generated file and `Makefile` were changed by both sides; `Makefile`
merged automatically. No Go file was conflict-resolved or regenerated, and no
requirement ID was renumbered. The initial `make -s spec-requirements-check`
passed.

The whole lane is rebound against the merged mainline commit, as required by
`docs/DOGFOOD.md`'s merge-forward contract. Earlier review records and raw evidence
remain preserved; this mechanical round makes no new behavior or precision
claim. Pre-change receipts remain historical context from the old base; they
are stale against the new base and are not presented as fresh observations.

The owner limits verification to conflict-resolved or regenerated Go packages
(none here) and `make -s spec-requirements-check line-citations-check`; broader
selection and integration qualification belong to the host verifier and batch
CI. The lane was not sealed, so this round stops after the committed CEM passes
`dogfood-check`, without a seal, publication or Tasks writes. Exact results and
remaining `NOT_PRODUCED` reasons are retained in the external merge handback.
