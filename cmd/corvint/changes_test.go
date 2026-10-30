package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	t.Run("UCL1142-V0-008 range over the bound names limit and commits", func(t *testing.T) {
		root, e := filepath.EvalSymlinks(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		var stream strings.Builder
		for i := 1; i <= 514; i++ {
			fmt.Fprintf(&stream, "commit refs/heads/main\nmark :%d\ncommitter Fixture <fixture@example.invalid> 1767225600 +0000\ndata 6\nplain\n", i)
			if i > 1 {
				fmt.Fprintf(&stream, "from :%d\n", i-1)
			}
			stream.WriteString("\n")
		}
		for _, step := range []struct {
			args  []string
			stdin string
		}{{[]string{"init", "-q", "-b", "main"}, ""}, {[]string{"fast-import", "--quiet"}, stream.String()}} {
			c := exec.Command("git", append([]string{"-C", root}, step.args...)...)
			c.Stdin = strings.NewReader(step.stdin)
			if b, e := c.CombinedOutput(); e != nil {
				t.Fatalf("git %v: %v %s", step.args, e, b)
			}
		}
		revs, e := exec.Command("git", "-C", root, "rev-parse", "main~513", "main").Output()
		if e != nil {
			t.Fatal(e)
		}
		ends := strings.Fields(string(revs))
		var out, err bytes.Buffer
		code := run([]string{"--root", root, "changes", "--from", ends[0], "--to", ends[1]}, strings.NewReader(""), &out, &err)
		var refusal map[string]any
		if code != 2 || out.Len() != 0 || json.Unmarshal(err.Bytes(), &refusal) != nil {
			t.Fatalf("refusal: %d %s %s", code, &out, &err)
		}
		if refusal["code"] != "changes-range-too-large" || refusal["ok"] != false || refusal["limit"] != float64(512) || refusal["commits"] != float64(513) {
			t.Fatalf("refusal fields: %v", refusal)
		}
	})
}

func TestSPCV0008ChangesSharedCLI(t *testing.T) {
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %v %s", args, e, b)
		}
		return strings.TrimSpace(string(b))
	}
	write := func(name, body string) {
		t.Helper()
		if e := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); e != nil {
			t.Fatal(e)
		}
	}
	git("init", "-q", "-b", "work")
	write(".corvint/shared-paths.json", `{"profile":"corvint-shared-paths/0","shared":["internal/testutil/**"],"generated":["docs/specs/REQUIREMENTS.tsv"],"marker":"shared-path:"}`)
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	base := git("rev-parse", "HEAD")
	write("internal/testutil/a.go", "a\n")
	write("cmd/b.go", "b\n")
	git("add", "-A")
	git("commit", "-q", "-m", "feat: mixed")
	head := git("rev-parse", "HEAD")
	call := func(args ...string) (int, map[string]any, string) {
		t.Helper()
		var out, err bytes.Buffer
		code := run(append([]string{"--root", root, "changes"}, args...), strings.NewReader(""), &out, &err)
		var v map[string]any
		_ = json.Unmarshal(out.Bytes(), &v)
		return code, v, err.String()
	}
	if code, v, _ := call("check-shared", "--base", base); code != 1 || v["profile"] != "corvint-shared-path-check/0" || v["ok"] != false {
		t.Fatalf("check-shared: %d %v", code, v)
	}
	if code, v, _ := call("split-shared", "--base", base, "--branch", "work"); code != 0 || v["applied"] != false || v["oldHead"] != head || git("rev-parse", "work") != head {
		t.Fatalf("split plan: %d %v", code, v)
	}
	for _, args := range [][]string{
		{"split-shared", "--base", base, "--branch", "work", "--apply"},
		{"split-shared", "--base", base, "--branch", "work", "--expect-head", head},
		{"check-shared"},
		{"check-shared", "--base", base, "--branch", "work"},
	} {
		if code, _, stderr := call(args...); code != 2 || !strings.Contains(stderr, "changes-invalid-arguments") {
			t.Fatalf("%v: %d %s", args, code, stderr)
		}
	}
	if code, _, stderr := call("check-shared", "--base", base, "--config", "absent.json"); code != 2 || !strings.Contains(stderr, "changes-shared-config-unavailable") {
		t.Fatalf("absent config: %d %s", code, stderr)
	}
	if code, _, stderr := call("split-shared", "--base", base, "--branch", "work", "--apply", "--expect-head", base); code != 2 || !strings.Contains(stderr, "changes-shared-head-mismatch") || git("rev-parse", "work") != head {
		t.Fatalf("head mismatch: %d %s", code, stderr)
	}
	code, v, _ := call("split-shared", "--base", base, "--branch", "work", "--apply", "--expect-head", head)
	if code != 0 || v["applied"] != true || v["oldHead"] != head || v["newHead"] != git("rev-parse", "work") || v["pushed"] != false {
		t.Fatalf("split apply: %d %v", code, v)
	}
	if git("rev-parse", "work^{tree}") != git("rev-parse", head+"^{tree}") {
		t.Fatal("final tree changed")
	}
}
