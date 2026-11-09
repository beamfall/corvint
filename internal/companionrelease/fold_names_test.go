package companionrelease

import "testing"

// CTF-V0-008: the release archive classifier keys on exact binary names, so companions whose
// names share the `corvint-test` prefix each classify as themselves.
func TestCTFV0008ExactCompanionNames(t *testing.T) {
	t.Parallel()
	for _, inv := range []bundleInventory{{}, {current: true}} {
		for _, name := range []string{"corvint-test-runner", "corvint-test-validity-mcp"} {
			if got := inv.name(name); got != name {
				t.Fatalf("current=%v: %s classifies as %s", inv.current, name, got)
			}
			if got := inv.binary(name); got != "bin/"+name {
				t.Fatalf("current=%v: %s binary path %s", inv.current, name, got)
			}
		}
	}
}
