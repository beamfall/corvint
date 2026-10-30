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
	// AMAP-V0-080: a ledger built from this map is accepted; a missing or unbound one refuses.
	code, out, diag = runFlowsCLI(root, "appmap", "edges", "--map", filename, "--full")
	ledger := filepath.Join(t.TempDir(), "ledger.json")
	if code != 0 || os.WriteFile(ledger, []byte(out), 0600) != nil {
		t.Fatalf("edges: %d %s", code, diag)
	}
	code, out, diag = runFlowsCLI(root, "appmap", "entries", "--map", filename, "--ledger", ledger, "--full")
	if code != 0 || !strings.Contains(out, `"basis":"static-entries+edge-ledger"`) || !strings.Contains(out, `"path_status":"NOT_OBSERVED"`) {
		t.Fatalf("ledger entries: %d %s %s", code, diag, out)
	}
	unbound := filepath.Join(t.TempDir(), "unbound.json")
	raw, _ := os.ReadFile(ledger)
	var doc map[string]any
	if json.Unmarshal(raw, &doc) != nil {
		t.Fatal("ledger json")
	}
	doc["map_digest"] = "sha256:other"
	raw, _ = json.Marshal(doc)
	if err := os.WriteFile(unbound, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, l := range []string{ledger + "missing", unbound} {
		if code, out, _ = runFlowsCLI(root, "appmap", "entries", "--map", filename, "--ledger", l); code != 2 || out != "" {
			t.Fatalf("admitted ledger %s: %d %s", l, code, out)
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

// AMAP-V0-060 AMAP-V0-064: stats writes the report and --min-resolved gates the exit code.
func TestStatsCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatalf("build: %s", diag)
	}
	filename := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(filename, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, diag = runFlowsCLI(root, "appmap", "stats", "--map", filename)
	var doc map[string]any
	if code != 0 || json.Unmarshal([]byte(out), &doc) != nil || doc["schema"] != "application-map-stats/0" || len(out) > 6144 {
		t.Fatalf("stats: %d %s %s", code, diag, out)
	}
	if code, _, _ = runFlowsCLI(root, "appmap", "stats", "--map", filename, "--min-resolved", "0"); code != 0 {
		t.Fatalf("0%% gate: %d", code)
	}
	code, out, diag = runFlowsCLI(root, "appmap", "stats", "--map", filename, "--min-resolved", "100")
	resolved, total := doc["screens_resolved"].(float64), doc["screens_total"].(float64)
	if resolved == total {
		if code != 0 {
			t.Fatalf("full map failed gate: %d %s", code, diag)
		}
	} else if code != 1 || out == "" || !strings.Contains(diag, "dominant unknown reasons") {
		t.Fatalf("gate: %d %q %s", code, out, diag)
	}
	for _, flags := range [][]string{{"--min-resolved", "101"}, {"--min-resolved", "-1"}, {"--min-resolved", "NaN"}, {"--budget", "1"}, {"extra"}} {
		if code, out, _ = runFlowsCLI(root, append([]string{"appmap", "stats", "--map", filename}, flags...)...); code != 2 || out != "" {
			t.Fatalf("admitted %v: %d", flags, code)
		}
	}
}
