package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// AMAP-V0-211: `flows appmap build --from MAP` writes the map a full build gives, carried forward
// or rebuilt; --verify agrees; --verify needs --from; an unreadable --from refuses.
func TestAMAPV0211BuildFromCLI(t *testing.T) {
	root, _ := railsCLIRepo(t, false)
	code, first, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--no-workspace")
	if code != 0 {
		t.Fatalf("base build %d %s", code, diag)
	}
	prev := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(prev, []byte(first), 0o644); err != nil {
		t.Fatal(err)
	}
	commit := func(rel, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "-m", rel}} {
			cmd := exec.Command("git", args...)
			cmd.Dir, cmd.Env = root, append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v %s", args, err, out)
			}
		}
	}
	// AMAP-V0-368: an added path no build input reads re-derives; an added web module rebuilds.
	for _, edit := range [][3]string{{"logo.png", "png", "re-derived only: routers, test-joins"},
		{"app/models/order.rb", "class Order < ActiveRecord::Base\n  # edited\nend\n", "re-derived only: routers, test-joins"},
		{"app/extra.js", "export const x = 1;\n", "rebuilt in full: changed-input"}} {
		commit(edit[0], edit[1])
		code, full, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--no-workspace")
		if code != 0 {
			t.Fatalf("%s full build %d %s", edit[0], code, diag)
		}
		for _, extra := range [][]string{nil, {"--verify"}} {
			args := append([]string{"appmap", "build", "--manifest", "appmap.json", "--no-workspace", "--from", prev}, extra...)
			code, out, diag := runFlowsCLI(root, args...)
			if code != 0 || out != full {
				t.Fatalf("%s --from %v: %d %s", edit[0], extra, code, diag)
			}
			// AMAP-V0-215: a --from build that rebuilt says why.
			if !strings.Contains(diag, edit[2]) {
				t.Fatalf("%s --from %v: stderr %q", edit[0], extra, diag)
			}
		}
	}
	// AMAP-V0-245: from a map of the previous commit, an application source edit re-derives the
	// routers and test joins, says so, and gives the full build's map with or without --verify.
	_, head, _ := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--no-workspace")
	if err := os.WriteFile(prev, []byte(head), 0o644); err != nil {
		t.Fatal(err)
	}
	commit("app/models/order.rb", "class Order < ActiveRecord::Base\n  # edited again\nend\n")
	_, full, _ := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--no-workspace")
	for _, extra := range [][]string{nil, {"--verify"}} {
		args := append([]string{"appmap", "build", "--manifest", "appmap.json", "--no-workspace", "--from", prev}, extra...)
		if code, out, diag := runFlowsCLI(root, args...); code != 0 || out != full || !strings.Contains(diag, "re-derived only: routers, test-joins") {
			t.Fatalf("source edit --from %v: %d %q", extra, code, diag)
		}
	}
	if err := os.WriteFile(prev, []byte(first), 0o644); err != nil {
		t.Fatal(err)
	}
	// The help documents the incremental flags and what each kind of edit re-derives.
	if help := runCatalogHelp(t, "flows", "appmap", "build", "--help"); !strings.Contains(help, "--from MAP") || !strings.Contains(help, "--verify") ||
		!strings.Contains(help, "AMAP-V0-245") || !strings.Contains(help, "index-admission-changed") {
		t.Fatalf("appmap build help lacks the incremental build: %s", help)
	}
	// A carried map is silent.
	if code, _, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--no-workspace", "--from", prev, "--revision", "HEAD~4"); code != 0 || diag != "" {
		t.Fatalf("carried --from: %d %q", code, diag)
	}
	stale, old := filepath.Join(t.TempDir(), "stale.json"), strings.Replace(first, `"builder":"`+appmap.BuilderVersion+`",`, "", 1)
	if old == first {
		t.Fatal("map carries no builder member")
	}
	if err := os.WriteFile(stale, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--no-workspace", "--from", stale); code != 0 || !strings.Contains(diag, "rebuilt in full: builder-version-differs") {
		t.Fatalf("map without a builder: %d %s", code, diag)
	}
	if code, _, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--verify"); code != 2 || !strings.Contains(diag, "--from MAP [--verify]") {
		t.Fatalf("--verify without --from: %d %s", code, diag)
	}
	if code, _, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--from", filepath.Join(root, "missing.json")); code == 0 || !strings.Contains(diag, "appmap-invalid-map") {
		t.Fatalf("missing --from: %d %s", code, diag)
	}
}
