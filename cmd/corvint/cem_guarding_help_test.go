package main

import (
	"strings"
	"testing"
)

// TestCEMHelpNamesGuardingTestsBounds pins that cem help documents
// --guarding-tests with its run bounds, its unconfined execution and its
// repository-TMPDIR refusal (V1-1376, TCQ-V0-062, CEM-V1-250).
func TestCEMHelpNamesGuardingTestsBounds(t *testing.T) {
	t.Parallel()
	code, out, _ := runCLI(t, "cem", "--help")
	if code != 0 {
		t.Fatalf("cem --help exited %d", code)
	}
	for _, want := range []string{"--guarding-tests", "300 s per go", "1200 s in total", "UNCONFINED", "a TMPDIR\n                       inside the repository is refused"} {
		if !strings.Contains(out, want) {
			t.Fatalf("cem help omits %q", want)
		}
	}
}
