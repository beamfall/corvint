<!-- SPDX-License-Identifier: Apache-2.0 -->
# Slowperf mainline merge-forward

The owner requested merging mainline `cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419`
into lane head `c7b67ba83e329a8783bd60cd8aa249ad74640244`, preserving both
parents and rebinding the lane against that mainline commit. Merge commit:
`184306e70af0983352621cdb7309da6f162177c2`.

Conflict dispositions:

- `docs/specs/README.md`: retain the lane verified-reads qualification summary
  and range through DCR-V1-9502, alongside the mainline additions.
- `docs/specs/documentation-corpus-verified-reads-v1.md`: retain the complete
  DCR-V1-9502 streamed-decoder fixture contract in its own subsection and the
  mainline DCR-V1-8000 checkout-stamp and DCR-V1-8001 canonical-comparison contracts.
- `docs/specs/REQUIREMENTS.tsv`: take the mainline bytes in the merge, then
  regenerate from all staged specs with `make spec-requirements`, which owns
  the table, INDEX.json and README projection together.

No Go file conflicted or was regenerated. No lane requirement ID collided
with the mainline: GL-V0-9500, AMAP-V0-9501 and DCR-V1-9502 remain unchanged.
The merge introduces no product behavior beyond composing the two parents.

Verification scope is the owner's requested focused merge-forward scope:
`make -s doc-gates`, and package tests only for conflict-resolved or regenerated
Go files (none). The spec regeneration target also runs its built-in focused
OCM obligation-bound check. Whole-selection tests and repository-wide gates
are NOT_RUN; the host verifier and batch CI own those checks. No new Linux,
Mac, CI-shard or quiet-host performance qualification is claimed. The original
2026-11-05 slowperf build log and external raw evidence remain intact; its
open qualification limits and NOT_PRODUCED reasons still apply, so the local
outcome stays blocked for those external qualifications.

The supplied head is a seal commit even though the round described the lane
as unsealed. Preserve that historical seal and its CEM unchanged. Per the
explicit round instruction, stop after the new-base dogfood-check passes;
create no new seal, publish nothing and perform no Tasks writes. The round
handback outside Git retains exact command return codes and new binding SHA.
Rollback is a new revert of this round, never rewriting either parent's commits.
