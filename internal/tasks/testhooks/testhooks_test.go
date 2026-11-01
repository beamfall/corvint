package testhooks

import (
	"reflect"
	"testing"
)

// TestASDV0956ExportTestSettersAreHookUsers: a setter declared only in
// export_test.go makes its external-package callers hook users, directly and
// through a helper (V1-2426); a keyed seam reached by a method call and a
// shadowing local are not hook writes.
func TestASDV0956ExportTestSettersAreHookUsers(t *testing.T) {
	t.Parallel()
	report, err := Analyze("testdata/exported", "example.test/p")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"count", "publishFault"}; !reflect.DeepEqual(report.Hooks, want) {
		t.Errorf("hooks %v, want %v", report.Hooks, want)
	}
	if want := []string{"SetPublishFaultForTest", "bump", "install"}; !reflect.DeepEqual(report.Setters, want) {
		t.Errorf("setters %v, want %v", report.Setters, want)
	}
	want := []User{
		{Test: "TestDirectWrite", File: "internal_test.go", Via: "publishFault"},
		{Test: "TestExportedSetterParallel", File: "external_test.go", Parallel: true, Via: "SetPublishFaultForTest"},
		{Test: "TestExportedSetterViaHelper", File: "external_test.go", Via: "install"},
		{Test: "TestIncrementViaHelper", File: "internal_test.go", Parallel: true, Via: "bump"},
	}
	if !reflect.DeepEqual(report.Users, want) {
		t.Errorf("users\n%+v\nwant\n%+v", report.Users, want)
	}
	if got := report.Violations(); len(got) != 2 || got[0].Test != "TestExportedSetterParallel" || got[1].Test != "TestIncrementViaHelper" {
		t.Errorf("violations %+v", got)
	}
}

// TestASDV0956RefusesMixedPackages: two non-test package names in one directory are an error.
func TestASDV0956RefusesMixedPackages(t *testing.T) {
	t.Parallel()
	if _, err := Analyze("testdata/mixed", "example.test/m"); err == nil {
		t.Fatal("mixed packages analysed without error")
	}
}
