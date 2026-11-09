package cli

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCEMV1694ReplaceFlagsAreGuarded checks CEM-V1-694 and CEM-V1-695 at the
// command line: cite --replace and --expected-hunk-digest are given together,
// the digest is 64 lowercase hex digits on cite and mark, and a paired
// citation, which never writes the map, refuses both.
func TestCEMV1694ReplaceFlagsAreGuarded(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	digest := strings.Repeat("a", 64)
	cite := []string{"cite", "--map", "change.cem.json", "--hunk", "1", "--evidence-path", "a.txt", "--lines", "1:2", "--relation", "specification"}
	mark := []string{"mark", "--map", "change.cem.json", "--hunk", "1", "--disposition", "unknown", "--reason", "insufficient-evidence"}
	for name, test := range map[string]struct {
		arguments []string
		message   string
	}{
		"replace without digest": {append(cite, "--replace"), "argument --replace: requires --expected-hunk-digest"},
		"digest without replace": {append(cite, "--expected-hunk-digest", digest), "argument --replace: requires --expected-hunk-digest"},
		"uppercase digest":       {append(cite, "--replace", "--expected-hunk-digest", strings.ToUpper(digest)), "argument --expected-hunk-digest: must be 64 lowercase hexadecimal digits"},
		"short mark digest":      {append(mark, "--expected-hunk-digest", "abc"), "argument --expected-hunk-digest: must be 64 lowercase hexadecimal digits"},
		"paired replace": {append(cite, "--paired-output", "p.json", "--paired-range", "t=a..b", "--paired-root", root, "--replace", "--expected-hunk-digest", digest),
			"argument --replace: not allowed with --paired-output"},
	} {
		exit, _, refusal := runCoreRead(t, root, test.arguments...)
		if exit == 0 || !strings.Contains(refusalText(refusal), test.message) {
			t.Errorf("%s: exit %d refusal %v, want %q", name, exit, refusal, test.message)
		}
	}
}
