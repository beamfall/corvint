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
