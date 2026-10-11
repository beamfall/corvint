package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestChangesCLI(t *testing.T) {
	t.Run("UCL1142-V0-005 help and dispatch", func(t *testing.T) {
		for _, args := range [][]string{{"--help"}, {"changes", "--help"}, {"help", "changes"}} {
			var out, err bytes.Buffer
			if code := run(args, strings.NewReader(""), &out, &err); code != 0 || !strings.Contains(out.String(), "changes") {
				t.Fatalf("%v: %d %s %s", args, code, &out, &err)
			}
		}
		root := t.TempDir()
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
		for _, args := range [][]string{{"changes"}, {"changes", "--from", sha, "--from", sha, "--to", sha}, {"changes", "--unknown", "secret=do-not-echo"}, {"--root", root, "changes", "--from", strings.Repeat("a", 40), "--to", sha}} {
			out.Reset()
			err.Reset()
			if code := run(args, strings.NewReader(""), &out, &err); code != 2 || out.Len() != 0 || !strings.Contains(err.String(), "changes-") || strings.Contains(err.String(), "do-not-echo") {
				t.Fatalf("refusal: %v %d %s %s", args, code, &out, &err)
			}
		}
	})
}
