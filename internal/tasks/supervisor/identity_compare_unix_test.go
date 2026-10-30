//go:build darwin || linux

package supervisor

import "testing"

const recordedLinux = "linux:1000:77"

// identityCases are fresh reads for a member recorded as recordedLinux: one
// second of btime jitter, another start tick, and a gap above one second.
var identityCases = []struct {
	name, fresh string
	want        IdentityMatch
}{
	{"jitter", "linux:1001:77", IdentitySame},
	{"start-ticks", "linux:1001:78", IdentityDifferent},
	{"over-one-second", "linux:1002:77", IdentityUnknown},
}

// TestPSRV0227_OwnedGroupAnchorToleratesJitter (V1-1091): the owned group's
// anchor keeps a leader whose fresh identity is one second off the recorded
// one, and drops one with another start tick or a wider gap.
func TestPSRV0227_OwnedGroupAnchorToleratesJitter(t *testing.T) {
	for _, tc := range identityCases {
		t.Run(tc.name, func(t *testing.T) {
			g := &ownedGroup{group: 12, members: map[int]string{12: recordedLinux},
				identity: func(int) (string, error) { return tc.fresh, nil },
				groupOf:  func(int) (int, error) { return 12, nil }}
			pid, id, e := g.anchor()
			if e != nil {
				t.Fatal(e)
			}
			if anchored := pid == 12 && id == recordedLinux; anchored != (tc.want == IdentitySame) {
				t.Fatalf("anchor = %d %q, want anchored %v", pid, id, tc.want == IdentitySame)
			}
		})
	}
}

// TestPSRV0227_EscapeAnchorToleratesJitter (V1-1091): a recorded escape
// group whose leader reads one second off is still anchored, another start
// tick proves the group gone, and a wider gap is uncertain, never gone.
func TestPSRV0227_EscapeAnchorToleratesJitter(t *testing.T) {
	for _, tc := range identityCases {
		t.Run(tc.name, func(t *testing.T) {
			x := &escapes{groups: map[int]string{},
				identity: func(int) (string, error) { return tc.fresh, nil },
				groupOf:  func(int) (int, error) { return 40, nil },
				exists:   func(int) (bool, error) { return true, nil }}
			anchored, gone := x.anchor(40, recordedLinux)
			switch tc.want {
			case IdentitySame:
				if !anchored || gone || x.uncertain {
					t.Fatalf("jitter: anchored=%v gone=%v uncertain=%v", anchored, gone, x.uncertain)
				}
			case IdentityDifferent:
				if anchored || !gone || x.uncertain {
					t.Fatalf("start ticks: anchored=%v gone=%v uncertain=%v", anchored, gone, x.uncertain)
				}
			case IdentityUnknown:
				if anchored || gone || !x.uncertain {
					t.Fatalf("over one second: anchored=%v gone=%v uncertain=%v", anchored, gone, x.uncertain)
				}
			}
		})
	}
}
