package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// PUB-V0-671 (rescue-c review): with CORVINT_HOST_ARCHIVE_PROOF=1 on Darwin arm64,
// TestCorvintHostArchivePartialProof archives the enclosing checkout's HEAD and builds all of
// cmd/corvint, and the affected planner does not condition a read scope on that variable. A
// declared scope would let a change confined to cmd/corvint exclude this package, so it keeps the
// UNBOUNDED_READER floor and declares no read scope.
func TestPUBV0671HostArchiveProofKeepsTheUnboundedFloor(t *testing.T) {
	const unit = "conformance/release-artifact-v0"
	root := filepath.Join("..", "..")
	var floor struct {
		Units   []string          `json:"units"`
		Reasons map[string]string `json:"reasons"`
	}
	readJSON(t, filepath.Join(root, ".corvint", "unbounded-readers.json"), &floor)
	if !slices.Contains(floor.Units, unit) || floor.Reasons[unit] == "" {
		t.Errorf("%s is not a reasoned member of the UNBOUNDED_READER floor in .corvint/unbounded-readers.json", unit)
	}
	var scopes struct {
		Packages map[string][]string `json:"packages"`
	}
	readJSON(t, filepath.Join(root, ".corvint", "test-read-scopes.json"), &scopes)
	if entries, ok := scopes.Packages[unit]; ok {
		t.Errorf("%s declares a read scope %v, which a cmd/corvint-only change could use to exclude the host archive proof", unit, entries)
	}
}

func readJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}
