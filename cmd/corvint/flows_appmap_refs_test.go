package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AMAP-V0-090 AMAP-V0-096 AMAP-V0-098: refs writes the bounded name-level report, exits 1 on an
// unlabelled miss and 0 once only labelled misses remain, refuses invalid input with exit 2 and
// changes no repository file.
func TestAMAPV0098RefsCLI(t *testing.T) {
	root := appmapCLIRepoFrom(t, "refs")
	mapFile := func() string {
		code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
		if code != 0 {
			t.Fatalf("build: %s", diag)
		}
		filename := filepath.Join(t.TempDir(), "map.json")
		if err := os.WriteFile(filename, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
		return filename
	}
	filename := mapFile()
	code, out, diag := runFlowsCLI(root, "appmap", "refs", "--map", filename, "--source", "src")
	var doc map[string]any
	if code != 1 || json.Unmarshal([]byte(out), &doc) != nil || doc["schema"] != "application-map-refs/0" || doc["check"] != "name-level" ||
		len(out) > 6144 || !strings.Contains(diag, "3 unlabelled reference misses") {
		t.Fatalf("refs: %d %s %s", code, diag, out)
	}
	for _, args := range [][]string{{"--source", "src"}, {"--map", filename}, {"--map", filename, "--source", "../src"},
		{"--map", filename, "--source", "src", "--budget", "1"}, {"--map", filename, "--source", "src", "--full", "--budget", "4096"},
		{"--map", filename, "--source", "src", "extra"}} {
		code, out, _ = runFlowsCLI(root, append([]string{"appmap", "refs"}, args...)...)
		if code != 2 || out != "" {
			t.Fatalf("admitted %v: %d %s", args, code, out)
		}
	}
	cmd := exec.Command("git", "status", "--porcelain", "--ignored")
	cmd.Dir = root
	if status, err := cmd.Output(); err != nil || len(status) != 0 {
		t.Fatalf("mutated repo: %s %v", status, err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "extra.ts"), []byte("export const n = ['invoices', 'cancel-order', 'secondary'];\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", "extra"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, b)
		}
	}
	code, out, diag = runFlowsCLI(root, "appmap", "refs", "--map", mapFile(), "--source", "src", "--budget", "4096")
	if code != 0 || !strings.Contains(out, `"misses_labelled":3`) || len(out) > 4096 {
		t.Fatalf("labelled only: %d %s %s", code, diag, out)
	}
	if !strings.Contains(flowsAppmapHelp, "flows appmap refs") {
		t.Fatal("help missing")
	}
}
