package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// AMAP-V0-810 AMAP-V0-812: `flows appmap coverage` writes application-map-coverage/0 and
// refuses a missing map, a stray argument and a bad budget with exit 2 and no stdout.
func TestFlowsAppmapCoverageCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatalf("build: %s", diag)
	}
	filename := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(filename, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, diag = runFlowsCLI(root, "appmap", "coverage", "--map", filename)
	var doc struct {
		Schema  string           `json:"schema"`
		Modules []map[string]any `json:"modules"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &doc) != nil || doc.Schema != "application-map-coverage/0" || len(doc.Modules) == 0 || len(out) > 6144 {
		t.Fatalf("coverage: %d %s %s", code, diag, out)
	}
	for _, flags := range [][]string{{"--budget", "1"}, {"extra"}, {"--full", "--budget", "4096"}} {
		if code, out, _ = runFlowsCLI(root, append([]string{"appmap", "coverage", "--map", filename}, flags...)...); code != 2 || out != "" {
			t.Fatalf("admitted %v: %d", flags, code)
		}
	}
	if code, out, _ = runFlowsCLI(root, "appmap", "coverage"); code != 2 || out != "" {
		t.Fatalf("admitted a missing --map: %d", code)
	}
}
