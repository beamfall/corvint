package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
)

// collect reads the prior index's divergent classes and new misses, the
// inputs CarryDivergent folds into the next index (MCJ-V0-018).
func TestDivergenceInputs(t *testing.T) {
	dir := t.TempDir()
	prior, err := linecov.Build("example.com/m", "c0", "go test", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	prior.Divergent = []linecov.Divergence{{Class: "k1", Files: []linecov.FileBlob{{Path: "a.go", OID: "x"}}}}
	priorFile := filepath.Join(dir, "prior")
	f, err := os.Create(priorFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := prior.Encode(f); err != nil {
		t.Fatal(err)
	}
	f.Close()
	misses, _ := json.Marshal([]linecov.Miss{{Class: "k2", Excluded: "p.TestA", Representative: "p.TestB"}})
	missFile := filepath.Join(dir, "misses.json")
	if err := os.WriteFile(missFile, misses, 0o644); err != nil {
		t.Fatal(err)
	}
	divs, ms, err := divergenceInputs(priorFile, missFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(divs) != 1 || divs[0].Class != "k1" || len(ms) != 1 || ms[0].Class != "k2" {
		t.Fatalf("divergent %+v misses %+v", divs, ms)
	}
	if d, m, err := divergenceInputs("", ""); err != nil || d != nil || m != nil {
		t.Fatalf("no inputs: %v %v %v", d, m, err)
	}
	if err := os.WriteFile(missFile, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := divergenceInputs("", missFile); err == nil {
		t.Fatal("malformed misses accepted")
	}
}
