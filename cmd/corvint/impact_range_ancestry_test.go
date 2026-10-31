package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gitancestry/ancestrytest"
)

// EEP-V0-770: `impact --base` and `prove --base` refuse a grafted checkout, whose info/grafts the
// range engine's merge-base honours, naming why; a shallow checkout keeps the engine's exact check.
func TestImpactRangeRefusesGraftedAncestry_EEPV0770(t *testing.T) {
	root := t.TempDir()
	reuseGit(t, root, "init", "-q")
	reuseGit(t, root, "config", "user.email", "corvint@example.test")
	reuseGit(t, root, "config", "user.name", "Corvint Test")
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/rangeancestry\n\ngo 1.22\n")
	write("a.go", "package rangeancestry\n\nconst A = 1\n")
	reuseGit(t, root, "add", ".")
	reuseGit(t, root, "commit", "-qm", "base")
	base := strings.TrimSpace(reuseGit(t, root, "rev-parse", "HEAD"))
	write("a.go", "package rangeancestry\n\nconst A = 2\n")
	reuseGit(t, root, "commit", "-qam", "change")
	if code, _, stderr := runCLI(t, "--root", root, "impact", "--base", base); code != 0 {
		t.Fatalf("plain checkout: exit %d %s", code, stderr)
	}
	undo := ancestrytest.Graft(t, root, "HEAD")
	for _, command := range []string{"impact", "prove"} {
		code, _, stderr := runCLI(t, "--root", root, command, "--base", base)
		if code != 2 || !strings.Contains(stderr, `"code": "unsupported-impact-range"`) || !strings.Contains(stderr, "--base ancestry cannot be trusted: checkout rewrites ancestry with info/grafts") {
			t.Errorf("grafted %s: exit %d %s", command, code, stderr)
		}
	}
	undo()
	ancestrytest.Shallow(t, root, "HEAD~1")
	if code, _, stderr := runCLI(t, "--root", root, "impact", "--base", base); code != 0 || strings.Contains(stderr, "ancestry cannot be trusted") {
		t.Errorf("shallow checkout with --base inside the boundary: exit %d %s", code, stderr)
	}
}
