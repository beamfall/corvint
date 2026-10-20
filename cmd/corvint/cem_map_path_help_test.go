package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestCEMV1317HelpDocumentsMapPaths proves the status, verify and anchor
// --help states the map-path rules: status and verify read an
// absolute map outside the repository, every other action is
// repository-relative, and anchor's committed-map limit stays explicit.
func TestCEMV1317HelpDocumentsMapPaths(t *testing.T) {
	wants := []string{
		"Map paths:",
		"status and verify read --map MAP repository-relative, or as an absolute",
		"path outside the repository and its Git directories",
		"an absolute path inside them is refused",
		"a symlinked map is refused",
		"every other action takes MAP",
		"there is no out-of-tree anchor or verdict-ledger mode yet",
	}
	for _, verb := range []string{"status", "verify", "anchor"} {
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
