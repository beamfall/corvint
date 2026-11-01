package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFlowsAppmapCoverageCLI_V12770(t *testing.T) {
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
		Total   int              `json:"screens_total"`
		Modules []map[string]any `json:"modules"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &doc) != nil || doc.Schema != "application-map-coverage/0" || len(doc.Modules) == 0 || doc.Total == 0 {
		t.Fatalf("coverage: %d %s %s", code, diag, out)
	}
	for _, flags := range [][]string{{}, {"--map", filename, "extra"}, {"--map", filename, "--budget", "1"}} {
		if code, out, _ = runFlowsCLI(root, append([]string{"appmap", "coverage"}, flags...)...); code != 2 || out != "" {
			t.Fatalf("admitted %v: %d %s", flags, code, out)
		}
	}
}
