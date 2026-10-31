package store

import (
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"time"
)

var ReconcileBeforeCommitForTest = reconcile

// CompactLivenessRunForTest is CompactLiveness with its whole-record run bound.
var CompactLivenessRunForTest = compactLiveness

var BarrierWithFaultsForTest = barrier

var PolicyUpdateBeforeCommitForTest = policyUpdate

var ImportBeforeBatchForTest = importWith

// SetPublishFaultForTest installs f before every artifact publication until
// the returned function restores the previous hook.
func SetPublishFaultForTest(f func(transaction.Artifact) error) func() {
	prev := publishFault
	publishFault = f
	return func() { publishFault = prev }
}

// SetLeaseStagesTogetherForTest sets LGL-V0-013's switch until the returned
// function restores it. A test that calls it must not run in parallel.
func SetLeaseStagesTogetherForTest(on bool) func() {
	prev := leaseStagesTogether
	leaseStagesTogether = on
	return func() { leaseStagesTogether = prev }
}

// SetLeasePublishSyncsOverlapForTest switches LGL-V0-016's overlapped post and head
// directory syncs and returns the restore function.
func SetLeasePublishSyncsOverlapForTest(on bool) func() {
	prev := leasePublishSyncsOverlap
	leasePublishSyncsOverlap = on
	return func() { leasePublishSyncsOverlap = prev }
}

// SetRedoBarrierObservedForTest runs f between redo's barrier observation and
// its removal until the returned function restores the previous hook.
func SetRedoBarrierObservedForTest(f func() error) func() {
	prev := redoBarrierObserved
	redoBarrierObserved = f
	return func() { redoBarrierObserved = prev }
}

// SetHealthPrepareHookForTest runs f before each health preparation until the
// returned function restores the previous hook.
func SetHealthPrepareHookForTest(f func(member string)) func() {
	prev := healthPrepareHook
	healthPrepareHook = f
	return func() { healthPrepareHook = prev }
}

// SetPoolPreparedFaultForTest makes PoolCommand fail with f's error right after
// its preparation or cleanup receipt commits, until the returned function
// restores the previous hook.
func SetPoolPreparedFaultForTest(f func() error) func() {
	prev := poolPreparedFault
	poolPreparedFault = f
	return func() { poolPreparedFault = prev }
}

// SetLeaseGuardFaultForTest replaces the result of leaseWrite's unlocked
// pre-round guard read with f(stateDir) when the read succeeded, until the
// returned restore is called. f sees every store's directory, so a test
// returns nil for stores other than its own (CAL-V0-372).
func SetLeaseGuardFaultForTest(f func(stateDir string) error) func() {
	prev := leaseGuardFault
	leaseGuardFault = f
	return func() { leaseGuardFault = prev }
}

// SetGateRecordFaultForTest fails GateRun after its gate program runs and
// before the result is recorded, until the returned restore is called.
func SetGateRecordFaultForTest(f func() error) func() {
	old := gateRecordFault
	gateRecordFault = f
	return func() { gateRecordFault = old }
}

// SetGateObserveFaultForTest runs f just before GateRun's post-run
// observation under the run's own context, until the returned restore is
// called.
func SetGateObserveFaultForTest(f func()) func() {
	old := gateObserveFault
	gateObserveFault = f
	return func() { gateObserveFault = old }
}

// ExecutableSha256ForTest is executableSha256.
var ExecutableSha256ForTest = executableSha256

// SetRunFaultForTest fails a supervised run over the store at stateDir at a
// named point, until the returned restore is called. Runs over other stores
// are unaffected, so tests that set it can run in parallel.
func SetRunFaultForTest(stateDir string, f func(point string) error) func() {
	old, had := runFaults.Swap(stateDir, f)
	return func() {
		if had {
			runFaults.Store(stateDir, old)
		} else {
			runFaults.Delete(stateDir)
		}
	}
}

// SetPoolCommandSecondForTest shortens the unit of a health/cleanup
// timeoutSeconds bound until the returned restore is called.
func SetPoolCommandSecondForTest(d time.Duration) func() {
	prev := poolCommandSecond
	poolCommandSecond = d
	return func() { poolCommandSecond = prev }
}

// SetPoolPinStepForTest installs a hook after each pinned-cwd proof step.
func SetPoolPinStepForTest(hook func(step string) error) func() {
	prev := poolPinStep
	poolPinStep = hook
	return func() { poolPinStep = prev }
}

// PoolBoundedProbeErrorForTest is a bounded Git probe failure of class.
func PoolBoundedProbeErrorForTest(class string) error {
	return sweepGitBounded{class: class, err: errors.New("bounded probe " + class)}
}

// PSRTestLiftObservationBounds lifts the fixed sweep Git and runner-identity
// probe bounds to bound for a sweep, pool preparation or pool cleanup run under
// ctx (V1-1324). Only tests whose subject is not those bounds use it; cleanup
// proof bounds are unchanged.
func PSRTestLiftObservationBounds(ctx context.Context, bound time.Duration) context.Context {
	return context.WithValue(ctx, sweepObservationBoundKey{}, bound)
}

// SetRetireStageForTest runs f after each durable retire or unretire step
// (JRT-V0-009); a non-nil return stops the run there as a crash would. The
// returned function restores the previous hook.
func SetRetireStageForTest(f func(string) error) func() {
	prev := retireStage
	retireStage = f
	return func() { retireStage = prev }
}

// SetSyncArchiveDirForTest replaces the retire archive's directory sync
// (JRT-V0-006). The returned function restores the previous one.
func SetSyncArchiveDirForTest(f func(string) error) func() {
	prev := syncArchiveDir
	syncArchiveDir = f
	return func() { syncArchiveDir = prev }
}

// SetAfterWorktreeLookupForTest runs f between a ticket-worktree ensure's
// branch lookup and its path check until the returned function restores the
// previous hook.
func SetAfterWorktreeLookupForTest(f func()) func() {
	prev := afterWorktreeLookup
	afterWorktreeLookup = f
	return func() { afterWorktreeLookup = prev }
}

// SetProgramWriteHookForTest runs f after a workflow composes a program phase
// record and before its native transition, until the returned function
// restores the previous hook (V1-0831).
func SetProgramWriteHookForTest(f func()) func() {
	prev := programWriteHook
	programWriteHook = f
	return func() { programWriteHook = prev }
}

// PersistForTest records the workflow's program at phase.
func (w *Workflow) PersistForTest(phase string) error { return w.persist(phase) }

// RemoverStoppedForTest is removerStopped (CAL-V0-368).
var RemoverStoppedForTest = removerStopped

// SetStoreClockForTest makes the workflow, supervisor and pool writers of the
// store at stateDir read clock instead of time.Now until restore runs
// (V1-2743). Other stores keep real time, so parallel tests are unaffected.
func SetStoreClockForTest(stateDir string, clock func() time.Time) (restore func()) {
	testClocks.Store(stateDir, clock)
	return func() { testClocks.Delete(stateDir) }
}

// SetStageWallForTest makes every supervised stage over the store at stateDir
// derive its own wall from the context wall returns, until restore runs
// (V1-3046). The program wall is unaffected.
func SetStageWallForTest(stateDir string, wall func(context.Context) (context.Context, func())) (restore func()) {
	stageWalls.Store(stateDir, wall)
	return func() { stageWalls.Delete(stateDir) }
}
