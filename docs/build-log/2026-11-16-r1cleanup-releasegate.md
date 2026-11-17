# Retire the unused release gate (V1-4805)

Owner charter: r1cleanup, base df315e35056d15c7933b5ff97c605cedf5ef4919.
`gh pr view 116 --repo beamfall/corvint --json state,mergedAt,mergeCommit`
reported MERGED at 2026-11-06T18:24:44Z, merge
13342d85f6245f897c58023891824212617fe0a7. The base has 19 releasegate files,
4,250 lines, and no Go importer. This supersedes the earlier 17-file inventory.

Decision: delete the package and rehome `TestTarArchiveRejectsTraversalAndSymlink`
to the live updater's tar extractor. All three cases (traversal, escaping symlink,
and both together) must refuse, leave the outside path absent, and never materialize
the link. The SOP-V0-007 hostile matrix now calls `internal/update`. The source-removal
verdict changes from keep to remove; obsolete ratchet/read-scope entries and live
spec witnesses are removed or explicitly marked retired. No product contract or
error text changes. Rollback: revert this retirement and restore the original matrix row.

Evidence: the rehomed test and its three subtests PASS in the unfiltered updater suite;
`script/check-hostile-regressions_test.sh` PASS. Changed-package Go vet PASS.
The whole updater suite FAILS only `TestUPDV0008CoreSwitchRefreshesIndexedCheckouts`:
the sandbox exposes a read-only empty `/home/russell/lanes/.git`, so scratch directories
under the required TMPDIR are mistaken for a checkout. No assertion was weakened.
`test-update.log` is 8,311 bytes, SHA-256
625020654d2451f1fef93d3f8413cd02a70b350faad571a1066c15b35bdc9117,
retained outside the repository at `/home/russell/lanes/r1cleanup-evidence/`.

Overall acceptance remains PARTIAL for external closeout: the host must run the
repository-wide build/vet gate and the updater suite outside this sandbox, and the
separate authorized GitHub change must remove the package-cost entry and walltime
fixture fragment in `.github/cishards/package-costs.json` and
`.github/cishards/walltime_test.go`. Those files are forbidden to this lane.
The full affected plan is not narrowed coverage; its retained UNKNOWNs and the
owner's focused-test limitation remain in the handback. Independent review is owned
by the orchestrator. No publication, Tasks mutation, CEM or seal was performed.
