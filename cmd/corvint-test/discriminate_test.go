package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/extmutate"
)

// CCE-V0-176: `corvint-test e2e discriminate` parses JSON argv, runs the discriminator and prints
// its document; malformed input refuses with a named code before anything runs.
func TestCCEV0176DiscriminateCommand(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	root, app := t.TempDir(), filepath.Join(t.TempDir(), "app")
	git := func(dir string, args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "limit.js"), []byte("module.exports = 1;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(root, "add", "-A")
	git(root, "commit", "-q", "-m", "base")
	base := git(root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "limit.js"), []byte("module.exports = (n) => n <= 10;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(root, "commit", "-q", "-am", "head")
	head := git(root, "rev-parse", "HEAD")
	git(root, "worktree", "add", "-q", "--detach", app, head)
	var stdout bytes.Buffer
	err := run(context.Background(), []string{"e2e", "discriminate", "--root", root, "--base", base, "--head", head, "--app-dir", app,
		"--test-command", `["grep","-q","n <= 10","limit.js"]`, "--max-mutants", "5"}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	var doc extmutate.Document
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Schema != extmutate.Schema || len(doc.Records) != 1 || doc.Records[0].State != extmutate.Killed || doc.Records[0].Path != "limit.js" || doc.AppRevision != head {
		t.Fatalf("document %s", stdout.Bytes())
	}
	for _, args := range [][]string{
		{"--test-command", `"grep"`},
		{"--test-command", `["true"]`, "--max-mutants", "0"},
		{"--test-command", `["true"]`, "--wall-time", "-1s"},
		{"--test-command", `["true"]`, "--dir-match", "prefix"},
		{"--test-command", `["true"]`, "--keep-dir", "app"},
		{"--test-command", `["true"]`, "--map", filepath.Join(root, "missing.json")},
	} {
		err := run(context.Background(), append([]string{"e2e", "discriminate", "--root", root, "--base", base, "--head", head, "--app-dir", app}, args...), &bytes.Buffer{})
		var refusal *extmutate.Refusal
		if !errors.As(err, &refusal) {
			t.Fatalf("%v: %v", args, err)
		}
	}
	// CCE-V0-237: the exclusion flags reach the recorded rule.
	stdout.Reset()
	if err := run(context.Background(), []string{"e2e", "discriminate", "--root", root, "--base", base, "--head", head, "--app-dir", app,
		"--test-command", `["true"]`, "--dir-match", "leading", "--keep-dir", "e2e", "--exclude-dir", "lib"}, &stdout); err != nil {
		t.Fatal(err)
	}
	doc = extmutate.Document{}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil || doc.Exclusion.DirectoryMatch != "leading" || strings.Contains(strings.Join(doc.Exclusion.Directories, ","), "e2e") || !strings.Contains(strings.Join(doc.Exclusion.Directories, ","), "lib") {
		t.Fatalf("exclusion flags: %v %+v", err, doc.Exclusion)
	}
	var help bytes.Buffer
	if err := run(context.Background(), []string{"e2e", "discriminate", "--help"}, &help); err != nil || !strings.Contains(help.String(), "Example:") || !strings.Contains(help.String(), "Required: --base REV") {
		t.Fatalf("help: %v %s", err, help.String())
	}
}
