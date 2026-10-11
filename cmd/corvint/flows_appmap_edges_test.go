package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/appmap"
)

// ANL-V0-004: end-to-end map -> ledger -> declaration -> proven requirement, no browser or writes.
func TestANLV0004EdgesCLI(t *testing.T) {
	root := appmapCLIRepo(t)
	code, out, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatal(diag)
	}
	var m appmap.Map
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	mapFile := filepath.Join(t.TempDir(), "map.json")
	if err := os.WriteFile(mapFile, []byte(out), 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"appmap", "edges", "--map", mapFile, "--full"}
	code, out, diag = runFlowsCLI(root, base...)
	if code != 0 {
		t.Fatal(diag)
	}
	var ledger appmap.EdgeLedger
	if err := json.Unmarshal([]byte(out), &ledger); err != nil || len(ledger.Edges) == 0 {
		t.Fatalf("%s %v", out, err)
	}
	id := ledger.Edges[0].ID
	require := append(append([]string{}, base...), "--require", id)
	code, out, diag = runFlowsCLI(root, require...)
	if code != 1 {
		t.Fatalf("unproven %d %s %s", code, out, diag)
	}
	receipt, key := rvnReceipt(t, m.Revision)
	declarations := filepath.Join(t.TempDir(), "declarations.json")
	data, _ := json.Marshal(appmap.EdgeDeclarations{Schema: appmap.EdgeDeclarationsSchema, Tests: []appmap.EdgeDeclaration{{TestKey: key, Proves: []string{id}, Assumes: []string{id}}}})
	if err := os.WriteFile(declarations, data, 0600); err != nil {
		t.Fatal(err)
	}
	code, out, diag = runFlowsCLI(root, append(require, "--receipt", receipt, "--declarations", declarations)...)
	if code != 0 {
		t.Fatalf("proven %d %s %s", code, out, diag)
	}
	if err := json.Unmarshal([]byte(out), &ledger); err != nil || ledger.Proven != 1 || !ledger.Satisfied {
		t.Fatal(out, err)
	}
	code, _, _ = runFlowsCLI(root, append(base, "--declarations", declarations)...)
	if code != 1 {
		t.Fatal("declaration invented proof", code)
	}
	if err := os.WriteFile(declarations, []byte(`{"schema":"application-edge-declarations/0","tests":[],"unknown":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	code, _, _ = runFlowsCLI(root, append(base, "--declarations", declarations)...)
	if code != 2 {
		t.Fatal("invalid sidecar", code)
	}
	cmd := exec.Command("git", "status", "--porcelain", "--ignored")
	cmd.Dir = root
	status, err := cmd.Output()
	if err != nil || len(status) != 0 {
		t.Fatalf("read mutated root %s %v", status, err)
	}
}
