# Selmin mainline merge-forward

Date: 2026-11-07
Mainline base: c3b29cbb54d31c8526d3cce4ad74356b6fb9b943
Lane input: 0f32ab76702f7d0e8a93eac0364710118b42eaac
Merge: 9ce61db7e641ab8dd49971c8b990d0d429501432
Regeneration: 563cdb00d8901f907b5e43b264f0e4be55efb4db

The host merged the mainline before this round. Rebind the complete delivered
lane range against that merged mainline commit, following DWC-V0-7601 and
DWC-V0-7602. Preserve the existing lane commits and their historical evidence.
The lane was unsealed and this round stops after a passing dogfood-check.

Resolution inventory from `script/dogfood-merge-base.sh --paths`:

- `docs/specs/README.md`: retain both sides' spec entries; regeneration updates
  the lane's AFP requirement range through AFP-V0-9011.
- `docs/specs/REQUIREMENTS.tsv`: regenerate from the merged spec sources,
  retaining the mainline requirements and AFP-V0-9009 through AFP-V0-9011.

The overlapping core-compatibility and falsifiable-packet spec edits merged
without a textual conflict, retaining both sides' citation adjustments. No Go
file required conflict resolution or regeneration. No requirement ID was
renumbered. The initial `make -s spec-requirements-check` passed. The final
verification is limited to that check and line-citations-check; the host owns
the subsequent broader verification. No whole selection or exhaustive gate is
run in this round.

The first uncited dogfood-change prepared the replacement map and returned
incomplete: citation-plan-not-provided, missing-intent-scope, not-ready and
outcome-input-not-provided. Supply the existing governing intent scopes and a
reviewed refreshed citation plan for the final binding. Retain every explicit
NOT_PRODUCED reason in the dogfood report, including guarding-tests-not-run;
prechange query and impact receipts remain NOT_OBSERVED at the new base.
This merge-forward does not repeat or broaden the lane's earlier qualification
or complete any task-store record. Rollback is a new revert of this round's
commits; no pre-existing commit is rewritten.
