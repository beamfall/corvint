package store

import (
	"errors"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"time"
)

var ReconcileBeforeCommitForTest = reconcile

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

// SetGateRecordFaultForTest fails GateRun after its gate program runs and
// before the result is recorded, until the returned restore is called.
func SetGateRecordFaultForTest(f func() error) func() {
	old := gateRecordFault
	gateRecordFault = f
	return func() { gateRecordFault = old }
}

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
