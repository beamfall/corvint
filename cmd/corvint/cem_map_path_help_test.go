package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestCEMV1317HelpDocumentsMapPaths proves the status, verify, report and
// anchor --help states the map-path rules: status, verify and report read an
// absolute map outside the repository (CEM-V1-640), so do prove --cem and
// its attestation verifier (CEM-V1-641), the mutators read one only into a
// repository-relative --output (CEM-V1-647), every other action is
// repository-relative, anchor's committed-map limit stays explicit, and no
// action takes a paired change (CEM-V1-648).
func TestCEMV1317HelpDocumentsMapPaths(t *testing.T) {
	wants := []string{
		"Map paths:",
		"status, verify and report read --map MAP repository-relative, or as an",
		"path outside the repository and its Git directories",
		"an absolute path inside them is refused",
		"a symlinked map is refused",
		"prove --cem and --verify-cem-attestation --cem read MAP the same way",
		"external:<sha256 of its bytes>",
		"cite, mark, cover and discriminate read an",
		"--output, because Core writes a map only under the repository's lock",
		"every other action takes MAP",
		"there is no out-of-tree anchor or verdict-ledger mode yet",
		"No action takes a paired change: --range, --range-root and --paired-from",
	}
	for _, verb := range []string{"status", "verify", "report", "anchor", "discriminate"} {
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
