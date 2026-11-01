package postmergeworkflow

import (
	"strings"
	"testing"
)

// PMR-V0-960 (V1-3662): EncodeID maps a branch name into the identifier
// grammar, DecodeID reverses it, and both refuse what the encoding cannot
// carry.
func TestPMRV0960EncodeIDRoundTrip(t *testing.T) {
	for value, want := range map[string]string{
		"claude/lane-fric-a-1101": "claude:2Flane-fric-a-1101",
		"main":                    "main",
		"a:b/c d":                 "a:3Ab:2Fc:20d",
		"v1.2_x-y":                "v1.2_x-y",
		"feat/ü":                  "feat:2F:C3:BC",
	} {
		got, err := EncodeID(value)
		if err != nil || got != want || !identifier.MatchString(got) {
			t.Errorf("EncodeID(%q) = %q, %v; want %q in the grammar", value, got, err, want)
			continue
		}
		back, err := DecodeID(got)
		if err != nil || back != value {
			t.Errorf("DecodeID(%q) = %q, %v; want %q", got, back, err, value)
		}
	}
	for _, value := range []string{"", "/lead", "-lead", strings.Repeat("a/", 50)} {
		if got, err := EncodeID(value); err == nil {
			t.Errorf("EncodeID(%q) = %q, want a refusal", value, got)
		}
	}
	for _, id := range []string{"a:2f", "a:2", "a:41", "a/b", ":2Fa", "a:XYZ"} {
		if got, err := DecodeID(id); err == nil {
			t.Errorf("DecodeID(%q) = %q, want a refusal", id, got)
		}
	}
}
