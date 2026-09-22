package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHistoricalBenchmarkEvidenceRetainsOriginalVersionIdentity binds PUB-V0-001's second
// sentence: "Historical benchmark and release evidence MUST retain its original version
// identities." TestPublicReleaseVersionTupleMovesTogether only checks that the *current*
// tuple moved together; nothing checked that a version bump left recorded past measurements
// alone. A blanket rewrite (for example a sed pass turning every `"Corvint 0.4.0aN"` into the
// new tuple) would have passed every existing gate.
func TestHistoricalBenchmarkEvidenceRetainsOriginalVersionIdentity(t *testing.T) {
	t.Run("PUB-V0-001-historical-version-identities", func(t *testing.T) {
		root := filepath.Join("..", "..")
		read := func(relative string) string {
			data, err := os.ReadFile(filepath.Join(root, relative))
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		}
		current := strings.TrimSpace(read("VERSION"))
		if current == "" {
			t.Fatal("VERSION is empty")
		}

		// Spot-check named historical evidence still carries the exact version recorded when it
		// was produced, not the current tuple.
		for relative, want := range map[string]string{
			"benchmarks/results/cem-reviewer-trial-pilot.json":                           `"version":"Corvint 0.4.0a1"`,
			"benchmarks/results/cem-reviewer-trial-pilot-2026-09-04.json":                `"version":"Corvint 0.4.0a3"`,
			"benchmarks/results/arb-v2-context-baseline-2026-09-05/trace2code.json":      `"version":"Corvint 0.4.0a3"`,
			"benchmarks/results/arb-v2-context-baseline-2026-09-05/edit2ripple.json":     `"version":"Corvint 0.4.0a3"`,
			"benchmarks/results/arb-v2-context-baseline-2026-09-05/code2test.json":       `"version":"Corvint 0.4.0a3"`,
			"benchmarks/results/arb-v2-context-baseline-2026-09-05/comment2context.json": `"version":"Corvint 0.4.0a3"`,
			".agent-evidence/docs-routing-2026-09-05/measurements.json":                  `"version": "Corvint 0.4.0a3"`,
		} {
			if !strings.Contains(read(relative), want) {
				t.Fatalf("%s no longer carries its original recorded version %s", relative, want)
			}
		}

		// Sweep every benchmark/evidence JSON file: none may carry the *current* version tuple,
		// since these directories hold past measurements, never a live re-stamp on bump.
		currentMarker := `"Corvint ` + current + `"`
		for _, dir := range []string{"benchmarks/results", ".agent-evidence"} {
			absDir := filepath.Join(root, dir)
			err := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || filepath.Ext(path) != ".json" {
					return nil
				}
				data, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				if strings.Contains(string(data), currentMarker) {
					t.Errorf("%s carries the current version tuple %s: historical evidence must keep its original identity", path, current)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	})
}
