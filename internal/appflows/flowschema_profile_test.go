package appflows

import "testing"

// CMP-V1-003: each supported version spells its profile literal, and the literal is the one
// FlowsProfile builds, so the compatibility page's flow-file label is read from the reader's code.
func TestCMPV1003FlowsProfileLiteralPerVersion_V15434(t *testing.T) {
	for _, s := range flowsSchemas {
		if want := FlowsProfile(s.version); s.profile != want {
			t.Errorf("version %d: profile literal %q, want %q", s.version, s.profile, want)
		}
	}
	if flowsSchemas[0].profile != "corvint-flows/0" {
		t.Fatalf("version 0 profile %q", flowsSchemas[0].profile)
	}
}
