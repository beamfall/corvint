package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestCEMV1317HelpDocumentsMapPaths proves the status, verify and report
// --help states the map-path rules: status, verify and report read an
// absolute map outside the repository (CEM-V1-640), the mutators read one
// only into a repository-relative --output (CEM-V1-647), every other action
// is repository-relative, and no action takes a paired change (CEM-V1-648).
func TestCEMV1317HelpDocumentsMapPaths(t *testing.T) {
	wants := []string{
		"Map paths:",
		// CEM-V1-7400 (V1-3863): verify and verify-stable also take an
		// in-repository map either way, so the shared rule now names status
		// and report only.
		"status and report read --map MAP repository-relative, or as an",
		"verify also accepts an absolute in-repository",
		"--repository (CEM-V1-7400)",
		"path outside the repository and its Git directories",
		"an absolute path inside them is refused",
		"a symlinked map is refused",
		"cite, mark and cover read an",
		"--output, because Core writes a map only under the repository's lock",
		"every other action takes MAP",
		"No action takes a paired change: --range, --range-root and --paired-from",
	}
	for _, verb := range []string{"status", "verify", "report"} {
		var out, errout bytes.Buffer
		if exit := runContext(context.Background(), []string{"cem", verb, "--help"}, nil, &out, &errout); exit != 0 {
			t.Fatalf("cem %s --help exit %d: %s", verb, exit, errout.String())
		}
		for _, want := range wants {
			if !strings.Contains(out.String(), want) {
				t.Errorf("cem %s --help lacks %q", verb, want)
			}
		}
	}
}
