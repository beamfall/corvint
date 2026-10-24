package sharedpath

import "testing"

// TestSingleAlternateAdmitsWindowsVolumes pins the form Git for Windows reports
// for `rev-parse --path-format=absolute --git-path objects`: the drive colon is
// not list syntax there, while ';' still is (V1-2488).
func TestSingleAlternateAdmitsWindowsVolumes(t *testing.T) {
	for _, store := range []string{`C:/cv/.git/objects`, `C:\cv\.git\objects`, `\\host\share\cv\.git\objects`} {
		if !singleAlternate(store) {
			t.Errorf("Windows object store %q refused", store)
		}
	}
	if singleAlternate(`C:/cv/.git/objects;D:/other`) {
		t.Error("two alternates admitted")
	}
}
