package releasecandidate

import (
	"strings"
	"testing"
)

// TestPUBV0053ManifestVersionLineIsTolerant: a manifest records either the earlier
// `(build N)` line or the current one with its commit; the version and build must still match.
func TestPUBV0053ManifestVersionLineIsTolerant(t *testing.T) {
	commit := strings.Repeat("d", 40)
	for line, want := range map[string]bool{
		"Corvint 1.0.0-rc.3 (build 9)":                              true,
		"Corvint 1.0.0-rc.3 (build 9, commit " + commit + ")":       true,
		"Corvint 1.0.0-rc.3 (build 8, commit " + commit + ")":       false,
		"Corvint 1.0.0-rc.3.dev (build 9, commit " + commit + ")":   false,
		"Corvint 1.0.0-rc.3 (commit " + commit + ")":                false,
		"Corvint 1.0.0-rc.3 (build 9, commit " + commit + ")\n":     false,
		"corvint-tasks 1.0.0-rc.3 (build 9, commit " + commit + ")": false,
	} {
		if got := manifestVersionLine(Manifest{Version: "1.0.0-rc.3", BuildNumber: "9", CorvintVersion: line}); got != want {
			t.Fatalf("%q: %v, want %v", line, got, want)
		}
	}
}
