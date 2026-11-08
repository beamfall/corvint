# avdrv mainline merge-forward

Owner scope: retain the host's merge, rebind against the merged mainline, run only
the specified focused checks, and stop after dogfood-check. No seal, publication,
native Tasks writes, or broader qualification is authorized for this round.

The lane head before this round was
`77819bf629fea9b6f96668a4b56ee4428e95f317`, based on
`205d188261990927b4118f73fdd1c232ccc0e2c7`. The host merged mainline
`9761ea1f459c0d3b3c963953e24a76eaeda16370` in merge commit
`f142dc46197b9bc8f1b663ce4284181be57df926`, then regenerated the requirement
index in `78c84892144c60271da028a95ff137c7c8ad652a`. This merged mainline is
the unchanged BASE for the new binding, following DOGFOOD's merge-forward route.

Conflict inventory: `script/dogfood-merge-base.sh --paths` and the merge's
`git show --remerge-diff` both name only `docs/specs/REQUIREMENTS.tsv`.
The host's merge retained the prior AFP-V0-171 summary temporarily; its follow-up
regeneration restored mainline's waiver-amended summary and retained the lane's
AFP-V0-9400, AFP-V0-9401 and AFP-V0-9402 rows. The initial
`make -s spec-requirements-check` passed. No requirement IDs were renumbered.
No Go file was conflict-resolved or regenerated, so no package test is required
by this round's conflict-based selection. Mainline's CI test changes arrived
unchanged from its parent and are outside that selection.

The original lane's driver requirements, regressions, failed evaluations and
limits remain in the preceding avdrv build-log entries. The merge does not
promote delivery maturity or supply independent interoperability evidence.
Fresh binding evidence and the final command results are retained by the dogfood
report and the external merge handback. Independent review and broader host
qualification remain NOT_OBSERVED in this round; the host owns those checks.
Pre-change receipts absent from the binding remain NOT_PRODUCED with their
original reasons, never reconstructed after the implementation.

Rollback is a new revert of this round's binding and merge changes, preserving
the original commits and their evidence. No stored-data migration is involved.
