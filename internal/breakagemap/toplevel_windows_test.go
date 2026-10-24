package breakagemap

import "testing"

// TestSameTopLevelAcceptsGitForWindowsOutput pins the literal form Git for
// Windows prints for `rev-parse --show-toplevel` (V1-2488).
func TestSameTopLevelAcceptsGitForWindowsOutput(t *testing.T) {
	if !sameTopLevel("C:/cv/checkout\n", `C:\cv\checkout`) {
		t.Fatal("C:/cv/checkout did not match C:\\cv\\checkout")
	}
}
