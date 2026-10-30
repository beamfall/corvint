package versionline

import (
	"strings"
	"testing"
)

// TestPUBV0053ParseIsTolerant: both the current and the earlier line parse, unknown members are
// ignored, and a malformed build or commit, a missing list or a second line is refused.
func TestPUBV0053ParseIsTolerant(t *testing.T) {
	commit := strings.Repeat("b", 40)
	for text, want := range map[string]Line{
		"Corvint 1.0.0-rc.3 (build 12)":                                      {Name: "Corvint", Version: "1.0.0-rc.3", Build: "12"},
		"Corvint 1.0.0-rc.3 (build 0)":                                       {Name: "Corvint", Version: "1.0.0-rc.3", Build: "0"},
		"Corvint 1.0.0-rc.3 (build 12, commit " + commit + ")":               {Name: "Corvint", Version: "1.0.0-rc.3", Build: "12", Commit: commit},
		"Corvint 1.0.0-rc.3.dev (build 12, commit " + commit + ", modified)": {Name: "Corvint", Version: "1.0.0-rc.3.dev", Build: "12", Commit: commit, Modified: true},
		"Corvint 1.0.0-rc.3.dev (commit unknown)":                            {Name: "Corvint", Version: "1.0.0-rc.3.dev", Commit: "unknown"},
		"Corvint 1.0.0 (build 3, signed by nobody)\n":                        {Name: "Corvint", Version: "1.0.0", Build: "3"},
	} {
		got, ok := Parse(text)
		if !ok || got != want {
			t.Fatalf("%q: %+v %v, want %+v", text, got, ok, want)
		}
	}
	for _, text := range []string{
		"Corvint 1.0.0", "Corvint 1.0.0 (build -1)", "Corvint 1.0.0 (build 01)", "Corvint 1.0.0 (build x)",
		"Corvint 1.0.0 (commit abc)", "Corvint 1.0.0 (build 1, build 2)", "Corvint (build 1)",
		"Corvint 1.0.0 extra (build 1)", "Corvint 1.0.0 (build 1)\nCorvint 1.0.0 (build 1)",
	} {
		if line, ok := Parse(text); ok {
			t.Fatalf("%q accepted: %+v", text, line)
		}
	}
}

// TestPUBV0053FormatRoundTrips: Format omits an unstamped build, names an unknown commit, and
// parses back to its members.
func TestPUBV0053FormatRoundTrips(t *testing.T) {
	commit := strings.Repeat("c", 64)
	if got := Format("Corvint", "1.0.0-rc.3", "0", "", false); got != "Corvint 1.0.0-rc.3 (commit unknown)" {
		t.Fatalf("unstamped: %q", got)
	}
	got := Format("Corvint", "1.0.0", "9", commit, true)
	if line, ok := Parse(got); !ok || line.Build != "9" || line.Commit != commit || !line.Modified {
		t.Fatalf("%q: %+v", got, line)
	}
	if !SameRelease("1.0.0-rc.3.dev", "1.0.0-rc.3") || !SameRelease("1.0.0-dev", "1.0.0") || SameRelease("1.0.0-rc.2", "1.0.0-rc.3") {
		t.Fatal("SameRelease")
	}
}
