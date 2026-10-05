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

// SetPoolPreparedFaultForTest makes PoolCommand fail with f's error right after
// its preparation or cleanup receipt commits, until the returned function
// restores the previous hook.
func SetPoolPreparedFaultForTest(f func() error) func() {
	prev := poolPreparedFault
	poolPreparedFault = f
	return func() { poolPreparedFault = prev }
}
