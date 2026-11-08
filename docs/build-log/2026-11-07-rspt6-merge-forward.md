<!-- SPDX-License-Identifier: Apache-2.0 -->
# rspt6 merge-forward to 9761ea1f

The owner requested a merge-forward of lane head
`92066b3970d2bcc618df98c9883bc566ce0b8731` from its original base
`8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734` onto mainline
`9761ea1f459c0d3b3c963953e24a76eaeda16370`, preserving both behaviours.

Merge commit: `c57b56af5d6a398b51c7702d6a5ba5752be48803`.

Conflicted path: `docs/specs/core-compatibility-freeze-v1.md`. Both sides retained
identical enum inventories but repinned different source line numbers. Preserve every
inventory entry and use the combined tree's affected.go lines 228 and 251 and
prove.go line 1791. No Go file required conflict resolution. The automatic merge
retains the lane's runner shadow and mainline's snapshot import-set comparison.

Generated `docs/specs/REQUIREMENTS.tsv` and `docs/specs/INDEX.json` were restored
from the merged mainline before committing the merge, then regenerated together
with their owning `make spec-requirements` target. The merged README keeps both
sides' entries. Mainline has no RSP-V0 IDs; no requirement renumbering is needed.

Rebind the whole lane against the merged mainline SHA, as DOGFOOD's conflicted
merge-forward route requires. Preserve the existing citation evidence, refresh
its base spans, and retain all NOT_PRODUCED limitations. The lane was not sealed;
stop after the committed CEM passes dogfood-check. No push or Tasks writes.

Verification scope is the owner's requested spec-requirements and line-citation
checks. There are no conflict-resolved or regenerated Go files, so that criterion
selects no Go package command. Whole-selection and repository gates are NOT_RUN;
the host verifier and batch CI own that coverage. Exact command results and
binding commits are retained in the lane handback outside the repository.
