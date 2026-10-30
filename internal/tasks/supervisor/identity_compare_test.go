package supervisor

import "testing"

// TestPSRV0227_CompareIdentity (V1-1091): equal start ticks with btime at
// most one second apart are the same process, another start tick or a
// non-linux mismatch is different, and a wider btime gap is unknown.
func TestPSRV0227_CompareIdentity(t *testing.T) {
	for _, tc := range []struct {
		fresh, recorded string
		want            IdentityMatch
	}{
		{"linux:1000:77", "linux:1000:77", IdentitySame},
		{"linux:1001:77", "linux:1000:77", IdentitySame},
		{"linux:999:77", "linux:1000:77", IdentitySame},
		{"linux:1000:78", "linux:1000:77", IdentityDifferent},
		{"linux:1001:78", "linux:1000:77", IdentityDifferent},
		{"linux:1002:77", "linux:1000:77", IdentityUnknown},
		{"linux:998:77", "linux:1000:77", IdentityUnknown},
		{"darwin:5:6", "darwin:5:6", IdentitySame},
		{"darwin:5:7", "darwin:5:6", IdentityDifferent},
		{"darwin:6:6", "linux:6:6", IdentityDifferent},
		{"", "", IdentityDifferent},
		{"", "linux:1000:77", IdentityDifferent},
		{"linux:1000:77", "", IdentityDifferent},
		{"linux:x:77", "linux:1000:77", IdentityDifferent},
		{"linux:1001:", "linux:1000:", IdentityDifferent},
	} {
		if got := CompareIdentity(tc.fresh, tc.recorded); got != tc.want {
			t.Errorf("CompareIdentity(%q, %q) = %d, want %d", tc.fresh, tc.recorded, got, tc.want)
		}
		if got := SameProcess(tc.fresh, tc.recorded); got != (tc.want == IdentitySame) {
			t.Errorf("SameProcess(%q, %q) = %v", tc.fresh, tc.recorded, got)
		}
	}
}
