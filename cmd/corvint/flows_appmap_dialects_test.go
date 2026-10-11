package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// AMAP-V0-048: the public build emits mixed-dialect screens and declared-scope coverage;
// the public screen projection can resolve the resulting Rails and React URLs.
func TestAMAPV0048DialectCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	raw, err := os.ReadFile(filepath.Join(root, "appmap.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest appmap.Manifest
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.HashPrefix = ""
	manifest.Routers = append(manifest.Routers, appmap.RouterFile{Path: "app/rails.rb", Dialect: appmap.DialectRails}, appmap.RouterFile{Path: "app/react.tsx", Dialect: appmap.DialectReact})
	manifest.RouterGlobs = []string{"app/*"}
	raw, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"appmap.json": string(raw), "app/rails.rb": "get 'legacy/:id', to: 'legacy#show', as: :legacy\n", "app/react.tsx": `createBrowserRouter([{id:'island',path:'/island/:id',Component:Island}])`, "app/extra.rb": "get 'extra'\n"}
	for name, body := range files {
		if err = os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "dialects"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
	}
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatalf("build: %d %s", code, diag)
	}
	var m appmap.Map
	if err = json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m.AppCoverage == nil || m.AppCoverage.Status != appmap.StatusUnknown || len(m.AppCoverage.Undeclared) != 1 || m.AppCoverage.Undeclared[0] != "app/extra.rb" {
		t.Fatalf("coverage: %+v", m.AppCoverage)
	}
	mapFile := filepath.Join(t.TempDir(), "map.json")
	if err = os.WriteFile(mapFile, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"/legacy/7", "/island/one"} {
		code, out, diag = runFlowsCLI(root, "appmap", "screen", "--map", mapFile, "--screen", url)
		var doc map[string]any
		if code != 0 || json.Unmarshal([]byte(out), &doc) != nil || doc["status"] != appmap.StatusResolved {
			t.Fatalf("%s: %d %s %s", url, code, diag, out)
		}
	}
}
