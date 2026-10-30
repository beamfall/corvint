package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestImpactBaseRefusesDeletionAndUnsafePathPerPath pins ERI-V0-305 (V1-2919,
// GitHub #1925): a deleted file or an unsafe path no longer refuses the whole
// `impact --base` range; each is listed under refusedPaths and the rest is
// computed.
func TestImpactBaseRefusesDeletionAndUnsafePathPerPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = root
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@example.com")
	run("config", "user.name", "t")
	write("go.mod", "module example.com/m\n\ngo 1.22\n")
	write("pkg/a.go", "package pkg\n\nfunc A() int { return 1 }\n")
	write("pkg/gone.go", "package pkg\n\nfunc Gone() int { return 2 }\n")
	commitAll(t, root, "base")
	base := run("rev-parse", "HEAD")
	write("pkg/a.go", "package pkg\n\nfunc A() int { return 3 }\n")
	write("notes/sp ace.txt", "unsafe\n")
	if err := os.Remove(filepath.Join(root, "pkg/gone.go")); err != nil {
		t.Fatal(err)
	}
	commitAll(t, root, "change")

	code, stdout, stderr := runCLI(t, "--root", root, "impact", "--base", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	receipt := decodeReceipt(t, stdout)["context"].(map[string]any)
	rules := map[string]string{}
	refused, _ := receipt["refusedPaths"].([]any)
	for _, row := range refused {
		member := row.(map[string]any)
		rules[member["path"].(string)] = member["rule"].(string)
	}
	if len(rules) != 2 || rules["pkg/gone.go"] != "DELETED" || rules["notes/sp ace.txt"] != "UNSAFE_PATH" {
		t.Fatalf("refusedPaths = %v in %s", rules, stdout)
	}
	if !strings.Contains(stdout, "pkg/a.go") {
		t.Fatalf("admitted path pkg/a.go is not analysed: %s", stdout)
	}
}
