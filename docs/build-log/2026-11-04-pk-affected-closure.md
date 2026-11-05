# pk-affected closure round

Owner intent: repair only the two failures from the host rerun of
`5df7d7a8e0336faf1ba0d09b84913f68ffe560c5`, against base
`6c33df33bc5670131482f5263b410cb980580657`; rebind and check without sealing.

## Decisions and acceptance evidence

1. V1-4439 / TSP-V0-7311, AFP-V0-035 and AFP-V0-640: preserve the intended
   compact/full projection. The snapshot failure is a comparison-helper mismatch,
   not a stale snapshot file: the helper removed viaLength before comparing to
   the full witness that now carries that field. Reconstruct via while retaining
   viaLength, so the comparison checks the complete witness in both forms.
   No selection or product implementation changes. The snapshot test has no
   golden update path. The actual committed-range Core goldens were regenerated
   with CORVINT_UPDATE_GOLDEN=1 through TestCoreVerbsEmitTheFrozenProfiles,
   selecting affected_committed_range and affected_full_committed_range: rc=0,
   no diff from the prior lane head. Against BASE the full golden changes only
   by the intended viaLength:1 field; the compact golden is unchanged.
2. V1-3424 / STR-V0-7313 and STR-V0-055: preserve the bounded descriptor read.
   The host failure is at the readable regular-plan JSON assertion, not at a
   blocked FIFO. Request --full only for that JSON assertion. All nonregular
   input checks and the 30-second deadline remain unchanged. No reader or
   resource bound is relaxed.

The focused closure regressions (TestAffectedSnapshot, TestSTRV0055 and
TestAFPV0035CompactDefaultPlanSummarizesTheFullPlan) pass through script/heavy.sh.
The requested broad cmd/corvint command ran, but fails only at
TestAffectedRejectsNonRepositoryAndExtraArguments: the sandbox's ancestor
/home/russell/lanes is discovered as a Git repository from a nominal nonrepository
temporary fixture. That external fixture condition is retained for host rerun;
repairing a third test is outside this narrow closure round. The two requested
failures do not recur in that run.

The four other focused packages from hostrun.sh pass: internal/liveverify/affected,
internal/sessiontest, internal/dogfoodflow and tools/str-parity. All Go tests use
script/heavy.sh, one package per command, selected by affected against the fixed
base. The plan is UNKNOWN; focused coverage does not imply exhaustive coverage.
make spec-requirements passes, including its focused lrfrepo obligation check;
generated files are unchanged. Spec trace rows name the corrected assertions.

## Limits and rollback

Scratch evidence and exact commands are retained at
/home/russell/lanes/pk-affected-closure-evidence. The installed affected binary
reports self-binary-stale; its selection is retained with that limitation.
Bound prechange query and impact receipts from the existing lane remain visible.
Independent review belongs to the orchestrator. The prior partial qualifications
for V1-3728, V1-4381 and V1-4576 remain partial; no new performance, real-session,
twenty-run, platform or exhaustive-gate qualification is claimed. No Tasks writes,
publication or seal is authorized. Rollback reverts these assertion/trace updates;
it does not remove the existing witness or summary contracts.

Final CEM binding and dogfood-check results belong to the external handback so
the check result can name its final immutable head without another source change.
