package companionrelease

import "testing"

// CTF-V0-008: the release archive classifier keys on exact binary names, so the folded
// `corvint-test` companion and the names that share its prefix each classify as themselves.
func TestCTFV0008ExactCompanionNames(t *testing.T) {
	for _, inv := range []bundleInventory{{}, {current: true}} {
		for _, name := range []string{"corvint-test", "corvint-test-runner", "corvint-test-validity-mcp"} {
			if got := inv.name(name); got != name {
				t.Fatalf("current=%v: %s classifies as %s", inv.current, name, got)
			}
			if got := inv.binary(name); got != "bin/"+name {
				t.Fatalf("current=%v: %s binary path %s", inv.current, name, got)
			}
		}
	}
}
