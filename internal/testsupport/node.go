package testsupport

import (
	"os"
	"os/exec"
	"testing"
)

// NodeOrSkip returns the node executable on PATH. When node is absent it skips
// t with a NOT_RUN reason naming the missing tool, so a lane host without node
// (Unraid) reports the test as not run instead of failed; under CI (CI set, as
// GitHub Actions sets it), where node is provisioned, it fails instead, so a
// missing toolchain there cannot pass silently (ASD-V0-973, V1-3801).
func NodeOrSkip(t testing.TB, what string) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err == nil {
		return node
	}
	if os.Getenv("CI") != "" {
		t.Fatalf("%s needs node, which CI provisions, but it is not on PATH: %v", what, err)
	}
	t.Skipf("NOT_RUN: %s needs node, which is not on PATH (install Node.js to run it)", what)
	return ""
}
