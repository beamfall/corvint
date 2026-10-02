//go:build !darwin

package main

// Outside Darwin no process-group retirement proof is qualified, so the
// canonical engine refuses with runtime/unsupported-process-containment before
// admission or any spawn (G04/G05).

func stableOwnerSupported() bool       { return false }
func stableKeeperControlPresent() bool { return false }
func stableKeeperMain()                {}
func stableClose(s *stableSession)     {}
func stableRun(s *stableSession, t *stableTx) *cemError {
	return operational("unsupported-process-containment")
}
func stableAdmit(repo string, h *stableHooks) (*stableAdmission, *cemError) {
	return nil, operational("unsupported-process-containment")
}
func stableRevalidate(a *stableAdmission) *cemError {
	return operational("unsupported-process-containment")
}
