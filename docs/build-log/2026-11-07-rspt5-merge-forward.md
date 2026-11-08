# rspt5 merge-forward to 9761ea1f

Merged mainline `9761ea1f459c0d3b3c963953e24a76eaeda16370` into lane head
`47dc31c5a014246d6cb5c930a88bccd4c6a1415f` with a two-parent merge,
`64aa32590866c62e3ea97d3b6078ef4984576867`. The whole-range rebind uses that
merged mainline as BASE, following docs/DOGFOOD.md's merge-forward contract.

The only textual conflict was `docs/specs/core-compatibility-freeze-v1.md`:
both sides retained identical state enumerations, but moved their citations.
The resolution retains lane affected.go lines 228/251 and mainline prove.go
line 1791, matching the combined source. No behavior or enumeration was dropped.
`docs/specs/REQUIREMENTS.tsv` and `docs/specs/INDEX.json` were restored from
mainline before the merge commit and regenerated afterward by
`make spec-requirements`, which owns both registries and the README table.
`docs/specs/README.md` retained both sides' entries through the automatic merge.
These are also the three paths reported by dogfood-merge-base --paths; the
two registry differences reflect the deliberate mainline-first regeneration.

No requirement-ID collisions were found between the lane's 59 new definitions
and mainline's 14 new definitions; no IDs were renumbered. The automatic code
merge retains runner-selection shadow advice and mainline import-set comparison
for snapshot affected plans. No Go file was conflict-resolved or regenerated,
so the requested merge-resolution package-test set is empty.

Verification: Go toolchain go1.27.3, affected at the merged mainline BASE,
make spec-requirements, and make -s spec-requirements-check line-citations-check
all returned 0. The affected receipt remains scope UNKNOWN with 95 unknowns;
its full selection was not executed. The citation checker retains its note
about 180 citations whose basename is not tracked at repository root. Linux
execution gives no Darwin evidence. Raw receipts are retained under
/home/russell/lanes/rspt5-evidence/merge-*.log.

The lane was unsealed at round start. This round stops at a passing committed
CEM check and does not seal. Broad gates, guarding-test execution, live
qualification, independent review and native Tasks completion remain
NOT_RUN/NOT_PRODUCED under the owner's bounded merge charter; the host verifier
owns subsequent validation. No push or remote action is authorized this round.
Existing reasoned unknowns must remain visible in the rebind; same-change RSP
intent is not historical authority at this BASE.

Friction: Git printed a packed-refs.lock read-only-filesystem maintenance error
after the merge commit, but committed successfully using the permitted metadata
paths. No metadata permissions were widened. The convenience lane-finish helper
also hardcodes email and seals by default, so the explicit unsealed bind route
is required by this round's identity and stop rules.

Rollback is a new revert of this integration and registry regeneration; no
pre-round commit is rewritten.
