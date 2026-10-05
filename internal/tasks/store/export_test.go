package store

import "github.com/Beamfall/corvint/internal/tasks/transaction"

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

// SetRunFaultForTest fails a supervised run at a named point, until the
// returned restore is called.
func SetRunFaultForTest(f func(point string) error) func() {
	old := runFault
	runFault = f
	return func() { runFault = old }
}
