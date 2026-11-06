<!-- SPDX-License-Identifier: Apache-2.0 -->
# Ratchetsel mainline merge-forward, 2026-11-06

The owner requested merging mainline `9208cc7e25bb82503ef9d61b0ce6a5f02039e649`
into lane head `bef43249fe7a478dd43b71f05e4d5bd9debec543`, preserving both
sides and rebinding the lane against that mainline. Merge commit:
`16f0c00846431491500f8441826f2f4efb20e7b4`.

The two conflicted paths were `docs/specs/README.md` and
`docs/specs/falsifiable-packet-v0.md`. The catalog retains mainline's
AFP-V0-2600 range and runner-locality description together with the lane's
AZU-V0-9902 range. The packet spec retains the NormalizePaths construction
citation; the unchanged content anchor moves to merged lines 1427–1440.
No requirement IDs were renumbered: mainline does not define AZU-V0-9900,
AZU-V0-9901 or AZU-V0-9902.

The generated REQUIREMENTS.tsv and INDEX.json were taken from mainline for
the merge, then regenerated through `make spec-requirements` (which owns
both outputs and the README projection). Regeneration restored the three
lane requirement rows; INDEX.json required no further change. Automatic
Go merges retain the lane's ownership selection inputs and fixture
classification alongside mainline's cached path tokens and ancestor-bound
runner locality. No Go file needed manual conflict resolution or regeneration.

Verification: `make -s spec-requirements-check line-citations-check` exited 0.
The generator's obligation-bound check also exited 0. Per the owner's narrow
round scope, no additional package test, doc-gates, corvint test,
gate-affected, whole selection or repository-wide gate was run. The host
verifier and batch CI provide the subsequent integration checks. This round
stops after a passing dogfood-check and does not seal, publish or change Tasks.
