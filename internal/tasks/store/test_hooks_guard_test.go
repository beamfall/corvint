package store_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/testhooks"
)

// TestASDV0956NoParallelTestWritesAPackageHook: no test of this package that
// writes a package-level test hook, directly or through a setter such as
// SetPublishFaultForTest in export_test.go, calls t.Parallel; the perf-1022s
// lane saw injected publication faults leak into parallel peers (V1-2426).
func TestASDV0956NoParallelTestWritesAPackageHook(t *testing.T) {
	t.Parallel()
	report, err := testhooks.Analyze(".", "github.com/Beamfall/corvint/internal/tasks/store")
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, user := range report.Users {
		seen = seen || user.Via == "SetPublishFaultForTest"
	}
	if !seen {
		t.Fatalf("no hook user reached SetPublishFaultForTest; the analysis lost export_test.go setters: %+v", report.Users)
	}
	for _, user := range report.Violations() {
		t.Errorf("%s (%s) calls t.Parallel but writes a package hook via %s: drop t.Parallel or key the seam by store", user.Test, user.File, user.Via)
	}
}
