# Bugfix verifier repair 1

Base: `8e2c13aa926fcaf1731fae71b27c41c41c8f09af`.
Repair input: `5b0cb389fa8e2246034d7f3d7753b776f53af42c` and the host
verifier's failing checks. This round is restricted to those integration failures.

The CIN-V0-2304 batch mirror now requires both the designated step name and
its exact body. A changed designated body also receives ordinary command
classification, preserving the existing ASD-V0-036 refusal alongside the
ASD-V0-2472 pinned-body diagnostic. The existing renamed-step and changed-body
assertions remain intact. No workflow, golden or assertion is changed.

The joined Pi/operator spec table row is split, restoring the SRG-V0-001
index entry. Pi's existing package version command synchronizes its version
pins to 0.3.14 after the merged shipped-content changes (AHI-020).
Unchanged moved citations are repinned with the existing citation tool.

For LPCV-V0-7603, the deadline fixture uses execfile.WriteFile to prevent
ETXTBSY, as required by ASD-V0-929. LPCV-V0-7601 now names the existing
covered-child scratch audit marker without making it a completeness claim.
TSP-V0-005 records the watch help's heavy.sh example as a mention. AFP-V0-023
declares tools/linecov's checkout reads: its own directory only; collection,
Git and module mutations in its tests use temporary fixture repositories.
This avoids adding a permanent unbounded-reader exemption.

Used: the original pre-change context/query/impact capture, and affected
against the specified base before testing. The initial installed binary reports
itself stale and selects 190 units with scope UNKNOWN (109 unknowns: 91
LANGUAGE_FRONTIER, seven NO_SELECTABLE_TEST, eleven UNOWNED_DIRTY_PATH).
That plan is selection evidence, not completeness. A fresh tree plan is retained
with the final binding. Frozen ranking evaluations and optional provider/live
qualification are not applicable to these verifier repairs. Independent review
belongs to the orchestrator; this round runs no reviewer process.

Evidence is outside the repository in `/home/russell/lanes/bugfix-evidence/`,
under `repair1-*`; the handback records final commands and results. The existing
pre-change capture is reused, never regenerated. Final binding/checking follows
the post-commit sequence and stops without a seal or publication. Outstanding
original ticket qualifications are not promoted by these integration repairs.

Rollback reverts this repair commit and rebinds the resulting diff; the listed
verifier failures would return. No frozen contracts or existing assertions are
changed to obtain passing output.
