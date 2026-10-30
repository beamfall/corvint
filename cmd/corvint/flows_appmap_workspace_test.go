package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// WSD-V0-004: `flows appmap build` takes --workspace FILE and --no-workspace. A workspace whose
// aliases the manifest reads nothing from builds the single-root map unchanged; an invalid one,
// named or discovered, refuses as workspace-invalid; the workspace never merges with --repo.
func TestWSDV0004BuildCLI(t *testing.T) {
	root, filename := railsCLIRepo(t, false)
	want, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	named := filepath.Join(dir, "workspace.json")
	if err := os.WriteFile(named, []byte(`{"profile":"corvint-workspace/0","repos":{"tests":{"root":"tests","revision":"main"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--workspace", named)
	if code != 0 || out != string(want) {
		t.Fatalf("workspace build %d %s", code, diag)
	}
	code, out, diag = runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--workspace", named, "--repo", "tests="+dir)
	if code != 2 || out != "" || !strings.Contains(diag, "invalid-arguments") {
		t.Fatalf("workspace with --repo: %d %s", code, diag)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"profile":"corvint-workspace/0","repos":{"tests":{"root":"tests"}},"extra":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, diag = runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--workspace", bad)
	if code != 2 || !strings.Contains(diag, "workspace-invalid") {
		t.Fatalf("invalid named workspace: %d %s", code, diag)
	}
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", "workspace.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, diag = runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 2 || !strings.Contains(diag, "workspace-invalid") {
		t.Fatalf("invalid discovered workspace: %d %s", code, diag)
	}
	code, out, diag = runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json", "--no-workspace")
	if code != 0 || out != string(want) {
		t.Fatalf("--no-workspace %d %s", code, diag)
	}
}
