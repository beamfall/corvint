package rust

import (
	"os"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestRealRepositoryDirectTestsCovered is an opt-in real-repository gate. It
// compares direct source attributes, not macro-expanded runtime discovery;
// repositories using generated tests must instead retain the generated-test
// frontier and compare against a compiled harness under a fixed Cargo profile.
func TestRealRepositoryDirectTestsCovered(t *testing.T) {
	root := os.Getenv("CORVINT_RUST_GROUND_TRUTH_ROOT")
	if root == "" {
		t.Skip("CORVINT_RUST_GROUND_TRUTH_ROOT is unset")
	}
	expected := expectedCount(t, "CORVINT_RUST_GROUND_TRUTH_DIRECT_TESTS")
	result, err := New().Units(root)
	if err != nil {
		t.Fatalf("units: %v", err)
	}
	testFiles := map[string]bool{}
	for _, unit := range result.Units {
		for _, relative := range unit.Tests {
			testFiles[relative] = true
		}
	}
	files, err := affected.SourceFiles(root, func(name string) bool { return New().Owns(name) })
	if err != nil {
		t.Fatal(err)
	}
	direct := 0
	covered := 0
	for _, relative := range files {
		body, readErr := affected.ReadSource(root, relative)
		if readErr != nil {
			t.Fatalf("read %s: %v", relative, readErr)
		}
		code := withoutRustStrings(string(body))
		count := len(testAttribute.FindAllStringIndex(code, -1))
		direct += count
		if testFiles[relative] {
			covered += count
		}
	}
	if direct != expected || covered != expected {
		t.Fatalf("direct-test coverage=%d/%d expected=%d", covered, direct, expected)
	}
	t.Logf("ground truth: %d Cargo packages, direct-test coverage %d/%d, frontier=%v", len(result.Units), covered, direct, result.Frontier)
}

func expectedCount(t *testing.T, name string) int {
	t.Helper()
	value := os.Getenv(name)
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		t.Fatalf("%s must be a non-negative integer", name)
	}
	return parsed
}
