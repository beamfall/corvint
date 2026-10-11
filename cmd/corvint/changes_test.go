package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestChangesCLI(t *testing.T) {
	t.Run("UCL1142-V0-005 UCL1142-V0-006 help and dispatch", func(t *testing.T) {
		for _, args := range [][]string{{"--help"}, {"changes", "--help"}, {"help", "changes"}} {
			var out, err bytes.Buffer
			if code := run(args, strings.NewReader(""), &out, &err); code != 0 || !strings.Contains(out.String(), "changes") {
				t.Fatalf("%v: %d %s %s", args, code, &out, &err)
			}
		}
		// The issue-state reader refuses symlinked path components, so the
		// fixture root is canonical (macOS temporary paths cross /var).
		root, e := filepath.EvalSymlinks(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		git := func(args ...string) string {
			t.Helper()
			c := exec.Command("git", append([]string{"-C", root}, args...)...)
			c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
			b, e := c.CombinedOutput()
			if e != nil {
				t.Fatalf("git: %v %s", e, b)
			}
			return strings.TrimSpace(string(b))
		}
		git("init", "-q")
		git("commit", "--allow-empty", "-m", "base")
		sha := git("rev-parse", "HEAD")
		var out, err bytes.Buffer
		code := run([]string{"--root", root, "changes", "--from", sha, "--to", sha}, strings.NewReader(""), &out, &err)
		var report map[string]any
		if code != 0 || json.Unmarshal(out.Bytes(), &report) != nil || report["profile"] != "corvint-upgrade-checklist/0" {
			t.Fatalf("dispatch: %d %s %s", code, &out, &err)
		}
		if e := os.WriteFile(root+"/issues.json", []byte(`[{"number":1,"state":"OPEN"}]`), 0o600); e != nil {
			t.Fatal(e)
		}
		out.Reset()
		err.Reset()
		code = run([]string{"--root", root, "changes", "--from", sha, "--to", sha, "--issue-state-file", "issues.json"}, strings.NewReader(""), &out, &err)
		if code != 0 || !strings.Contains(out.String(), "operator-supplied file, not verified by Corvint") {
			t.Fatalf("issue state: %d %s %s", code, &out, &err)
		}
		out.Reset()
		err.Reset()
		if code := run([]string{"--root", root, "changes", "--from", sha, "--to", sha, "--issue-state-file", "absent.json"}, strings.NewReader(""), &out, &err); code != 2 || !strings.Contains(err.String(), "changes-issue-state-unavailable") {
			t.Fatalf("issue state refusal: %d %s %s", code, &out, &err)
		}
		for _, args := range [][]string{{"changes"}, {"changes", "--from", sha, "--from", sha, "--to", sha}, {"changes", "--issue-state-file", "a", "--issue-state-file", "b", "--from", sha, "--to", sha}, {"changes", "--unknown", "secret=do-not-echo"}, {"--root", root, "changes", "--from", strings.Repeat("a", 40), "--to", sha}} {
			out.Reset()
			err.Reset()
			if code := run(args, strings.NewReader(""), &out, &err); code != 2 || out.Len() != 0 || !strings.Contains(err.String(), "changes-") || strings.Contains(err.String(), "do-not-echo") {
				t.Fatalf("refusal: %v %d %s %s", args, code, &out, &err)
			}
		}
	})
}
