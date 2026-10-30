package companionrelease

import "testing"

// PUB-V0-013 (V1-2690): new VSIX builds require the one declared Node/npm
// pair, and retained rc.3 manifests stay readable on their exact pair only.
func TestV12690VSIXNodeNPMPairs(t *testing.T) {
	if requiredNodeVersion != "v24.11.1" || requiredNPMVersion != "11.6.2" {
		t.Fatalf("declared VSIX tuple = %s/%s", requiredNodeVersion, requiredNPMVersion)
	}
	for _, pair := range [][2]string{{"v24.11.1", "11.6.2"}, {"v22.23.2", "10.9.8"}} {
		if !retainedVSIXNodeNPMAdmitted(pair[0], pair[1]) {
			t.Fatalf("pair %v refused", pair)
		}
	}
	for _, pair := range [][2]string{{"v24.11.1", "10.9.8"}, {"v22.23.2", "11.6.2"}, {"v24.22.1", "11.19.1"}, {"", ""}} {
		if retainedVSIXNodeNPMAdmitted(pair[0], pair[1]) {
			t.Fatalf("pair %v admitted", pair)
		}
	}
}
