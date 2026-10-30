package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ANL-V0-015: end to end over the committed fixture; an unmapped template forces the full suite.
func TestANLV0015ImpactCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatal(diag)
	}
	mapFile := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(mapFile, []byte(out), 0o600); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		b, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=F", "-c", "user.email=f@example.invalid"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(args, err, string(b))
		}
		return strings.TrimSpace(string(b))
	}
	base := git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "club.html"), []byte("<p></p>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-qm", "template")
	code, out, diag = runFlowsCLI(root, "appmap", "impact", "--map", mapFile, "--base", base)
	if code != 0 || !strings.Contains(out, "application-map-impact/0") || !strings.Contains(out, "full-suite-required") {
		t.Fatal(code, out, diag)
	}
	if code, _, _ = runFlowsCLI(root, "appmap", "impact", "--map", mapFile); code == 0 {
		t.Fatal("missing --base accepted")
	}
}
