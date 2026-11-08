<!-- SPDX-License-Identifier: Apache-2.0 -->
# rspt9 merge-forward composition

Merged mainline `9761ea1f459c0d3b3c963953e24a76eaeda16370` into lane head
`903d26243eb543f22683c53b3a71759b8a8f7444` with a two-parent merge.

Resolved `docs/specs/core-compatibility-freeze-v1.md` by retaining both sides' enum
contracts and selecting the line citations that match the merged sources: affected
advice/provider lines 228/251 and prove state line 1791. The automatic code merge
retains runner-selection shadow advice alongside mainline changes.

Restored `docs/specs/REQUIREMENTS.tsv` and `docs/specs/INDEX.json` from the merged
mainline before committing the merge, then regenerated both with their owning
`make spec-requirements` target. Both paths appear in the merge-resolution worklist.
The merged `docs/specs/README.md` preserves both sets of entries. No requirement IDs
needed renumbering. No Go files required conflict resolution or regeneration.

The whole lane is rebound against the merged mainline, retaining reasoned unknowns
and any NOT_PRODUCED reasons. This round ends after dogfood-check; no seal is requested.
Focused validation is limited to spec-requirements-check and line-citations-check.
The host verifier owns the broader checks; their outcome is NOT_OBSERVED here.
