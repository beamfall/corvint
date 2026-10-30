package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestSPCV0012ChangesBehindCLI(t *testing.T) {
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
			"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %v %s", args, e, b)
		}
		return strings.TrimSpace(string(b))
	}
	commit := func(msg, name, body string) string {
		t.Helper()
		if e := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); e != nil {
			t.Fatal(e)
		}
		git("add", "-A")
		git("commit", "-q", "-m", msg)
		return git("rev-parse", "HEAD")
	}
	git("init", "-q", "-b", "main")
	commit("base", ".corvint/shared-paths.json", `{"profile":"corvint-shared-paths/0","shared":["internal/testutil/**"],"generated":["docs/specs/REQUIREMENTS.tsv"],"marker":"shared-path:"}`)
	git("checkout", "-q", "-b", "feature")
	head := commit("feat: f", "cmd/f.go", "f\n")
	git("checkout", "-q", "main")
	shared := commit("shared-path: helper", "internal/testutil/a.go", "a\n")
	call := func(args ...string) (int, string, string) {
		t.Helper()
		var out, err bytes.Buffer
		code := run(append([]string{"--root", root, "changes", "behind"}, args...), strings.NewReader(""), &out, &err)
		return code, out.String(), err.String()
	}
	code, stdout, _ := call("--base", "main", "--head", "feature")
	var report map[string]any
	if json.Unmarshal([]byte(stdout), &report) != nil || code != 0 || report["profile"] != "corvint-shared-path-behind/0" || report["mutates"] != false ||
		report["head"] != head || !strings.HasPrefix(report["configDigest"].(string), "sha256:") || report["dryMerge"].(map[string]any)["result"] != "clean" {
		t.Fatalf("json: %d %s", code, stdout)
	}
	if behind := report["behind"].([]any); len(behind) != 1 || behind[0].(map[string]any)["commit"] != shared {
		t.Fatalf("behind: %v", behind)
	}
	code, stdout, _ = call("--base", "main", "--head", "feature", "--ticket", "V1-1710", "--stale-hours", "1", "--format", "doctor-plugin")
	var items []map[string]any
	if json.Unmarshal([]byte(stdout), &items) != nil || code != 0 || len(items) != 1 {
		t.Fatalf("doctor-plugin: %d %s", code, stdout)
	}
	keys := []string{}
	for k := range items[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "detail,kind,remedy,who" || !regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`).MatchString(items[0]["kind"].(string)) ||
		items[0]["kind"] != "SHARED_BASE_BEHIND" || items[0]["who"] != "V1-1710" || !strings.Contains(items[0]["detail"].(string), shared) {
		t.Fatalf("plugin item: %v", items[0])
	}
	if code, stdout, _ := call("--base", "main", "--head", "main", "--ticket", "V1-1710", "--stale-hours", "1", "--format", "doctor-plugin"); code != 0 || strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("empty plugin array: %d %s", code, stdout)
	}
	for _, args := range [][]string{
		{},
		{"--head", "feature"},
		{"--base", "main", "--ticket", "V1-1710"},
		{"--base", "main", "--stale-hours", "4"},
		{"--base", "main", "--ticket", "V1-1710", "--stale-hours", "0"},
		{"--base", "main", "--ticket", "V1-1710", "--stale-hours", "x"},
		{"--base", "main", "--ticket", "bad ticket", "--stale-hours", "4"},
		{"--base", "main", "--format", "doctor-plugin"},
		{"--base", "main", "--format", "yaml"},
		{"--base", "main", "--base", "main"},
		{"--base", "main", "extra"},
	} {
		if code, _, stderr := call(args...); code != 2 || !strings.Contains(stderr, "changes-invalid-arguments") {
			t.Fatalf("%v: %d %s", args, code, stderr)
		}
	}
	if code, _, stderr := call("--base", "main", "--config", "absent.json"); code != 2 || !strings.Contains(stderr, "changes-shared-config-unavailable") {
		t.Fatalf("absent config: %d %s", code, stderr)
	}
	if code, _, stderr := call("--base", "no-such-ref"); code != 2 || !strings.Contains(stderr, "changes-shared-range-unavailable") {
		t.Fatalf("unknown base: %d %s", code, stderr)
	}
}
