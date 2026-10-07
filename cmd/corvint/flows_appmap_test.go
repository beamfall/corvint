package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// appmapCLIRepo commits the internal/appmap fixture application into a fresh repository.
func appmapCLIRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join("..", "..", "internal", "appmap", "testdata", "fixture")
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "fixture"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return root
}

// AMAP-V0-009 AMAP-V0-011 AMAP-V0-015: the CLI builds the map to stdout and serves every
// projection within its default cap, without writing to the repository.
func TestAMAPV0FlowsAppmapCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	code, out, diagnostic := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 || !strings.HasPrefix(out, `{"schema":"application-map/0"`) {
		t.Fatalf("build %d %s", code, diagnostic)
	}
	mapFile := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(mapFile, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args []string
		cap  int
	}{
		{[]string{"screen", "--screen", "/#!/clubs/7/teesheets"}, 4096},
		{[]string{"flow", "--flow", "book-tee-time"}, 6144},
		{[]string{"find", "--text", "teesheet"}, 2048},
		{[]string{"scaffold", "--flow", "book-tee-time"}, 6144},
		{[]string{"screen", "--screen", "app.home", "--budget", "2048"}, 2048},
	} {
		code, out, diagnostic := runFlowsCLI(root, append([]string{"appmap", c.args[0], "--map", mapFile}, c.args[1:]...)...)
		var doc map[string]any
		if code != 0 || len(out) > c.cap || json.Unmarshal([]byte(out), &doc) != nil {
			t.Fatalf("%v: %d %d bytes %s", c.args, code, len(out), diagnostic)
		}
		if _, ok := doc["omitted"].(map[string]any); !ok {
			t.Fatalf("%v: no omitted counts", c.args)
		}
	}
	if code, out, _ := runFlowsCLI(root, "appmap", "screen", "--map", mapFile, "--screen", "app.home", "--full"); code != 0 || !strings.Contains(out, `"full":true`) {
		t.Fatalf("--full %d", code)
	}
	for _, args := range [][]string{
		{"appmap"},
		{"appmap", "plan"},
		{"appmap", "build"},
		{"appmap", "screen", "--map", mapFile},
		{"appmap", "screen", "--map", mapFile, "--screen", "app.home", "--budget", "4096", "--full"},
		{"appmap", "screen", "--map", mapFile, "--screen", "app.home", "--budget", "10"},
		{"appmap", "build", "--manifest", "appmap.json", "--revision", "no-such-rev"},
	} {
		if code, out, _ := runFlowsCLI(root, args...); code != 2 || out != "" {
			t.Errorf("%v: %d %q", args, code, out)
		}
	}
	cmd := exec.Command("git", "status", "--porcelain", "--ignored")
	cmd.Dir = root
	if status, err := cmd.Output(); err != nil || len(status) != 0 {
		t.Fatalf("worktree changed: %s %v", status, err)
	}
}
