# Avgo merge-forward evidence binding

Owner intent: retain the avgo changes while merging mainline
`9761ea1f459c0d3b3c963953e24a76eaeda16370`, then rebind and check without sealing.
The host committed merge `419f12e4a05853367e07cece2707811ab8a8e666` and regenerated
the requirements index in `fcca3a40f2cd85d67de40ded62cde88e06e491ad`.

Conflict resolution: `docs/specs/REQUIREMENTS.tsv` retained the lane rows and was
regenerated to restore mainline's AFP-V0-171 summary. Both sides' requirements
remain present. No requirement IDs were renumbered, and no Go file required a
conflict resolution or regeneration. `make -s spec-requirements-check` passed.

The binding uses the merged mainline as BASE, as required by the merge-forward
contract in `docs/DOGFOOD.md`. Prior citations are refreshed against that base;
unsupported evidence remains explicitly unknown. The initial collection reported
`coordination-time-impact NOT_PRODUCED unsupported-impact-range`, guarding tests
`NOT_PRODUCED guarding-tests-not-run`, and prechange query/impact
`NOT_OBSERVED agent-receipt-absent`. These remain limits, not inferred successes.

Only the requested spec requirements and line citation checks run in this round;
there are no conflict-resolved Go packages to test. Broad selection, doc gates,
qualification, publication and Tasks writes remain with the host. The external
`/home/agent/lanes/avgo.merge.handback.md` records final binding/check results.
