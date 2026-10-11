package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AMAP-V0-050 AMAP-V0-057 AMAP-V0-059: real CLI build/load/project stays bounded and
// read-only; runtime flags and malformed inputs refuse before any output or application visit.
func TestEntriesCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatalf("build: %s", diag)
	}
	filename := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(filename, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{nil, {"--budget", "2048"}, {"--full"}} {
		code, out, diag = runFlowsCLI(root, append([]string{"appmap", "entries", "--map", filename}, flags...)...)
		var doc map[string]any
		if code != 0 || json.Unmarshal([]byte(out), &doc) != nil || doc["schema"] != "application-map-entries/0" || doc["basis"] != "static-only" {
			t.Fatalf("entries: %d %s %s", code, diag, out)
		}
		cap := 6144
		if len(flags) > 0 {
			if flags[0] == "--full" {
				cap = 1 << 20
			} else {
				cap = 2048
			}
		}
		if len(out) > cap || doc["omitted"] == nil {
			t.Fatalf("cap: %s", out)
		}
	}
	for _, flags := range [][]string{{"--runtime"}, {"--trusted-local"}, {"--receipt", "absent"}, {"--budget", "1"}, {"--budget", "4096", "--full"}, {"extra"}} {
		code, out, _ = runFlowsCLI(root, append([]string{"appmap", "entries", "--map", filename}, flags...)...)
		if code != 2 || out != "" {
			t.Fatalf("admitted %v: %d %s", flags, code, out)
		}
	}
	for _, args := range [][]string{{"appmap", "entries"}, {"appmap", "entries", "--map", filename + "missing"}} {
		code, out, _ = runFlowsCLI(root, args...)
		if code != 2 || out != "" {
			t.Fatalf("admitted %v", args)
		}
	}
	if !strings.Contains(flowsAppmapHelp, "flows appmap entries") {
		t.Fatal("help missing")
	}
	cmd := exec.Command("git", "status", "--porcelain", "--ignored")
	cmd.Dir = root
	if status, err := cmd.Output(); err != nil || len(status) != 0 {
		t.Fatalf("mutated repo: %s %v", status, err)
	}
}
