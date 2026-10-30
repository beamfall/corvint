//go:build !unix

package main

import "testing"

// assertFinishSignalCLI needs POSIX SIGINT/SIGTERM delivery; the in-process
// cancellation in TestLocalCompletionRealEvidenceWorkflow still pins LCP-V0-043.
func assertFinishSignalCLI(t *testing.T, _, _, _ string) {
	t.Helper()
	t.Log("NOT_RUN: LCP-V0-060 real-signal delivery is POSIX-only")
}
