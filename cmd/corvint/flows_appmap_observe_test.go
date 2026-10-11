package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// ANL-V0-006: end-to-end map -> reporter JSON -> observation over the committed fixture.
func TestANLV0006ObserveCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatal(diag)
	}
	var m appmap.Map
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	mapFile := filepath.Join(dir, "map.json")
	if err := os.WriteFile(mapFile, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	tpl := ""
	for _, s := range m.Screens {
		if s.Status == appmap.StatusResolved && !s.Abstract && s.Template != "" && !strings.ContainsAny(s.Template, "{:") {
			tpl = m.HashPrefix + s.Template
			break
		}
	}
	trace := filepath.Join(dir, "t.json")
	body := `{"schema":"playwright-navigation-events/0","test_key":"k","events":[{"kind":"navigate","url":"https://x.test/` + tpl + `?token=abc"},{"kind":"navigate","url":"https://x.test/nowhere/at/all"}]}`
	if err := os.WriteFile(trace, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, diag = runFlowsCLI(root, "appmap", "observe", "--map", mapFile, "--trace", trace)
	if code != 0 || strings.Contains(out, "abc") || !strings.Contains(out, "unmapped-url") || !strings.Contains(out, "application-navigation-observation/0") {
		t.Fatal(code, out, diag)
	}
	if code, _, _ = runFlowsCLI(root, "appmap", "observe", "--map", mapFile); code == 0 {
		t.Fatal("missing --trace accepted")
	}
}
