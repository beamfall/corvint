# Heavycap merge-forward evidence

The owner requested a merge-forward of lane head
`c74010ae5e800815c26804e9b907d4958c4305a8` onto mainline
`c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`, retaining the lane's existing
commits. The host supplied merge `3f01a1c5a6f90a16d3b9a47b65425a0aee9b156a`
and regenerated the spec index in `438167a81b35b4d39fef05dc1d812194a91ae373`.

`script/dogfood-merge-base.sh --paths` reports these resolved paths:

- `docs/specs/README.md`: preserve mainline entries and the lane's selection,
  replay concurrency and CPU-budget descriptions and requirement ranges.
- `docs/specs/REQUIREMENTS.tsv`: regenerated from the combined specs, retaining
  lane requirements alongside mainline requirements.
- `docs/specs/INDEX.json`: regenerated from the combined specs after the merge.

The initial `make -s spec-requirements-check` passed (exit 0). No Go file was
conflict-resolved or regenerated in this round; the requested package-test
selection is empty. No requirement IDs were renumbered in this round.

Rebinding uses the merged mainline SHA above as BASE, as required by
DWC-V0-7601 and DWC-V0-7602. The lane was not sealed before this round and the
owner explicitly requests stopping after dogfood-check passes. No seal is
produced. Final check results and exact bind/head SHAs are retained in the
external lane merge handback.

Earlier evidence remains historical at its recorded base and target. In
particular, absent external pre-commit integration remains NOT_PRODUCED and
quiet-host wall qualification remains NOT_RUN. This round makes no new claim
for those outcomes, platform qualification, whole-selection tests or host
integration checks. The host verifier and batch CI own the deferred checks.
