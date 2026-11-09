package cli

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCAXV0032PairedFlagsAreWhole checks CAX-V0-032 and CAX-V0-033 at the
// command line: the paired cite flags and the paired status flags are each
// accepted only as a complete set, --output is refused beside
// --paired-output because a paired citation never writes the map, and status
// --paired refuses every map-reading option it would otherwise ignore.
func TestCAXV0032PairedFlagsAreWhole(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	cite := []string{"cite", "--map", "change.cem.json", "--hunk", "1", "--evidence-path", "a_test.go", "--lines", "1:2", "--relation", "test-claim"}
	cases := map[string]struct {
		arguments []string
		message   string
	}{
		"cite without --paired-root":    {append(cite, "--paired-output", "p.json", "--paired-range", "t=a..b"), "argument --paired-output: requires --paired-root"},
		"cite with only --paired-range": {append(cite, "--paired-range", "t=a..b"), "argument --paired-range: requires --paired-output and --paired-root"},
		"cite with --output": {append(cite, "--paired-output", "p.json", "--paired-range", "t=a..b", "--paired-root", root, "--output", "x.json"),
			"argument --output: not allowed with --paired-output"},
		"status without --paired-root": {[]string{"status", "--map", "change.cem.json", "--paired", "p.json"}, "argument --paired: requires --paired-root"},
		"status with --citation": {[]string{"status", "--map", "change.cem.json", "--paired", "p.json", "--paired-root", root, "--citation"},
			"argument --citation: not allowed with --paired"},
		"status with --max-unknown": {[]string{"status", "--map", "change.cem.json", "--paired", "p.json", "--paired-root", root, "--max-unknown", "0"},
			"argument --max-unknown: not allowed with --paired"},
	}
	for name, test := range cases {
		exit, _, refusal := runCoreRead(t, root, test.arguments...)
		if exit == 0 || !strings.Contains(refusalText(refusal), test.message) {
			t.Errorf("%s: exit %d refusal %v, want %q", name, exit, refusal, test.message)
		}
	}
}

func refusalText(refusal map[string]any) string {
	var text strings.Builder
	for _, value := range refusal {
		if s, ok := value.(string); ok {
			text.WriteString(s + "\n")
		}
	}
	return text.String()
}
